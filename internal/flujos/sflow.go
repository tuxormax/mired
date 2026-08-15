package flujos

import (
	"encoding/binary"
	"net/netip"
)

// sFlow v5: el que exportan HP, Dell, Brocade y buena parte del equipo de
// centro de datos.
//
// **sFlow no cuenta el trafico: lo muestrea.** El switch toma uno de cada N
// paquetes, le corta la cabecera y la manda tal cual. Quien recibe reconstruye
// el total multiplicando por N. Eso tiene dos consecuencias que mandan sobre
// todo el diseño de este archivo:
//
//  1. **La cifra es una estimacion, no una medicion.** Con 1 de cada 2 000, un
//     equipo que hizo pasar 100 MB puede salir con 96 o con 104. Sirve de sobra
//     para "¿quien se esta llevando el ancho de banda?" y no sirve para
//     facturar. Por eso todo lo que sale de aqui va marcado como estimado hasta
//     la pantalla.
//  2. **Hay que leer paquetes crudos, no registros con campos.** El switch manda
//     los primeros bytes del paquete tal como pasaron por el cable, asi que aqui
//     se desarma Ethernet y luego IP a mano. No hay atajo.
//
// Del datagrama solo interesan las muestras de flujo. Las de contadores traen lo
// mismo que ya se lee por SNMP, y se ignoran.
const (
	// Formatos de muestra. Los "expandidos" son iguales pero con los
	// identificadores de interfaz en 64 bits, para chasis con muchos puertos.
	muestraDeFlujo          = 1
	muestraDeFlujoExpandida = 3

	// Tipos de registro dentro de una muestra de flujo.
	registroPaqueteCrudo = 1

	// Protocolos de cabecera que se saben desarmar.
	cabeceraEthernet = 1

	tamanoCabeceraEthernet = 14
	tipoIPv4               = 0x0800
	tipoIPv6               = 0x86DD
	tipoVLAN               = 0x8100
	tipoVLANApilada        = 0x88A8
)

func (r *Receptor) procesarSFlow(datos []byte) {
	lector := &lectorDeBytes{datos: datos}

	lector.saltar(4) // version, ya comprobada
	tipoDireccion, ok := lector.uint32()
	if !ok {
		return
	}
	// La direccion del agente mide 4 o 16 bytes segun lo que acaba de decir.
	// Saltarla con el tamano equivocado descoloca todo lo que sigue.
	switch tipoDireccion {
	case 1:
		lector.saltar(4)
	case 2:
		lector.saltar(16)
	default:
		return
	}
	lector.saltar(4) // subagente
	lector.saltar(4) // numero de secuencia
	lector.saltar(4) // tiempo encendido

	cuantas, ok := lector.uint32()
	if !ok || cuantas > 4096 {
		return
	}

	for i := uint32(0); i < cuantas; i++ {
		tipo, ok := lector.uint32()
		if !ok {
			return
		}
		largo, ok := lector.uint32()
		if !ok {
			return
		}
		cuerpo, ok := lector.tomar(int(largo))
		if !ok {
			return
		}

		// El tipo lleva el numero de empresa en los 20 bits altos. Solo interesan
		// los formatos estandar, que son los de empresa cero.
		if tipo>>12 != 0 {
			continue
		}
		switch tipo & 0xFFF {
		case muestraDeFlujo:
			r.leerMuestraDeFlujo(cuerpo, false)
		case muestraDeFlujoExpandida:
			r.leerMuestraDeFlujo(cuerpo, true)
		}
	}
}

// leerMuestraDeFlujo saca de una muestra la tasa de muestreo y los paquetes.
func (r *Receptor) leerMuestraDeFlujo(cuerpo []byte, expandida bool) {
	lector := &lectorDeBytes{datos: cuerpo}

	lector.saltar(4) // numero de secuencia
	if expandida {
		lector.saltar(8) // tipo e indice de la fuente, en dos palabras
	} else {
		lector.saltar(4) // fuente empaquetada en una
	}

	tasa, ok := lector.uint32()
	if !ok {
		return
	}
	// Una tasa de cero o absurda haria que un solo paquete se convirtiera en
	// gigabytes inventados. Ante la duda, se cuenta el paquete tal cual.
	if tasa == 0 || tasa > 1_000_000 {
		tasa = 1
	}

	lector.saltar(4) // total de paquetes de los que salio la muestra
	lector.saltar(4) // paquetes descartados
	if expandida {
		lector.saltar(16) // interfaces de entrada y salida, en dos palabras cada una
	} else {
		lector.saltar(8) // interfaces de entrada y salida
	}

	cuantos, ok := lector.uint32()
	if !ok || cuantos > 256 {
		return
	}

	for i := uint32(0); i < cuantos; i++ {
		tipo, ok := lector.uint32()
		if !ok {
			return
		}
		largo, ok := lector.uint32()
		if !ok {
			return
		}
		registro, ok := lector.tomar(int(largo))
		if !ok {
			return
		}
		if tipo>>12 == 0 && tipo&0xFFF == registroPaqueteCrudo {
			r.leerPaqueteCrudo(registro, uint64(tasa))
		}
	}
}

