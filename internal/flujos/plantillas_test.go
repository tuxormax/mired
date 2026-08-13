package flujos

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

// Estas pruebas arman datagramas de NetFlow v9 y de IPFIX byte por byte, porque
// no hay forma de tener un router de verdad dentro de una prueba.
//
// Lo que se comprueba no es que "no reviente": es que los registros se lean
// **con la plantilla que les toca**. Leerlos con otra no da error, da cifras
// equivocadas atribuidas al equipo equivocado, que es el peor fallo posible en
// una herramienta de medicion.

// plantillaV9 arma un conjunto de plantillas de NetFlow v9.
func plantillaV9(id uint16, campos []campoDePlantilla) []byte {
	cuerpo := make([]byte, 4+len(campos)*4)
	binary.BigEndian.PutUint16(cuerpo[0:2], id)
	binary.BigEndian.PutUint16(cuerpo[2:4], uint16(len(campos)))
	for i, campo := range campos {
		binary.BigEndian.PutUint16(cuerpo[4+i*4:6+i*4], campo.tipo)
		binary.BigEndian.PutUint16(cuerpo[6+i*4:8+i*4], campo.largo)
	}
	return conjunto(conjuntoPlantillasV9, cuerpo)
}

func conjunto(id uint16, cuerpo []byte) []byte {
	salida := make([]byte, 4+len(cuerpo))
	binary.BigEndian.PutUint16(salida[0:2], id)
	binary.BigEndian.PutUint16(salida[2:4], uint16(4+len(cuerpo)))
	copy(salida[4:], cuerpo)
	return salida
}

func datagramaV9(dominio uint32, conjuntos ...[]byte) []byte {
	datos := make([]byte, tamanoCabeceraV9)
	binary.BigEndian.PutUint16(datos[0:2], versionV9)
	binary.BigEndian.PutUint16(datos[2:4], uint16(len(conjuntos)))
	binary.BigEndian.PutUint32(datos[16:20], dominio)
	for _, trozo := range conjuntos {
		datos = append(datos, trozo...)
	}
	return datos
}

func datagramaIPFIX(dominio uint32, conjuntos ...[]byte) []byte {
	datos := make([]byte, tamanoCabeceraIPFIX)
	binary.BigEndian.PutUint16(datos[0:2], versionIPFIX)
	binary.BigEndian.PutUint32(datos[12:16], dominio)
	for _, trozo := range conjuntos {
		datos = append(datos, trozo...)
	}
	binary.BigEndian.PutUint16(datos[2:4], uint16(len(datos)))
	return datos
}

// registro arma los bytes de un flujo con origen, destino y octetos de 4 bytes.
func registro(origen, destino string, octetos uint32) []byte {
	datos := make([]byte, 12)
	desde, _ := netip.ParseAddr(origen)
	hasta, _ := netip.ParseAddr(destino)
	copy(datos[0:4], desde.AsSlice())
	copy(datos[4:8], hasta.AsSlice())
	binary.BigEndian.PutUint32(datos[8:12], octetos)
	return datos
}

var camposBasicos = []campoDePlantilla{
	{tipo: campoOrigenIPv4, largo: 4},
	{tipo: campoDestinoIPv4, largo: 4},
	{tipo: campoOctetos, largo: 4},
}

func TestNetFlowV9NecesitaSuPlantillaAntesDeEntenderNada(t *testing.T) {
	receptor := receptorDePrueba("192.168.1.0/24")

	// Al arrancar, el router manda datos antes que plantillas. Eso NO es un
	// error: es lo normal, y esos registros se tiran callados.
	receptor.Procesar(datagramaV9(1,
		conjunto(256, registro("192.168.1.50", "8.8.8.8", 5000))))
	if consumos := receptor.Cortar(); len(consumos) != 0 {
		t.Fatalf("sin plantilla no se puede interpretar nada: %+v", consumos)
	}

	// Con la plantilla ya cargada, el mismo registro si se entiende.
	receptor.Procesar(datagramaV9(1, plantillaV9(256, camposBasicos)))
	receptor.Procesar(datagramaV9(1,
		conjunto(256, registro("192.168.1.50", "8.8.8.8", 5000))))

	consumos := receptor.Cortar()
	if len(consumos) != 1 || consumos[0].BytesSube != 5000 {
		t.Fatalf("con plantilla deberia contarse: %+v", consumos)
	}
	if consumos[0].Estimado {
		t.Fatal("NetFlow cuenta de verdad; no deberia marcarse como estimado")
	}
}

