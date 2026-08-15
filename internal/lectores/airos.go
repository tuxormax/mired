package lectores

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
)

// AirOS lee las antenas Ubiquiti sueltas: airMAX, AirCube y los UniFi sin
// controladora.
//
// El panel cambio entre versiones, asi que se prueban las direcciones conocidas
// en orden y se lee la primera que conteste algo con MAC dentro. Lo que no se
// entiende no se inventa.
type AirOS struct{}

func (AirOS) Nombre() string { return "AirOS (Ubiquiti)" }

func (AirOS) Reconoce(pistas string) bool {
	return conPistas(pistas, "ubiquiti", "ubnt", "airos", "airmax", "unifi", "aircube")
}

// rutasDeEstaciones son los sitios donde las distintas versiones publican la
// lista de asociados.
var rutasDeEstaciones = []string{"/sta.cgi", "/api/stations", "/status.cgi"}

func (a AirOS) Leer(ctx context.Context, destino Destino) ([]Conectado, error) {
	base := comoURL(destino.Direccion, true)
	cliente := nuevaSesion(destino.Espera)

	// AirOS usa un formulario clasico y una galleta de sesion. Se pide antes la
	// portada porque algunas versiones exigen tener ya la galleta.
	_, _ = pedir(ctx, cliente, "GET", base+"/login.cgi", nil, nil, "", "")

	formulario := url.Values{}
	formulario.Set("username", destino.Usuario)
	formulario.Set("password", destino.Clave)
	formulario.Set("uri", "/")

	if _, err := pedir(ctx, cliente, "POST", base+"/login.cgi",
		strings.NewReader(formulario.Encode()),
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, "", ""); err != nil {
		return nil, err
	}

	for _, ruta := range rutasDeEstaciones {
		crudo, err := pedir(ctx, cliente, "GET", base+ruta, nil, nil, "", "")
		if err != nil || len(crudo) == 0 {
			continue
		}
		if conectados := leerEstacionesAirOS(crudo); len(conectados) > 0 {
			return conectados, nil
		}
	}
	return nil, ErrNoSeEntendio
}

// leerEstacionesAirOS entiende las dos formas en que AirOS contesta: una lista
// suelta de estaciones, o un objeto con la lista dentro.
func leerEstacionesAirOS(crudo []byte) []Conectado {
	var lista []estacionAirOS
	if err := json.Unmarshal(crudo, &lista); err == nil && len(lista) > 0 {
		return convertirAirOS(lista, "")
	}

	var sobre struct {
		Wireless struct {
			Essid string          `json:"essid"`
			Sta   []estacionAirOS `json:"sta"`
		} `json:"wireless"`
		Sta []estacionAirOS `json:"sta"`
	}
	if err := json.Unmarshal(crudo, &sobre); err != nil {
		return nil
	}

	crudas := sobre.Wireless.Sta
	if len(crudas) == 0 {
		crudas = sobre.Sta
	}
	return convertirAirOS(crudas, sobre.Wireless.Essid)
}

type estacionAirOS struct {
	MAC      string `json:"mac"`
	Nombre   string `json:"name"`
	Hostname string `json:"hostname"`
	Essid    string `json:"essid"`
	Senal    int    `json:"signal"`
	RSSI     int    `json:"rssi"`
	Remoto   struct {
		Nombre string `json:"hostname"`
	} `json:"remote"`
}

func convertirAirOS(crudas []estacionAirOS, essidComun string) []Conectado {
	conectados := make([]Conectado, 0, len(crudas))
	for _, cruda := range crudas {
		if strings.TrimSpace(cruda.MAC) == "" {
			continue
		}
		conectado := Conectado{
			MAC:      cruda.MAC,
			Red:      cruda.Essid,
			SenalDbm: cruda.Senal,
		}
		if conectado.Red == "" {
			conectado.Red = essidComun
		}
		if conectado.SenalDbm == 0 {
			conectado.SenalDbm = cruda.RSSI
		}
		switch {
		case cruda.Nombre != "":
			conectado.Nombre = cruda.Nombre
		case cruda.Hostname != "":
			conectado.Nombre = cruda.Hostname
		case cruda.Remoto.Nombre != "":
			conectado.Nombre = cruda.Remoto.Nombre
		}
		conectados = append(conectados, conectado)
	}
	return conectados
}
