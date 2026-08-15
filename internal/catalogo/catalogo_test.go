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

func TestLoGenericoNoLeGanaALoQueIdentifica(t *testing.T) {
	// El caso real: una antena Ubiquiti con el panel web abierto salia como
	// "Servidor web" porque esa era la unica definicion que coincidia. Con el
	// fabricante conocido, la generica ya no tiene nada que hacer.
	catalogo := catalogoDePrueba(t, map[string]map[string]string{
		"paquete": {
			"web.toml": `
nombre = "Equipo con pagina web"
categoria = "otro"
generico = true
[coincidencias]
puertos_alguno = [80, 443]
`,
			"antena.toml": `
nombre = "Ubiquiti"
categoria = "punto_de_acceso"
[coincidencias]
fabricantes = ["Ubiquiti"]
`,
		},
	})

	definicion := catalogo.Reconocer(Equipo{
		Fabricante: "Ubiquiti Inc", MAC: "f4:92:bf:0b:8c:b6", Puertos: []int{80, 443, 22},
	})
	if definicion == nil || definicion.Categoria != "punto_de_acceso" {
		t.Fatalf("debia ganar el fabricante y no la pagina web: %+v", definicion)
	}
}

func TestLoGenericoContestaCuandoNoHayNadaMejor(t *testing.T) {
	// Degradar lo generico no es callarlo: si no se sabe nada mas, decir "tiene
	// pagina web" sigue siendo mejor que no decir nada.
	catalogo := catalogoDePrueba(t, map[string]map[string]string{
		"paquete": {
			"web.toml": `
nombre = "Equipo con pagina web"
categoria = "otro"
generico = true
[coincidencias]
puertos_alguno = [80, 443]
`,
			"antena.toml": `
nombre = "Ubiquiti"
categoria = "punto_de_acceso"
[coincidencias]
fabricantes = ["Ubiquiti"]
`,
		},
	})

	definicion := catalogo.Reconocer(Equipo{MAC: "a0:95:7f:bf:5c:60", Puertos: []int{80}})
	if definicion == nil || definicion.Nombre != "Equipo con pagina web" {
		t.Fatalf("sin nada mejor debia contestar la generica: %+v", definicion)
	}
}

func TestUnaPaginaWebYaNoCuentaComoServidor(t *testing.T) {
	// Contra el catalogo real del repo: un modem con el 80 abierto no puede
	// acabar contado en la columna de "Servidores".
	catalogo, err := Cargar([]string{"../../catalogo/dispositivos"})
	if err != nil {
		t.Fatalf("no se pudo cargar el catalogo del repo: %v", err)
	}

	definicion := catalogo.Reconocer(Equipo{MAC: "a0:95:7f:bf:5c:60", Puertos: []int{80, 443, 8080}})
	if definicion != nil && definicion.Categoria == "servidor" {
		t.Fatalf("tener pagina web no hace servidor a nadie: %+v", definicion)
	}
}

