package basedatos

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/tuxormax/mired/internal/secreto"
)

// El WiFi no tiene puertos: a una antena se le cuelgan uno o VARIOS equipos sin
// inventarle un puerto a cada uno. La regla va al reves: un equipo cuelga de UNA
// antena a la vez, porque asi funciona el WiFi.

func TestDeUnaAntenaCuelganVariosEquipos(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	antena, err := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "AP del patio", Categoria: "punto_de_acceso"})
	if err != nil {
		t.Fatalf("no se pudo declarar la antena: %v", err)
	}
	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "bb:bb:bb:00:00:10", Metodo: "arp"},
		{IP: "192.168.1.11", MAC: "bb:bb:bb:00:00:11", Metodo: "arp"},
		{IP: "192.168.1.12", MAC: "bb:bb:bb:00:00:12", Metodo: "arp"},
	})

	equipos, _ := base.ListarEquipos(ctx, false)
	colgados := 0
	for _, equipo := range equipos {
		if equipo.ID == antena.ID {
			continue
		}
		if _, err := base.GuardarEnlaceInalambrico(ctx, EnlaceInalambrico{
			EquipoID: equipo.ID, AntenaID: antena.ID, Red: "CASA-5G",
		}); err != nil {
			t.Fatalf("no se pudo colgar %s: %v", equipo.IP, err)
		}
		colgados++
	}

	enlaces, err := base.ListarEnlacesInalambricos(ctx)
	if err != nil {
		t.Fatalf("no se pudieron leer los enlaces: %v", err)
	}
	if len(enlaces) != colgados {
		t.Fatalf("se colgaron %d equipos y se leyeron %d", colgados, len(enlaces))
	}
	for _, enlace := range enlaces {
		if enlace.AntenaID != antena.ID || enlace.Red != "CASA-5G" {
			t.Errorf("el enlace quedo raro: %+v", enlace)
		}
		if enlace.AntenaNombre != "AP del patio" {
			t.Errorf("el nombre de la antena no se resolvio: %+v", enlace)
		}
	}
}

func TestUnEquipoCuelgaDeUnaSolaAntena(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	patio, _ := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "AP del patio", Categoria: "punto_de_acceso"})
	sala, _ := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "AP de la sala", Categoria: "punto_de_acceso"})
	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "bb:bb:bb:00:00:10", Metodo: "arp"},
	})

	equipos, _ := base.ListarEquipos(ctx, false)
	var telefono int64
	for _, equipo := range equipos {
		if equipo.IP == "192.168.1.10" {
			telefono = equipo.ID
		}
	}

	if _, err := base.GuardarEnlaceInalambrico(ctx, EnlaceInalambrico{
		EquipoID: telefono, AntenaID: patio.ID}); err != nil {
		t.Fatalf("no se pudo colgar del patio: %v", err)
	}
	// Se movio de antena: reemplaza, no se suma. Un telefono no esta asociado a
	// dos antenas a la vez.
	if _, err := base.GuardarEnlaceInalambrico(ctx, EnlaceInalambrico{
		EquipoID: telefono, AntenaID: sala.ID}); err != nil {
		t.Fatalf("no se pudo mover a la sala: %v", err)
	}

	enlaces, _ := base.ListarEnlacesInalambricos(ctx)
	if len(enlaces) != 1 {
		t.Fatalf("el telefono quedo colgando de %d antenas", len(enlaces))
	}
	if enlaces[0].AntenaID != sala.ID {
		t.Fatalf("quedo en la antena equivocada: %+v", enlaces[0])
	}
}

func TestUnaAntenaNoSeCuelgaDeSiMisma(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	antena, _ := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "AP", Categoria: "punto_de_acceso"})

	_, err := base.GuardarEnlaceInalambrico(ctx, EnlaceInalambrico{
		EquipoID: antena.ID, AntenaID: antena.ID})
	if !errors.Is(err, ErrEnlaceAlReves) {
		t.Fatalf("se esperaba ErrEnlaceAlReves y llego: %v", err)
	}
}

