package aire

import (
	"testing"
	"time"
)

func TestSeEntiendeLoQueContestaNmcli(t *testing.T) {
	// El formato terse de nmcli escapa los dos puntos que van DENTRO de un
	// valor. Sin deshacer ese escape, la MAC del radio —que va llena de dos
	// puntos— se parte en seis campos y no queda nada legible.
	campos := partirTerse(`CASA-5G:F4\:92\:BF\:0B\:8C\:B6:36:5180 MHz:74:WPA2:wlp3s0`)

	if len(campos) != 7 {
		t.Fatalf("se esperaban 7 campos y salieron %d: %q", len(campos), campos)
	}
	if campos[0] != "CASA-5G" {
		t.Errorf("el SSID salio %q", campos[0])
	}
	if campos[1] != "F4:92:BF:0B:8C:B6" {
		t.Errorf("el BSSID salio %q", campos[1])
	}
	if campos[6] != "wlp3s0" {
		t.Errorf("la tarjeta salio %q", campos[6])
	}
}

func TestUnSSIDConDosPuntosNoRompeLaLectura(t *testing.T) {
	// Alguien puede llamar a su red "casa: arriba". El formato lo escapa igual.
	campos := partirTerse(`casa\: arriba:AA\:BB\:CC\:DD\:EE\:FF:6:2437 MHz:60:WPA2:wlan0`)
	if campos[0] != "casa: arriba" {
		t.Fatalf("el SSID con dos puntos salio %q", campos[0])
	}
	if campos[1] != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("el BSSID salio %q", campos[1])
	}
}

func TestElCanalSaleDeLaFrecuencia(t *testing.T) {
	casos := map[int]struct {
		canal int
		banda string
	}{
		2412: {1, "2.4 GHz"},
		2437: {6, "2.4 GHz"},
		2462: {11, "2.4 GHz"},
		5180: {36, "5 GHz"},
		5745: {149, "5 GHz"},
	}
	for frecuencia, esperado := range casos {
		if canal := canalDeFrecuencia(frecuencia); canal != esperado.canal {
			t.Errorf("%d MHz dio canal %d y debia ser %d", frecuencia, canal, esperado.canal)
		}
		if banda := bandaDeFrecuencia(frecuencia); banda != esperado.banda {
			t.Errorf("%d MHz dio banda %q y debia ser %q", frecuencia, banda, esperado.banda)
		}
	}
}

func TestSeEntiendeLoQueContestaIw(t *testing.T) {
	salida := `BSS f4:92:bf:0b:8c:b6(on wlp3s0)
	TSF: 1234 usec
	freq: 5180
	signal: -47.00 dBm
	SSID: CASA-5G
	RSN:	 * Version: 1
BSS a0:95:7f:bf:5c:60(on wlp3s0)
	freq: 2437
	signal: -71.00 dBm
	SSID: INFINITUM1234
	WPA:	 * Version: 1
BSS 00:11:22:33:44:55(on wlp3s0)
	freq: 2412
	signal: -80.00 dBm
	SSID: `

	redes := leerSalidaDeIw(salida, "wlp3s0")
	if len(redes) != 3 {
		t.Fatalf("se esperaban 3 antenas y salieron %d", len(redes))
	}

	if redes[0].SSID != "CASA-5G" || redes[0].BSSID != "f4:92:bf:0b:8c:b6" {
		t.Errorf("la primera antena salio %+v", redes[0])
	}
	if redes[0].Senal != -47 || redes[0].Canal != 36 || redes[0].Banda != "5 GHz" {
		t.Errorf("la senal o el canal salieron mal: %+v", redes[0])
	}
	if redes[0].Seguridad != "WPA2" {
		t.Errorf("la seguridad salio %q", redes[0].Seguridad)
	}
	// Una red sin nombre existe igual: esconder el SSID no la borra del aire.
	if redes[2].SSID != "" || redes[2].BSSID != "00:11:22:33:44:55" {
		t.Errorf("la red oculta salio %+v", redes[2])
	}
	if redes[2].Seguridad != "abierta" {
		t.Errorf("una red sin cifrado deberia decirse abierta: %q", redes[2].Seguridad)
	}
}

func TestSinTarjetaSeExplicaEnVezDeDevolverNada(t *testing.T) {
	// Una lista vacia sin explicacion se lee como "aqui no hay redes WiFi", que
	// casi nunca es verdad: lo que no hay es con que oirlas.
	resultado := terminar(time.Now(), Resultado{
		Explicacion: "Este equipo no tiene tarjeta WiFi",
	})
	if resultado.Redes == nil {
		t.Fatal("las redes nunca pueden venir en nulo: la interfaz las recorre")
	}
	if resultado.Explicacion == "" {
		t.Fatal("sin redes tiene que haber una explicacion")
	}
}

func TestLasMasFuertesVanPrimero(t *testing.T) {
	resultado := terminar(time.Now(), Resultado{Redes: []RedInalambrica{
		{SSID: "lejos", Senal: -85},
		{SSID: "cerca", Senal: -40},
		{SSID: "media", Senal: -65},
	}})

	if resultado.Redes[0].SSID != "cerca" || resultado.Redes[2].SSID != "lejos" {
		t.Fatalf("el orden por senal salio mal: %+v", resultado.Redes)
	}
}
