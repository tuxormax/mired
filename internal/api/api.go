// Paquete api publica el servicio HTTP de MiRed: las rutas que consume la
// interfaz Flutter y la entrega de la propia interfaz ya compilada.
//
// Toda respuesta usa el mismo sobre (ver Respuesta) y todo error sale con el
// bloque de detalles de la casa, para que soporte no dependa de que el usuario
// sepa contar que paso.
package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/tuxormax/mired/internal/autenticacion"
	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/catalogo"
	"github.com/tuxormax/mired/internal/programador"
	"github.com/tuxormax/mired/internal/secreto"
)

// API reune lo que necesitan los manejadores.
type API struct {
	Datos    *basedatos.Enrutador
	Auth     *autenticacion.Servicio
	Bitacora *slog.Logger
	// Seguro marca las cookies como Secure. Se activa cuando MiRed se sirve
	// detras de HTTPS.
	Seguro bool
	// SocketSonda es por donde se le pregunta a mired-sonda.
	SocketSonda string
	// Programador lanza los barridos, pedidos o por agenda.
	Programador *programador.Servicio
	// Secretos cifra las claves de los equipos. La llave vive con la
	// configuracion y NO con los datos, para que un respaldo de la base no
	// venga con que abrirse.
	Secretos *secreto.Caja

	// Catalogo reconoce que es cada aparato. Puede ser nil.
	Catalogo *catalogo.Catalogo
	// CarpetasCatalogo son las carpetas de donde salio, para poder releerlas al
	// guardar una definicion o al traer las de la comunidad.
	CarpetasCatalogo []string
	// CarpetaPropia es donde se ESCRIBE lo que se declara aqui, y
	// CarpetaComunidad donde se deja lo que se baja del repositorio. Corriendo
	// como programa no se puede escribir en /etc ni en /var/lib: van a la
	// carpeta del usuario.
	CarpetaPropia    string
	CarpetaComunidad string
}

