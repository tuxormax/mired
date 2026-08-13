// Paquete controladora le pregunta a las controladoras WiFi que aparato esta
// colgado de que antena.
//
// **Por que hace falta:** un punto de acceso no tiene puertos, tiene antenas, y
// quien sabe quien esta conectado a cual es la controladora, no el aparato. Sin
// esto, en una oficina moderna la mitad de los equipos —telefonos, portatiles,
// camaras— aparecen como "sin ubicar" en el mapa, y el plano queda a medias sin
// que nadie sepa por que.
//
// **Donde corre:** en el servidor, no en la sonda. Hablar con la controladora es
// HTTPS contra una direccion de la red, y eso no necesita ningun privilegio
// especial. La sonda existe para lo que si lo necesita.
//
// De momento se entiende UniFi, que es la controladora que hay instalada en
// nueve de cada diez sitios chicos. Lo que devuelve este paquete es
// deliberadamente independiente de la marca, para que agregar otra sea escribir
// otro archivo y no tocar nada mas.
package controladora

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"time"
)

// Acceso es lo que hace falta para hablar con una controladora.
type Acceso struct {
	Nombre       string
	URL          string
	Usuario      string
	Clave        string
	Sitio        string
	VerificarTLS bool
}

// Aparato es un switch o un punto de acceso que la controladora administra.
type Aparato struct {
	MAC    string
	IP     string
	Nombre string
	Modelo string
	// EsPuntoDeAcceso distingue una antena de un switch. En el mapa no se dibujan
	// igual: un switch tiene bocas numeradas y un punto de acceso tiene redes.
	EsPuntoDeAcceso bool
	Bocas           []Boca
}

// Boca es un puerto de un switch o una radio de un punto de acceso.
type Boca struct {
	Indice        int
	Nombre        string
	Activa        bool
	VelocidadMbps int
}

// Conectado es un equipo que la controladora ve colgado de una boca.
type Conectado struct {
	MAC string
	IP  string
	// AparatoMAC es de que switch o punto de acceso cuelga.
	AparatoMAC string
	// Boca es el numero de puerto del switch, o el indice sintetico de la red
	// inalambrica cuando cuelga de una antena.
	Boca int
	// Red es el nombre de la red WiFi. Vacio cuando el equipo esta por cable.
	Red      string
	PorCable bool
}

// Lectura es todo lo que la controladora contesto de una vez.
type Lectura struct {
	Aparatos   []Aparato
	Conectados []Conectado
}

// Consultar entra a la controladora y devuelve que hay colgado de que.
func Consultar(ctx context.Context, acceso Acceso, espera time.Duration) (Lectura, error) {
	cliente, err := nuevoCliente(acceso, espera)
	if err != nil {
		return Lectura{}, err
	}
	if err := cliente.entrar(ctx); err != nil {
		return Lectura{}, err
	}

	aparatos, err := cliente.aparatos(ctx)
	if err != nil {
		return Lectura{}, err
	}
	conectados, err := cliente.conectados(ctx)
	if err != nil {
		return Lectura{}, err
	}
	return Lectura{Aparatos: aparatos, Conectados: conectados}, nil
}

type cliente struct {
	acceso Acceso
	http   *http.Client
	base   string
	// unifiOS distingue las controladoras nuevas (Dream Machine, Cloud Key con
	// UniFi OS) de las clasicas. Cambian la ruta para entrar y **todas** las
	// rutas de datos, asi que hay que saberlo antes de pedir nada.
	unifiOS bool
}

func nuevoCliente(acceso Acceso, espera time.Duration) (*cliente, error) {
	if strings.TrimSpace(acceso.URL) == "" {
		return nil, fmt.Errorf("la controladora %s no tiene direccion", acceso.Nombre)
	}
	if espera <= 0 {
		espera = 15 * time.Second
	}
	if acceso.Sitio == "" {
		acceso.Sitio = "default"
	}

	// La sesion de UniFi es una galleta: sin tarro, cada peticion volveria a
	// llegar sin autenticar y contestaria 401.
	tarro, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	transporte := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !acceso.VerificarTLS},
		DialContext:     (&net.Dialer{Timeout: espera}).DialContext,
	}

	return &cliente{
		acceso: acceso,
		base:   strings.TrimRight(acceso.URL, "/"),
		http:   &http.Client{Timeout: espera, Jar: tarro, Transport: transporte},
	}, nil
}

