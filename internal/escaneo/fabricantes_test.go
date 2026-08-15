package escaneo

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// prepararLista deja a MiRed leyendo el archivo que arma la prueba, y no lo que
// haya instalado en el equipo donde se corre.
func prepararLista(t *testing.T, contenido string) {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "fabricantes.txt")
	if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
		t.Fatalf("no se pudo escribir la lista: %v", err)
	}
	anteriores := archivosDeFabricantes
	archivosDeFabricantes = []string{ruta}
	cargaOUI = sync.Once{}
	t.Cleanup(func() {
		archivosDeFabricantes = anteriores
		cargaOUI = sync.Once{}
	})
}

func TestElPrefijoMasLargoGana(t *testing.T) {
	// Este es el caso real que rompia el reconocimiento: la IEEE parte un bloque
	// de 24 bits entre varias empresas chicas. Si gana el corto, medio catalogo
	// de aparatos baratos sale con el nombre del mayorista.
	prepararLista(t, "8c1f64\tIEEE mayorista\n8c1f64a\tFabricante chico\n8c1f64ab0\tFabricante chiquito\n")

	if nombre := Fabricante("8c:1f:64:ab:01:02"); nombre != "Fabricante chiquito" {
		t.Fatalf("debia ganar el prefijo de 36 bits, contesto %q", nombre)
	}
	if nombre := Fabricante("8c:1f:64:a0:01:02"); nombre != "Fabricante chico" {
		t.Fatalf("debia ganar el prefijo de 28 bits, contesto %q", nombre)
	}
	if nombre := Fabricante("8c:1f:64:00:01:02"); nombre != "IEEE mayorista" {
		t.Fatalf("sin bloque chico manda el de 24 bits, contesto %q", nombre)
	}
}

func TestLaMacSeLeeComoVenga(t *testing.T) {
	prepararLista(t, "f492bf\tUbiquiti Inc\n")

	for _, mac := range []string{"f4:92:bf:11:22:33", "F4-92-BF-11-22-33", "f492bf112233", "f492.bf11.2233"} {
		if nombre := Fabricante(mac); nombre != "Ubiquiti Inc" {
			t.Fatalf("con %q contesto %q", mac, nombre)
		}
	}
}

func TestLaMacInventadaSeDiceQueEsInventada(t *testing.T) {
	prepararLista(t, "f492bf\tUbiquiti Inc\n")

	// Bit 2 del primer byte encendido: la MAC la invento el propio telefono para
	// no ser rastreado. Dejarlo vacio haria pensar en un fabricante que falta.
	if nombre := Fabricante("da:1a:79:2f:b1:a4"); nombre != "MAC aleatoria (privacidad)" {
		t.Fatalf("una MAC administrada localmente debe decirse, contesto %q", nombre)
	}
}

func TestSinArchivoQuedaLaListaCorta(t *testing.T) {
	anteriores := archivosDeFabricantes
	archivosDeFabricantes = []string{filepath.Join(t.TempDir(), "no-existe.txt")}
	cargaOUI = sync.Once{}
	t.Cleanup(func() {
		archivosDeFabricantes = anteriores
		cargaOUI = sync.Once{}
	})

	// Que falte el archivo no puede dejar a MiRed sin reconocer nada: la lista
	// embebida es el suelo.
	if nombre := Fabricante("b8:27:eb:11:22:33"); nombre != "Raspberry Pi" {
		t.Fatalf("la lista embebida deberia seguir contestando, dijo %q", nombre)
	}
	if _, cuantos := FuenteDeFabricantes(); cuantos == 0 {
		t.Fatal("la lista embebida no puede quedar vacia")
	}
}

func TestLaListaQueViajaEnElRepoSirve(t *testing.T) {
	// La lista de verdad, la que instala el paquete. Si alguien la regenera mal
	// —o la deja a medias— esto lo dice antes de empaquetar.
	ruta := filepath.Join("..", "..", "catalogo", "fabricantes.txt")
	if _, err := os.Stat(ruta); err != nil {
		t.Skipf("no esta %s: %v", ruta, err)
	}

	anteriores := archivosDeFabricantes
	archivosDeFabricantes = []string{ruta}
	cargaOUI = sync.Once{}
	t.Cleanup(func() {
		archivosDeFabricantes = anteriores
		cargaOUI = sync.Once{}
	})

	casos := map[string]string{
		"f4:92:bf:0b:8c:b6": "Ubiquiti",
		"3c:e3:6b:06:c6:f8": "Dahua",
		"e8:5a:8b:2f:17:61": "Xiaomi",
		"b8:27:eb:11:22:33": "Raspberry Pi",
	}
	for mac, esperado := range casos {
		nombre := Fabricante(mac)
		if nombre == "" {
			t.Errorf("%s se quedo sin fabricante", mac)
			continue
		}
		if !strings.Contains(strings.ToLower(nombre), strings.ToLower(esperado)) {
			t.Errorf("%s dio %q, se esperaba algo de %s", mac, nombre, esperado)
		}
	}

	if _, cuantos := FuenteDeFabricantes(); cuantos < 30000 {
		t.Errorf("la lista trae %d prefijos: parece recortada", cuantos)
	}
}
