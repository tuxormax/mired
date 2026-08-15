// Paquete dpi identifica QUE aplicacion consume el ancho de banda, no solo
// cuanto.
//
// Es la fase 10 del plan y va en un paquete aparte (`mired-dpi`) a proposito:
// mirar todos los paquetes cuesta mucho mas que preguntarle a un switch cuantos
// bytes paso por un puerto, y esa es exactamente la diferencia entre MiRed y las
// herramientas que acaban en "consumo alto". Quien no lo instale no paga nada
// por ello.
//
// # Como se identifica una aplicacion sin descifrar nada
//
// Hoy casi todo el trafico va cifrado, asi que **no se mira el contenido: se
// mira el sobre**. Tres cosas viajan en claro incluso dentro de una conexion
// cifrada, y las tres dicen con quien se esta hablando:
//
//	SNI de TLS   el nombre del servidor va en claro en el saludo inicial
//	Host de HTTP para lo poco que sigue sin cifrar
//	Consultas DNS el equipo pregunta el nombre antes de conectarse
//
// Eso da "netflix.com", "backup.empresa.mx" o "windowsupdate.com", que es lo que
// de verdad se quiere saber. **MiRed no descifra nada ni lo intenta**: no hay
// intercepcion, no hay certificados propios, no se toca el contenido.
package dpi

import (
	"encoding/binary"
	"strings"
)

// Lo que se saca de un paquete, si es que dice algo.
type Pista struct {
	// Nombre es el servidor con el que se esta hablando: "netflix.com".
	Nombre string
	// Como dice de donde salio: "tls", "http", "dns" o "puerto".
	Como string
}

// IdentificarTCP mira la carga util de un paquete TCP a ver si se delata.
//
// Solo sirve el PRIMER paquete de una conexion, que es donde va el saludo. Los
// siguientes son datos cifrados y no dicen nada; por eso esto es barato aunque
// pase por todos los paquetes: casi todos se descartan en las dos primeras
// comprobaciones.
func IdentificarTCP(carga []byte, puertoDestino int) (Pista, bool) {
	if nombre, ok := nombreDeTLS(carga); ok {
		return Pista{Nombre: nombre, Como: "tls"}, true
	}
	if nombre, ok := nombreDeHTTP(carga); ok {
		return Pista{Nombre: nombre, Como: "http"}, true
	}
	if servicio, ok := porPuerto[puertoDestino]; ok {
		// Ultimo recurso: el puerto. Dice el tipo de servicio, no con quien se
		// habla, y por eso solo se usa cuando no hubo nada mejor.
		return Pista{Nombre: servicio, Como: "puerto"}, true
	}
	return Pista{}, false
}

// IdentificarUDP saca de una consulta DNS el nombre que el equipo pregunto.
//
// Vale mas de lo que parece: un equipo pregunta el nombre ANTES de conectarse, y
// esa consulta va en claro aunque toda la conexion posterior vaya cifrada y
// aunque el servidor use QUIC, que no tiene saludo legible.
func IdentificarUDP(carga []byte, puertoDestino int) (Pista, bool) {
	if puertoDestino == 53 {
		if nombre, ok := nombreDeConsultaDNS(carga); ok {
			return Pista{Nombre: nombre, Como: "dns"}, true
		}
		return Pista{}, false
	}
	if servicio, ok := porPuerto[puertoDestino]; ok {
		return Pista{Nombre: servicio, Como: "puerto"}, true
	}
	return Pista{}, false
}

// nombreDeTLS saca el SNI del saludo de TLS.
//
// El saludo de TLS es la unica parte de una conexion cifrada que va en claro, y
// dentro lleva el nombre del servidor porque el cliente TIENE que decirlo para
// que le manden el certificado correcto. Recorrerlo es contar longitudes: cada
// bloque dice cuanto mide el siguiente.
func nombreDeTLS(carga []byte) (string, bool) {
	// Registro TLS: tipo 0x16 (handshake), version, longitud.
	if len(carga) < 43 || carga[0] != 0x16 {
		return "", false
	}
	// Mensaje de handshake: tipo 0x01 (ClientHello).
	if carga[5] != 0x01 {
		return "", false
	}

	lector := &lector{datos: carga, posicion: 43} // salta cabeceras, version y aleatorio

	// Identificador de sesion.
	largoSesion, ok := lector.byte()
	if !ok {
		return "", false
	}
	lector.saltar(int(largoSesion))

	// Suites de cifrado.
	largoSuites, ok := lector.uint16()
	if !ok {
		return "", false
	}
	lector.saltar(int(largoSuites))

	// Metodos de compresion.
	largoCompresion, ok := lector.byte()
	if !ok {
		return "", false
	}
	lector.saltar(int(largoCompresion))

	// Extensiones.
	if _, ok := lector.uint16(); !ok {
		return "", false
	}
	for {
		tipo, ok := lector.uint16()
		if !ok {
			return "", false
		}
		largo, ok := lector.uint16()
		if !ok {
			return "", false
		}
		cuerpo, ok := lector.tomar(int(largo))
		if !ok {
			return "", false
		}
		if tipo != 0 { // 0 = server_name
			continue
		}

		// Dentro: lista de nombres; de cada uno, tipo y longitud.
		if len(cuerpo) < 5 {
			return "", false
		}
		if cuerpo[2] != 0 { // tipo 0 = nombre de maquina
			return "", false
		}
		largoNombre := int(binary.BigEndian.Uint16(cuerpo[3:5]))
		if 5+largoNombre > len(cuerpo) {
			return "", false
		}
		return limpiarNombre(string(cuerpo[5 : 5+largoNombre])), true
	}
}

