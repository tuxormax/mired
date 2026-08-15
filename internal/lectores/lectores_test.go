package lectores

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSinCredencialNoSeTocaNadie(t *testing.T) {
	// Probar claves de fabrica contra el aparato de alguien es exactamente lo
	// que este proyecto NO hace, aunque casi siempre funcione.
	_, err := Leer(context.Background(), Destino{Direccion: "192.0.2.1"})
	if err != ErrSinCredencial {
		t.Fatalf("se esperaba ErrSinCredencial y llego: %v", err)
	}
}

func TestLaMacSeGuardaSiempreIgual(t *testing.T) {
	for _, escrita := range []string{
		"AA:BB:CC:11:22:33", "aa-bb-cc-11-22-33", "AABB.CC11.2233", "aabbcc112233",
	} {
		if NormalizarMAC(escrita) != "aa:bb:cc:11:22:33" {
			t.Errorf("%q quedo como %q", escrita, NormalizarMAC(escrita))
		}
	}
	// Y lo que no es una MAC no se convierte en una.
	for _, texto := range []string{"", "hola", "aa:bb:cc", "1.2.3.4"} {
		if NormalizarMAC(texto) != "" {
			t.Errorf("%q se tomo por una MAC", texto)
		}
	}
}

func TestElLectorGenericoSacaLasMacDeUnPanel(t *testing.T) {
	// El panel de un aparato cualquiera: una tabla de conectados en HTML. No se
	// entiende el formato, pero las MAC son inconfundibles.
	panel := httptest.NewServer(http.HandlerFunc(func(escritor http.ResponseWriter, _ *http.Request) {
		_, _ = escritor.Write([]byte(`<html><body><h1>Equipos conectados</h1>
			<table>
			<tr><td>telefono</td><td>AA:BB:CC:11:22:33</td><td>192.168.1.20</td></tr>
			<tr><td>laptop</td><td>dd-ee-ff-44-55-66</td><td>192.168.1.21</td></tr>
			</table></body></html>`))
	}))
	defer panel.Close()

	conectados, err := (Generico{}).Leer(context.Background(), Destino{
		Direccion: panel.URL, Usuario: "admin", Clave: "admin", Espera: 3 * time.Second,
	})
	if err != nil {
		t.Fatalf("no se pudo leer el panel: %v", err)
	}
	if len(conectados) != 2 {
		t.Fatalf("se esperaban 2 conectados y salieron %d: %+v", len(conectados), conectados)
	}
	// Sin saber como entro cada uno, se marcan como cable: colgar del WiFi algo
	// que puede estar enchufado seria dibujar una red que no existe.
	for _, conectado := range conectados {
		if !conectado.PorCable {
			t.Errorf("el generico no puede afirmar que algo entro por WiFi: %+v", conectado)
		}
	}
}

func TestUnPanelSinMacNoInventaConectados(t *testing.T) {
	panel := httptest.NewServer(http.HandlerFunc(func(escritor http.ResponseWriter, _ *http.Request) {
		_, _ = escritor.Write([]byte(`<html><body>Bienvenido. No hay nada aqui.</body></html>`))
	}))
	defer panel.Close()

	_, err := (Generico{}).Leer(context.Background(), Destino{
		Direccion: panel.URL, Usuario: "admin", Clave: "admin", Espera: 3 * time.Second,
	})
	if err != ErrNoSeEntendio {
		t.Fatalf("se esperaba ErrNoSeEntendio y llego: %v", err)
	}
}

func TestUnaCredencialRechazadaSeDistingueDeUnFallo(t *testing.T) {
	// Un 401 se corrige cambiando la credencial; un fallo de red se corrige
	// esperando. Confundirlos manda al usuario a buscar donde no es.
	panel := httptest.NewServer(http.HandlerFunc(func(escritor http.ResponseWriter, _ *http.Request) {
		escritor.WriteHeader(http.StatusUnauthorized)
	}))
	defer panel.Close()

	_, err := pedir(context.Background(), nuevaSesion(time.Second), "GET", panel.URL,
		nil, nil, "admin", "mala")
	if !EsCredencialRechazada(err) {
		t.Fatalf("un 401 tiene que reconocerse como credencial rechazada: %v", err)
	}
	if !strings.Contains(err.Error(), "credencial") {
		t.Errorf("el mensaje no explica que fue la credencial: %v", err)
	}
}

