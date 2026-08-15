package lectores

import "testing"

func TestSeEntiendeLaListaSueltaDeAirOS(t *testing.T) {
	// Forma 1: una lista de estaciones, como contesta /sta.cgi en AirOS 6.
	crudo := []byte(`[
		{"mac":"AA:BB:CC:11:22:33","name":"telefono-de-ana","signal":-52,"essid":"CASA-5G"},
		{"mac":"dd:ee:ff:44:55:66","hostname":"tv-sala","rssi":-71}
	]`)

	conectados := leerEstacionesAirOS(crudo)
	if len(conectados) != 2 {
		t.Fatalf("se esperaban 2 estaciones y salieron %d", len(conectados))
	}
	if conectados[0].Nombre != "telefono-de-ana" || conectados[0].SenalDbm != -52 {
		t.Errorf("la primera estacion salio %+v", conectados[0])
	}
	// La segunda trae la senal en el otro campo y el nombre en hostname.
	if conectados[1].Nombre != "tv-sala" || conectados[1].SenalDbm != -71 {
		t.Errorf("la segunda estacion salio %+v", conectados[1])
	}
	// Y nada de esto es por cable: son asociados a un radio.
	for _, conectado := range conectados {
		if conectado.PorCable {
			t.Errorf("un asociado por WiFi no puede marcarse como cable: %+v", conectado)
		}
	}
}

func TestSeEntiendeElSobreDeAirOS(t *testing.T) {
	// Forma 2: el objeto con la lista dentro, como contesta /status.cgi. El SSID
	// esta una vez para todos, no en cada estacion.
	crudo := []byte(`{"wireless":{"essid":"CASA-5G","mode":"ap",
		"sta":[{"mac":"11:22:33:44:55:66","signal":-45,"remote":{"hostname":"laptop"}}]}}`)

	conectados := leerEstacionesAirOS(crudo)
	if len(conectados) != 1 {
		t.Fatalf("se esperaba 1 estacion y salieron %d", len(conectados))
	}
	if conectados[0].Red != "CASA-5G" {
		t.Errorf("el SSID salio %q: deberia heredarse del sobre", conectados[0].Red)
	}
	if conectados[0].Nombre != "laptop" {
		t.Errorf("el nombre salio %q", conectados[0].Nombre)
	}
}

func TestDeAirOSNoSeInventaLoQueNoSeEntiende(t *testing.T) {
	for _, crudo := range [][]byte{
		[]byte(`<html>404 not found</html>`),
		[]byte(`{"otra":"cosa"}`),
		[]byte(`[{"sin":"mac"}]`),
		nil,
	} {
		if conectados := leerEstacionesAirOS(crudo); len(conectados) != 0 {
			t.Errorf("de %q salieron estaciones: %+v", crudo, conectados)
		}
	}
}
