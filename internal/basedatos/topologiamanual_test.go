package basedatos

import (
	"context"
	"errors"
	"testing"
)

// Un switch de casa no tiene direccion, no contesta a nada y ningun escaneo lo
// va a ver jamas. Toda esta parte existe para que ese aparato pueda estar en el
// mapa sin que se confunda con lo que si se midio.

func TestUnSwitchTontoSeDeclaraConSusBocas(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	creado, err := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "Switch del rack", Categoria: "switch_simple", Tipo: "Switch no administrable",
		Modelo: "TP-Link SG108", Puertos: 8,
	})
	if err != nil {
		t.Fatalf("no se pudo declarar el switch: %v", err)
	}
	if creado.Origen != "manual" {
		t.Fatalf("un equipo tecleado tiene que quedar marcado como manual: %q", creado.Origen)
	}
	if creado.Identidad == "" || creado.Identidad == "manual:0" {
		t.Fatalf("la identidad quedo mal armada: %q", creado.Identidad)
	}

	puertos, err := base.ListarPuertosFisicos(ctx)
	if err != nil {
		t.Fatalf("no se pudieron leer las bocas: %v", err)
	}
	if len(puertos) != 8 {
		t.Fatalf("se pidieron 8 bocas y hay %d", len(puertos))
	}

	// Repetir el nombre en la misma red se rechaza con un mensaje de negocio, no
	// con un choque de indice.
	if _, err := base.CrearEquipoManual(ctx, EquipoManual{Nombre: "switch del rack", Categoria: "switch_simple"}); !errors.Is(err, ErrEquipoRepetido) {
		t.Fatalf("se esperaba ErrEquipoRepetido y llego: %v", err)
	}
}

// Esta es la razon de la columna `origen`: sin ella, el switch declarado se
// marcaria ausente en cada barrido y la alerta de "lleva dias sin aparecer"
// avisaria de un aparato atornillado a la pared.
func TestUnEquipoDeclaradoNoSeMarcaAusenteAlEscanear(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	declarado, err := base.CrearEquipoManual(ctx, EquipoManual{Nombre: "Switch tonto", Categoria: "switch_simple", Puertos: 5})
	if err != nil {
		t.Fatalf("no se pudo declarar el switch: %v", err)
	}

	// Un barrido que ve otra cosa y NO ve el switch declarado.
	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "bb:bb:bb:00:00:10", Metodo: "arp"},
	})

	equipos, err := base.ListarEquipos(ctx, false)
	if err != nil {
		t.Fatalf("no se pudieron listar los equipos: %v", err)
	}
	for _, equipo := range equipos {
		if equipo.ID == declarado.ID && !equipo.Presente {
			t.Fatal("el escaneo marco ausente un equipo que nunca va a contestar")
		}
	}
}

func TestUnaBocaLlevaUnSoloCable(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	switchTonto, err := base.CrearEquipoManual(ctx, EquipoManual{Nombre: "Switch", Categoria: "switch_simple", Puertos: 4})
	if err != nil {
		t.Fatalf("no se pudo declarar el switch: %v", err)
	}
	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "bb:bb:bb:00:00:10", Metodo: "arp"},
		{IP: "192.168.1.11", MAC: "bb:bb:bb:00:00:11", Metodo: "arp"},
	})
	equipos, _ := base.ListarEquipos(ctx, false)
	var laptop, impresora int64
	for _, equipo := range equipos {
		switch equipo.IP {
		case "192.168.1.10":
			laptop = equipo.ID
		case "192.168.1.11":
			impresora = equipo.ID
		}
	}

	puertos, _ := base.ListarPuertosFisicos(ctx)
	boca := puertos[0].ID

	if _, err := base.CrearEnlaceManual(ctx, EnlaceFisico{
		PuertoOrigenID: boca, EquipoDestinoID: &laptop,
	}); err != nil {
		t.Fatalf("no se pudo conectar la laptop: %v", err)
	}

	// Volver a conectar la MISMA boca reemplaza: dos cables en una boca serian
	// dos verdades incompatibles colgando del mismo sitio.
	if _, err := base.CrearEnlaceManual(ctx, EnlaceFisico{
		PuertoOrigenID: boca, EquipoDestinoID: &impresora,
	}); err != nil {
		t.Fatalf("no se pudo reconectar la boca: %v", err)
	}

	enlaces, err := base.ListarEnlacesFisicos(ctx)
	if err != nil {
		t.Fatalf("no se pudieron leer los cables: %v", err)
	}
	if len(enlaces) != 1 {
		t.Fatalf("una boca deberia llevar un solo cable y hay %d", len(enlaces))
	}
	if enlaces[0].EquipoDestinoID == nil || *enlaces[0].EquipoDestinoID != impresora {
		t.Fatalf("el cable quedo apuntando al equipo viejo: %+v", enlaces[0])
	}
	if enlaces[0].OrigenDato != OrigenManual {
		t.Fatalf("un cable tecleado tiene que decir que es manual: %q", enlaces[0].OrigenDato)
	}
	if enlaces[0].EquipoOrigenID != switchTonto.ID {
		t.Fatalf("no se resolvio de que equipo sale el cable: %+v", enlaces[0])
	}
}

