package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/tuxormax/mired/internal/autenticacion"
	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/programador"
)

type peticionEscaneo struct {
	// SoloPresencia hace el barrido rapido: quien esta, sin tocar puertos.
	SoloPresencia bool `json:"soloPresencia"`
}

// lanzarEscaneo arranca un barrido y contesta de inmediato.
func (a *API) lanzarEscaneo(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())
	nivel, _ := autenticacion.NivelDe(peticion.Context())
	if !autenticacion.PuedeEscribir(nivel) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Escaneo", Accion: "Lanzar", Causa: CausaPermiso,
			Codigo: http.StatusForbidden,
		}, "Necesita permiso de escritura sobre esta red.", nil)
		return
	}

	var cuerpo peticionEscaneo
	if peticion.ContentLength > 0 && !a.leerCuerpo(escritor, peticion, &cuerpo, "Escaneo", "Lanzar") {
		return
	}

	escaneoID, subredes, err := a.Programador.Lanzar(peticion.Context(), clave, cuerpo.SoloPresencia)
	switch {
	case errors.Is(err, programador.ErrSinSubredes):
		a.errorValidacion(escritor, peticion, "Escaneo", "Lanzar",
			"Esta red no tiene ninguna subred marcada para escanear. Agregue al menos una.")
		return
	case errors.Is(err, programador.ErrYaEnCurso):
		a.errorValidacion(escritor, peticion, "Escaneo", "Lanzar",
			"Ya hay un escaneo en curso en esta red. Espere a que termine.")
		return
	case err != nil:
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Escaneo", Accion: "Lanzar", Causa: CausaBaseDatos,
			Tabla: "escaneos", Codigo: http.StatusInternalServerError,
			Estado: "El escaneo NO se inicio.",
		}, "No se pudo iniciar el escaneo.", err)
		return
	}

	tipo := basedatos.TipoProfundo
	if cuerpo.SoloPresencia {
		tipo = basedatos.TipoPresencia
	}

	a.anotarActividad(peticion, "Escaneo", "Lanzar escaneo "+tipo+" en "+clave)
	responderOk(escritor, map[string]any{
		"escaneoId": escaneoID,
		"estado":    "en_curso",
		"tipo":      tipo,
		"subredes":  subredes,
	})
}

// listarEscaneos devuelve las ultimas corridas, para que la interfaz sepa si hay
// una en curso y como termino la anterior.
func (a *API) listarEscaneos(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var corridas []map[string]any
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		filas, err := base.QueryContext(peticion.Context(), `
			SELECT id, tipo, iniciado, terminado, estado, equipos_vistos, equipos_nuevos, detalle
			  FROM escaneos
			 ORDER BY id DESC
			 LIMIT 20`)
		if err != nil {
			return err
		}
		defer filas.Close()

		for filas.Next() {
			var id, vistos, nuevos int64
			var tipo, iniciado, estado string
			var terminado, detalle *string
			if err := filas.Scan(&id, &tipo, &iniciado, &terminado, &estado, &vistos, &nuevos, &detalle); err != nil {
				return err
			}
			corridas = append(corridas, map[string]any{
				"id": id, "tipo": tipo, "iniciado": iniciado, "terminado": terminado,
				"estado": estado, "vistos": vistos, "nuevos": nuevos, "detalle": detalle,
			})
		}
		return filas.Err()
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Escaneo", Accion: "Listar", Causa: CausaBaseDatos,
			Tabla: "escaneos", Codigo: http.StatusInternalServerError,
		}, "No se pudieron leer los escaneos.", err)
		return
	}
	if corridas == nil {
		corridas = []map[string]any{}
	}
	responderOk(escritor, map[string]any{
		"escaneos": corridas,
		"enCurso":  a.Programador.EnCurso(clave),
	})
}

type peticionAgenda struct {
	Programado            bool `json:"programado"`
	PresenciaCadaSegundos int  `json:"presenciaCadaSegundos"`
	ProfundoCadaMinutos   int  `json:"profundoCadaMinutos"`
}

