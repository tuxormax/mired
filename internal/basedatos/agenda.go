package basedatos

import (
	"context"
	"fmt"
	"time"
)

// Tipos de barrido que sabe agendar el programador.
const (
	// TipoPresencia es el barrido corto y frecuente: solo quien esta.
	TipoPresencia = "presencia"
	// TipoProfundo es el caro: puertos, nombres y servicios.
	TipoProfundo = "profundo"
)

// Limites de la agenda. Existen para que nadie configure por accidente un
// barrido cada segundo y deje la red —o la Raspberry— de rodillas.
const (
	PresenciaMinimaSegundos = 15
	PresenciaMaximaSegundos = 3600
	ProfundoMinimoMinutos   = 5
	ProfundoMaximoMinutos   = 10080 // una semana
)

// TareaProgramada es un barrido que ya toca correr.
type TareaProgramada struct {
	Clave  string
	Nombre string
	Tipo   string
}

// TareasPendientes devuelve los barridos que ya vencieron.
//
// Se resuelve con UNA consulta al catalogo: el programador no abre el archivo de
// ninguna red para saber si le toca. Con treinta redes, preguntarle a cada
// archivo cada quince segundos seria trabajo constante para no hacer nada.
func (e *Enrutador) TareasPendientes(ctx context.Context) ([]TareaProgramada, error) {
	ahora := Ahora()

	filas, err := e.Catalogo.QueryContext(ctx, `
		SELECT clave, nombre,
		       CASE WHEN proximo_profundo IS NOT NULL AND proximo_profundo <= ?
		            THEN 'profundo' ELSE 'presencia' END AS tipo
		  FROM redes
		 WHERE estatus = 1
		   AND programado = 1
		   AND (
		         (proxima_presencia IS NOT NULL AND proxima_presencia <= ?)
		      OR (proximo_profundo  IS NOT NULL AND proximo_profundo  <= ?)
		       )
		 ORDER BY nombre`, ahora, ahora, ahora)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer las tareas programadas: %w", err)
	}
	defer filas.Close()

	tareas := []TareaProgramada{}
	for filas.Next() {
		var tarea TareaProgramada
		if err := filas.Scan(&tarea.Clave, &tarea.Nombre, &tarea.Tipo); err != nil {
			return nil, err
		}
		tareas = append(tareas, tarea)
	}
	return tareas, filas.Err()
}

// ProgramarSiguiente corre la agenda de una red hacia adelante.
//
// Cuando toca el barrido profundo tambien se recorre el de presencia: el
// profundo ya averigua quien esta, y encadenar los dos seguidos duplicaria el
// trabajo sin averiguar nada nuevo.
func (e *Enrutador) ProgramarSiguiente(ctx context.Context, clave, tipo string) error {
	red, err := e.BuscarRed(ctx, clave)
	if err != nil {
		return err
	}

	ahora := time.Now()
	presencia := ahora.Add(time.Duration(red.PresenciaCadaSegundos) * time.Second).Format(time.RFC3339)

	if tipo == TipoProfundo {
		profundo := ahora.Add(time.Duration(red.ProfundoCadaMinutos) * time.Minute).Format(time.RFC3339)
		_, err = e.Catalogo.ExecContext(ctx,
			`UPDATE redes SET proxima_presencia = ?, proximo_profundo = ? WHERE clave = ?`,
			presencia, profundo, clave)
	} else {
		_, err = e.Catalogo.ExecContext(ctx,
			`UPDATE redes SET proxima_presencia = ? WHERE clave = ?`, presencia, clave)
	}
	if err != nil {
		return fmt.Errorf("no se pudo recorrer la agenda de %s: %w", clave, err)
	}
	return nil
}

// ConfigurarAgenda enciende o apaga los barridos automaticos de una red y fija
// cada cuanto corren.
func (e *Enrutador) ConfigurarAgenda(ctx context.Context, clave string, programado bool, presenciaCada, profundoCada int) (Red, error) {
	if presenciaCada < PresenciaMinimaSegundos || presenciaCada > PresenciaMaximaSegundos {
		return Red{}, fmt.Errorf("el barrido de presencia debe ir de %d a %d segundos",
			PresenciaMinimaSegundos, PresenciaMaximaSegundos)
	}
	if profundoCada < ProfundoMinimoMinutos || profundoCada > ProfundoMaximoMinutos {
		return Red{}, fmt.Errorf("el escaneo profundo debe ir de %d a %d minutos",
			ProfundoMinimoMinutos, ProfundoMaximoMinutos)
	}

	var proximaPresencia, proximoProfundo any
	if programado {
		ahora := time.Now()
		// Al encender la agenda el primer barrido sale pronto pero no de
		// inmediato: si se enciende en varias redes seguidas, arrancarlas todas
		// en el mismo instante las pone a competir.
		proximaPresencia = ahora.Add(10 * time.Second).Format(time.RFC3339)
		proximoProfundo = ahora.Add(time.Duration(profundoCada) * time.Minute).Format(time.RFC3339)
	}

	resultado, err := e.Catalogo.ExecContext(ctx, `
		UPDATE redes
		   SET programado = ?, presencia_cada_segundos = ?, profundo_cada_minutos = ?,
		       proxima_presencia = ?, proximo_profundo = ?, modificada = ?
		 WHERE clave = ? AND estatus = 1`,
		boolAEntero(programado), presenciaCada, profundoCada,
		proximaPresencia, proximoProfundo, Ahora(), clave)
	if err != nil {
		return Red{}, fmt.Errorf("no se pudo guardar la agenda: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return Red{}, ErrRedNoExiste
	}
	return e.BuscarRed(ctx, clave)
}
