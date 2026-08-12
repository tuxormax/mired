package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tuxormax/mired/internal/autenticacion"
	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/sonda"
	"github.com/tuxormax/mired/internal/version"
)

// topeCuerpo limita lo que se acepta en un POST. Sin tope, una peticion enorme
// puede tumbar el servicio sin necesidad de ningun truco.
const topeCuerpo = 1 << 20 // 1 MB

// estado dice quien es este servidor. Es publico a proposito: sirve para
// comprobar desde fuera que el servicio esta vivo.
func (a *API) estado(escritor http.ResponseWriter, peticion *http.Request) {
	responderOk(escritor, map[string]any{
		"servicio": "MiRed",
		"version":  version.Numero,
		"revision": version.Revision,
		"build":    version.Build,
	})
}

// ---------------------------------------------------------------- sesion ----

type peticionSesion struct {
	Usuario string `json:"usuario"`
	Clave   string `json:"clave"`
}

func (a *API) iniciarSesion(escritor http.ResponseWriter, peticion *http.Request) {
	var cuerpo peticionSesion
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Sesion", "Entrar") {
		return
	}
	if strings.TrimSpace(cuerpo.Usuario) == "" || cuerpo.Clave == "" {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Sesion", Accion: "Entrar", Causa: CausaValidacion,
			Codigo: http.StatusBadRequest,
		}, "Capture su usuario y su clave.", nil)
		return
	}

	sesion, usuario, err := a.Auth.IniciarSesion(peticion.Context(), cuerpo.Usuario,
		cuerpo.Clave, direccionDe(peticion), peticion.UserAgent())
	if errors.Is(err, autenticacion.ErrCredencialesInvalidas) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Sesion", Accion: "Entrar", Causa: CausaValidacion,
			Codigo: http.StatusUnauthorized,
		}, "Usuario o clave incorrectos.", nil)
		return
	}
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Sesion", Accion: "Entrar", Causa: CausaBaseDatos,
			Tabla: "usuarios", Codigo: http.StatusInternalServerError,
			Estado: "No se pudo entrar. Vuelva a intentarlo.",
		}, "No se pudo iniciar la sesion.", err)
		return
	}

	autenticacion.PonerCookie(escritor, sesion.ID, a.Auth.Duracion, a.Seguro)
	responderOk(escritor, map[string]any{
		"token":   sesion.ID,
		"expira":  sesion.Expira,
		"usuario": usuario,
	})
}

func (a *API) cerrarSesion(escritor http.ResponseWriter, peticion *http.Request) {
	if _, token, err := a.Auth.Identificar(peticion); err == nil {
		a.Auth.CerrarSesion(peticion.Context(), token)
	}
	autenticacion.QuitarCookie(escritor, a.Seguro)
	responderOk(escritor, map[string]any{"cerrada": true})
}

func (a *API) sesionActual(escritor http.ResponseWriter, peticion *http.Request) {
	usuario, _ := autenticacion.UsuarioDe(peticion.Context())

	permisos, err := a.Datos.RedesDeUsuario(peticion.Context(), usuario)
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Sesion", Accion: "Consultar", Causa: CausaBaseDatos,
			Tabla: "permisos_red", Codigo: http.StatusInternalServerError,
		}, "No se pudieron leer sus permisos.", err)
		return
	}

	responderOk(escritor, map[string]any{
		"usuario":  usuario,
		"permisos": permisos,
		"version":  version.Completa(),
	})
}

// ----------------------------------------------------------------- sonda ----

// estadoSonda pregunta a mired-sonda si esta viva. Que no conteste NO es un
// error del sistema: es informacion util que la interfaz muestra tal cual, para
// que se entienda por que no se puede escanear.
func (a *API) estadoSonda(escritor http.ResponseWriter, peticion *http.Request) {
	estado, err := sonda.PedirEstado(a.SocketSonda, 5*time.Second)
	if err != nil {
		responderOk(escritor, map[string]any{
			"viva":   false,
			"socket": a.SocketSonda,
			"motivo": "No se pudo hablar con la sonda de escaneo. Revise que el servicio mired-sonda este corriendo.",
			"error":  err.Error(),
		})
		return
	}
	responderOk(escritor, map[string]any{"viva": true, "estado": estado})
}