// nombreDeHTTP saca la cabecera Host de una peticion.
func nombreDeHTTP(carga []byte) (string, bool) {
	if len(carga) < 16 {
		return "", false
	}
	// Solo los metodos comunes: comprobarlo antes evita recorrer como texto los
	// megabytes de un paquete cifrado que casualmente empiece parecido.
	texto := string(carga[:min(len(carga), 8)])
	if !empiezaPorMetodo(texto) {
		return "", false
	}

	completo := string(carga[:min(len(carga), 2048)])
	for _, renglon := range strings.Split(completo, "\r\n") {
		if len(renglon) > 5 && strings.EqualFold(renglon[:5], "host:") {
			maquina := strings.TrimSpace(renglon[5:])
			// El puerto no es parte del nombre: "sitio.com:8080" y "sitio.com"
			// son el mismo destino y separarlos partiria el consumo en dos.
			if corte := strings.LastIndex(maquina, ":"); corte > 0 {
				maquina = maquina[:corte]
			}
			return limpiarNombre(maquina), maquina != ""
		}
	}
	return "", false
}

func empiezaPorMetodo(texto string) bool {
	for _, metodo := range []string{"GET ", "POST ", "PUT ", "HEAD ", "DELETE ", "OPTIONS ", "PATCH "} {
		if strings.HasPrefix(texto, metodo) {
			return true
		}
	}
	return false
}

// nombreDeConsultaDNS saca el nombre preguntado de un mensaje DNS.
//
// Solo se miran las CONSULTAS (bit de respuesta apagado): la respuesta lleva lo
// mismo y contarla duplicaria todo.
func nombreDeConsultaDNS(carga []byte) (string, bool) {
	if len(carga) < 13 {
		return "", false
	}
	if carga[2]&0x80 != 0 {
		return "", false // es una respuesta
	}
	if binary.BigEndian.Uint16(carga[4:6]) == 0 {
		return "", false // no pregunta nada
	}

	var partes []string
	posicion := 12
	for posicion < len(carga) {
		largo := int(carga[posicion])
		posicion++
		if largo == 0 {
			break
		}
		// Los punteros de compresion no aparecen en la pregunta; si aparece uno,
		// el mensaje esta mal armado y se descarta en vez de adivinar.
		if largo > 63 || posicion+largo > len(carga) {
			return "", false
		}
		partes = append(partes, string(carga[posicion:posicion+largo]))
		posicion += largo
	}
	if len(partes) == 0 {
		return "", false
	}
	return limpiarNombre(strings.Join(partes, ".")), true
}

// limpiarNombre deja el nombre en minusculas y sin el punto final.
func limpiarNombre(nombre string) string {
	nombre = strings.ToLower(strings.TrimSpace(nombre))
	nombre = strings.TrimSuffix(nombre, ".")
	// Un nombre con caracteres raros seria de un paquete mal leido, no de un
	// servidor: mejor descartarlo que meter basura en el informe.
	for _, letra := range nombre {
		if letra < 0x20 || letra > 0x7E {
			return ""
		}
	}
	return nombre
}

// DominioPrincipal recorta el nombre a lo que le dice algo a una persona.
//
// "video-edge-a1b2.ftl.iad03.hls.ttvnw.net" no le dice nada a nadie; "ttvnw.net"
// tampoco mucho, pero al menos se agrupa. Sin este recorte, un solo servicio
// aparece como cincuenta destinos distintos y el informe deja de servir.
func DominioPrincipal(nombre string) string {
	partes := strings.Split(nombre, ".")
	if len(partes) <= 2 {
		return nombre
	}

	// Los dominios de dos niveles (com.mx, co.uk) necesitan una etiqueta mas o
	// todo el pais quedaria agrupado bajo "com.mx".
	penultima := partes[len(partes)-2]
	if len(partes) >= 3 && esDominioDeSegundoNivel(penultima) {
		return strings.Join(partes[len(partes)-3:], ".")
	}
	return strings.Join(partes[len(partes)-2:], ".")
}

func esDominioDeSegundoNivel(etiqueta string) bool {
	switch etiqueta {
	case "com", "net", "org", "gob", "edu", "co", "ac":
		return true
	default:
		return false
	}
}

// porPuerto es el ultimo recurso: dice el tipo de servicio cuando no hubo nombre.
var porPuerto = map[int]string{
	22:    "SSH",
	25:    "Correo (SMTP)",
	123:   "Hora (NTP)",
	143:   "Correo (IMAP)",
	445:   "Compartir archivos (SMB)",
	587:   "Correo (SMTP)",
	993:   "Correo (IMAP)",
	1194:  "VPN (OpenVPN)",
	3389:  "Escritorio remoto",
	5060:  "Telefonia (SIP)",
	51820: "VPN (WireGuard)",
}

// lector recorre un paquete sin salirse nunca de su tamano.
type lector struct {
	datos    []byte
	posicion int
}

func (l *lector) saltar(cuantos int) { l.posicion += cuantos }

func (l *lector) byte() (byte, bool) {
	if l.posicion < 0 || l.posicion+1 > len(l.datos) {
		return 0, false
	}
	valor := l.datos[l.posicion]
	l.posicion++
	return valor, true
}

func (l *lector) uint16() (uint16, bool) {
	if l.posicion < 0 || l.posicion+2 > len(l.datos) {
		return 0, false
	}
	valor := binary.BigEndian.Uint16(l.datos[l.posicion : l.posicion+2])
	l.posicion += 2
	return valor, true
}

func (l *lector) tomar(cuantos int) ([]byte, bool) {
	if cuantos < 0 || l.posicion < 0 || l.posicion+cuantos > len(l.datos) {
		return nil, false
	}
	trozo := l.datos[l.posicion : l.posicion+cuantos]
	l.posicion += cuantos
	return trozo, true
}
