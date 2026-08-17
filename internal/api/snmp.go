package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/tuxormax/mired/internal/autenticacion"
	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/snmp"
	"github.com/tuxormax/mired/internal/sonda"
)

// listarCredenciales devuelve las credenciales SNMP SIN sus secretos.
//
// La comunidad SNMP es en la practica una contrasena: no tiene por que viajar de
// vuelta al navegador solo para pintar una lista.
func (a *API) listarCredenciales(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var credenciales []basedatos.CredencialSNMP
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		credenciales, err = base.ListarCredencialesSNMP(peticion.Context())
		return err
	})
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

	if !a.exigeEscritura(escritor, peticion, "Crear credencial") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var credencial basedatos.CredencialSNMP
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		credencial, err = base.CrearCredencialSNMP(peticion.Context(), cuerpo)
		return err
	})
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

	if !a.exigeEscritura(escritor, peticion, "Borrar credencial") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	err = a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		return base.BorrarCredencialSNMP(peticion.Context(), id)
	})
	if err != nil {
		a.errorValidacion(escritor, peticion, "Credenciales SNMP", "Borrar", err.Error())
		return
	}

	a.anotarActividad(peticion, "Credenciales SNMP", "Borrar credencial "+strconv.FormatInt(id, 10))
	responderOk(escritor, map[string]any{"borrada": true})
}

// mapaDePuertos devuelve que hay conectado en cada puerto de cada switch, y que
// tan seguro es ese dato en esta red.
func (a *API) mapaDePuertos(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var mapa []basedatos.PuertoDeSwitch
	var enlaces []basedatos.EnlaceEntreEquipos
	var capacidad, momento string
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		mapa, err = base.MapaDePuertos(peticion.Context())
		if err != nil {
			return err
		}
		// De cuando son estos datos. Va con el mapa para que lo exportado pueda
		// decir de que momento es, no solo cuando se guardo el archivo.
		momento, err = base.MomentoDelMapa(peticion.Context())
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
		"momento":     momento,
	})
}

// explicarCapacidad dice, sin jerga, que se puede saber en esta red y que no.
//
// El texto depende del punto en el que se esta, y eso importa mas de lo que
// parece: antes decia siempre "cargue una credencial SNMP", que le pedia al
// usuario un paso que en una red de casa **no existe** —ahi no hay switches
// administrables a los que preguntarles nada— y le hacia creer que le faltaba
// algo por hacer.
//
// Ahora MiRed lo averigua solo: en cada escaneo prueba la comunidad de fabrica
// contra los equipos que encontro. Si nadie contesta, la respuesta es que son
// switches simples, y se dice tal cual.
func explicarCapacidad(capacidad string) string {
	switch capacidad {
	case basedatos.CapacidadExacta:
		return "En esta red se sabe el puerto exacto de cada equipo: sus switches son " +
			"administrables y contestaron."
	case basedatos.CapacidadPorGrupo:
		return "Se sabe en que puerto cuelga cada grupo de equipos, pero no el puerto exacto de " +
			"cada uno: detras de esos puertos hay switches simples, que no pueden decir mas."
	case basedatos.CapacidadNoDisponible:
		return "Sus switches son simples: no pueden decir que hay enchufado en cada puerto, y por " +
			"eso no hay mapa de puertos. Es lo normal en una red de casa o de una oficina " +
			"chica. Todo lo demas de MiRed funciona igual: el inventario, quien esta " +
			"encendido, los puertos de cada equipo, las alertas y el consumo."
	default:
		return "Todavia no se ha escaneado esta red. Al escanear, MiRed averigua solo si sus " +
			"switches pueden decir que hay en cada puerto."
	}
}

