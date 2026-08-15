package catalogo

import (
	"fmt"
	"sort"
	"strings"
)

// La LISTA UNICA de lo que puede componer una red.
//
// Es la fuente de la verdad de las categorias, y la usan tres sitios a la vez:
//
//  1. El catalogo `.toml`: el campo `categoria` de cada definicion tiene que ser
//     una clave de aqui, y hay una prueba que lo comprueba. Antes era texto
//     libre y ya habia empezado a divergir.
//  2. El formulario de "agregar un aparato a mano": su desplegable sale de esta
//     lista, **nunca es texto libre**. Si el que se agrega a mano y el que se
//     descubre no hablan el mismo idioma, el contador muestra "AP: 1" y "Punto
//     de acceso: 2" como si fueran cosas distintas.
//  3. El contador de la pantalla de equipos, que agrupa por esto.
//
// **Esta lista viaja en paralelo con `interfaz/lib/modelos/categorias.dart`.**
// Una prueba compara las dos y falla si se separan: es la unica forma de que no
// se desincronicen en silencio, que es exactamente como se ensucian los datos.

// Categoria es un tipo de aparato que puede formar parte de una red.
type Categoria struct {
	// Clave es lo que se guarda en la base. No cambia nunca.
	Clave string
	// Singular y Plural son lo que lee una persona. El plural NO se calcula
	// pegando una "s": "switch" da "switches" y "punto de acceso" da "puntos de
	// acceso".
	Singular string
	Plural   string
	// Icono es el nombre del icono de Material que le toca.
	Icono string
	// Automatica dice si un escaneo puede reconocerla solo.
	//
	// Lo que es `false` **solo puede llegar declarado a mano**, y esa es la razon
	// de ser del modulo 15: un switch no administrable no tiene direccion, no
	// contesta a nada y ningun barrido lo va a ver jamas.
	Automatica bool
	// Explicacion dice, sin jerga, que entra en esta categoria.
	Explicacion string
}

// Categorias es la lista, en el orden en que conviene leerla: primero lo que
// arma la red, despues lo que cuelga de ella.
var Categorias = []Categoria{
	{
		Clave: "gateway", Singular: "Modem o gateway", Plural: "Modems y gateways",
		Icono: "router", Automatica: true,
		Explicacion: "La caja del proveedor de internet, la que trae el cable de la calle.",
	},
	{
		Clave: "router", Singular: "Router", Plural: "Routers",
		Icono: "router", Automatica: true,
		Explicacion: "Reparte entre redes distintas. En una casa suele ser el mismo aparato que el modem.",
	},
	{
		Clave: "switch_administrable", Singular: "Switch administrable", Plural: "Switches administrables",
		Icono: "settings_ethernet", Automatica: true,
		Explicacion: "Contesta por SNMP y puede decir que hay enchufado en cada puerto.",
	},
	{
		Clave: "switch_simple", Singular: "Switch no administrable", Plural: "Switches no administrables",
		Icono: "settings_ethernet", Automatica: false,
		Explicacion: "El switch de toda la vida: reparte y calla. No tiene direccion, " +
			"asi que ningun escaneo lo ve nunca y solo puede declararse a mano.",
	},
	{
		Clave: "punto_de_acceso", Singular: "Punto de acceso WiFi", Plural: "Puntos de acceso WiFi",
		Icono: "wifi", Automatica: true,
		Explicacion: "La antena de la que cuelga el WiFi.",
	},
	{
		Clave: "computadora", Singular: "PC o laptop", Plural: "PC y laptops",
		Icono: "computer", Automatica: true,
		Explicacion: "Equipos de trabajo, por cable o por WiFi.",
	},
	{
		Clave: "telefono", Singular: "Telefono o celular", Plural: "Telefonos y celulares",
		Icono: "smartphone", Automatica: true,
		Explicacion: "Celulares, tabletas y telefonos IP.",
	},
	{
		Clave: "impresora", Singular: "Impresora", Plural: "Impresoras",
		Icono: "print", Automatica: true,
		Explicacion: "Cualquier cosa que imprima por red.",
	},
	{
		Clave: "camara", Singular: "Camara o grabador", Plural: "Camaras y grabadores",
		Icono: "videocam", Automatica: true,
		Explicacion: "Camaras IP y los grabadores DVR o NVR de los que cuelgan.",
	},
	{
		Clave: "servidor", Singular: "Servidor", Plural: "Servidores",
		Icono: "dns", Automatica: true,
		Explicacion: "Equipos que dan un servicio: web, base de datos, virtualizacion.",
	},
	{
		Clave: "almacenamiento", Singular: "Almacenamiento en red", Plural: "Almacenamientos en red",
		Icono: "storage", Automatica: true,
		Explicacion: "NAS y discos de red.",
	},
	{
		Clave: "multimedia", Singular: "TV o multimedia", Plural: "TV y multimedia",
		Icono: "cast", Automatica: true,
		Explicacion: "Televisiones, Chromecast, consolas y bocinas.",
	},
	{
		Clave: "otro", Singular: "Otro aparato", Plural: "Otros aparatos",
		Icono: "devices_other", Automatica: true,
		Explicacion: "Lo que no encaja en ninguna de las de arriba pero se sabe que es.",
	},
	{
		Clave: "sin_reconocer", Singular: "Sin reconocer", Plural: "Sin reconocer",
		Icono: "help_outline", Automatica: true,
		Explicacion: "Se vio en la red pero nadie supo que es. Aqui es donde el catalogo " +
			"necesita crecer, y desde la ficha se puede proponer su definicion.",
	},
}

