package catalogo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// catalogoDePrueba escribe definiciones en carpetas temporales y las carga.
func catalogoDePrueba(t *testing.T, archivos map[string]map[string]string) *Catalogo {
	t.Helper()

	var carpetas []string
	for carpeta, contenidos := range archivos {
		ruta := filepath.Join(t.TempDir(), carpeta)
		if err := os.MkdirAll(ruta, 0o755); err != nil {
			t.Fatalf("no se pudo crear %s: %v", ruta, err)
		}
		for nombre, contenido := range contenidos {
			if err := os.WriteFile(filepath.Join(ruta, nombre), []byte(contenido), 0o644); err != nil {
				t.Fatalf("no se pudo escribir %s: %v", nombre, err)
			}
		}
		carpetas = append(carpetas, ruta)
	}

	catalogo, err := Cargar(carpetas)
	if err != nil {
		t.Fatalf("no se pudo cargar el catalogo: %v", err)
	}
	return catalogo
}

func TestReconoceLoMasEspecifico(t *testing.T) {
	// Dos definiciones coinciden: gana la que describe mejor al aparato, no la
	// que se cargo primero.
	catalogo := catalogoDePrueba(t, map[string]map[string]string{
		"paquete": {
			"generica.toml": `
nombre = "Impresora de red"
categoria = "impresora"
[coincidencias]
puertos_alguno = [9100, 631]
`,
			"hp.toml": `
nombre = "Impresora HP"
categoria = "impresora"
[coincidencias]
fabricantes = ["HP"]
puertos_todos = [9100]
`,
		},
	})

	definicion := catalogo.Reconocer(Equipo{
		IP: "192.168.1.50", MAC: "3c:d9:2b:11:22:33",
		Fabricante: "HP", Puertos: []int{80, 9100},
	})
	if definicion == nil {
		t.Fatal("deberia haber reconocido la impresora")
	}
	if definicion.Nombre != "Impresora HP" {
		t.Fatalf("gano la definicion menos especifica: %s", definicion.Nombre)
	}
}

func TestTodasLasCondicionesDebenCumplirse(t *testing.T) {
	catalogo := catalogoDePrueba(t, map[string]map[string]string{
		"paquete": {
			"hp.toml": `
nombre = "Impresora HP"
[coincidencias]
fabricantes = ["HP"]
puertos_todos = [9100]
`,
		},
	})

	// Es HP pero no tiene el 9100: no es la impresora.
	if definicion := catalogo.Reconocer(Equipo{Fabricante: "HP", Puertos: []int{80}}); definicion != nil {
		t.Fatalf("no deberia reconocerlo: %s", definicion.Nombre)
	}
	// Tiene el 9100 pero es de otra marca: tampoco.
	if definicion := catalogo.Reconocer(Equipo{Fabricante: "Canon", Puertos: []int{9100}}); definicion != nil {
		t.Fatalf("no deberia reconocerlo: %s", definicion.Nombre)
	}
}

func TestLaCarpetaDelUsuarioMandaSobreLaDelPaquete(t *testing.T) {
	// Quien corrige una definicion del paquete no tiene que esperar a que le
	// acepten el cambio: la pisa en su equipo con el mismo nombre de archivo.
	base := t.TempDir()
	paquete := filepath.Join(base, "paquete")
	propio := filepath.Join(base, "propio")
	os.MkdirAll(paquete, 0o755)
	os.MkdirAll(propio, 0o755)

	os.WriteFile(filepath.Join(paquete, "camara.toml"), []byte(`
nombre = "Camara IP"
[coincidencias]
puertos_todos = [554]
`), 0o644)
	os.WriteFile(filepath.Join(propio, "camara.toml"), []byte(`
nombre = "Camara del pasillo"
[coincidencias]
puertos_todos = [554]
`), 0o644)

	catalogo, err := Cargar([]string{paquete, propio})
	if err != nil {
		t.Fatalf("no se pudo cargar: %v", err)
	}
	if len(catalogo.Definiciones()) != 1 {
		t.Fatalf("deberia quedar una sola definicion: %d", len(catalogo.Definiciones()))
	}

	definicion := catalogo.Reconocer(Equipo{Puertos: []int{554}})
	if definicion == nil || definicion.Nombre != "Camara del pasillo" {
		t.Fatalf("la definicion propia deberia mandar: %+v", definicion)
	}
}