// ----------------------------------------------------------------- redes ----

func (a *API) listarRedes(escritor http.ResponseWriter, peticion *http.Request) {
	usuario, _ := autenticacion.UsuarioDe(peticion.Context())

	redes, err := a.Datos.ListarRedes(peticion.Context())
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Redes", Accion: "Listar", Causa: CausaBaseDatos,
			Tabla: "redes", Codigo: http.StatusInternalServerError,
		}, "No se pudieron listar las redes.", err)
		return
	}

	// El panel se arma con el resumen guardado en el catalogo: no se abre el
	// archivo de ninguna red para pintar esta pantalla.
	if !usuario.Superadmin {
		permisos, err := a.Datos.RedesDeUsuario(peticion.Context(), usuario)
		if err != nil {
			a.responderError(escritor, peticion, contextoError{
				Modulo: "Redes", Accion: "Listar", Causa: CausaBaseDatos,
				Tabla: "permisos_red", Codigo: http.StatusInternalServerError,
			}, "No se pudieron leer sus permisos.", err)
			return
		}
		visibles := make([]basedatos.Red, 0, len(redes))
		for _, red := range redes {
			if _, hay := permisos[red.Clave]; hay {
				visibles = append(visibles, red)
			}
		}
		redes = visibles
	}

	responderOk(escritor, redes)
}

type peticionRed struct {
	Nombre      string   `json:"nombre"`
	Descripcion string   `json:"descripcion"`
	Subredes    []string `json:"subredes"`
}

func (a *API) crearRed(escritor http.ResponseWriter, peticion *http.Request) {
	usuario, _ := autenticacion.UsuarioDe(peticion.Context())
	if !usuario.Superadmin {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Redes", Accion: "Crear", Causa: CausaPermiso,
			Codigo: http.StatusForbidden,
		}, "Solo el administrador del sistema puede crear redes.", nil)
		return
	}

	var cuerpo peticionRed
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Redes", "Crear") {
		return
	}
	if strings.TrimSpace(cuerpo.Nombre) == "" {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Redes", Accion: "Crear", Causa: CausaValidacion,
			Codigo: http.StatusBadRequest,
		}, "La red necesita un nombre.", nil)
		return
	}

	// Las subredes se validan ANTES de crear nada: mas vale rechazar la captura
	// que dejar una red creada a medias con un rango mal escrito.
	limpias := make([]string, 0, len(cuerpo.Subredes))
	for _, cidr := range cuerpo.Subredes {
		if strings.TrimSpace(cidr) == "" {
			continue
		}
		canonico, err := basedatos.NormalizarCIDR(cidr)
		if err != nil {
			a.responderError(escritor, peticion, contextoError{
				Modulo: "Redes", Accion: "Crear", Causa: CausaValidacion,
				Codigo: http.StatusBadRequest,
			}, "La subred \""+cidr+"\" no es valida. Se espera algo como 192.168.1.0/24.", nil)
			return
		}
		limpias = append(limpias, canonico)
	}

	red, err := a.Datos.CrearRed(peticion.Context(), cuerpo.Nombre, cuerpo.Descripcion, limpias)
	if errors.Is(err, basedatos.ErrNombreRepetido) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Redes", Accion: "Crear", Causa: CausaValidacion,
			Tabla: "redes", Codigo: http.StatusConflict,
		}, "Ya existe una red con ese nombre.", nil)
		return
	}
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Redes", Accion: "Crear", Causa: CausaBaseDatos,
			Tabla: "redes", Codigo: http.StatusInternalServerError,
			Estado: "La red NO se creo.",
		}, "No se pudo crear la red.", err)
		return
	}

	a.anotarActividad(peticion, "Redes", "Crear red "+red.Nombre)
	responderOk(escritor, red)
}

