package huella

import (
	"encoding/binary"
	"strings"
)

// mDNS es como se presentan en la red los aparatos de Apple, los Chromecast, las
// impresoras modernas y media casa inteligente. Publican su nombre de pila —el
// que le puso el dueno, "TV de la sala"— y a que juegan: _googlecast, _airplay,
// _ipp, _printer, _homekit.
//
// Aqui va un lector minimo del formato DNS. Se escribe a mano por la misma razon
// que el resto del proyecto: meter una dependencia de 8 000 lineas para leer
// cuatro campos seria pagar mantenimiento ajeno para siempre.

// serviciosBuscados son los tipos que se preguntan. Salen de lo que de verdad
// aparece en una casa o una oficina chica.
var serviciosBuscados = []string{
	"_services._dns-sd._udp.local.", // el catalogo: "digame que servicios tiene"
	"_googlecast._tcp.local.",
	"_airplay._tcp.local.",
	"_raop._tcp.local.", // AirPlay de audio
	"_ipp._tcp.local.",
	"_printer._tcp.local.",
	"_pdl-datastream._tcp.local.",
	"_workstation._tcp.local.",
	"_smb._tcp.local.",
	"_homekit._tcp.local.",
	"_hap._tcp.local.",
	"_device-info._tcp.local.",
}

// peticionMDNS arma una consulta con todos los servicios de una vez.
//
// Se pide respuesta UNICAST (el bit alto de la clase): asi las respuestas llegan
// al puerto desde el que se pregunto, sin tener que unirse al grupo multicast ni
// competir con el demonio de mDNS que ya corra en el equipo.
func peticionMDNS() []byte {
	mensaje := make([]byte, 12)
	binary.BigEndian.PutUint16(mensaje[0:2], 0)                              // sin identificador
	binary.BigEndian.PutUint16(mensaje[4:6], uint16(len(serviciosBuscados))) // preguntas

	for _, servicio := range serviciosBuscados {
		mensaje = append(mensaje, comoNombreDNS(servicio)...)
		mensaje = append(mensaje, 0x00, 0x0c) // PTR
		mensaje = append(mensaje, 0x80, 0x01) // clase IN, con respuesta unicast
	}
	return mensaje
}

func comoNombreDNS(nombre string) []byte {
	var crudo []byte
	for _, tramo := range strings.Split(strings.TrimSuffix(nombre, "."), ".") {
		if tramo == "" {
			continue
		}
		if len(tramo) > 63 {
			tramo = tramo[:63]
		}
		crudo = append(crudo, byte(len(tramo)))
		crudo = append(crudo, tramo...)
	}
	return append(crudo, 0x00)
}

// leerMDNS saca de la respuesta el nombre del aparato y a que juega.
func leerMDNS(huella *Huella, mensaje []byte) {
	if len(mensaje) < 12 {
		return
	}
	preguntas := int(binary.BigEndian.Uint16(mensaje[4:6]))
	respuestas := int(binary.BigEndian.Uint16(mensaje[6:8])) +
		int(binary.BigEndian.Uint16(mensaje[8:10])) +
		int(binary.BigEndian.Uint16(mensaje[10:12]))

	posicion := 12
	for i := 0; i < preguntas && posicion < len(mensaje); i++ {
		_, siguiente := leerNombre(mensaje, posicion)
		posicion = siguiente + 4 // tipo y clase
	}

	for i := 0; i < respuestas && posicion+10 <= len(mensaje); i++ {
		nombre, siguiente := leerNombre(mensaje, posicion)
		if siguiente+10 > len(mensaje) {
			return
		}
		tipo := binary.BigEndian.Uint16(mensaje[siguiente : siguiente+2])
		largo := int(binary.BigEndian.Uint16(mensaje[siguiente+8 : siguiente+10]))
		inicioDatos := siguiente + 10
		if inicioDatos+largo > len(mensaje) {
			return
		}
		datos := mensaje[inicioDatos : inicioDatos+largo]
		posicion = inicioDatos + largo

		switch tipo {
		case 12: // PTR: apunta a la instancia concreta del servicio
			apuntado, _ := leerNombre(mensaje, inicioDatos)
			anotarServicioMDNS(huella, apuntado, nombre)
		case 16: // TXT: pares clave=valor, donde muchos ponen el modelo
			anotarTextoMDNS(huella, datos)
		case 33: // SRV: el nombre de la maquina que da el servicio
			if largo > 6 {
				maquina, _ := leerNombre(mensaje, inicioDatos+6)
				huella.Agregar(FuenteMDNS, ClaveNombre, sinLocal(maquina))
			}
		}
	}
}

