package importacion

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/tuxormax/mired/internal/catalogo"
)

// Aqui se decide QUE se va a hacer con cada renglon, sin hacer nada todavia.
//
// El plan se ensena antes de tocar la base y esa es toda su razon de ser:
// importar 23 aparatos y descubrir despues que tres estaban mal significa
// borrarlos a mano de uno en uno. Aqui se ve antes, con el renglon senalado y el
// motivo escrito en cristiano.

// Lo que se puede hacer con un renglon.
const (
	AccionCrear      = "crear"
	AccionActualizar = "actualizar"
	AccionRechazar   = "rechazar"
)

// Que hacer con los que ya existen. Lo elige quien importa, viendo el plan.
const (
	RepetidosActualizar = "actualizar"
	RepetidosSaltar     = "saltar"
)

// Renglon es un renglon del archivo, ya validado y traducido a lo que MiRed
// guarda.
type Renglon struct {
	Renglon int    `json:"renglon"`
	Nombre  string `json:"nombre"`
	Accion  string `json:"accion"`
	// Motivo explica el rechazo, en cristiano y diciendo que hacer.
	Motivo string `json:"motivo,omitempty"`
	// Aviso es para lo que SI se importa pero conviene mirar.
	Aviso string `json:"aviso,omitempty"`

	Categoria string `json:"categoria,omitempty"`
	Tipo      string `json:"tipo,omitempty"`
	Puertos   int    `json:"puertos,omitempty"`

	CuelgaDe     string `json:"cuelgaDe,omitempty"`
	PuertoTipo   string `json:"puertoTipo,omitempty"`
	PuertoNumero int    `json:"puertoNumero,omitempty"`

	Ubicacion string `json:"ubicacion,omitempty"`
	IP        string `json:"ip,omitempty"`
	MAC       string `json:"mac,omitempty"`
	Modelo    string `json:"modelo,omitempty"`
	Notas     string `json:"notas,omitempty"`

	Acceso    string `json:"acceso,omitempty"`
	Usuario   string `json:"usuario,omitempty"`
	Clave     string `json:"clave,omitempty"`
	Direccion string `json:"direccion,omitempty"`

	// ExisteID es el equipo que ya estaba con ese nombre, si lo hay.
	ExisteID int64 `json:"existeId,omitempty"`
}

// TieneCable dice si de este renglon sale ademas un cable que dibujar.
func (r Renglon) TieneCable() bool {
	return r.CuelgaDe != "" && r.PuertoNumero > 0
}

// Plan es todo lo que se va a hacer con el archivo.
type Plan struct {
	Renglones []Renglon `json:"renglones"`
	// Ignoradas son las columnas del archivo que MiRed no entiende.
	Ignoradas []string `json:"ignoradas,omitempty"`
	// Reconocidas son las que si.
	Reconocidas []string `json:"reconocidas"`

	Crear      int `json:"crear"`
	Actualizar int `json:"actualizar"`
	Rechazar   int `json:"rechazar"`
	Cables     int `json:"cables"`
	// ConClave son los renglones que traen contrasena. Se cuentan aparte para
	// poder avisar de que el archivo lleva claves en claro.
	ConClave int `json:"conClave"`
}

