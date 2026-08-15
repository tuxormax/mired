// Paquete aire escucha que redes inalambricas se oyen desde aqui.
//
// Es la unica medicion de MiRed que no pasa por un cable. Contesta preguntas que
// ningun barrido de IP puede contestar:
//
//   - que SSID emite cada antena, y con que BSSID —que es **la MAC del puerto
//     inalambrico**, distinta de la del cable—;
//   - en que canal y en que banda, para ver dos antenas peleando por el mismo;
//   - con que fuerza llega cada una, que es lo que dice si una antena alcanza
//     donde tiene que alcanzar;
//   - y que redes hay alrededor que NO son de uno.
//
// **Hace falta una tarjeta WiFi en el equipo donde corre MiRed.** Una PC de
// escritorio conectada por cable no oye nada, y eso no es una falla: es que no
// tiene con que oir. Se dice con esas palabras en vez de devolver una lista
// vacia, que se leeria como "no hay ninguna red".
//
// El barrido se le pide al sistema y no se implementa aqui. Hablar nl80211 por
// netlink serian cientos de lineas para repetir lo que ya hace una herramienta
// que viene en todas las distribuciones, y que ademas resuelve sola el permiso:
// con NetworkManager delante, un usuario normal puede leer el ultimo barrido sin
// ser root.
package aire

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// RedInalambrica es una red que se oye desde aqui.
type RedInalambrica struct {
	// SSID es el nombre que emite. Puede venir vacio: una red oculta existe
	// igual, y esconderla no la hace desaparecer del aire.
	SSID string `json:"ssid"`
	// BSSID es la MAC del radio que la emite. Es el dato que permite decir "esta
	// red la emite ESTA antena", y el que se cruza con el inventario.
	BSSID string `json:"bssid"`
	// Canal y Banda: 2.4 GHz o 5 GHz.
	Canal int    `json:"canal"`
	Banda string `json:"banda"`
	// Senal en dBm. Mas cerca de cero es mejor: -45 es al lado, -85 es a duras
	// penas.
	Senal int `json:"senal"`
	// Seguridad es como esta protegida: WPA2, WPA3, abierta.
	Seguridad string `json:"seguridad"`
	// Tarjeta es con que interfaz de este equipo se oyo.
	Tarjeta string `json:"tarjeta"`
}

// Resultado es lo que dejo un barrido del aire.
type Resultado struct {
	Redes []RedInalambrica `json:"redes"`
	// Como se hizo: "nmcli", "iw" o vacio si no se pudo.
	Herramienta string `json:"herramienta"`
	// Explicacion dice por que no hay nada, cuando no hay nada. Una lista vacia
	// sin explicacion se lee como "aqui no hay redes WiFi", que casi nunca es
	// verdad.
	Explicacion string `json:"explicacion"`
	DuracionMs  int64  `json:"duracionMs"`
}

// Barrer escucha el aire y devuelve lo que se oye.
//
// Prueba las herramientas en orden de menos a mas privilegio: primero la del
// escritorio, que ya tiene un barrido reciente y no necesita permisos; despues
// la de bajo nivel, que si los necesita pero funciona en un equipo sin
// escritorio, como una Raspberry en una sucursal.
func Barrer(ctx context.Context) Resultado {
	inicio := time.Now()

	if _, err := exec.LookPath("nmcli"); err == nil {
		if redes, err := conNmcli(ctx); err == nil && len(redes) > 0 {
			return terminar(inicio, Resultado{Redes: redes, Herramienta: "nmcli"})
		}
	}

	tarjetas := TarjetasInalambricas(ctx)
	if len(tarjetas) == 0 {
		return terminar(inicio, Resultado{
			Explicacion: "Este equipo no tiene tarjeta WiFi, asi que no puede oir ninguna " +
				"red inalambrica. Todo lo demas de MiRed funciona igual: esto solo afecta a " +
				"saber que SSID emite cada antena.",
		})
	}

	if _, err := exec.LookPath("iw"); err != nil {
		return terminar(inicio, Resultado{
			Explicacion: "Hay tarjeta WiFi pero falta la herramienta del sistema para barrer " +
				"el aire. Instale el paquete `iw` (o NetworkManager) y vuelva a intentarlo.",
		})
	}

	var todas []RedInalambrica
	var fallos []string
	for _, tarjeta := range tarjetas {
		redes, err := conIw(ctx, tarjeta)
		if err != nil {
			fallos = append(fallos, fmt.Sprintf("%s: %v", tarjeta, err))
			continue
		}
		todas = append(todas, redes...)
	}

	resultado := Resultado{Redes: todas, Herramienta: "iw"}
	if len(todas) == 0 {
		resultado.Herramienta = ""
		resultado.Explicacion = "Hay tarjeta WiFi pero el barrido no devolvio nada. " +
			"Suele ser falta de permisos: la sonda necesita CAP_NET_ADMIN para pedir un " +
			"barrido del aire."
		if len(fallos) > 0 {
			resultado.Explicacion += " Detalle: " + strings.Join(fallos, "; ")
		}
	}
	return terminar(inicio, resultado)
}