func TestNoSePuedeConectarUnEquipoConsigoMismo(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	creado, err := base.CrearEquipoManual(ctx, EquipoManual{Nombre: "Switch", Categoria: "switch_simple", Puertos: 2})
	if err != nil {
		t.Fatalf("no se pudo declarar el switch: %v", err)
	}
	puertos, _ := base.ListarPuertosFisicos(ctx)

	_, err = base.CrearEnlaceManual(ctx, EnlaceFisico{
		PuertoOrigenID: puertos[0].ID, PuertoDestinoID: &puertos[1].ID,
	})
	if !errors.Is(err, ErrEnlaceAlReves) {
		t.Fatalf("se esperaba ErrEnlaceAlReves y llego: %v", err)
	}

	suPropioID := creado.ID
	_, err = base.CrearEnlaceManual(ctx, EnlaceFisico{
		PuertoOrigenID: puertos[0].ID, EquipoDestinoID: &suPropioID,
	})
	if !errors.Is(err, ErrEnlaceAlReves) {
		t.Fatalf("se esperaba ErrEnlaceAlReves y llego: %v", err)
	}
}

// El caso que da sentido a la reconciliacion: el switch tonto se cambio por uno
// administrable y ahora el equipo contesta algo distinto a lo que se tecleo hace
// meses. Lo peligroso no es que cambie, es que cambie en silencio.
func TestSNMPQueContradiceLoDeclaradoSeReporta(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.1", MAC: "aa:aa:aa:00:00:01", Metodo: "arp"},  // el switch
		{IP: "192.168.1.10", MAC: "bb:bb:bb:00:00:10", Metodo: "arp"}, // lo que se declaro
		{IP: "192.168.1.11", MAC: "bb:bb:bb:00:00:11", Metodo: "arp"}, // lo que hay de verdad
	})

	equipos, _ := base.ListarEquipos(ctx, false)
	var elSwitch, declarado int64
	for _, equipo := range equipos {
		switch equipo.IP {
		case "192.168.1.1":
			elSwitch = equipo.ID
		case "192.168.1.10":
			declarado = equipo.ID
		}
	}

	boca, err := base.AgregarPuertoFisico(ctx, PuertoFisico{
		EquipoID: elSwitch, Numero: 5, Tipo: "lan",
	})
	if err != nil {
		t.Fatalf("no se pudo declarar la boca: %v", err)
	}
	if _, err := base.CrearEnlaceManual(ctx, EnlaceFisico{
		PuertoOrigenID: boca.ID, EquipoDestinoID: &declarado,
	}); err != nil {
		t.Fatalf("no se pudo declarar el cable: %v", err)
	}

	// Mientras nadie contradiga nada, no hay nada que avisar.
	choques, err := base.Contradicciones(ctx)
	if err != nil {
		t.Fatalf("no se pudieron comparar los cables: %v", err)
	}
	if len(choques) != 0 {
		t.Fatalf("no deberia haber contradicciones todavia: %+v", choques)
	}

	// Ahora el switch contesta, y dice otra cosa en esa misma boca.
	if _, err := base.GuardarSNMP(ctx, []FichaSNMP{{
		IP: "192.168.1.1", Nombre: "sw-nuevo", EsSwitch: true,
		Interfaces:    []InterfazSNMP{{Indice: 5, Nombre: "Gi0/5", Activa: true}},
		MacsPorPuerto: map[string][]string{"5": {"bb:bb:bb:00:00:11"}},
	}}); err != nil {
		t.Fatalf("no se pudo guardar lo de SNMP: %v", err)
	}

	choques, err = base.Contradicciones(ctx)
	if err != nil {
		t.Fatalf("no se pudieron comparar los cables: %v", err)
	}
	if len(choques) != 1 {
		t.Fatalf("se esperaba una contradiccion y hay %d: %+v", len(choques), choques)
	}
	if choques[0].Numero != 5 || choques[0].Fuente != "snmp" {
		t.Fatalf("la contradiccion no describe el tramo: %+v", choques[0])
	}
	if choques[0].Declarado == choques[0].Medido {
		t.Fatalf("no se distinguen las dos versiones: %+v", choques[0])
	}
}

func TestLaFichaGuardaLoQueNingunEscaneoAveriguaria(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "bb:bb:bb:00:00:10", Metodo: "arp"},
	})
	equipos, _ := base.ListarEquipos(ctx, false)

	err := base.ActualizarFicha(ctx, equipos[0].ID, FichaEquipo{
		Modelo: "Dell Latitude 5420", Notas: "La de contabilidad", Conexion: "wifi",
	})
	if err != nil {
		t.Fatalf("no se pudo guardar la ficha: %v", err)
	}

	equipos, _ = base.ListarEquipos(ctx, false)
	if equipos[0].Modelo != "Dell Latitude 5420" || equipos[0].Conexion != "wifi" {
		t.Fatalf("la ficha no se guardo: %+v", equipos[0])
	}

	// Una conexion que no es ni cable ni WiFi se rechaza con mensaje de negocio.
	if err := base.ActualizarFicha(ctx, equipos[0].ID, FichaEquipo{Conexion: "paloma"}); err == nil {
		t.Fatal("se acepto una forma de conexion que no existe")
	}
}