func TestUnArchivoRotoNoTumbaElCatalogo(t *testing.T) {
	catalogo := catalogoDePrueba(t, map[string]map[string]string{
		"paquete": {
			"roto.toml": "esto no es = = toml valido [[[",
			"buena.toml": `
nombre = "Camara IP"
[coincidencias]
puertos_todos = [554]
`,
		},
	})

	if len(catalogo.Definiciones()) != 1 {
		t.Fatalf("la definicion buena deberia cargarse igual: %d", len(catalogo.Definiciones()))
	}
	if len(catalogo.Problemas()) != 1 {
		t.Fatalf("el archivo roto deberia reportarse: %+v", catalogo.Problemas())
	}
	// Y el problema dice cual es el archivo, para poder arreglarlo.
	if !strings.Contains(catalogo.Problemas()[0], "roto.toml") {
		t.Fatalf("el problema no dice que archivo fallo: %s", catalogo.Problemas()[0])
	}
}

func TestUnaDefinicionSinCondicionesNoAtrapaTodo(t *testing.T) {
	// Una definicion vacia coincidiria con cualquier cosa. Eso no es un
	// dispositivo, es un error de quien lo escribio.
	catalogo := catalogoDePrueba(t, map[string]map[string]string{
		"paquete": {
			"vacia.toml": `nombre = "Cualquier cosa"`,
		},
	})

	if definicion := catalogo.Reconocer(Equipo{IP: "192.168.1.5"}); definicion != nil {
		t.Fatalf("una definicion sin condiciones no deberia coincidir: %s", definicion.Nombre)
	}
}

func TestPrefijoDeMacReconoceSinQueElEquipoConteste(t *testing.T) {
	catalogo := catalogoDePrueba(t, map[string]map[string]string{
		"paquete": {
			"pi.toml": `
nombre = "Raspberry Pi"
[coincidencias]
prefijos_mac = ["b8:27:eb"]
`,
		},
	})

	// Sin puertos, sin nombre, sin nada: solo la MAC. Es la senal mas fuerte.
	definicion := catalogo.Reconocer(Equipo{MAC: "B8:27:EB:AA:BB:CC"})
	if definicion == nil || definicion.Nombre != "Raspberry Pi" {
		t.Fatalf("deberia reconocerlo solo por la MAC: %+v", definicion)
	}
}

func TestProponerArmaUnTomlUtilizable(t *testing.T) {
	propuesta := Proponer(Equipo{
		IP: "192.168.1.77", MAC: "aa:bb:cc:dd:ee:ff", Fabricante: "Acme",
		Puertos: []int{80, 8443}, Banners: []string{"Acme Camera v2"},
	}, "Camara Acme")

	for _, esperado := range []string{
		`nombre = "Camara Acme"`,
		`prefijos_mac = ["aa:bb:cc"]`,
		`fabricantes = ["Acme"]`,
		"puertos_todos = [80, 8443]",
		"Acme Camera v2",
	} {
		if !strings.Contains(propuesta, esperado) {
			t.Errorf("la propuesta no incluye %q:\n%s", esperado, propuesta)
		}
	}

	// Y lo propuesto tiene que poder cargarse de vuelta: si no, no sirve de nada.
	carpeta := t.TempDir()
	if err := os.WriteFile(filepath.Join(carpeta, "propuesta.toml"), []byte(propuesta), 0o644); err != nil {
		t.Fatalf("no se pudo escribir: %v", err)
	}
	catalogo, err := Cargar([]string{carpeta})
	if err != nil {
		t.Fatalf("no se pudo cargar la propuesta: %v", err)
	}
	if len(catalogo.Definiciones()) != 1 {
		t.Fatalf("la propuesta generada no se pudo cargar: %+v", catalogo.Problemas())
	}
}

func TestElCatalogoQueTraeElPaqueteEsValido(t *testing.T) {
	// Carga las definiciones reales del repo: un `.toml` mal escrito ahi dejaria
	// sin reconocer a todo el mundo, y es justo lo que esta prueba evita.
	catalogo, err := Cargar([]string{"../../catalogo/dispositivos"})
	if err != nil {
		t.Fatalf("no se pudo cargar el catalogo del repo: %v", err)
	}
	if len(catalogo.Problemas()) > 0 {
		t.Fatalf("el catalogo del repo tiene archivos con problemas: %+v", catalogo.Problemas())
	}
	if len(catalogo.Definiciones()) < 10 {
		t.Fatalf("se esperaban al menos 10 definiciones y hay %d", len(catalogo.Definiciones()))
	}

	// Y una impresora HP de verdad tiene que salir reconocida.
	definicion := catalogo.Reconocer(Equipo{
		Fabricante: "HP", MAC: "3c:d9:2b:11:22:33", Puertos: []int{80, 9100},
	})
	if definicion == nil || definicion.Categoria != "impresora" {
		t.Fatalf("el catalogo del repo no reconocio una impresora HP: %+v", definicion)
	}
}
