package api

import (
	"net/http"
	"strconv"

	"github.com/tuxormax/mired/internal/autenticacion"
	"github.com/tuxormax/mired/internal/basedatos"
)

func (a *API) listarAlertas(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())
	soloSinVer := peticion.URL.Query().Get("sinVer") == "1"

	var alertas []basedatos.Alerta
	var abiertas int
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		alertas, err = base.ListarAlertas(peticion.Context(), soloSinVer, 200)
		if err != nil {
			return err
		}
		abiertas, err = base.ContarAlertasAbiertas(peticion.Context())
		return err
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Alertas", Accion: "Listar", Causa: CausaBaseDatos,
			Tabla: "alertas", Codigo: http.StatusInternalServerError,
		}, "No se pudieron listar las alertas.", err)
		return
	}

	responderOk(escritor, map[string]any{"alertas": alertas, "abiertas": abiertas})
}

type peticionVistas struct {
	// IDs vacio significa "todas": es el boton de "ya las vi todas".
	IDs []int64 `json:"ids"`
}

func (a *API) marcarAlertasVistas(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())
	nivel, _ := autenticacion.NivelDe(peticion.Context())
	if !autenticacion.PuedeEscribir(nivel) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Alertas", Accion: "Marcar vistas", Causa: CausaPermiso,
			Codigo: http.StatusForbidden,
		}, "Necesita permiso de escritura sobre esta red.", nil)
		return
	}

	var cuerpo peticionVistas
	if peticion.ContentLength > 0 && !a.leerCuerpo(escritor, peticion, &cuerpo, "Alertas", "Marcar vistas") {
		return
	}

	var marcadas, abiertas int
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		marcadas, err = base.MarcarAlertasVistas(peticion.Context(), cuerpo.IDs)
		if err != nil {
			return err
		}
		abiertas, err = base.ContarAlertasAbiertas(peticion.Context())
		return err
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Alertas", Accion: "Marcar vistas", Causa: CausaBaseDatos,
			Tabla: "alertas", Codigo: http.StatusInternalServerError,
		}, "No se pudieron marcar las alertas.", err)
		return
	}

	// El contador del catalogo se actualiza aqui tambien: si no, el panel de
	// inicio seguiria mostrando alertas que ya se atendieron hasta el siguiente
	// escaneo.
	if err := a.Datos.ActualizarAlertasEnCatalogo(peticion.Context(), clave, abiertas); err != nil {
		a.Bitacora.Warn("no se pudo actualizar el contador de alertas", "red", clave, "error", err)
	}

	responderOk(escritor, map[string]any{"marcadas": marcadas, "abiertas": abiertas})
}

func (a *API) listarReglas(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var reglas map[string]basedatos.Regla
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		reglas, err = base.Reglas(peticion.Context())
		return err
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Alertas", Accion: "Listar reglas", Causa: CausaBaseDatos,
			Tabla: "reglas_alerta", Codigo: http.StatusInternalServerError,
		}, "No se pudieron leer las reglas de alerta.", err)
		return
	}

	lista := make([]basedatos.Regla, 0, len(reglas))
	for _, regla := range reglas {
		lista = append(lista, regla)
	}
	responderOk(escritor, lista)
}

func (a *API) guardarRegla(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())
	nivel, _ := autenticacion.NivelDe(peticion.Context())
	if !autenticacion.PuedeAdministrar(nivel) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Alertas", Accion: "Guardar regla", Causa: CausaPermiso,
			Codigo: http.StatusForbidden,
		}, "Necesita permiso de administracion sobre esta red.", nil)
		return
	}

	var regla basedatos.Regla
	if !a.leerCuerpo(escritor, peticion, &regla, "Alertas", "Guardar regla") {
		return
	}
	regla.Tipo = peticion.PathValue("tipo")
	if regla.Umbral < 0 || regla.Umbral > 525600 { // un ano en minutos
		a.errorValidacion(escritor, peticion, "Alertas", "Guardar regla",
			"El umbral debe estar entre 0 minutos y un ano.")
		return
	}

	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		return base.GuardarRegla(peticion.Context(), regla)
	})
	if err != nil {
		a.errorValidacion(escritor, peticion, "Alertas", "Guardar regla", err.Error())
		return
	}

	a.anotarActividad(peticion, "Alertas", "Configurar regla "+regla.Tipo+" en "+clave)
	responderOk(escritor, regla)
}

func (a *API) listarDestinos(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var destinos []basedatos.DestinoAlerta
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		destinos, err = base.ListarDestinos(peticion.Context())
		return err
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Alertas", Accion: "Listar destinos", Causa: CausaBaseDatos,
			Tabla: "destinos_alerta", Codigo: http.StatusInternalServerError,
		}, "No se pudieron listar los destinos de aviso.", err)
		return
	}

	// El extra puede llevar un token o la clave del correo: no vuelve al
	// navegador. La misma razon que con las comunidades SNMP.
	for i := range destinos {
		if destinos[i].Extra != "" {
			destinos[i].Extra = "(guardado)"
		}
	}
	responderOk(escritor, destinos)
}

func (a *API) crearDestino(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())
	nivel, _ := autenticacion.NivelDe(peticion.Context())
	if !autenticacion.PuedeAdministrar(nivel) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Alertas", Accion: "Crear destino", Causa: CausaPermiso,
			Codigo: http.StatusForbidden,
		}, "Necesita permiso de administracion sobre esta red.", nil)
		return
	}

	var destino basedatos.DestinoAlerta
	if !a.leerCuerpo(escritor, peticion, &destino, "Alertas", "Crear destino") {
		return
	}

	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		destino, err = base.CrearDestino(peticion.Context(), destino)
		return err
	})
	if err != nil {
		a.errorValidacion(escritor, peticion, "Alertas", "Crear destino", err.Error())
		return
	}

	a.anotarActividad(peticion, "Alertas", "Agregar destino "+destino.Nombre+" en "+clave)
	destino.Extra = ""
	responderOk(escritor, destino)
}

func (a *API) borrarDestino(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())
	nivel, _ := autenticacion.NivelDe(peticion.Context())
	if !autenticacion.PuedeAdministrar(nivel) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Alertas", Accion: "Borrar destino", Causa: CausaPermiso,
			Codigo: http.StatusForbidden,
		}, "Necesita permiso de administracion sobre esta red.", nil)
		return
	}

	id, err := strconv.ParseInt(peticion.PathValue("destino"), 10, 64)
	if err != nil || id <= 0 {
		a.errorValidacion(escritor, peticion, "Alertas", "Borrar destino",
			"El destino no es valido.")
		return
	}

	err = a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		return base.BorrarDestino(peticion.Context(), id)
	})
	if err != nil {
		a.errorValidacion(escritor, peticion, "Alertas", "Borrar destino", err.Error())
		return
	}

	a.anotarActividad(peticion, "Alertas", "Borrar destino de "+clave)
	responderOk(escritor, map[string]any{"borrado": true})
}
