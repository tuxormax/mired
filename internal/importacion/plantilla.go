// Package importacion lee una hoja de aparatos y la convierte en el plan de lo
// que habria que crear en una red.
//
// Nace de un caso real: la hoja de cableado de una clinica —23 rosetas, una
// columna PUERTO, otra NODO, otra UBICACION y otra OBSERVACIONES— que estaba en
// una hoja de calculo desde hacia anos. Capturar eso aparato por aparato son 23
// altas a mano, y a nadie le dan ganas.
//
// **Aqui NO se escribe nada en la base.** Este paquete lee, valida y arma un
// plan; quien lo aplica es el servidor, que sigue siendo el unico escritor.
package importacion

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tuxormax/mired/internal/catalogo"
)

// Columna es cada columna que la plantilla entiende.
//
// Las columnas se reconocen por su nombre, no por su posicion: quien llena la
// plantilla puede mover una columna de sitio, o traer su propia hoja con otros
// encabezados equivalentes, y sigue funcionando. Es lo unico que evita tener que
// reacomodar a mano cada hoja que llega de un cliente.
type Columna struct {
	Clave string `json:"clave"`
	// Sinonimos son los encabezados que se aceptan para esta columna, ya
	// normalizados. El primero es el de la plantilla.
	Sinonimos []string `json:"sinonimos"`
	// Obligatoria: sin ella el archivo no se puede importar y se dice cual falta.
	Obligatoria bool   `json:"obligatoria"`
	Ayuda       string `json:"ayuda"`
	// Ejemplo es un valor de verdad, para la guia de la pantalla. Una guia con
	// «texto» de ejemplo no ensena nada.
	Ejemplo string `json:"ejemplo"`
}

// Las claves de las columnas. Se usan como llaves del mapa de cada renglon.
const (
	ColNombre    = "NOMBRE"
	ColQueEs     = "QUE_ES"
	ColPuertos   = "PUERTOS"
	ColCuelgaDe  = "CUELGA_DE"
	ColPuerto    = "PUERTO"
	ColUbicacion = "UBICACION"
	ColIP        = "IP"
	ColMAC       = "MAC"
	ColModelo    = "MODELO"
	ColNotas     = "NOTAS"
	ColAcceso    = "ACCESO"
	ColUsuario   = "USUARIO"
	ColClave     = "CLAVE"
	ColDireccion = "DIRECCION"
)

// Columnas es la plantilla, en el orden en que conviene llenarla.
//
// Los sinonimos no son un capricho: la hoja de la clinica traia NODO,
// OBSERVACIONES y PUERTO, y con reconocerlos se importa casi tal cual, sin que
// nadie tenga que renombrar encabezados en un archivo que lleva anos en uso.
var Columnas = []Columna{
	{
		Clave:       ColNombre,
		Sinonimos:   []string{"NOMBRE", "NODO", "EQUIPO", "APARATO", "DISPOSITIVO", "HOST"},
		Obligatoria: true,
		Ayuda:       "Como se llama. No se puede repetir en la red",
		Ejemplo:     "D01",
	},
	{
		Clave:       ColQueEs,
		Sinonimos:   []string{"QUE_ES", "TIPO", "CATEGORIA", "CLASE"},
		Obligatoria: true,
		Ayuda:       "De la lista de abajo. Nunca texto libre",
		Ejemplo:     "pc",
	},
	{
		Clave:     ColPuertos,
		Sinonimos: []string{"PUERTOS", "NUM_PUERTOS", "NUMERO_DE_PUERTOS", "BOCAS"},
		Ayuda:     "Solo para switches y modems: cuantos puertos tiene. Los que sobren salen como libres en el mapa",
		Ejemplo:   "24",
	},
	{
		Clave:     ColCuelgaDe,
		Sinonimos: []string{"CUELGA_DE", "PADRE", "CONECTADO_A", "VA_A", "SWITCH"},
		Ayuda:     "El NOMBRE de otro renglon (o de un aparato que ya este en la red)",
		Ejemplo:   "switch site",
	},
	{
		Clave:     ColPuerto,
		Sinonimos: []string{"PUERTO", "BOCA", "JACK", "PUERTO_DEL_SWITCH"},
		Ayuda:     "El puerto de ESE otro aparato, no del suyo",
		Ejemplo:   "7",
	},
	{
		Clave:     ColUbicacion,
		Sinonimos: []string{"UBICACION", "LUGAR", "SITIO", "AREA", "LOCALIZACION"},
		Ayuda:     "Donde esta fisicamente. No es lo mismo que de donde cuelga",
		Ejemplo:   "cons 5",
	},
	{
		Clave: ColIP, Sinonimos: []string{"IP", "DIRECCION_IP"},
		Ayuda: "Si la tiene. Un switch tonto no tiene", Ejemplo: "192.168.1.31",
	},
	{
		Clave: ColMAC, Sinonimos: []string{"MAC", "DIRECCION_MAC", "FISICA"},
		Ayuda:   "Si se sabe. Se aceptan los doce digitos seguidos",
		Ejemplo: "b4:2e:99:3b:e3:76",
	},
	{
		Clave: ColModelo, Sinonimos: []string{"MODELO", "MARCA_Y_MODELO"},
		Ayuda: "Marca y modelo", Ejemplo: "TL-SG1024D",
	},
	{
		Clave:     ColNotas,
		Sinonimos: []string{"NOTAS", "OBSERVACIONES", "COMENTARIOS", "NOTA"},
		Ayuda:     "Lo que haya que recordar de ese punto",
		Ejemplo:   "se poncho de nuevo",
	},
	{
		Clave:     ColAcceso,
		Sinonimos: []string{"ACCESO", "TIPO_DE_ACCESO", "TIPO_ACCESO"},
		Ayuda:     "Como se entra: web, ssh, consola o app. Por omision, web",
		Ejemplo:   "web",
	},
	{
		Clave: ColUsuario, Sinonimos: []string{"USUARIO", "USER", "LOGIN"},
		Ayuda: "Con que usuario se entra a su panel", Ejemplo: "admin",
	},
	{
		Clave:     ColClave,
		Sinonimos: []string{"CLAVE", "CONTRASENA", "PASSWORD", "PASS"},
		Ayuda:     "Se guarda CIFRADA. Ojo: en el archivo va en claro",
		Ejemplo:   "",
	},
	{
		Clave:     ColDireccion,
		Sinonimos: []string{"DIRECCION", "PANEL", "URL", "DIRECCION_DEL_PANEL"},
		Ayuda:     "Su panel. Si se deja vacio se arma con la IP",
		Ejemplo:   "http://192.168.1.1",
	},
}

