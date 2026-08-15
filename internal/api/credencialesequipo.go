package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/tuxormax/mired/internal/autenticacion"
	"github.com/tuxormax/mired/internal/basedatos"
)

// Las credenciales de cada equipo.
//
// Tres reglas sostienen esto, y las tres estan en el codigo, no en un documento:
//
//  1. La clave se guarda cifrada (ver internal/secreto).
//  2. La clave NO viaja en ningun listado. Solo cuando alguien la pide, y eso
//     queda anotado en la bitacora de actividad con quien fue.
//  3. Nunca sale en un mapa exportado.

// guardarCredencialEquipo guarda como se entra a un aparato.
func (a *API) guardarCredencialEquipo(escritor http.ResponseWriter, peticion *http.Request) {
	if !a.exigeEscritura(escritor, peticion, "Guardar credencial") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	equipoID, ok := a.leerIDDeRuta(escritor, peticion, "equipo", "Guardar credencial",
		"El equipo no es valido.")
	if !ok {
		return
	}

	var cuerpo basedatos.CredencialEquipo
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Credenciales", "Guardar credencial") {
		return
	}
	cuerpo.EquipoID = equipoID

	if err := a.Secretos.Listo(); err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Credenciales", Accion: "Guardar credencial", Causa: CausaArchivo,
			Codigo: http.StatusInternalServerError,
		}, "No se pudo preparar el cifrado de las claves.", err)
		return
	}

	var guardada basedatos.CredencialEquipo
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		guardada, err = base.GuardarCredencialEquipo(peticion.Context(), a.Secretos, cuerpo)
		return err
	})
	switch {
	case errors.Is(err, basedatos.ErrEquipoNoExiste):
		a.errorValidacion(escritor, peticion, "Credenciales", "Guardar credencial",
			"Ese equipo ya no existe.")
		return
	case err != nil:
		a.errorValidacion(escritor, peticion, "Credenciales", "Guardar credencial", err.Error())
		return
	}

	// En la bitacora va QUE se guardo, nunca la clave.
	a.anotarActividad(peticion, "Credenciales",
		fmt.Sprintf("Guardar la credencial %s del equipo %d en %s", guardada.Tipo, equipoID, clave))
	responderOk(escritor, guardada)
}

// verClaveDeEquipo entrega la clave en claro.
//
// Es la unica puerta por donde sale una clave, y por eso: exige permiso de
// ESCRITURA —no basta con poder mirar la red— y **queda anotado en la bitacora
// quien la pidio y cuando**. Una clave que se puede ver sin dejar rastro es una
// clave que nadie sabe quien tiene.
func (a *API) verClaveDeEquipo(escritor http.ResponseWriter, peticion *http.Request) {
	if !a.exigeEscritura(escritor, peticion, "Ver la clave") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	equipoID, ok := a.leerIDDeRuta(escritor, peticion, "equipo", "Ver la clave",
		"El equipo no es valido.")
	if !ok {
		return
	}
	tipo := peticion.URL.Query().Get("tipo")
	if tipo == "" {
		tipo = "web"
	}

	var credencial basedatos.CredencialEquipo
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		credencial, err = base.CredencialDeEquipo(peticion.Context(), a.Secretos, equipoID, tipo, true)
		return err
	})
	switch {
	case errors.Is(err, basedatos.ErrCredencialNoExiste):
		a.errorValidacion(escritor, peticion, "Credenciales", "Ver la clave",
			"Ese equipo no tiene guardada esa credencial.")
		return
	case err != nil:
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Credenciales", Accion: "Ver la clave", Causa: CausaArchivo,
			Tabla: "credenciales_equipo", Codigo: http.StatusInternalServerError,
		}, "No se pudo leer la clave guardada.", err)
		return
	}

	a.anotarActividad(peticion, "Credenciales",
		fmt.Sprintf("VER la clave %s del equipo %d en %s", tipo, equipoID, clave))
	responderOk(escritor, credencial)
}

// borrarCredencialEquipo quita una credencial guardada.
func (a *API) borrarCredencialEquipo(escritor http.ResponseWriter, peticion *http.Request) {
	if !a.exigeEscritura(escritor, peticion, "Borrar credencial") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	id, ok := a.leerIDDeRuta(escritor, peticion, "credencial", "Borrar credencial",
		"La credencial no es valida.")
	if !ok {
		return
	}

	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		return base.BorrarCredencialEquipo(peticion.Context(), id)
	})
	if errors.Is(err, basedatos.ErrCredencialNoExiste) {
		a.errorValidacion(escritor, peticion, "Credenciales", "Borrar credencial",
			"Esa credencial ya no existe.")
		return
	}
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Credenciales", Accion: "Borrar credencial", Causa: CausaBaseDatos,
			Tabla: "credenciales_equipo", Codigo: http.StatusInternalServerError,
		}, "No se pudo borrar la credencial.", err)
		return
	}

	a.anotarActividad(peticion, "Credenciales",
		fmt.Sprintf("Borrar la credencial %d en %s", id, clave))
	responderOk(escritor, map[string]any{"borrado": true})
}
