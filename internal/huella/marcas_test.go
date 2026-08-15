package huella

import (
	"crypto/aes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestSeEntiendeUnSwitchNetgear(t *testing.T) {
	// NSDP es de los documentados: cabecera de 32 bytes con la firma, y despues
	// campos con tipo, largo y valor.
	respuesta := make([]byte, 32)
	respuesta[0], respuesta[1] = 0x01, 0x02 // version y "esto es una respuesta"
	copy(respuesta[24:28], []byte("NSDP"))

	campo := func(tipo uint16, valor []byte) {
		cabeza := make([]byte, 4)
		binary.BigEndian.PutUint16(cabeza[0:2], tipo)
		binary.BigEndian.PutUint16(cabeza[2:4], uint16(len(valor)))
		respuesta = append(respuesta, cabeza...)
		respuesta = append(respuesta, valor...)
	}
	campo(0x0001, []byte("GS308E"))
	campo(0x0003, []byte("switch-bodega"))
	campo(0x0004, []byte{0x9c, 0x3d, 0xcf, 0x11, 0x22, 0x33})
	campo(0x000d, []byte("V1.00.11"))
	respuesta = append(respuesta, 0xff, 0xff, 0x00, 0x00)

	huella := Huella{Datos: leerNSDP(respuesta)}
	if huella.Valor(ClaveModelo) != "GS308E" {
		t.Errorf("el modelo salio %q", huella.Valor(ClaveModelo))
	}
	if huella.Valor(ClaveNombre) != "switch-bodega" {
		t.Errorf("el nombre salio %q", huella.Valor(ClaveNombre))
	}
	if huella.Valor(ClaveMAC) != "9c:3d:cf:11:22:33" {
		t.Errorf("la MAC salio %q", huella.Valor(ClaveMAC))
	}
	if huella.Valor(ClaveFirmware) != "V1.00.11" {
		t.Errorf("el firmware salio %q", huella.Valor(ClaveFirmware))
	}
}

func TestUnaRespuestaQueNoEsNSDPNoSeInventa(t *testing.T) {
	if datos := leerNSDP([]byte("cualquier cosa que conteste otro aparato")); datos != nil {
		t.Fatalf("se invento una lectura: %+v", datos)
	}
}

func TestElNumeroDeSerieDeFortinetDiceElModelo(t *testing.T) {
	// Fortinet numera sus aparatos con el modelo delante. Es la unica forma de
	// saber el modelo sin entrar al equipo.
	casos := map[string]string{
		"FGT60FTK20012345":  "FortiGate 60F",
		"FGT100ETK99999999": "FortiGate 100E",
		"FWF61ETK11112222":  "FortiWiFi 61E",
		"FAP221ETF12345678": "FortiAP 221E",
		"XXX123":            "",
		"corto":             "",
	}
	for serie, esperado := range casos {
		if modelo := modeloDeSerieFortinet(serie); modelo != esperado {
			t.Errorf("%s dio %q y se esperaba %q", serie, modelo, esperado)
		}
	}
}

func TestSeReconoceUnFortiGatePorSuCertificado(t *testing.T) {
	for _, texto := range []string{"FGT60FTK20012345", "Fortinet Ltd.", "fortigate"} {
		if !pareceFortinet(texto) {
			t.Errorf("%q deberia reconocerse como Fortinet", texto)
		}
	}
	// Y no cualquier cosa: "fort" suelto no es una marca.
	for _, texto := range []string{"comfort", "Ubiquiti Inc", ""} {
		if pareceFortinet(texto) {
			t.Errorf("%q NO es Fortinet", texto)
		}
	}
}

func TestSeEntiendeElAnuncioDeUnAparatoTuya(t *testing.T) {
	// Los aparatos Tuya no se dejan preguntar: avisan solos. El anuncio va
	// tapado con una llave FIJA y publica, la misma en todo el mundo.
	claro := []byte(`{"ip":"192.168.1.71","gwId":"bf1234567890abcdef","active":2,` +
		`"productKey":"keyxxxxxxxxxxxxx","version":"3.3"}`)

	bloque, err := aes.NewCipher(llaveTuya[:])
	if err != nil {
		t.Fatalf("no se pudo preparar la llave: %v", err)
	}
	relleno := aes.BlockSize - len(claro)%aes.BlockSize
	for i := 0; i < relleno; i++ {
		claro = append(claro, byte(relleno))
	}
	tapado := make([]byte, len(claro))
	for inicio := 0; inicio < len(claro); inicio += aes.BlockSize {
		bloque.Encrypt(tapado[inicio:inicio+aes.BlockSize], claro[inicio:inicio+aes.BlockSize])
	}

	// Cabecera de 20 bytes y cola de 8, como manda el aparato.
	mensaje := append(make([]byte, 20), tapado...)
	mensaje = append(mensaje, make([]byte, 8)...)

	huella := Huella{Datos: leerAnuncioTuya(mensaje)}
	if huella.Valor(ClaveSerie) != "bf1234567890abcdef" {
		t.Errorf("el identificador salio %q", huella.Valor(ClaveSerie))
	}
	if huella.Valor(ClaveFirmware) != "3.3" {
		t.Errorf("la version salio %q", huella.Valor(ClaveFirmware))
	}
	if !strings.Contains(huella.Valor(ClaveFabricante), "Tuya") {
		t.Errorf("el fabricante salio %q", huella.Valor(ClaveFabricante))
	}
}