// ejemplosDeLaPlantilla son los renglones ya llenos que lleva la plantilla.
//
// Van en el mismo orden que [Columnas]. Los usan el CSV descargable y la guia de
// la pantalla: si cada uno tuviera los suyos, la hoja que se descarga y la que se
// explica no serian la misma.
var ejemplosDeLaPlantilla = [][]string{
	{"switch site", "switch", "24", "", "", "site", "", "", "TL-SG1024D", "", "", "", "", ""},
	{"D01", "computadora", "", "switch site", "5", "admon", "192.168.1.31", "", "", "se poncho de nuevo", "web", "admin", "", ""},
}

// columnaPorEncabezado busca a que columna corresponde un encabezado.
func columnaPorEncabezado(encabezado string) (string, bool) {
	buscado := Normalizar(encabezado)
	if buscado == "" {
		return "", false
	}
	for _, columna := range Columnas {
		for _, sinonimo := range columna.Sinonimos {
			if sinonimo == buscado {
				return columna.Clave, true
			}
		}
	}
	return "", false
}

// Normalizar deja un texto comparable: sin acentos, en mayusculas y con guion
// bajo por separador.
//
// Sirve para los encabezados y para las categorias. Sin esto, «Ubicación» y
// «UBICACION» serian dos columnas distintas, que es exactamente el tipo de
// detalle que hace que un importador «no funcione» sin decir por que.
func Normalizar(texto string) string {
	var salida strings.Builder
	for _, letra := range strings.TrimSpace(strings.ToUpper(texto)) {
		switch letra {
		case 'Á', 'À', 'Ä', 'Â':
			salida.WriteRune('A')
		case 'É', 'È', 'Ë', 'Ê':
			salida.WriteRune('E')
		case 'Í', 'Ì', 'Ï', 'Î':
			salida.WriteRune('I')
		case 'Ó', 'Ò', 'Ö', 'Ô':
			salida.WriteRune('O')
		case 'Ú', 'Ù', 'Ü', 'Û':
			salida.WriteRune('U')
		case 'Ñ':
			salida.WriteRune('N')
		case ' ', '-', '.', '/':
			salida.WriteRune('_')
		default:
			salida.WriteRune(letra)
		}
	}
	// Los espacios repetidos dejan guiones bajos repetidos: «NUM  PUERTOS».
	limpio := salida.String()
	for strings.Contains(limpio, "__") {
		limpio = strings.ReplaceAll(limpio, "__", "_")
	}
	return strings.Trim(limpio, "_")
}