// Planear valida la hoja contra lo que ya hay en la red.
//
// [existentes] son los equipos que ya estan, con su nombre normalizado como
// llave. Se pasa hecho en vez de consultarlo aqui porque este paquete **no toca
// la base**: es lo que permite probarlo entero sin levantar nada.
func Planear(hoja Hoja, existentes map[string]int64) Plan {
	plan := Plan{Ignoradas: hoja.Ignoradas, Reconocidas: hoja.Reconocidas}

	// Los nombres del propio archivo cuentan como padres validos: en la hoja de
	// la clinica el switch esta en un renglon y sus 23 nodos lo nombran, y
	// exigir que existiera antes obligaria a importar dos veces.
	enElArchivo := map[string]int{}
	for _, fila := range hoja.Filas {
		nombre := Normalizar(fila.Valor(ColNombre))
		if nombre == "" {
			continue
		}
		enElArchivo[nombre]++
	}

	vistos := map[string]int{}
	for _, fila := range hoja.Filas {
		renglon := planearFila(fila, existentes, enElArchivo, vistos)
		switch renglon.Accion {
		case AccionCrear:
			plan.Crear++
		case AccionActualizar:
			plan.Actualizar++
		default:
			plan.Rechazar++
		}
		if renglon.Accion != AccionRechazar {
			if renglon.TieneCable() {
				plan.Cables++
			}
			if renglon.Clave != "" {
				plan.ConClave++
			}
		}
		plan.Renglones = append(plan.Renglones, renglon)
	}
	return plan
}

func planearFila(fila Fila, existentes map[string]int64, enElArchivo, vistos map[string]int) Renglon {
	renglon := Renglon{Renglon: fila.Renglon, Nombre: fila.Valor(ColNombre)}

	rechazar := func(motivo string) Renglon {
		renglon.Accion = AccionRechazar
		renglon.Motivo = motivo
		return renglon
	}

	if renglon.Nombre == "" {
		return rechazar("le falta el NOMBRE")
	}
	if len([]rune(renglon.Nombre)) > 120 {
		return rechazar("el NOMBRE pasa de 120 caracteres")
	}

	// Repetido dentro del propio archivo: el segundo no se puede crear, porque el
	// nombre es unico en la red. Se senala el renglon anterior para que sea facil
	// de encontrar.
	normalizado := Normalizar(renglon.Nombre)
	if antes, repetido := vistos[normalizado]; repetido {
		return rechazar(fmt.Sprintf(
			"«%s» ya venia en el renglon %d de este mismo archivo", renglon.Nombre, antes))
	}
	vistos[normalizado] = fila.Renglon

	categoria, tipo, err := categoriaDe(fila.Valor(ColQueEs))
	if err != nil {
		return rechazar(err.Error())
	}
	renglon.Categoria, renglon.Tipo = categoria, tipo

	if crudo := fila.Valor(ColPuertos); crudo != "" {
		cuantos, err := strconv.Atoi(crudo)
		if err != nil || cuantos < 0 || cuantos > 512 {
			return rechazar(fmt.Sprintf(
				"PUERTOS dice «%s»; tiene que ser un numero entre 0 y 512", crudo))
		}
		renglon.Puertos = cuantos
	}

	if ip := fila.Valor(ColIP); ip != "" {
		if net.ParseIP(ip) == nil {
			return rechazar(fmt.Sprintf("«%s» no es una direccion IP", ip))
		}
		renglon.IP = ip
	}

	if mac := fila.Valor(ColMAC); mac != "" {
		limpia, err := macValida(mac)
		if err != nil {
			return rechazar(err.Error())
		}
		renglon.MAC = limpia
	}

	renglon.Ubicacion = recortar(fila.Valor(ColUbicacion), 120)
	renglon.Modelo = recortar(fila.Valor(ColModelo), 120)
	renglon.Notas = recortar(fila.Valor(ColNotas), 2000)

	// --- de donde cuelga
	renglon.CuelgaDe = fila.Valor(ColCuelgaDe)
	puertoCrudo := fila.Valor(ColPuerto)

	if renglon.CuelgaDe != "" {
		padre := Normalizar(renglon.CuelgaDe)
		if padre == normalizado {
			return rechazar("un aparato no puede colgar de si mismo")
		}
		_, estaEnLaBase := existentes[padre]
		if _, estaEnElArchivo := enElArchivo[padre]; !estaEnElArchivo && !estaEnLaBase {
			return rechazar(fmt.Sprintf(
				"CUELGA_DE dice «%s» y no hay ningun aparato con ese nombre, "+
					"ni en la red ni en este archivo", renglon.CuelgaDe))
		}
	}

	if puertoCrudo != "" {
		tipoPuerto, numero, err := puertoDe(puertoCrudo)
		if err != nil {
			return rechazar(err.Error())
		}
		if renglon.CuelgaDe == "" {
			return rechazar(fmt.Sprintf(
				"dice PUERTO «%s» pero no dice CUELGA_DE: falta el aparato al que pertenece "+
					"ese puerto", puertoCrudo))
		}
		renglon.PuertoTipo, renglon.PuertoNumero = tipoPuerto, numero
	} else if renglon.CuelgaDe != "" {
		renglon.Aviso = fmt.Sprintf(
			"se crea, pero SIN cable: dice que cuelga de «%s» y no dice por que puerto",
			renglon.CuelgaDe)
		renglon.CuelgaDe = ""
	}

	// --- como se entra al aparato
	acceso, err := accesoDe(fila.Valor(ColAcceso))
	if err != nil {
		return rechazar(err.Error())
	}
	renglon.Usuario = recortar(fila.Valor(ColUsuario), 120)
	renglon.Clave = fila.Valor(ColClave)
	renglon.Direccion = recortar(fila.Valor(ColDireccion), 300)
	if renglon.Usuario != "" || renglon.Clave != "" || renglon.Direccion != "" {
		renglon.Acceso = acceso
		// Sin direccion, el panel de un aparato con IP es su IP. Es lo que
		// alguien escribiria a mano, y ahorra una columna en el archivo.
		if renglon.Direccion == "" && renglon.IP != "" {
			renglon.Direccion = "http://" + renglon.IP
		}
	}

	if id, existe := existentes[normalizado]; existe {
		renglon.Accion = AccionActualizar
		renglon.ExisteID = id
		return renglon
	}
	renglon.Accion = AccionCrear
	return renglon
}

