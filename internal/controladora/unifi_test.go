package controladora

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Estas pruebas levantan una controladora de mentira que contesta lo mismo que
// una UniFi de verdad.
//
// Lo que se comprueba es lo que se descubre tarde y duele: que se reconozca sola
// la controladora clasica y la nueva —cambian TODAS las rutas—, que la sesion
// viaje en la galleta, y que un equipo por WiFi acabe colgado de su antena y no
// de cualquier otra cosa.

// unifiDeMentira arma un servidor que se comporta como una controladora.
//
// El parametro unifiOS decide cual de las dos generaciones imita: la nueva
// acepta /api/auth/login y sirve los datos bajo /proxy/network; la clasica
// acepta /api/login y los sirve directo.
func unifiDeMentira(t *testing.T, unifiOS bool, aparatos, conectados any) *httptest.Server {
	t.Helper()

	responder := func(escritor http.ResponseWriter, datos any) {
		escritor.Header().Set("Content-Type", "application/json")
		json.NewEncoder(escritor).Encode(map[string]any{
			"meta": map[string]string{"rc": "ok"},
			"data": datos,
		})
	}

	prefijo := "/api/s/default/"
	rutaEntrar := "/api/login"
	if unifiOS {
		prefijo = "/proxy/network/api/s/default/"
		rutaEntrar = "/api/auth/login"
	}

	manejador := http.NewServeMux()
	manejador.HandleFunc("/", func(escritor http.ResponseWriter, peticion *http.Request) {
		switch peticion.URL.Path {
		case rutaEntrar:
			// La sesion viaja en una galleta: sin ella, todo lo demas es 401.
			// El Path es "/" a proposito, como la manda una controladora de
			// verdad: sin el, la galleta solo valdria para /api/auth y las
			// peticiones de datos irian sin sesion.
			http.SetCookie(escritor, &http.Cookie{
				Name: "unifises", Value: "abc123", Path: "/"})
			responder(escritor, nil)
		case prefijo + "stat/device":
			if _, err := peticion.Cookie("unifises"); err != nil {
				http.Error(escritor, "sin sesion", http.StatusUnauthorized)
				return
			}
			responder(escritor, aparatos)
		case prefijo + "stat/sta":
			if _, err := peticion.Cookie("unifises"); err != nil {
				http.Error(escritor, "sin sesion", http.StatusUnauthorized)
				return
			}
			responder(escritor, conectados)
		default:
			http.Error(escritor, "no existe", http.StatusNotFound)
		}
	})

	servidor := httptest.NewServer(manejador)
	t.Cleanup(servidor.Close)
	return servidor
}

var aparatosDePrueba = []map[string]any{
	{
		"mac": "AA:BB:CC:00:00:01", "ip": "192.168.1.2", "name": "sw-oficina",
		"model": "US8P150", "type": "usw",
		"port_table": []map[string]any{
			{"port_idx": 1, "name": "Puerto 1", "up": true, "speed": 1000},
			{"port_idx": 5, "name": "Impresora", "up": true, "speed": 100},
		},
	},
	{
		"mac": "AA:BB:CC:00:00:02", "ip": "192.168.1.3", "name": "antena-recepcion",
		"model": "U6LR", "type": "uap",
	},
}

var conectadosDePrueba = []map[string]any{
	{"mac": "11:22:33:44:55:66", "ip": "192.168.1.50", "is_wired": true,
		"sw_mac": "AA:BB:CC:00:00:01", "sw_port": 5},
	{"mac": "77:88:99:AA:BB:CC", "ip": "192.168.1.51", "is_wired": false,
		"ap_mac": "AA:BB:CC:00:00:02", "essid": "Oficina"},
	{"mac": "DD:EE:FF:00:11:22", "ip": "192.168.1.52", "is_wired": false,
		"ap_mac": "AA:BB:CC:00:00:02", "essid": "Invitados"},
	{"mac": "99:99:99:99:99:99", "ip": "192.168.1.53", "is_wired": false,
		"essid": "Oficina"}, // la controladora lo ve pero no dice de donde cuelga
}

func TestSeReconocenLasDosGeneracionesDeControladora(t *testing.T) {
	// Una controladora nueva y una clasica contestan a la misma direccion y solo
	// se distinguen por que ruta de entrada aceptan. Si se elige mal, TODAS las
	// rutas de datos quedan equivocadas y no se lee nada.
	for _, unifiOS := range []bool{false, true} {
		servidor := unifiDeMentira(t, unifiOS, aparatosDePrueba, conectadosDePrueba)

		lectura, err := Consultar(context.Background(), Acceso{
			Nombre: "prueba", URL: servidor.URL, Usuario: "monitor", Clave: "x",
		}, 5*time.Second)
		if err != nil {
			t.Fatalf("unifiOS=%v: %v", unifiOS, err)
		}
		if len(lectura.Aparatos) != 2 {
			t.Fatalf("unifiOS=%v: se esperaban dos aparatos: %+v", unifiOS, lectura.Aparatos)
		}
	}
}

