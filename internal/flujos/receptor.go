// Paquete flujos recibe lo que el router exporta sobre el trafico que pasa por
// el.
//
// Es la respuesta a "¿quien consume?" en los sitios SIN switches administrables,
// que es el caso mas comun en una red chica. El router ya sabe quien habla con
// quien y cuanto; MiRed solo escucha y suma. No captura paquetes ni se pone en
// el camino del trafico: por eso cuesta casi nada.
//
// Se entienden los cuatro formatos que exporta el equipo que uno se encuentra:
//
//	NetFlow v5   tamano fijo, el mas viejo y el mas comun (MikroTik, pfSense)
//	NetFlow v9   con plantillas; el exportador dice primero que campos manda
//	IPFIX (v10)  el estandar de IETF, hermano de v9 con otro sobre
//	sFlow v5     muestreo: manda 1 de cada N paquetes crudos (HP, Dell, Brocade)
//
// Los tres primeros CUENTAN el trafico; sFlow lo ESTIMA, porque solo ve una
// muestra. Esa diferencia no se disimula: viaja en el dato hasta la pantalla.
//
// Cada formato vive en su archivo: netflow.go, plantillas.go y sflow.go.
package flujos

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"
)

// PuertoPorOmision es el que usa NetFlow por convencion.
const PuertoPorOmision = 2055

// Version que anuncia cada formato en su cabecera.
const (
	versionV5    = 5
	versionV9    = 9
	versionIPFIX = 10
	versionSFlow = 5 // pero en 32 bits, ver Procesar
)

// Consumo es lo que junto una direccion durante un periodo.
type Consumo struct {
	IP             string
	BytesSube      uint64
	BytesBaja      uint64
	Conversaciones int
	// Estimado dice que la cifra sale de un muestreo (sFlow) y no de una cuenta
	// exacta. **Se conserva hasta la pantalla**: presentar un estimado como si
	// fuera una medicion es la clase de mentira silenciosa que este proyecto
	// evita a proposito en el mapa, y aqui vale igual.
	Estimado bool
}

// Receptor escucha los flujos y los va sumando por direccion.
type Receptor struct {
	// EsLocal dice si una direccion es de una red vigilada. Sin esto no se puede
	// saber si un flujo es de subida o de bajada.
	EsLocal func(netip.Addr) bool
	// AlCortar entrega lo acumulado cada periodo y limpia la cuenta.
	AlCortar func(consumos []Consumo)
	// Periodo es cada cuanto se entrega lo acumulado.
	Periodo time.Duration

	mu     sync.Mutex
	suma   map[string]*Consumo
	socket net.PacketConn

	// plantillas guarda lo que cada exportador declaro de sus registros. Solo la
	// usan NetFlow v9 e IPFIX; v5 y sFlow no necesitan nada previo.
	plantillas almacenDePlantillas
}

// Escuchar abre el puerto y recibe hasta que se cierre.
func (r *Receptor) Escuchar(direccion string) error {
	if r.Periodo <= 0 {
		r.Periodo = time.Minute
	}

	r.mu.Lock()
	if r.suma == nil {
		r.suma = map[string]*Consumo{}
	}
	r.mu.Unlock()

	socket, err := net.ListenPacket("udp", direccion)
	if err != nil {
		return fmt.Errorf("no se pudo escuchar flujos en %s: %w", direccion, err)
	}
	r.mu.Lock()
	r.socket = socket
	r.mu.Unlock()

	go r.cortarCadaTanto()

	buzon := make([]byte, 65535)
	for {
		leidos, remitente, err := socket.ReadFrom(buzon)
		if err != nil {
			// Socket cerrado: es la forma normal de terminar.
			return nil
		}
		// De quien llego importa: las plantillas de NetFlow v9 e IPFIX son de
		// cada exportador, y dos routers pueden numerarlas igual.
		r.ProcesarDe(soloLaDireccion(remitente), buzon[:leidos])
	}
}

// Cerrar detiene la escucha.
func (r *Receptor) Cerrar() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.socket == nil {
		return nil
	}
	socket := r.socket
	r.socket = nil
	return socket.Close()
}

