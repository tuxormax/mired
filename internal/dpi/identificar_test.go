package dpi

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

// Estas pruebas arman saludos de TLS, peticiones de HTTP y consultas de DNS byte
// por byte.
//
// Es donde esta el riesgo de verdad de todo el paquete: recorrer un saludo de
// TLS es contar longitudes encadenadas, y equivocarse en una no da error, da un
// nombre cortado o el nombre de otro sitio. Y un informe que dice "netflix.com"
// cuando era "windowsupdate.com" es peor que no tener informe.

// saludoTLS arma un ClientHello con el nombre de servidor indicado.
func saludoTLS(nombre string) []byte {
	// Extension server_name: tipo(2) largo(2) | lista(2) tipo(1) largo(2) nombre
	extension := []byte{0, 0, 0, 0}
	interior := []byte{0, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(interior[0:2], uint16(len(nombre)+3))
	interior[2] = 0 // tipo: nombre de maquina
	binary.BigEndian.PutUint16(interior[3:5], uint16(len(nombre)))
	interior = append(interior, nombre...)
	binary.BigEndian.PutUint16(extension[0:2], 0) // server_name
	binary.BigEndian.PutUint16(extension[2:4], uint16(len(interior)))
	extension = append(extension, interior...)

	cuerpo := []byte{0x03, 0x03} // version
	cuerpo = append(cuerpo, make([]byte, 32)...)
	cuerpo = append(cuerpo, 0)          // sin identificador de sesion
	cuerpo = append(cuerpo, 0, 2, 0, 0) // una suite de cifrado
	cuerpo = append(cuerpo, 1, 0)       // un metodo de compresion
	largoExtensiones := make([]byte, 2)
	binary.BigEndian.PutUint16(largoExtensiones, uint16(len(extension)))
	cuerpo = append(cuerpo, largoExtensiones...)
	cuerpo = append(cuerpo, extension...)

	saludo := []byte{0x01, 0, 0, 0} // ClientHello y su largo de 3 bytes
	saludo[1] = byte(len(cuerpo) >> 16)
	saludo[2] = byte(len(cuerpo) >> 8)
	saludo[3] = byte(len(cuerpo))
	saludo = append(saludo, cuerpo...)

	registro := []byte{0x16, 0x03, 0x01, 0, 0} // handshake
	binary.BigEndian.PutUint16(registro[3:5], uint16(len(saludo)))
	return append(registro, saludo...)
}

// consultaDNS arma una pregunta por un nombre.
func consultaDNS(nombre string) []byte {
	mensaje := make([]byte, 12)
	binary.BigEndian.PutUint16(mensaje[0:2], 0x1234) // identificador
	binary.BigEndian.PutUint16(mensaje[4:6], 1)      // una pregunta

	inicio := 0
	for i := 0; i <= len(nombre); i++ {
		if i == len(nombre) || nombre[i] == '.' {
			mensaje = append(mensaje, byte(i-inicio))
			mensaje = append(mensaje, nombre[inicio:i]...)
			inicio = i + 1
		}
	}
	return append(mensaje, 0, 0, 1, 0, 1) // fin del nombre, tipo A, clase IN
}

func TestElNombreDelServidorSaleDelSaludoDeTLS(t *testing.T) {
	// Es el caso que mas importa: hoy casi todo el trafico va cifrado, y el
	// saludo de TLS es lo unico que sigue diciendo con quien se habla.
	pista, ok := IdentificarTCP(saludoTLS("www.netflix.com"), 443)
	if !ok {
		t.Fatal("no se reconocio el saludo de TLS")
	}
	if pista.Nombre != "www.netflix.com" || pista.Como != "tls" {
		t.Fatalf("se leyo mal: %+v", pista)
	}
}

func TestUnNombreLargoNoSeCortaNiSeCorre(t *testing.T) {
	// Un nombre largo obliga a que las longitudes encadenadas del saludo esten
	// todas bien: si una se lee corrida, el nombre sale truncado o con basura, y
	// ninguna de las dos cosas da error.
	largo := "video-edge-a1b2c3.ftl.iad03.hls.contribucion.ttvnw.example.net"
	pista, ok := IdentificarTCP(saludoTLS(largo), 443)
	if !ok || pista.Nombre != largo {
		t.Fatalf("se esperaba %q y salio %+v", largo, pista)
	}
}

func TestLaCabeceraHostIdentificaLoQueSigueSinCifrar(t *testing.T) {
	peticion := "GET /actualizacion.bin HTTP/1.1\r\n" +
		"Host: descargas.empresa.com.mx:8080\r\n" +
		"User-Agent: algo\r\n\r\n"

	pista, ok := IdentificarTCP([]byte(peticion), 80)
	if !ok {
		t.Fatal("no se reconocio la peticion HTTP")
	}
	// El puerto NO es parte del nombre: separarlo partiria el consumo del mismo
	// destino en dos renglones.
	if pista.Nombre != "descargas.empresa.com.mx" || pista.Como != "http" {
		t.Fatalf("se leyo mal: %+v", pista)
	}
}

func TestLaConsultaDeDNSDelataInclusoLoQueVaPorQUIC(t *testing.T) {
	// Vale mas de lo que parece: el equipo pregunta el nombre ANTES de
	// conectarse, y esa consulta va en claro aunque la conexion posterior use
	// QUIC, que no tiene saludo legible.
	pista, ok := IdentificarUDP(consultaDNS("videollamada.ejemplo.org"), 53)
	if !ok || pista.Nombre != "videollamada.ejemplo.org" || pista.Como != "dns" {
		t.Fatalf("se leyo mal la consulta DNS: %+v", pista)
	}
}

func TestLaRespuestaDeDNSNoSeCuentaDosVeces(t *testing.T) {
	respuesta := consultaDNS("ejemplo.org")
	respuesta[2] |= 0x80 // bit de respuesta

	if _, ok := IdentificarUDP(respuesta, 53); ok {
		t.Fatal("solo deberian contarse las consultas, no las respuestas")
	}
}

func TestElPuertoEsElUltimoRecurso(t *testing.T) {
	// Cuando nada se delata, el puerto dice el TIPO de servicio. No dice con
	// quien se habla, y por eso se marca distinto.
	pista, ok := IdentificarTCP([]byte("datos cifrados sin sentido aparente"), 3389)
	if !ok || pista.Nombre != "Escritorio remoto" || pista.Como != "puerto" {
		t.Fatalf("se esperaba el nombre del servicio por puerto: %+v", pista)
	}

	// Y un puerto que no dice nada no inventa nada.
	if _, ok := IdentificarTCP([]byte("datos cifrados"), 44321); ok {
		t.Fatal("un puerto desconocido no deberia identificar nada")
	}
}

func TestUnPaqueteRotoNoSeInventaUnNombre(t *testing.T) {
	// Cualquier cosa puede pasar por el cable. Lo que no se entiende se descarta;
	// inventar un nombre seria meter basura en el informe.
	casos := [][]byte{
		{},
		{0x16},
		{0x16, 0x03, 0x01, 0xFF, 0xFF, 0x01}, // dice medir mas de lo que trae
		append([]byte{0x16, 0x03, 0x01, 0, 0, 0x01}, make([]byte, 200)...),
		[]byte("GET / HTTP/1.1\r\n\r\n"), // sin cabecera Host
	}
	for i, datos := range casos {
		if pista, ok := IdentificarTCP(datos, 9999); ok {
			t.Errorf("caso %d: no deberia haber identificado nada y dio %+v", i, pista)
		}
	}
}

func TestUnNombreConBasuraSeDescarta(t *testing.T) {
	// Un nombre con caracteres de control seria de un paquete mal leido, no de un
	// servidor de verdad.
	pista, _ := IdentificarTCP(saludoTLS("mal\x00nombre"), 443)
	if pista.Nombre != "" {
		t.Fatalf("un nombre con basura deberia descartarse y dio %q", pista.Nombre)
	}
}

func TestElDominioSeRecortaAAlgoQueUnaPersonaLea(t *testing.T) {
	// Sin este recorte, un solo servicio aparece como cincuenta destinos
	// distintos y el informe deja de servir.
	casos := map[string]string{
		"video-edge-a1b2.ftl.iad03.hls.ttvnw.net": "ttvnw.net",
		"www.netflix.com":                         "netflix.com",
		"netflix.com":                             "netflix.com",
		"algo":                                    "algo",
		// Los dominios de dos niveles necesitan una etiqueta mas, o todo el pais
		// quedaria agrupado bajo "com.mx".
		"descargas.empresa.com.mx": "empresa.com.mx",
		"www.gob.mx":               "www.gob.mx",
	}
	for entrada, esperado := range casos {
		if obtenido := DominioPrincipal(entrada); obtenido != esperado {
			t.Errorf("de %q se esperaba %q y salio %q", entrada, esperado, obtenido)
		}
	}
}

// ------------------------------------------------ la captura, sin capturar ---

// tramaConTLS arma una trama Ethernet + IPv4 + TCP con un saludo de TLS dentro.
func tramaConTLS(origen, destino, nombre string, etiquetasVLAN int) []byte {
	carga := saludoTLS(nombre)

	tcp := make([]byte, 20)
	binary.BigEndian.PutUint16(tcp[0:2], 51234)
	binary.BigEndian.PutUint16(tcp[2:4], 443)
	tcp[12] = 5 << 4 // cabecera de 20 bytes
	tcp = append(tcp, carga...)

	ip := make([]byte, 20)
	ip[0] = 0x45
	ip[9] = 6 // TCP
	desde, _ := netip.ParseAddr(origen)
	hasta, _ := netip.ParseAddr(destino)
	copy(ip[12:16], desde.AsSlice())
	copy(ip[16:20], hasta.AsSlice())
	binary.BigEndian.PutUint16(ip[2:4], uint16(len(ip)+len(tcp)))
	ip = append(ip, tcp...)

	trama := []byte{
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55,
		0x66, 0x77, 0x88, 0x99, 0xAA, 0xBB,
	}
	for i := 0; i < etiquetasVLAN; i++ {
		trama = append(trama, 0x81, 0x00, 0x00, 0x0A)
	}
	trama = append(trama, 0x08, 0x00)
	return append(trama, ip...)
}

func capturadorDePrueba(locales string) *Capturador {
	prefijo := netip.MustParsePrefix(locales)
	return &Capturador{
		EsLocal: func(direccion netip.Addr) bool { return prefijo.Contains(direccion) },
	}
}

func TestElConsumoSeLeApuntaAlEquipoDeLaRed(t *testing.T) {
	capturador := capturadorDePrueba("192.168.1.0/24")

	// Sale de la red hacia internet.
	capturador.Procesar(tramaConTLS("192.168.1.50", "8.8.8.8", "www.netflix.com", 0))
	// Y viene de internet hacia la red: el ancho de banda sigue siendo del mismo.
	capturador.Procesar(tramaConTLS("1.1.1.1", "192.168.1.50", "www.netflix.com", 0))

	consumos := capturador.Cortar()
	if len(consumos) != 1 {
		t.Fatalf("las dos direcciones son el mismo consumo: %+v", consumos)
	}
	if consumos[0].IP != "192.168.1.50" {
		t.Fatalf("se le apunto al equipo equivocado: %+v", consumos[0])
	}
	if consumos[0].Aplicacion != "netflix.com" {
		t.Fatalf("se esperaba el dominio recortado: %+v", consumos[0])
	}
	if consumos[0].Paquetes != 2 {
		t.Fatalf("se esperaban dos paquetes: %+v", consumos[0])
	}
}

func TestElTraficoDeAfueraAAfueraNoEsDeNadie(t *testing.T) {
	capturador := capturadorDePrueba("192.168.1.0/24")
	capturador.Procesar(tramaConTLS("8.8.8.8", "1.1.1.1", "algo.com", 0))

	if consumos := capturador.Cortar(); len(consumos) != 0 {
		t.Fatalf("no es consumo de nadie de esta red: %+v", consumos)
	}
}

func TestUnPuertoEspejoConVLANSeLeeIgual(t *testing.T) {
	// Un puerto espejo suele entregar las tramas con su etiqueta de VLAN puesta.
	// Sin saltarla, TODO el trafico de una red con VLAN desapareceria del
	// informe sin dar un solo error.
	for _, etiquetas := range []int{0, 1, 2} {
		capturador := capturadorDePrueba("192.168.1.0/24")
		capturador.Procesar(tramaConTLS("192.168.1.60", "8.8.8.8", "ejemplo.com", etiquetas))

		consumos := capturador.Cortar()
		if len(consumos) != 1 || consumos[0].Aplicacion != "ejemplo.com" {
			t.Fatalf("con %d etiquetas de VLAN no se leyo: %+v", etiquetas, consumos)
		}
	}
}

func TestLoQueNoSeIdentificaSigueSumandoBytes(t *testing.T) {
	// Un paquete que no se delata NO se tira: sus bytes pasaron por el cable de
	// verdad. Tirarlos haria que el informe sumara menos que el consumo real y
	// nadie sabria por que.
	capturador := capturadorDePrueba("192.168.1.0/24")

	trama := tramaConTLS("192.168.1.70", "8.8.8.8", "ejemplo.com", 0)
	// Se rompe el saludo dejando la trama por lo demas valida.
	trama[34+20] = 0x17 // ya no es un handshake

	capturador.Procesar(trama)
	consumos := capturador.Cortar()
	if len(consumos) != 1 {
		t.Fatalf("los bytes tienen que aparecer en algun lado: %+v", consumos)
	}
	if consumos[0].Aplicacion != "Sin identificar" {
		t.Fatalf("se esperaba que quedara sin identificar: %+v", consumos[0])
	}
	if consumos[0].Bytes == 0 {
		t.Fatal("los bytes deberian contarse aunque no se sepa de que son")
	}
}

func TestElInformeVaDeMayorAMenor(t *testing.T) {
	// Lo que interesa de este informe son los primeros renglones.
	capturador := capturadorDePrueba("192.168.1.0/24")
	capturador.Procesar(tramaConTLS("192.168.1.50", "8.8.8.8", "poco.com", 0))
	for i := 0; i < 5; i++ {
		capturador.Procesar(tramaConTLS("192.168.1.51", "8.8.8.8", "mucho.com", 0))
	}

	consumos := capturador.Cortar()
	if len(consumos) != 2 || consumos[0].Aplicacion != "mucho.com" {
		t.Fatalf("el que mas gasta va primero: %+v", consumos)
	}
}

func TestCortarLimpiaLaCuenta(t *testing.T) {
	capturador := capturadorDePrueba("192.168.1.0/24")
	capturador.Procesar(tramaConTLS("192.168.1.50", "8.8.8.8", "ejemplo.com", 0))

	if len(capturador.Cortar()) != 1 {
		t.Fatal("el primer corte deberia traer algo")
	}
	if len(capturador.Cortar()) != 0 {
		t.Fatal("el segundo corte deberia venir vacio")
	}
	// Pero la cuenta de paquetes vistos NO se reinicia: es lo que dice si el
	// puerto espejo esta bien configurado.
	if capturador.Paquetes() != 1 {
		t.Fatalf("los paquetes vistos no deberian reiniciarse: %d", capturador.Paquetes())
	}
}
