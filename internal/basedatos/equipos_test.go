package basedatos

import (
	"context"
	"testing"
)

// conRedDePrueba crea una red y deja lista su base para trabajar.
func conRedDePrueba(t *testing.T) (*Enrutador, *Base, func()) {
	t.Helper()

	enrutador := enrutadorDePrueba(t)
	red, err := enrutador.CrearRed(context.Background(), "Prueba", "", []string{"192.168.1.0/24"})
	if err != nil {
		t.Fatalf("no se pudo crear la red: %v", err)
	}

	base, devolver, err := enrutador.Red(context.Background(), red.Clave)
	if err != nil {
		t.Fatalf("no se pudo abrir la red: %v", err)
	}
	return enrutador, base, devolver
}

func TestGuardarDescubrimientoDaDeAltaYActualiza(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	escaneo, err := base.IniciarEscaneo(ctx, "profundo")
	if err != nil {
		t.Fatalf("no se pudo iniciar el escaneo: %v", err)
	}

	resumen, err := base.GuardarDescubrimiento(ctx, escaneo, true, []EquipoDescubierto{
		{
			IP: "192.168.1.10", MAC: "B8:27:EB:1A:2B:3C", Nombre: "impresora",
			Fabricante: "Raspberry Pi", Metodo: "arp", Subred: "192.168.1.0/24",
			Puertos: []PuertoDescubierto{{Numero: 9100, Protocolo: "tcp", Servicio: "Impresion cruda"}},
		},
		{IP: "192.168.1.20", Metodo: "tcp", Subred: "192.168.1.0/24"},
	})
	if err != nil {
		t.Fatalf("no se pudo guardar el descubrimiento: %v", err)
	}
	if resumen.Vistos != 2 || resumen.Nuevos != 2 {
		t.Fatalf("resumen inesperado: %+v", resumen)
	}

	equipos, err := base.ListarEquipos(ctx, false)
	if err != nil {
		t.Fatalf("no se pudieron listar los equipos: %v", err)
	}
	if len(equipos) != 2 {
		t.Fatalf("se esperaban 2 equipos y hay %d", len(equipos))
	}

	// La identidad es la MAC cuando la hay, y la IP marcada cuando no.
	if equipos[0].Identidad != "b8:27:eb:1a:2b:3c" {
		t.Fatalf("la identidad deberia ser la MAC en minusculas: %s", equipos[0].Identidad)
	}
	if equipos[1].Identidad != "ip:192.168.1.20" {
		t.Fatalf("sin MAC la identidad deberia marcarse como IP: %s", equipos[1].Identidad)
	}
	if len(equipos[0].Puertos) != 1 || equipos[0].Puertos[0].Numero != 9100 {
		t.Fatalf("no se guardo el puerto: %+v", equipos[0].Puertos)
	}
}

func TestLaIPCambiaYElEquipoNoSeDuplica(t *testing.T) {
	// Es la razon de usar la MAC como identidad: con DHCP la IP cambia sola, y
	// tratarla como identidad duplicaria el equipo en cada reconexion.
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	primero, _ := base.IniciarEscaneo(ctx, "profundo")
	_, err := base.GuardarDescubrimiento(ctx, primero, true, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "aa:bb:cc:dd:ee:ff", Nombre: "laptop", Metodo: "arp"},
	})
	if err != nil {
		t.Fatalf("primer escaneo: %v", err)
	}

	segundo, _ := base.IniciarEscaneo(ctx, "profundo")
	resumen, err := base.GuardarDescubrimiento(ctx, segundo, true, []EquipoDescubierto{
		{IP: "192.168.1.55", MAC: "AA:BB:CC:DD:EE:FF", Metodo: "arp"},
	})
	if err != nil {
		t.Fatalf("segundo escaneo: %v", err)
	}
	if resumen.Nuevos != 0 {
		t.Fatalf("el equipo se duplico al cambiar de IP: %+v", resumen)
	}

	equipos, _ := base.ListarEquipos(ctx, false)
	if len(equipos) != 1 {
		t.Fatalf("se esperaba 1 equipo y hay %d", len(equipos))
	}
	if equipos[0].IP != "192.168.1.55" {
		t.Fatalf("no se actualizo la IP: %s", equipos[0].IP)
	}
	// El nombre que trajo el primer escaneo no se pierde porque el segundo no lo
	// haya traido.
	if equipos[0].Nombre != "laptop" {
		t.Fatalf("se perdio el nombre descubierto antes: %q", equipos[0].Nombre)
	}
}

