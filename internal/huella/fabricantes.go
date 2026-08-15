package huella

import (
	"context"
	"encoding/binary"
	"net"
	"strconv"
	"strings"
	"time"
)

// Los protocolos propios de los fabricantes que mas se venden aqui.
//
// Cada marca tiene su forma de contestar "quien soy" en un puerto UDP suyo. Es
// la respuesta mas exacta que existe —modelo, firmware, nombre y hasta el SSID,
// dichos por el propio aparato— y no hace falta ninguna credencial: son los
// mismos mensajes que usan las aplicaciones del fabricante para encontrar sus
// equipos en la red.

// preguntarUDP manda una carga y espera una respuesta. Un aparato que no habla
// ese idioma simplemente no contesta, y eso no es un error.
func preguntarUDP(ctx context.Context, ip string, puerto int, carga []byte, espera time.Duration) []byte {
	marcador := net.Dialer{Timeout: espera}
	conexion, err := marcador.DialContext(ctx, "udp", net.JoinHostPort(ip, strconv.Itoa(puerto)))
	if err != nil {
		return nil
	}
	defer conexion.Close()

	if err := conexion.SetDeadline(time.Now().Add(espera)); err != nil {
		return nil
	}
	if _, err := conexion.Write(carga); err != nil {
		return nil
	}

	buzon := make([]byte, 4096)
	leidos, err := conexion.Read(buzon)
	if err != nil || leidos == 0 {
		return nil
	}
	return buzon[:leidos]
}

// ------------------------------------------------------------- Ubiquiti --

// puertoUbiquiti es donde escuchan los UniFi, los AirMax y los AirCube.
const puertoUbiquiti = 10001

// DeUbiquiti pregunta por el protocolo propio de Ubiquiti.
//
// Contesta modelo, firmware, nombre del aparato, sus MAC y —si es una antena—
// **el SSID de la red que emite**. Es la unica forma de saber la MAC del puerto
// inalambrico sin tener una tarjeta WiFi escuchando el aire.
func DeUbiquiti(ctx context.Context, ip string, espera time.Duration) []Dato {
	return leerUbiquiti(preguntarUDP(ctx, ip, puertoUbiquiti, []byte{0x01, 0x00, 0x00, 0x00}, espera))
}

// leerUbiquiti desarma la respuesta: cabecera, y despues tipo, largo y valor.
func leerUbiquiti(respuesta []byte) []Dato {
	if len(respuesta) < 4 {
		return nil
	}
	// Cabecera: version, comando y el largo de lo que sigue.
	if respuesta[0] != 0x01 && respuesta[0] != 0x02 {
		return nil
	}

	var huella Huella
	cuerpo := respuesta[4:]
	for len(cuerpo) >= 3 {
		tipo := cuerpo[0]
		largo := int(binary.BigEndian.Uint16(cuerpo[1:3]))
		if largo < 0 || 3+largo > len(cuerpo) {
			break
		}
		valor := cuerpo[3 : 3+largo]
		cuerpo = cuerpo[3+largo:]

		switch tipo {
		case 0x01: // MAC a secas
			if len(valor) == 6 {
				huella.Agregar(FuenteUbiquiti, ClaveMAC, comoMAC(valor))
			}
		case 0x02: // MAC + IP de una tarjeta
			if len(valor) == 10 {
				huella.Agregar(FuenteUbiquiti, ClaveMAC, comoMAC(valor[:6]))
			}
		case 0x03:
			huella.Agregar(FuenteUbiquiti, ClaveFirmware, string(valor))
		case 0x0b:
			huella.Agregar(FuenteUbiquiti, ClaveNombre, string(valor))
		case 0x0c, 0x14, 0x15:
			huella.Agregar(FuenteUbiquiti, ClaveModelo, string(valor))
		case 0x0d: // el SSID que emite
			huella.Agregar(FuenteUbiquiti, ClaveRed, string(valor))
		}
	}

	if len(huella.Datos) > 0 {
		huella.Agregar(FuenteUbiquiti, ClaveFabricante, "Ubiquiti")
	}
	return huella.Datos
}

