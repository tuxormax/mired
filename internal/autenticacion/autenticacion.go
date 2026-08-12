// Paquete autenticacion resuelve quien entra a MiRed y a que redes puede llegar.
//
// Ademas del usuario, este paquete fija en el contexto de la peticion la RED
// ACTIVA. Esa decision es la que permite que el resto del proyecto no tenga que
// arrastrar la red por parametro en cada funcion: la capa de base de datos la
// lee del contexto y resuelve sola a que archivo va.
package autenticacion

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/tuxormax/mired/internal/basedatos"
)

// Errores que la capa de arriba distingue para responder con el mensaje justo.
var (
	// ErrCredencialesInvalidas cubre usuario inexistente y clave equivocada, a
	// proposito: decir cual de las dos fallo le regala informacion a quien
	// prueba contrasenas.
	ErrCredencialesInvalidas = errors.New("usuario o clave incorrectos")
	// ErrSinSesion es que la peticion no trae credenciales.
	ErrSinSesion = errors.New("no hay sesion activa")
	// ErrSinPermiso es que la sesion es valida pero no alcanza para esto.
	ErrSinPermiso = errors.New("no tiene permiso para esta red")
)

// NombreCookie es la cookie donde viaja el token de sesion.
const NombreCookie = "mired_sesion"

// Datos del superadministrador que se siembra en la primera instalacion. Es la
// convencion de la casa y se cambia desde la interfaz al entrar.
const (
	UsuarioSuperadmin = "usuario-quitado"
	ClaveSuperadmin   = "clave-quitada"
	NombreSuperadmin  = "Superadministrador"
)

// Servicio agrupa lo que hace falta para autenticar.
type Servicio struct {
	Datos    *basedatos.Enrutador
	Duracion time.Duration
}

// Nuevo arma el servicio de autenticacion.
func Nuevo(datos *basedatos.Enrutador, duracion time.Duration) *Servicio {
	if duracion <= 0 {
		duracion = 12 * time.Hour
	}
	return &Servicio{Datos: datos, Duracion: duracion}
}

// HashClave protege una contrasena con bcrypt.
func HashClave(clave string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(clave), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("no se pudo proteger la clave: %w", err)
	}
	return string(hash), nil
}

// ComprobarClave verifica una contrasena contra su hash.
func ComprobarClave(hash, clave string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(clave)) == nil
}

// SembrarSuperadmin crea el primer usuario si el catalogo esta vacio. Devuelve
// true si lo creo, para que el arranque lo avise en la bitacora.
func (s *Servicio) SembrarSuperadmin(ctx context.Context) (bool, error) {
	hay, err := s.Datos.HayUsuarios(ctx)
	if err != nil || hay {
		return false, err
	}

	hash, err := HashClave(ClaveSuperadmin)
	if err != nil {
		return false, err
	}
	if _, err := s.Datos.CrearUsuario(ctx, UsuarioSuperadmin, NombreSuperadmin, hash, "", true); err != nil {
		return false, fmt.Errorf("no se pudo sembrar el superadministrador: %w", err)
	}
	return true, nil
}

// IniciarSesion comprueba las credenciales y abre una sesion.
func (s *Servicio) IniciarSesion(ctx context.Context, usuario, clave, direccion, agente string) (basedatos.Sesion, basedatos.Usuario, error) {
	encontrado, err := s.Datos.BuscarUsuario(ctx, usuario)
	if errors.Is(err, basedatos.ErrUsuarioNoExiste) {
		// Se gasta el mismo tiempo que en una comprobacion real para no delatar
		// por la demora si el usuario existe o no.
		bcrypt.CompareHashAndPassword([]byte("$2a$10$invalidoinvalidoinvalidoinvalidoinvalidoinvalidoinvalidoinv"), []byte(clave))
		return basedatos.Sesion{}, basedatos.Usuario{}, ErrCredencialesInvalidas
	}
	if err != nil {
		return basedatos.Sesion{}, basedatos.Usuario{}, err
	}

	if !ComprobarClave(encontrado.Clave, clave) {
		return basedatos.Sesion{}, basedatos.Usuario{}, ErrCredencialesInvalidas
	}

	sesion, err := s.Datos.CrearSesion(ctx, encontrado.ID, s.Duracion, direccion, agente)
	if err != nil {
		return basedatos.Sesion{}, basedatos.Usuario{}, err
	}
	s.Datos.MarcarAcceso(ctx, encontrado.ID)

	encontrado.Clave = ""
	return sesion, encontrado, nil
}

// CerrarSesion invalida el token.
func (s *Servicio) CerrarSesion(ctx context.Context, token string) error {
	return s.Datos.BorrarSesion(ctx, token)
}

// Identificar resuelve quien hace la peticion, ya sea por cookie o por cabecera
// Authorization. Devuelve tambien el token, para poder cerrar la sesion.
func (s *Servicio) Identificar(peticion *http.Request) (basedatos.Usuario, string, error) {
	token := tokenDePeticion(peticion)
	if token == "" {
		return basedatos.Usuario{}, "", ErrSinSesion
	}

	usuario, err := s.Datos.BuscarSesion(peticion.Context(), token, s.Duracion)
	if errors.Is(err, basedatos.ErrSesionInvalida) {
		return basedatos.Usuario{}, "", ErrSinSesion
	}
	if err != nil {
		return basedatos.Usuario{}, "", err
	}
	usuario.Clave = ""
	return usuario, token, nil
}

func tokenDePeticion(peticion *http.Request) string {
	if cookie, err := peticion.Cookie(NombreCookie); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	cabecera := peticion.Header.Get("Authorization")
	if despues, hay := strings.CutPrefix(cabecera, "Bearer "); hay {
		return strings.TrimSpace(despues)
	}
	return ""
}

// PonerCookie deja el token en el navegador.
func PonerCookie(escritor http.ResponseWriter, token string, duracion time.Duration, seguro bool) {
	http.SetCookie(escritor, &http.Cookie{
		Name:     NombreCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   seguro,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(duracion),
	})
}

// QuitarCookie borra el token del navegador.
func QuitarCookie(escritor http.ResponseWriter, seguro bool) {
	http.SetCookie(escritor, &http.Cookie{
		Name:     NombreCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   seguro,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// PermisoSobre devuelve el nivel de acceso del usuario a una red, o ErrSinPermiso.
func (s *Servicio) PermisoSobre(ctx context.Context, usuario basedatos.Usuario, claveRed string) (string, error) {
	if usuario.Superadmin {
		return basedatos.NivelAdministra, nil
	}
	permisos, err := s.Datos.RedesDeUsuario(ctx, usuario)
	if err != nil {
		return "", err
	}
	nivel, hay := permisos[claveRed]
	if !hay {
		return "", ErrSinPermiso
	}
	return nivel, nil
}

// PuedeEscribir dice si el nivel alcanza para modificar.
func PuedeEscribir(nivel string) bool {
	return nivel == basedatos.NivelEscritura || nivel == basedatos.NivelAdministra
}

// PuedeAdministrar dice si el nivel alcanza para tocar la configuracion de la red.
func PuedeAdministrar(nivel string) bool {
	return nivel == basedatos.NivelAdministra
}
