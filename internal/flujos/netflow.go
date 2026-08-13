package flujos

import (
	"encoding/binary"
	"net/netip"
)

// NetFlow v5: el formato viejo, de tamano fijo, y el que exportan practicamente
// todos los routers de casa y de oficina (MikroTik y pfSense incluidos).
//
// Al ser de tamano fijo no hace falta ninguna negociacion: cada registro mide
// siempre 48 bytes y cada campo esta siempre en el mismo lugar. Es la razon por
// la que sigue vivo treinta anos despues, y por la que fue el primero que se
// implemento aqui.
const (
	tamanoCabecera = 24
	tamanoRegistro = 48
	maximoV5       = 30 // registros que caben en un datagrama

	// Posiciones dentro del registro de 48 bytes.
	posOrigenV5  = 0
	posDestinoV5 = 4
	posOctetosV5 = 20
)

func (r *Receptor) procesarNetFlow5(datos []byte) {
	if len(datos) < tamanoCabecera {
		return
	}

	cuantos := int(binary.BigEndian.Uint16(datos[2:4]))
	if cuantos > maximoV5 {
		cuantos = maximoV5
	}

	for i := 0; i < cuantos; i++ {
		inicio := tamanoCabecera + i*tamanoRegistro
		if inicio+tamanoRegistro > len(datos) {
			return
		}
		registro := datos[inicio : inicio+tamanoRegistro]

		origen, ok := netip.AddrFromSlice(registro[posOrigenV5 : posOrigenV5+4])
		if !ok {
			continue
		}
		destino, ok := netip.AddrFromSlice(registro[posDestinoV5 : posDestinoV5+4])
		if !ok {
			continue
		}
		octetos := uint64(binary.BigEndian.Uint32(registro[posOctetosV5 : posOctetosV5+4]))

		// v5 cuenta de verdad, no muestrea: el ultimo argumento es falso.
		r.sumarFlujo(origen, destino, octetos, false)
	}
}