func TestSeEntiendeLaTablaDeUnMikrotik(t *testing.T) {
	crudo := []byte(`[
		{"mac-address":"AA:BB:CC:11:22:33","ssid":"CASA-5G","signal-strength":"-58dBm@1Mbps"},
		{"mac-address":"DD:EE:FF:44:55:66","interface":"wlan1","signal":"-71"}
	]`)

	conectados := leerRegistroMikrotik(crudo)
	if len(conectados) != 2 {
		t.Fatalf("se esperaban 2 y salieron %d", len(conectados))
	}
	if conectados[0].Red != "CASA-5G" || conectados[0].SenalDbm != -58 {
		t.Errorf("el primero salio %+v", conectados[0])
	}
	if conectados[0].PorCable {
		t.Error("un asociado por WiFi no puede marcarse como cable")
	}
}

func TestLoQueRepartePorDhcpNoSeCuelgaDelWifi(t *testing.T) {
	// Una direccion repartida por DHCP no dice como entro el equipo. Marcarla
	// como inalambrica seria inventar.
	conectados := leerArrendamientosMikrotik([]byte(
		`[{"mac-address":"AA:BB:CC:11:22:33","host-name":"laptop","address":"192.168.1.20"}]`))

	if len(conectados) != 1 || !conectados[0].PorCable {
		t.Fatalf("un arrendamiento DHCP no puede darse por inalambrico: %+v", conectados)
	}
	if conectados[0].Nombre != "laptop" || conectados[0].IP != "192.168.1.20" {
		t.Errorf("no se aprovecho el nombre ni la direccion: %+v", conectados[0])
	}
}

func TestSeEntiendeLaRespuestaDeUnModemPorTR064(t *testing.T) {
	// Un modem contesta SOAP. Lo que importa: la MAC, si esta activo y si entro
	// por cable o por el aire.
	respuesta := `<?xml version="1.0"?><s:Envelope><s:Body>
		<u:GetGenericHostEntryResponse>
		<NewMACAddress>AA:BB:CC:11:22:33</NewMACAddress>
		<NewHostName>telefono-de-ana</NewHostName>
		<NewIPAddress>192.168.1.20</NewIPAddress>
		<NewActive>1</NewActive>
		<NewInterfaceType>802.11</NewInterfaceType>
		</u:GetGenericHostEntryResponse></s:Body></s:Envelope>`

	if valor := entreEtiquetas(respuesta, "NewMACAddress"); valor != "AA:BB:CC:11:22:33" {
		t.Errorf("la MAC salio %q", valor)
	}
	if valor := entreEtiquetas(respuesta, "NewHostName"); valor != "telefono-de-ana" {
		t.Errorf("el nombre salio %q", valor)
	}
	if valor := entreEtiquetas(respuesta, "NewInterfaceType"); valor != "802.11" {
		t.Errorf("el tipo de interfaz salio %q", valor)
	}
	// Y con prefijo de espacio de nombres tambien: cada fabricante usa el suyo.
	conPrefijo := `<u:NewMACAddress>DD:EE:FF:44:55:66</u:NewMACAddress>`
	if valor := entreEtiquetas(conPrefijo, "NewMACAddress"); valor != "DD:EE:FF:44:55:66" {
		t.Errorf("con prefijo salio %q", valor)
	}
}

func TestLaDireccionDelPanelSeEntiendeComoVenga(t *testing.T) {
	casos := map[string]string{
		"192.168.1.254":              "192.168.1.254",
		"http://192.168.1.254":       "192.168.1.254",
		"https://192.168.1.254:8443": "192.168.1.254",
		"192.168.1.254:8080/panel":   "192.168.1.254",
	}
	for escrita, esperada := range casos {
		if soloMaquina(escrita) != esperada {
			t.Errorf("%q dio %q y se esperaba %q", escrita, soloMaquina(escrita), esperada)
		}
	}
}

func TestPrimeroSePruebaElLectorDeLaMarca(t *testing.T) {
	// Un modem Huawei no tiene por que aguantar que le hablen en el idioma de
	// MikroTik antes de que alguien pruebe el suyo.
	if !(TR064{}).Reconoce("huawei hg8245 internetgatewaydevice") {
		t.Error("TR-064 deberia reconocer un modem Huawei")
	}
	if (MikroTik{}).Reconoce("huawei hg8245") {
		t.Error("MikroTik no deberia reconocer un Huawei")
	}
	if !(AirOS{}).Reconoce("ubiquiti nanostation airos") {
		t.Error("AirOS deberia reconocer una antena Ubiquiti")
	}
	// El generico no es de nadie, y por eso va al final.
	if (Generico{}).Reconoce("cualquier cosa") {
		t.Error("el generico no puede declararse de ninguna marca")
	}
}
