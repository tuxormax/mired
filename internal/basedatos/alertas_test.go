package basedatos

import (
	"context"
	"testing"
)

func TestAvisaDeUnEquipoNuevoYNoLoRepite(t *testing.T) {
	// Repetir el mismo aviso en cada corrida es como se logra que la gente deje
	// de hacerle caso a las alertas.
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	primero, _ := base.IniciarEscaneo(ctx, TipoProfundo)
	base.GuardarDescubrimiento(ctx, primero, true, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "aa:bb:cc:00:00:01", Nombre: "laptop", Metodo: "arp"},
	})

	nuevas, err := base.GenerarAlertas(ctx, primero)
	if err != nil {
		t.Fatalf("no se pudieron generar alertas: %v", err)
	}
	if len(nuevas) != 1 || nuevas[0].Tipo != AlertaEquipoNuevo {
		t.Fatalf("se esperaba una alerta de equipo nuevo: %+v", nuevas)
	}

	// Generarlas otra vez sobre el mismo escaneo no debe duplicar nada.
	repetidas, _ := base.GenerarAlertas(ctx, primero)
	if len(repetidas) != 0 {
		t.Fatalf("la alerta se repitio: %+v", repetidas)
	}
}

func TestAvisaDeUnPuertoNuevoEnUnEquipoConocido(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	primero, _ := base.IniciarEscaneo(ctx, TipoProfundo)
	base.GuardarDescubrimiento(ctx, primero, true, []EquipoDescubierto{{
		IP: "192.168.1.10", MAC: "aa:bb:cc:00:00:01", Metodo: "arp",
		Puertos: []PuertoDescubierto{{Numero: 22, Protocolo: "tcp"}},
	}})
	base.GenerarAlertas(ctx, primero)

	// Ahora le aparece el 3389, que es justo la senal que preocupa.
	segundo, _ := base.IniciarEscaneo(ctx, TipoProfundo)
	base.GuardarDescubrimiento(ctx, segundo, true, []EquipoDescubierto{{
		IP: "192.168.1.10", MAC: "aa:bb:cc:00:00:01", Metodo: "arp",
		Puertos: []PuertoDescubierto{
			{Numero: 22, Protocolo: "tcp"},
			{Numero: 3389, Protocolo: "tcp", Servicio: "Escritorio remoto"},
		},
	}})

	nuevas, err := base.GenerarAlertas(ctx, segundo)
	if err != nil {
		t.Fatalf("no se pudieron generar alertas: %v", err)
	}
	if len(nuevas) != 1 || nuevas[0].Tipo != AlertaPuertoNuevo {
		t.Fatalf("se esperaba una alerta de puerto nuevo: %+v", nuevas)
	}
}

func TestAvisaDeUnCambioDeIP(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	primero, _ := base.IniciarEscaneo(ctx, TipoPresencia)
	base.GuardarDescubrimiento(ctx, primero, false, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "aa:bb:cc:00:00:01", Metodo: "arp"},
	})
	base.GenerarAlertas(ctx, primero)

	segundo, _ := base.IniciarEscaneo(ctx, TipoPresencia)
	base.GuardarDescubrimiento(ctx, segundo, false, []EquipoDescubierto{
		{IP: "192.168.1.55", MAC: "aa:bb:cc:00:00:01", Metodo: "arp"},
	})

	nuevas, err := base.GenerarAlertas(ctx, segundo)
	if err != nil {
		t.Fatalf("no se pudieron generar alertas: %v", err)
	}
	if len(nuevas) != 1 || nuevas[0].Tipo != AlertaCambioIP {
		t.Fatalf("se esperaba una alerta de cambio de IP: %+v", nuevas)
	}
	if nuevas[0].Detalle != "Antes 192.168.1.10, ahora 192.168.1.55" {
		t.Fatalf("el detalle no dice el cambio: %s", nuevas[0].Detalle)
	}
}

func TestUnaReglaApagadaNoAvisa(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	if err := base.GuardarRegla(ctx, Regla{Tipo: AlertaEquipoNuevo, Activa: false}); err != nil {
		t.Fatalf("no se pudo apagar la regla: %v", err)
	}

	escaneo, _ := base.IniciarEscaneo(ctx, TipoProfundo)
	base.GuardarDescubrimiento(ctx, escaneo, true, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "aa:bb:cc:00:00:01", Metodo: "arp"},
	})

	nuevas, _ := base.GenerarAlertas(ctx, escaneo)
	for _, alerta := range nuevas {
		if alerta.Tipo == AlertaEquipoNuevo {
			t.Fatalf("la regla estaba apagada y aviso igual: %+v", alerta)
		}
	}
}