// Rutas arma el enrutador HTTP completo.
func (a *API) Rutas() http.Handler {
	mux := http.NewServeMux()

	// Publico: sirve para que la interfaz sepa a que version se conecto y para
	// comprobar desde fuera que el servicio esta vivo.
	mux.HandleFunc("GET /api/estado", a.estado)

	// Sesion.
	mux.HandleFunc("POST /api/sesion", a.iniciarSesion)
	// Sin sesion a proposito: es la puerta del primer acceso, y se cierra sola
	// en cuanto existe el primer usuario.
	mux.HandleFunc("POST /api/primer-administrador", a.crearPrimerAdministrador)
	mux.HandleFunc("DELETE /api/sesion", a.cerrarSesion)
	mux.Handle("GET /api/sesion", a.conSesion(a.sesionActual))
	mux.Handle("GET /api/versiones", a.conSesion(a.historialVersiones))

	// Redes.
	mux.Handle("GET /api/redes", a.conSesion(a.listarRedes))
	mux.Handle("POST /api/redes", a.conSesion(a.crearRed))
	mux.Handle("GET /api/redes/{clave}", a.conRed(a.verRed))
	mux.Handle("PATCH /api/redes/{clave}", a.conRed(a.editarRed))
	mux.Handle("DELETE /api/redes/{clave}", a.conRed(a.borrarRed))
	mux.Handle("GET /api/redes/{clave}/subredes", a.conRed(a.listarSubredes))
	mux.Handle("POST /api/redes/{clave}/subredes", a.conRed(a.crearSubred))

	// Escaneo e inventario de equipos.
	mux.Handle("POST /api/redes/{clave}/escaneos", a.conRed(a.lanzarEscaneo))
	mux.Handle("GET /api/redes/{clave}/escaneos", a.conRed(a.listarEscaneos))
	mux.Handle("PUT /api/redes/{clave}/agenda", a.conRed(a.configurarAgenda))
	mux.Handle("GET /api/redes/{clave}/equipos", a.conRed(a.listarEquipos))
	mux.Handle("PATCH /api/redes/{clave}/equipos/{equipo}", a.conRed(a.ponerAlias))
	mux.Handle("GET /api/redes/{clave}/equipos/{equipo}/presencia", a.conRed(a.listarPresencia))

	// Sonda: si esta viva y con que permisos. La interfaz lo usa para avisar
	// por que el escaneo no esta disponible, en vez de dejar pantallas vacias.
	mux.Handle("GET /api/sonda", a.conSesion(a.estadoSonda))

	mux.Handle("GET /api/redes/{clave}/mapa-puertos", a.conRed(a.mapaDePuertos))

	// Topologia declarada a mano: la tercera fuente del mapa, junto al enlace
	// confirmado por SNMP y al inferido por conteo de MAC. Cuelga de la red
	// porque lo declarado vive en la base de ESA red.
	mux.Handle("GET /api/redes/{clave}/topologia-manual", a.conRed(a.verTopologiaManual))
	// De que esta hecha la red: el total y cuantos de cada tipo.
	mux.Handle("GET /api/redes/{clave}/composicion", a.conRed(a.composicionDeLaRed))
	mux.Handle("POST /api/redes/{clave}/equipos", a.conRed(a.crearEquipoManual))
	mux.Handle("PUT /api/redes/{clave}/equipos/{equipo}", a.conRed(a.guardarFicha))
	mux.Handle("DELETE /api/redes/{clave}/equipos/{equipo}", a.conRed(a.borrarEquipoManual))
	mux.Handle("POST /api/redes/{clave}/equipos/{equipo}/puertos", a.conRed(a.agregarPuertoFisico))
	mux.Handle("PUT /api/redes/{clave}/puertos/{puerto}", a.conRed(a.editarPuertoFisico))
	mux.Handle("DELETE /api/redes/{clave}/puertos/{puerto}", a.conRed(a.borrarPuertoFisico))
	mux.Handle("POST /api/redes/{clave}/enlaces", a.conRed(a.crearEnlaceManual))
	mux.Handle("DELETE /api/redes/{clave}/enlaces/{enlace}", a.conRed(a.borrarEnlaceManual))

	// Alertas: lo que convierte el inventario en algo que avisa.
	mux.Handle("GET /api/redes/{clave}/alertas", a.conRed(a.listarAlertas))
	mux.Handle("POST /api/redes/{clave}/alertas/vistas", a.conRed(a.marcarAlertasVistas))
	mux.Handle("GET /api/redes/{clave}/reglas", a.conRed(a.listarReglas))
	mux.Handle("PUT /api/redes/{clave}/reglas/{tipo}", a.conRed(a.guardarRegla))
	mux.Handle("GET /api/redes/{clave}/destinos", a.conRed(a.listarDestinos))
	mux.Handle("POST /api/redes/{clave}/destinos", a.conRed(a.crearDestino))
	mux.Handle("DELETE /api/redes/{clave}/destinos/{destino}", a.conRed(a.borrarDestino))

	// Ancho de banda por los contadores del switch.
	mux.Handle("GET /api/redes/{clave}/consumo", a.conRed(a.consumoActual))
	mux.Handle("GET /api/redes/{clave}/consumo/{switch}/{puerto}", a.conRed(a.historialTrafico))
	mux.Handle("GET /api/redes/{clave}/aplicaciones", a.conRed(a.consumoPorAplicacion))

	// Parar la agenda mientras alguien edita el mapa: ahi se declara cableado,
	// no se mide la red, y un barrido por debajo solo gasta el equipo.
	mux.Handle("POST /api/redes/{clave}/pausa", a.conRed(a.pausarAgenda))
	mux.Handle("DELETE /api/redes/{clave}/pausa", a.conRed(a.reanudarAgenda))

	// Lo que cuelga de una antena por el aire. El WiFi no tiene puertos: a una
	// antena se le cuelgan uno o VARIOS equipos de una sola vez.
	mux.Handle("POST /api/redes/{clave}/inalambricos", a.conRed(a.colgarPorWiFi))
	mux.Handle("DELETE /api/redes/{clave}/inalambricos/{enlace}", a.conRed(a.descolgarDeWiFi))

	// Las credenciales de cada equipo. La clave solo sale por verClaveDeEquipo,
	// que exige escritura y deja rastro en la bitacora.
	mux.Handle("PUT /api/redes/{clave}/equipos/{equipo}/credencial", a.conRed(a.guardarCredencialEquipo))
	mux.Handle("GET /api/redes/{clave}/equipos/{equipo}/credencial/clave", a.conRed(a.verClaveDeEquipo))
	mux.Handle("DELETE /api/redes/{clave}/credenciales/{credencial}", a.conRed(a.borrarCredencialEquipo))

	// El aire: que redes inalambricas se oyen desde este equipo. Va con la red
	// activa porque el cruce con el inventario —de que aparato es cada antena—
	// se hace contra los equipos de esa red.
	mux.Handle("GET /api/redes/{clave}/aire", a.conRed(a.barrerAire))

	// Catalogo abierto de dispositivos.
	mux.Handle("GET /api/catalogo", a.conSesion(a.listarCatalogo))
	// Aportar al catalogo cambia como se reconoce a TODA la red, en todas las
	// redes: por eso lo hace el superadministrador, igual que las credenciales.
	mux.Handle("POST /api/catalogo/dispositivos", a.conSuperadmin(a.guardarDefinicion))
	mux.Handle("POST /api/catalogo/actualizar", a.conSuperadmin(a.actualizarCatalogo))
	mux.Handle("GET /api/redes/{clave}/equipos/{equipo}/propuesta", a.conRed(a.proponerDefinicion))

	// Credenciales SNMP: son secretos compartidos entre redes, asi que las
	// administra solo el superadministrador.
	mux.Handle("GET /api/credenciales-snmp", a.conSuperadmin(a.listarCredenciales))
	mux.Handle("POST /api/credenciales-snmp", a.conSuperadmin(a.crearCredencial))
	mux.Handle("DELETE /api/credenciales-snmp/{id}", a.conSuperadmin(a.borrarCredencial))
	mux.Handle("GET /api/controladoras", a.conSuperadmin(a.listarControladoras))
	mux.Handle("POST /api/controladoras", a.conSuperadmin(a.crearControladora))
	mux.Handle("DELETE /api/controladoras/{id}", a.conSuperadmin(a.borrarControladora))

	// Usuarios y permisos: solo el superadministrador.
	mux.Handle("GET /api/usuarios", a.conSuperadmin(a.listarUsuarios))
	mux.Handle("POST /api/usuarios", a.conSuperadmin(a.crearUsuario))
	mux.Handle("DELETE /api/usuarios/{id}", a.conSuperadmin(a.borrarUsuario))
	mux.Handle("PUT /api/usuarios/{id}/permisos/{red}", a.conSuperadmin(a.otorgarPermiso))
	mux.Handle("DELETE /api/usuarios/{id}/permisos/{red}", a.conSuperadmin(a.quitarPermiso))

	// Bitacora de errores de la interfaz y de la sonda.
	mux.HandleFunc("POST /api/log-error", a.recibirError)

	// La interfaz. Va al final: cualquier ruta que no sea /api/ la atiende el
	// sitio compilado.
	// La raiz NO sirve ninguna pagina: MiRed es un programa de escritorio y esto
	// es solo su motor. Quien llegue aqui con un navegador se lleva una
	// explicacion, no un error a secas.
	mux.HandleFunc("/", a.noHayPaginaWeb)

	return conRegistro(a.Bitacora, mux)
}

