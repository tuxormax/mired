package dpi

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"sync"
	"time"

	"github.com/mdlayher/packet"
)

// La captura: leer del cable y sumar.
//
// Se usa AF_PACKET en Go puro (mdlayher/packet), NO libpcap. Es la misma razon
// por la que todo el proyecto es Go: meter libpcap obligaria a compilar con cgo,
// y ahi se acaban los binarios estaticos y el `.deb` que se instala sin
// dependencias.
//
// **Para que esto sirva de algo hace falta un puerto espejo.** Un switch manda a
// cada puerto solo lo suyo, asi que un equipo conectado normalmente ve su propio
// trafico y poco mas. Hay que decirle al switch que copie al puerto de MiRed
// todo lo que pasa por otra —lo que se llama puerto espejo o SPAN—. Sin eso,
// esto no falla: mide menos, y el informe lo dice.

// Consumo es lo que un equipo movio hablando con un destino.
type Consumo struct {
	IP         string `json:"ip"`
	Aplicacion string `json:"aplicacion"`
	Como       string `json:"como"`
	Bytes      uint64 `json:"bytes"`
	Paquetes   int    `json:"paquetes"`
}

// Capturador escucha una interfaz y va sumando por equipo y aplicacion.
type Capturador struct {
	// Interfaz es la tarjeta donde llega el puerto espejo.
	Interfaz string
	// EsLocal dice si una direccion es de una red vigilada. Sin esto no se sabe
	// a cual de los dos extremos de una conversacion apuntarle el consumo.
	EsLocal func(netip.Addr) bool

	mu       sync.Mutex
	suma     map[claveConsumo]*Consumo
	vistos   uint64
	desde    time.Time
	conexion *packet.Conn
}

type claveConsumo struct {
	ip         string
	aplicacion string
}

// Escuchar abre la interfaz en modo promiscuo y captura hasta que se cancele.
//
// El modo promiscuo es imprescindible: sin el, la tarjeta descarta en hardware
// todo lo que no vaya dirigido a su propia MAC, que es justo todo lo que copia
// el puerto espejo.
func (c *Capturador) Escuchar(ctx context.Context) error {
	interfaz, err := net.InterfaceByName(c.Interfaz)
	if err != nil {
		return fmt.Errorf("no existe la interfaz %s: %w", c.Interfaz, err)
	}

	conexion, err := packet.Listen(interfaz, packet.Raw, 0, nil)
	if err != nil {
		return fmt.Errorf("no se pudo escuchar en %s (¿falta CAP_NET_RAW?): %w", c.Interfaz, err)
	}
	defer conexion.Close()

	if err := conexion.SetPromiscuous(true); err != nil {
		return fmt.Errorf("no se pudo poner %s en modo promiscuo: %w", c.Interfaz, err)
	}

	c.mu.Lock()
	c.conexion = conexion
	c.desde = time.Now()
	if c.suma == nil {
		c.suma = map[claveConsumo]*Consumo{}
	}
	c.mu.Unlock()

	// Cerrar la conexion es lo que desbloquea la lectura: sin esto, cancelar el
	// contexto no serviria de nada porque ReadFrom se queda esperando un paquete
	// que quiza no llegue nunca.
	go func() {
		<-ctx.Done()
		conexion.Close()
	}()

	buzon := make([]byte, 2048)
	for {
		leidos, _, err := conexion.ReadFrom(buzon)
		if err != nil {
			if ctx.Err() != nil {
				return nil // cierre normal
			}
			return fmt.Errorf("se corto la captura en %s: %w", c.Interfaz, err)
		}
		c.Procesar(buzon[:leidos])
	}
}

// Cortar entrega lo acumulado y limpia la cuenta.
func (c *Capturador) Cortar() []Consumo {
	c.mu.Lock()
	consumos := make([]Consumo, 0, len(c.suma))
	for _, consumo := range c.suma {
		consumos = append(consumos, *consumo)
	}
	c.suma = map[claveConsumo]*Consumo{}
	c.mu.Unlock()

	// De mayor a menor: lo que interesa de este informe son los diez primeros.
	sort.Slice(consumos, func(i, j int) bool { return consumos[i].Bytes > consumos[j].Bytes })
	return consumos
}

// Paquetes dice cuantos se han visto desde que arranco.
//
// Sirve para lo unico que no se puede deducir del informe: si el puerto espejo
// esta bien configurado. Con un puñado de paquetes por minuto, la respuesta es
// que no.
func (c *Capturador) Paquetes() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.vistos
}

