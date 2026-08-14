package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/tuxormax/mired/internal/autenticacion"
	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/catalogo"
)

// La edicion manual de la topologia: lo que el usuario declara porque tiene el
// cable delante y ninguna herramienta puede averiguarlo.
//
// Todas las rutas cuelgan de /api/redes/{clave}/ porque lo declarado vive en la
// base de ESA red, y `conRed` es lo unico que fija la red activa en el contexto.
// Una ruta suelta como /puertos/{id} no sabria a que archivo escribir.
//
// Lo escribe solo mired-servidor, sin excepcion a la regla de un unico escritor.

// exigeEscritura corta la peticion si el usuario solo puede mirar esta red.
func (a *API) exigeEscritura(escritor http.ResponseWriter, peticion *http.Request, accion string) bool {
	nivel, _ := autenticacion.NivelDe(peticion.Context())
	if autenticacion.PuedeEscribir(nivel) {
		return true
	}
	a.responderError(escritor, peticion, contextoError{
		Modulo: "Topologia manual", Accion: accion, Causa: CausaPermiso,
		Codigo: http.StatusForbidden,
	}, "Necesita permiso de escritura sobre esta red.", nil)
	return false
}

// verTopologiaManual devuelve las bocas declaradas, sus cables y donde eso no
// cuadra con lo que reportan los equipos.
func (a *API) verTopologiaManual(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var topologia basedatos.TopologiaManual
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		topologia, err = base.LeerTopologiaManual(peticion.Context())
		return err
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Topologia manual", Accion: "Consultar", Causa: CausaBaseDatos,
			Tabla: "puertos_fisicos", Codigo: http.StatusInternalServerError,
		}, "No se pudo leer lo que hay declarado a mano.", err)
		return
	}

	responderOk(escritor, topologia)
}

// composicionDeLaRed responde de que esta hecha la red: cuantos equipos hay en
// total y cuantos de cada tipo.
//
// Sale de la MISMA tabla que la lista de equipos y que el mapa, asi que los tres
// no pueden discrepar. Un switch declarado a mano cuenta aqui en cuanto se
// declara: no hay nada que sincronizar ni que verificar despues.
func (a *API) composicionDeLaRed(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var resumen []basedatos.CuentaPorCategoria
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		resumen, err = base.ResumenDeCategorias(peticion.Context(), catalogo.SinReconocer)
		return err
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Equipos", Accion: "Contar", Causa: CausaBaseDatos,
			Tabla: "equipos", Codigo: http.StatusInternalServerError,
		}, "No se pudo contar de que esta hecha la red.", err)
		return
	}

	total, presentes, declarados := 0, 0, 0
	for _, cuenta := range resumen {
		total += cuenta.Cuantos
		presentes += cuenta.Presentes
		declarados += cuenta.Declarados
	}

	responderOk(escritor, map[string]any{
		"total":     total,
		"presentes": presentes,
		// Cuantos los puso una persona. Se dice aparte porque no los vio ningun
		// escaneo: la cuenta es igual de real, pero no viene de una medicion.
		"declarados": declarados,
		"categorias": resumen,
	})
}

// crearEquipoManual da de alta un aparato que ningun barrido va a encontrar: el
// switch tonto, el modem que no habla SNMP hacia la LAN.
func (a *API) crearEquipoManual(escritor http.ResponseWriter, peticion *http.Request) {
	if !a.exigeEscritura(escritor, peticion, "Crear equipo") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var cuerpo basedatos.EquipoManual
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Topologia manual", "Crear equipo") {
		return
	}

	var creado basedatos.Equipo
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		creado, err = base.CrearEquipoManual(peticion.Context(), cuerpo)
		return err
	})
	if errors.Is(err, basedatos.ErrEquipoRepetido) {
		a.errorValidacion(escritor, peticion, "Topologia manual", "Crear equipo",
			"Ya hay un equipo con ese nombre en esta red.")
		return
	}
	if err != nil {
		// La capa de datos valida en lenguaje llano; lo que llegue aqui sin ser de
		// validacion se trata como fallo de base.
		a.errorValidacion(escritor, peticion, "Topologia manual", "Crear equipo", err.Error())
		return
	}

	a.anotarActividad(peticion, "Topologia manual", "Crear equipo "+creado.ComoSeLlama()+" en "+clave)
	responderOk(escritor, creado)
}

