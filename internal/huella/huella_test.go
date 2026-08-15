package huella

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
)

func puertoDe(t *testing.T, servidor *httptest.Server) (string, int) {
	t.Helper()
	direccion, err := url.Parse(servidor.URL)
	if err != nil {
		t.Fatalf("no se pudo leer la direccion del servidor: %v", err)
	}
	numero, _ := strconv.Atoi(direccion.Port())
	return direccion.Hostname(), numero
}

func TestLaPaginaDiceQueEsElAparato(t *testing.T) {
	// El caso real: un modem y una antena tienen los dos el puerto 80 abierto.
	// Lo unico que los distingue sin entrar es lo que dice su propia pagina.
	servidor := httptest.NewServer(http.HandlerFunc(func(escritor http.ResponseWriter, _ *http.Request) {
		escritor.Header().Set("Server", "lighttpd/1.4.59")
		escritor.Header().Set("WWW-Authenticate", `Basic realm="UniFi AP-AC-LR"`)
		_, _ = escritor.Write([]byte("<html><head><title>UniFi Network</title></head><body>hola</body></html>"))
	}))
	defer servidor.Close()

	ip, puerto := puertoDe(t, servidor)
	var huella Huella
	leerPagina(context.Background(), &huella, ip, puerto, false, 2*time.Second)
	if huella.Valor(ClaveTitulo) != "UniFi Network" {
		t.Errorf("el titulo salio %q", huella.Valor(ClaveTitulo))
	}
	if huella.Valor(ClaveServidor) != "lighttpd/1.4.59" {
		t.Errorf("el encabezado Server salio %q", huella.Valor(ClaveServidor))
	}
	if huella.Valor(ClaveRealm) != "UniFi AP-AC-LR" {
		t.Errorf("el realm salio %q", huella.Valor(ClaveRealm))
	}
}

func TestElCertificadoTraeLaMarca(t *testing.T) {
	// Los aparatos de red se firman su propio certificado con su nombre de
	// fabrica. Es un dato que no cambia aunque le pongan clave al panel.
	llave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("no se pudo generar la llave: %v", err)
	}
	plantilla := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:         "UBNT",
			Organization:       []string{"Ubiquiti Inc"},
			OrganizationalUnit: []string{"NanoStation M5"},
		},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	crudo, err := x509.CreateCertificate(rand.Reader, &plantilla, &plantilla, &llave.PublicKey, llave)
	if err != nil {
		t.Fatalf("no se pudo firmar el certificado: %v", err)
	}

	servidor := httptest.NewUnstartedServer(http.HandlerFunc(func(escritor http.ResponseWriter, _ *http.Request) {
		_, _ = escritor.Write([]byte("<title>airOS</title>"))
	}))
	servidor.TLS = &tls.Config{Certificates: []tls.Certificate{{
		Certificate: [][]byte{crudo},
		PrivateKey:  llave,
	}}}
	servidor.StartTLS()
	defer servidor.Close()

	ip, puerto := puertoDe(t, servidor)
	datos := delCertificado(context.Background(), ip, puerto, 2*time.Second)

	huella := Huella{Datos: datos}
	if huella.Valor(ClaveNombre) != "UBNT" {
		t.Errorf("el nombre del certificado salio %q", huella.Valor(ClaveNombre))
	}
	if huella.Valor(ClaveFabricante) != "Ubiquiti Inc" {
		t.Errorf("la organizacion salio %q", huella.Valor(ClaveFabricante))
	}
	if huella.Valor(ClaveModelo) != "NanoStation M5" {
		t.Errorf("el modelo salio %q", huella.Valor(ClaveModelo))
	}
}

func TestSeEntiendeLoQueContestaUnUbiquiti(t *testing.T) {
	// Respuesta armada como la manda un AirMax: cabecera y luego tipo, largo y
	// valor. Trae el SSID, que es la unica forma de saber que red emite sin
	// tener una tarjeta WiFi escuchando.
	respuesta := []byte{0x01, 0x06, 0x00, 0x00}
	agregar := func(tipo byte, valor []byte) {
		cabeza := make([]byte, 3)
		cabeza[0] = tipo
		binary.BigEndian.PutUint16(cabeza[1:3], uint16(len(valor)))
		respuesta = append(respuesta, cabeza...)
		respuesta = append(respuesta, valor...)
	}
	agregar(0x01, []byte{0xf4, 0x92, 0xbf, 0x0b, 0x8c, 0xb6})
	agregar(0x03, []byte("WA.ar934x.v6.3.11"))
	agregar(0x0b, []byte("antena-patio"))
	agregar(0x0c, []byte("NanoStation M5"))
	agregar(0x0d, []byte("CASA-5G"))

	ip, cerrar := servidorUDPDePrueba(t, respuesta)
	defer cerrar()

	// Se apunta el paquete a donde escucha la prueba.
	datos := conPuertoCambiado(t, ip, func(destino string, puerto int) []Dato {
		return leerUbiquiti(preguntarUDP(context.Background(), destino, puerto,
			[]byte{0x01, 0x00, 0x00, 0x00}, time.Second))
	})

	huella := Huella{Datos: datos}
	if huella.Valor(ClaveModelo) != "NanoStation M5" {
		t.Errorf("el modelo salio %q", huella.Valor(ClaveModelo))
	}
	if huella.Valor(ClaveNombre) != "antena-patio" {
		t.Errorf("el nombre salio %q", huella.Valor(ClaveNombre))
	}
	if huella.Valor(ClaveRed) != "CASA-5G" {
		t.Errorf("el SSID salio %q", huella.Valor(ClaveRed))
	}
	if huella.Valor(ClaveMAC) != "f4:92:bf:0b:8c:b6" {
		t.Errorf("la MAC del radio salio %q", huella.Valor(ClaveMAC))
	}
}