func (a *API) verRed(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())
	nivel, _ := autenticacion.NivelDe(peticion.Context())

	red, err := a.Datos.BuscarRed(peticion.Context(), clave)
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Redes", Accion: "Consultar", Causa: CausaBaseDatos,
			Tabla: "redes", Codigo: http.StatusInternalServerError,
		}, "No se pudo leer la red.", err)
		return
	}
	responderOk(escritor, map[string]any{"red": red, "nivel": nivel})
}

func (a *API) editarRed(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())
	nivel, _ := autenticacion.NivelDe(peticion.Context())
	if !autenticacion.PuedeAdministrar(nivel) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Redes", Accion: "Editar", Causa: CausaPermiso,
			Codigo: http.StatusForbidden,
		}, "Necesita permiso de administracion sobre esta red.", nil)
		return
	}

	var cuerpo peticionRed
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Redes", "Editar") {
		return
	}

	red, err := a.Datos.RenombrarRed(peticion.Context(), clave, cuerpo.Nombre, cuerpo.Descripcion)
	if errors.Is(err, basedatos.ErrNombreRepetido) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Redes", Accion: "Editar", Causa: CausaValidacion,
			Tabla: "redes", Codigo: http.StatusConflict,
		}, "Ya existe otra red con ese nombre.", nil)
		return
	}
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Redes", Accion: "Editar", Causa: CausaBaseDatos,
			Tabla: "redes", Codigo: http.StatusInternalServerError,
		}, "No se pudo guardar el cambio.", err)
		return
	}

	a.anotarActividad(peticion, "Redes", "Editar red "+red.Nombre)
	responderOk(escritor, red)
}

func (a *API) borrarRed(escritor http.ResponseWriter, peticion *http.Request) {
	usuario, _ := autenticacion.UsuarioDe(peticion.Context())
	clave, _ := autenticacion.RedActivaDe(peticion.Context())
	if !usuario.Superadmin {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Redes", Accion: "Borrar", Causa: CausaPermiso,
			Codigo: http.StatusForbidden,
		}, "Solo el administrador del sistema puede borrar redes.", nil)
		return
	}

	if err := a.Datos.BorrarRed(peticion.Context(), clave); err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Redes", Accion: "Borrar", Causa: CausaBaseDatos,
			Tabla: "redes", Codigo: http.StatusInternalServerError,
		}, "No se pudo borrar la red.", err)
		return
	}

	a.anotarActividad(peticion, "Redes", "Borrar red "+clave)
	// El archivo .db NO se elimina: queda para respaldo y para poder reactivar
	// la red por su nombre. Se dice para que nadie lo busque en la papelera.
	responderOk(escritor, map[string]any{
		"borrada": true,
		"aviso":   "El archivo de datos de la red se conservo en el servidor.",
	})
}

// -------------------------------------------------------------- subredes ----

func (a *API) listarSubredes(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var subredes []basedatos.Subred
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		subredes, err = base.ListarSubredes(peticion.Context())
		return err
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Subredes", Accion: "Listar", Causa: CausaBaseDatos,
			Tabla: "subredes", Codigo: http.StatusInternalServerError,
		}, "No se pudieron listar las subredes.", err)
		return
	}
	responderOk(escritor, subredes)
}

type peticionSubred struct {
	CIDR        string `json:"cidr"`
	Nombre      string `json:"nombre"`
	Descripcion string `json:"descripcion"`
	VLAN        *int   `json:"vlan"`
}

