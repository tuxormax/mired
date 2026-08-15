package basedatos

import (
	"context"
	"testing"
	"time"
)

// sembrarSwitch deja un switch con un puerto lista para medirle trafico.
func sembrarSwitch(t *testing.T, base *Base) {
	t.Helper()
	ctx := context.Background()

	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.1", MAC: "aa:aa:aa:00:00:01", Metodo: "arp"},
		{IP: "192.168.1.10", MAC: "bb:bb:bb:00:00:10", Nombre: "laptop", Metodo: "arp"},
	})
	if _, err := base.GuardarSNMP(ctx, []FichaSNMP{{
		IP: "192.168.1.1", Nombre: "sw", EsSwitch: true,
		Interfaces:    []InterfazSNMP{{Indice: 5, Nombre: "Gi0/5", Activa: true}},
		MacsPorPuerto: map[string][]string{"5": {"bb:bb:bb:00:00:10"}},
	}}); err != nil {
		t.Fatalf("no se pudo sembrar el switch: %v", err)
	}
}

func TestLaPrimeraMuestraNoTieneTasa(t *testing.T) {
	// Con una sola lectura no hay nada que restar: inventar una tasa ahi seria
	// mostrar un numero que nadie midio.
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	sembrarSwitch(t, base)
	if err := base.GuardarTrafico(ctx, "192.168.1.1", []ContadorPuerto{
		{Indice: 5, Entrada: 1_000_000, Salida: 500_000, SesentaYCuatro: true},
	}); err != nil {
		t.Fatalf("no se pudo guardar: %v", err)
	}

	consumo, err := base.ConsumoActual(ctx)
	if err != nil {
		t.Fatalf("no se pudo leer el consumo: %v", err)
	}
	if len(consumo) != 0 {
		t.Fatalf("la primera muestra no deberia aparecer como consumo: %+v", consumo)
	}
}

func TestLaSegundaMuestraDaLaTasa(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	sembrarSwitch(t, base)

	// Se escribe la primera muestra con una marca de hace 10 segundos, porque la
	// tasa se calcula contra el tiempo transcurrido.
	hace10 := time.Now().Add(-10 * time.Second).Format(time.RFC3339)
	if _, err := base.ExecContext(ctx, `
		INSERT INTO muestras_trafico (switch_id, interfaz_indice, momento, bytes_entrada, bytes_salida)
		SELECT id, 5, ?, 1000000, 500000 FROM equipos WHERE ip = '192.168.1.1'`, hace10); err != nil {
		t.Fatalf("no se pudo sembrar la muestra: %v", err)
	}

	// 10 segundos despues han pasado 1 250 000 bytes mas de entrada: 1 Mbps.
	if err := base.GuardarTrafico(ctx, "192.168.1.1", []ContadorPuerto{
		{Indice: 5, Entrada: 1_000_000 + 1_250_000, Salida: 500_000, SesentaYCuatro: true},
	}); err != nil {
		t.Fatalf("no se pudo guardar: %v", err)
	}

	consumo, err := base.ConsumoActual(ctx)
	if err != nil {
		t.Fatalf("no se pudo leer el consumo: %v", err)
	}
	if len(consumo) != 1 {
		t.Fatalf("se esperaba un puerto con consumo: %+v", consumo)
	}
	// 1 250 000 bytes en 10 s = 1 000 000 bits/s. Se admite holgura por el
	// redondeo del reloj a segundos.
	if consumo[0].BpsEntrada < 900_000 || consumo[0].BpsEntrada > 1_100_000 {
		t.Fatalf("la tasa calculada esta mal: %d bps", consumo[0].BpsEntrada)
	}
	// Y dice de quien es el puerto, que es lo que vuelve util el numero.
	if consumo[0].EquipoNombre != "laptop" {
		t.Fatalf("el consumo deberia decir de quien es: %+v", consumo[0])
	}
}

