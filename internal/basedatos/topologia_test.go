package basedatos

import (
	"context"
	"testing"
)

// sembrarEquipos deja unos equipos descubiertos para colgarlos de un switch.
func sembrarEquipos(t *testing.T, base *Base, equipos []EquipoDescubierto) {
	t.Helper()
	ctx := context.Background()

	escaneo, err := base.IniciarEscaneo(ctx, TipoProfundo)
	if err != nil {
		t.Fatalf("no se pudo iniciar el escaneo: %v", err)
	}
	if _, err := base.GuardarDescubrimiento(ctx, escaneo, true, equipos); err != nil {
		t.Fatalf("no se pudieron sembrar los equipos: %v", err)
	}
}

func TestUnaBocaConUnaMacEsUnEnlaceConfirmado(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.1", MAC: "aa:aa:aa:00:00:01", Metodo: "arp"},  // el switch
		{IP: "192.168.1.10", MAC: "bb:bb:bb:00:00:10", Metodo: "arp"}, // una laptop
	})

	err := base.GuardarSNMP(ctx, []FichaSNMP{{
		IP: "192.168.1.1", Nombre: "sw-principal", EsSwitch: true, Credencial: "publica",
		Interfaces: []InterfazSNMP{{Indice: 5, Nombre: "Gi0/5", Activa: true, VelocidadMbps: 1000}},
		MacsPorPuerto: map[string][]string{
			"5": {"bb:bb:bb:00:00:10"},
		},
	}})
	if err != nil {
		t.Fatalf("no se pudo guardar lo de SNMP: %v", err)
	}

	mapa, err := base.MapaDePuertos(ctx)
	if err != nil {
		t.Fatalf("no se pudo leer el mapa: %v", err)
	}
	if len(mapa) != 1 {
		t.Fatalf("se esperaba un renglon y hay %d", len(mapa))
	}
	if !mapa[0].Confirmado {
		t.Fatal("una boca con una sola MAC deberia ser un enlace confirmado")
	}
	if mapa[0].Puerto != "Gi0/5" {
		t.Fatalf("no se uso el nombre de la interfaz: %s", mapa[0].Puerto)
	}
	// Y la MAC se enlazo con el equipo que ya se conocia, no quedo suelta.
	if mapa[0].EquipoID == nil || mapa[0].EquipoIP != "192.168.1.10" {
		t.Fatalf("la boca no quedo enlazada al equipo: %+v", mapa[0])
	}

	capacidad, err := base.CalcularCapacidades(ctx)
	if err != nil {
		t.Fatalf("no se pudieron calcular las capacidades: %v", err)
	}
	if capacidad != CapacidadExacta {
		t.Fatalf("con un enlace confirmado la capacidad deberia ser exacta: %s", capacidad)
	}
}

func TestVariasMacEnLaMismaBocaSonUnGrupo(t *testing.T) {
	// Es el caso del switch no administrable colgado de uno administrable. Lo
	// honesto es decir "estos cuelgan del puerto 7", no inventarle un puerto a
	// cada uno.
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.1", MAC: "aa:aa:aa:00:00:01", Metodo: "arp"},
		{IP: "192.168.1.20", MAC: "cc:cc:cc:00:00:20", Metodo: "arp"},
		{IP: "192.168.1.21", MAC: "cc:cc:cc:00:00:21", Metodo: "arp"},
		{IP: "192.168.1.22", MAC: "cc:cc:cc:00:00:22", Metodo: "arp"},
	})

	err := base.GuardarSNMP(ctx, []FichaSNMP{{
		IP: "192.168.1.1", EsSwitch: true,
		Interfaces: []InterfazSNMP{{Indice: 7, Nombre: "Gi0/7", Activa: true}},
		MacsPorPuerto: map[string][]string{
			"7": {"cc:cc:cc:00:00:20", "cc:cc:cc:00:00:21", "cc:cc:cc:00:00:22"},
		},
	}})
	if err != nil {
		t.Fatalf("no se pudo guardar lo de SNMP: %v", err)
	}

	mapa, _ := base.MapaDePuertos(ctx)
	if len(mapa) != 3 {
		t.Fatalf("se esperaban 3 renglones y hay %d", len(mapa))
	}
	for _, renglon := range mapa {
		if renglon.Confirmado {
			t.Fatalf("con tres MAC en la boca nada deberia darse por confirmado: %+v", renglon)
		}
		if renglon.CuantosEnBoca != 3 {
			t.Fatalf("deberia decir que en esa boca hay 3: %+v", renglon)
		}
	}

	capacidad, _ := base.CalcularCapacidades(ctx)
	if capacidad != CapacidadPorGrupo {
		t.Fatalf("sin ninguna boca confirmada la capacidad deberia ser por grupo: %s", capacidad)
	}
}