func TestMarcarVistasBajaElContador(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	escaneo, _ := base.IniciarEscaneo(ctx, TipoProfundo)
	base.GuardarDescubrimiento(ctx, escaneo, true, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "aa:bb:cc:00:00:01", Metodo: "arp"},
		{IP: "192.168.1.11", MAC: "aa:bb:cc:00:00:02", Metodo: "arp"},
	})
	base.GenerarAlertas(ctx, escaneo)

	abiertas, _ := base.ContarAlertasAbiertas(ctx)
	if abiertas != 2 {
		t.Fatalf("se esperaban 2 alertas abiertas y hay %d", abiertas)
	}

	marcadas, err := base.MarcarAlertasVistas(ctx, nil)
	if err != nil {
		t.Fatalf("no se pudieron marcar: %v", err)
	}
	if marcadas != 2 {
		t.Fatalf("se marcaron %d de 2", marcadas)
	}

	abiertas, _ = base.ContarAlertasAbiertas(ctx)
	if abiertas != 0 {
		t.Fatalf("quedaron %d alertas abiertas", abiertas)
	}
}

func TestDestinosDeAviso(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	if _, err := base.CrearDestino(ctx, DestinoAlerta{
		Nombre: "Telefono", Tipo: "ntfy", Destino: "https://ntfy.sh/mi-red",
	}); err != nil {
		t.Fatalf("no se pudo crear el destino: %v", err)
	}

	if _, err := base.CrearDestino(ctx, DestinoAlerta{
		Nombre: "Malo", Tipo: "paloma-mensajera", Destino: "x",
	}); err == nil {
		t.Fatal("un tipo de destino inventado deberia rechazarse")
	}

	destinos, err := base.ListarDestinos(ctx)
	if err != nil {
		t.Fatalf("no se pudieron listar: %v", err)
	}
	if len(destinos) != 1 || destinos[0].Tipo != "ntfy" {
		t.Fatalf("destinos inesperados: %+v", destinos)
	}

	if err := base.BorrarDestino(ctx, destinos[0].ID); err != nil {
		t.Fatalf("no se pudo borrar: %v", err)
	}
	destinos, _ = base.ListarDestinos(ctx)
	if len(destinos) != 0 {
		t.Fatalf("el destino borrado sigue ahi: %+v", destinos)
	}
}

func TestAvisaCuandoUnEquipoSeCambiaDePuerto(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.1", MAC: "aa:aa:aa:00:00:01", Metodo: "arp"},
		{IP: "192.168.1.10", MAC: "bb:bb:bb:00:00:10", Nombre: "laptop", Metodo: "arp"},
	})

	ficha := FichaSNMP{
		IP: "192.168.1.1", Nombre: "sw-principal", EsSwitch: true,
		Interfaces: []InterfazSNMP{
			{Indice: 5, Nombre: "Gi0/5"},
			{Indice: 9, Nombre: "Gi0/9"},
		},
		MacsPorPuerto: map[string][]string{"5": {"bb:bb:bb:00:00:10"}},
	}
	if _, err := base.GuardarSNMP(ctx, []FichaSNMP{ficha}); err != nil {
		t.Fatalf("primera consulta: %v", err)
	}

	// Alguien lo cambio del puerto 5 al 9.
	ficha.MacsPorPuerto = map[string][]string{"9": {"bb:bb:bb:00:00:10"}}
	movimientos, err := base.GuardarSNMP(ctx, []FichaSNMP{ficha})
	if err != nil {
		t.Fatalf("segunda consulta: %v", err)
	}
	if len(movimientos) != 1 {
		t.Fatalf("se esperaba un movimiento: %+v", movimientos)
	}
	if movimientos[0].Nombre != "laptop" {
		t.Fatalf("el movimiento deberia decir de quien es: %+v", movimientos[0])
	}

	if err := base.AlertasDeMovimiento(ctx, movimientos); err != nil {
		t.Fatalf("no se pudo generar la alerta: %v", err)
	}
	alertas, _ := base.ListarAlertas(ctx, true, 50)
	encontrada := false
	for _, alerta := range alertas {
		if alerta.Tipo == AlertaCambioPuertoSwitch {
			encontrada = true
			if alerta.Detalle == "" {
				t.Fatal("la alerta deberia decir de que puerto a que puerto")
			}
		}
	}
	if !encontrada {
		t.Fatalf("no se genero la alerta de cambio de puerto: %+v", alertas)
	}
}

func TestUnaRedQueNuncaSeEscaneoNoAvisaDeCaida(t *testing.T) {
	// Avisar aqui seria ruido el mismo dia que se crea el sitio.
	_, base, devolver := conRedDePrueba(t)
	defer devolver()

	nuevas, err := base.AlertaSiDejoDeReportar(context.Background())
	if err != nil {
		t.Fatalf("no deberia fallar: %v", err)
	}
	if len(nuevas) != 0 {
		t.Fatalf("una red sin escaneos no deberia avisar: %+v", nuevas)
	}
}
