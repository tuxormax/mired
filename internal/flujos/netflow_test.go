package flujos

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

// datagramaV5 arma un paquete NetFlow v5 con los flujos indicados, para poder
// probar el receptor sin un router de verdad.
func datagramaV5(flujos [][2]string, octetos []uint32) []byte {
	datos := make([]byte, tamanoCabecera+len(flujos)*tamanoRegistro)
	binary.BigEndian.PutUint16(datos[0:2], versionV5)
	binary.BigEndian.PutUint16(datos[2:4], uint16(len(flujos)))

	for i, flujo := range flujos {
		inicio := tamanoCabecera + i*tamanoRegistro
		origen, _ := netip.ParseAddr(flujo[0])
		destino, _ := netip.ParseAddr(flujo[1])
		copy(datos[inicio:inicio+4], origen.AsSlice())
		copy(datos[inicio+4:inicio+8], destino.AsSlice())
		binary.BigEndian.PutUint32(datos[inicio+20:inicio+24], octetos[i])
	}
	return datos
}

func receptorDePrueba(locales string) *Receptor {
	prefijo := netip.MustParsePrefix(locales)
	return &Receptor{
		EsLocal: func(direccion netip.Addr) bool { return prefijo.Contains(direccion) },
	}
}

func TestUnFlujoHaciaAfueraEsSubida(t *testing.T) {
	receptor := receptorDePrueba("192.168.1.0/24")

	// Un equipo de la red mandando 5 000 bytes a internet.
	receptor.Procesar(datagramaV5(
		[][2]string{{"192.168.1.50", "8.8.8.8"}}, []uint32{5000}))

	consumos := receptor.Cortar()
	if len(consumos) != 1 {
		t.Fatalf("se esperaba un consumo: %+v", consumos)
	}
	if consumos[0].IP != "192.168.1.50" {
		t.Fatalf("se conto la direccion equivocada: %s", consumos[0].IP)
	}
	if consumos[0].BytesSube != 5000 || consumos[0].BytesBaja != 0 {
		t.Fatalf("lo que sale de la red es subida: %+v", consumos[0])
	}
}

func TestUnFlujoDesdeAfueraEsBajada(t *testing.T) {
	receptor := receptorDePrueba("192.168.1.0/24")
	receptor.Procesar(datagramaV5(
		[][2]string{{"1.1.1.1", "192.168.1.50"}}, []uint32{9000}))

	consumos := receptor.Cortar()
	if len(consumos) != 1 || consumos[0].BytesBaja != 9000 || consumos[0].BytesSube != 0 {
		t.Fatalf("lo que entra a la red es bajada: %+v", consumos)
	}
}

func TestUnFlujoEntreDosLocalesCuentaEnLosDos(t *testing.T) {
	// Para el que manda es subida y para el que recibe es bajada: el mismo
	// trafico, visto desde cada lado.
	receptor := receptorDePrueba("192.168.1.0/24")
	receptor.Procesar(datagramaV5(
		[][2]string{{"192.168.1.10", "192.168.1.20"}}, []uint32{3000}))

	consumos := receptor.Cortar()
	if len(consumos) != 2 {
		t.Fatalf("se esperaban dos consumos: %+v", consumos)
	}
	for _, consumo := range consumos {
		switch consumo.IP {
		case "192.168.1.10":
			if consumo.BytesSube != 3000 {
				t.Fatalf("el que manda deberia tener subida: %+v", consumo)
			}
		case "192.168.1.20":
			if consumo.BytesBaja != 3000 {
				t.Fatalf("el que recibe deberia tener bajada: %+v", consumo)
			}
		default:
			t.Fatalf("direccion inesperada: %s", consumo.IP)
		}
	}
}

func TestElTraficoDeAfueraAAfueraNoSeCuenta(t *testing.T) {
	// Un router puede exportar flujos que no tocan la red vigilada. Contarlos
	// inflaria el consumo con trafico que no es de nadie de aqui.
	receptor := receptorDePrueba("192.168.1.0/24")
	receptor.Procesar(datagramaV5(
		[][2]string{{"8.8.8.8", "1.1.1.1"}}, []uint32{100000}))

	if consumos := receptor.Cortar(); len(consumos) != 0 {
		t.Fatalf("no deberia contarse nada: %+v", consumos)
	}
}

func TestSeSumanVariosFlujosDelMismoEquipo(t *testing.T) {
	receptor := receptorDePrueba("192.168.1.0/24")
	receptor.Procesar(datagramaV5([][2]string{
		{"192.168.1.50", "8.8.8.8"},
		{"192.168.1.50", "1.1.1.1"},
		{"9.9.9.9", "192.168.1.50"},
	}, []uint32{1000, 2000, 7000}))

	consumos := receptor.Cortar()
	if len(consumos) != 1 {
		t.Fatalf("se esperaba un solo equipo: %+v", consumos)
	}
	if consumos[0].BytesSube != 3000 || consumos[0].BytesBaja != 7000 {
		t.Fatalf("las sumas estan mal: %+v", consumos[0])
	}
	if consumos[0].Conversaciones != 3 {
		t.Fatalf("se esperaban 3 conversaciones: %+v", consumos[0])
	}
}

func TestSeIgnoraLoQueNoEsNetflowV5(t *testing.T) {
	// Un router mal configurado puede mandar v9 o basura. Debe ignorarse sin
	// romper nada y sin llenar la bitacora.
	receptor := receptorDePrueba("192.168.1.0/24")

	otraVersion := datagramaV5([][2]string{{"192.168.1.5", "8.8.8.8"}}, []uint32{1000})
	binary.BigEndian.PutUint16(otraVersion[0:2], 9)
	receptor.Procesar(otraVersion)
	receptor.Procesar([]byte{1, 2, 3})
	receptor.Procesar(nil)

	if consumos := receptor.Cortar(); len(consumos) != 0 {
		t.Fatalf("no deberia haber contado nada: %+v", consumos)
	}
}

func TestCortarLimpiaLaCuenta(t *testing.T) {
	// Los flujos son del periodo, no acumulados: si no se limpiara, cada corte
	// repetiria el trafico del corte anterior.
	receptor := receptorDePrueba("192.168.1.0/24")
	receptor.Procesar(datagramaV5(
		[][2]string{{"192.168.1.50", "8.8.8.8"}}, []uint32{5000}))

	if len(receptor.Cortar()) != 1 {
		t.Fatal("el primer corte deberia traer el consumo")
	}
	if consumos := receptor.Cortar(); len(consumos) != 0 {
		t.Fatalf("el segundo corte deberia venir vacio: %+v", consumos)
	}
}