// El contador tiene que cuadrar con el mapa **por construccion**, no por
// sincronizarse: los dos leen la misma tabla. Esta prueba lo fija, porque el dia
// que alguien meta los equipos declarados en otro sitio, el contador seguiria
// dando un numero plausible y falso.
func TestElContadorCuentaTambienLoDeclaradoAMano(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.1", MAC: "aa:aa:aa:00:00:01", Metodo: "arp"},
		{IP: "192.168.1.10", MAC: "bb:bb:bb:00:00:10", Metodo: "arp"},
		{IP: "192.168.1.11", MAC: "bb:bb:bb:00:00:11", Metodo: "arp"},
	})

	// Lo descubierto todavia no tiene categoria: la pone el catalogo. Mientras
	// tanto cuenta como "sin reconocer", que es la verdad.
	resumen, err := base.ResumenDeCategorias(ctx, "sin_reconocer")
	if err != nil {
		t.Fatalf("no se pudo contar: %v", err)
	}
	if len(resumen) != 1 || resumen[0].Categoria != "sin_reconocer" || resumen[0].Cuantos != 3 {
		t.Fatalf("los equipos sin reconocer deberian contarse aparte, no esconderse: %+v", resumen)
	}

	// Ahora el catalogo reconoce dos, y SNMP dice que el tercero es un switch.
	equipos, _ := base.ListarEquipos(ctx, false)
	tipos := map[int64]Reconocido{}
	for _, equipo := range equipos {
		if equipo.IP != "192.168.1.1" {
			tipos[equipo.ID] = Reconocido{Tipo: "PC con Windows", Categoria: "computadora"}
		}
	}
	if _, err := base.PonerTipos(ctx, tipos); err != nil {
		t.Fatalf("no se pudieron poner los tipos: %v", err)
	}
	if _, err := base.GuardarSNMP(ctx, []FichaSNMP{{
		IP: "192.168.1.1", Nombre: "sw-principal", EsSwitch: true,
	}}); err != nil {
		t.Fatalf("no se pudo guardar lo de SNMP: %v", err)
	}
	if _, err := base.MarcarSwitchesAdministrables(ctx, "switch_administrable"); err != nil {
		t.Fatalf("no se pudieron marcar los switches: %v", err)
	}

	// Y se declara a mano un switch tonto, que ningun escaneo vera jamas.
	if _, err := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "Switch del rack", Categoria: "switch_simple", Puertos: 8,
	}); err != nil {
		t.Fatalf("no se pudo declarar el switch: %v", err)
	}

	resumen, err = base.ResumenDeCategorias(ctx, "sin_reconocer")
	if err != nil {
		t.Fatalf("no se pudo contar: %v", err)
	}

	cuenta := map[string]CuentaPorCategoria{}
	total := 0
	for _, fila := range resumen {
		cuenta[fila.Categoria] = fila
		total += fila.Cuantos
	}

	if total != 4 {
		t.Fatalf("la red tiene 4 aparatos y el contador dice %d: %+v", total, resumen)
	}
	if cuenta["computadora"].Cuantos != 2 {
		t.Fatalf("deberian contarse 2 computadoras: %+v", resumen)
	}
	if cuenta["switch_administrable"].Cuantos != 1 {
		t.Fatalf("el switch que contesto por SNMP deberia contar como administrable: %+v", resumen)
	}
	// La razon de ser de todo esto: el switch tonto NO se ve en ningun escaneo y
	// aun asi forma parte de la red.
	if cuenta["switch_simple"].Cuantos != 1 || cuenta["switch_simple"].Declarados != 1 {
		t.Fatalf("el switch declarado a mano tiene que contar, y decirse que es declarado: %+v", resumen)
	}
	if _, sobra := cuenta["sin_reconocer"]; sobra {
		t.Fatalf("ya no deberia quedar nada sin reconocer: %+v", resumen)
	}
}

// El catalogo NO le corrige la categoria a quien tenia el aparato delante.
func TestElCatalogoNoPisaLoQueSeDeclaroAMano(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	creado, err := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "Modem de la sala", Categoria: "gateway", IP: "192.168.1.254",
	})
	if err != nil {
		t.Fatalf("no se pudo declarar el modem: %v", err)
	}

	if _, err := base.PonerTipos(ctx, map[int64]Reconocido{
		creado.ID: {Tipo: "Router MikroTik", Categoria: "router"},
	}); err != nil {
		t.Fatalf("no se pudieron poner los tipos: %v", err)
	}

	equipos, _ := base.ListarEquipos(ctx, false)
	for _, equipo := range equipos {
		if equipo.ID == creado.ID && equipo.Categoria != "gateway" {
			t.Fatalf("el catalogo piso lo que declaro una persona: %q", equipo.Categoria)
		}
	}
}
