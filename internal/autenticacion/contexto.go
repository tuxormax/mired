package autenticacion

import (
	"context"

	"github.com/tuxormax/mired/internal/basedatos"
)

// El contexto de la peticion lleva quien pregunta y sobre que red. Es la pieza
// que evita tener que pasar la red por parametro en cada funcion del proyecto.
type claveContexto string

const (
	claveUsuario   claveContexto = "mired.usuario"
	claveRedActiva claveContexto = "mired.red_activa"
	claveNivel     claveContexto = "mired.nivel"
)

// ConUsuario deja al usuario identificado en el contexto.
func ConUsuario(ctx context.Context, usuario basedatos.Usuario) context.Context {
	return context.WithValue(ctx, claveUsuario, usuario)
}

// UsuarioDe recupera al usuario de la peticion.
func UsuarioDe(ctx context.Context) (basedatos.Usuario, bool) {
	usuario, hay := ctx.Value(claveUsuario).(basedatos.Usuario)
	return usuario, hay
}

// ConRedActiva fija sobre que red trabaja esta peticion, y con que nivel.
//
// A partir de aqui, cualquier consulta a datos operativos va al archivo de esa
// red. Si nadie la fija, no hay red activa: eso es a proposito, para que una
// consulta que se olvide de decir en que red trabaja falle en vez de escribir
// en el archivo equivocado.
func ConRedActiva(ctx context.Context, claveRed, nivel string) context.Context {
	ctx = context.WithValue(ctx, claveRedActiva, claveRed)
	return context.WithValue(ctx, claveNivel, nivel)
}

// RedActivaDe devuelve la red sobre la que trabaja la peticion.
func RedActivaDe(ctx context.Context) (string, bool) {
	clave, hay := ctx.Value(claveRedActiva).(string)
	return clave, hay && clave != ""
}

// NivelDe devuelve el nivel de acceso del usuario sobre la red activa.
func NivelDe(ctx context.Context) (string, bool) {
	nivel, hay := ctx.Value(claveNivel).(string)
	return nivel, hay && nivel != ""
}