// aplicaciones responde "en que se gasta el ancho de banda", no solo "cuanto".
//
// Lo llena el paquete OPCIONAL mired-dpi. Si no esta instalado la lista viene
// vacia, y la interfaz lo explica en vez de dejar una pantalla en blanco.
func (a *API) consumoPorAplicacion(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	horas := 24
	if valor := peticion.URL.Query().Get("horas"); valor != "" {
		if numero, err := strconv.Atoi(valor); err == nil && numero > 0 && numero <= 720 {
			horas = numero
		}
	}

	var consumo []basedatos.ConsumoPorAplicacion
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		consumo, err = base.ConsumoPorAplicaciones(peticion.Context(), horas)
		return err
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Consumo por aplicacion", Accion: "Consultar", Causa: CausaBaseDatos,
			Tabla: "trafico_aplicaciones", Codigo: http.StatusInternalServerError,
		}, "No se pudo leer el consumo por aplicacion.", err)
		return
	}

	responderOk(escritor, map[string]any{
		"consumo": consumo,
		"explicacion": "Sale del paquete opcional mired-dpi, que mira los paquetes que copia " +
			"el puerto espejo del switch. El nombre no se obtiene descifrando nada: viaja en " +
			"claro en el saludo de TLS, en la cabecera Host de HTTP y en las consultas de DNS.",
		"disponible": len(consumo) > 0,
	})
}

// -------------------------------------------------------- controladoras WiFi --

// Las controladoras son DE CADA RED, como las credenciales SNMP. Estuvieron en
// el catalogo, compartidas, hasta la Rev 44: ver la migracion 0020 de red para
// por que se movieron.

func (a *API) listarControladoras(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var controladoras []basedatos.Controladora
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		controladoras, err = base.ListarControladoras(peticion.Context())
		return err
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Controladoras WiFi", Accion: "Listar", Causa: CausaBaseDatos,
			Tabla: "controladoras", Codigo: http.StatusInternalServerError,
		}, "No se pudieron listar las controladoras.", err)
		return
	}

	publicas := make([]basedatos.Controladora, 0, len(controladoras))
	for _, controladora := range controladoras {
		publicas = append(publicas, controladora.SinSecretos())
	}
	responderOk(escritor, publicas)
}

func (a *API) crearControladora(escritor http.ResponseWriter, peticion *http.Request) {
	var cuerpo basedatos.Controladora
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Controladoras WiFi", "Crear") {
		return
	}

	if !a.exigeEscritura(escritor, peticion, "Crear controladora") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var controladora basedatos.Controladora
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		controladora, err = base.CrearControladora(peticion.Context(), cuerpo)
		return err
	})
	if errors.Is(err, basedatos.ErrControladoraRepetida) {
		a.errorValidacion(escritor, peticion, "Controladoras WiFi", "Crear",
			"Ya existe una controladora con ese nombre.")
		return
	}
	if err != nil {
		a.errorValidacion(escritor, peticion, "Controladoras WiFi", "Crear", err.Error())
		return
	}

	a.anotarActividad(peticion, "Controladoras WiFi", "Crear controladora "+controladora.Nombre)
	responderOk(escritor, controladora.SinSecretos())
}

func (a *API) borrarControladora(escritor http.ResponseWriter, peticion *http.Request) {
	id, err := strconv.ParseInt(peticion.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		a.errorValidacion(escritor, peticion, "Controladoras WiFi", "Borrar",
			"El identificador no es valido.")
		return
	}

	if !a.exigeEscritura(escritor, peticion, "Borrar controladora") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	err = a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		return base.BorrarControladora(peticion.Context(), id)
	})
	if err != nil {
		a.errorValidacion(escritor, peticion, "Controladoras WiFi", "Borrar", err.Error())
		return
	}

	a.anotarActividad(peticion, "Controladoras WiFi", "Borrar controladora")
	responderOk(escritor, map[string]any{"borrada": true})
}