// Procesar desarma un paquete y le apunta los bytes a quien corresponda.
func (c *Capturador) Procesar(trama []byte) {
	c.mu.Lock()
	c.vistos++
	if c.suma == nil {
		c.suma = map[claveConsumo]*Consumo{}
	}
	c.mu.Unlock()

	origen, destino, protocolo, carga, ok := desarmar(trama)
	if !ok {
		return
	}

	esLocal := func(direccion netip.Addr) bool {
		return c.EsLocal == nil || c.EsLocal(direccion)
	}

	// El consumo se le apunta al equipo de la red, hable el o le hablen: en los
	// dos casos el ancho de banda es suyo.
	var quien netip.Addr
	switch {
	case esLocal(origen) && !esLocal(destino):
		quien = origen
	case esLocal(destino) && !esLocal(origen):
		quien = destino
	case esLocal(origen):
		quien = origen // trafico interno: se le apunta al que empezo
	default:
		return // ninguno de los dos es de aqui
	}

	var pista Pista
	var reconocido bool
	if protocolo == 6 {
		pista, reconocido = IdentificarTCP(carga.datos, carga.puertoDestino)
	} else {
		pista, reconocido = IdentificarUDP(carga.datos, carga.puertoDestino)
	}
	if !reconocido || pista.Nombre == "" {
		// Un paquete que no se delata no se tira: sus bytes son reales y tienen
		// que aparecer en algun lado, o el informe sumaria menos de lo que paso
		// por el cable y nadie sabria por que.
		pista = Pista{Nombre: "Sin identificar", Como: "desconocido"}
	} else if pista.Como != "puerto" {
		pista.Nombre = DominioPrincipal(pista.Nombre)
	}

	clave := claveConsumo{ip: quien.String(), aplicacion: pista.Nombre}

	c.mu.Lock()
	defer c.mu.Unlock()
	consumo, hay := c.suma[clave]
	if !hay {
		consumo = &Consumo{IP: clave.ip, Aplicacion: clave.aplicacion, Como: pista.Como}
		c.suma[clave] = consumo
	}
	consumo.Bytes += uint64(len(trama))
	consumo.Paquetes++
	// El nombre manda sobre el puerto: si en algun paquete de esta conversacion
	// se supo con quien se hablaba, esa es la mejor respuesta.
	if consumo.Como == "desconocido" && pista.Como != "desconocido" {
		consumo.Como = pista.Como
	}
}

type cargaUtil struct {
	datos         []byte
	puertoDestino int
}

const (
	tipoIPv4 = 0x0800
	tipoIPv6 = 0x86DD
	tipoVLAN = 0x8100
)

// desarmar saca de una trama las direcciones, el protocolo y la carga util.
func desarmar(trama []byte) (netip.Addr, netip.Addr, uint8, cargaUtil, bool) {
	var vacia netip.Addr
	if len(trama) < 14 {
		return vacia, vacia, 0, cargaUtil{}, false
	}

	posicion := 12
	tipo := binary.BigEndian.Uint16(trama[posicion : posicion+2])
	posicion += 2
	// Un puerto espejo suele entregar las tramas con su etiqueta de VLAN puesta.
	for i := 0; i < 2 && tipo == tipoVLAN; i++ {
		if posicion+4 > len(trama) {
			return vacia, vacia, 0, cargaUtil{}, false
		}
		tipo = binary.BigEndian.Uint16(trama[posicion+2 : posicion+4])
		posicion += 4
	}

	var origen, destino netip.Addr
	var protocolo uint8
	switch tipo {
	case tipoIPv4:
		if posicion+20 > len(trama) {
			return vacia, vacia, 0, cargaUtil{}, false
		}
		cabecera := int(trama[posicion]&0x0F) * 4
		if cabecera < 20 || posicion+cabecera > len(trama) {
			return vacia, vacia, 0, cargaUtil{}, false
		}
		protocolo = trama[posicion+9]
		origen, _ = netip.AddrFromSlice(trama[posicion+12 : posicion+16])
		destino, _ = netip.AddrFromSlice(trama[posicion+16 : posicion+20])
		posicion += cabecera
	case tipoIPv6:
		if posicion+40 > len(trama) {
			return vacia, vacia, 0, cargaUtil{}, false
		}
		protocolo = trama[posicion+6]
		origen, _ = netip.AddrFromSlice(trama[posicion+8 : posicion+24])
		destino, _ = netip.AddrFromSlice(trama[posicion+24 : posicion+40])
		posicion += 40
	default:
		return vacia, vacia, 0, cargaUtil{}, false
	}

	if !origen.IsValid() || !destino.IsValid() {
		return vacia, vacia, 0, cargaUtil{}, false
	}

	switch protocolo {
	case 6: // TCP
		if posicion+20 > len(trama) {
			return vacia, vacia, 0, cargaUtil{}, false
		}
		puerto := int(binary.BigEndian.Uint16(trama[posicion+2 : posicion+4]))
		cabecera := int(trama[posicion+12]>>4) * 4
		if cabecera < 20 || posicion+cabecera > len(trama) {
			return vacia, vacia, 0, cargaUtil{}, false
		}
		return origen, destino, protocolo,
			cargaUtil{datos: trama[posicion+cabecera:], puertoDestino: puerto}, true
	case 17: // UDP
		if posicion+8 > len(trama) {
			return vacia, vacia, 0, cargaUtil{}, false
		}
		puerto := int(binary.BigEndian.Uint16(trama[posicion+2 : posicion+4]))
		return origen, destino, protocolo,
			cargaUtil{datos: trama[posicion+8:], puertoDestino: puerto}, true
	default:
		// ICMP y demas: son bytes reales que pasaron por el cable, y se cuentan
		// como "sin identificar" en vez de desaparecer de la suma.
		return origen, destino, protocolo, cargaUtil{}, true
	}
}