// categoriaDe traduce lo que escribio una persona a una clave del catalogo.
//
// Se acepta la clave, el nombre que sale en pantalla, el plural y un punado de
// formas de decirlo que usa todo el mundo («ap», «nvr», «pc»). Lo que no se
// reconoce **se rechaza con la lista delante**: inventar una categoria rompe el
// contador de la red en silencio, que es la peor forma de romperse.
func categoriaDe(crudo string) (string, string, error) {
	texto := Normalizar(crudo)
	if texto == "" {
		return "", "", fmt.Errorf("le falta QUE_ES. Ponga una de: %s", listaDeCategorias())
	}

	if clave, hay := apodos[texto]; hay {
		texto = Normalizar(clave)
	}

	for _, categoria := range catalogo.Categorias {
		if categoria.Clave == catalogo.SinReconocer {
			continue
		}
		if texto == Normalizar(categoria.Clave) ||
			texto == Normalizar(categoria.Singular) ||
			texto == Normalizar(categoria.Plural) {
			return categoria.Clave, categoria.Singular, nil
		}
	}
	return "", "", fmt.Errorf(
		"«%s» no es una categoria de MiRed. Ponga una de: %s", crudo, listaDeCategorias())
}

// apodos son las formas cortas con que la gente escribe una categoria.
//
// «switch» a secas es el **no administrable**: el que se declara a mano es
// justamente el que ningun escaneo ve, y si fuera administrable contestaria
// SNMP y no haria falta capturarlo.
var apodos = map[string]string{
	"SWITCH":            "switch_simple",
	"SWITCH_TONTO":      "switch_simple",
	"SWITCH_SIMPLE":     "switch_simple",
	"SWITCH_ADMIN":      "switch_administrable",
	"MODEM":             "gateway",
	"ONT":               "gateway",
	"MODEM_ROUTER":      "gateway",
	"AP":                "punto_de_acceso",
	"ACCESS_POINT":      "punto_de_acceso",
	"ANTENA":            "punto_de_acceso",
	"PC":                "computadora",
	"LAPTOP":            "computadora",
	"COMPUTADORA":       "computadora",
	"CPU":               "computadora",
	"NVR":               "camara",
	"DVR":               "camara",
	"CAMARA":            "camara",
	"NAS":               "almacenamiento",
	"TV":                "multimedia",
	"TELEVISION":        "multimedia",
	"CELULAR":           "telefono",
	"TELEFONO_IP":       "telefono",
	"VOIP":              "telefono",
	"SERVIDOR":          "servidor",
	"IMPRESORA":         "impresora",
	"MULTIFUNCIONAL":    "impresora",
	"RELOJ_CHECADOR":    "otro",
	"OTRO":              "otro",
	"DESCONOCIDO":       "otro",
	"FIREWALL":          "router",
	"ROUTER":            "router",
	"CONTROL_DE_ACCESO": "otro",
}

