// Paquete lectores entra al panel de un aparato con su credencial guardada y
// lee su tabla de conectados.
//
// Es el tercer camino para saber quien cuelga de quien, y el ultimo:
//
//  1. La controladora WiFi, si la hay, lo sabe todo y no necesita nada de aqui.
//  2. La tabla de asociados por SNMP no pide credenciales de administrador.
//  3. Esto: entrar al panel con el usuario y la clave que el dueno ya guardo en
//     la ficha del aparato.
//
// **Solo se entra donde hay credencial guardada.** Sin ella no se intenta nada:
// probar claves de fabrica contra el equipo de alguien es exactamente lo que
// este proyecto no hace, aunque "casi siempre funcione".
//
// # Por que un marco y no un lector suelto
//
// Cada marca publica su tabla a su manera, y ninguna se parece a la otra: AirOS
// contesta JSON, RouterOS tiene su API REST, y los modems del proveedor —Huawei,
// ZTE, Askey, Technicolor— hablan TR-064, que es un estandar viejo que casi
// nadie recuerda pero que ellos si implementan. Cada uno vive en su archivo y
// aqui solo esta lo que comparten.
//
// El ultimo de la lista es el lector GENERICO, que entra al panel y saca las MAC
// que encuentre en la pagina. Es poco, pero es cierto: "estas direcciones estan
// conectadas a este aparato" ya ubica en el mapa lo que antes flotaba.
package lectores

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

// ErrSinCredencial la devuelve Leer cuando nadie guardo como entrar.
var ErrSinCredencial = errors.New("ese aparato no tiene credencial guardada")

// ErrNoSeEntendio la devuelve un lector que hablo con el aparato pero no
// reconocio lo que contesto. Es distinto de un fallo de red, y por eso se
// distingue: uno se arregla revisando la credencial y el otro esperando.
var ErrNoSeEntendio = errors.New("el aparato contesto, pero no se entendio su tabla de conectados")

// Conectado es un equipo que el aparato dice tener colgando.
type Conectado struct {
	// MAC es lo unico que casi siempre se sabe, y lo que permite cruzarlo con el
	// inventario.
	MAC string `json:"mac"`
	// Nombre es como lo llama el aparato: casi siempre el nombre que el propio
	// equipo pidio al conectarse.
	Nombre string `json:"nombre,omitempty"`
	IP     string `json:"ip,omitempty"`
	// Red es el SSID, cuando el aparato lo dice.
	Red string `json:"red,omitempty"`
	// SenalDbm solo tiene sentido en los inalambricos.
	SenalDbm int `json:"senalDbm,omitempty"`
	// PorCable distingue lo que entra por un puerto de lo que entra por el aire.
	//
	// **Importa mucho**: el modem del proveedor lista las dos cosas juntas, y
	// colgar del WiFi a un equipo que esta enchufado por cable seria dibujar una
	// red que no existe.
	PorCable bool `json:"porCable"`
}

// Destino es a quien se le pregunta y con que.
type Destino struct {
	// Direccion es donde vive el panel: puede ser una IP o una URL completa.
	Direccion string
	Usuario   string
	Clave     string
	// Pistas es lo que ya se sabe del aparato —fabricante, modelo, huella— y
	// sirve para probar primero el lector de su marca en vez de todos.
	Pistas string
	Espera time.Duration
}

// Lector sabe leer la tabla de conectados de una familia de aparatos.
type Lector interface {
	// Nombre es como se llama el lector, para poder decir cual contesto.
	Nombre() string
	// Reconoce dice si las pistas apuntan a esta marca. Un lector que no
	// reconoce nada se prueba igual, pero al final.
	Reconoce(pistas string) bool
	// Leer entra y devuelve lo que el aparato diga tener conectado.
	Leer(ctx context.Context, destino Destino) ([]Conectado, error)
}

// Registrados son todos los lectores que MiRed sabe usar, en el orden en que
// conviene probarlos. El generico va al final a proposito: solo se usa cuando
// ninguno de los que entienden el aparato pudo.
func Registrados() []Lector {
	return []Lector{
		AirOS{},
		MikroTik{},
		TR064{},
		Generico{},
	}
}

// Resultado dice que se leyo y quien lo leyo.
type Resultado struct {
	Lector     string      `json:"lector"`
	Conectados []Conectado `json:"conectados"`
}

// Leer prueba los lectores contra un aparato y devuelve el primero que entienda.
//
// Se prueban PRIMERO los que reconocen la marca por las pistas. Un modem Huawei
// no tiene por que aguantar que le hablen en el idioma de MikroTik antes de que
// alguien pruebe el suyo.
func Leer(ctx context.Context, destino Destino) (Resultado, error) {
	if destino.Usuario == "" || destino.Clave == "" {
		return Resultado{}, ErrSinCredencial
	}
	if destino.Espera <= 0 {
		destino.Espera = 10 * time.Second
	}

	pistas := strings.ToLower(destino.Pistas)
	lectores := Registrados()
	sort.SliceStable(lectores, func(i, j int) bool {
		return lectores[i].Reconoce(pistas) && !lectores[j].Reconoce(pistas)
	})

	ultimo := ErrNoSeEntendio
	for _, lector := range lectores {
		conectados, err := lector.Leer(ctx, destino)
		if err != nil {
			ultimo = err
			continue
		}
		if len(conectados) == 0 {
			continue
		}
		return Resultado{Lector: lector.Nombre(), Conectados: limpiar(conectados)}, nil
	}
	return Resultado{}, ultimo
}

// limpiar deja la lista sin repetidos y con las MAC en una sola forma.
func limpiar(conectados []Conectado) []Conectado {
	porMAC := map[string]Conectado{}
	for _, conectado := range conectados {
		mac := NormalizarMAC(conectado.MAC)
		if mac == "" {
			continue
		}
		conectado.MAC = mac

		// Si el mismo aparato aparece dos veces, se conserva lo mas completo.
		if anterior, hay := porMAC[mac]; hay {
			if conectado.Nombre == "" {
				conectado.Nombre = anterior.Nombre
			}
			if conectado.Red == "" {
				conectado.Red = anterior.Red
			}
			if conectado.IP == "" {
				conectado.IP = anterior.IP
			}
			if conectado.SenalDbm == 0 {
				conectado.SenalDbm = anterior.SenalDbm
			}
		}
		porMAC[mac] = conectado
	}

	lista := make([]Conectado, 0, len(porMAC))
	for _, conectado := range porMAC {
		lista = append(lista, conectado)
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].MAC < lista[j].MAC })
	return lista
}

// NormalizarMAC deja una direccion fisica como se guarda en MiRed: minusculas y
// separada por dos puntos. Los paneles la escriben de todas las formas posibles.
func NormalizarMAC(mac string) string {
	limpia := make([]rune, 0, 12)
	for _, letra := range strings.ToLower(mac) {
		if (letra >= '0' && letra <= '9') || (letra >= 'a' && letra <= 'f') {
			limpia = append(limpia, letra)
		}
	}
	if len(limpia) != 12 {
		return ""
	}
	texto := string(limpia)
	partes := make([]string, 6)
	for i := 0; i < 6; i++ {
		partes[i] = texto[i*2 : i*2+2]
	}
	return strings.Join(partes, ":")
}

// conPistas dice si alguna de las palabras aparece en las pistas.
func conPistas(pistas string, palabras ...string) bool {
	for _, palabra := range palabras {
		if strings.Contains(pistas, palabra) {
			return true
		}
	}
	return false
}