func TestLasPlantillasNoSeMezclanEntreExportadores(t *testing.T) {
	// Dos routers pueden usar el numero de plantilla 256 para cosas distintas.
	// Si se mezclaran, el segundo leeria sus bytes con la forma del primero y
	// las cifras saldrian mal SIN dar ningun error. Por eso la clave lleva de
	// quien llego.
	receptor := receptorDePrueba("192.168.1.0/24")

	receptor.ProcesarDe("10.0.0.1", datagramaV9(1, plantillaV9(256, camposBasicos)))

	// El segundo router manda datos con la plantilla 256, que el nunca declaro.
	receptor.ProcesarDe("10.0.0.2", datagramaV9(1,
		conjunto(256, registro("192.168.1.50", "8.8.8.8", 5000))))
	if consumos := receptor.Cortar(); len(consumos) != 0 {
		t.Fatalf("la plantilla del otro router no deberia servir: %+v", consumos)
	}

	// Y al primero si le sirve la suya.
	receptor.ProcesarDe("10.0.0.1", datagramaV9(1,
		conjunto(256, registro("192.168.1.50", "8.8.8.8", 5000))))
	if consumos := receptor.Cortar(); len(consumos) != 1 {
		t.Fatalf("al que la declaro si deberia servirle: %+v", consumos)
	}
}

func TestVariosRegistrosEnUnConjuntoSeLeenTodos(t *testing.T) {
	receptor := receptorDePrueba("192.168.1.0/24")
	receptor.Procesar(datagramaV9(1, plantillaV9(256, camposBasicos)))

	var cuerpo []byte
	cuerpo = append(cuerpo, registro("192.168.1.50", "8.8.8.8", 1000)...)
	cuerpo = append(cuerpo, registro("192.168.1.51", "8.8.8.8", 2000)...)
	cuerpo = append(cuerpo, registro("1.1.1.1", "192.168.1.50", 3000)...)
	receptor.Procesar(datagramaV9(1, conjunto(256, cuerpo)))

	consumos := receptor.Cortar()
	if len(consumos) != 2 {
		t.Fatalf("se esperaban dos direcciones: %+v", consumos)
	}
	for _, consumo := range consumos {
		switch consumo.IP {
		case "192.168.1.50":
			if consumo.BytesSube != 1000 || consumo.BytesBaja != 3000 {
				t.Errorf(".50 mal contado: %+v", consumo)
			}
		case "192.168.1.51":
			if consumo.BytesSube != 2000 {
				t.Errorf(".51 mal contado: %+v", consumo)
			}
		}
	}
}

func TestElContadorSeLeeDelTamanoQueElRouterHayaElegido(t *testing.T) {
	// El mismo campo llega de 4 bytes en un router y de 8 en otro. Suponer el
	// tamano es como se leen cifras absurdas.
	receptor := receptorDePrueba("192.168.1.0/24")
	receptor.Procesar(datagramaV9(1, plantillaV9(300, []campoDePlantilla{
		{tipo: campoOrigenIPv4, largo: 4},
		{tipo: campoDestinoIPv4, largo: 4},
		{tipo: campoOctetos, largo: 8},
	})))

	cuerpo := make([]byte, 16)
	origen, _ := netip.ParseAddr("192.168.1.50")
	destino, _ := netip.ParseAddr("8.8.8.8")
	copy(cuerpo[0:4], origen.AsSlice())
	copy(cuerpo[4:8], destino.AsSlice())
	binary.BigEndian.PutUint64(cuerpo[8:16], 12_000_000_000)
	receptor.Procesar(datagramaV9(1, conjunto(300, cuerpo)))

	consumos := receptor.Cortar()
	if len(consumos) != 1 || consumos[0].BytesSube != 12_000_000_000 {
		t.Fatalf("un contador de 8 bytes deberia leerse entero: %+v", consumos)
	}
}

