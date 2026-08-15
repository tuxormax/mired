package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/tuxormax/mired/internal/autenticacion"
	"github.com/tuxormax/mired/internal/basedatos"
)

// Lo que cuelga de una antena por el aire.
//
// El WiFi no tiene puertos, asi que a una antena se le cuelgan **uno o varios**
// equipos sin inventarle un puerto a cada uno. La regla va al reves: un equipo
// cuelga de UNA antena a la vez, porque asi funciona el WiFi.

// colgarPorWiFi cuelga uno o varios equipos de una antena, de una sola vez.
//
// Se acepta una lista y no un equipo por llamada porque asi es como se hace en
// la vida real: uno se para frente al mapa, ve los seis aparatos que sabe que
// entran por WiFi, y los cuelga todos. Obligar a repetir el flujo seis veces
// seria castigar al que declara bien su red.
func (a *API) colgarPorWiFi(escritor http.ResponseWriter, peticion *http.Request) {
	if !a.exigeEscritura(escritor, peticion, "Colgar por WiFi") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var cuerpo struct {
		AntenaID int64   `json:"antenaId"`
		Equipos  []int64 `json:"equipos"`
		Red      string  `json:"red"`
		Notas    string  `json:"notas"`
	}
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Topologia inalambrica", "Colgar por WiFi") {
		return
	}
	if cuerpo.AntenaID <= 0 || len(cuerpo.Equipos) == 0 {
		a.errorValidacion(escritor, peticion, "Topologia inalambrica", "Colgar por WiFi",
			"Hace falta decir de que antena cuelgan y que equipos son.")
		return
	}

	var colgados []basedatos.EnlaceInalambrico
	var rechazados []string

	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		for _, equipoID := range cuerpo.Equipos {
			enlace, err := base.GuardarEnlaceInalambrico(peticion.Context(), basedatos.EnlaceInalambrico{
				EquipoID:   equipoID,
				AntenaID:   cuerpo.AntenaID,
				Red:        cuerpo.Red,
				Notas:      cuerpo.Notas,
				OrigenDato: basedatos.OrigenManual,
			})
			switch {
			case errors.Is(err, basedatos.ErrEnlaceAlReves):
				rechazados = append(rechazados, "una antena no se cuelga de si misma")
			case errors.Is(err, basedatos.ErrEquipoNoExiste):
				rechazados = append(rechazados, fmt.Sprintf("el equipo %d ya no existe", equipoID))
			case errors.Is(err, basedatos.ErrAntenaNoExiste):
				return err
			case err != nil:
				return err
			default:
				colgados = append(colgados, enlace)
			}
		}
		return nil
	})

	switch {
	case errors.Is(err, basedatos.ErrAntenaNoExiste):
		a.errorValidacion(escritor, peticion, "Topologia inalambrica", "Colgar por WiFi",
			"Esa antena ya no existe.")
		return
	case err != nil:
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Topologia inalambrica", Accion: "Colgar por WiFi", Causa: CausaBaseDatos,
			Tabla: "enlaces_inalambricos", Codigo: http.StatusInternalServerError,
		}, "No se pudo colgar por WiFi.", err)
		return
	}

	a.anotarActividad(peticion, "Topologia inalambrica",
		fmt.Sprintf("Colgar %d equipos por WiFi de la antena %d en %s",
			len(colgados), cuerpo.AntenaID, clave))
	responderOk(escritor, map[string]any{
		"colgados": colgados,
		// Lo que no se pudo colgar se DICE. Colgar cinco de seis y contestar que
		// todo salio bien es la peor forma de que alguien se entere tarde.
		"rechazados": rechazados,
	})
}

// descolgarDeWiFi quita un equipo de su antena.
func (a *API) descolgarDeWiFi(escritor http.ResponseWriter, peticion *http.Request) {
	if !a.exigeEscritura(escritor, peticion, "Descolgar del WiFi") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	id, ok := a.leerIDDeRuta(escritor, peticion, "enlace", "Descolgar del WiFi",
		"El enlace no es valido.")
	if !ok {
		return
	}

	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		return base.BorrarEnlaceInalambrico(peticion.Context(), id)
	})
	if errors.Is(err, basedatos.ErrEnlaceNoExiste) {
		a.errorValidacion(escritor, peticion, "Topologia inalambrica", "Descolgar del WiFi",
			"Ese enlace ya no existe.")
		return
	}
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Topologia inalambrica", Accion: "Descolgar del WiFi", Causa: CausaBaseDatos,
			Tabla: "enlaces_inalambricos", Codigo: http.StatusInternalServerError,
		}, "No se pudo descolgar el equipo.", err)
		return
	}

	a.anotarActividad(peticion, "Topologia inalambrica",
		fmt.Sprintf("Descolgar el enlace inalambrico %d de %s", id, clave))
	responderOk(escritor, map[string]any{"borrado": true})
}