func TestLoQueElAparatoDiceDeSiMismoManda(t *testing.T) {
	// Los tres casos de la red de una casa, que antes salian los tres iguales
	// —"Servidor web"— porque lo unico que se miraba era el puerto 80.
	catalogo, err := Cargar([]string{"../../catalogo/dispositivos"})
	if err != nil {
		t.Fatalf("no se pudo cargar el catalogo del repo: %v", err)
	}

	casos := []struct {
		que       string
		equipo    Equipo
		categoria string
	}{
		{
			que: "el modem del proveedor, que se anuncia por UPnP",
			equipo: Equipo{
				MAC: "a0:95:7f:bf:5c:60", Puertos: []int{80, 443, 8080},
				Huella: "Linux/3.10 UPnP/1.0 urn:schemas-upnp-org:device:InternetGatewayDevice:1",
			},
			categoria: "gateway",
		},
		{
			que: "la antena, que contesta su propio protocolo",
			equipo: Equipo{
				MAC: "f4:92:bf:0b:8c:b6", Puertos: []int{22, 80, 443},
				Huella: "NanoStation M5 airOS UBNT CASA-5G",
			},
			categoria: "punto_de_acceso",
		},
		{
			que: "el grabador de video, que habla ONVIF",
			equipo: Equipo{
				MAC: "3c:e3:6b:06:c6:f8", Puertos: []int{80, 554, 37777},
				Huella: "onvif://www.onvif.org/hardware/XVR5108HS camara ONVIF",
			},
			categoria: "camara",
		},
		{
			que: "la television, que se anuncia para recibir video",
			equipo: Equipo{
				MAC: "a0:d7:f3:b7:44:e2", Puertos: []int{80, 8001, 8080},
				Huella: "urn:schemas-upnp-org:device:MediaRenderer:1 Tizen Samsung",
			},
			categoria: "multimedia",
		},
	}

	for _, caso := range casos {
		definicion := catalogo.Reconocer(caso.equipo)
		if definicion == nil {
			t.Errorf("%s: no lo reconocio nadie", caso.que)
			continue
		}
		if definicion.Categoria != caso.categoria {
			t.Errorf("%s: salio como %q (%s) y debia ser %s",
				caso.que, definicion.Nombre, definicion.Categoria, caso.categoria)
		}
	}
}

func TestGuardarUnaDefinicionPropia(t *testing.T) {
	carpeta := t.TempDir()

	// Lo normal: una definicion valida se guarda y desde ahi se reconoce.
	guardado, err := Guardar(carpeta, "camara-acme", `
nombre = "Camara Acme"
categoria = "camara"
[coincidencias]
prefijos_mac = ["aa:bb:cc"]
`)
	if err != nil {
		t.Fatalf("no se pudo guardar: %v", err)
	}
	if guardado.Archivo != "camara-acme.toml" {
		t.Errorf("se guardo como %q", guardado.Archivo)
	}

	catalogo, err := Cargar([]string{carpeta})
	if err != nil {
		t.Fatalf("no se pudo cargar lo guardado: %v", err)
	}
	if definicion := catalogo.Reconocer(Equipo{MAC: "aa:bb:cc:11:22:33"}); definicion == nil {
		t.Fatal("lo que se acaba de guardar no reconoce a su propio aparato")
	}
}

func TestNoSeGuardaUnaDefinicionQueReconoceriaATodos(t *testing.T) {
	carpeta := t.TempDir()

	// Sin condiciones, la definicion coincidiria con TODO y le pondria el mismo
	// nombre a media red. Es el error mas facil de cometer y el mas caro de
	// descubrir despues.
	if _, err := Guardar(carpeta, "todo.toml", "nombre = \"Cualquier cosa\"\n"); err == nil {
		t.Fatal("se guardo una definicion sin ninguna condicion")
	}
	// Y un archivo roto se rechaza en la cara, no se guarda para que despues
	// aparezca como problema en la pantalla.
	if _, err := Guardar(carpeta, "roto.toml", "esto no es toml ["); err == nil {
		t.Fatal("se guardo una definicion con error de formato")
	}
	// Un nombre con rutas no puede escribir fuera de la carpeta.
	if _, err := Guardar(carpeta, "../../etc/passwd", "nombre = \"x\"\n"); err == nil {
		t.Fatal("se acepto un nombre de archivo con ruta")
	}
}

func TestLaDireccionDelAporteLlevaLaDefinicion(t *testing.T) {
	direccion := URLParaAportar("Camara Acme", "camara-acme.toml", "nombre = \"Camara Acme\"\n")

	if !strings.HasPrefix(direccion, "https://github.com/"+Repositorio+"/issues/new?") {
		t.Fatalf("la direccion no apunta al repositorio: %s", direccion)
	}
	for _, pedazo := range []string{"Camara+Acme", "camara-acme.toml", "toml"} {
		if !strings.Contains(direccion, pedazo) {
			t.Errorf("la direccion no lleva %q: %s", pedazo, direccion)
		}
	}
}