// -------------------------------------------------------------- MikroTik --

const puertoMikrotik = 5678

// DeMikrotik pregunta por MNDP, el protocolo de vecinos de MikroTik.
//
// Devuelve la identidad que le pusieron al router, la version de RouterOS y la
// placa exacta ("hEX S", "hAP ac2"), que es justo lo que hace falta para no
// confundir un router de la marca con un switch de la marca.
func DeMikrotik(ctx context.Context, ip string, espera time.Duration) []Dato {
	// Un paquete vacio basta para que conteste quien habla MNDP.
	return leerMikrotik(preguntarUDP(ctx, ip, puertoMikrotik, []byte{0x00, 0x00, 0x00, 0x00}, espera))
}

// leerMikrotik desarma los campos de MNDP: tipo, largo y valor.
func leerMikrotik(respuesta []byte) []Dato {
	if len(respuesta) < 8 {
		return nil
	}

	var huella Huella
	cuerpo := respuesta[4:] // se salta la cabecera de la version
	for len(cuerpo) >= 4 {
		tipo := binary.BigEndian.Uint16(cuerpo[0:2])
		largo := int(binary.BigEndian.Uint16(cuerpo[2:4]))
		if largo < 0 || 4+largo > len(cuerpo) {
			break
		}
		valor := cuerpo[4 : 4+largo]
		cuerpo = cuerpo[4+largo:]

		switch tipo {
		case 1:
			if len(valor) == 6 {
				huella.Agregar(FuenteMikrotik, ClaveMAC, comoMAC(valor))
			}
		case 5:
			huella.Agregar(FuenteMikrotik, ClaveNombre, string(valor))
		case 7:
			huella.Agregar(FuenteMikrotik, ClaveFirmware, string(valor))
		case 8:
			huella.Agregar(FuenteMikrotik, ClaveFabricante, string(valor))
		case 12:
			huella.Agregar(FuenteMikrotik, ClaveModelo, string(valor))
		}
	}
	return huella.Datos
}

// ---------------------------------------------------------------- TP-Link --

const puertoTPLink = 9999

// DeTPLink pregunta a los enchufes, focos y camaras Kasa de TP-Link.
//
// Hablan JSON tapado con un XOR encadenado de clave fija: no es cifrado, es una
// formalidad del protocolo, y todo el mundo la implementa igual. Contestan
// modelo, alias que le puso el dueno, MAC y version de firmware.
func DeTPLink(ctx context.Context, ip string, espera time.Duration) []Dato {
	peticion := taparTPLink([]byte(`{"system":{"get_sysinfo":{}}}`))
	return leerTPLink(preguntarUDP(ctx, ip, puertoTPLink, peticion, espera))
}

// leerTPLink destapa la respuesta y saca los cuatro campos que importan.
func leerTPLink(respuesta []byte) []Dato {
	if len(respuesta) == 0 {
		return nil
	}

	texto := string(destaparTPLink(respuesta))
	if !strings.Contains(texto, "sysinfo") && !strings.Contains(texto, "model") {
		return nil
	}

	var huella Huella
	huella.Agregar(FuenteTPLink, ClaveFabricante, "TP-Link")
	huella.Agregar(FuenteTPLink, ClaveModelo, entreComillas(texto, `"model"`))
	huella.Agregar(FuenteTPLink, ClaveNombre, entreComillas(texto, `"alias"`))
	huella.Agregar(FuenteTPLink, ClaveFirmware, entreComillas(texto, `"sw_ver"`))
	huella.Agregar(FuenteTPLink, ClaveMAC, entreComillas(texto, `"mac"`))
	return huella.Datos
}

// taparTPLink aplica el XOR encadenado con la clave 171 que usa el fabricante.
func taparTPLink(claro []byte) []byte {
	tapado := make([]byte, len(claro))
	llave := byte(171)
	for i, letra := range claro {
		llave = letra ^ llave
		tapado[i] = llave
	}
	return tapado
}