// entrar prueba primero la controladora nueva y se cae a la clasica.
//
// No hay forma de preguntarle a una controladora cual es sin intentarlo: las dos
// contestan a la misma direccion y solo se distinguen por que ruta de entrada
// acepta. Se prueba la nueva primero porque es la que se vende hoy.
func (c *cliente) entrar(ctx context.Context) error {
	credenciales := map[string]string{
		"username": c.acceso.Usuario,
		"password": c.acceso.Clave,
	}

	if err := c.pedir(ctx, http.MethodPost, "/api/auth/login", credenciales, nil); err == nil {
		c.unifiOS = true
		return nil
	}

	if err := c.pedir(ctx, http.MethodPost, "/api/login", credenciales, nil); err != nil {
		return fmt.Errorf("la controladora %s no acepto el usuario o la clave: %w",
			c.acceso.Nombre, err)
	}
	c.unifiOS = false
	return nil
}

// ruta arma la direccion de un recurso segun el tipo de controladora.
func (c *cliente) ruta(recurso string) string {
	if c.unifiOS {
		return "/proxy/network/api/s/" + c.acceso.Sitio + "/" + recurso
	}
	return "/api/s/" + c.acceso.Sitio + "/" + recurso
}

// respuestaUnifi es el sobre que UniFi pone alrededor de todo.
type respuestaUnifi struct {
	Meta struct {
		Rc      string `json:"rc"`
		Mensaje string `json:"msg"`
	} `json:"meta"`
	Datos json.RawMessage `json:"data"`
}

func (c *cliente) pedir(ctx context.Context, metodo, recurso string, cuerpo any, destino any) error {
	var lector *strings.Reader
	if cuerpo != nil {
		crudo, err := json.Marshal(cuerpo)
		if err != nil {
			return err
		}
		lector = strings.NewReader(string(crudo))
	} else {
		lector = strings.NewReader("")
	}

	peticion, err := http.NewRequestWithContext(ctx, metodo, c.base+recurso, lector)
	if err != nil {
		return err
	}
	peticion.Header.Set("Content-Type", "application/json")
	peticion.Header.Set("Accept", "application/json")

	respuesta, err := c.http.Do(peticion)
	if err != nil {
		return fmt.Errorf("no se pudo hablar con %s: %w", c.base, err)
	}
	defer respuesta.Body.Close()

	if respuesta.StatusCode >= 400 {
		return fmt.Errorf("%s contesto %d en %s", c.base, respuesta.StatusCode, recurso)
	}

	var sobre respuestaUnifi
	if err := json.NewDecoder(respuesta.Body).Decode(&sobre); err != nil {
		// Al entrar, la controladora nueva contesta un cuerpo que no lleva este
		// sobre. Que no se pueda leer no significa que haya fallado.
		if destino == nil {
			return nil
		}
		return fmt.Errorf("no se entendio lo que contesto %s: %w", c.base, err)
	}
	if sobre.Meta.Rc != "" && sobre.Meta.Rc != "ok" {
		return fmt.Errorf("%s contesto %q: %s", c.base, sobre.Meta.Rc, sobre.Meta.Mensaje)
	}
	if destino == nil || len(sobre.Datos) == 0 {
		return nil
	}
	return json.Unmarshal(sobre.Datos, destino)
}

// aparatoUnifi es lo que interesa de cada equipo administrado.
type aparatoUnifi struct {
	MAC       string `json:"mac"`
	IP        string `json:"ip"`
	Nombre    string `json:"name"`
	Modelo    string `json:"model"`
	Tipo      string `json:"type"` // usw = switch, uap = antena, ugw/udm = router
	PortTable []struct {
		Indice    int    `json:"port_idx"`
		Nombre    string `json:"name"`
		Arriba    bool   `json:"up"`
		Velocidad int    `json:"speed"`
	} `json:"port_table"`
	RadioTable []struct {
		Nombre string `json:"name"`
		Radio  string `json:"radio"`
	} `json:"radio_table"`
}