func TestLoQueDijoLaAntenaPisaLoDeclarado(t *testing.T) {
	// Si alguien declaro que el telefono cuelga del AP del patio y despues la
	// antena de la sala dice que lo tiene asociado, gana la MEDICION: la antena
	// sabe quien esta asociado ahora mismo.
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	patio, _ := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "AP del patio", Categoria: "punto_de_acceso"})
	sala, _ := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "AP de la sala", Categoria: "punto_de_acceso"})
	sembrarEquipos(t, base, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "bb:bb:bb:00:00:10", Metodo: "arp"},
	})

	equipos, _ := base.ListarEquipos(ctx, false)
	var telefono int64
	for _, equipo := range equipos {
		if equipo.IP == "192.168.1.10" {
			telefono = equipo.ID
		}
	}

	if _, err := base.GuardarEnlaceInalambrico(ctx, EnlaceInalambrico{
		EquipoID: telefono, AntenaID: patio.ID, OrigenDato: OrigenManual}); err != nil {
		t.Fatalf("no se pudo declarar: %v", err)
	}

	senal := -47
	guardados, err := base.GuardarAsociadosDeAntena(ctx, sala.ID, OrigenPanel,
		[]EnlaceInalambrico{{EquipoID: telefono, Red: "CASA-5G", SenalDbm: &senal}})
	if err != nil {
		t.Fatalf("no se pudieron guardar los asociados: %v", err)
	}
	if guardados != 1 {
		t.Fatalf("se guardaron %d asociados", guardados)
	}

	enlaces, _ := base.ListarEnlacesInalambricos(ctx)
	if len(enlaces) != 1 || enlaces[0].AntenaID != sala.ID {
		t.Fatalf("no gano lo medido: %+v", enlaces)
	}
	if enlaces[0].OrigenDato != OrigenPanel {
		t.Errorf("el origen quedo en %q", enlaces[0].OrigenDato)
	}
	if enlaces[0].SenalDbm == nil || *enlaces[0].SenalDbm != -47 {
		t.Errorf("la senal no se guardo: %+v", enlaces[0])
	}
}

func TestUnAsociadoQueNadieConoceNoRompeNada(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	antena, _ := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "AP", Categoria: "punto_de_acceso"})

	// Una MAC que el inventario todavia no vio: no es un error, aparecera en el
	// proximo barrido.
	guardados, err := base.GuardarAsociadosDeAntena(ctx, antena.ID, OrigenSNMP,
		[]EnlaceInalambrico{{EquipoID: 99999}})
	if err != nil {
		t.Fatalf("un asociado desconocido no puede fallar: %v", err)
	}
	if guardados != 0 {
		t.Fatalf("se guardaron %d enlaces de un equipo que no existe", guardados)
	}
}

// ------------------------------------------------- credenciales de equipo --

func TestLaClaveDeUnEquipoSeGuardaCifrada(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()
	caja := secreto.NuevaCaja(filepath.Join(t.TempDir(), "llave"))

	equipo, _ := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "Modem", Categoria: "gateway"})

	if _, err := base.GuardarCredencialEquipo(ctx, caja, CredencialEquipo{
		EquipoID: equipo.ID, Tipo: "web", Usuario: "admin",
		Clave: "clave-secreta-2026", Direccion: "http://192.168.1.254",
	}); err != nil {
		t.Fatalf("no se pudo guardar: %v", err)
	}

	// En la base NO puede estar en claro.
	var guardada string
	if err := base.QueryRowContext(ctx,
		`SELECT clave FROM credenciales_equipo WHERE equipo_id = ?`, equipo.ID).Scan(&guardada); err != nil {
		t.Fatalf("no se pudo leer la fila: %v", err)
	}
	if guardada == "clave-secreta-2026" {
		t.Fatal("la clave quedo en claro dentro de la base")
	}

	// Y al pedirla expresamente, vuelve entera.
	credencial, err := base.CredencialDeEquipo(ctx, caja, equipo.ID, "web", true)
	if err != nil {
		t.Fatalf("no se pudo leer la credencial: %v", err)
	}
	if credencial.Clave != "clave-secreta-2026" {
		t.Fatalf("la clave volvio como %q", credencial.Clave)
	}
	if credencial.Usuario != "admin" || credencial.Direccion != "http://192.168.1.254" {
		t.Errorf("la credencial volvio incompleta: %+v", credencial)
	}
}

