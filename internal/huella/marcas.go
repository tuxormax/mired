package huella

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5" // #nosec G501 — no es seguridad: es la llave fija y publica de Tuya
	"encoding/binary"
	"strings"
	"sync"
	"time"
)

// Los protocolos de descubrimiento de las demas marcas.
//
// Aqui hay dos clases de protocolo y **conviene no confundirlas**:
//
//   - Los **documentados**, donde se sabe el formato exacto y se decodifica de
//     verdad: Netgear NSDP, Xiaomi miIO, Tuya.
//   - Los que solo se conocen por **ingenieria inversa** —D-Link DDP, Dahua
//     DHDiscover, Ruckus, TP-Link Omada—. Ahi MiRed manda el saludo que se sabe
//     que funciona y **lee lo que entienda** de la respuesta, sin fingir que
//     decodifica el protocolo entero. Si el aparato contesta algo con su modelo
//     dentro, se saca; si contesta otra cosa, no se inventa nada.
//
// Esa segunda clase es honesta y util a la vez: para el inventario alcanza con
// saber "esto contesto el protocolo de D-Link y dijo DGS-1210-10P".

const (
	puertoOmada    = 29810 // TP-Link Omada / EAP
	puertoDLink    = 62976 // D-Link Discovery Protocol
	puertoNetgear  = 63322 // Netgear Switch Discovery Protocol
	puertoDahua    = 37810 // Dahua DHDiscover
	puertoXiaomi   = 54321 // Xiaomi miIO
	puertoRuckus   = 12223 // Ruckus / ZoneDirector
	puertoFortiFGM = 541   // FortiGate a FortiManager (FGFM)
)

// ------------------------------------------------------------- TP-Link Omada --