// anotarServicioMDNS separa el nombre de pila del aparato del tipo de servicio.
//
// Una instancia se llama "TV de la sala._googlecast._tcp.local": lo de antes del
// primer punto es como le dice el dueno, y el resto es a que juega.
func anotarServicioMDNS(huella *Huella, instancia, tipo string) {
	instancia = sinLocal(instancia)
	if instancia == "" {
		return
	}
	if corte := strings.Index(instancia, "._"); corte > 0 {
		huella.Agregar(FuenteMDNS, ClaveNombre, descaparEscapes(instancia[:corte]))
		huella.Agregar(FuenteMDNS, ClaveServicio, instancia[corte+1:])
		return
	}
	// La respuesta al catalogo de servicios: no trae nombre, solo el tipo.
	if strings.HasPrefix(instancia, "_") {
		huella.Agregar(FuenteMDNS, ClaveServicio, instancia)
		return
	}
	huella.Agregar(FuenteMDNS, ClaveServicio, sinLocal(tipo))
}

// anotarTextoMDNS lee los pares del registro TXT. Ahi es donde Apple pone el
// modelo exacto ("md=Chromecast Ultra", "model=MacBookPro18,3").
func anotarTextoMDNS(huella *Huella, datos []byte) {
	for len(datos) > 0 {
		largo := int(datos[0])
		if largo == 0 || largo+1 > len(datos) {
			return
		}
		par := string(datos[1 : 1+largo])
		datos = datos[1+largo:]

		partes := strings.SplitN(par, "=", 2)
		if len(partes) != 2 || partes[1] == "" {
			continue
		}
		switch strings.ToLower(partes[0]) {
		case "md", "model", "ty", "usb_mdl", "product":
			huella.Agregar(FuenteMDNS, ClaveModelo, partes[1])
		case "fn", "n":
			huella.Agregar(FuenteMDNS, ClaveNombre, partes[1])
		case "usb_mfg", "mfg", "manufacturer":
			huella.Agregar(FuenteMDNS, ClaveFabricante, partes[1])
		case "fv", "vs", "ve":
			huella.Agregar(FuenteMDNS, ClaveFirmware, partes[1])
		case "id", "sn", "serialnumber":
			huella.Agregar(FuenteMDNS, ClaveSerie, partes[1])
		}
	}
}

// leerNombre desarma un nombre DNS, siguiendo los punteros de compresion.
//
// La compresion es lo que obliga a escribir esto: un nombre puede terminar en un
// puntero a otro pedazo del mensaje, y sin seguirlo se leen nombres cortados.
func leerNombre(mensaje []byte, posicion int) (string, int) {
	var tramos []string
	siguiente := -1
	saltos := 0

	for posicion < len(mensaje) {
		largo := int(mensaje[posicion])

		if largo == 0 {
			posicion++
			break
		}
		if largo&0xc0 == 0xc0 { // puntero
			if posicion+1 >= len(mensaje) || saltos > 16 {
				break
			}
			destino := int(binary.BigEndian.Uint16(mensaje[posicion:posicion+2]) & 0x3fff)
			if siguiente < 0 {
				siguiente = posicion + 2
			}
			posicion = destino
			saltos++
			continue
		}
		if posicion+1+largo > len(mensaje) {
			break
		}
		tramos = append(tramos, string(mensaje[posicion+1:posicion+1+largo]))
		posicion += 1 + largo
	}

	if siguiente < 0 {
		siguiente = posicion
	}
	return strings.Join(tramos, "."), siguiente
}

func sinLocal(nombre string) string {
	return strings.TrimSuffix(strings.TrimSuffix(nombre, "."), ".local")
}

// descaparEscapes deshace lo que el formato DNS escapa: los espacios de un
// nombre puesto por una persona vienen como "\032".
func descaparEscapes(texto string) string {
	texto = strings.ReplaceAll(texto, `\032`, " ")
	texto = strings.ReplaceAll(texto, `\.`, ".")
	return texto
}
