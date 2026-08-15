// Paquete catalogo reconoce que es cada aparato de la red.
//
// Esta es la apuesta del proyecto para no depender de una sola persona: las
// definiciones NO son codigo, son archivos `.toml` que cualquiera puede escribir
// sin saber Go, copiar a una carpeta y usar reiniciando el servicio.
//
// Se cargan de dos lugares, en este orden:
//  1. /usr/share/mired/dispositivos/  — los que trae el paquete
//  2. /etc/mired/dispositivos/        — los del usuario, que MANDAN sobre los
//     anteriores (mismo nombre de archivo = reemplazo)
//
// Asi, quien corrija una definicion del paquete no tiene que esperar a que le
// acepten el cambio: la pisa en su equipo y sigue trabajando.
package catalogo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/pelletier/go-toml/v2"
)

// Carpetas donde se buscan definiciones, de menor a mayor precedencia.
var CarpetasPorOmision = []string{
	"/usr/share/mired/dispositivos",
	"/etc/mired/dispositivos",
}

// Definicion es un dispositivo reconocible.
type Definicion struct {
	// Archivo es de donde salio, para poder decir quien reconocio que.
	Archivo string `toml:"-"`

	Nombre      string `toml:"nombre"`
	Categoria   string `toml:"categoria"`
	Icono       string `toml:"icono"`
	Descripcion string `toml:"descripcion"`
	Aporta      string `toml:"aporta"`

	// Generico marca una definicion que describe un SINTOMA, no un aparato:
	// "tiene el 80 abierto", "acepta SSH". Casi todo aparato de red trae panel
	// web, asi que esa senal no identifica a nadie.
	//
	// Una definicion generica NUNCA le gana a una que si identifica: solo se usa
	// cuando ninguna otra coincidio. Sin esto, un modem, una antena y una
	// televisión salen los tres como "Servidor web", que es la peor respuesta
	// posible: parece un dato y no lo es.
	Generico bool `toml:"generico"`

	Coincidencias Coincidencias `toml:"coincidencias"`
}

// Coincidencias son las condiciones para decir "esto es uno de estos".
//
// Todas las que esten llenas deben cumplirse (Y); dentro de cada una basta con
// que coincida un elemento (O). Un dispositivo que solo pone "fabricantes"
// atrapa a todos los de esa marca, que a veces es justo lo que se quiere y a
// veces es demasiado: por eso el peso de la coincidencia lo decide la
// especificidad, no el orden del archivo.
type Coincidencias struct {
	// Fabricantes se compara contra el fabricante deducido de la MAC.
	Fabricantes []string `toml:"fabricantes"`
	// PuertosTodos exige que TODOS esos puertos esten abiertos.
	PuertosTodos []int `toml:"puertos_todos"`
	// PuertosAlguno exige que al menos uno lo este.
	PuertosAlguno []int `toml:"puertos_alguno"`
	// BannerContiene busca texto en lo que los servicios contestaron.
	BannerContiene []string `toml:"banner_contiene"`
	// NombreContiene busca texto en el nombre descubierto del equipo.
	NombreContiene []string `toml:"nombre_contiene"`
	// SnmpContiene busca texto en la descripcion que da el equipo por SNMP.
	SnmpContiene []string `toml:"snmp_contiene"`
	// HuellaContiene busca en TODO lo que el aparato conto de si mismo: el
	// titulo de su pagina, su certificado, lo que anuncia por mDNS o UPnP y lo
	// que contesta al protocolo de su fabricante. Es la senal mas fuerte que hay
	// despues del prefijo de la MAC, porque la dijo el aparato.
	HuellaContiene []string `toml:"huella_contiene"`
	// ModeloContiene busca solo en el modelo, cuando dos aparatos de la misma
	// marca hay que separarlos.
	ModeloContiene []string `toml:"modelo_contiene"`
	// PrefijosMac son prefijos de MAC exactos (aa:bb:cc).
	PrefijosMac []string `toml:"prefijos_mac"`
}

// Equipo es lo que se sabe de un aparato al momento de reconocerlo.
type Equipo struct {
	IP         string
	MAC        string
	Fabricante string
	Nombre     string
	Puertos    []int
	Banners    []string
	SnmpDescr  string
	// Huella es todo lo que el aparato conto de si mismo, ya junto en una linea:
	// titulo de su pagina, certificado, mDNS, UPnP y protocolo del fabricante.
	Huella string
	// Modelo es lo que se dedujo como modelo, si algo lo dijo.
	Modelo string
}

