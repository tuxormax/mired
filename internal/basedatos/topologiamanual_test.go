package basedatos

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// Un switch de casa no tiene direccion, no contesta a nada y ningun escaneo lo
// va a ver jamas. Toda esta parte existe para que ese aparato pueda estar en el
// mapa sin que se confunda con lo que si se midio.

func TestUnSwitchTontoSeDeclaraConSusPuertos(t *testing.T) {
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
		t.Fatalf("no se pudieron leer los puertos: %v", err)
	}
	if len(puertos) != 8 {
		t.Fatalf("se pidieron 8 puertos y hay %d", len(puertos))
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

func TestUnPuertoLlevaUnSoloCable(t *testing.T) {
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
	puerto := puertos[0].ID

	if _, err := base.CrearEnlaceManual(ctx, EnlaceFisico{
		PuertoOrigenID: puerto, EquipoDestinoID: &laptop,
	}); err != nil {
		t.Fatalf("no se pudo conectar la laptop: %v", err)
	}

	// Volver a conectar el MISMO puerto reemplaza: dos cables en un puerto serian
	// dos verdades incompatibles colgando del mismo sitio.
	if _, err := base.CrearEnlaceManual(ctx, EnlaceFisico{
		PuertoOrigenID: puerto, EquipoDestinoID: &impresora,
	}); err != nil {
		t.Fatalf("no se pudo reconectar el puerto: %v", err)
	}

	enlaces, err := base.ListarEnlacesFisicos(ctx)
	if err != nil {
		t.Fatalf("no se pudieron leer los cables: %v", err)
	}
	if len(enlaces) != 1 {
		t.Fatalf("un puerto deberia llevar un solo cable y hay %d", len(enlaces))
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

	puerto, err := base.AgregarPuertoFisico(ctx, PuertoFisico{
		EquipoID: elSwitch, Numero: 5, Tipo: "lan",
	})
	if err != nil {
		t.Fatalf("no se pudo declarar el puerto: %v", err)
	}
	if _, err := base.CrearEnlaceManual(ctx, EnlaceFisico{
		PuertoOrigenID: puerto.ID, EquipoDestinoID: &declarado,
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

	// Ahora el switch contesta, y dice otra cosa en ese mismo puerto.
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

func TestElCableOcupaUnPuertoEnLasDosPuntas(t *testing.T) {
	// El caso de una casa: un modem con un switch colgado. Si el cable solo
	// ocupa el puerto del modem, el switch de cinco sigue diciendo que tiene
	// cinco libres, y esa cuenta es justamente la que se usa para saber cuanto
	// lugar queda.
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	modem, err := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "Modem", Categoria: "gateway", Puertos: 1})
	if err != nil {
		t.Fatalf("no se pudo declarar el modem: %v", err)
	}
	conmutador, err := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "Switch", Categoria: "switch_simple", Puertos: 5})
	if err != nil {
		t.Fatalf("no se pudo declarar el switch: %v", err)
	}

	puertos, _ := base.ListarPuertosFisicos(ctx)
	var delModem, delSwitch []int64
	for _, puerto := range puertos {
		switch puerto.EquipoID {
		case modem.ID:
			delModem = append(delModem, puerto.ID)
		case conmutador.ID:
			delSwitch = append(delSwitch, puerto.ID)
		}
	}
	if len(delModem) != 1 || len(delSwitch) != 5 {
		t.Fatalf("se esperaban 1 y 5 puertos y hay %d y %d", len(delModem), len(delSwitch))
	}

	uplink := delSwitch[4]
	if _, err := base.CrearEnlaceManual(ctx, EnlaceFisico{
		PuertoOrigenID: delModem[0], PuertoDestinoID: &uplink,
	}); err != nil {
		t.Fatalf("no se pudo conectar puerto contra puerto: %v", err)
	}

	topologia, err := base.LeerTopologiaManual(ctx)
	if err != nil {
		t.Fatalf("no se pudo leer la topologia: %v", err)
	}

	ocupados := map[int64]bool{}
	for _, enlace := range topologia.Enlaces {
		ocupados[enlace.PuertoOrigenID] = true
		if enlace.PuertoDestinoID != nil {
			ocupados[*enlace.PuertoDestinoID] = true
		}
	}
	if !ocupados[uplink] {
		t.Fatal("el puerto del switch por donde sube el cable tiene que quedar ocupado")
	}

	libres := 0
	for _, puerto := range delSwitch {
		if !ocupados[puerto] {
			libres++
		}
	}
	if libres != 4 {
		t.Fatalf("al switch de cinco con el uplink puesto le quedan 4 libres, no %d", libres)
	}
}

func TestUnPuertoDeDestinoTampocoLlevaDosCables(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	uno, err := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "Switch de arriba", Categoria: "switch_simple", Puertos: 2})
	if err != nil {
		t.Fatalf("no se pudo declarar: %v", err)
	}
	dos, err := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "Switch de abajo", Categoria: "switch_simple", Puertos: 2})
	if err != nil {
		t.Fatalf("no se pudo declarar: %v", err)
	}

	puertos, _ := base.ListarPuertosFisicos(ctx)
	var arriba, abajo []int64
	for _, puerto := range puertos {
		if puerto.EquipoID == uno.ID {
			arriba = append(arriba, puerto.ID)
		}
		if puerto.EquipoID == dos.ID {
			abajo = append(abajo, puerto.ID)
		}
	}

	destino := abajo[0]
	if _, err := base.CrearEnlaceManual(ctx, EnlaceFisico{
		PuertoOrigenID: arriba[0], PuertoDestinoID: &destino,
	}); err != nil {
		t.Fatalf("no se pudo conectar el primer cable: %v", err)
	}
	// Otro cable, desde otro puerto, al MISMO puerto de destino: reemplaza, no
	// se suma. Dos cables en un puerto serian dos verdades incompatibles.
	if _, err := base.CrearEnlaceManual(ctx, EnlaceFisico{
		PuertoOrigenID: arriba[1], PuertoDestinoID: &destino,
	}); err != nil {
		t.Fatalf("no se pudo reconectar el puerto de destino: %v", err)
	}

	topologia, _ := base.LeerTopologiaManual(ctx)
	cuantos := 0
	for _, enlace := range topologia.Enlaces {
		if enlace.PuertoDestinoID != nil && *enlace.PuertoDestinoID == destino {
			cuantos++
		}
	}
	if cuantos != 1 {
		t.Fatalf("el puerto de destino quedo con %d cables", cuantos)
	}
}

