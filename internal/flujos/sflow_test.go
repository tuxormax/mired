package flujos

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

// Pruebas de sFlow, que es el formato que funciona distinto a los otros tres:
// no cuenta el trafico, lo muestrea, y manda paquetes crudos en vez de campos.
//
// Lo que hay que comprobar aqui es justo eso: que se desarme bien un paquete de
// verdad —con y sin VLAN— y que la multiplicacion por la tasa de muestreo salga
// exacta, porque de ella depende toda la cifra.

func palabra(valor uint32) []byte {
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes, valor)
	return bytes
}

// tramaEthernet arma una trama de verdad: dos MAC, tipo y cabecera IPv4.
func tramaEthernet(origen, destino string, etiquetasVLAN int) []byte {
	trama := []byte{
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, // MAC destino
		0x66, 0x77, 0x88, 0x99, 0xAA, 0xBB, // MAC origen
	}
	for i := 0; i < etiquetasVLAN; i++ {
		trama = append(trama, 0x81, 0x00, 0x00, 0x0A) // etiqueta de VLAN 10
	}
	trama = append(trama, 0x08, 0x00) // tipo: IPv4

	ip := make([]byte, 20)
	ip[0] = 0x45 // version 4, cabecera de 20 bytes
	desde, _ := netip.ParseAddr(origen)
	hasta, _ := netip.ParseAddr(destino)
	copy(ip[12:16], desde.AsSlice())
	copy(ip[16:20], hasta.AsSlice())
	return append(trama, ip...)
}

// datagramaSFlow arma un datagrama con una sola muestra de flujo.
func datagramaSFlow(tasa uint32, tamanoDeTrama uint32, trama []byte) []byte {
	registro := append([]byte{}, palabra(1)...)            // protocolo: Ethernet
	registro = append(registro, palabra(tamanoDeTrama)...) // tamano real de la trama
	registro = append(registro, palabra(0)...)             // bytes quitados del final
	registro = append(registro, palabra(uint32(len(trama)))...)
	registro = append(registro, trama...)
	for len(registro)%4 != 0 { // el formato exige alinear a cuatro
		registro = append(registro, 0)
	}

	muestra := append([]byte{}, palabra(1)...)  // secuencia
	muestra = append(muestra, palabra(0)...)    // fuente
	muestra = append(muestra, palabra(tasa)...) // tasa de muestreo
	muestra = append(muestra, palabra(0)...)    // total muestreado
	muestra = append(muestra, palabra(0)...)    // descartados
	muestra = append(muestra, palabra(5)...)    // interfaz de entrada
	muestra = append(muestra, palabra(7)...)    // interfaz de salida
	muestra = append(muestra, palabra(1)...)    // un registro
	muestra = append(muestra, palabra(1)...)    // tipo: paquete crudo
	muestra = append(muestra, palabra(uint32(len(registro)))...)
	muestra = append(muestra, registro...)

	datos := append([]byte{}, palabra(5)...) // version de sFlow
	datos = append(datos, palabra(1)...)     // direccion del agente: IPv4
	datos = append(datos, 10, 0, 0, 1)       // el agente
	datos = append(datos, palabra(0)...)     // subagente
	datos = append(datos, palabra(1)...)     // secuencia
	datos = append(datos, palabra(1000)...)  // tiempo encendido
	datos = append(datos, palabra(1)...)     // una muestra
	datos = append(datos, palabra(1)...)     // tipo: muestra de flujo
	datos = append(datos, palabra(uint32(len(muestra)))...)
	return append(datos, muestra...)
}

