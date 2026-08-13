package version

import (
	_ "embed"
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

// El historial de versiones viaja DENTRO del binario.
//
// En los proyectos PHP de la casa esto vive en un seeder SQL que alguien corre
// contra la base. Aqui no puede: MiRed se instala con un `.deb` en equipos donde
// nadie va a abrir un cliente de SQL. Asi que el historial se compila dentro y
// el servidor siembra las tablas al arrancar.
//
// **Este archivo es tambien la fuente de la version.** `herramientas/construir.sh`
// lee la ultima entrada para saber con que version y revision compilar, y por eso
// ya no se teclean a mano — que era como se podian reiniciar sin que nadie se
// diera cuenta.

//go:embed historial.toml
var historialCrudo []byte

// Entrada es una linea del historial: una entrega y que trajo.
type Entrada struct {
	Version  string `toml:"version" json:"version"`
	Revision int    `toml:"revision" json:"revision"`
	Fecha    string `toml:"fecha" json:"fecha"`
	Tipo     string `toml:"tipo" json:"tipo"`
	Modulo   string `toml:"modulo" json:"modulo"`
	Notas    string `toml:"notas" json:"notas"`
}

// Historial son las dos listas: la del sistema y la de la base de datos.
type Historial struct {
	Sistema []Entrada `toml:"sistema" json:"sistema"`
	Base    []Entrada `toml:"base" json:"base"`
}

// LeerHistorial devuelve el historial que se compilo dentro del binario.
func LeerHistorial() (Historial, error) {
	var historial Historial
	if err := toml.Unmarshal(historialCrudo, &historial); err != nil {
		return Historial{}, fmt.Errorf("el historial de versiones esta mal escrito: %w", err)
	}
	if len(historial.Sistema) == 0 {
		return Historial{}, fmt.Errorf("el historial de versiones no tiene ninguna entrada")
	}
	return historial, nil
}

// Ultima devuelve la entrega mas reciente del sistema.
//
// Es la que manda: la version y la revision del binario salen de aqui, no de un
// argumento que alguien teclea al compilar.
func (h Historial) Ultima() Entrada {
	ultima := h.Sistema[0]
	for _, entrada := range h.Sistema[1:] {
		if entrada.Revision > ultima.Revision {
			ultima = entrada
		}
	}
	return ultima
}