// leerPaqueteCrudo desarma la cabecera que el switch copio del cable.
func (r *Receptor) leerPaqueteCrudo(registro []byte, tasa uint64) {
	lector := &lectorDeBytes{datos: registro}

	protocolo, ok := lector.uint32()
	if !ok || protocolo != cabeceraEthernet {
		return
	}
	tamanoDeTrama, ok := lector.uint32()
	if !ok {
		return
	}
	lector.saltar(4) // bytes que el switch quito del final
	largoCabecera, ok := lector.uint32()
	if !ok {
		return
	}
	cabecera, ok := lector.tomar(int(largoCabecera))
	if !ok {
		return
	}

	origen, destino, ok := direccionesDe(cabecera)
	if !ok {
		return
	}

	// Aqui esta la estimacion: el tamano de la trama que se vio, por una de cada
	// cuantas se vio. Se usa el tamano de la TRAMA COMPLETA y no el de la
	// cabecera copiada, que suele venir cortada a 128 bytes.
	r.sumarFlujo(origen, destino, uint64(tamanoDeTrama)*tasa, true)
}

// direccionesDe saca el origen y el destino de una trama Ethernet.
//
// Se saltan las etiquetas de VLAN —una o dos— porque un switch con VLAN las
// manda tal cual, y sin saltarlas el tipo de protocolo se lee cuatro bytes antes
// de donde esta y el paquete parece basura.
func direccionesDe(trama []byte) (netip.Addr, netip.Addr, bool) {
	var vacia netip.Addr
	if len(trama) < tamanoCabeceraEthernet {
		return vacia, vacia, false
	}

	posicion := 12 // se saltan las dos MAC
	tipo := binary.BigEndian.Uint16(trama[posicion : posicion+2])
	posicion += 2

	for i := 0; i < 2 && (tipo == tipoVLAN || tipo == tipoVLANApilada); i++ {
		if posicion+4 > len(trama) {
			return vacia, vacia, false
		}
		tipo = binary.BigEndian.Uint16(trama[posicion+2 : posicion+4])
		posicion += 4
	}

	switch tipo {
	case tipoIPv4:
		if posicion+20 > len(trama) {
			return vacia, vacia, false
		}
		origen, _ := netip.AddrFromSlice(trama[posicion+12 : posicion+16])
		destino, _ := netip.AddrFromSlice(trama[posicion+16 : posicion+20])
		return origen, destino, origen.IsValid() && destino.IsValid()
	case tipoIPv6:
		if posicion+40 > len(trama) {
			return vacia, vacia, false
		}
		origen, _ := netip.AddrFromSlice(trama[posicion+8 : posicion+24])
		destino, _ := netip.AddrFromSlice(trama[posicion+24 : posicion+40])
		return origen, destino, origen.IsValid() && destino.IsValid()
	default:
		// ARP, spanning tree y demas: no son trafico de nadie en particular.
		return vacia, vacia, false
	}
}

// lectorDeBytes recorre un datagrama sin salirse nunca de su tamano.
//
// sFlow encadena largos que vienen del propio datagrama, asi que un paquete
// malformado —o hecho a mala fe— puede pedir leer mas de lo que hay. Concentrar
// el limite aqui evita tener que acordarse en cada campo.
type lectorDeBytes struct {
	datos    []byte
	posicion int
}

func (l *lectorDeBytes) saltar(cuantos int) {
	l.posicion += cuantos
}

func (l *lectorDeBytes) uint32() (uint32, bool) {
	if l.posicion < 0 || l.posicion+4 > len(l.datos) {
		return 0, false
	}
	valor := binary.BigEndian.Uint32(l.datos[l.posicion : l.posicion+4])
	l.posicion += 4
	return valor, true
}

// tomar devuelve los siguientes bytes, redondeando al multiplo de cuatro que
// exige el formato: sFlow rellena cada bloque para que quede alineado.
func (l *lectorDeBytes) tomar(cuantos int) ([]byte, bool) {
	if cuantos < 0 || l.posicion < 0 || l.posicion+cuantos > len(l.datos) {
		return nil, false
	}
	trozo := l.datos[l.posicion : l.posicion+cuantos]
	l.posicion += cuantos + relleno(cuantos)
	return trozo, true
}

func relleno(cuantos int) int {
	if resto := cuantos % 4; resto != 0 {
		return 4 - resto
	}
	return 0
}
