package lectores

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// clienteHTTP es lo que necesitan los lectores de un cliente. Se nombra para
// poder pasarlo entre archivos sin arrastrar todo net/http.
type clienteHTTP = *http.Client

// Generico entra a un panel cualquiera y se queda con las direcciones fisicas
// que encuentre.
//
// Es el ultimo recurso, y su promesa es pequena a proposito: **no dice de que
// SSID cuelga nadie ni con que senal llega**. Dice "estas MAC aparecen en el
// panel de este aparato", que ya es suficiente para ubicar en el mapa lo que
// antes flotaba abajo sin dueno.
//
// Funciona porque en el panel de casi cualquier aparato de red hay una sola cosa
// con forma de MAC: la tabla de conectados. Se prueban las direcciones donde
// suele vivir esa tabla y se lee lo que haya.
//
// Lo que NO hace: adivinar el formulario de entrada de cada fabricante. Prueba
// la autenticacion normal de HTTP y el formulario mas comun; si el panel usa
// otra cosa, este lector no contesta y se dice, en vez de dejar creer que el
// aparato no tiene a nadie conectado.
type Generico struct{}

func (Generico) Nombre() string { return "panel generico" }

// Reconoce siempre dice que no: este lector no es de ninguna marca, y por eso se
// prueba al final de todos.
func (Generico) Reconoce(string) bool { return false }

// rutasConTablas son los sitios donde los paneles suelen publicar sus
// conectados. Van de lo mas especifico a lo mas general.
var rutasConTablas = []string{
	"/", // muchos paneles chicos lo pintan todo en la portada
	"/index.html",
	"/status.html",
	"/dhcp.html",
	"/dhcpinfo.html",
	"/lan_status.html",
	"/wlan_status.html",
	"/DeviceInfo.html",
	"/cgi-bin/luci/admin/status/overview",
	"/api/dhcp/leases",
	"/data/getDeviceList.json",
}

func (g Generico) Leer(ctx context.Context, destino Destino) ([]Conectado, error) {
	cliente := nuevaSesion(destino.Espera)

	// Se prueban las dos formas: HTTPS primero, porque un panel que ya redirige
	// ahi no acepta nada por el puerto 80.
	for _, seguro := range []bool{true, false} {
		base := comoURL(destino.Direccion, seguro)

		// Primero la autenticacion normal de HTTP, que es lo que usan la mitad
		// de los aparatos viejos y no necesita saber nada del panel.
		if conectados := g.recorrer(ctx, cliente, base, destino, true); len(conectados) > 0 {
			return conectados, nil
		}

		// Y si no, el formulario mas comun. No se prueban veinte variantes: lo
		// que no entra asi, no entra, y decirlo es mejor que inventar.
		g.entrarConFormulario(ctx, cliente, base, destino)
		if conectados := g.recorrer(ctx, cliente, base, destino, false); len(conectados) > 0 {
			return conectados, nil
		}
	}
	return nil, ErrNoSeEntendio
}

func (Generico) recorrer(ctx context.Context, cliente clienteHTTP, base string,
	destino Destino, conAuth bool) []Conectado {
	usuario, clave := "", ""
	if conAuth {
		usuario, clave = destino.Usuario, destino.Clave
	}

	vistas := map[string]bool{}
	var conectados []Conectado

	for _, ruta := range rutasConTablas {
		crudo, err := pedir(ctx, cliente, "GET", base+ruta, nil, nil, usuario, clave)
		if err != nil || len(crudo) == 0 {
			continue
		}
		for _, mac := range macsDe(string(crudo)) {
			if vistas[mac] {
				continue
			}
			vistas[mac] = true
			// **PorCable en verdadero a proposito.** El panel no dijo como entro
			// ese equipo, y colgar del WiFi algo que puede estar enchufado seria
			// dibujar una red que no existe. Aqui solo consta que el aparato lo
			// tiene conectado.
			conectados = append(conectados, Conectado{MAC: mac, PorCable: true})
		}
	}
	return conectados
}

// entrarConFormulario prueba el formulario de entrada mas comun.
func (Generico) entrarConFormulario(ctx context.Context, cliente clienteHTTP, base string,
	destino Destino) {
	formulario := url.Values{}
	formulario.Set("username", destino.Usuario)
	formulario.Set("password", destino.Clave)
	// Varios paneles esperan estos nombres en vez de los de arriba. Mandar los
	// dos juegos a la vez es feo y funciona: el que no los espera los ignora.
	formulario.Set("user", destino.Usuario)
	formulario.Set("pass", destino.Clave)
	formulario.Set("luci_username", destino.Usuario)
	formulario.Set("luci_password", destino.Clave)

	for _, ruta := range []string{"/login.cgi", "/login", "/cgi-bin/luci", "/"} {
		_, _ = pedir(ctx, cliente, "POST", base+ruta, strings.NewReader(formulario.Encode()),
			map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, "", "")
	}
}