// DeOmada pregunta a los puntos de acceso y switches Omada de TP-Link.
//
// Es la otra familia de TP-Link, la de oficina: los EAP y los switches
// administrables. No hablan el protocolo de los focos Kasa.
func DeOmada(ctx context.Context, ip string, espera time.Duration) []Dato {
	// El saludo de descubrimiento del controlador Omada. Los aparatos contestan
	// con un bloque que lleva su modelo y su firmware en texto claro.
	saludo := []byte{0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	respuesta := preguntarUDP(ctx, ip, puertoOmada, saludo, espera)
	return leerRespuestaDeMarca(respuesta, FuenteTPLink, "TP-Link", []string{"eap", "omada", "tl-"})
}

// ---------------------------------------------------------------- D-Link --

// DeDLink pregunta por el protocolo de descubrimiento de D-Link.
//
// Es el que usa su "Network Assistant" para encontrar switches y puntos de
// acceso que no tienen ni direccion configurada.
func DeDLink(ctx context.Context, ip string, espera time.Duration) []Dato {
	saludo := make([]byte, 20)
	saludo[0] = 0x0b // version del protocolo
	saludo[1] = 0x01 // descubrir
	respuesta := preguntarUDP(ctx, ip, puertoDLink, saludo, espera)
	return leerRespuestaDeMarca(respuesta, FuenteDLink, "D-Link", []string{"dgs-", "des-", "dap-", "dir-", "dws-"})
}

// ---------------------------------------------------------------- Netgear --

// DeNetgear pregunta por NSDP, el protocolo de los switches ProSafe.
//
// Este si esta documentado: cabecera de 32 bytes y despues campos con tipo,
// largo y valor. Devuelve modelo, nombre, MAC y firmware.
func DeNetgear(ctx context.Context, ip string, espera time.Duration) []Dato {
	respuesta := preguntarUDP(ctx, ip, puertoNetgear, peticionNSDP(), espera)
	return leerNSDP(respuesta)
}

func peticionNSDP() []byte {
	peticion := make([]byte, 32)
	peticion[0] = 0x01 // version
	peticion[1] = 0x01 // leer
	copy(peticion[24:28], []byte("NSDP"))

	// Se piden los campos por su numero, con largo cero: "digame estos".
	for _, campo := range []uint16{0x0001, 0x0003, 0x0004, 0x0006, 0x000d} {
		dos := make([]byte, 4)
		binary.BigEndian.PutUint16(dos[0:2], campo)
		peticion = append(peticion, dos...)
	}
	// Marca de fin.
	return append(peticion, 0xff, 0xff, 0x00, 0x00)
}

func leerNSDP(respuesta []byte) []Dato {
	if len(respuesta) < 36 || string(respuesta[24:28]) != "NSDP" {
		return nil
	}

	var huella Huella
	cuerpo := respuesta[32:]
	for len(cuerpo) >= 4 {
		tipo := binary.BigEndian.Uint16(cuerpo[0:2])
		largo := int(binary.BigEndian.Uint16(cuerpo[2:4]))
		if tipo == 0xffff || 4+largo > len(cuerpo) {
			break
		}
		valor := cuerpo[4 : 4+largo]
		cuerpo = cuerpo[4+largo:]

		switch tipo {
		case 0x0001:
			huella.Agregar(FuenteNetgear, ClaveModelo, string(valor))
		case 0x0003:
			huella.Agregar(FuenteNetgear, ClaveNombre, string(valor))
		case 0x0004:
			if len(valor) == 6 {
				huella.Agregar(FuenteNetgear, ClaveMAC, comoMAC(valor))
			}
		case 0x000d:
			huella.Agregar(FuenteNetgear, ClaveFirmware, string(valor))
		}
	}

	if len(huella.Datos) > 0 {
		huella.Agregar(FuenteNetgear, ClaveFabricante, "NETGEAR")
	}
	return huella.Datos
}

// ------------------------------------------------------------------ Dahua --

// DeDahua pregunta por DHDiscover, el protocolo de las camaras y grabadores
// Dahua.
//
// ONVIF ya identifica a esas camaras; esto agrega el detalle fino —numero de
// serie, tipo exacto de aparato y firmware—, que es lo que hace falta para
// distinguir dos grabadores del mismo modelo.
func DeDahua(ctx context.Context, ip string, espera time.Duration) []Dato {
	consulta := []byte(`{"method":"DHDiscover.search","params":{"mac":"","uni":1}}`)

	// Cabecera de 32 bytes: marca, largo del cuerpo dos veces y relleno. Es lo
	// que espera el aparato antes del JSON.
	paquete := make([]byte, 32)
	paquete[0] = 0xa3
	binary.LittleEndian.PutUint32(paquete[4:8], uint32(len(consulta)))
	binary.LittleEndian.PutUint32(paquete[16:20], uint32(len(consulta)))
	paquete = append(paquete, consulta...)

	return leerDahua(preguntarUDP(ctx, ip, puertoDahua, paquete, espera))
}

// leerDahua saca del JSON lo que la camara dice de si misma.
func leerDahua(respuesta []byte) []Dato {
	if len(respuesta) == 0 {
		return nil
	}

	texto := string(respuesta)
	if !strings.Contains(texto, "{") {
		return nil
	}

	var huella Huella
	huella.Agregar(FuenteDahua, ClaveFabricante, "Dahua")
	huella.Agregar(FuenteDahua, ClaveModelo, entreComillas(texto, `"DeviceType"`))
	huella.Agregar(FuenteDahua, ClaveSerie, entreComillas(texto, `"SerialNo"`))
	huella.Agregar(FuenteDahua, ClaveFirmware, entreComillas(texto, `"Version"`))
	huella.Agregar(FuenteDahua, ClaveMAC, entreComillas(texto, `"mac"`))
	return huella.Datos
}

// ----------------------------------------------------------------- Xiaomi --

// DeXiaomi saluda por miIO, el protocolo de los aparatos de casa de Xiaomi.
//
// Sin el token del aparato no se puede leer nada mas que esto, y se dice: el
// saludo confirma que ES un aparato Xiaomi de casa inteligente —no un telefono—
// y da su identificador. Prometer mas seria mentir.
func DeXiaomi(ctx context.Context, ip string, espera time.Duration) []Dato {
	saludo := make([]byte, 32)
	saludo[0], saludo[1] = 0x21, 0x31 // marca del protocolo
	saludo[2], saludo[3] = 0x00, 0x20 // largo: 32
	for i := 4; i < 32; i++ {
		saludo[i] = 0xff
	}

	return leerXiaomi(preguntarUDP(ctx, ip, puertoXiaomi, saludo, espera))
}

// leerXiaomi confirma el saludo y saca el identificador del aparato.
func leerXiaomi(respuesta []byte) []Dato {
	if len(respuesta) < 16 || respuesta[0] != 0x21 || respuesta[1] != 0x31 {
		return nil
	}

	var huella Huella
	huella.Agregar(FuenteXiaomi, ClaveFabricante, "Xiaomi")
	huella.Agregar(FuenteXiaomi, ClaveServicio, "aparato de casa inteligente (miIO)")
	identificador := binary.BigEndian.Uint32(respuesta[8:12])
	if identificador != 0 && identificador != 0xffffffff {
		huella.Agregar(FuenteXiaomi, ClaveSerie, formatearNumero(identificador))
	}
	return huella.Datos
}

// ----------------------------------------------------------------- Ruckus --

// DeRuckus pregunta por el protocolo de descubrimiento de Ruckus.
//
// Es el que usan los puntos de acceso para encontrar a su controladora
// (ZoneDirector o Unleashed). La respuesta trae el modelo y el nombre que le
// pusieron al punto de acceso.
func DeRuckus(ctx context.Context, ip string, espera time.Duration) []Dato {
	saludo := []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	respuesta := preguntarUDP(ctx, ip, puertoRuckus, saludo, espera)
	return leerRespuestaDeMarca(respuesta, FuenteRuckus, "Ruckus",
		[]string{"ruckus", "zonedirector", "unleashed", "zf", "r3", "r5", "r6", "t3"})
}

// ---------------------------------------------------------------- Fortinet --

// DeFortinet reconoce a un FortiGate por lo que deja ver sin preguntarle nada.
//
// No tiene protocolo de descubrimiento abierto —se administra por su web y, del
// lado del proveedor, por FGFM—, pero se delata solo: su certificado lleva la
// organizacion Fortinet y el numero de serie en el nombre comun, y el puerto 541
// no lo usa nadie mas.
func DeFortinet(ctx context.Context, ip string, puertosAbiertos []int, espera time.Duration) []Dato {
	var huella Huella

	if tienePuerto(puertosAbiertos, puertoFortiFGM) {
		huella.Agregar(FuenteFortinet, ClaveServicio, "FGFM, el canal de gestion de Fortinet")
		huella.Agregar(FuenteFortinet, ClaveFabricante, "Fortinet")
	}

	// Su panel vive en HTTPS y casi siempre fuera del 443. Se prueban los suyos
	// solo si estan abiertos: no se toca un puerto que nadie anuncio.
	for _, puerto := range []int{10443, 8443, 443} {
		if !tienePuerto(puertosAbiertos, puerto) {
			continue
		}
		for _, dato := range delCertificado(ctx, ip, puerto, espera) {
			// El certificado de fabrica de un FortiGate trae la organizacion
			// Fortinet y, en el nombre comun, el numero de serie del aparato,
			// que empieza por el modelo: FGT60F..., FWF61E..., FAP221E...
			if !pareceFortinet(dato.Valor) {
				continue
			}
			huella.Agregar(FuenteFortinet, ClaveFabricante, "Fortinet")
			if dato.Clave == ClaveNombre {
				huella.Agregar(FuenteFortinet, ClaveSerie, dato.Valor)
				huella.Agregar(FuenteFortinet, ClaveModelo, modeloDeSerieFortinet(dato.Valor))
			}
		}
	}
	return huella.Datos
}

func pareceFortinet(texto string) bool {
	texto = strings.ToLower(texto)
	for _, marca := range []string{"fortinet", "fortigate", "fortiwifi", "fortiap", "fgt", "fwf", "fap"} {
		if strings.Contains(texto, marca) {
			return true
		}
	}
	return false
}

// modeloDeSerieFortinet saca el modelo del numero de serie.
//
// Fortinet numera sus aparatos con el modelo delante: FGT60FTK1234567890 es un
// FortiGate 60F. Se devuelve lo que se sabe leer y nada mas.
func modeloDeSerieFortinet(serie string) string {
	limpio := strings.ToUpper(strings.TrimSpace(serie))
	familias := map[string]string{
		"FGT": "FortiGate", "FWF": "FortiWiFi", "FAP": "FortiAP",
		"FSW": "FortiSwitch", "FAZ": "FortiAnalyzer", "FMG": "FortiManager",
	}
	if len(limpio) < 6 {
		return ""
	}
	familia, hay := familias[limpio[:3]]
	if !hay {
		return ""
	}
	// Lo que sigue a la familia es el modelo, hasta que aparece la letra del
	// lote: en FGT60FTK..., el modelo es 60F.
	modelo := strings.Builder{}
	for _, letra := range limpio[3:] {
		if letra >= '0' && letra <= '9' {
			modelo.WriteRune(letra)
			continue
		}
		if modelo.Len() > 0 {
			modelo.WriteRune(letra)
		}
		break
	}
	if modelo.Len() == 0 {
		return familia
	}
	return familia + " " + modelo.String()
}

// -------------------------------------------------------- Tuya / Smart Life --

// llaveTuya es la llave FIJA y publica con la que Tuya tapa sus anuncios.
//
// No es un secreto ni hay nada que romper: es la misma en todos los aparatos del
// mundo y esta en la documentacion de sus propias librerias. Sirve para leer el
// anuncio que el aparato manda a gritos a toda la red, no para entrar en el.
var llaveTuya = md5.Sum([]byte("yGAdlopoPVldABfn")) // #nosec G401

// EscucharTuya oye los anuncios de los aparatos Tuya / Smart Life.
//
// Estos no se dejan preguntar: **avisan solos**, cada pocos segundos, a toda la
// red. Por eso aqui no se manda nada, solo se escucha. Es la familia de los
// focos, los enchufes y las camaras baratas que se venden en cualquier lado, y
// hasta ahora salian como "sin reconocer" porque no contestan a nada.
func EscucharTuya(ctx context.Context, espera time.Duration) map[string][]Dato {
	encontrados := map[string][]Dato{}
	if espera <= 0 {
		espera = 3 * time.Second
	}

	// El 6667 va tapado y el 6666 en claro, segun la edad del aparato. Se
	// escuchan los dos A LA VEZ: uno tras otro serian dos esperas en fila para
	// oir lo mismo.
	var candado sync.Mutex
	var grupo sync.WaitGroup

	for _, puerto := range []int{6667, 6666} {
		grupo.Add(1)
		go func(cual int) {
			defer grupo.Done()
			for ip, mensajes := range escucharDifusion(ctx, cual, espera) {
				for _, mensaje := range mensajes {
					datos := leerAnuncioTuya(mensaje)
					if len(datos) == 0 {
						continue
					}
					candado.Lock()
					encontrados[ip] = append(encontrados[ip], datos...)
					candado.Unlock()
				}
			}
		}(puerto)
	}
	grupo.Wait()

	return encontrados
}

// leerAnuncioTuya destapa el anuncio si hace falta y saca lo que dice.
//
// Se decide por el CONTENIDO y no por si trae una llave o una comilla: un
// mensaje tapado puede llevar por casualidad cualquier byte, y confiar en eso
// haria que el reconocimiento funcionara o no segun la suerte.
func leerAnuncioTuya(mensaje []byte) []Dato {
	texto := string(mensaje)
	if !pareceAnuncioTuya(texto) {
		texto = destaparTuya(mensaje)
	}
	if !pareceAnuncioTuya(texto) {
		return nil
	}

	var huella Huella
	huella.Agregar(FuenteTuya, ClaveFabricante, "Tuya / Smart Life")
	huella.Agregar(FuenteTuya, ClaveServicio, "aparato de casa inteligente (Tuya)")
	huella.Agregar(FuenteTuya, ClaveSerie, entreComillas(texto, `"gwId"`))
	huella.Agregar(FuenteTuya, ClaveModelo, entreComillas(texto, `"productKey"`))
	huella.Agregar(FuenteTuya, ClaveFirmware, entreComillas(texto, `"version"`))
	return huella.Datos
}

func pareceAnuncioTuya(texto string) bool {
	return strings.Contains(texto, "gwId") || strings.Contains(texto, "productKey")
}

// destaparTuya quita la cabecera del anuncio y lo descifra con la llave publica.
func destaparTuya(mensaje []byte) string {
	// Cabecera de 20 bytes y cola de 8: lo de en medio es el mensaje.
	if len(mensaje) <= 28 {
		return ""
	}
	cuerpo := mensaje[20 : len(mensaje)-8]
	if len(cuerpo)%aes.BlockSize != 0 || len(cuerpo) == 0 {
		return ""
	}

	bloque, err := aes.NewCipher(llaveTuya[:])
	if err != nil {
		return ""
	}
	claro := make([]byte, len(cuerpo))
	for inicio := 0; inicio < len(cuerpo); inicio += aes.BlockSize {
		bloque.Decrypt(claro[inicio:inicio+aes.BlockSize], cuerpo[inicio:inicio+aes.BlockSize])
	}
	return string(quitarRelleno(claro))
}

// quitarRelleno saca el relleno PKCS#7 del final.
func quitarRelleno(datos []byte) []byte {
	if len(datos) == 0 {
		return datos
	}
	relleno := int(datos[len(datos)-1])
	if relleno <= 0 || relleno > aes.BlockSize || relleno > len(datos) {
		return datos
	}
	return datos[:len(datos)-relleno]
}

// asegurar que el paquete crypto/cipher se use aunque cambie la implementacion.
var _ cipher.Block

// ------------------------------------------------------------ auxiliares --

// leerRespuestaDeMarca saca lo aprovechable de un protocolo que solo se conoce
// por ingenieria inversa.
//
// No se finge decodificar el formato entero: se buscan dentro de la respuesta
// las cadenas legibles, y si alguna se parece a un modelo de la marca, se anota.
// Es poco, pero es CIERTO, que es la diferencia que importa.
func leerRespuestaDeMarca(respuesta []byte, fuente, fabricante string, modelos []string) []Dato {
	if len(respuesta) == 0 {
		return nil
	}

	var huella Huella
	huella.Agregar(fuente, ClaveFabricante, fabricante)

	for _, cadena := range cadenasLegibles(respuesta, 4) {
		minusculas := strings.ToLower(cadena)
		for _, pedazo := range modelos {
			if strings.Contains(minusculas, pedazo) {
				huella.Agregar(fuente, ClaveModelo, cadena)
				break
			}
		}
		// Un nombre puesto por una persona no se parece a ningun modelo, pero
		// tampoco se puede tomar cualquier cadena: solo la primera que sea
		// legible y razonablemente corta.
		if huella.Valor(ClaveNombre) == "" && len(cadena) <= 32 && !strings.ContainsAny(cadena, "{}=") {
			huella.Agregar(fuente, ClaveNombre, cadena)
		}
	}
	return huella.Datos
}

// cadenasLegibles saca los textos imprimibles de un bloque binario.
func cadenasLegibles(datos []byte, minimo int) []string {
	var cadenas []string
	var actual strings.Builder

	guardar := func() {
		if actual.Len() >= minimo {
			cadenas = append(cadenas, strings.TrimSpace(actual.String()))
		}
		actual.Reset()
	}

	for _, letra := range datos {
		if letra >= 32 && letra < 127 {
			actual.WriteByte(letra)
			continue
		}
		guardar()
	}
	guardar()
	return cadenas
}

func formatearNumero(valor uint32) string {
	if valor == 0 {
		return ""
	}
	digitos := ""
	for valor > 0 {
		digitos = string(rune('0'+valor%10)) + digitos
		valor /= 10
	}
	return digitos
}
