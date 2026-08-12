// Paquete flujos recibe lo que el router exporta sobre el trafico que pasa por
// el.
//
// Es la respuesta a "¿quien consume?" en los sitios SIN switches administrables,
// que es el caso mas comun en una red chica. El router ya sabe quien habla con
// quien y cuanto; MiRed solo escucha y suma. No captura paquetes ni se pone en
// el camino del trafico: por eso cuesta casi nada.
//
// Se implementa NetFlow v5, que es el formato de tamano fijo que exportan
// practicamente todos los routers de casa y de oficina (MikroTik y pfSense
// incluidos). Los formatos con plantilla (v9 e IPFIX) quedan para despues.
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

// Tamanos del formato v5, que es fijo.
const (
	tamanoCabecera = 24
	tamanoRegistro = 48
	versionV5      = 5
	maximoV5       = 30 // registros que caben en un datagrama
)

// Consumo es lo que junto una direccion durante un periodo.
type Consumo struct {
	IP             string
	BytesSube      uint64
	BytesBaja      uint64
	Conversaciones int
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
		leidos, _, err := socket.ReadFrom(buzon)
		if err != nil {
			// Socket cerrado: es la forma normal de terminar.
			return nil
		}
		r.Procesar(buzon[:leidos])
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

// Procesar interpreta un datagrama NetFlow v5.
func (r *Receptor) Procesar(datos []byte) {
	if len(datos) < tamanoCabecera {
		return
	}
	if binary.BigEndian.Uint16(datos[0:2]) != versionV5 {
		// Otro formato (v9 o IPFIX): se ignora en silencio en vez de llenar la
		// bitacora, porque un router mal configurado mandaria miles por minuto.
		return
	}

	cuantos := int(binary.BigEndian.Uint16(datos[2:4]))
	if cuantos > maximoV5 {
		cuantos = maximoV5
	}

	r.mu.Lock()
	if r.suma == nil {
		r.suma = map[string]*Consumo{}
	}
	r.mu.Unlock()

	for i := 0; i < cuantos; i++ {
		inicio := tamanoCabecera + i*tamanoRegistro
		if inicio+tamanoRegistro > len(datos) {
			return
		}
		r.sumarRegistro(datos[inicio : inicio+tamanoRegistro])
	}
}

func (r *Receptor) sumarRegistro(registro []byte) {
	origen, ok := netip.AddrFromSlice(registro[0:4])
	if !ok {
		return
	}
	destino, ok := netip.AddrFromSlice(registro[4:8])
	if !ok {
		return
	}
	octetos := uint64(binary.BigEndian.Uint32(registro[20:24]))
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
	}
	if esLocal(destino) {
		consumo := r.tomar(destino.String())
		consumo.BytesBaja += octetos
		consumo.Conversaciones++
	}
}

func (r *Receptor) tomar(ip string) *Consumo {
	if existente, hay := r.suma[ip]; hay {
		return existente
	}
	nuevo := &Consumo{IP: ip}
	r.suma[ip] = nuevo
	return nuevo
}