// Rehacer una tabla en SQLite es lo mas parecido que hay a una operacion a
// corazon abierto: `enlaces_fisicos` cuelga de `puertos_fisicos` con ON DELETE
// CASCADE, y soltar la tabla vieja en el orden equivocado borra TODOS los cables
// declarados sin decir nada. Esta prueba corre la migracion 0018 sobre las
// tablas de antes, con datos dentro, y comprueba que no se perdio ninguno.
func TestLaMigracionDeTiposDePuertoNoSeLlevaLosCables(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	// Se vuelve a la forma vieja: tablas de 0011, con la lista corta de tipos.
	viejo := `
        DROP TABLE enlaces_fisicos;
        DROP TABLE puertos_fisicos;
        CREATE TABLE puertos_fisicos (
            id             INTEGER PRIMARY KEY AUTOINCREMENT,
            equipo_id      INTEGER NOT NULL REFERENCES equipos (id) ON DELETE CASCADE,
            numero         INTEGER NOT NULL CHECK (numero BETWEEN 1 AND 512),
            tipo           TEXT    NOT NULL CHECK (tipo IN ('lan', 'wan')),
            velocidad_mbps INTEGER CHECK (velocidad_mbps IS NULL OR velocidad_mbps > 0),
            notas          TEXT,
            creado_en      TEXT    NOT NULL
        );
        CREATE UNIQUE INDEX ux_puertos_fisicos ON puertos_fisicos (equipo_id, numero, tipo);
        CREATE TABLE enlaces_fisicos (
            id                INTEGER PRIMARY KEY AUTOINCREMENT,
            puerto_origen_id  INTEGER NOT NULL REFERENCES puertos_fisicos (id) ON DELETE CASCADE,
            puerto_destino_id INTEGER REFERENCES puertos_fisicos (id) ON DELETE CASCADE,
            equipo_destino_id INTEGER REFERENCES equipos (id) ON DELETE CASCADE,
            origen_dato       TEXT NOT NULL CHECK (
                                  origen_dato IN ('manual', 'snmp', 'lldp', 'cdp', 'inferido')
                              ),
            notas             TEXT,
            creado_en         TEXT NOT NULL,
            CHECK (
                (puerto_destino_id IS NOT NULL AND equipo_destino_id IS NULL)
                OR
                (puerto_destino_id IS NULL AND equipo_destino_id IS NOT NULL)
            )
        );
        CREATE UNIQUE INDEX ux_enlaces_fisicos_origen ON enlaces_fisicos (puerto_origen_id);`
	if _, err := base.ExecContext(ctx, viejo); err != nil {
		t.Fatalf("no se pudo dejar la base como estaba antes: %v", err)
	}

	modem, err := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "Modem", Categoria: "gateway", Puertos: 0,
	})
	if err != nil {
		t.Fatalf("no se pudo declarar el modem: %v", err)
	}
	conmutador, err := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "Switch", Categoria: "switch_simple", Puertos: 0,
	})
	if err != nil {
		t.Fatalf("no se pudo declarar el switch: %v", err)
	}

	ahora := Ahora()
	if _, err := base.ExecContext(ctx, `
        INSERT INTO puertos_fisicos (id, equipo_id, numero, tipo, notas, creado_en)
        VALUES (1, ?, 1, 'lan', '', ?), (2, ?, 1, 'lan', '', ?)`,
		modem.ID, ahora, conmutador.ID, ahora); err != nil {
		t.Fatalf("no se pudieron sembrar los puertos: %v", err)
	}
	if _, err := base.ExecContext(ctx, `
        INSERT INTO enlaces_fisicos
            (id, puerto_origen_id, puerto_destino_id, origen_dato, notas, creado_en)
        VALUES (1, 1, 2, 'manual', '', ?)`, ahora); err != nil {
		t.Fatalf("no se pudo sembrar el cable: %v", err)
	}

	// Y ahora la migracion de verdad, leida del mismo archivo que se aplica en
	// produccion: una copia del SQL aqui probaria otra cosa.
	cuerpo, err := esquemas.ReadFile("esquema/red/0018_tipos_de_puerto.sql")
	if err != nil {
		t.Fatalf("no se encontro la migracion: %v", err)
	}
	if err := base.EnTransaccion(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, string(cuerpo))
		return err
	}); err != nil {
		t.Fatalf("la migracion no corrio: %v", err)
	}

	var cables, puertos int
	if err := base.QueryRowContext(ctx, `SELECT COUNT(*) FROM enlaces_fisicos`).Scan(&cables); err != nil {
		t.Fatalf("no se pudieron contar los cables: %v", err)
	}
	if err := base.QueryRowContext(ctx, `SELECT COUNT(*) FROM puertos_fisicos`).Scan(&puertos); err != nil {
		t.Fatalf("no se pudieron contar los puertos: %v", err)
	}
	if puertos != 2 {
		t.Fatalf("se sembraron 2 puertos y quedaron %d", puertos)
	}
	if cables != 1 {
		t.Fatalf("el cable declarado se perdio en la migracion: quedaron %d", cables)
	}

	// El tipo nuevo entra, y la lista sigue cerrada.
	if _, err := base.ExecContext(ctx, `
        INSERT INTO puertos_fisicos (equipo_id, numero, tipo, notas, creado_en)
        VALUES (?, 2, 'dmz', '', ?)`, modem.ID, ahora); err != nil {
		t.Fatalf("un puerto DMZ tendria que entrar: %v", err)
	}
	if _, err := base.ExecContext(ctx, `
        INSERT INTO puertos_fisicos (equipo_id, numero, tipo, notas, creado_en)
        VALUES (?, 3, 'inventado', '', ?)`, modem.ID, ahora); err == nil {
		t.Fatal("un tipo que no existe tiene que rebotar")
	}

	// Y el cascade sigue en pie: borrar el equipo se lleva sus puertos y su cable.
	if err := base.BorrarEquipoManual(ctx, modem.ID); err != nil {
		t.Fatalf("no se pudo borrar el modem: %v", err)
	}
	if err := base.QueryRowContext(ctx, `SELECT COUNT(*) FROM enlaces_fisicos`).Scan(&cables); err != nil {
		t.Fatalf("no se pudieron contar los cables: %v", err)
	}
	if cables != 0 {
		t.Fatalf("al borrar el equipo su cable tenia que irse con el: quedaron %d", cables)
	}
}