// conSesion exige una sesion valida y deja al usuario en el contexto.
func (a *API) conSesion(manejador http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(escritor http.ResponseWriter, peticion *http.Request) {
		usuario, _, err := a.Auth.Identificar(peticion)
		if err != nil {
			a.responderError(escritor, peticion, contextoError{
				Modulo: "Sesion", Accion: "Identificar", Causa: CausaSesion,
				Codigo: http.StatusUnauthorized,
			}, "Su sesion no esta activa o ya vencio. Vuelva a entrar.", nil)
			return
		}
		manejador(escritor, peticion.WithContext(autenticacion.ConUsuario(peticion.Context(), usuario)))
	})
}

// conSuperadmin exige sesion de superadministrador.
func (a *API) conSuperadmin(manejador http.HandlerFunc) http.Handler {
	return a.conSesion(func(escritor http.ResponseWriter, peticion *http.Request) {
		usuario, _ := autenticacion.UsuarioDe(peticion.Context())
		if !usuario.Superadmin {
			a.responderError(escritor, peticion, contextoError{
				Modulo: "Usuarios", Accion: "Administrar", Causa: CausaPermiso,
				Codigo: http.StatusForbidden,
			}, "Esta pantalla es solo para el administrador del sistema.", nil)
			return
		}
		manejador(escritor, peticion)
	})
}

