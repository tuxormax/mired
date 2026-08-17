package basedatos

import (
	"context"
	"testing"
)

// Las credenciales SNMP y las controladoras eran de TODAS las redes. Ahora son
// de cada una, y estas pruebas vigilan las dos mitades del cambio: que ya no se
// vean entre redes, y que lo que habia compartido no se pierda al actualizar.

func TestUnaCredencialEsSoloDeSuRed(t *testing.T) {
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()

	unaRed, err := enrutador.CrearRed(ctx, "Cliente A", "", []string{"192.168.1.0/24"})
	if err != nil {
		t.Fatalf("no se pudo crear la red: %v", err)
	}
	otraRed, err := enrutador.CrearRed(ctx, "Cliente B", "", []string{"10.0.0.0/24"})
	if err != nil {
		t.Fatalf("no se pudo crear la otra red: %v", err)
	}

	err = enrutador.ConRed(ctx, unaRed.Clave, func(base *Base) error {
		_, err := base.CrearCredencialSNMP(ctx, CredencialSNMP{
			Nombre: "Switches del cliente A", Version: "v2c", Comunidad: "secreta-de-A",
		})
		return err
	})
	if err != nil {
		t.Fatalf("no se pudo crear la credencial: %v", err)
	}

	// La de un cliente NO se ve desde la red de otro. Compartidas, la comunidad
	// de A se probaba contra los switches de B y el intento fallido quedaba
	// anotado en la bitacora de un equipo ajeno.
	var enLaOtra []CredencialSNMP
	if err := enrutador.ConRed(ctx, otraRed.Clave, func(base *Base) error {
		var err error
		enLaOtra, err = base.ListarCredencialesSNMP(ctx)
		return err
	}); err != nil {
		t.Fatalf("no se pudo listar en la otra red: %v", err)
	}
	if len(enLaOtra) != 0 {
		t.Fatalf("la credencial de una red se vio desde otra: %+v", enLaOtra)
	}

	// Y el mismo nombre puede existir en las dos: son cosas distintas.
	if err := enrutador.ConRed(ctx, otraRed.Clave, func(base *Base) error {
		_, err := base.CrearCredencialSNMP(ctx, CredencialSNMP{
			Nombre: "Switches del cliente A", Version: "v2c", Comunidad: "otra",
		})
		return err
	}); err != nil {
		t.Fatalf("el mismo nombre en otra red tenia que poder crearse: %v", err)
	}
}

func TestLoCompartidoSeRepartePorTodasLasRedesAlActualizar(t *testing.T) {
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()

	unaRed, _ := enrutador.CrearRed(ctx, "Sitio uno", "", []string{"192.168.1.0/24"})
	otraRed, _ := enrutador.CrearRed(ctx, "Sitio dos", "", []string{"10.0.0.0/24"})

	// Se simula la instalacion vieja: las filas viven en el catalogo, que es
	// donde estaban antes de la Rev 44.
	momento := Ahora()
	if _, err := enrutador.Catalogo.ExecContext(ctx, `
		INSERT INTO credenciales_snmp (nombre, version, comunidad, estatus, creada)
		VALUES ('La de siempre', 'v2c', 'publica', 1, ?)`, momento); err != nil {
		t.Fatalf("no se pudo sembrar la credencial vieja: %v", err)
	}
	if _, err := enrutador.Catalogo.ExecContext(ctx, `
		INSERT INTO controladoras (nombre, tipo, url, usuario, clave, sitio,
		                           verificar_tls, estatus, creada)
		VALUES ('UniFi de la oficina', 'unifi', 'https://10.0.0.5:8443', 'admin',
		        'clave', 'default', 0, 1, ?)`, momento); err != nil {
		t.Fatalf("no se pudo sembrar la controladora vieja: %v", err)
	}

	credenciales, controladoras, err := enrutador.MudarCredencialesALasRedes(ctx)
	if err != nil {
		t.Fatalf("la mudanza fallo: %v", err)
	}
	if credenciales != 1 || controladoras != 1 {
		t.Fatalf("se mudaron %d credenciales y %d controladoras", credenciales, controladoras)
	}

	// Estaban en uso en TODAS las redes: se copian a todas. Dejarlas en una sola
	// le quitaria el SNMP a las demas sin avisar a nadie.
	for _, clave := range []string{unaRed.Clave, otraRed.Clave} {
		var lista []CredencialSNMP
		var antenas []Controladora
		if err := enrutador.ConRed(ctx, clave, func(base *Base) error {
			var err error
			if lista, err = base.ListarCredencialesSNMP(ctx); err != nil {
				return err
			}
			antenas, err = base.ListarControladoras(ctx)
			return err
		}); err != nil {
			t.Fatalf("no se pudo leer la red %s: %v", clave, err)
		}
		if len(lista) != 1 || lista[0].Comunidad != "publica" {
			t.Fatalf("la red %s no recibio la credencial: %+v", clave, lista)
		}
		if len(antenas) != 1 || antenas[0].URL != "https://10.0.0.5:8443" {
			t.Fatalf("la red %s no recibio la controladora: %+v", clave, antenas)
		}
	}

	// Y el catalogo queda vacio: es lo que hace la mudanza irrepetible sin
	// necesidad de una marca aparte.
	var quedan int
	if err := enrutador.Catalogo.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM credenciales_snmp`).Scan(&quedan); err != nil {
		t.Fatalf("no se pudo contar el catalogo: %v", err)
	}
	if quedan != 0 {
		t.Fatalf("el catalogo se quedo con %d credenciales", quedan)
	}

	// Correrla otra vez no duplica nada.
	credenciales, controladoras, err = enrutador.MudarCredencialesALasRedes(ctx)
	if err != nil || credenciales != 0 || controladoras != 0 {
		t.Fatalf("la segunda mudanza tenia que no hacer nada: %d, %d, %v",
			credenciales, controladoras, err)
	}
}

func TestLaMudanzaNoPisaLoQueLaRedYaTenia(t *testing.T) {
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()
	red, _ := enrutador.CrearRed(ctx, "Sitio", "", []string{"192.168.1.0/24"})

	// Alguien ya creo una con ese nombre en la red, despues de actualizar.
	if err := enrutador.ConRed(ctx, red.Clave, func(base *Base) error {
		_, err := base.CrearCredencialSNMP(ctx, CredencialSNMP{
			Nombre: "La de siempre", Version: "v2c", Comunidad: "la-nueva",
		})
		return err
	}); err != nil {
		t.Fatalf("no se pudo crear: %v", err)
	}

	if _, err := enrutador.Catalogo.ExecContext(ctx, `
		INSERT INTO credenciales_snmp (nombre, version, comunidad, estatus, creada)
		VALUES ('La de siempre', 'v2c', 'la-vieja', 1, ?)`, Ahora()); err != nil {
		t.Fatalf("no se pudo sembrar: %v", err)
	}

	if _, _, err := enrutador.MudarCredencialesALasRedes(ctx); err != nil {
		t.Fatalf("la mudanza fallo: %v", err)
	}

	var lista []CredencialSNMP
	if err := enrutador.ConRed(ctx, red.Clave, func(base *Base) error {
		var err error
		lista, err = base.ListarCredencialesSNMP(ctx)
		return err
	}); err != nil {
		t.Fatalf("no se pudo listar: %v", err)
	}
	if len(lista) != 1 || lista[0].Comunidad != "la-nueva" {
		t.Fatalf("la mudanza piso lo que alguien ya habia escrito: %+v", lista)
	}
}
