package basedatos

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// El consumo por aplicacion, que llena el paquete opcional mired-dpi.
//
// A diferencia del consumo por puerto o por flujos, aqui la pregunta que se
// responde no es "cuanto gasta este equipo" sino "en que se lo gasta".

// ConsumoPorAplicacion es lo que un equipo movio hablando con un destino.
type ConsumoPorAplicacion struct {
	IP         string `json:"ip"`
	Equipo     string `json:"equipo"`
	Aplicacion string `json:"aplicacion"`
	// Como dice de donde salio el nombre: tls, http, dns, puerto o desconocido.
	// No todos valen igual, y esconderlo seria presentar una suposicion con la
	// misma cara que un dato.
	Como     string `json:"como"`
	Bytes    int64  `json:"bytes"`
	Paquetes int64  `json:"paquetes"`
	Momento  string `json:"momento"`
}

// GuardarAplicaciones anota lo que vio mired-dpi y lo enlaza con el equipo que
// ya se conocia.
func (b *Base) GuardarAplicaciones(ctx context.Context, consumos []ConsumoPorAplicacion) error {
	if len(consumos) == 0 {
		return nil
	}
	momento := Ahora()

	return b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		for _, consumo := range consumos {
			var equipoID any
			if id, hay := porIP(ctx, tx, consumo.IP); hay {
				equipoID = id
			}
			_, err := tx.ExecContext(ctx, `
				INSERT INTO trafico_aplicaciones (equipo_id, ip, aplicacion, como,
				                                  momento, bytes, paquetes)
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				equipoID, consumo.IP, consumo.Aplicacion, consumo.Como, momento,
				consumo.Bytes, consumo.Paquetes)
			if err != nil {
				return fmt.Errorf("no se pudo guardar el consumo de %s: %w", consumo.IP, err)
			}
		}
		return nil
	})
}

// ConsumoPorAplicaciones suma en que se gastaron el ancho de banda las ultimas
// horas.
//
// Se agrupa por equipo y aplicacion, no solo por aplicacion: "Netflix se llevo
// 40 GB" no sirve de nada si no se sabe de que maquina salieron.
func (b *Base) ConsumoPorAplicaciones(ctx context.Context, horas int) ([]ConsumoPorAplicacion, error) {
	if horas <= 0 || horas > 720 {
		horas = 24
	}
	desde := time.Now().Add(-time.Duration(horas) * time.Hour).Format(time.RFC3339)

	filas, err := b.QueryContext(ctx, `
		SELECT a.ip,
		       COALESCE(e.alias, e.nombre, a.ip),
		       a.aplicacion,
		       -- Un mismo destino puede haberse identificado de varias formas en
		       -- el periodo. Se conserva la mejor: MIN pone antes 'dns', 'http' y
		       -- 'puerto' que 'tls', asi que se elige a mano abajo.
		       MIN(a.como),
		       SUM(a.bytes), SUM(a.paquetes), MAX(a.momento)
		  FROM trafico_aplicaciones a
		  LEFT JOIN equipos e ON e.id = a.equipo_id
		 WHERE a.momento >= ?
		 GROUP BY a.ip, a.aplicacion
		 ORDER BY SUM(a.bytes) DESC
		 LIMIT 200`, desde)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el consumo por aplicacion: %w", err)
	}
	defer filas.Close()

	consumo := []ConsumoPorAplicacion{}
	for filas.Next() {
		var c ConsumoPorAplicacion
		if err := filas.Scan(&c.IP, &c.Equipo, &c.Aplicacion, &c.Como,
			&c.Bytes, &c.Paquetes, &c.Momento); err != nil {
			return nil, err
		}
		consumo = append(consumo, c)
	}
	return consumo, filas.Err()
}

// PodarAplicaciones borra las mediciones viejas.
//
// mired-dpi escribe MUCHO mas que los otros dos medidores —un renglon por equipo
// y destino cada pocos minutos—, asi que sin podar la tabla crece rapido, y en
// una Raspberry eso importa.
func (b *Base) PodarAplicaciones(ctx context.Context, diasAConservar int) error {
	if diasAConservar <= 0 {
		diasAConservar = 14
	}
	limite := time.Now().AddDate(0, 0, -diasAConservar).Format(time.RFC3339)

	_, err := b.ExecContext(ctx, `DELETE FROM trafico_aplicaciones WHERE momento < ?`, limite)
	return err
}
