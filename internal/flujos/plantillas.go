package flujos

import (
	"encoding/binary"
	"net/netip"
	"sync"
)

// NetFlow v9 e IPFIX: los formatos con plantilla.
//
// A diferencia de v5, aqui el exportador no manda campos en posiciones fijas:
// **primero manda una plantilla** que dice "mis registros llevan estos campos,
// en este orden, con estos tamanos", y despues manda los registros en crudo. Un
// registro sin su plantilla es una tira de bytes que no significa nada.
//
// De ahi salen las dos cosas que hay que hacer bien y que no se parecen a v5:
//
//  1. **Guardar las plantillas por exportador.** Dos routers pueden usar el
//     mismo numero de plantilla para cosas distintas. Mezclarlas no da un error:
//     da cifras equivocadas, que es peor. Por eso la clave incluye de que
//     direccion llego.
//  2. **Aguantar los registros que llegan antes que su plantilla.** Es lo normal
//     al arrancar: el router manda plantillas cada pocos minutos y datos todo el
//     tiempo. Esos registros se descartan en silencio; cuando llegue la
//     plantilla, los siguientes ya se entienden.
//
// v9 e IPFIX se leen casi igual. Cambian el tamano del sobre, los numeros que
// identifican un conjunto de plantillas, y que IPFIX admite campos de largo
// variable y de fabricante. Por eso comparten codigo en vez de duplicarse.

// Numeros de campo de IANA. Son los mismos en v9 y en IPFIX para lo basico, que
// es justo lo que hace falta aqui.
const (
	campoOctetos       = 1
	campoOrigenIPv4    = 8
	campoDestinoIPv4   = 12
	campoOrigenIPv6    = 27
	campoDestinoIPv6   = 28
	campoOctetosTotal  = 85
	campoOctetosSalida = 23 // postOctetDeltaCount, el que usan algunos routers

	// Conjuntos especiales.
	conjuntoPlantillasV9      = 0
	conjuntoOpcionesV9        = 1
	conjuntoPlantillasIPFIX   = 2
	conjuntoOpcionesIPFIX     = 3
	primerConjuntoDeDatos     = 256
	largoVariableIPFIX        = 0xFFFF
	tamanoCabeceraV9          = 20
	tamanoCabeceraIPFIX       = 16
	tamanoCabeceraDeConjunto  = 4
	tamanoCabeceraDePlantilla = 4
)

// campoDePlantilla es un campo declarado por el exportador.
type campoDePlantilla struct {
	tipo  uint16
	largo uint16
}

// almacenDePlantillas guarda lo que cada exportador dijo de sus registros.
type almacenDePlantillas struct {
	mu    sync.RWMutex
	datos map[clavePlantilla][]campoDePlantilla
}

type clavePlantilla struct {
	exportador string
	dominio    uint32
	id         uint16
}

func (a *almacenDePlantillas) guardar(clave clavePlantilla, campos []campoDePlantilla) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.datos == nil {
		a.datos = map[clavePlantilla][]campoDePlantilla{}
	}
	a.datos[clave] = campos
}

func (a *almacenDePlantillas) buscar(clave clavePlantilla) ([]campoDePlantilla, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	campos, hay := a.datos[clave]
	return campos, hay
}

func (r *Receptor) procesarConPlantilla(exportador string, datos []byte, version uint16) {
	cabecera := tamanoCabeceraV9
	if version == versionIPFIX {
		cabecera = tamanoCabeceraIPFIX
	}
	if len(datos) < cabecera {
		return
	}

	// IPFIX declara el largo total del mensaje; v9 no, y ahi manda el datagrama.
	fin := len(datos)
	if version == versionIPFIX {
		declarado := int(binary.BigEndian.Uint16(datos[2:4]))
		if declarado >= cabecera && declarado < fin {
			fin = declarado
		}
	}

	// El identificador del dominio de observacion va al final de la cabecera en
	// los dos formatos.
	dominio := binary.BigEndian.Uint32(datos[cabecera-4 : cabecera])

	posicion := cabecera
	for posicion+tamanoCabeceraDeConjunto <= fin {
		conjunto := binary.BigEndian.Uint16(datos[posicion : posicion+2])
		largo := int(binary.BigEndian.Uint16(datos[posicion+2 : posicion+4]))

		// Un largo imposible significa datagrama corrupto o mal armado. Se corta
		// aqui: seguir leyendo con un largo malo es como se entra en un bucle.
		if largo < tamanoCabeceraDeConjunto || posicion+largo > fin {
			return
		}
		cuerpo := datos[posicion+tamanoCabeceraDeConjunto : posicion+largo]

		switch {
		case esConjuntoDePlantillas(conjunto, version):
			r.leerPlantillas(cuerpo, clavePlantilla{
				exportador: exportador, dominio: dominio,
			}, version)
		case esConjuntoDeOpciones(conjunto, version):
			// Las plantillas de opciones describen metadatos del exportador
			// (tasas de muestreo, nombres de interfaz), no flujos. No estorban.
		case conjunto >= primerConjuntoDeDatos:
			r.leerRegistros(cuerpo, clavePlantilla{
				exportador: exportador, dominio: dominio, id: conjunto,
			})
		}

		posicion += largo
	}
}