func (c *cliente) aparatos(ctx context.Context) ([]Aparato, error) {
	var crudos []aparatoUnifi
	if err := c.pedir(ctx, http.MethodGet, c.ruta("stat/device"), nil, &crudos); err != nil {
		return nil, err
	}

	aparatos := make([]Aparato, 0, len(crudos))
	for _, crudo := range crudos {
		aparato := Aparato{
			MAC:             normalizarMAC(crudo.MAC),
			IP:              crudo.IP,
			Nombre:          crudo.Nombre,
			Modelo:          crudo.Modelo,
			EsPuntoDeAcceso: crudo.Tipo == "uap",
		}
		for _, puerto := range crudo.PortTable {
			nombre := puerto.Nombre
			if nombre == "" {
				nombre = "Puerto " + strconv.Itoa(puerto.Indice)
			}
			aparato.Bocas = append(aparato.Bocas, Boca{
				Indice:        puerto.Indice,
				Nombre:        nombre,
				Activa:        puerto.Arriba,
				VelocidadMbps: puerto.Velocidad,
			})
		}
		aparatos = append(aparatos, aparato)
	}
	return aparatos, nil
}

// conectadoUnifi es un equipo que la controladora ve conectado ahora mismo.
type conectadoUnifi struct {
	MAC      string `json:"mac"`
	IP       string `json:"ip"`
	PorCable bool   `json:"is_wired"`
	// Por cable: de que switch y en que boca.
	SwitchMAC   string `json:"sw_mac"`
	SwitchPorto int    `json:"sw_port"`
	// Por WiFi: de que antena y en que red.
	AntenaMAC string `json:"ap_mac"`
	Red       string `json:"essid"`
}

func (c *cliente) conectados(ctx context.Context) ([]Conectado, error) {
	var crudos []conectadoUnifi
	if err := c.pedir(ctx, http.MethodGet, c.ruta("stat/sta"), nil, &crudos); err != nil {
		return nil, err
	}

	// Cada red WiFi necesita un numero de boca para poder colgar de ella en el
	// mapa, igual que un puerto de switch. Se reparten desde un numero alto para
	// que NUNCA choquen con un puerto de verdad: un switch de 48 bocas es grande,
	// y uno de mil no existe.
	const primeraBocaInalambrica = 1000
	bocaDeRed := map[string]int{}

	conectados := make([]Conectado, 0, len(crudos))
	for _, crudo := range crudos {
		conectado := Conectado{
			MAC:      normalizarMAC(crudo.MAC),
			IP:       crudo.IP,
			PorCable: crudo.PorCable,
		}
		if conectado.MAC == "" {
			continue
		}

		if crudo.PorCable {
			conectado.AparatoMAC = normalizarMAC(crudo.SwitchMAC)
			conectado.Boca = crudo.SwitchPorto
		} else {
			conectado.AparatoMAC = normalizarMAC(crudo.AntenaMAC)
			conectado.Red = crudo.Red
			clave := conectado.AparatoMAC + "|" + crudo.Red
			numero, hay := bocaDeRed[clave]
			if !hay {
				numero = primeraBocaInalambrica + len(bocaDeRed)
				bocaDeRed[clave] = numero
			}
			conectado.Boca = numero
		}

		if conectado.AparatoMAC == "" {
			// La controladora lo ve pero no dice de donde cuelga. Se descarta:
			// inventarle un sitio en el mapa seria peor que dejarlo sin ubicar,
			// que es exactamente lo que hara el resto del sistema con el.
			continue
		}
		conectados = append(conectados, conectado)
	}
	return conectados, nil
}

// BocasDeRedes arma las bocas sinteticas de un punto de acceso a partir de los
// equipos que cuelgan de el.
//
// Un punto de acceso no tiene una lista de "puertos" que consultar: sus bocas
// son las redes WiFi, y solo se sabe cuales estan en uso mirando quien esta
// conectado. Por eso se derivan aqui y no vienen de la controladora.
func BocasDeRedes(conectados []Conectado, aparatoMAC string) []Boca {
	vistas := map[int]string{}
	for _, conectado := range conectados {
		if conectado.AparatoMAC != aparatoMAC || conectado.PorCable {
			continue
		}
		nombre := conectado.Red
		if nombre == "" {
			nombre = "WiFi"
		}
		vistas[conectado.Boca] = nombre
	}

	bocas := make([]Boca, 0, len(vistas))
	for indice, nombre := range vistas {
		bocas = append(bocas, Boca{Indice: indice, Nombre: nombre, Activa: true})
	}
	return bocas
}

func normalizarMAC(mac string) string {
	return strings.ToLower(strings.TrimSpace(mac))
}