func terminar(inicio time.Time, resultado Resultado) Resultado {
	if resultado.Redes == nil {
		resultado.Redes = []RedInalambrica{}
	}
	// Se ordena por senal: lo que mejor se oye primero, que es lo que alguien
	// mira cuando se pregunta "que antena esta cubriendo aqui".
	sort.SliceStable(resultado.Redes, func(i, j int) bool {
		return resultado.Redes[i].Senal > resultado.Redes[j].Senal
	})
	resultado.DuracionMs = time.Since(inicio).Milliseconds()
	return resultado
}

// TarjetasInalambricas dice que interfaces de este equipo pueden oir el aire.
func TarjetasInalambricas(ctx context.Context) []string {
	salida, err := ejecutar(ctx, 5*time.Second, "iw", "dev")
	if err != nil {
		return nil
	}
	var tarjetas []string
	lector := bufio.NewScanner(strings.NewReader(salida))
	for lector.Scan() {
		campos := strings.Fields(lector.Text())
		if len(campos) == 2 && campos[0] == "Interface" {
			tarjetas = append(tarjetas, campos[1])
		}
	}
	return tarjetas
}

// conNmcli lee el barrido de NetworkManager, que ya lo tiene hecho.
//
// El formato terse (`-t`) separa con dos puntos y escapa los que aparezcan
// dentro de un valor con barra invertida: por eso la MAC, que va llena de dos
// puntos, llega como `AA\:BB\:CC\:...`.
func conNmcli(ctx context.Context) ([]RedInalambrica, error) {
	salida, err := ejecutar(ctx, 20*time.Second,
		"nmcli", "-t", "-f", "SSID,BSSID,CHAN,FREQ,SIGNAL,SECURITY,DEVICE", "device", "wifi", "list")
	if err != nil {
		return nil, err
	}

	var redes []RedInalambrica
	lector := bufio.NewScanner(strings.NewReader(salida))
	for lector.Scan() {
		campos := partirTerse(lector.Text())
		if len(campos) < 6 {
			continue
		}
		red := RedInalambrica{
			SSID:      campos[0],
			BSSID:     strings.ToLower(campos[1]),
			Canal:     numero(campos[2]),
			Senal:     senalDesdePorcentaje(numero(campos[4])),
			Seguridad: seguridadLegible(campos[5]),
		}
		if len(campos) > 6 {
			red.Tarjeta = campos[6]
		}
		red.Banda = bandaDeFrecuencia(numero(strings.Fields(campos[3])[0]))
		if red.BSSID == "" {
			continue
		}
		redes = append(redes, red)
	}
	return redes, nil
}

// conIw pide un barrido a la tarjeta y lee la respuesta.
func conIw(ctx context.Context, tarjeta string) ([]RedInalambrica, error) {
	salida, err := ejecutar(ctx, 30*time.Second, "iw", "dev", tarjeta, "scan")
	if err != nil {
		// Sin permiso para barrer, se pide lo ultimo que la tarjeta oyo. Es peor
		// dato —puede ser viejo— pero es mucho mejor que nada.
		salida, err = ejecutar(ctx, 10*time.Second, "iw", "dev", tarjeta, "scan", "dump")
		if err != nil {
			return nil, err
		}
	}

	return leerSalidaDeIw(salida, tarjeta), nil
}

