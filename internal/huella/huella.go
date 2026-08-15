// Paquete huella averigua QUE es un aparato, preguntandoselo a el.
//
// El escaneo dice que hay algo en una direccion y que puertos tiene abiertos.
// Eso alcanza para contar aparatos, no para nombrarlos: en una red de casa el
// modem, la television y la antena tienen los tres el puerto 80 abierto, y
// llamarlos a los tres "servidor web" es la peor respuesta posible, porque
// parece un dato y no lo es.
//
// Aqui se recoge lo que cada aparato cuenta de si mismo sin que haya que
// entrar en el:
//
//   - el titulo y el encabezado Server de su pagina, y el nombre del candado
//     que pide para entrar (el "realm");
//   - el certificado de su HTTPS, donde muchos fabricantes ponen su marca;
//   - lo que anuncia por mDNS/Bonjour, por UPnP y por WS-Discovery;
//   - su nombre de Windows por NetBIOS;
//   - y los protocolos propios de los fabricantes que mas se venden aqui, que
//     contestan modelo, firmware y nombre exactos.
//
// **Nada de esto entra a ningun lado.** No se prueban claves, no se explota
// nada: se pregunta lo que el aparato publica para que le pregunten. Un equipo
// que no quiera contestar, no contesta, y eso tambien es una respuesta.
package huella

import (
	"sort"
	"strings"
)

// Fuentes de donde puede salir un dato. Se guardan con el dato para poder decir
// en la interfaz de donde se supo, en vez de presentar todo como si fuera igual
// de firme.
const (
	FuenteWeb          = "web"
	FuenteCertificado  = "certificado"
	FuenteSSH          = "ssh"
	FuenteMDNS         = "mdns"
	FuenteSSDP         = "ssdp"
	FuenteNetBIOS      = "netbios"
	FuenteONVIF        = "onvif"
	FuenteUbiquiti     = "ubiquiti"
	FuenteMikrotik     = "mikrotik"
	FuenteTPLink       = "tplink"
	FuenteHikvision    = "hikvision"
	FuenteDahua        = "dahua"
	FuenteRoku         = "roku"
	FuenteImpresora    = "impresora"
	FuenteTelevisionLG = "television"
	FuenteDLink        = "dlink"
	FuenteNetgear      = "netgear"
	FuenteXiaomi       = "xiaomi"
	FuenteRuckus       = "ruckus"
	FuenteFortinet     = "fortinet"
	FuenteTuya         = "tuya"
)

// Claves de lo que se puede averiguar. Son pocas y siempre las mismas, para que
// el catalogo pueda buscar en ellas sin saber de que fuente vinieron.
const (
	ClaveNombre     = "nombre"
	ClaveModelo     = "modelo"
	ClaveFabricante = "fabricante"
	ClaveFirmware   = "firmware"
	ClaveSerie      = "serie"
	ClaveTitulo     = "titulo"
	ClaveServidor   = "servidor"
	ClaveRealm      = "realm"
	ClaveMAC        = "mac"
	ClaveServicio   = "servicio"
	ClaveRed        = "red"
)

// Dato es una cosa que un aparato dijo de si mismo.
type Dato struct {
	Fuente string `json:"fuente"`
	Clave  string `json:"clave"`
	Valor  string `json:"valor"`
}

// Huella es todo lo que se le saco a una direccion.
type Huella struct {
	IP    string `json:"ip"`
	Datos []Dato `json:"datos,omitempty"`
}

// Agregar suma un dato, sin repetir y sin guardar vacios.
func (h *Huella) Agregar(fuente, clave, valor string) {
	valor = limpiar(valor)
	if valor == "" {
		return
	}
	for _, dato := range h.Datos {
		if dato.Fuente == fuente && dato.Clave == clave && dato.Valor == valor {
			return
		}
	}
	h.Datos = append(h.Datos, Dato{Fuente: fuente, Clave: clave, Valor: valor})
}

// Valor devuelve lo primero que se sepa de una clave, mirando todas las fuentes.
func (h Huella) Valor(clave string) string {
	for _, dato := range h.Datos {
		if dato.Clave == clave {
			return dato.Valor
		}
	}
	return ""
}

// Texto junta todo en una sola linea, que es contra lo que busca el catalogo.
func (h Huella) Texto() string {
	partes := make([]string, 0, len(h.Datos))
	for _, dato := range h.Datos {
		partes = append(partes, dato.Valor)
	}
	return strings.Join(partes, " ")
}

// Ordenar deja los datos en un orden estable, para que dos escaneos iguales no
// se vean distintos solo porque las respuestas llegaron en otro orden.
func (h *Huella) Ordenar() {
	sort.SliceStable(h.Datos, func(i, j int) bool {
		if h.Datos[i].Fuente != h.Datos[j].Fuente {
			return h.Datos[i].Fuente < h.Datos[j].Fuente
		}
		if h.Datos[i].Clave != h.Datos[j].Clave {
			return h.Datos[i].Clave < h.Datos[j].Clave
		}
		return h.Datos[i].Valor < h.Datos[j].Valor
	})
}

// limpiar deja un valor presentable: sin caracteres de control, sin espacios de
// sobra y sin novelas. Lo que contesta un aparato puede venir con basura y con
// kilobytes de relleno.
func limpiar(texto string) string {
	texto = strings.Map(func(letra rune) rune {
		if letra < 32 || letra == 127 {
			return ' '
		}
		return letra
	}, texto)
	texto = strings.Join(strings.Fields(texto), " ")
	if len(texto) > 160 {
		texto = strings.TrimSpace(texto[:160])
	}
	return texto
}
