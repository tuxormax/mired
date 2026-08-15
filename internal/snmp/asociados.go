package snmp

import (
	"strings"

	"github.com/gosnmp/gosnmp"
)

// Quien esta asociado a una antena, preguntandoselo a la antena.
//
// El WiFi no tiene puertos: un telefono no entra por el 3 ni por el 7, entra por
// el aire. Por eso esto no se puede resolver con la tabla de puentes que usa el
// mapa de puertos —esa dice "estas MAC estan detras de este aparato", en grupo—
// y hace falta preguntar por la tabla de ASOCIADOS, que es la que sabe de que
// SSID cuelga cada uno y con que senal llega.
//
// Los OID de esa tabla no son estandar: cada fabricante puso la suya. Aqui van
// los de las dos familias que mas se ven, y se recorren todas: la que no exista
// simplemente no devuelve nada.

const (
	// AirOS (las antenas airMAX y los AirCube): tabla de estaciones asociadas.
	oidUbntEstaciones = "1.3.6.1.4.1.10002.1.1.1.4.2.1"
	// UniFi suelto (sin controladora): tabla de VAP, con su SSID y sus clientes.
	oidUnifiVAP = "1.3.6.1.4.1.41112.1.6.1.2.1"
	// El SSID de cada VAP, dentro de la tabla anterior.
	oidUnifiVAPEssid = "1.3.6.1.4.1.41112.1.6.1.2.1.1.6"
	// MikroTik: tabla de registro de clientes inalambricos.
	oidMikrotikRegistro = "1.3.6.1.4.1.14988.1.1.1.2.1"
	// La senal de cada cliente de MikroTik, dentro de esa tabla.
	oidMikrotikSenal = "1.3.6.1.4.1.14988.1.1.1.2.1.3"
)

// Asociado es un equipo colgado de una antena por el aire.
type Asociado struct {
	// MAC del cliente. Es lo unico que la antena sabe con certeza de el.
	MAC string `json:"mac"`
	// Red es el SSID por el que entro, cuando la antena lo dice.
	Red string `json:"red,omitempty"`
	// SenalDbm es con cuanta fuerza llega. Cero cuando no se pudo leer: un cero
	// inventado en dBm seria una senal perfecta, justo lo contrario de "no se".
	SenalDbm int `json:"senalDbm,omitempty"`
}

// leerAsociados pregunta por la tabla de asociados de una antena.
//
// Devuelve vacio si el equipo no es una antena o no expone la tabla, que es lo
// normal en un switch: no es un error, es que ahi no hay nada que preguntar.
func leerAsociados(conexion *gosnmp.GoSNMP) []Asociado {
	porMAC := map[string]*Asociado{}

	anotar := func(mac string) *Asociado {
		mac = normalizarMAC(mac)
		if mac == "" {
			return nil
		}
		if existente, hay := porMAC[mac]; hay {
			return existente
		}
		nuevo := &Asociado{MAC: mac}
		porMAC[mac] = nuevo
		return nuevo
	}

	// --- AirOS: el indice de la tabla ES la MAC del cliente, en decimal.
	recorrer(conexion, oidUbntEstaciones, func(sufijo string, dato gosnmp.SnmpPDU) {
		if mac := macDesdeOID(sufijo); mac != "" {
			anotar(mac)
			return
		}
		// Algunas versiones ponen la MAC en el VALOR y no en el indice.
		if mac := macDesdeValor(dato); mac != "" {
			anotar(mac)
		}
	})

	// --- UniFi suelto: los clientes cuelgan de la tabla de VAP, y el SSID vive
	// en una columna de esa misma tabla.
	essidPorVAP := map[string]string{}
	recorrer(conexion, oidUnifiVAPEssid, func(sufijo string, dato gosnmp.SnmpPDU) {
		if texto := comoTexto(dato); texto != "" {
			essidPorVAP[primerTramo(sufijo)] = texto
		}
	})
	recorrer(conexion, oidUnifiVAP, func(sufijo string, dato gosnmp.SnmpPDU) {
		mac := macDesdeValor(dato)
		if mac == "" {
			mac = macDesdeOID(sufijo)
		}
		if mac == "" {
			return
		}
		if asociado := anotar(mac); asociado != nil && asociado.Red == "" {
			asociado.Red = essidPorVAP[primerTramo(sufijo)]
		}
	})

	// --- MikroTik: la MAC va en el indice y la senal en su propia columna.
	senalPorMAC := map[string]int{}
	recorrer(conexion, oidMikrotikSenal, func(sufijo string, dato gosnmp.SnmpPDU) {
		if mac := macDesdeOID(sufijo); mac != "" {
			senalPorMAC[normalizarMAC(mac)] = int(comoEntero(dato))
		}
	})
	recorrer(conexion, oidMikrotikRegistro, func(sufijo string, dato gosnmp.SnmpPDU) {
		mac := macDesdeOID(sufijo)
		if mac == "" {
			return
		}
		if asociado := anotar(mac); asociado != nil && asociado.SenalDbm == 0 {
			asociado.SenalDbm = senalPorMAC[normalizarMAC(mac)]
		}
	})

	asociados := make([]Asociado, 0, len(porMAC))
	for _, asociado := range porMAC {
		asociados = append(asociados, *asociado)
	}
	return asociados
}

// macDesdeValor saca una MAC de lo que contesto el equipo, cuando viene en el
// valor y no en el indice del OID.
func macDesdeValor(dato gosnmp.SnmpPDU) string {
	crudo, ok := dato.Value.([]byte)
	if !ok || len(crudo) != 6 {
		return ""
	}
	partes := make([]string, 6)
	for i, letra := range crudo {
		partes[i] = strings.ToLower(dosDigitos(letra))
	}
	return strings.Join(partes, ":")
}

func dosDigitos(valor byte) string {
	const digitos = "0123456789abcdef"
	return string([]byte{digitos[valor>>4], digitos[valor&0x0f]})
}

func normalizarMAC(mac string) string {
	limpia := make([]rune, 0, 12)
	for _, letra := range strings.ToLower(mac) {
		if (letra >= '0' && letra <= '9') || (letra >= 'a' && letra <= 'f') {
			limpia = append(limpia, letra)
		}
	}
	if len(limpia) != 12 {
		return ""
	}
	texto := string(limpia)
	partes := make([]string, 6)
	for i := 0; i < 6; i++ {
		partes[i] = texto[i*2 : i*2+2]
	}
	return strings.Join(partes, ":")
}

// primerTramo devuelve el primer numero de un sufijo de OID, que en las tablas
// de UniFi identifica al VAP.
func primerTramo(sufijo string) string {
	if corte := strings.Index(sufijo, "."); corte > 0 {
		return sufijo[:corte]
	}
	return sufijo
}