func (a *API) crearSubred(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())
	nivel, _ := autenticacion.NivelDe(peticion.Context())
	if !autenticacion.PuedeEscribir(nivel) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Subredes", Accion: "Crear", Causa: CausaPermiso,
			Codigo: http.StatusForbidden,
		}, "Necesita permiso de escritura sobre esta red.", nil)
		return
	}

	var cuerpo peticionSubred
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Subredes", "Crear") {
		return
	}
	if cuerpo.VLAN != nil && (*cuerpo.VLAN < 1 || *cuerpo.VLAN > 4094) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Subredes", Accion: "Crear", Causa: CausaValidacion,
			Codigo: http.StatusBadRequest,
		}, "El VLAN debe estar entre 1 y 4094.", nil)
		return
	}

	var subred basedatos.Subred
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		subred, err = base.CrearSubred(peticion.Context(), cuerpo.CIDR, cuerpo.Nombre,
			cuerpo.Descripcion, cuerpo.VLAN)
		return err
	})
	if errors.Is(err, basedatos.ErrCIDRInvalido) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Subredes", Accion: "Crear", Causa: CausaValidacion,
			Codigo: http.StatusBadRequest,
		}, "La subred no es valida. Se espera algo como 192.168.1.0/24.", nil)
		return
	}
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Subredes", Accion: "Crear", Causa: CausaBaseDatos,
			Tabla: "subredes", Codigo: http.StatusInternalServerError,
		}, "No se pudo guardar la subred.", err)
		return
	}

	a.anotarActividad(peticion, "Subredes", "Agregar "+subred.CIDR+" a "+clave)
	responderOk(escritor, subred)
}

// -------------------------------------------------------------- usuarios ----

func (a *API) listarUsuarios(escritor http.ResponseWriter, peticion *http.Request) {
	usuarios, err := a.Datos.ListarUsuarios(peticion.Context())
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Usuarios", Accion: "Listar", Causa: CausaBaseDatos,
			Tabla: "usuarios", Codigo: http.StatusInternalServerError,
		}, "No se pudieron listar los usuarios.", err)
		return
	}
	responderOk(escritor, usuarios)
}

type peticionUsuario struct {
	Usuario    string `json:"usuario"`
	Nombre     string `json:"nombre"`
	Clave      string `json:"clave"`
	Correo     string `json:"correo"`
	Superadmin bool   `json:"superadmin"`
}

func (a *API) crearUsuario(escritor http.ResponseWriter, peticion *http.Request) {
	var cuerpo peticionUsuario
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Usuarios", "Crear") {
		return
	}

	// Lo mismo que exige la base: el formulario no debe poder mandar algo que la
	// columna vaya a rechazar, y el servidor lo comprueba igual.
	switch {
	case strings.TrimSpace(cuerpo.Usuario) == "":
		a.errorValidacion(escritor, peticion, "Usuarios", "Crear", "Capture el usuario de acceso.")
		return
	case len(cuerpo.Usuario) > 60:
		a.errorValidacion(escritor, peticion, "Usuarios", "Crear", "El usuario no puede pasar de 60 caracteres.")
		return
	case strings.TrimSpace(cuerpo.Nombre) == "":
		a.errorValidacion(escritor, peticion, "Usuarios", "Crear", "Capture el nombre de la persona.")
		return
	case len(cuerpo.Clave) < 8:
		a.errorValidacion(escritor, peticion, "Usuarios", "Crear", "La clave debe tener al menos 8 caracteres.")
		return
	}

	hash, err := autenticacion.HashClave(cuerpo.Clave)
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Usuarios", Accion: "Crear", Causa: CausaInterno,
			Codigo: http.StatusInternalServerError,
		}, "No se pudo proteger la clave.", err)
		return
	}

	usuario, err := a.Datos.CrearUsuario(peticion.Context(), cuerpo.Usuario, cuerpo.Nombre,
		hash, cuerpo.Correo, cuerpo.Superadmin)
	if errors.Is(err, basedatos.ErrUsuarioRepetido) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Usuarios", Accion: "Crear", Causa: CausaValidacion,
			Tabla: "usuarios", Codigo: http.StatusConflict,
		}, "Ya existe un usuario con ese nombre de acceso.", nil)
		return
	}
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Usuarios", Accion: "Crear", Causa: CausaBaseDatos,
			Tabla: "usuarios", Codigo: http.StatusInternalServerError,
			Estado: "El usuario NO se creo.",
		}, "No se pudo crear el usuario.", err)
		return
	}

	a.anotarActividad(peticion, "Usuarios", "Crear usuario "+usuario.Usuario)
	responderOk(escritor, usuario)
}