// conRed exige sesion Y permiso sobre la red de la ruta, y deja esa red fijada
// como RED ACTIVA en el contexto.
//
// Esta es la pieza central del diseno de "una base por red": a partir de aqui,
// cualquier consulta operativa sabe sola a que archivo va, sin que ninguna
// funcion tenga que recibir la red por parametro.
func (a *API) conRed(manejador http.HandlerFunc) http.Handler {
	return a.conSesion(func(escritor http.ResponseWriter, peticion *http.Request) {
		usuario, _ := autenticacion.UsuarioDe(peticion.Context())
		clave := peticion.PathValue("clave")

		nivel, err := a.Auth.PermisoSobre(peticion.Context(), usuario, clave)
		if errors.Is(err, autenticacion.ErrSinPermiso) {
			a.responderError(escritor, peticion, contextoError{
				Modulo: "Redes", Accion: "Abrir red", Causa: CausaPermiso,
				Codigo: http.StatusForbidden,
			}, "No tiene acceso a esta red.", nil)
			return
		}
		if err != nil {
			a.responderError(escritor, peticion, contextoError{
				Modulo: "Redes", Accion: "Abrir red", Causa: CausaBaseDatos,
				Tabla: "permisos_red", Codigo: http.StatusInternalServerError,
			}, "No se pudieron comprobar sus permisos sobre esta red.", err)
			return
		}

		// La red existe y es visible: se comprueba antes de fijarla como activa
		// para no dejar que el resto del codigo trabaje sobre una red borrada.
		if _, err := a.Datos.BuscarRed(peticion.Context(), clave); err != nil {
			a.responderError(escritor, peticion, contextoError{
				Modulo: "Redes", Accion: "Abrir red", Causa: CausaValidacion,
				Tabla: "redes", Codigo: http.StatusNotFound,
			}, "Esa red no existe o fue borrada.", err)
			return
		}

		ctx := autenticacion.ConRedActiva(peticion.Context(), clave, nivel)
		manejador(escritor, peticion.WithContext(ctx))
	})
}

// noHayPaginaWeb explica a quien llegue con un navegador que aqui no hay nada
// que mirar.
//
// **MiRed no tiene interfaz web.** Es un programa de escritorio; esto es su
// motor, y habla HTTP solo porque es la forma mas simple de que el programa y
// los servicios se entiendan, incluso a traves de la red cuando se quiere ver
// otro equipo. Devolver un 404 seco haria pensar que algo se rompio.
func (a *API) noHayPaginaWeb(escritor http.ResponseWriter, peticion *http.Request) {
	if strings.HasPrefix(peticion.URL.Path, "/api/") {
		http.NotFound(escritor, peticion)
		return
	}
	escritor.Header().Set("Content-Type", "text/plain; charset=utf-8")
	escritor.WriteHeader(http.StatusNotFound)
	escritor.Write([]byte(
		"MiRed no se usa desde el navegador: es un programa de escritorio.\n\n" +
			"Abralo desde el menu de aplicaciones, o escriba  mired  en una terminal.\n" +
			"Para conectarse a ESTE equipo desde otro, abra el programa alla y ponga\n" +
			"esta direccion en el boton del servidor.\n\n" +
			"Esto de aqui es el motor, y si funciona: pruebe /api/estado\n"))
}

// conRegistro anota cada peticion en la bitacora, con su duracion.
func conRegistro(bitacora *slog.Logger, siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(escritor http.ResponseWriter, peticion *http.Request) {
		inicio := time.Now()
		vigilado := &respuestaVigilada{ResponseWriter: escritor, codigo: http.StatusOK}
		siguiente.ServeHTTP(vigilado, peticion)

		// El detalle de cada peticion solo estorba salvo cuando se depura.
		bitacora.Debug("peticion",
			"metodo", peticion.Method,
			"ruta", peticion.URL.Path,
			"codigo", vigilado.codigo,
			"ms", time.Since(inicio).Milliseconds())
	})
}

type respuestaVigilada struct {
	http.ResponseWriter
	codigo int
}

func (r *respuestaVigilada) WriteHeader(codigo int) {
	r.codigo = codigo
	r.ResponseWriter.WriteHeader(codigo)
}
