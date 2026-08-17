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
	"strings"
)

// Columna es cada columna que la plantilla entiende.
//
// Las columnas se reconocen por su nombre, no por su posicion: quien llena la
// plantilla puede mover una columna de sitio, o traer su propia hoja con otros
// encabezados equivalentes, y sigue funcionando. Es lo unico que evita tener que
// reacomodar a mano cada hoja que llega de un cliente.
type Columna struct {
	Clave string
	// Sinonimos son los encabezados que se aceptan para esta columna, ya
	// normalizados. El primero es el de la plantilla.
	Sinonimos []string
	// Obligatoria: sin ella el archivo no se puede importar y se dice cual falta.
	Obligatoria bool
	Ayuda       string
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
		Ayuda:       "Como se llama: D01, serv1, switch site",
	},
	{
		Clave:       ColQueEs,
		Sinonimos:   []string{"QUE_ES", "TIPO", "CATEGORIA", "CLASE"},
		Obligatoria: true,
		Ayuda:       "De la lista de MiRed: switch, computadora, camara, impresora, otro...",
	},
	{
		Clave:     ColPuertos,
		Sinonimos: []string{"PUERTOS", "NUM_PUERTOS", "NUMERO_DE_PUERTOS", "BOCAS"},
		Ayuda:     "Solo para switches y modems: cuantos puertos tiene (24)",
	},
	{
		Clave:     ColCuelgaDe,
		Sinonimos: []string{"CUELGA_DE", "PADRE", "CONECTADO_A", "VA_A", "SWITCH"},
		Ayuda:     "El NOMBRE del aparato del que cuelga: switch site",
	},
	{
		Clave:     ColPuerto,
		Sinonimos: []string{"PUERTO", "BOCA", "JACK", "PUERTO_DEL_SWITCH"},
		Ayuda:     "El puerto de ESE aparato: 7, o LAN 7, o WAN 1",
	},
	{
		Clave:     ColUbicacion,
		Sinonimos: []string{"UBICACION", "LUGAR", "SITIO", "AREA", "LOCALIZACION"},
		Ayuda:     "Donde esta: farmacia, cons 5, rack del site",
	},
	{Clave: ColIP, Sinonimos: []string{"IP", "DIRECCION_IP"}, Ayuda: "Cuando se sepa"},
	{Clave: ColMAC, Sinonimos: []string{"MAC", "DIRECCION_MAC", "FISICA"}, Ayuda: "Cuando se sepa"},
	{Clave: ColModelo, Sinonimos: []string{"MODELO", "MARCA_Y_MODELO"}, Ayuda: "TL-SG1024D"},
	{
		Clave:     ColNotas,
		Sinonimos: []string{"NOTAS", "OBSERVACIONES", "COMENTARIOS", "NOTA"},
		Ayuda:     "Se poncho de nuevo, lockers hombres...",
	},
	{
		Clave:     ColAcceso,
		Sinonimos: []string{"ACCESO", "TIPO_DE_ACCESO", "TIPO_ACCESO"},
		Ayuda:     "Como se entra al aparato: panel web, ssh, consola o app. Por omision, panel web",
	},
	{Clave: ColUsuario, Sinonimos: []string{"USUARIO", "USER", "LOGIN"}, Ayuda: "admin"},
	{
		Clave:     ColClave,
		Sinonimos: []string{"CLAVE", "CONTRASENA", "PASSWORD", "PASS"},
		Ayuda:     "Se guarda CIFRADA. Ojo: en el archivo va en claro",
	},
	{
		Clave:     ColDireccion,
		Sinonimos: []string{"DIRECCION", "PANEL", "URL", "DIRECCION_DEL_PANEL"},
		Ayuda:     "http://192.168.1.1  — si se deja vacio se arma con la IP",
	},
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

	ejemplos := [][]string{
		{"switch site", "switch", "24", "", "", "site", "", "", "TL-SG1024D", "", "", "", "", ""},
		{"D01", "computadora", "", "switch site", "5", "admon", "192.168.1.31", "", "", "se poncho de nuevo", "web", "admin", "", ""},
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
	for _, ejemplo := range ejemplos {
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