// Catalogo es el juego de definiciones cargadas.
//
// Se puede recargar en caliente —al guardar una definicion propia o al traer las
// de la comunidad— sin reiniciar el servicio, y por eso lleva candado: mientras
// una peticion lo reemplaza, un escaneo puede estar reconociendo equipos.
type Catalogo struct {
	candado      sync.RWMutex
	definiciones []Definicion
	problemas    []string
}

// Reemplazar cambia el contenido por el de otro catalogo recien cargado.
//
// Se cambia el CONTENIDO y no el puntero a proposito: el puntero lo guardan el
// servidor y el programador por separado, y cambiarlo en uno solo dejaria a los
// escaneos reconociendo con el catalogo viejo para siempre.
func (c *Catalogo) Reemplazar(otro *Catalogo) {
	if otro == nil {
		return
	}
	otro.candado.RLock()
	definiciones, problemas := otro.definiciones, otro.problemas
	otro.candado.RUnlock()

	c.candado.Lock()
	defer c.candado.Unlock()
	c.definiciones, c.problemas = definiciones, problemas
}

// Cargar lee todas las definiciones de las carpetas dadas.
//
// Un archivo mal escrito NO tumba la carga: se anota como problema y se sigue.
// Un catalogo comunitario con un archivo roto no puede dejar el servicio sin
// reconocer nada.
func Cargar(carpetas []string) (*Catalogo, error) {
	if len(carpetas) == 0 {
		carpetas = CarpetasPorOmision
	}

	porArchivo := map[string]Definicion{}
	catalogo := &Catalogo{}

	for _, carpeta := range carpetas {
		entradas, err := os.ReadDir(carpeta)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			catalogo.problemas = append(catalogo.problemas,
				fmt.Sprintf("no se pudo leer %s: %v", carpeta, err))
			continue
		}

		for _, entrada := range entradas {
			if entrada.IsDir() || !strings.HasSuffix(entrada.Name(), ".toml") {
				continue
			}
			ruta := filepath.Join(carpeta, entrada.Name())
			contenido, err := os.ReadFile(ruta)
			if err != nil {
				catalogo.problemas = append(catalogo.problemas,
					fmt.Sprintf("no se pudo leer %s: %v", ruta, err))
				continue
			}

			var definicion Definicion
			if err := toml.Unmarshal(contenido, &definicion); err != nil {
				catalogo.problemas = append(catalogo.problemas,
					fmt.Sprintf("%s tiene un error de formato: %v", ruta, err))
				continue
			}
			if definicion.Nombre == "" {
				catalogo.problemas = append(catalogo.problemas,
					fmt.Sprintf("%s no tiene nombre y se omitio", ruta))
				continue
			}
			definicion.Archivo = ruta

			// Mismo nombre de archivo en una carpeta posterior = reemplazo.
			porArchivo[entrada.Name()] = definicion
		}
	}

	for _, definicion := range porArchivo {
		catalogo.definiciones = append(catalogo.definiciones, definicion)
	}
	sort.Slice(catalogo.definiciones, func(i, j int) bool {
		return catalogo.definiciones[i].Nombre < catalogo.definiciones[j].Nombre
	})
	return catalogo, nil
}

// Definiciones devuelve todo lo cargado.
func (c *Catalogo) Definiciones() []Definicion {
	c.candado.RLock()
	defer c.candado.RUnlock()
	return c.definiciones
}

// Problemas devuelve los archivos que no se pudieron usar, para poder decirlo en
// la interfaz en vez de que el usuario se pregunte por que su definicion no
// funciona.
func (c *Catalogo) Problemas() []string {
	c.candado.RLock()
	defer c.candado.RUnlock()
	return c.problemas
}

// Reconocer devuelve la definicion que mejor describe al equipo, o nil.
//
// Gana la MAS ESPECIFICA, no la primera: si una definicion dice solo "es HP" y
// otra dice "es HP y tiene el 9100 abierto", la segunda describe mejor al
// aparato y debe ganar sin importar como se llamen los archivos.
//
// Y antes que eso: **lo que identifica gana a lo que solo describe un sintoma**.
// Una definicion generica (ver Definicion.Generico) se guarda para el final y
// solo contesta si ninguna otra pudo.
func (c *Catalogo) Reconocer(equipo Equipo) *Definicion {
	c.candado.RLock()
	defer c.candado.RUnlock()

	mejor, mejorGenerica := -1, -1
	var elegida, generica *Definicion

	for i := range c.definiciones {
		puntos, coincide := evaluar(c.definiciones[i].Coincidencias, equipo)
		if !coincide {
			continue
		}
		if c.definiciones[i].Generico {
			if puntos > mejorGenerica {
				mejorGenerica = puntos
				generica = &c.definiciones[i]
			}
			continue
		}
		if puntos > mejor {
			mejor = puntos
			elegida = &c.definiciones[i]
		}
	}

	if elegida != nil {
		return elegida
	}
	return generica
}