func TestSinSwitchesNoHayMapaDePuertos(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "bb:bb:bb:00:00:10", Metodo: "arp"},
	})

	capacidad, err := base.CalcularCapacidades(ctx)
	if err != nil {
		t.Fatalf("no se pudieron calcular las capacidades: %v", err)
	}
	if capacidad != CapacidadNoDisponible {
		t.Fatalf("sin switches el mapa de puertos no esta disponible: %s", capacidad)
	}
}

func TestElMapaSeRehaceEnCadaConsulta(t *testing.T) {
	// La tabla de reenvio del switch es una foto del momento: si un equipo se
	// cambio de boca, la conexion vieja NO debe quedarse, o el plano mostraria
	// dos ubicaciones para el mismo aparato.
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.1", MAC: "aa:aa:aa:00:00:01", Metodo: "arp"},
		{IP: "192.168.1.10", MAC: "bb:bb:bb:00:00:10", Metodo: "arp"},
	})

	ficha := FichaSNMP{
		IP: "192.168.1.1", EsSwitch: true,
		Interfaces: []InterfazSNMP{
			{Indice: 5, Nombre: "Gi0/5"},
			{Indice: 9, Nombre: "Gi0/9"},
		},
		MacsPorPuerto: map[string][]string{"5": {"bb:bb:bb:00:00:10"}},
	}
	if err := base.GuardarSNMP(ctx, []FichaSNMP{ficha}); err != nil {
		t.Fatalf("primera consulta: %v", err)
	}

	// Se movio de la boca 5 a la 9.
	ficha.MacsPorPuerto = map[string][]string{"9": {"bb:bb:bb:00:00:10"}}
	if err := base.GuardarSNMP(ctx, []FichaSNMP{ficha}); err != nil {
		t.Fatalf("segunda consulta: %v", err)
	}

	mapa, _ := base.MapaDePuertos(ctx)
	if len(mapa) != 1 {
		t.Fatalf("el equipo aparece en %d bocas a la vez: %+v", len(mapa), mapa)
	}
	if mapa[0].Puerto != "Gi0/9" {
		t.Fatalf("no se movio de boca: %s", mapa[0].Puerto)
	}
}

func TestSeGuardanLosVecinosDeLLDP(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.1", MAC: "aa:aa:aa:00:00:01", Metodo: "arp"},
		{IP: "192.168.1.2", MAC: "aa:aa:aa:00:00:02", Metodo: "arp"},
	})

	err := base.GuardarSNMP(ctx, []FichaSNMP{{
		IP: "192.168.1.1", EsSwitch: true,
		Interfaces: []InterfazSNMP{{Indice: 1, Nombre: "Gi0/1"}},
		Vecinos: []VecinoSNMP{{
			InterfazLocal: "Gi0/1", Nombre: "sw-bodega",
			PuertoRemoto: "Gi0/24", ChasisID: "aa:aa:aa:00:00:02",
		}},
	}})
	if err != nil {
		t.Fatalf("no se pudo guardar: %v", err)
	}

	var vecino string
	var vecinoID *int64
	err = base.QueryRowContext(ctx,
		`SELECT vecino_nombre, vecino_equipo_id FROM enlaces WHERE interfaz_local = 'Gi0/1'`).
		Scan(&vecino, &vecinoID)
	if err != nil {
		t.Fatalf("no se guardo el enlace: %v", err)
	}
	if vecino != "sw-bodega" {
		t.Fatalf("nombre del vecino incorrecto: %s", vecino)
	}
	// Y el vecino se enlazo con el equipo que ya se conocia por su MAC.
	if vecinoID == nil {
		t.Fatal("el vecino deberia haberse enlazado al equipo conocido por su chasis")
	}
}