func (a *API) borrarUsuario(escritor http.ResponseWriter, peticion *http.Request) {
	id, ok := a.leerID(escritor, peticion, "Usuarios", "Borrar")
	if !ok {
		return
	}

	quien, _ := autenticacion.UsuarioDe(peticion.Context())
	if quien.ID == id {
		a.errorValidacion(escritor, peticion, "Usuarios", "Borrar", "No puede borrarse a si mismo.")
		return
	}

	if err := a.Datos.BorrarUsuario(peticion.Context(), id); err != nil {
		if errors.Is(err, basedatos.ErrUsuarioNoExiste) {
			a.errorValidacion(escritor, peticion, "Usuarios", "Borrar",
				"Ese usuario no existe, o es el administrador del sistema y no se puede borrar.")
			return
		}
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Usuarios", Accion: "Borrar", Causa: CausaBaseDatos,
			Tabla: "usuarios", Codigo: http.StatusInternalServerError,
		}, "No se pudo borrar el usuario.", err)
		return
	}

	a.anotarActividad(peticion, "Usuarios", "Borrar usuario "+strconv.FormatInt(id, 10))
	responderOk(escritor, map[string]any{"borrado": true})
}

type peticionPermiso struct {
	Nivel string `json:"nivel"`
}

func (a *API) otorgarPermiso(escritor http.ResponseWriter, peticion *http.Request) {
	id, ok := a.leerID(escritor, peticion, "Usuarios", "Otorgar permiso")
	if !ok {
		return
	}
	var cuerpo peticionPermiso
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Usuarios", "Otorgar permiso") {
		return
	}

	red := peticion.PathValue("red")
	if err := a.Datos.OtorgarPermiso(peticion.Context(), id, red, cuerpo.Nivel); err != nil {
		if errors.Is(err, basedatos.ErrRedNoExiste) {
			a.errorValidacion(escritor, peticion, "Usuarios", "Otorgar permiso", "Esa red no existe.")
			return
		}
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Usuarios", Accion: "Otorgar permiso", Causa: CausaBaseDatos,
			Tabla: "permisos_red", Codigo: http.StatusInternalServerError,
		}, "No se pudo otorgar el permiso.", err)
		return
	}

	a.anotarActividad(peticion, "Usuarios", "Permiso "+cuerpo.Nivel+" sobre "+red)
	responderOk(escritor, map[string]any{"otorgado": true})
}

func (a *API) quitarPermiso(escritor http.ResponseWriter, peticion *http.Request) {
	id, ok := a.leerID(escritor, peticion, "Usuarios", "Quitar permiso")
	if !ok {
		return
	}

	red := peticion.PathValue("red")
	if err := a.Datos.QuitarPermiso(peticion.Context(), id, red); err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Usuarios", Accion: "Quitar permiso", Causa: CausaBaseDatos,
			Tabla: "permisos_red", Codigo: http.StatusInternalServerError,
		}, "No se pudo quitar el permiso.", err)
		return
	}

	a.anotarActividad(peticion, "Usuarios", "Quitar permiso sobre "+red)
	responderOk(escritor, map[string]any{"quitado": true})
}

// ------------------------------------------------- bitacora de errores ----

type peticionErrorExterno struct {
	ErrorDetalle
	Origen      string `json:"origen"`
	Trayectoria string `json:"trayectoria"`
	Pila        string `json:"pila"`
}

