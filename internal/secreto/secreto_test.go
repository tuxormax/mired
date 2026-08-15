package secreto

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnSecretoVaYVuelve(t *testing.T) {
	caja := NuevaCaja(filepath.Join(t.TempDir(), "llave"))

	guardado, err := caja.Cifrar("clave-del-modem-2026")
	if err != nil {
		t.Fatalf("no se pudo cifrar: %v", err)
	}
	if strings.Contains(guardado, "clave-del-modem") {
		t.Fatal("la clave quedo legible dentro de lo guardado")
	}

	claro, err := caja.Descifrar(guardado)
	if err != nil {
		t.Fatalf("no se pudo descifrar: %v", err)
	}
	if claro != "clave-del-modem-2026" {
		t.Fatalf("volvio %q", claro)
	}
}

func TestDosVecesLaMismaClaveSeGuardanDistinto(t *testing.T) {
	// Si dos equipos con la misma clave se vieran iguales en la base, cualquiera
	// que la mire sabria que comparten clave sin necesidad de descifrar nada.
	caja := NuevaCaja(filepath.Join(t.TempDir(), "llave"))

	uno, _ := caja.Cifrar("admin")
	otro, _ := caja.Cifrar("admin")
	if uno == otro {
		t.Fatal("la misma clave se guardo identica dos veces")
	}
}

func TestConOtraLlaveNoSeAbre(t *testing.T) {
	// Es lo que protege el respaldo: la base copiada a otro lado, sin la llave
	// —que vive con la configuracion, no con los datos—, no se abre.
	primera := NuevaCaja(filepath.Join(t.TempDir(), "llave"))
	guardado, err := primera.Cifrar("clave-del-switch")
	if err != nil {
		t.Fatalf("no se pudo cifrar: %v", err)
	}

	otra := NuevaCaja(filepath.Join(t.TempDir(), "otra-llave"))
	if _, err := otra.Descifrar(guardado); err == nil {
		t.Fatal("se descifro con una llave que no era")
	}
}

func TestLaLlaveNoLaLeeCualquiera(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "llave")
	caja := NuevaCaja(ruta)
	if _, err := caja.Cifrar("algo"); err != nil {
		t.Fatalf("no se pudo cifrar: %v", err)
	}

	ficha, err := os.Stat(ruta)
	if err != nil {
		t.Fatalf("no se creo la llave: %v", err)
	}
	if permisos := ficha.Mode().Perm(); permisos != 0o600 {
		t.Fatalf("la llave quedo con permisos %o: la puede leer alguien mas", permisos)
	}
}

func TestLaMismaLlaveSirveDespuesDeReiniciar(t *testing.T) {
	// La llave se guarda: si se generara una nueva en cada arranque, todo lo
	// guardado ayer quedaria ilegible hoy.
	ruta := filepath.Join(t.TempDir(), "llave")
	guardado, err := NuevaCaja(ruta).Cifrar("clave-de-la-antena")
	if err != nil {
		t.Fatalf("no se pudo cifrar: %v", err)
	}

	claro, err := NuevaCaja(ruta).Descifrar(guardado)
	if err != nil || claro != "clave-de-la-antena" {
		t.Fatalf("tras reabrir volvio %q con error %v", claro, err)
	}
}

func TestLoQueYaEstabaEnClaroSeSigueLeyendo(t *testing.T) {
	// Una base escrita antes de que existiera el cifrado tiene textos en claro.
	// Romperse ahi dejaria inservible lo que el usuario ya habia guardado.
	caja := NuevaCaja(filepath.Join(t.TempDir(), "llave"))

	claro, err := caja.Descifrar("una clave vieja en claro")
	if err != nil {
		t.Fatalf("no se pudo leer lo viejo: %v", err)
	}
	if claro != "una clave vieja en claro" {
		t.Fatalf("volvio %q", claro)
	}
}

func TestLoVacioSeQuedaVacio(t *testing.T) {
	caja := NuevaCaja(filepath.Join(t.TempDir(), "llave"))

	if guardado, _ := caja.Cifrar(""); guardado != "" {
		t.Fatalf("un secreto vacio se guardo como %q", guardado)
	}
	if claro, _ := caja.Descifrar(""); claro != "" {
		t.Fatalf("lo vacio volvio como %q", claro)
	}
}