// evaluar dice si el equipo cumple las condiciones y que tan especificas son.
func evaluar(condiciones Coincidencias, equipo Equipo) (int, bool) {
	puntos := 0
	algunaCondicion := false

	if len(condiciones.PrefijosMac) > 0 {
		algunaCondicion = true
		if !empiezaConAlguno(normalizarMac(equipo.MAC), normalizarLista(condiciones.PrefijosMac)) {
			return 0, false
		}
		// El prefijo de MAC es la senal mas fuerte: identifica al fabricante sin
		// depender de que el aparato quiera contestar nada.
		puntos += 5
	}

	if len(condiciones.Fabricantes) > 0 {
		algunaCondicion = true
		if !contieneAlguno(strings.ToLower(equipo.Fabricante), enMinusculas(condiciones.Fabricantes)) {
			return 0, false
		}
		puntos += 2
	}

	if len(condiciones.PuertosTodos) > 0 {
		algunaCondicion = true
		abiertos := comoConjunto(equipo.Puertos)
		for _, puerto := range condiciones.PuertosTodos {
			if !abiertos[puerto] {
				return 0, false
			}
		}
		puntos += 3 * len(condiciones.PuertosTodos)
	}

	if len(condiciones.PuertosAlguno) > 0 {
		algunaCondicion = true
		abiertos := comoConjunto(equipo.Puertos)
		encontrado := false
		for _, puerto := range condiciones.PuertosAlguno {
			if abiertos[puerto] {
				encontrado = true
				break
			}
		}
		if !encontrado {
			return 0, false
		}
		puntos += 2
	}

	if len(condiciones.BannerContiene) > 0 {
		algunaCondicion = true
		todo := strings.ToLower(strings.Join(equipo.Banners, " "))
		if !contieneAlguno(todo, enMinusculas(condiciones.BannerContiene)) {
			return 0, false
		}
		puntos += 4
	}

	if len(condiciones.NombreContiene) > 0 {
		algunaCondicion = true
		if !contieneAlguno(strings.ToLower(equipo.Nombre), enMinusculas(condiciones.NombreContiene)) {
			return 0, false
		}
		puntos += 3
	}

	if len(condiciones.SnmpContiene) > 0 {
		algunaCondicion = true
		if !contieneAlguno(strings.ToLower(equipo.SnmpDescr), enMinusculas(condiciones.SnmpContiene)) {
			return 0, false
		}
		puntos += 4
	}

	if len(condiciones.HuellaContiene) > 0 {
		algunaCondicion = true
		if !contieneAlguno(strings.ToLower(equipo.Huella), enMinusculas(condiciones.HuellaContiene)) {
			return 0, false
		}
		// Vale casi tanto como el prefijo de la MAC: lo dijo el propio aparato,
		// no se dedujo de que tenga un puerto abierto.
		puntos += 5
	}

	if len(condiciones.ModeloContiene) > 0 {
		algunaCondicion = true
		if !contieneAlguno(strings.ToLower(equipo.Modelo), enMinusculas(condiciones.ModeloContiene)) {
			return 0, false
		}
		puntos += 5
	}

	// Una definicion sin ninguna condicion coincidiria con todo. Eso no es un
	// dispositivo: es un error de quien lo escribio, y se descarta.
	if !algunaCondicion {
		return 0, false
	}
	return puntos, true
}

