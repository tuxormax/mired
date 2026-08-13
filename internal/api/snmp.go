package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/tuxormax/mired/internal/autenticacion"
	"github.com/tuxormax/mired/internal/basedatos"
)

// listarCredenciales devuelve las credenciales SNMP SIN sus secretos.
//
// La comunidad SNMP es en la practica una contrasena: no tiene por que viajar de
// vuelta al navegador solo para pintar una lista.
func (a *API) listarCredenciales(escritor http.ResponseWriter, peticion *http.Request) {
	credenciales, err := a.Datos.ListarCredencialesSNMP(peticion.Context())
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Credenciales SNMP", Accion: "Listar", Causa: CausaBaseDatos,
			Tabla: "credenciales_snmp", Codigo: http.StatusInternalServerError,
		}, "No se pudieron listar las credenciales SNMP.", err)
		return
	}

	publicas := make([]basedatos.CredencialSNMP, 0, len(credenciales))
	for _, credencial := range credenciales {
		publicas = append(publicas, credencial.SinSecretos())
	}
	responderOk(escritor, publicas)
}

func (a *API) crearCredencial(escritor http.ResponseWriter, peticion *http.Request) {
	var cuerpo basedatos.CredencialSNMP
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Credenciales SNMP", "Crear") {
		return
	}

	credencial, err := a.Datos.CrearCredencialSNMP(peticion.Context(), cuerpo)
	if errors.Is(err, basedatos.ErrCredencialRepetida) {
		a.errorValidacion(escritor, peticion, "Credenciales SNMP", "Crear",
			"Ya existe una credencial con ese nombre.")
		return
	}
	if err != nil {
		// Los datos incompletos (falta comunidad, version invalida) los detecta
		// la capa de datos y su mensaje ya esta en lenguaje llano.
		a.errorValidacion(escritor, peticion, "Credenciales SNMP", "Crear", err.Error())
		return
	}

	a.anotarActividad(peticion, "Credenciales SNMP", "Crear credencial "+credencial.Nombre)
	responderOk(escritor, credencial.SinSecretos())
}

func (a *API) borrarCredencial(escritor http.ResponseWriter, peticion *http.Request) {
	id, err := strconv.ParseInt(peticion.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		a.errorValidacion(escritor, peticion, "Credenciales SNMP", "Borrar",
			"El identificador no es valido.")
		return
	}

	if err := a.Datos.BorrarCredencialSNMP(peticion.Context(), id); err != nil {
		a.errorValidacion(escritor, peticion, "Credenciales SNMP", "Borrar", err.Error())
		return
	}

	a.anotarActividad(peticion, "Credenciales SNMP", "Borrar credencial "+strconv.FormatInt(id, 10))
	responderOk(escritor, map[string]any{"borrada": true})
}

// mapaDePuertos devuelve que hay conectado en cada boca de cada switch, y que
// tan seguro es ese dato en esta red.
func (a *API) mapaDePuertos(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var mapa []basedatos.PuertoDeSwitch
	var enlaces []basedatos.EnlaceEntreEquipos
	var capacidad string
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		mapa, err = base.MapaDePuertos(peticion.Context())
		if err != nil {
			return err
		}
		enlaces, err = base.Enlaces(peticion.Context())
		if err != nil {
			return err
		}
		return base.QueryRowContext(peticion.Context(),
			`SELECT capacidad_mapa_puertos FROM ficha WHERE id = 1`).Scan(&capacidad)
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Mapa de puertos", Accion: "Consultar", Causa: CausaBaseDatos,
			Tabla: "conexiones_puerto", Codigo: http.StatusInternalServerError,
		}, "No se pudo leer el mapa de puertos.", err)
		return
	}

	// La interfaz lo dice sin rodeos en vez de mostrar una pantalla vacia: si no
	// hay mapa, se explica por que y que haria falta para tenerlo.
	responderOk(escritor, map[string]any{
		"capacidad":   capacidad,
		"explicacion": explicarCapacidad(capacidad),
		"puertos":     mapa,
		"enlaces":     enlaces,
	})
}

func explicarCapacidad(capacidad string) string {
	switch capacidad {
	case basedatos.CapacidadExacta:
		return "En esta red se sabe el puerto exacto de cada equipo, porque los switches son " +
			"administrables y contestan SNMP."
	case basedatos.CapacidadPorGrupo:
		return "En esta red se sabe en que boca cuelga cada grupo de equipos, pero no el puerto " +
			"exacto de cada uno: detras de esas bocas hay switches no administrables."
	case basedatos.CapacidadNoDisponible:
		return "En esta red no se puede saber el puerto de cada equipo: ningun switch contesto " +
			"SNMP. Se sigue teniendo el inventario, la presencia y las alertas. Para tener el mapa " +
			"de puertos hace falta al menos un switch administrable con SNMP habilitado y su " +
			"credencial cargada en MiRed."
	default:
		return "Todavia no se ha consultado a los switches de esta red. Cargue una credencial SNMP " +
			"y corra un escaneo completo."
	}
}