// SinReconocer es donde caen los equipos que nadie identifico.
//
// **No se esconden ni se reparten entre las demas.** Un inventario que calla lo
// que no supo clasificar se lee como si estuviera completo, y justamente ahi es
// donde hay que mirar: un aparato desconocido en la red es lo primero que
// interesa de un mapa.
const SinReconocer = "sin_reconocer"

// SwitchAdministrable la asigna SNMP, no el catalogo: si un equipo contesto que
// es un switch, eso manda sobre cualquier deduccion por puertos abiertos.
const SwitchAdministrable = "switch_administrable"

// SwitchSimple es la categoria que da sentido al modulo 15.
const SwitchSimple = "switch_simple"

var porClave = func() map[string]Categoria {
	indice := make(map[string]Categoria, len(Categorias))
	for _, categoria := range Categorias {
		indice[categoria.Clave] = categoria
	}
	return indice
}()

// BuscarCategoria devuelve la categoria de una clave.
func BuscarCategoria(clave string) (Categoria, bool) {
	categoria, hay := porClave[strings.TrimSpace(clave)]
	return categoria, hay
}

// EsCategoriaValida dice si una clave existe en la lista.
func EsCategoriaValida(clave string) bool {
	_, hay := porClave[strings.TrimSpace(clave)]
	return hay
}

// NombreDeCategoria devuelve como se lee una clave, o la clave misma si no se
// conoce. Nunca devuelve vacio: un renglon en blanco en un informe no dice si es
// que no hay dato o si el programa fallo.
func NombreDeCategoria(clave string) string {
	if categoria, hay := BuscarCategoria(clave); hay {
		return categoria.Singular
	}
	if strings.TrimSpace(clave) == "" {
		return "Sin reconocer"
	}
	return clave
}

// ClavesDeCategorias devuelve las claves ordenadas, para comparar listas.
func ClavesDeCategorias() []string {
	claves := make([]string, 0, len(Categorias))
	for _, categoria := range Categorias {
		claves = append(claves, categoria.Clave)
	}
	sort.Strings(claves)
	return claves
}

// ComprobarCategorias revisa que toda definicion del catalogo use una categoria
// de la lista.
//
// Se llama al cargar el catalogo y **no impide arrancar**: una definicion con la
// categoria mal escrita se sigue usando para reconocer el aparato, solo que
// contara como "sin reconocer". Tumbar el servicio por un archivo que alguien
// aporto seria peor que el problema.
func ComprobarCategorias(definiciones []Definicion) []error {
	var problemas []error
	for _, definicion := range definiciones {
		if definicion.Categoria == "" {
			problemas = append(problemas, fmt.Errorf(
				"%s: no dice de que categoria es", definicion.Archivo))
			continue
		}
		if !EsCategoriaValida(definicion.Categoria) {
			problemas = append(problemas, fmt.Errorf(
				"%s: la categoria %q no esta en la lista de MiRed", definicion.Archivo, definicion.Categoria))
		}
	}
	return problemas
}
