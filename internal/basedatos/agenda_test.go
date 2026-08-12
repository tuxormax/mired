package basedatos

import (
	"context"
	"testing"
)

func TestAgendaApagadaNoProduceTareas(t *testing.T) {
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()

	if _, err := enrutador.CrearRed(ctx, "Matriz", "", []string{"192.168.1.0/24"}); err != nil {
		t.Fatalf("no se pudo crear la red: %v", err)
	}

	// Una red recien creada NO se escanea sola: encenderlo es una decision de
	// quien la administra, no algo que pase por omision.
	tareas, err := enrutador.TareasPendientes(ctx)
	if err != nil {
		t.Fatalf("no se pudieron leer las tareas: %v", err)
	}
	if len(tareas) != 0 {
		t.Fatalf("una red sin agenda no deberia tener tareas: %+v", tareas)
	}
}

func TestEncenderAgendaProduceTareaDePresencia(t *testing.T) {
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()

	red, _ := enrutador.CrearRed(ctx, "Matriz", "", []string{"192.168.1.0/24"})
	if _, err := enrutador.ConfigurarAgenda(ctx, red.Clave, true, 60, 360); err != nil {
		t.Fatalf("no se pudo configurar la agenda: %v", err)
	}

	// Recien encendida, la primera presencia se agenda unos segundos adelante,
	// asi que todavia no vence.
	tareas, _ := enrutador.TareasPendientes(ctx)
	if len(tareas) != 0 {
		t.Fatalf("la primera tarea no deberia vencer de inmediato: %+v", tareas)
	}

	// Se fuerza el vencimiento para comprobar que la consulta la encuentra.
	if _, err := enrutador.Catalogo.ExecContext(ctx,
		`UPDATE redes SET proxima_presencia = '2000-01-01T00:00:00-06:00' WHERE clave = ?`,
		red.Clave); err != nil {
		t.Fatalf("no se pudo forzar el vencimiento: %v", err)
	}

	tareas, _ = enrutador.TareasPendientes(ctx)
	if len(tareas) != 1 || tareas[0].Tipo != TipoPresencia {
		t.Fatalf("se esperaba una tarea de presencia: %+v", tareas)
	}
}

func TestElProfundoGanaCuandoLosDosVencen(t *testing.T) {
	// Si vencieron los dos, corre el profundo: ya averigua quien esta, asi que
	// hacer ademas el de presencia seria repetir trabajo.
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()

	red, _ := enrutador.CrearRed(ctx, "Matriz", "", []string{"192.168.1.0/24"})
	enrutador.ConfigurarAgenda(ctx, red.Clave, true, 60, 360)
	enrutador.Catalogo.ExecContext(ctx, `
		UPDATE redes SET proxima_presencia = '2000-01-01T00:00:00-06:00',
		                 proximo_profundo  = '2000-01-01T00:00:00-06:00'
		 WHERE clave = ?`, red.Clave)

	tareas, _ := enrutador.TareasPendientes(ctx)
	if len(tareas) != 1 || tareas[0].Tipo != TipoProfundo {
		t.Fatalf("se esperaba una tarea profunda: %+v", tareas)
	}

	// Y al recorrer la agenda de un profundo se recorren las dos, para no
	// encadenar un barrido de presencia justo despues.
	if err := enrutador.ProgramarSiguiente(ctx, red.Clave, TipoProfundo); err != nil {
		t.Fatalf("no se pudo recorrer la agenda: %v", err)
	}
	tareas, _ = enrutador.TareasPendientes(ctx)
	if len(tareas) != 0 {
		t.Fatalf("despues de recorrer la agenda no deberia quedar nada vencido: %+v", tareas)
	}
}

func TestLaAgendaRechazaRitmosAbsurdos(t *testing.T) {
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()

	red, _ := enrutador.CrearRed(ctx, "Matriz", "", nil)

	if _, err := enrutador.ConfigurarAgenda(ctx, red.Clave, true, 1, 360); err == nil {
		t.Fatal("un barrido cada segundo deberia rechazarse")
	}
	if _, err := enrutador.ConfigurarAgenda(ctx, red.Clave, true, 60, 1); err == nil {
		t.Fatal("un escaneo profundo cada minuto deberia rechazarse")
	}
}

func TestSeAnotaCadaCambioDePresenciaYSoloLosCambios(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	// Aparece.
	primero, _ := base.IniciarEscaneo(ctx, TipoPresencia)
	base.GuardarDescubrimiento(ctx, primero, false, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "aa:bb:cc:dd:ee:01", Metodo: "arp"},
	})

	// Sigue ahi: no debe anotarse nada nuevo.
	segundo, _ := base.IniciarEscaneo(ctx, TipoPresencia)
	base.GuardarDescubrimiento(ctx, segundo, false, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "aa:bb:cc:dd:ee:01", Metodo: "arp"},
	})

	// Se va.
	tercero, _ := base.IniciarEscaneo(ctx, TipoPresencia)
	base.GuardarDescubrimiento(ctx, tercero, false, nil)

	// Vuelve.
	cuarto, _ := base.IniciarEscaneo(ctx, TipoPresencia)
	base.GuardarDescubrimiento(ctx, cuarto, false, []EquipoDescubierto{
		{IP: "192.168.1.10", MAC: "aa:bb:cc:dd:ee:01", Metodo: "arp"},
	})

	equipos, _ := base.ListarEquipos(ctx, false)
	eventos, err := base.ListarPresencia(ctx, equipos[0].ID, 100)
	if err != nil {
		t.Fatalf("no se pudo leer la presencia: %v", err)
	}

	// Aparecio, se fue y volvio: tres eventos. El barrido en que no cambio nada
	// no debe dejar rastro, porque si no la tabla crece sin decir nada nuevo.
	if len(eventos) != 3 {
		t.Fatalf("se esperaban 3 eventos y hay %d: %+v", len(eventos), eventos)
	}
	if !eventos[0].Presente || eventos[1].Presente || !eventos[2].Presente {
		t.Fatalf("la secuencia de presencia quedo mal: %+v", eventos)
	}
}