// recibirError guarda un error reportado por la interfaz. Es lo que permite que
// soporte vea los errores sin depender de que el usuario los reporte.
//
// No exige sesion a proposito: si el error fue justamente al entrar, obligar a
// tener sesion dejaria fuera los errores que mas importan.
func (a *API) recibirError(escritor http.ResponseWriter, peticion *http.Request) {
	var cuerpo peticionErrorExterno
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Bitacora", "Recibir error") {
		return
	}

	origen := cuerpo.Origen
	if origen != "sonda" && origen != "interfaz" {
		origen = "interfaz"
	}

	if err := a.RegistrarErrorExterno(peticion.Context(), origen, cuerpo.ErrorDetalle,
		cuerpo.Trayectoria, cuerpo.Pila); err != nil {
		// Si ni esto se puede guardar, no se le devuelve otro error al usuario:
		// ya esta viendo uno. Queda en el diario del sistema.
		a.Bitacora.Error("no se pudo guardar el error reportado", "error", err)
	}
	responderOk(escritor, map[string]any{"recibido": true})
}

// ------------------------------------------------------------- auxiliares ----

func (a *API) leerCuerpo(escritor http.ResponseWriter, peticion *http.Request, destino any, modulo, accion string) bool {
	limitado := http.MaxBytesReader(escritor, peticion.Body, topeCuerpo)
	defer limitado.Close()

	crudo, err := io.ReadAll(limitado)
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: modulo, Accion: accion, Causa: CausaValidacion,
			Codigo: http.StatusRequestEntityTooLarge,
		}, "La peticion es demasiado grande.", err)
		return false
	}
	if len(crudo) == 0 {
		a.responderError(escritor, peticion, contextoError{
			Modulo: modulo, Accion: accion, Causa: CausaValidacion,
			Codigo: http.StatusBadRequest,
		}, "La peticion llego vacia.", nil)
		return false
	}
	if err := json.Unmarshal(crudo, destino); err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: modulo, Accion: accion, Causa: CausaValidacion,
			Codigo: http.StatusBadRequest,
		}, "Los datos enviados no se pudieron interpretar.", err)
		return false
	}
	return true
}

func (a *API) leerID(escritor http.ResponseWriter, peticion *http.Request, modulo, accion string) (int64, bool) {
	id, err := strconv.ParseInt(peticion.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		a.errorValidacion(escritor, peticion, modulo, accion, "El identificador no es valido.")
		return 0, false
	}
	return id, true
}

func (a *API) errorValidacion(escritor http.ResponseWriter, peticion *http.Request, modulo, accion, mensaje string) {
	a.responderError(escritor, peticion, contextoError{
		Modulo: modulo, Accion: accion, Causa: CausaValidacion,
		Codigo: http.StatusBadRequest,
	}, mensaje, nil)
}

// anotarActividad deja constancia de quien hizo que. Es mejor perder la
// anotacion que la operacion, asi que un fallo aqui solo se registra.
func (a *API) anotarActividad(peticion *http.Request, modulo, detalle string) {
	usuario, hay := autenticacion.UsuarioDe(peticion.Context())
	var usuarioID any
	if hay {
		usuarioID = usuario.ID
	}

	_, err := a.Datos.Catalogo.ExecContext(peticion.Context(), `
		INSERT INTO log_actividad (momento, usuario_id, modulo, accion, detalle, direccion)
		VALUES (?, ?, ?, ?, ?, ?)`,
		basedatos.Ahora(), usuarioID, modulo, detalle, nil, direccionDe(peticion))
	if err != nil {
		a.Bitacora.Warn("no se pudo anotar la actividad", "modulo", modulo, "error", err)
	}
}

func direccionDe(peticion *http.Request) string {
	if reenviado := peticion.Header.Get("X-Forwarded-For"); reenviado != "" {
		if coma := strings.Index(reenviado, ","); coma > 0 {
			return strings.TrimSpace(reenviado[:coma])
		}
		return strings.TrimSpace(reenviado)
	}
	if dosPuntos := strings.LastIndex(peticion.RemoteAddr, ":"); dosPuntos > 0 {
		return peticion.RemoteAddr[:dosPuntos]
	}
	return peticion.RemoteAddr
}