func TestIpfixSeLeeIgualQueV9PeroConSuSobre(t *testing.T) {
	receptor := receptorDePrueba("192.168.1.0/24")

	plantilla := make([]byte, 4+len(camposBasicos)*4)
	binary.BigEndian.PutUint16(plantilla[0:2], 256)
	binary.BigEndian.PutUint16(plantilla[2:4], uint16(len(camposBasicos)))
	for i, campo := range camposBasicos {
		binary.BigEndian.PutUint16(plantilla[4+i*4:6+i*4], campo.tipo)
		binary.BigEndian.PutUint16(plantilla[6+i*4:8+i*4], campo.largo)
	}

	receptor.Procesar(datagramaIPFIX(7,
		conjunto(conjuntoPlantillasIPFIX, plantilla),
		conjunto(256, registro("192.168.1.77", "8.8.4.4", 8000))))

	consumos := receptor.Cortar()
	if len(consumos) != 1 || consumos[0].IP != "192.168.1.77" || consumos[0].BytesSube != 8000 {
		t.Fatalf("IPFIX mal leido: %+v", consumos)
	}
}

func TestUnCampoDeFabricanteNoDescolocaLosQueSiguen(t *testing.T) {
	// IPFIX deja meter campos propios de cada fabricante, y esos traen CUATRO
	// bytes mas en la plantilla. Si no se saltan, todos los campos que vienen
	// despues se leen corridos y el flujo sale con direcciones inventadas.
	receptor := receptorDePrueba("192.168.1.0/24")

	campos := []struct {
		tipo    uint16
		largo   uint16
		empresa bool
	}{
		{tipo: campoOrigenIPv4, largo: 4},
		{tipo: 0x8001, largo: 4, empresa: true}, // campo propio del fabricante
		{tipo: campoDestinoIPv4, largo: 4},
		{tipo: campoOctetos, largo: 4},
	}

	plantilla := []byte{0, 0, 0, 0}
	binary.BigEndian.PutUint16(plantilla[0:2], 256)
	binary.BigEndian.PutUint16(plantilla[2:4], uint16(len(campos)))
	for _, campo := range campos {
		trozo := make([]byte, 4)
		binary.BigEndian.PutUint16(trozo[0:2], campo.tipo)
		binary.BigEndian.PutUint16(trozo[2:4], campo.largo)
		plantilla = append(plantilla, trozo...)
		if campo.empresa {
			plantilla = append(plantilla, 0, 0, 0, 9) // numero de empresa
		}
	}

	cuerpo := make([]byte, 16)
	origen, _ := netip.ParseAddr("192.168.1.90")
	destino, _ := netip.ParseAddr("8.8.8.8")
	copy(cuerpo[0:4], origen.AsSlice())
	// 4..8 es el campo del fabricante, que se lee y se descarta
	copy(cuerpo[8:12], destino.AsSlice())
	binary.BigEndian.PutUint32(cuerpo[12:16], 4500)

	receptor.Procesar(datagramaIPFIX(1,
		conjunto(conjuntoPlantillasIPFIX, plantilla),
		conjunto(256, cuerpo)))

	consumos := receptor.Cortar()
	if len(consumos) != 1 || consumos[0].IP != "192.168.1.90" || consumos[0].BytesSube != 4500 {
		t.Fatalf("el campo del fabricante descoloco la lectura: %+v", consumos)
	}
}

func TestUnDatagramaMalArmadoNoCuelgaNiRevienta(t *testing.T) {
	// Cualquiera puede mandar un datagrama a ese puerto. Un largo imposible no
	// debe dejar al receptor dando vueltas ni tumbarlo.
	receptor := receptorDePrueba("192.168.1.0/24")

	casos := [][]byte{
		{0, 9},                                 // version y nada mas
		datagramaV9(1, []byte{1, 0, 0, 0}),     // conjunto con largo cero
		datagramaV9(1, []byte{1, 0, 255, 255}), // largo mayor que el datagrama
		datagramaIPFIX(1, []byte{0, 2, 0, 0}),  // conjunto de plantillas vacio
		make([]byte, 8),                        // puros ceros
		{0, 0, 0, 5},                           // sFlow truncado
	}

	for _, datos := range casos {
		receptor.Procesar(datos) // no debe colgarse ni entrar en panico
	}
	if consumos := receptor.Cortar(); len(consumos) != 0 {
		t.Fatalf("de basura no deberia salir ningun consumo: %+v", consumos)
	}
}