func TestElXorDeTPLinkVaYVuelve(t *testing.T) {
	claro := []byte(`{"system":{"get_sysinfo":{}}}`)
	if vuelta := string(destaparTPLink(taparTPLink(claro))); vuelta != string(claro) {
		t.Fatalf("el mensaje no volvio igual: %q", vuelta)
	}

	respuesta := taparTPLink([]byte(
		`{"system":{"get_sysinfo":{"sw_ver":"1.5.6","model":"HS103(US)","alias":"Foco de la sala","mac":"AC:84:C6:00:11:22"}}}`))
	texto := string(destaparTPLink(respuesta))
	if entreComillas(texto, `"model"`) != "HS103(US)" {
		t.Errorf("el modelo salio %q", entreComillas(texto, `"model"`))
	}
	if entreComillas(texto, `"alias"`) != "Foco de la sala" {
		t.Errorf("el alias salio %q", entreComillas(texto, `"alias"`))
	}
}

func TestSeEntiendeElNombreDeUnaMaquinaWindows(t *testing.T) {
	// Respuesta de estado de nodo con dos nombres: la maquina y su grupo.
	respuesta := make([]byte, 56)
	respuesta[56-1] = 0
	respuesta = append(respuesta, 2) // cuantos nombres vienen

	nombre := func(texto string, tipo byte, banderas uint16) []byte {
		relleno := texto + strings.Repeat(" ", 15-len(texto))
		fila := append([]byte(relleno), tipo)
		dos := make([]byte, 2)
		binary.BigEndian.PutUint16(dos, banderas)
		return append(fila, dos...)
	}
	respuesta = append(respuesta, nombre("CAJA-RECEPCION", 0x00, 0x0400)...)
	respuesta = append(respuesta, nombre("OFICINA", 0x00, 0x8400)...)

	var huella Huella
	for _, dato := range leerNetBIOS(respuesta) {
		huella.Agregar(dato.Fuente, dato.Clave, dato.Valor)
	}
	if huella.Valor(ClaveNombre) != "CAJA-RECEPCION" {
		t.Errorf("el nombre de la maquina salio %q", huella.Valor(ClaveNombre))
	}
	if !strings.Contains(huella.Valor(ClaveServicio), "OFICINA") {
		t.Errorf("el grupo de trabajo salio %q", huella.Valor(ClaveServicio))
	}
}

func TestSeEntiendeUnaRespuestaDeSSDP(t *testing.T) {
	var huella Huella
	leerSSDP(&huella, "HTTP/1.1 200 OK\r\n"+
		"CACHE-CONTROL: max-age=1800\r\n"+
		"SERVER: Linux/3.10 UPnP/1.0 Huawei-HG8245Q2/1.0\r\n"+
		"ST: urn:schemas-upnp-org:device:InternetGatewayDevice:1\r\n"+
		"USN: uuid:3e4b1c22-0000-1000-8000-a0957fbf5c60::upnp:rootdevice\r\n\r\n")

	if !strings.Contains(huella.Valor(ClaveServidor), "Huawei-HG8245Q2") {
		t.Errorf("el servidor salio %q", huella.Valor(ClaveServidor))
	}
	if !strings.Contains(huella.Valor(ClaveServicio), "InternetGatewayDevice") {
		t.Errorf("el servicio salio %q", huella.Valor(ClaveServicio))
	}
}