func TestSflowMultiplicaPorLaTasaDeMuestreo(t *testing.T) {
	// El switch vio UN paquete de 1 500 bytes y muestrea 1 de cada 2 000: la
	// estimacion son 3 MB. Equivocarse en esta multiplicacion es equivocarse en
	// toda la cifra, y no hay forma de notarlo mirando la pantalla.
	receptor := receptorDePrueba("192.168.1.0/24")
	receptor.Procesar(datagramaSFlow(2000, 1500,
		tramaEthernet("192.168.1.50", "8.8.8.8", 0)))

	consumos := receptor.Cortar()
	if len(consumos) != 1 {
		t.Fatalf("se esperaba un consumo: %+v", consumos)
	}
	if consumos[0].BytesSube != 1500*2000 {
		t.Fatalf("se esperaban %d bytes y salieron %d", 1500*2000, consumos[0].BytesSube)
	}
	// Y va marcado como estimacion, que es la mitad del valor del dato.
	if !consumos[0].Estimado {
		t.Fatal("lo que sale de un muestreo tiene que ir marcado como estimado")
	}
}

func TestSflowSaltaLasEtiquetasDeVLAN(t *testing.T) {
	// Un switch con VLAN manda la trama con su etiqueta puesta. Sin saltarla, el
	// tipo de protocolo se lee cuatro bytes antes de donde esta y el paquete
	// parece basura: el trafico de una red con VLAN desapareceria entero.
	for _, etiquetas := range []int{0, 1, 2} {
		receptor := receptorDePrueba("192.168.1.0/24")
		receptor.Procesar(datagramaSFlow(1, 1000,
			tramaEthernet("192.168.1.60", "1.1.1.1", etiquetas)))

		consumos := receptor.Cortar()
		if len(consumos) != 1 || consumos[0].IP != "192.168.1.60" {
			t.Fatalf("con %d etiquetas de VLAN no se leyo el paquete: %+v", etiquetas, consumos)
		}
	}
}

func TestSflowNoSeConfundeConNetFlowV5(t *testing.T) {
	// Los dos dicen "version 5". NetFlow la pone en dos bytes y sFlow en cuatro,
	// asi que un datagrama de sFlow leido como NetFlow daria version 0. Es la
	// unica forma de distinguirlos, y tiene que funcionar en los dos sentidos.
	receptor := receptorDePrueba("192.168.1.0/24")

	receptor.Procesar(datagramaV5([][2]string{{"192.168.1.50", "8.8.8.8"}}, []uint32{7000}))
	unoV5 := receptor.Cortar()
	if len(unoV5) != 1 || unoV5[0].BytesSube != 7000 || unoV5[0].Estimado {
		t.Fatalf("NetFlow v5 mal leido: %+v", unoV5)
	}

	receptor.Procesar(datagramaSFlow(10, 700, tramaEthernet("192.168.1.50", "8.8.8.8", 0)))
	unoSFlow := receptor.Cortar()
	if len(unoSFlow) != 1 || unoSFlow[0].BytesSube != 7000 || !unoSFlow[0].Estimado {
		t.Fatalf("sFlow mal leido: %+v", unoSFlow)
	}
}

func TestSflowIgnoraLoQueNoEsTraficoDeAlguien(t *testing.T) {
	// ARP y spanning tree pasan por el cable pero no son consumo de nadie.
	receptor := receptorDePrueba("192.168.1.0/24")

	arp := []byte{
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0x66, 0x77, 0x88, 0x99, 0xAA, 0xBB,
		0x08, 0x06, // tipo: ARP
	}
	arp = append(arp, make([]byte, 28)...)
	receptor.Procesar(datagramaSFlow(100, 60, arp))

	if consumos := receptor.Cortar(); len(consumos) != 0 {
		t.Fatalf("ARP no es consumo de nadie: %+v", consumos)
	}
}

func TestUnaTasaAbsurdaNoInventaGigabytes(t *testing.T) {
	// Una tasa de cero o disparatada convertiria un paquete suelto en gigabytes
	// que nadie consumio. Ante la duda se cuenta el paquete tal cual.
	for _, tasa := range []uint32{0, 50_000_000} {
		receptor := receptorDePrueba("192.168.1.0/24")
		receptor.Procesar(datagramaSFlow(tasa, 1200,
			tramaEthernet("192.168.1.50", "8.8.8.8", 0)))

		consumos := receptor.Cortar()
		if len(consumos) != 1 || consumos[0].BytesSube != 1200 {
			t.Fatalf("con tasa %d se esperaban 1200 bytes: %+v", tasa, consumos)
		}
	}
}