func TestElQueNoContestaQuedaAusenteYNoSeBorra(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	primero, _ := base.IniciarEscaneo(ctx, "profundo")
	base.GuardarDescubrimiento(ctx, primero, true, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "aa:bb:cc:dd:ee:01", Metodo: "arp"},
		{IP: "192.168.1.11", MAC: "aa:bb:cc:dd:ee:02", Metodo: "arp"},
	})

	segundo, _ := base.IniciarEscaneo(ctx, "profundo")
	resumen, err := base.GuardarDescubrimiento(ctx, segundo, true, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "aa:bb:cc:dd:ee:01", Metodo: "arp"},
	})
	if err != nil {
		t.Fatalf("segundo escaneo: %v", err)
	}
	if resumen.Ausentes != 1 {
		t.Fatalf("se esperaba 1 ausente y hubo %d", resumen.Ausentes)
	}

	todos, _ := base.ListarEquipos(ctx, false)
	if len(todos) != 2 {
		t.Fatalf("un equipo apagado no debe borrarse: quedan %d", len(todos))
	}
	presentes, _ := base.ListarEquipos(ctx, true)
	if len(presentes) != 1 {
		t.Fatalf("se esperaba 1 presente y hay %d", len(presentes))
	}
}

func TestUnPuertoQueSeCierraSeMarcaYNoDesaparece(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	primero, _ := base.IniciarEscaneo(ctx, "profundo")
	base.GuardarDescubrimiento(ctx, primero, true, []EquipoDescubierto{{
		IP: "192.168.1.10", MAC: "aa:bb:cc:dd:ee:01", Metodo: "arp",
		Puertos: []PuertoDescubierto{
			{Numero: 22, Protocolo: "tcp"},
			{Numero: 3389, Protocolo: "tcp"},
		},
	}})

	segundo, _ := base.IniciarEscaneo(ctx, "profundo")
	_, err := base.GuardarDescubrimiento(ctx, segundo, true, []EquipoDescubierto{{
		IP: "192.168.1.10", MAC: "aa:bb:cc:dd:ee:01", Metodo: "arp",
		Puertos: []PuertoDescubierto{{Numero: 22, Protocolo: "tcp"}},
	}})
	if err != nil {
		t.Fatalf("segundo escaneo: %v", err)
	}

	equipos, _ := base.ListarEquipos(ctx, false)
	if len(equipos[0].Puertos) != 1 || equipos[0].Puertos[0].Numero != 22 {
		t.Fatalf("deberia quedar solo el 22 abierto: %+v", equipos[0].Puertos)
	}

	// Pero la fila sigue ahi: "tenia abierto el 3389 y ya no" es informacion.
	var cuantos int
	base.QueryRowContext(ctx, `SELECT COUNT(*) FROM puertos WHERE numero = 3389`).Scan(&cuantos)
	if cuantos != 1 {
		t.Fatalf("el puerto cerrado no debe borrarse de la historia")
	}
}

func TestResumenDeRedAlimentaElCatalogo(t *testing.T) {
	enrutador, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	escaneo, _ := base.IniciarEscaneo(ctx, "profundo")
	base.GuardarDescubrimiento(ctx, escaneo, true, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "aa:bb:cc:dd:ee:01", Metodo: "arp"},
		{IP: "192.168.1.11", MAC: "aa:bb:cc:dd:ee:02", Metodo: "arp"},
	})

	equipos, presentes, ultimo, err := base.ResumenDeRed(ctx)
	if err != nil {
		t.Fatalf("no se pudo armar el resumen: %v", err)
	}
	if equipos != 2 || presentes != 2 || ultimo == "" {
		t.Fatalf("resumen inesperado: %d equipos, %d presentes, ultimo %q", equipos, presentes, ultimo)
	}

	redes, _ := enrutador.ListarRedes(ctx)
	if err := enrutador.ActualizarResumen(ctx, redes[0].Clave, equipos, presentes, ultimo); err != nil {
		t.Fatalf("no se pudo actualizar el resumen: %v", err)
	}

	// El panel de inicio se pinta con esto, sin abrir el archivo de la red.
	redes, _ = enrutador.ListarRedes(ctx)
	if redes[0].Equipos != 2 || redes[0].EquiposPresentes != 2 {
		t.Fatalf("el catalogo no quedo con el resumen: %+v", redes[0])
	}
}

func TestPonerAlias(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	escaneo, _ := base.IniciarEscaneo(ctx, "profundo")
	base.GuardarDescubrimiento(ctx, escaneo, true, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "aa:bb:cc:dd:ee:01", Nombre: "hp-lj", Metodo: "arp"},
	})

	equipos, _ := base.ListarEquipos(ctx, false)
	if err := base.PonerAlias(ctx, equipos[0].ID, "Impresora de contabilidad"); err != nil {
		t.Fatalf("no se pudo poner el alias: %v", err)
	}

	equipos, _ = base.ListarEquipos(ctx, false)
	if equipos[0].ComoSeLlama() != "Impresora de contabilidad" {
		t.Fatalf("el alias deberia mandar sobre el nombre descubierto: %s", equipos[0].ComoSeLlama())
	}
}