// configurarAgenda enciende o apaga los barridos automaticos de una red.
func (a *API) configurarAgenda(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())
	nivel, _ := autenticacion.NivelDe(peticion.Context())
	if !autenticacion.PuedeAdministrar(nivel) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Agenda", Accion: "Configurar", Causa: CausaPermiso,
			Codigo: http.StatusForbidden,
		}, "Necesita permiso de administracion sobre esta red.", nil)
		return
	}

	var cuerpo peticionAgenda
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Agenda", "Configurar") {
		return
	}

	// Los mismos limites que impone la capa de datos, para que el rechazo no
	// llegue despues de mandar el formulario.
	switch {
	case cuerpo.PresenciaCadaSegundos < basedatos.PresenciaMinimaSegundos ||
		cuerpo.PresenciaCadaSegundos > basedatos.PresenciaMaximaSegundos:
		a.errorValidacion(escritor, peticion, "Agenda", "Configurar",
			fmt.Sprintf("El barrido de presencia debe ir de %d a %d segundos.",
				basedatos.PresenciaMinimaSegundos, basedatos.PresenciaMaximaSegundos))
		return
	case cuerpo.ProfundoCadaMinutos < basedatos.ProfundoMinimoMinutos ||
		cuerpo.ProfundoCadaMinutos > basedatos.ProfundoMaximoMinutos:
		a.errorValidacion(escritor, peticion, "Agenda", "Configurar",
			fmt.Sprintf("El escaneo profundo debe ir de %d a %d minutos.",
				basedatos.ProfundoMinimoMinutos, basedatos.ProfundoMaximoMinutos))
		return
	}

	red, err := a.Datos.ConfigurarAgenda(peticion.Context(), clave, cuerpo.Programado,
		cuerpo.PresenciaCadaSegundos, cuerpo.ProfundoCadaMinutos)
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Agenda", Accion: "Configurar", Causa: CausaBaseDatos,
			Tabla: "redes", Codigo: http.StatusInternalServerError,
		}, "No se pudo guardar la agenda.", err)
		return
	}

	a.anotarActividad(peticion, "Agenda", "Configurar agenda de "+clave)
	responderOk(escritor, red)
}

// ------------------------------------------------------------- equipos ----

func (a *API) listarEquipos(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())
	soloPresentes := peticion.URL.Query().Get("presentes") == "1"

	var equipos []basedatos.Equipo
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		equipos, err = base.ListarEquipos(peticion.Context(), soloPresentes)
		return err
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Equipos", Accion: "Listar", Causa: CausaBaseDatos,
			Tabla: "equipos", Codigo: http.StatusInternalServerError,
		}, "No se pudieron listar los equipos.", err)
		return
	}
	responderOk(escritor, equipos)
}

// listarPresencia devuelve el historial de conexiones de un equipo: a que hora
// aparece y a que hora se va.
func (a *API) listarPresencia(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	id, err := strconv.ParseInt(peticion.PathValue("equipo"), 10, 64)
	if err != nil || id <= 0 {
		a.errorValidacion(escritor, peticion, "Equipos", "Presencia", "El equipo no es valido.")
		return
	}

	var eventos []basedatos.EventoPresencia
	err = a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		eventos, err = base.ListarPresencia(peticion.Context(), id, 100)
		return err
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Equipos", Accion: "Presencia", Causa: CausaBaseDatos,
			Tabla: "eventos_presencia", Codigo: http.StatusInternalServerError,
		}, "No se pudo leer el historial de presencia.", err)
		return
	}
	responderOk(escritor, eventos)
}

type peticionAlias struct {
	Alias string `json:"alias"`
}

// ponerAlias le pone al equipo el nombre con el que lo conoce la gente. Es lo
// que convierte "192.168.1.47" en "Impresora de contabilidad".
func (a *API) ponerAlias(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())
	nivel, _ := autenticacion.NivelDe(peticion.Context())
	if !autenticacion.PuedeEscribir(nivel) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Equipos", Accion: "Renombrar", Causa: CausaPermiso,
			Codigo: http.StatusForbidden,
		}, "Necesita permiso de escritura sobre esta red.", nil)
		return
	}

	id, err := strconv.ParseInt(peticion.PathValue("equipo"), 10, 64)
	if err != nil || id <= 0 {
		a.errorValidacion(escritor, peticion, "Equipos", "Renombrar", "El equipo no es valido.")
		return
	}

	var cuerpo peticionAlias
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Equipos", "Renombrar") {
		return
	}
	if len(cuerpo.Alias) > 120 {
		a.errorValidacion(escritor, peticion, "Equipos", "Renombrar",
			"El nombre no puede pasar de 120 caracteres.")
		return
	}

	err = a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		return base.PonerAlias(peticion.Context(), id, cuerpo.Alias)
	})
	if err != nil {
		if errors.Is(err, basedatos.ErrEquipoNoExiste) {
			a.errorValidacion(escritor, peticion, "Equipos", "Renombrar", "Ese equipo no existe.")
			return
		}
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Equipos", Accion: "Renombrar", Causa: CausaBaseDatos,
			Tabla: "equipos", Codigo: http.StatusInternalServerError,
		}, "No se pudo guardar el nombre.", err)
		return
	}

	a.anotarActividad(peticion, "Equipos", fmt.Sprintf("Renombrar equipo %d de %s", id, clave))
	responderOk(escritor, map[string]any{"guardado": true})
}
