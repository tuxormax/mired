package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/tuxormax/mired/internal/aire"
	"github.com/tuxormax/mired/internal/autenticacion"
	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/sonda"
)

// barrerAire escucha que redes inalambricas se oyen desde este equipo.
//
// Es la unica medicion de MiRed que no pasa por un cable, y contesta lo que
// ningun barrido de IP puede: que SSID emite cada antena y con que **MAC de
// radio**, que es distinta de la del cable.
//
// No se guarda en la base: es una foto del momento. El aire cambia con quien
// pasa por el pasillo, y guardar cada barrido como si fuera inventario llenaria
// la base de ruido.
func (a *API) barrerAire(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	crudo, err := sonda.PedirAire(a.SocketSonda, 2*time.Minute)
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Aire", Accion: "Barrer el aire", Causa: CausaRed,
			Codigo: http.StatusBadGateway,
		}, "No se pudo hablar con la sonda para escuchar el aire.", err)
		return
	}

	var resultado aire.Resultado
	if err := json.Unmarshal(crudo, &resultado); err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Aire", Accion: "Barrer el aire", Causa: CausaRed,
			Codigo: http.StatusBadGateway,
		}, "La sonda contesto algo que no se entendio.", err)
		return
	}

	// Se cruza con el inventario para poder decir de QUE aparato es cada antena.
	var equipos []basedatos.Equipo
	if clave != "" {
		_ = a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
			var err error
			equipos, err = base.ListarEquipos(peticion.Context(), false)
			return err
		})
	}

	redes := make([]map[string]any, 0, len(resultado.Redes))
	for _, red := range resultado.Redes {
		fila := map[string]any{
			"ssid":      red.SSID,
			"bssid":     red.BSSID,
			"canal":     red.Canal,
			"banda":     red.Banda,
			"senal":     red.Senal,
			"seguridad": red.Seguridad,
		}
		if equipo, comoSeSupo := deQuienEsLaAntena(red.BSSID, equipos); equipo != nil {
			fila["equipoId"] = equipo.ID
			fila["equipoNombre"] = nombreDeEquipo(*equipo)
			fila["comoSeSupo"] = comoSeSupo
		}
		redes = append(redes, fila)
	}

	responderOk(escritor, map[string]any{
		"redes":       redes,
		"herramienta": resultado.Herramienta,
		"explicacion": resultado.Explicacion,
		"duracionMs":  resultado.DuracionMs,
	})
}

// deQuienEsLaAntena busca de que equipo del inventario es ese radio.
//
// Hay dos formas, y NO valen lo mismo:
//
//  1. La MAC del radio coincide con la del equipo. Eso es un hecho.
//  2. La MAC del radio esta a uno o dos numeros de la del cable. Casi todos los
//     fabricantes numeran asi sus interfaces, pero es una DEDUCCION, y se
//     devuelve marcada como tal para que la interfaz no la presente como si
//     alguien la hubiera medido.
func deQuienEsLaAntena(bssid string, equipos []basedatos.Equipo) (*basedatos.Equipo, string) {
	radio := soloHex(bssid)
	if len(radio) != 12 {
		return nil, ""
	}

	for i := range equipos {
		if soloHex(equipos[i].MAC) == radio {
			return &equipos[i], "medido"
		}
	}

	// La deduccion: mismo prefijo de fabricante y los ultimos bytes a menos de
	// cuatro de distancia.
	for i := range equipos {
		cable := soloHex(equipos[i].MAC)
		if len(cable) != 12 || cable[:6] != radio[:6] {
			continue
		}
		if distancia(cable, radio) <= 4 {
			return &equipos[i], "deducido por cercania de MAC"
		}
	}
	return nil, ""
}

func distancia(una, otra string) int64 {
	uno, dos := hexANumero(una[6:]), hexANumero(otra[6:])
	if uno > dos {
		return uno - dos
	}
	return dos - uno
}

func hexANumero(texto string) int64 {
	var valor int64
	for _, letra := range texto {
		valor *= 16
		switch {
		case letra >= '0' && letra <= '9':
			valor += int64(letra - '0')
		case letra >= 'a' && letra <= 'f':
			valor += int64(letra-'a') + 10
		}
	}
	return valor
}

func soloHex(texto string) string {
	var limpio strings.Builder
	for _, letra := range strings.ToLower(texto) {
		if (letra >= '0' && letra <= '9') || (letra >= 'a' && letra <= 'f') {
			limpio.WriteRune(letra)
		}
	}
	return limpio.String()
}

func nombreDeEquipo(equipo basedatos.Equipo) string {
	switch {
	case equipo.Alias != "":
		return equipo.Alias
	case equipo.Nombre != "":
		return equipo.Nombre
	default:
		return equipo.IP
	}
}
