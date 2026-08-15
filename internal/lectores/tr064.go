package lectores

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// TR-064 es el idioma con el que se le pregunta a un MODEM del proveedor.
//
// Es la pieza que faltaba para las redes de casa de aqui: el modem de Telmex, de
// Totalplay, de izzi o de Megacable es de una marca distinta cada vez —Huawei,
// ZTE, Askey, Technicolor, Arris— y ninguno tiene un panel parecido al otro.
// Pero casi todos implementan TR-064, que es un estandar del Broadband Forum
// pensado justo para esto, y ahi hay una tabla que se llama **Hosts** con todo
// lo que el modem tiene conectado, por cable y por WiFi, con su nombre y su
// direccion.
//
// Es viejo, es SOAP y casi nadie lo recuerda. Y funciona.
//
// Si el modem lo tiene apagado —algunos proveedores lo desactivan— este lector
// no contesta y el generico se queda con lo que pueda del panel.
type TR064 struct{}

func (TR064) Nombre() string { return "TR-064 (modem del proveedor)" }

func (TR064) Reconoce(pistas string) bool {
	return conPistas(pistas, "huawei", "zte", "askey", "technicolor", "arris",
		"sagemcom", "nokia", "fiberhome", "sernet", "gateway", "modem",
		"internetgatewaydevice")
}

// puertosTR064 son donde escucha: el 49000 es el del estandar y el 5000 el que
// usan varios fabricantes para lo mismo.
var puertosTR064 = []int{49000, 5000}

// rutasDeHosts son los sitios donde vive el servicio de Hosts segun el
// fabricante.
var rutasDeHosts = []string{
	"/upnp/control/hosts",
	"/upnp/control/lanhostconfigmgm",
	"/control/Hosts",
}

const espacioHosts = "urn:dslforum-org:service:Hosts:1"

func (t TR064) Leer(ctx context.Context, destino Destino) ([]Conectado, error) {
	cliente := nuevaSesion(destino.Espera)
	maquina := soloMaquina(destino.Direccion)

	for _, puerto := range puertosTR064 {
		base := fmt.Sprintf("http://%s:%d", maquina, puerto)
		for _, ruta := range rutasDeHosts {
			conectados, err := t.leerDesde(ctx, cliente, base+ruta, destino)
			if err == nil && len(conectados) > 0 {
				return conectados, nil
			}
			if EsCredencialRechazada(err) {
				return nil, err
			}
		}
	}
	return nil, ErrNoSeEntendio
}

func (t TR064) leerDesde(ctx context.Context, cliente clienteHTTP, direccion string,
	destino Destino) ([]Conectado, error) {
	// Primero, cuantos hay. Si esto no contesta, aqui no hay TR-064.
	crudo, err := t.llamar(ctx, cliente, direccion, "GetHostNumberOfEntries", "", destino)
	if err != nil {
		return nil, err
	}
	cuantos, _ := strconv.Atoi(entreEtiquetas(string(crudo), "NewHostNumberOfEntries"))
	if cuantos <= 0 {
		return nil, ErrNoSeEntendio
	}
	// Un modem de casa tiene decenas de equipos, no miles. El tope evita que una
	// respuesta rara mande a MiRed a preguntar diez mil veces.
	if cuantos > 254 {
		cuantos = 254
	}

	conectados := make([]Conectado, 0, cuantos)
	for indice := 0; indice < cuantos; indice++ {
		cuerpo := fmt.Sprintf("<NewIndex>%d</NewIndex>", indice)
		crudo, err := t.llamar(ctx, cliente, direccion, "GetGenericHostEntry", cuerpo, destino)
		if err != nil {
			continue
		}

		texto := string(crudo)
		mac := entreEtiquetas(texto, "NewMACAddress")
		if NormalizarMAC(mac) == "" {
			continue
		}
		// El modem dice si esta activo ahora mismo: los que no lo estan son
		// historia, y colgarlos del mapa diria que estan cuando no estan.
		if activo := entreEtiquetas(texto, "NewActive"); activo == "0" || activo == "false" {
			continue
		}

		// InterfaceType dice "Ethernet" o "802.11": es lo que separa lo que entra
		// por un puerto de lo que entra por el aire.
		tipo := strings.ToLower(entreEtiquetas(texto, "NewInterfaceType"))
		conectados = append(conectados, Conectado{
			MAC:      mac,
			Nombre:   entreEtiquetas(texto, "NewHostName"),
			IP:       entreEtiquetas(texto, "NewIPAddress"),
			PorCable: !strings.Contains(tipo, "802.11") && !strings.Contains(tipo, "wifi"),
		})
	}

	if len(conectados) == 0 {
		return nil, ErrNoSeEntendio
	}
	return conectados, nil
}

// llamar manda una peticion SOAP y devuelve la respuesta.
func (TR064) llamar(ctx context.Context, cliente clienteHTTP, direccion, accion, cuerpo string,
	destino Destino) ([]byte, error) {
	sobre := `<?xml version="1.0" encoding="utf-8"?>` +
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" ` +
		`s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body>` +
		`<u:` + accion + ` xmlns:u="` + espacioHosts + `">` + cuerpo +
		`</u:` + accion + `></s:Body></s:Envelope>`

	return pedir(ctx, cliente, "POST", direccion, strings.NewReader(sobre), map[string]string{
		"Content-Type": `text/xml; charset="utf-8"`,
		"SOAPAction":   espacioHosts + "#" + accion,
	}, destino.Usuario, destino.Clave)
}

// soloMaquina se queda con el nombre o la IP, sin esquema ni puerto: TR-064 vive
// en su propio puerto, no en el del panel web.
func soloMaquina(direccion string) string {
	direccion = strings.TrimSpace(direccion)
	direccion = strings.TrimPrefix(strings.TrimPrefix(direccion, "https://"), "http://")
	if corte := strings.IndexAny(direccion, "/"); corte >= 0 {
		direccion = direccion[:corte]
	}
	if corte := strings.LastIndex(direccion, ":"); corte > 0 && !strings.Contains(direccion, "]") {
		direccion = direccion[:corte]
	}
	return direccion
}

// entreEtiquetas saca el contenido de una etiqueta XML, sin importar el prefijo
// de espacio de nombres con el que venga.
func entreEtiquetas(texto, etiqueta string) string {
	minusculas := strings.ToLower(texto)
	buscada := strings.ToLower(etiqueta)

	desde := 0
	for desde < len(minusculas) {
		abre := strings.Index(minusculas[desde:], "<")
		if abre < 0 {
			return ""
		}
		abre += desde
		cierra := strings.Index(minusculas[abre:], ">")
		if cierra < 0 {
			return ""
		}
		cierra += abre

		nombre := minusculas[abre+1 : cierra]
		if corte := strings.IndexAny(nombre, " \t\r\n"); corte >= 0 {
			nombre = nombre[:corte]
		}
		if corte := strings.Index(nombre, ":"); corte >= 0 {
			nombre = nombre[corte+1:]
		}

		if nombre == buscada {
			fin := strings.Index(minusculas[cierra:], "</")
			if fin < 0 {
				return ""
			}
			return strings.TrimSpace(texto[cierra+1 : cierra+fin])
		}
		desde = cierra + 1
	}
	return ""
}