func listaDeCategorias() string {
	nombres := []string{}
	for _, categoria := range catalogo.Categorias {
		if categoria.Clave == catalogo.SinReconocer {
			continue
		}
		nombres = append(nombres, categoria.Clave)
	}
	return strings.Join(nombres, ", ")
}

// puertoDe entiende «7», «LAN 7», «lan7» y «WAN 1».
//
// Lo que NO entiende es «modem 2», que es como venia en la hoja de la clinica el
// unico nodo que no colgaba del switch: ahi hay dos datos —de que aparato y que
// puerto— y van en dos columnas. Se dice asi, senalando la columna que falta.
func puertoDe(crudo string) (string, int, error) {
	texto := Normalizar(crudo)

	tipo := "lan"
	for _, posible := range []string{"LAN", "WAN", "DMZ", "SFP", "CONSOLA"} {
		if strings.HasPrefix(texto, posible) {
			tipo = strings.ToLower(posible)
			texto = strings.TrimSpace(strings.Trim(strings.TrimPrefix(texto, posible), "_"))
			break
		}
	}

	numero, err := strconv.Atoi(texto)
	if err != nil || numero < 1 || numero > 512 {
		return "", 0, fmt.Errorf(
			"PUERTO dice «%s» y no se entiende. Escriba solo el numero (7) o el nombre "+
				"del puerto (LAN 7, WAN 1). Si «%s» es otro aparato, va en CUELGA_DE",
			crudo, crudo)
	}
	return tipo, numero, nil
}

func accesoDe(crudo string) (string, error) {
	switch Normalizar(crudo) {
	case "", "WEB", "PANEL", "PANEL_WEB", "HTTP", "HTTPS":
		return "web", nil
	case "SSH":
		return "ssh", nil
	case "CONSOLA", "SERIAL", "TELNET":
		return "consola", nil
	case "APP", "APLICACION", "NUBE":
		return "app", nil
	default:
		return "", fmt.Errorf(
			"ACCESO dice «%s»; solo puede ser panel web, ssh, consola o app", crudo)
	}
}

func macValida(crudo string) (string, error) {
	limpia := strings.ToLower(strings.TrimSpace(crudo))
	limpia = strings.NewReplacer("-", ":", ".", "", " ", "").Replace(limpia)
	// Doce digitos seguidos, como los escribe media herramienta del mundo.
	if len(limpia) == 12 && !strings.Contains(limpia, ":") {
		var conDosPuntos []string
		for i := 0; i < 12; i += 2 {
			conDosPuntos = append(conDosPuntos, limpia[i:i+2])
		}
		limpia = strings.Join(conDosPuntos, ":")
	}
	if _, err := net.ParseMAC(limpia); err != nil {
		return "", fmt.Errorf("«%s» no es una direccion MAC", crudo)
	}
	return limpia, nil
}

func recortar(texto string, largo int) string {
	letras := []rune(texto)
	if len(letras) <= largo {
		return texto
	}
	return string(letras[:largo])
}