func TestUnEquipoPorWifiQuedaColgadoDeSuAntena(t *testing.T) {
	servidor := unifiDeMentira(t, false, aparatosDePrueba, conectadosDePrueba)

	lectura, err := Consultar(context.Background(), Acceso{
		Nombre: "prueba", URL: servidor.URL, Usuario: "monitor", Clave: "x",
	}, 5*time.Second)
	if err != nil {
		t.Fatalf("no se pudo consultar: %v", err)
	}

	porMAC := map[string]Conectado{}
	for _, conectado := range lectura.Conectados {
		porMAC[conectado.MAC] = conectado
	}

	// El cableado cuelga del switch, en su puerto real.
	cableado := porMAC["11:22:33:44:55:66"]
	if cableado.AparatoMAC != "aa:bb:cc:00:00:01" || cableado.Boca != 5 || !cableado.PorCable {
		t.Fatalf("el equipo cableado quedo mal ubicado: %+v", cableado)
	}

	// El inalambrico cuelga de la antena, con el nombre de su red.
	inalambrico := porMAC["77:88:99:aa:bb:cc"]
	if inalambrico.AparatoMAC != "aa:bb:cc:00:00:02" || inalambrico.Red != "Oficina" {
		t.Fatalf("el equipo por WiFi quedo mal ubicado: %+v", inalambrico)
	}

	// **Las bocas inalambricas no pueden chocar con puertos de verdad.** Si la
	// red "Oficina" se numerara como boca 1, sus equipos aparecerian colgando del
	// puerto 1 de un switch.
	if inalambrico.Boca < 1000 {
		t.Fatalf("la boca inalambrica %d podria chocar con un puerto real", inalambrico.Boca)
	}

	// Dos redes distintas de la misma antena son dos bocas distintas.
	invitados := porMAC["dd:ee:ff:00:11:22"]
	if invitados.Boca == inalambrico.Boca {
		t.Fatal("dos redes WiFi distintas deberian ser dos bocas distintas")
	}

	// Y el que la controladora ve pero no sabe de donde cuelga se descarta:
	// inventarle un sitio en el mapa seria peor que dejarlo sin ubicar.
	if _, hay := porMAC["99:99:99:99:99:99"]; hay {
		t.Fatal("un equipo sin antena conocida no deberia colgarse de nada")
	}
}

func TestLasBocasDeUnaAntenaSalenDeQuienEstaConectado(t *testing.T) {
	// Un punto de acceso no tiene una lista de puertos que consultar: sus bocas
	// son las redes WiFi, y solo se sabe cuales estan en uso mirando quien esta
	// conectado.
	servidor := unifiDeMentira(t, false, aparatosDePrueba, conectadosDePrueba)

	lectura, err := Consultar(context.Background(), Acceso{
		Nombre: "prueba", URL: servidor.URL, Usuario: "monitor", Clave: "x",
	}, 5*time.Second)
	if err != nil {
		t.Fatalf("no se pudo consultar: %v", err)
	}

	bocas := BocasDeRedes(lectura.Conectados, "aa:bb:cc:00:00:02")
	if len(bocas) != 2 {
		t.Fatalf("se esperaban dos redes WiFi: %+v", bocas)
	}
	nombres := map[string]bool{}
	for _, boca := range bocas {
		nombres[boca.Nombre] = true
	}
	if !nombres["Oficina"] || !nombres["Invitados"] {
		t.Fatalf("las bocas deberian llamarse como las redes: %+v", bocas)
	}
}

func TestUnaControladoraQueNoAceptaLaClaveLoDiceClaro(t *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(
		func(escritor http.ResponseWriter, _ *http.Request) {
			http.Error(escritor, "no", http.StatusUnauthorized)
		}))
	defer servidor.Close()

	_, err := Consultar(context.Background(), Acceso{
		Nombre: "matriz", URL: servidor.URL, Usuario: "monitor", Clave: "mala",
	}, 2*time.Second)
	if err == nil {
		t.Fatal("una clave equivocada tiene que dar error")
	}
	// El mensaje lleva el nombre de la controladora: con tres dadas de alta, un
	// "no acepto la clave" a secas no dice cual hay que ir a revisar.
	if !strings.Contains(err.Error(), "matriz") {
		t.Fatalf("el error deberia decir de que controladora habla: %v", err)
	}
}

func TestUnaDireccionVaciaSeRechazaAntesDeSalirALaRed(t *testing.T) {
	_, err := Consultar(context.Background(), Acceso{Nombre: "vacia"}, time.Second)
	if err == nil {
		t.Fatal("sin direccion no hay nada que consultar")
	}
}
