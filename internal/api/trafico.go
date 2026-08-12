package api

import (
	"net/http"
	"strconv"

	"github.com/tuxormax/mired/internal/autenticacion"
	"github.com/tuxormax/mired/internal/basedatos"
)

// consumoActual responde "quien se esta comiendo el internet".
//
// Sale de los contadores que el switch ya llevaba, cruzados con el mapa de
// puertos: sin capturar un solo paquete y sin poner a MiRed en el camino del
// trafico.
func (a *API) consumoActual(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var consumo []basedatos.ConsumoDePuerto
	var capacidad string
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		consumo, err = base.ConsumoActual(peticion.Context())
		if err != nil {
			return err
		}
		return base.QueryRowContext(peticion.Context(),
			`SELECT capacidad_mapa_puertos FROM ficha WHERE id = 1`).Scan(&capacidad)
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Trafico", Accion: "Consultar", Causa: CausaBaseDatos,
			Tabla: "muestras_trafico", Codigo: http.StatusInternalServerError,
		}, "No se pudo leer el consumo de la red.", err)
		return
	}

	responderOk(escritor, map[string]any{
		"consumo":     consumo,
		"explicacion": explicarTrafico(capacidad, len(consumo)),
	})
}

// historialTrafico devuelve los puntos de la grafica de una boca.
func (a *API) historialTrafico(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	switchID, err := strconv.ParseInt(peticion.PathValue("switch"), 10, 64)
	if err != nil || switchID <= 0 {
		a.errorValidacion(escritor, peticion, "Trafico", "Historial", "El switch no es valido.")
		return
	}
	indice, err := strconv.Atoi(peticion.PathValue("puerto"))
	if err != nil || indice <= 0 {
		a.errorValidacion(escritor, peticion, "Trafico", "Historial", "El puerto no es valido.")
		return
	}

	var puntos []basedatos.PuntoTrafico
	err = a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		puntos, err = base.HistorialTrafico(peticion.Context(), switchID, indice, 200)
		return err
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Trafico", Accion: "Historial", Causa: CausaBaseDatos,
			Tabla: "muestras_trafico", Codigo: http.StatusInternalServerError,
		}, "No se pudo leer el historial de trafico.", err)
		return
	}
	responderOk(escritor, puntos)
}

func explicarTrafico(capacidad string, cuantos int) string {
	switch {
	case cuantos > 0 && capacidad == basedatos.CapacidadExacta:
		return "El consumo sale de los contadores de cada boca del switch, cruzados con el equipo " +
			"que cuelga de ella. Da volumen, no aplicaciones."
	case cuantos > 0:
		return "Hay consumo medido por boca, pero en algunas cuelga mas de un equipo: ahi el " +
			"numero es del grupo entero, no de un aparato."
	case capacidad == basedatos.CapacidadNoDisponible:
		return "En esta red no se puede medir consumo por puerto: ningun switch contesto SNMP. " +
			"Para medirlo sin switches administrables haria falta que el router exporte flujos " +
			"(NetFlow o sFlow), que todavia no esta implementado."
	default:
		return "Todavia no hay mediciones. Hacen falta al menos dos escaneos completos con " +
			"credenciales SNMP cargadas: el consumo se calcula restando dos lecturas."
	}
}