func TestRecargarCambiaLoQueSeReconoceSinReiniciar(t *testing.T) {
	// El catalogo se recarga en caliente al guardar una definicion. Si el
	// contenido no cambiara, habria que reiniciar el servicio para que sirviera
	// de algo aportar.
	carpeta := t.TempDir()
	catalogo, err := Cargar([]string{carpeta})
	if err != nil {
		t.Fatalf("no se pudo cargar: %v", err)
	}
	if definicion := catalogo.Reconocer(Equipo{MAC: "aa:bb:cc:11:22:33"}); definicion != nil {
		t.Fatal("un catalogo vacio no puede reconocer nada")
	}

	if _, err := Guardar(carpeta, "camara-acme.toml", `
nombre = "Camara Acme"
categoria = "camara"
[coincidencias]
prefijos_mac = ["aa:bb:cc"]
`); err != nil {
		t.Fatalf("no se pudo guardar: %v", err)
	}

	nuevo, err := Cargar([]string{carpeta})
	if err != nil {
		t.Fatalf("no se pudo recargar: %v", err)
	}
	catalogo.Reemplazar(nuevo)

	if definicion := catalogo.Reconocer(Equipo{MAC: "aa:bb:cc:11:22:33"}); definicion == nil {
		t.Fatal("despues de recargar deberia reconocerlo")
	}
}

func TestLaRedDeEsquipulasSeReconoce(t *testing.T) {
	// Datos de una red real escaneada el 2026-08-14: un FortiGate de puerta de
	// enlace y puntos de acceso Ruckus. Antes salian como "Equipo Linux o Unix"
	// o sin nada, que es justo lo que este modulo viene a arreglar.
	catalogo, err := Cargar([]string{"../../catalogo/dispositivos"})
	if err != nil {
		t.Fatalf("no se pudo cargar el catalogo del repo: %v", err)
	}

	casos := []struct {
		que       string
		equipo    Equipo
		categoria string
	}{
		{
			que: "el FortiGate de la puerta de enlace",
			equipo: Equipo{
				MAC: "04:d5:90:78:9b:8b", Fabricante: "Fortinet, Inc.",
				Puertos: []int{443, 541, 10443},
				Huella:  "Fortinet Ltd. FGT60FTK20012345 FortiGate 60F FGFM",
			},
			categoria: "router",
		},
		{
			que: "un punto de acceso que se llama a si mismo RuckusAP",
			equipo: Equipo{
				MAC: "00:03:52:09:b7:c8", Nombre: "RuckusAP",
				Puertos: []int{22, 80, 443},
			},
			categoria: "punto_de_acceso",
		},
		{
			que: "un aparato Tuya que se anuncia solo",
			equipo: Equipo{
				MAC:    "08:65:f0:0b:d4:ba",
				Huella: "Tuya / Smart Life gwId bf1234567890 productKey keyxxxx 3.3",
			},
			categoria: "otro",
		},
		{
			que: "un switch D-Link que dijo su modelo",
			equipo: Equipo{
				MAC: "1c:bd:b9:11:22:33", Fabricante: "D-Link",
				Modelo: "DGS-1210-10P", Huella: "D-Link DGS-1210-10P switch-recepcion",
			},
			categoria: "switch_administrable",
		},
	}

	for _, caso := range casos {
		definicion := catalogo.Reconocer(caso.equipo)
		if definicion == nil {
			t.Errorf("%s: no lo reconocio nadie", caso.que)
			continue
		}
		if definicion.Categoria != caso.categoria {
			t.Errorf("%s: salio como %q (%s) y debia ser %s",
				caso.que, definicion.Nombre, definicion.Categoria, caso.categoria)
		}
	}
}
