package catalogo

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

// La lista de categorias vive DOS VECES: en Go, porque el servidor la valida y
// la guarda, y en Dart, porque el formulario la ofrece. No hay forma de tener
// una sola —son dos lenguajes y dos binarios—, asi que lo unico que queda es
// vigilar que no se separen.
//
// Esta es la regla de la casa sobre concordancia entre clientes, aplicada donde
// mas duele: si el formulario ofrece "ap" y el servidor guarda
// "punto_de_acceso", el contador muestra dos cubos para la misma cosa y nadie se
// entera hasta que alguien mira el informe y no le cuadra.

var claveEnDart = regexp.MustCompile(`clave:\s*'([a-z_]+)'`)

func TestLasDosListasDeCategoriasDicenLoMismo(t *testing.T) {
	ruta := filepath.Join("..", "..", "interfaz", "lib", "modelos", "categorias.dart")
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no se pudo leer la lista de la interfaz: %v", err)
	}

	var enDart []string
	for _, coincidencia := range claveEnDart.FindAllStringSubmatch(string(crudo), -1) {
		enDart = append(enDart, coincidencia[1])
	}
	sort.Strings(enDart)

	enGo := ClavesDeCategorias()

	if len(enDart) == 0 {
		t.Fatal("no se encontro ninguna categoria en la lista de la interfaz: " +
			"¿cambio el formato del archivo?")
	}
	if len(enDart) != len(enGo) {
		t.Fatalf("las listas tienen distinto tamano: Go %d, interfaz %d\nGo: %v\nDart: %v",
			len(enGo), len(enDart), enGo, enDart)
	}
	for i := range enGo {
		if enGo[i] != enDart[i] {
			t.Fatalf("las listas se separaron: Go tiene %q donde la interfaz tiene %q\nGo: %v\nDart: %v",
				enGo[i], enDart[i], enGo, enDart)
		}
	}
}

// El catalogo que se entrega tiene que usar categorias que existan. Una
// definicion con la categoria mal escrita reconoce igual el aparato, pero lo
// deja contando como "sin reconocer" sin que nadie lo note.
func TestElCatalogoQueSeEntregaUsaCategoriasDeLaLista(t *testing.T) {
	catalogo, err := Cargar([]string{filepath.Join("..", "..", "catalogo", "dispositivos")})
	if err != nil {
		t.Fatalf("no se pudo cargar el catalogo: %v", err)
	}
	definiciones := catalogo.Definiciones()
	if len(definiciones) == 0 {
		t.Fatal("el catalogo semilla esta vacio")
	}

	for _, problema := range ComprobarCategorias(definiciones) {
		t.Error(problema)
	}
}

func TestLaCategoriaDesconocidaNoSeInventa(t *testing.T) {
	// Nunca devuelve vacio: un renglon en blanco en un informe no dice si es que
	// no hay dato o si el programa fallo.
	if NombreDeCategoria("") != "Sin reconocer" {
		t.Fatalf("una categoria vacia deberia leerse como sin reconocer: %q", NombreDeCategoria(""))
	}
	if NombreDeCategoria("inventada") != "inventada" {
		t.Fatalf("una categoria desconocida deberia devolverse tal cual: %q",
			NombreDeCategoria("inventada"))
	}
	if EsCategoriaValida("ap") {
		t.Fatal("'ap' no es una categoria de MiRed: la buena es punto_de_acceso")
	}
	if !EsCategoriaValida(SwitchSimple) {
		t.Fatal("el switch no administrable tiene que estar en la lista: es la razon del modulo 15")
	}
}
