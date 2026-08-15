package basedatos

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// La lista de tipos de puerto vive TRES VECES: en el CHECK de la tabla (red
// 0018), en Go —porque el servidor valida antes de guardar— y en Dart, porque el
// formulario la ofrece. No hay forma de tener una sola, asi que lo unico que
// queda es vigilar que no se separen.
//
// Si el formulario ofrece un tipo que la tabla no admite, quien lo elija ve
// reventar el guardado por una lista desincronizada; y si la tabla admite uno
// que el formulario no ofrece, ese puerto no se puede declarar desde ningun
// lado. Es la misma regla que ya cuida las categorias de equipo.

var tipoEnDart = regexp.MustCompile(`clave:\s*'([a-z_]+)'`)

func TestLasDosListasDeTiposDePuertoDicenLoMismo(t *testing.T) {
	ruta := filepath.Join("..", "..", "interfaz", "lib", "modelos", "tipos_de_puerto.dart")
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no se pudo leer la lista de la interfaz: %v", err)
	}

	var enDart []string
	for _, coincidencia := range tipoEnDart.FindAllStringSubmatch(string(crudo), -1) {
		enDart = append(enDart, coincidencia[1])
	}

	if len(enDart) == 0 {
		t.Fatal("no se encontro ningun tipo en la lista de la interfaz: " +
			"¿cambio el formato del archivo?")
	}
	if len(enDart) != len(TiposDePuerto) {
		t.Fatalf("las listas tienen distinto tamano: Go %v, interfaz %v", TiposDePuerto, enDart)
	}
	for i := range TiposDePuerto {
		if TiposDePuerto[i] != enDart[i] {
			t.Fatalf("las listas se separaron: Go tiene %q donde la interfaz tiene %q",
				TiposDePuerto[i], enDart[i])
		}
	}
}

// Y la tabla tiene que admitir exactamente lo mismo que Go valida: un tipo que
// pase la validacion y rebote en el CHECK seria un error 500 en vez de un
// mensaje de negocio.
func TestLaTablaAdmiteTodosLosTiposQueGoDaPorBuenos(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := t.Context()

	equipo, err := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "Router", Categoria: "gateway", Puertos: 0,
	})
	if err != nil {
		t.Fatalf("no se pudo declarar el equipo: %v", err)
	}

	for i, tipo := range TiposDePuerto {
		if _, err := base.AgregarPuertoFisico(ctx, PuertoFisico{
			EquipoID: equipo.ID, Numero: i + 1, Tipo: tipo,
		}); err != nil {
			t.Fatalf("el tipo %q lo da por bueno Go y lo rechaza la tabla: %v", tipo, err)
		}
	}
}