// Proponer arma el .toml de un equipo que nadie reconocio, ya relleno con lo que
// se vio. Es la puerta de entrada para que alguien aporte al catalogo sin saber
// programar: revisa, le pone nombre y lo manda.
func Proponer(equipo Equipo, nombre string) string {
	if nombre == "" {
		nombre = "Dispositivo sin identificar"
	}

	var salida strings.Builder
	salida.WriteString("# Definicion propuesta por MiRed a partir de un equipo real.\n")
	salida.WriteString("# Revise el nombre y la categoria, quite lo que sobre y mande el archivo\n")
	salida.WriteString("# a https://github.com/tuxormax/mired para que lo tenga todo el mundo.\n\n")
	salida.WriteString(fmt.Sprintf("nombre = %q\n", nombre))
	salida.WriteString("categoria = \"otro\"   # camara, impresora, switch, router, telefono, computadora...\n")
	salida.WriteString("icono = \"devices\"\n")
	salida.WriteString("descripcion = \"\"\n")
	salida.WriteString("aporta = \"\"   # su nombre, si quiere aparecer como autor\n\n")
	salida.WriteString("[coincidencias]\n")

	if prefijo := prefijoDeMac(equipo.MAC); prefijo != "" {
		salida.WriteString(fmt.Sprintf("prefijos_mac = [%q]\n", prefijo))
	}
	if equipo.Fabricante != "" {
		salida.WriteString(fmt.Sprintf("fabricantes = [%q]\n", equipo.Fabricante))
	}
	// Lo que el aparato dijo de si mismo es la mejor condicion que puede llevar
	// una definicion: no depende de que tenga un puerto abierto ni de en que
	// bloque de MAC le toco caer.
	if equipo.Modelo != "" {
		salida.WriteString(fmt.Sprintf("modelo_contiene = [%q]\n", equipo.Modelo))
	} else if equipo.Huella != "" {
		salida.WriteString(fmt.Sprintf("# huella_contiene = [%q]   # descomente y recorte a lo que identifique\n",
			recortar(equipo.Huella, 60)))
	}
	if len(equipo.Puertos) > 0 {
		puertos := make([]string, 0, len(equipo.Puertos))
		for _, puerto := range equipo.Puertos {
			puertos = append(puertos, fmt.Sprintf("%d", puerto))
		}
		salida.WriteString("puertos_todos = [" + strings.Join(puertos, ", ") + "]\n")
	}
	for _, banner := range equipo.Banners {
		if strings.TrimSpace(banner) != "" {
			salida.WriteString(fmt.Sprintf("# banner visto: %s\n", banner))
		}
	}
	if equipo.SnmpDescr != "" {
		salida.WriteString(fmt.Sprintf("# SNMP dijo: %s\n", equipo.SnmpDescr))
	}
	if equipo.Huella != "" {
		salida.WriteString(fmt.Sprintf("# el aparato dijo de si mismo: %s\n", recortar(equipo.Huella, 200)))
	}

	salida.WriteString("\n# Guarde este archivo en /etc/mired/dispositivos/ y reinicie mired-servidor\n")
	salida.WriteString("# para que empiece a reconocerlo.\n")
	return salida.String()
}

// ------------------------------------------------------------- auxiliares ---

func comoConjunto(numeros []int) map[int]bool {
	conjunto := make(map[int]bool, len(numeros))
	for _, numero := range numeros {
		conjunto[numero] = true
	}
	return conjunto
}

func enMinusculas(textos []string) []string {
	salida := make([]string, 0, len(textos))
	for _, texto := range textos {
		salida = append(salida, strings.ToLower(texto))
	}
	return salida
}

func contieneAlguno(donde string, que []string) bool {
	if donde == "" {
		return false
	}
	for _, texto := range que {
		if texto != "" && strings.Contains(donde, texto) {
			return true
		}
	}
	return false
}

func empiezaConAlguno(donde string, que []string) bool {
	if donde == "" {
		return false
	}
	for _, texto := range que {
		if texto != "" && strings.HasPrefix(donde, texto) {
			return true
		}
	}
	return false
}

// normalizarMac deja la MAC en minusculas y sin separadores, para poder comparar
// prefijos escritos de cualquier forma.
func normalizarMac(mac string) string {
	var limpio strings.Builder
	for _, letra := range strings.ToLower(mac) {
		if (letra >= '0' && letra <= '9') || (letra >= 'a' && letra <= 'f') {
			limpio.WriteRune(letra)
		}
	}
	return limpio.String()
}

func normalizarLista(macs []string) []string {
	salida := make([]string, 0, len(macs))
	for _, mac := range macs {
		salida = append(salida, normalizarMac(mac))
	}
	return salida
}

func prefijoDeMac(mac string) string {
	limpio := normalizarMac(mac)
	if len(limpio) < 6 {
		return ""
	}
	return limpio[0:2] + ":" + limpio[2:4] + ":" + limpio[4:6]
}

// recortar deja un texto en un largo presentable para un comentario.
func recortar(texto string, largo int) string {
	texto = strings.Join(strings.Fields(texto), " ")
	if len(texto) <= largo {
		return texto
	}
	return strings.TrimSpace(texto[:largo]) + "..."
}