func destaparTPLink(tapado []byte) []byte {
	claro := make([]byte, len(tapado))
	llave := byte(171)
	for i, letra := range tapado {
		claro[i] = letra ^ llave
		llave = letra
	}
	return claro
}

// --------------------------------------------------------------- NetBIOS --

const puertoNetBIOS = 137

// DeNetBIOS pregunta el nombre de una maquina Windows.
//
// Es un protocolo viejo y sigue siendo la forma mas directa de ponerle nombre a
// una PC de oficina que no aparece en el DNS de nadie.
func DeNetBIOS(ctx context.Context, ip string, espera time.Duration) []Dato {
	return leerNetBIOS(preguntarUDP(ctx, ip, puertoNetBIOS, peticionNetBIOS(), espera))
}

// leerNetBIOS saca los nombres de la respuesta de estado de nodo.
func leerNetBIOS(respuesta []byte) []Dato {
	// Cabecera de 12 bytes, la pregunta repetida (34 bytes) y luego los nombres.
	if len(respuesta) < 57 {
		return nil
	}

	cuantos := int(respuesta[56])
	cuerpo := respuesta[57:]
	var huella Huella

	for i := 0; i < cuantos && len(cuerpo) >= 18; i++ {
		nombre := strings.TrimSpace(string(cuerpo[:15]))
		tipo := cuerpo[15]
		banderas := binary.BigEndian.Uint16(cuerpo[16:18])
		cuerpo = cuerpo[18:]

		// El bit alto marca los nombres de GRUPO: ese es el dominio o el grupo
		// de trabajo, no el nombre de la maquina.
		esGrupo := banderas&0x8000 != 0
		switch {
		case tipo == 0x00 && !esGrupo:
			huella.Agregar(FuenteNetBIOS, ClaveNombre, nombre)
		case tipo == 0x00 && esGrupo:
			huella.Agregar(FuenteNetBIOS, ClaveServicio, "grupo de trabajo "+nombre)
		}
	}
	return huella.Datos
}

// peticionNetBIOS arma la consulta de estado de nodo: "digame todos sus
// nombres". El nombre preguntado es el comodin "*".
func peticionNetBIOS() []byte {
	peticion := []byte{
		0x82, 0x28, // identificador cualquiera
		0x00, 0x00, // sin banderas: es una pregunta
		0x00, 0x01, // una pregunta
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x20, // el nombre codificado mide 32 letras
	}
	// El comodin "*" relleno con ceros, codificado como pide el protocolo:
	// cada byte se parte en dos y se le suma la letra 'A'.
	nombre := make([]byte, 16)
	nombre[0] = '*'
	for _, letra := range nombre {
		peticion = append(peticion, 'A'+(letra>>4), 'A'+(letra&0x0f))
	}
	peticion = append(peticion,
		0x00,       // fin del nombre
		0x00, 0x21, // tipo NBSTAT
		0x00, 0x01, // clase INTERNET
	)
	return peticion
}

// ------------------------------------------------------------- auxiliares --

func comoMAC(crudo []byte) string {
	partes := make([]string, len(crudo))
	for i, letra := range crudo {
		partes[i] = strings.ToLower(hexDe(letra))
	}
	return strings.Join(partes, ":")
}

func hexDe(letra byte) string {
	const digitos = "0123456789abcdef"
	return string([]byte{digitos[letra>>4], digitos[letra&0x0f]})
}

// entreComillas saca el valor de "clave":"valor" sin armar un JSON completo: lo
// que contestan estos aparatos no siempre es JSON valido, y una respuesta rara
// no puede tumbar el reconocimiento.
func entreComillas(texto, clave string) string {
	inicio := strings.Index(texto, clave)
	if inicio < 0 {
		return ""
	}
	resto := texto[inicio+len(clave):]
	abre := strings.Index(resto, `"`)
	if abre < 0 {
		return ""
	}
	resto = resto[abre+1:]
	cierra := strings.Index(resto, `"`)
	if cierra < 0 {
		return ""
	}
	return resto[:cierra]
}