func TestUnContadorQueSeReiniciaNoInventaUnPico(t *testing.T) {
	// Si el switch se reinicia, el contador vuelve a cero. Restar daria un
	// numero negativo, y tratarlo como positivo pintaria un pico enorme que
	// nunca ocurrio.
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	sembrarSwitch(t, base)
	hace10 := time.Now().Add(-10 * time.Second).Format(time.RFC3339)
	base.ExecContext(ctx, `
		INSERT INTO muestras_trafico (switch_id, interfaz_indice, momento, bytes_entrada, bytes_salida)
		SELECT id, 5, ?, 900000000, 900000000 FROM equipos WHERE ip = '192.168.1.1'`, hace10)

	if err := base.GuardarTrafico(ctx, "192.168.1.1", []ContadorPuerto{
		{Indice: 5, Entrada: 1000, Salida: 1000, SesentaYCuatro: true},
	}); err != nil {
		t.Fatalf("no se pudo guardar: %v", err)
	}

	consumo, _ := base.ConsumoActual(ctx)
	if len(consumo) != 0 {
		t.Fatalf("un contador reiniciado no deberia producir consumo: %+v", consumo)
	}
}

func TestPodarBorraLoViejoYConservaLoNuevo(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	sembrarSwitch(t, base)
	viejo := time.Now().AddDate(0, 0, -200).Format(time.RFC3339)
	base.ExecContext(ctx, `
		INSERT INTO muestras_trafico (switch_id, interfaz_indice, momento, bytes_entrada, bytes_salida)
		SELECT id, 5, ?, 1, 1 FROM equipos WHERE ip = '192.168.1.1'`, viejo)
	base.GuardarTrafico(ctx, "192.168.1.1", []ContadorPuerto{{Indice: 5, Entrada: 2, Salida: 2}})

	if err := base.PodarTrafico(ctx, 90); err != nil {
		t.Fatalf("no se pudo podar: %v", err)
	}

	var cuantas int
	base.QueryRowContext(ctx, `SELECT COUNT(*) FROM muestras_trafico`).Scan(&cuantas)
	if cuantas != 1 {
		t.Fatalf("deberia quedar solo la muestra nueva: quedan %d", cuantas)
	}
}

func TestUnaMedicionMuestreadaSeGuardaMarcadaComoEstimada(t *testing.T) {
	// sFlow no cuenta el trafico: lo muestrea. Una vez que las dos clases de
	// medicion estan en la misma tabla, sin esta marca ya no hay forma de volver
	// a separarlas, y una estimacion pasaria por medicion para siempre.
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.50", MAC: "aa:aa:aa:00:00:50", Metodo: "arp"},
	})

	if err := base.GuardarFlujos(ctx, []ConsumoPorFlujo{
		{IP: "192.168.1.50", BytesSube: 3_000_000, BytesBaja: 1_000_000,
			Conversaciones: 4, Estimado: true},
	}); err != nil {
		t.Fatalf("no se pudieron guardar los flujos: %v", err)
	}

	consumo, err := base.ConsumoPorEquipo(ctx, 24)
	if err != nil {
		t.Fatalf("no se pudo leer el consumo: %v", err)
	}
	if len(consumo) != 1 {
		t.Fatalf("se esperaba un renglon: %+v", consumo)
	}
	if !consumo[0].Estimado {
		t.Fatal("lo que salio de un muestreo tiene que leerse como estimado")
	}
	// Y la etiqueta lo dice, para que no haya que saberlo de memoria.
	if consumo[0].Puerto != "por el router (muestreo)" {
		t.Fatalf("la etiqueta deberia decir que es muestreo: %q", consumo[0].Puerto)
	}
}

func TestLoQueSeCuentaDeVerdadNoSeMarcaComoEstimado(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.51", MAC: "aa:aa:aa:00:00:51", Metodo: "arp"},
	})

	if err := base.GuardarFlujos(ctx, []ConsumoPorFlujo{
		{IP: "192.168.1.51", BytesSube: 1000, BytesBaja: 2000, Conversaciones: 1},
	}); err != nil {
		t.Fatalf("no se pudieron guardar los flujos: %v", err)
	}

	consumo, err := base.ConsumoPorEquipo(ctx, 24)
	if err != nil {
		t.Fatalf("no se pudo leer el consumo: %v", err)
	}
	if len(consumo) != 1 || consumo[0].Estimado {
		t.Fatalf("NetFlow cuenta de verdad, no estima: %+v", consumo)
	}
}