// guardarFicha escribe lo que una persona sabe del equipo: modelo, notas y si
// esta por cable o por WiFi. Sirve igual para lo descubierto y para lo
// declarado; por eso NO es una ruta de "equipos manuales".
func (a *API) guardarFicha(escritor http.ResponseWriter, peticion *http.Request) {
	if !a.exigeEscritura(escritor, peticion, "Editar ficha") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	id, ok := a.leerIDDeRuta(escritor, peticion, "equipo", "Editar ficha", "El equipo no es valido.")
	if !ok {
		return
	}

	var cuerpo basedatos.FichaEquipo
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Topologia manual", "Editar ficha") {
		return
	}

	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		return base.ActualizarFicha(peticion.Context(), id, cuerpo)
	})
	if errors.Is(err, basedatos.ErrEquipoNoExiste) {
		a.errorValidacion(escritor, peticion, "Topologia manual", "Editar ficha",
			"Ese equipo no existe.")
		return
	}
	if err != nil {
		a.errorValidacion(escritor, peticion, "Topologia manual", "Editar ficha", err.Error())
		return
	}

	a.anotarActividad(peticion, "Topologia manual", fmt.Sprintf("Editar ficha del equipo %d de %s", id, clave))
	responderOk(escritor, map[string]any{"guardada": true})
}

// borrarEquipoManual quita un aparato declarado. Los descubiertos NO se borran:
// se marcan ausentes, porque su historia es lo que despues permite avisar de que
// llevan dias sin aparecer.
func (a *API) borrarEquipoManual(escritor http.ResponseWriter, peticion *http.Request) {
	if !a.exigeEscritura(escritor, peticion, "Borrar equipo") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	id, ok := a.leerIDDeRuta(escritor, peticion, "equipo", "Borrar equipo", "El equipo no es valido.")
	if !ok {
		return
	}

	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		return base.BorrarEquipoManual(peticion.Context(), id)
	})
	if errors.Is(err, basedatos.ErrEquipoNoExiste) {
		a.errorValidacion(escritor, peticion, "Topologia manual", "Borrar equipo",
			"Ese equipo no existe, o no se declaro a mano: los descubiertos no se borran.")
		return
	}
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Topologia manual", Accion: "Borrar equipo", Causa: CausaBaseDatos,
			Tabla: "equipos", Codigo: http.StatusInternalServerError,
		}, "No se pudo borrar el equipo.", err)
		return
	}

	a.anotarActividad(peticion, "Topologia manual", fmt.Sprintf("Borrar equipo manual %d de %s", id, clave))
	responderOk(escritor, map[string]any{"borrado": true})
}

// ---------------------------------------------------------------- bocas --

func (a *API) agregarPuertoFisico(escritor http.ResponseWriter, peticion *http.Request) {
	if !a.exigeEscritura(escritor, peticion, "Agregar puerto") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	equipoID, ok := a.leerIDDeRuta(escritor, peticion, "equipo", "Agregar puerto", "El equipo no es valido.")
	if !ok {
		return
	}

	var cuerpo basedatos.PuertoFisico
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Topologia manual", "Agregar puerto") {
		return
	}
	cuerpo.EquipoID = equipoID

	var creado basedatos.PuertoFisico
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		creado, err = base.AgregarPuertoFisico(peticion.Context(), cuerpo)
		return err
	})
	if errors.Is(err, basedatos.ErrPuertoRepetido) {
		a.errorValidacion(escritor, peticion, "Topologia manual", "Agregar puerto",
			"Ese equipo ya tiene declarada esa boca.")
		return
	}
	if err != nil {
		a.errorValidacion(escritor, peticion, "Topologia manual", "Agregar puerto", err.Error())
		return
	}

	a.anotarActividad(peticion, "Topologia manual",
		fmt.Sprintf("Declarar puerto %d del equipo %d de %s", creado.Numero, equipoID, clave))
	responderOk(escritor, creado)
}

func (a *API) editarPuertoFisico(escritor http.ResponseWriter, peticion *http.Request) {
	if !a.exigeEscritura(escritor, peticion, "Editar puerto") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	id, ok := a.leerIDDeRuta(escritor, peticion, "puerto", "Editar puerto", "El puerto no es valido.")
	if !ok {
		return
	}

	var cuerpo basedatos.PuertoFisico
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Topologia manual", "Editar puerto") {
		return
	}
	cuerpo.ID = id

	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		return base.EditarPuertoFisico(peticion.Context(), cuerpo)
	})
	if errors.Is(err, basedatos.ErrPuertoRepetido) {
		a.errorValidacion(escritor, peticion, "Topologia manual", "Editar puerto",
			"Ese equipo ya tiene declarada esa boca.")
		return
	}
	if errors.Is(err, basedatos.ErrPuertoNoExiste) {
		a.errorValidacion(escritor, peticion, "Topologia manual", "Editar puerto",
			"Ese puerto ya no existe.")
		return
	}
	if err != nil {
		a.errorValidacion(escritor, peticion, "Topologia manual", "Editar puerto", err.Error())
		return
	}

	a.anotarActividad(peticion, "Topologia manual", fmt.Sprintf("Editar puerto %d de %s", id, clave))
	responderOk(escritor, map[string]any{"guardado": true})
}

