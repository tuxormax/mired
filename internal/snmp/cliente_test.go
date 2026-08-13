package snmp

import (
	"testing"

	"github.com/gosnmp/gosnmp"
)

// Estas pruebas cubren la parte mas delicada de todo el modulo SNMP: interpretar
// lo que contesta el equipo. Es donde se equivoca uno leyendo la MIB, y donde un
// error no se ve como un fallo sino como datos silenciosamente incorrectos.

func TestLaMacVieneEscondidaEnElIndiceDelOID(t *testing.T) {
	// La tabla de reenvio NO guarda la MAC como valor: la mete en el indice de
	// la fila, en decimal, byte por byte. Es lo mas facil de leer mal de toda la
	// MIB del puente.
	casos := []struct {
		nombre   string
		sufijo   string
		esperado string
	}{
		{
			nombre:   "seis numeros son la MAC directa",
			sufijo:   "184.39.235.26.43.60",
			esperado: "b8:27:eb:1a:2b:3c",
		},
		{
			nombre:   "con VLAN vienen siete y el primero se descarta",
			sufijo:   "10.184.39.235.26.43.60",
			esperado: "b8:27:eb:1a:2b:3c",
		},
		{
			nombre:   "ceros a la izquierda se conservan",
			sufijo:   "0.0.0.1.2.3",
			esperado: "00:00:00:01:02:03",
		},
		{nombre: "un numero fuera de rango invalida todo", sufijo: "184.39.999.26.43.60"},
		{nombre: "faltan bytes", sufijo: "184.39.235"},
		{nombre: "vacio", sufijo: ""},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			obtenido := macDesdeOID(caso.sufijo)
			if obtenido != caso.esperado {
				t.Fatalf("de %q se obtuvo %q y se esperaba %q", caso.sufijo, obtenido, caso.esperado)
			}
		})
	}
}

func TestLaMacDeUnaInterfazLlegaComoBytesCrudos(t *testing.T) {
	dato := gosnmp.SnmpPDU{Value: []byte{0xB8, 0x27, 0xEB, 0x1A, 0x2B, 0x3C}}
	if obtenido := comoMAC(dato); obtenido != "b8:27:eb:1a:2b:3c" {
		t.Fatalf("se obtuvo %q", obtenido)
	}

	// Muchos equipos devuelven una MAC vacia en las interfaces logicas. Eso no
	// es un error: es que esa boca no tiene direccion fisica.
	if obtenido := comoMAC(gosnmp.SnmpPDU{Value: []byte{}}); obtenido != "" {
		t.Fatalf("una MAC vacia deberia quedar vacia y dio %q", obtenido)
	}
	if obtenido := comoMAC(gosnmp.SnmpPDU{Value: "no soy bytes"}); obtenido != "" {
		t.Fatalf("un valor que no son bytes deberia quedar vacio y dio %q", obtenido)
	}
}

func TestLosContadoresGrandesNoSeVuelvenNegativos(t *testing.T) {
	// Un puerto gigabit pasa de los 2 000 millones en minutos. Si el valor se
	// leyera con signo, el consumo saldria negativo y la grafica no tendria
	// sentido.
	casos := []struct {
		valor    any
		esperado uint64
	}{
		{valor: uint64(18446744073709551000), esperado: 18446744073709551000},
		{valor: uint32(4294967295), esperado: 4294967295},
		{valor: uint(3000000000), esperado: 3000000000},
		{valor: 12345, esperado: 12345},
		{valor: int64(-5), esperado: 0}, // un negativo no significa nada aqui
		{valor: "texto", esperado: 0},
		{valor: nil, esperado: 0},
	}

	for _, caso := range casos {
		if obtenido := comoSinSigno(gosnmp.SnmpPDU{Value: caso.valor}); obtenido != caso.esperado {
			t.Errorf("de %v se obtuvo %d y se esperaba %d", caso.valor, obtenido, caso.esperado)
		}
	}
}

func TestElTextoLlegaDeVariasFormas(t *testing.T) {
	// Segun el fabricante, una cadena llega como string o como bytes, y a veces
	// con espacios o saltos de linea pegados.
	casos := []struct {
		valor    any
		esperado string
	}{
		{valor: "sw-principal", esperado: "sw-principal"},
		{valor: []byte("  sw-bodega \n"), esperado: "sw-bodega"},
		{valor: nil, esperado: ""},
		{valor: 42, esperado: "42"},
	}

	for _, caso := range casos {
		if obtenido := comoTexto(gosnmp.SnmpPDU{Value: caso.valor}); obtenido != caso.esperado {
			t.Errorf("de %v se obtuvo %q y se esperaba %q", caso.valor, obtenido, caso.esperado)
		}
	}
}