// probarCredencial dice si con esos datos contesta algun equipo de la red.
//
// Es la diferencia entre configurar a ciegas y saber. Quien no sabe que es SNMP
// no tiene forma de comprobar si escribio bien la contrasena del switch: la
// guarda, se va, y tres dias despues el mapa sigue sin puertos sin que nadie le
// haya dicho por que. Aqui se pulsa un boton y contesta «respondieron 2
// switches» o «no contesto ninguno», que es lo unico que hace falta entender.
//
// **No guarda nada.** Se prueba con lo que hay escrito en el formulario, y quien
// prueba decide despues si lo guarda.
func (a *API) probarCredencial(escritor http.ResponseWriter, peticion *http.Request) {
	if !a.exigeEscritura(escritor, peticion, "Probar credencial") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var cuerpo basedatos.CredencialSNMP
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Credenciales SNMP", "Probar") {
		return
	}

	var equipos []basedatos.Equipo
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		equipos, err = base.ListarEquipos(peticion.Context(), false)
		return err
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Credenciales SNMP", Accion: "Probar", Causa: CausaBaseDatos,
			Tabla: "equipos", Codigo: http.StatusInternalServerError,
		}, "No se pudo leer que equipos hay en la red.", err)
		return
	}

	destinos := make([]string, 0, len(equipos))
	for _, equipo := range equipos {
		if equipo.IP != "" {
			destinos = append(destinos, equipo.IP)
		}
	}
	if len(destinos) == 0 {
		// Sin equipos no hay a quien preguntarle, y eso NO es un fallo de la
		// credencial: hay que decirlo distinto o la persona se pone a cambiar una
		// contrasena que estaba bien.
		responderOk(escritor, map[string]any{
			"consultados": 0, "contestaron": 0, "switches": []any{},
			"explicacion": "Todavia no hay equipos en esta red: primero un escaneo, " +
				"y despues se puede probar contra ellos.",
		})
		return
	}

	resultado, err := sonda.PedirSNMP(a.SocketSonda, sonda.PeticionSNMP{
		Destinos:     destinos,
		Credenciales: []snmp.Credencial{aCredencialDeSonda(cuerpo)},
		EsperaMs:     2000,
	}, 60*time.Second)
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Credenciales SNMP", Accion: "Probar", Causa: CausaRed,
			Codigo: http.StatusServiceUnavailable,
		}, "No se pudo hablar con la sonda para hacer la prueba.", err)
		return
	}

	switches := make([]map[string]any, 0, len(resultado.Fichas))
	for _, ficha := range resultado.Fichas {
		switches = append(switches, map[string]any{
			"ip":       ficha.IP,
			"nombre":   ficha.Nombre,
			"esSwitch": ficha.EsSwitch,
			"puertos":  len(ficha.Interfaces),
		})
	}

	responderOk(escritor, map[string]any{
		"consultados": resultado.Consultados,
		"contestaron": len(resultado.Fichas),
		"switches":    switches,
		"explicacion": explicacionDeLaPrueba(len(resultado.Fichas), len(destinos)),
	})
}

// explicacionDeLaPrueba cuenta el resultado como se lo diria una persona a otra.
func explicacionDeLaPrueba(contestaron, consultados int) string {
	switch {
	case contestaron == 0:
		return "No contesto ninguno de los " + itoa(consultados) + " aparatos. O la " +
			"contrasena no es esa, o los switches de esta red no son administrables. " +
			"MiRed funciona igual: lo unico que no podra decir es en que puerto esta " +
			"cada aparato."
	case contestaron == 1:
		return "Contesto 1 aparato. Con esto MiRed ya puede decir que hay conectado en " +
			"cada uno de sus puertos."
	default:
		return "Contestaron " + itoa(contestaron) + " aparatos. Con esto MiRed ya puede " +
			"decir que hay conectado en cada uno de sus puertos."
	}
}

func aCredencialDeSonda(credencial basedatos.CredencialSNMP) snmp.Credencial {
	return snmp.Credencial{
		Nombre:                 credencial.Nombre,
		Version:                credencial.Version,
		Comunidad:              credencial.Comunidad,
		Usuario:                credencial.Usuario,
		AutenticacionProtocolo: credencial.AutenticacionProtocolo,
		AutenticacionClave:     credencial.AutenticacionClave,
		PrivacidadProtocolo:    credencial.PrivacidadProtocolo,
		PrivacidadClave:        credencial.PrivacidadClave,
	}
}