func TestElAnuncioTuyaViejoVaEnClaro(t *testing.T) {
	// Los aparatos mas viejos anuncian sin tapar, por el 6666.
	mensaje := []byte(`{"ip":"192.168.1.71","gwId":"bfabcdef","version":"3.1"}`)
	huella := Huella{Datos: leerAnuncioTuya(mensaje)}

	if huella.Valor(ClaveSerie) != "bfabcdef" {
		t.Errorf("el identificador salio %q", huella.Valor(ClaveSerie))
	}
}

func TestLoQueNoEsTuyaNoSeAnota(t *testing.T) {
	if datos := leerAnuncioTuya([]byte("hola, soy otro protocolo cualquiera")); len(datos) != 0 {
		t.Fatalf("se anoto un aparato que no es Tuya: %+v", datos)
	}
}

func TestSeEntiendeElSaludoDeXiaomi(t *testing.T) {
	respuesta := make([]byte, 32)
	respuesta[0], respuesta[1] = 0x21, 0x31
	binary.BigEndian.PutUint32(respuesta[8:12], 123456789)

	huella := Huella{Datos: leerXiaomi(respuesta)}
	if huella.Valor(ClaveFabricante) != "Xiaomi" {
		t.Errorf("el fabricante salio %q", huella.Valor(ClaveFabricante))
	}
	if huella.Valor(ClaveSerie) != "123456789" {
		t.Errorf("el identificador salio %q", huella.Valor(ClaveSerie))
	}
	// Lo que no es miIO no se toma por miIO.
	if datos := leerXiaomi([]byte("no soy un saludo miIO")); datos != nil {
		t.Errorf("se invento un Xiaomi: %+v", datos)
	}
}

func TestDeLoQueSoloSeConocePorIngenieriaInversaSeSacaLoCierto(t *testing.T) {
	// De un protocolo que no se decodifica entero se sacan las cadenas legibles
	// y se anota SOLO lo que se reconoce. Es poco, pero es cierto.
	respuesta := append([]byte{0x0b, 0x02, 0x00, 0x1f, 0x00, 0x00},
		[]byte("switch-recepcion\x00DGS-1210-10P\x00\x01\x02")...)

	huella := Huella{Datos: leerRespuestaDeMarca(respuesta, FuenteDLink, "D-Link",
		[]string{"dgs-", "des-", "dap-"})}

	if huella.Valor(ClaveModelo) != "DGS-1210-10P" {
		t.Errorf("el modelo salio %q", huella.Valor(ClaveModelo))
	}
	if huella.Valor(ClaveNombre) != "switch-recepcion" {
		t.Errorf("el nombre salio %q", huella.Valor(ClaveNombre))
	}
	if huella.Valor(ClaveFabricante) != "D-Link" {
		t.Errorf("el fabricante salio %q", huella.Valor(ClaveFabricante))
	}
}

func TestSinRespuestaNoSeAnotaNiElFabricante(t *testing.T) {
	// Que un aparato no conteste el protocolo de una marca NO lo hace de esa
	// marca. Sin respuesta no se anota nada, ni siquiera el nombre.
	if datos := leerRespuestaDeMarca(nil, FuenteDLink, "D-Link", []string{"dgs-"}); datos != nil {
		t.Fatalf("se anoto un fabricante sin respuesta: %+v", datos)
	}
}

func TestLasCadenasCortasSonRuidoYSeTiran(t *testing.T) {
	cadenas := cadenasLegibles([]byte{0x01, 'a', 'b', 0x00, 'h', 'o', 'l', 'a', 0x00, 'x'}, 4)
	if len(cadenas) != 1 || cadenas[0] != "hola" {
		t.Fatalf("las cadenas salieron %+v", cadenas)
	}
}