func TestElNivelDeSeguridadSaleDeLoQueSeCapturo(t *testing.T) {
	// Pedirle al usuario que elija el nivel de seguridad de SNMPv3 seria pedirle
	// que sepa algo que ya se deduce de lo que lleno.
	sinNada := Credencial{Version: "v3", Usuario: "monitor"}
	if nivel := nivelSeguridad(sinNada); nivel != gosnmp.NoAuthNoPriv {
		t.Fatalf("sin claves no deberia haber ni autenticacion ni cifrado: %v", nivel)
	}

	soloAuth := Credencial{Version: "v3", Usuario: "monitor", AutenticacionClave: "x"}
	conCifrado := Credencial{Version: "v3", Usuario: "monitor",
		AutenticacionClave: "x", PrivacidadClave: "y"}

	if nivelSeguridad(soloAuth) == nivelSeguridad(conCifrado) {
		t.Fatal("con clave de privacidad el nivel tiene que ser distinto")
	}
}

func TestLosProtocolosDesconocidosNoRompen(t *testing.T) {
	// Un `.toml` o un formulario pueden traer cualquier cosa. Ante algo que no
	// se reconoce, se cae a "sin autenticacion" en vez de reventar.
	if protocoloAutenticacion("inventado") != gosnmp.NoAuth {
		t.Fatal("un protocolo desconocido deberia quedar en ninguno")
	}
	if protocoloPrivacidad("inventado") != gosnmp.NoPriv {
		t.Fatal("un protocolo de privacidad desconocido deberia quedar en ninguno")
	}
	// Y los nombres se aceptan sin importar como se escribieron.
	if protocoloAutenticacion("sha256") != protocoloAutenticacion("SHA256") {
		t.Fatal("el nombre del protocolo no deberia depender de mayusculas")
	}
}

func TestCadaProtocoloDeVecinosPoneLaBocaEnOtroLugarDelIndice(t *testing.T) {
	// Es el error mas facil de cometer de los dos protocolos, y el mas caro: no
	// revienta nada, solo cuelga cada vecino de la boca equivocada, y el mapa
	// sale plausible y falso.
	porIndice := map[int]string{5: "Gi0/5", 7: "Gi0/7"}

	// LLDP: tiempo . puertoLocal . numeroDeVecino  -> el de EN MEDIO.
	if boca, ok := bocaLocal("124578.5.1", 1, porIndice); !ok || boca != "Gi0/5" {
		t.Fatalf("LLDP deberia leer la boca 5 y dio %q (ok=%v)", boca, ok)
	}
	// CDP: ifIndexLocal . numeroDeVecino  -> el PRIMERO.
	if boca, ok := bocaLocal("7.2", 0, porIndice); !ok || boca != "Gi0/7" {
		t.Fatalf("CDP deberia leer la boca 7 y dio %q (ok=%v)", boca, ok)
	}

	// Una boca que el equipo no listo en su tabla de interfaces se queda con el
	// numero crudo: es peor perder el vecino que mostrarlo sin nombre bonito.
	if boca, ok := bocaLocal("99.1", 0, porIndice); !ok || boca != "99" {
		t.Fatalf("sin nombre deberia quedar el numero y dio %q (ok=%v)", boca, ok)
	}

	// Y lo que no se puede leer se descarta, no se inventa.
	if _, ok := bocaLocal("124578", 1, porIndice); ok {
		t.Fatal("un indice corto no deberia dar boca")
	}
	if _, ok := bocaLocal("no.es.numero", 0, porIndice); ok {
		t.Fatal("un indice que no es numero no deberia dar boca")
	}
}

func TestLaDireccionQueAnunciaCdpLlegaComoBytes(t *testing.T) {
	// CDP manda la IP del vecino en crudo. Es lo unico con lo que se le puede
	// poner cara a un enlace visto solo por CDP, porque CDP no manda el chasis.
	if obtenido := comoIP(gosnmp.SnmpPDU{Value: []byte{192, 168, 1, 2}}); obtenido != "192.168.1.2" {
		t.Fatalf("se obtuvo %q", obtenido)
	}

	casos := []any{
		[]byte{0, 0, 0, 0},  // sin configurar: no es una direccion
		[]byte{192, 168, 1}, // incompleta
		[]byte{},            // vacia
		"192.168.1.2",       // texto, no bytes
		nil,
	}
	for _, valor := range casos {
		if obtenido := comoIP(gosnmp.SnmpPDU{Value: valor}); obtenido != "" {
			t.Errorf("de %v se esperaba vacio y dio %q", valor, obtenido)
		}
	}
}