// Cortar entrega lo acumulado y limpia la cuenta. Lo llama el reloj interno, y
// las pruebas lo usan directo.
func (r *Receptor) Cortar() []Consumo {
	r.mu.Lock()
	consumos := make([]Consumo, 0, len(r.suma))
	for _, consumo := range r.suma {
		consumos = append(consumos, *consumo)
	}
	r.suma = map[string]*Consumo{}
	r.mu.Unlock()
	return consumos
}

func (r *Receptor) cortarCadaTanto() {
	reloj := time.NewTicker(r.Periodo)
	defer reloj.Stop()

	for range reloj.C {
		r.mu.Lock()
		vivo := r.socket != nil
		r.mu.Unlock()
		if !vivo {
			return
		}

		consumos := r.Cortar()
		if len(consumos) > 0 && r.AlCortar != nil {
			r.AlCortar(consumos)
		}
	}
}

// Procesar interpreta un datagrama que no dice de quien viene.
//
// Sirve para un solo exportador y para las pruebas. En produccion se usa
// ProcesarDe, porque las plantillas de v9 e IPFIX son de cada router.
func (r *Receptor) Procesar(datos []byte) { r.ProcesarDe("", datos) }

// ProcesarDe interpreta un datagrama y lo reparte al formato que le toca.
//
// Los cuatro formatos se distinguen por los primeros bytes, y la distincion es
// **exacta, no heuristica**: NetFlow pone su version en los dos primeros bytes,
// y sFlow en los cuatro primeros. Como la version de sFlow tambien es 5, un
// datagrama de sFlow leido como NetFlow daria version 0 en esos dos bytes; por
// eso el cero es la senal de que hay que leer cuatro.
func (r *Receptor) ProcesarDe(exportador string, datos []byte) {
	if len(datos) < 8 {
		return
	}

	r.mu.Lock()
	if r.suma == nil {
		r.suma = map[string]*Consumo{}
	}
	r.mu.Unlock()

	switch binary.BigEndian.Uint16(datos[0:2]) {
	case versionV5:
		r.procesarNetFlow5(datos)
	case versionV9:
		r.procesarConPlantilla(exportador, datos, versionV9)
	case versionIPFIX:
		r.procesarConPlantilla(exportador, datos, versionIPFIX)
	case 0:
		if binary.BigEndian.Uint32(datos[0:4]) == versionSFlow {
			r.procesarSFlow(datos)
		}
	default:
		// Formato desconocido: se ignora en silencio en vez de llenar la
		// bitacora, porque un router mal configurado mandaria miles por minuto.
	}
}

// sumarFlujo apunta un flujo ya interpretado, venga del formato que venga.
//
// Es el unico punto donde se decide si algo es subida o bajada, para que los
// cuatro formatos no puedan contestar distinto a la misma pregunta.
func (r *Receptor) sumarFlujo(origen, destino netip.Addr, octetos uint64, estimado bool) {
	if octetos == 0 {
		return
	}

	esLocal := func(direccion netip.Addr) bool {
		return r.EsLocal != nil && r.EsLocal(direccion)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Un flujo entre dos equipos de la misma red suma a los dos: para el que
	// manda es subida y para el que recibe es bajada.
	if esLocal(origen) {
		consumo := r.tomar(origen.String())
		consumo.BytesSube += octetos
		consumo.Conversaciones++
		consumo.Estimado = consumo.Estimado || estimado
	}
	if esLocal(destino) {
		consumo := r.tomar(destino.String())
		consumo.BytesBaja += octetos
		consumo.Conversaciones++
		consumo.Estimado = consumo.Estimado || estimado
	}
}

// soloLaDireccion se queda con la IP del remitente y tira el puerto.
//
// El puerto de origen de un router cambia entre datagramas; usarlo en la clave
// haria que cada paquete pareciera venir de un exportador nuevo y las
// plantillas nunca se encontrarian.
func soloLaDireccion(remitente net.Addr) string {
	if udp, ok := remitente.(*net.UDPAddr); ok && udp != nil {
		return udp.IP.String()
	}
	if remitente == nil {
		return ""
	}
	return remitente.String()
}

func (r *Receptor) tomar(ip string) *Consumo {
	if existente, hay := r.suma[ip]; hay {
		return existente
	}
	nuevo := &Consumo{IP: ip}
	r.suma[ip] = nuevo
	return nuevo
}
