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
)

// API reune lo que necesitan los manejadores.
type API struct {
	Datos    *basedatos.Enrutador
	Auth     *autenticacion.Servicio
	Bitacora *slog.Logger
	// RutaWeb es la carpeta con la interfaz Flutter compilada. Si esta vacia o
	// no existe, el servicio funciona igual: solo responde la API.
	RutaWeb string
	// Seguro marca las cookies como Secure. Se activa cuando MiRed se sirve
	// detras de HTTPS.
	Seguro bool
	// SocketSonda es por donde se le pregunta a mired-sonda.
	SocketSonda string
}

// Rutas arma el enrutador HTTP completo.
func (a *API) Rutas() http.Handler {
	mux := http.NewServeMux()

	// Publico: sirve para que la interfaz sepa a que version se conecto y para
	// comprobar desde fuera que el servicio esta vivo.
	mux.HandleFunc("GET /api/estado", a.estado)

	// Sesion.
	mux.HandleFunc("POST /api/sesion", a.iniciarSesion)
	mux.HandleFunc("DELETE /api/sesion", a.cerrarSesion)
	mux.Handle("GET /api/sesion", a.conSesion(a.sesionActual))

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
	mux.Handle("GET /api/redes/{clave}/equipos", a.conRed(a.listarEquipos))
	mux.Handle("PATCH /api/redes/{clave}/equipos/{equipo}", a.conRed(a.ponerAlias))

	// Sonda: si esta viva y con que permisos. La interfaz lo usa para avisar
	// por que el escaneo no esta disponible, en vez de dejar pantallas vacias.
	mux.Handle("GET /api/sonda", a.conSesion(a.estadoSonda))

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
	mux.Handle("/", a.servirInterfaz())

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

// servirInterfaz entrega la interfaz Flutter compilada, con retorno al index
// para que las rutas internas de la aplicacion funcionen al recargar.
func (a *API) servirInterfaz() http.Handler {
	if a.RutaWeb == "" {
		return http.HandlerFunc(a.sinInterfaz)
	}
	archivos := http.FileServer(http.Dir(a.RutaWeb))

	return http.HandlerFunc(func(escritor http.ResponseWriter, peticion *http.Request) {
		if strings.HasPrefix(peticion.URL.Path, "/api/") {
			http.NotFound(escritor, peticion)
			return
		}
		archivos.ServeHTTP(escritor, peticion)
	})
}

func (a *API) sinInterfaz(escritor http.ResponseWriter, peticion *http.Request) {
	if strings.HasPrefix(peticion.URL.Path, "/api/") {
		http.NotFound(escritor, peticion)
		return
	}
	escritor.Header().Set("Content-Type", "text/plain; charset=utf-8")
	escritor.WriteHeader(http.StatusServiceUnavailable)
	escritor.Write([]byte("La interfaz de MiRed no esta instalada en este servidor.\n" +
		"El servicio funciona: pruebe /api/estado\n"))
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