// PlantillaCSV es el archivo que se descarga desde el programa para llenarlo.
//
// La ayuda de cada columna va arriba, en renglones que empiezan por `#`, y el
// lector los salta. Ponerla debajo de los encabezados seria mas comodo de leer y
// un error: al volver a subir el archivo, esa fila llegaria como un aparato mas
// y habria que rechazarla explicando por que.
//
// Lleva DOS ejemplos y no uno: con uno solo no se ve lo unico que de verdad
// cuesta entender, que es que **el switch tambien es un renglon** y que los
// demas cuelgan de el por su nombre.
func PlantillaCSV() string {
	encabezados := make([]string, 0, len(Columnas))
	for _, columna := range Columnas {
		encabezados = append(encabezados, columna.Clave)
	}

	var salida strings.Builder
	salida.WriteString("\uFEFF") // marca de codificacion, o Excel rompe los acentos
	salida.WriteString("# Plantilla de MiRed: un renglon por aparato.\n")
	salida.WriteString("# El switch tambien lleva su renglon; los demas cuelgan de el poniendo su NOMBRE en CUELGA_DE.\n")
	salida.WriteString("# El orden de los renglones da igual. Lo que empieza por # no se lee.\n")
	for _, columna := range Columnas {
		obligatoria := ""
		if columna.Obligatoria {
			obligatoria = " (obligatoria)"
		}
		fmt.Fprintf(&salida, "# %s%s: %s\n", columna.Clave, obligatoria, columna.Ayuda)
	}
	fmt.Fprintf(&salida, "%s\n", paraCSV(encabezados))
	for _, ejemplo := range ejemplosDeLaPlantilla {
		fmt.Fprintf(&salida, "%s\n", paraCSV(ejemplo))
	}
	return salida.String()
}

// paraCSV entrecomilla lo que lleva coma o comillas.
func paraCSV(campos []string) string {
	salida := make([]string, len(campos))
	for i, campo := range campos {
		if strings.ContainsAny(campo, ",\"\n") {
			salida[i] = `"` + strings.ReplaceAll(campo, `"`, `""`) + `"`
			continue
		}
		salida[i] = campo
	}
	return strings.Join(salida, ",")
}

// ------------------------------------------------------------------ guia ---

// GuiaDeLlenado es todo lo que la pantalla necesita para explicar la plantilla.
//
// Sale de **la misma definicion** con la que se lee el archivo y se arma el CSV
// de ejemplo. Si la guia se escribiera aparte, el dia que se agregue una columna
// habria dos verdades: la que el servidor acepta y la que la pantalla dice que
// acepta, y la segunda se descubre equivocada cuando alguien ya llenó la hoja.
type GuiaDeLlenado struct {
	Columnas   []Columna       `json:"columnas"`
	Categorias []CategoriaGuia `json:"categorias"`
	// Ejemplo son unos renglones ya llenos, los mismos que trae la plantilla.
	Ejemplo []map[string]string `json:"ejemplo"`
}

// CategoriaGuia es un valor que se puede escribir en QUE_ES.
type CategoriaGuia struct {
	Clave string `json:"clave"`
	// ComoSeLee es el nombre que sale en pantalla.
	ComoSeLee string `json:"comoSeLee"`
	// Apodos son las otras formas de escribirlo que tambien se aceptan.
	Apodos []string `json:"apodos"`
}

// Guia arma la guia de llenado.
func Guia() GuiaDeLlenado {
	guia := GuiaDeLlenado{Columnas: Columnas}

	for _, categoria := range catalogo.Categorias {
		if categoria.Clave == catalogo.SinReconocer {
			continue
		}
		fila := CategoriaGuia{Clave: categoria.Clave, ComoSeLee: categoria.Singular}
		// Los apodos que apuntan a esta categoria, en minusculas y sin guion
		// bajo: es como los va a escribir una persona.
		for apodo, destino := range apodos {
			if destino != categoria.Clave {
				continue
			}
			legible := strings.ToLower(strings.ReplaceAll(apodo, "_", " "))
			if legible == categoria.Clave {
				continue // el apodo es la propia clave; no aporta
			}
			fila.Apodos = append(fila.Apodos, legible)
		}
		sort.Strings(fila.Apodos)
		guia.Categorias = append(guia.Categorias, fila)
	}

	// El ejemplo es el MISMO que lleva la plantilla descargable: lo que se ve en
	// pantalla y lo que se abre en Calc tienen que ser la misma hoja.
	for _, renglon := range ejemplosDeLaPlantilla {
		fila := map[string]string{}
		for i, columna := range Columnas {
			if i < len(renglon) && renglon[i] != "" {
				fila[columna.Clave] = renglon[i]
			}
		}
		guia.Ejemplo = append(guia.Ejemplo, fila)
	}
	return guia
}