func TestSeEntiendeUnaCamaraOnvif(t *testing.T) {
	var huella Huella
	leerWSDiscovery(&huella, `<e:Envelope><e:Body><d:ProbeMatches><d:ProbeMatch>`+
		`<d:Scopes>onvif://www.onvif.org/name/DVR-Entrada onvif://www.onvif.org/hardware/XVR5108HS `+
		`onvif://www.onvif.org/manufacturer/Dahua</d:Scopes>`+
		`</d:ProbeMatch></d:ProbeMatches></e:Body></e:Envelope>`)

	if huella.Valor(ClaveModelo) != "XVR5108HS" {
		t.Errorf("el modelo salio %q", huella.Valor(ClaveModelo))
	}
	if huella.Valor(ClaveFabricante) != "Dahua" {
		t.Errorf("el fabricante salio %q", huella.Valor(ClaveFabricante))
	}
	if huella.Valor(ClaveNombre) != "DVR-Entrada" {
		t.Errorf("el nombre salio %q", huella.Valor(ClaveNombre))
	}
}

func TestSeEntiendeUnChromecastPorMDNS(t *testing.T) {
	// Un PTR que apunta a la instancia, y un TXT con el modelo. Es exactamente
	// lo que contesta un Chromecast.
	mensaje := armarRespuestaMDNS(t)

	var huella Huella
	leerMDNS(&huella, mensaje)

	if huella.Valor(ClaveNombre) != "TV de la sala" {
		t.Errorf("el nombre salio %q", huella.Valor(ClaveNombre))
	}
	if huella.Valor(ClaveModelo) != "Chromecast Ultra" {
		t.Errorf("el modelo salio %q", huella.Valor(ClaveModelo))
	}
	if !strings.Contains(huella.Valor(ClaveServicio), "_googlecast") {
		t.Errorf("el servicio salio %q", huella.Valor(ClaveServicio))
	}
}

func TestLoQueNoContestaNoSeInventa(t *testing.T) {
	// Una direccion donde no hay nadie no puede producir ni un solo dato.
	ctx, cancelar := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelar()

	if datos := DeUnEquipo(ctx, "192.0.2.1", []int{80, 443}, 300*time.Millisecond); len(datos) != 0 {
		t.Fatalf("de una direccion vacia salieron %d datos: %+v", len(datos), datos)
	}
}

// ------------------------------------------------------------ auxiliares --

// servidorUDPDePrueba levanta algo que conteste siempre lo mismo.
func servidorUDPDePrueba(t *testing.T, respuesta []byte) (string, func()) {
	t.Helper()
	conexion, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Skipf("no se pudo abrir un puerto UDP: %v", err)
	}

	listo := make(chan struct{})
	go func() {
		defer close(listo)
		buzon := make([]byte, 1024)
		_ = conexion.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, quien, err := conexion.ReadFromUDP(buzon)
		if err != nil {
			return
		}
		_, _ = conexion.WriteToUDP(respuesta, quien)
	}()

	direccion := conexion.LocalAddr().(*net.UDPAddr)
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(direccion.Port)), func() {
		_ = conexion.Close()
		<-listo
	}
}

// conPuertoCambiado corre la pregunta contra el puerto que levanto la prueba.
func conPuertoCambiado(t *testing.T, direccion string, hacer func(string, int) []Dato) []Dato {
	t.Helper()
	maquina, puerto, err := net.SplitHostPort(direccion)
	if err != nil {
		t.Fatalf("direccion rara: %v", err)
	}
	numero, _ := strconv.Atoi(puerto)
	return hacer(maquina, numero)
}

func armarRespuestaMDNS(t *testing.T) []byte {
	t.Helper()
	mensaje := make([]byte, 12)
	binary.BigEndian.PutUint16(mensaje[2:4], 0x8400) // es una respuesta
	binary.BigEndian.PutUint16(mensaje[6:8], 2)      // dos respuestas

	instancia := "TV de la sala._googlecast._tcp.local."
	servicio := "_googlecast._tcp.local."

	// PTR: el servicio apunta a la instancia.
	mensaje = append(mensaje, comoNombreDNS(servicio)...)
	mensaje = append(mensaje, 0x00, 0x0c, 0x00, 0x01, 0x00, 0x00, 0x00, 0x78)
	apuntado := comoNombreDNS(instancia)
	dos := make([]byte, 2)
	binary.BigEndian.PutUint16(dos, uint16(len(apuntado)))
	mensaje = append(mensaje, dos...)
	mensaje = append(mensaje, apuntado...)

	// TXT con el modelo.
	mensaje = append(mensaje, comoNombreDNS(instancia)...)
	mensaje = append(mensaje, 0x00, 0x10, 0x00, 0x01, 0x00, 0x00, 0x00, 0x78)
	texto := []byte("md=Chromecast Ultra")
	datos := append([]byte{byte(len(texto))}, texto...)
	binary.BigEndian.PutUint16(dos, uint16(len(datos)))
	mensaje = append(mensaje, dos...)
	mensaje = append(mensaje, datos...)

	return mensaje
}