func TestLaClaveNoViajaSiNadieLaPidio(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()
	caja := secreto.NuevaCaja(filepath.Join(t.TempDir(), "llave"))

	equipo, _ := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "Switch", Categoria: "switch_simple"})
	if _, err := base.GuardarCredencialEquipo(ctx, caja, CredencialEquipo{
		EquipoID: equipo.ID, Usuario: "admin", Clave: "otra-clave"}); err != nil {
		t.Fatalf("no se pudo guardar: %v", err)
	}

	// Ni al leerla sin pedirla...
	credencial, _ := base.CredencialDeEquipo(ctx, caja, equipo.ID, "web", false)
	if credencial.Clave != "" {
		t.Error("la clave viajo sin que nadie la pidiera")
	}
	if !credencial.TieneClave {
		t.Error("deberia decir que hay una clave guardada")
	}

	// ...ni en el listado de la red, que es lo que pinta la lista de equipos.
	porEquipo, err := base.CredencialesDeRed(ctx)
	if err != nil {
		t.Fatalf("no se pudieron listar: %v", err)
	}
	for _, lista := range porEquipo {
		for _, credencial := range lista {
			if credencial.Clave != "" {
				t.Fatal("una clave viajo en el listado de la red")
			}
		}
	}

	// Y en el listado de equipos tampoco.
	equipos, _ := base.ListarEquipos(ctx, false)
	for _, fila := range equipos {
		for _, credencial := range fila.Credenciales {
			if credencial.Clave != "" {
				t.Fatal("una clave viajo en la lista de equipos")
			}
		}
	}
}

func TestGuardarSinClaveNoBorraLaQueHabia(t *testing.T) {
	// En un formulario donde la clave se muestra oculta, dejarla en blanco
	// significa "no la toques", no "quitala".
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()
	caja := secreto.NuevaCaja(filepath.Join(t.TempDir(), "llave"))

	equipo, _ := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "Antena", Categoria: "punto_de_acceso"})
	if _, err := base.GuardarCredencialEquipo(ctx, caja, CredencialEquipo{
		EquipoID: equipo.ID, Usuario: "ubnt", Clave: "la-de-siempre"}); err != nil {
		t.Fatalf("no se pudo guardar: %v", err)
	}

	// Se corrige solo el usuario, sin tocar la clave.
	if _, err := base.GuardarCredencialEquipo(ctx, caja, CredencialEquipo{
		EquipoID: equipo.ID, Usuario: "admin"}); err != nil {
		t.Fatalf("no se pudo actualizar: %v", err)
	}

	credencial, _ := base.CredencialDeEquipo(ctx, caja, equipo.ID, "web", true)
	if credencial.Usuario != "admin" {
		t.Errorf("el usuario no se actualizo: %q", credencial.Usuario)
	}
	if credencial.Clave != "la-de-siempre" {
		t.Fatalf("la clave se perdio al guardar sin ella: %q", credencial.Clave)
	}
}

func TestNoSeGuardaUnaCredencialDeTipoInventado(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()
	caja := secreto.NuevaCaja(filepath.Join(t.TempDir(), "llave"))

	equipo, _ := base.CrearEquipoManual(ctx, EquipoManual{
		Nombre: "Modem", Categoria: "gateway"})

	if _, err := base.GuardarCredencialEquipo(ctx, caja, CredencialEquipo{
		EquipoID: equipo.ID, Tipo: "telepatia", Usuario: "admin"}); err == nil {
		t.Fatal("se acepto un tipo de credencial que no existe")
	}
}