// leerSalidaDeIw desarma lo que imprime `iw scan`: un bloque por antena.
func leerSalidaDeIw(salida, tarjeta string) []RedInalambrica {
	var redes []RedInalambrica
	var actual *RedInalambrica

	guardar := func() {
		if actual != nil && actual.BSSID != "" {
			redes = append(redes, *actual)
		}
	}

	lector := bufio.NewScanner(strings.NewReader(salida))
	for lector.Scan() {
		linea := strings.TrimSpace(lector.Text())
		switch {
		case strings.HasPrefix(linea, "BSS "):
			guardar()
			campos := strings.Fields(linea)
			mac := ""
			if len(campos) > 1 {
				mac = strings.ToLower(strings.SplitN(campos[1], "(", 2)[0])
			}
			actual = &RedInalambrica{BSSID: mac, Tarjeta: tarjeta, Seguridad: "abierta"}
		case actual == nil:
			continue
		case strings.HasPrefix(linea, "SSID:"):
			actual.SSID = strings.TrimSpace(strings.TrimPrefix(linea, "SSID:"))
		case strings.HasPrefix(linea, "signal:"):
			actual.Senal = int(decimal(strings.TrimSpace(strings.TrimPrefix(linea, "signal:"))))
		case strings.HasPrefix(linea, "freq:"):
			frecuencia := numero(strings.TrimSpace(strings.TrimPrefix(linea, "freq:")))
			actual.Banda = bandaDeFrecuencia(frecuencia)
			actual.Canal = canalDeFrecuencia(frecuencia)
		case strings.HasPrefix(linea, "DS Parameter set: channel"):
			actual.Canal = numero(strings.TrimPrefix(linea, "DS Parameter set: channel"))
		case strings.Contains(linea, "RSN:"):
			actual.Seguridad = "WPA2"
		case strings.Contains(linea, "WPA:"):
			if actual.Seguridad == "abierta" {
				actual.Seguridad = "WPA"
			}
		}
	}
	guardar()
	return redes
}

func ejecutar(ctx context.Context, plazo time.Duration, programa string, argumentos ...string) (string, error) {
	conPlazo, cancelar := context.WithTimeout(ctx, plazo)
	defer cancelar()

	salida, err := exec.CommandContext(conPlazo, programa, argumentos...).Output()
	if err != nil {
		return "", err
	}
	return string(salida), nil
}

// partirTerse separa la salida de nmcli respetando los dos puntos escapados.
func partirTerse(linea string) []string {
	var campos []string
	var actual strings.Builder
	escapado := false

	for _, letra := range linea {
		switch {
		case escapado:
			actual.WriteRune(letra)
			escapado = false
		case letra == '\\':
			escapado = true
		case letra == ':':
			campos = append(campos, actual.String())
			actual.Reset()
		default:
			actual.WriteRune(letra)
		}
	}
	return append(campos, actual.String())
}

func numero(texto string) int {
	campos := strings.Fields(texto)
	if len(campos) == 0 {
		return 0
	}
	valor, _ := strconv.Atoi(strings.TrimSpace(campos[0]))
	return valor
}

func decimal(texto string) float64 {
	campos := strings.Fields(texto)
	if len(campos) == 0 {
		return 0
	}
	valor, _ := strconv.ParseFloat(campos[0], 64)
	return valor
}

// senalDesdePorcentaje convierte el 0-100 de nmcli a dBm aproximados.
//
// Es una conversion APROXIMADA y se dice: nmcli no expone el dBm crudo, y
// presentar el porcentaje como si fuera una medicion en dBm seria inventar
// precision que no hay.
func senalDesdePorcentaje(porcentaje int) int {
	if porcentaje <= 0 {
		return 0
	}
	if porcentaje > 100 {
		porcentaje = 100
	}
	return porcentaje/2 - 100
}

// seguridadLegible traduce lo que contesta nmcli a algo que se entienda.
func seguridadLegible(crudo string) string {
	crudo = strings.TrimSpace(crudo)
	if crudo == "" || strings.EqualFold(crudo, "--") {
		return "abierta"
	}
	return crudo
}

func bandaDeFrecuencia(mhz int) string {
	switch {
	case mhz >= 2400 && mhz <= 2500:
		return "2.4 GHz"
	case mhz >= 4900 && mhz <= 5900:
		return "5 GHz"
	case mhz >= 5925:
		return "6 GHz"
	default:
		return ""
	}
}

func canalDeFrecuencia(mhz int) int {
	switch {
	case mhz == 2484:
		return 14
	case mhz >= 2412 && mhz <= 2472:
		return (mhz - 2407) / 5
	case mhz >= 5000 && mhz <= 5900:
		return (mhz - 5000) / 5
	default:
		return 0
	}
}
