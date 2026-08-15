package lectores

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
)

// MikroTik lee un RouterOS por su API REST.
//
// Es de los mas agradecidos: contesta JSON limpio, con autenticacion HTTP
// normal, y trae **las dos cosas a la vez** —quien esta asociado por WiFi y que
// direcciones reparte por DHCP—, que es justo lo que hace falta para colgar del
// aparato lo que cuelga de el.
//
// La API REST existe desde RouterOS 7. En un RouterOS 6 no hay nada que hacer
// por aqui, y entonces el lector generico se queda con lo que pueda del panel.
type MikroTik struct{}

func (MikroTik) Nombre() string { return "RouterOS (MikroTik)" }

func (MikroTik) Reconoce(pistas string) bool {
	return conPistas(pistas, "mikrotik", "routeros", "routerboard")
}

func (m MikroTik) Leer(ctx context.Context, destino Destino) ([]Conectado, error) {
	cliente := nuevaSesion(destino.Espera)
	// RouterOS sirve la REST por HTTPS con su certificado de fabrica.
	base := comoURL(destino.Direccion, true) + "/rest"

	var conectados []Conectado
	hablo := false

	// Los asociados por WiFi. RouterOS 7 movio la tabla de sitio, asi que se
	// prueban las dos rutas: la nueva y la de siempre.
	for _, ruta := range []string{"/interface/wifi/registration-table", "/interface/wireless/registration-table"} {
		crudo, err := pedir(ctx, cliente, "GET", base+ruta, nil,
			map[string]string{"Accept": "application/json"}, destino.Usuario, destino.Clave)
		if err != nil {
			if EsCredencialRechazada(err) {
				return nil, err
			}
			continue
		}
		hablo = true
		conectados = append(conectados, leerRegistroMikrotik(crudo)...)
	}

	// Y lo que reparte por DHCP, que es como se sabe el nombre y la direccion de
	// lo que entra por cable.
	crudo, err := pedir(ctx, cliente, "GET", base+"/ip/dhcp-server/lease", nil,
		map[string]string{"Accept": "application/json"}, destino.Usuario, destino.Clave)
	if err == nil {
		hablo = true
		conectados = append(conectados, leerArrendamientosMikrotik(crudo)...)
	} else if EsCredencialRechazada(err) {
		return nil, err
	}

	if !hablo {
		return nil, ErrNoSeEntendio
	}
	return conectados, nil
}

func leerRegistroMikrotik(crudo []byte) []Conectado {
	var filas []map[string]any
	if err := json.Unmarshal(crudo, &filas); err != nil {
		return nil
	}

	conectados := make([]Conectado, 0, len(filas))
	for _, fila := range filas {
		mac := textoDe(fila, "mac-address")
		if mac == "" {
			continue
		}
		conectados = append(conectados, Conectado{
			MAC:      mac,
			Nombre:   textoDe(fila, "comment"),
			Red:      textoDe(fila, "ssid", "interface"),
			SenalDbm: enteroDe(textoDe(fila, "signal-strength", "signal")),
		})
	}
	return conectados
}

func leerArrendamientosMikrotik(crudo []byte) []Conectado {
	var filas []map[string]any
	if err := json.Unmarshal(crudo, &filas); err != nil {
		return nil
	}

	conectados := make([]Conectado, 0, len(filas))
	for _, fila := range filas {
		mac := textoDe(fila, "mac-address")
		if mac == "" {
			continue
		}
		// Una direccion repartida por DHCP no dice por si sola si el equipo esta
		// enchufado o entro por WiFi. Se marca como cable y, si ademas aparece
		// en la tabla de asociados, esa gana al juntar: ahi si consta que entro
		// por el aire.
		conectados = append(conectados, Conectado{
			MAC:      mac,
			Nombre:   textoDe(fila, "host-name", "comment"),
			IP:       textoDe(fila, "address"),
			PorCable: true,
		})
	}
	return conectados
}

// textoDe saca el primer campo que exista, como texto.
func textoDe(fila map[string]any, claves ...string) string {
	for _, clave := range claves {
		valor, hay := fila[clave]
		if !hay {
			continue
		}
		switch tipo := valor.(type) {
		case string:
			if texto := strings.TrimSpace(tipo); texto != "" {
				return texto
			}
		case float64:
			return strconv.FormatFloat(tipo, 'f', -1, 64)
		}
	}
	return ""
}

// enteroDe lee un numero que puede venir con basura pegada: RouterOS escribe la
// senal como "-58dBm@1Mbps".
func enteroDe(texto string) int {
	inicio := -1
	fin := -1
	for i, letra := range texto {
		esDigito := letra >= '0' && letra <= '9'
		esSigno := letra == '-' && inicio < 0
		if (esDigito || esSigno) && inicio < 0 {
			inicio = i
		}
		if inicio >= 0 && !esDigito && !(letra == '-' && i == inicio) {
			fin = i
			break
		}
	}
	if inicio < 0 {
		return 0
	}
	if fin < 0 {
		fin = len(texto)
	}
	numero, err := strconv.Atoi(texto[inicio:fin])
	if err != nil {
		return 0
	}
	return numero
}