func (a *API) borrarPuertoFisico(escritor http.ResponseWriter, peticion *http.Request) {
	if !a.exigeEscritura(escritor, peticion, "Borrar puerto") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	id, ok := a.leerIDDeRuta(escritor, peticion, "puerto", "Borrar puerto", "El puerto no es valido.")
	if !ok {
		return
	}

	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		return base.BorrarPuertoFisico(peticion.Context(), id)
	})
	if errors.Is(err, basedatos.ErrPuertoNoExiste) {
		a.errorValidacion(escritor, peticion, "Topologia manual", "Borrar puerto",
			"Ese puerto ya no existe.")
		return
	}
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Topologia manual", Accion: "Borrar puerto", Causa: CausaBaseDatos,
			Tabla: "puertos_fisicos", Codigo: http.StatusInternalServerError,
		}, "No se pudo borrar el puerto.", err)
		return
	}

	a.anotarActividad(peticion, "Topologia manual", fmt.Sprintf("Borrar puerto %d de %s", id, clave))
	responderOk(escritor, map[string]any{"borrado": true})
}

// --------------------------------------------------------------- cables --

// crearEnlaceManual declara un cable. Siempre entra con origen_dato = manual: la
// unica forma de que la base tenga un enlace 'snmp' es que lo escriba el propio
// escaneo, no una peticion de la interfaz.
func (a *API) crearEnlaceManual(escritor http.ResponseWriter, peticion *http.Request) {
	if !a.exigeEscritura(escritor, peticion, "Conectar") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var cuerpo basedatos.EnlaceFisico
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Topologia manual", "Conectar") {
		return
	}

	var creado basedatos.EnlaceFisico
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		creado, err = base.CrearEnlaceManual(peticion.Context(), cuerpo)
		return err
	})
	switch {
	case errors.Is(err, basedatos.ErrPuertoNoExiste):
		a.errorValidacion(escritor, peticion, "Topologia manual", "Conectar",
			"Alguna de las dos bocas ya no existe.")
		return
	case errors.Is(err, basedatos.ErrEquipoNoExiste):
		a.errorValidacion(escritor, peticion, "Topologia manual", "Conectar",
			"El equipo al que quiere conectar ya no existe.")
		return
	case errors.Is(err, basedatos.ErrEnlaceAlReves):
		a.errorValidacion(escritor, peticion, "Topologia manual", "Conectar",
			"Un equipo no se puede conectar consigo mismo.")
		return
	case err != nil:
		a.errorValidacion(escritor, peticion, "Topologia manual", "Conectar", err.Error())
		return
	}

	a.anotarActividad(peticion, "Topologia manual",
		fmt.Sprintf("Conectar la boca %d en %s", creado.PuertoOrigenID, clave))
	responderOk(escritor, creado)
}

func (a *API) borrarEnlaceManual(escritor http.ResponseWriter, peticion *http.Request) {
	if !a.exigeEscritura(escritor, peticion, "Desconectar") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	id, ok := a.leerIDDeRuta(escritor, peticion, "enlace", "Desconectar", "El enlace no es valido.")
	if !ok {
		return
	}

	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		return base.BorrarEnlaceFisico(peticion.Context(), id)
	})
	if errors.Is(err, basedatos.ErrEnlaceNoExiste) {
		a.errorValidacion(escritor, peticion, "Topologia manual", "Desconectar",
			"Ese cable ya no existe.")
		return
	}
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Topologia manual", Accion: "Desconectar", Causa: CausaBaseDatos,
			Tabla: "enlaces_fisicos", Codigo: http.StatusInternalServerError,
		}, "No se pudo quitar el cable.", err)
		return
	}

	a.anotarActividad(peticion, "Topologia manual", fmt.Sprintf("Desconectar el cable %d de %s", id, clave))
	responderOk(escritor, map[string]any{"borrado": true})
}

// leerIDDeRuta saca un identificador de la ruta y responde con un mensaje de negocio
// si no sirve, en vez de dejar que la consulta reviente mas adentro.
func (a *API) leerIDDeRuta(escritor http.ResponseWriter, peticion *http.Request,
	campo, accion, mensaje string) (int64, bool) {
	id, err := strconv.ParseInt(peticion.PathValue(campo), 10, 64)
	if err != nil || id <= 0 {
		a.errorValidacion(escritor, peticion, "Topologia manual", accion, mensaje)
		return 0, false
	}
	return id, true
}
