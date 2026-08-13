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

	"github.com/tuxormax/mired/internal/basedatos"
)

// hashDeMentira tiene el formato bueno pero no corresponde a ninguna clave. Se
// usa para gastar el mismo trabajo cuando el usuario ni siquiera existe.
const hashDeMentira = "tuxor$14$00000000000000000000000000000000$" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000"

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
	// ErrYaEstrenado es que alguien intento crear el primer administrador en una
	// instalacion que ya tiene usuarios. Esa puerta se cierra en cuanto existe
	// el primero.
	ErrYaEstrenado = errors.New("esta instalacion ya tiene administrador")
)

// NombreCookie es la cookie donde viaja el token de sesion.
const NombreCookie = "mired_sesion"

// **Aqui NO hay ningun usuario ni clave sembrados, a proposito.**
//
// Una instalacion recien hecha no tiene usuarios: la primera vez que alguien
// entra, la interfaz le pide que cree el administrador con el usuario y la clave
// que el elija. Un usuario y una clave iguales en todas las instalaciones son,
// en un proyecto publico, una puerta abierta que cualquiera puede buscar: basta
// rastrear el puerto y probarlos. Que nadie tenga que "acordarse de cambiarla"
// es la unica forma de que no se quede sin cambiar.

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

// SinEstrenar dice si la instalacion todavia no tiene ningun usuario.
//
// Es lo que la interfaz consulta antes de pintar la pantalla de entrada: si no
// hay nadie, en vez del formulario de entrar muestra el de crear el
// administrador.
func (s *Servicio) SinEstrenar(ctx context.Context) (bool, error) {
	hay, err := s.Datos.HayUsuarios(ctx)
	return !hay, err
}

// CrearPrimerAdministrador da de alta al superadministrador de una instalacion
// recien hecha.
//
// **Solo funciona mientras no haya ningun usuario.** Esa comprobacion es lo que
// impide que alguien de fuera se cree un administrador en un MiRed ya en uso: en
// cuanto existe el primero, esta puerta se cierra para siempre.
func (s *Servicio) CrearPrimerAdministrador(ctx context.Context, usuario, nombre, clave, correo string) (basedatos.Usuario, error) {
	hay, err := s.Datos.HayUsuarios(ctx)
	if err != nil {
		return basedatos.Usuario{}, err
	}
	if hay {
		return basedatos.Usuario{}, ErrYaEstrenado
	}

	usuario = strings.TrimSpace(usuario)
	nombre = strings.TrimSpace(nombre)
	if usuario == "" {
		return basedatos.Usuario{}, fmt.Errorf("capture el usuario")
	}
	if len(clave) < 8 {
		return basedatos.Usuario{}, fmt.Errorf("la clave debe tener al menos 8 caracteres")
	}
	if nombre == "" {
		nombre = usuario
	}

	// TUXOR exige que el usuario o la clave lleven un operador. Se comprueba
	// aqui con el mensaje completo, porque es la regla que mas sorprende y la
	// que hay que explicar, no solo rechazar.
	if !TuxorValido(usuario) && !TuxorValido(clave) {
		return basedatos.Usuario{}, fmt.Errorf(
			"el usuario o la clave deben empezar o terminar con uno de estos signos: %s",
			OperadoresParaLeer())
	}

	hash, err := HashClave(usuario, clave)
	if err != nil {
		return basedatos.Usuario{}, err
	}

	creado, err := s.Datos.CrearUsuario(ctx, usuario, nombre, hash, correo, true)
	if err != nil {
		return basedatos.Usuario{}, fmt.Errorf("no se pudo crear el administrador: %w", err)
	}
	creado.Clave = ""
	return creado, nil
}

// IniciarSesion comprueba las credenciales y abre una sesion.
func (s *Servicio) IniciarSesion(ctx context.Context, usuario, clave, direccion, agente string) (basedatos.Sesion, basedatos.Usuario, error) {
	encontrado, err := s.Datos.BuscarUsuario(ctx, usuario)
	if errors.Is(err, basedatos.ErrUsuarioNoExiste) {
		// Se gasta el mismo trabajo que en una comprobacion real para no delatar
		// por la demora si el usuario existe o no. Con scrypt eso no es un
		// detalle: la diferencia entre comprobar y no comprobar es de decimas de
		// segundo, perfectamente medible desde fuera.
		ComprobarClave(hashDeMentira, usuario, clave)
		return basedatos.Sesion{}, basedatos.Usuario{}, ErrCredencialesInvalidas
	}
	if err != nil {
		return basedatos.Sesion{}, basedatos.Usuario{}, err
	}

	// El usuario forma parte del calculo del TUXOR, no solo la clave.
	if !ComprobarClave(encontrado.Clave, usuario, clave) {
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