func esConjuntoDePlantillas(conjunto uint16, version uint16) bool {
	if version == versionIPFIX {
		return conjunto == conjuntoPlantillasIPFIX
	}
	return conjunto == conjuntoPlantillasV9
}

func esConjuntoDeOpciones(conjunto uint16, version uint16) bool {
	if version == versionIPFIX {
		return conjunto == conjuntoOpcionesIPFIX
	}
	return conjunto == conjuntoOpcionesV9
}

// leerPlantillas apunta que campos declaro el exportador.
func (r *Receptor) leerPlantillas(cuerpo []byte, base clavePlantilla, version uint16) {
	posicion := 0
	for posicion+tamanoCabeceraDePlantilla <= len(cuerpo) {
		id := binary.BigEndian.Uint16(cuerpo[posicion : posicion+2])
		cuantos := int(binary.BigEndian.Uint16(cuerpo[posicion+2 : posicion+4]))
		posicion += tamanoCabeceraDePlantilla

		campos := make([]campoDePlantilla, 0, cuantos)
		for i := 0; i < cuantos; i++ {
			if posicion+4 > len(cuerpo) {
				return
			}
			tipo := binary.BigEndian.Uint16(cuerpo[posicion : posicion+2])
			largo := binary.BigEndian.Uint16(cuerpo[posicion+2 : posicion+4])
			posicion += 4

			// IPFIX marca con el bit alto los campos de fabricante, que traen
			// cuatro bytes mas con el numero de empresa. Aqui no interesan, pero
			// **hay que saltarlos o todo lo que sigue queda corrido**.
			if version == versionIPFIX && tipo&0x8000 != 0 {
				if posicion+4 > len(cuerpo) {
					return
				}
				posicion += 4
				tipo = 0 // sin numero de empresa no se puede interpretar
			}
			campos = append(campos, campoDePlantilla{tipo: tipo, largo: largo})
		}

		clave := base
		clave.id = id
		r.plantillas.guardar(clave, campos)
	}
}

// leerRegistros interpreta los datos con la plantilla que les corresponde.
func (r *Receptor) leerRegistros(cuerpo []byte, clave clavePlantilla) {
	campos, hay := r.plantillas.buscar(clave)
	if !hay || len(campos) == 0 {
		// Todavia no llego su plantilla. Es lo normal al arrancar.
		return
	}

	posicion := 0
	for posicion < len(cuerpo) {
		var origen, destino netip.Addr
		var octetos uint64
		avanzo := false

		for _, campo := range campos {
			largo := int(campo.largo)

			// Campo de largo variable de IPFIX: el tamano real va delante. Un
			// byte, o 255 seguido de dos bytes cuando no cabe.
			if campo.largo == largoVariableIPFIX {
				if posicion >= len(cuerpo) {
					return
				}
				largo = int(cuerpo[posicion])
				posicion++
				if largo == 255 {
					if posicion+2 > len(cuerpo) {
						return
					}
					largo = int(binary.BigEndian.Uint16(cuerpo[posicion : posicion+2]))
					posicion += 2
				}
			}

			if largo <= 0 || posicion+largo > len(cuerpo) {
				return
			}
			valor := cuerpo[posicion : posicion+largo]
			posicion += largo
			avanzo = true

			switch campo.tipo {
			case campoOrigenIPv4, campoOrigenIPv6:
				if direccion, ok := netip.AddrFromSlice(valor); ok {
					origen = direccion
				}
			case campoDestinoIPv4, campoDestinoIPv6:
				if direccion, ok := netip.AddrFromSlice(valor); ok {
					destino = direccion
				}
			case campoOctetos, campoOctetosTotal, campoOctetosSalida:
				octetos += comoEnteroGrande(valor)
			}
		}

		// Una plantilla de puros campos de largo cero dejaria la posicion quieta
		// y el bucle daria vueltas para siempre.
		if !avanzo {
			return
		}
		if origen.IsValid() && destino.IsValid() {
			r.sumarFlujo(origen, destino, octetos, false)
		}
	}
}

// comoEnteroGrande lee un contador del tamano que el exportador haya elegido.
//
// El mismo campo llega de cuatro bytes en un router y de ocho en otro, y de dos
// en algun equipo empotrado. Suponer un tamano es como se leen cifras absurdas.
func comoEnteroGrande(valor []byte) uint64 {
	var numero uint64
	for _, byte := range valor {
		numero = numero<<8 | uint64(byte)
	}
	return numero
}
