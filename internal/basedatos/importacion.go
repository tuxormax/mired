package basedatos

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/tuxormax/mired/internal/importacion"
	"github.com/tuxormax/mired/internal/secreto"
)

// Aqui se aplica una importacion: se toma el plan ya validado y se escribe.
//
// **Todo en UNA transaccion.** Importar 23 aparatos y que falle el numero 18
// dejaria media hoja capturada y la otra media no, sin forma de saber por donde
// iba: o entra el archivo entero o no entra nada.
//
// El plan viene del paquete `importacion`, que no toca la base. Aqui se vuelve a
// mirar lo que ya existe **dentro de la transaccion**: entre que se enseno la
// vista previa y que se pulso el boton pudo pasar un escaneo y aparecer un
// aparato con ese mismo nombre.

// ResumenImportacion es lo que se hizo, para decirlo al terminar.
type ResumenImportacion struct {
	Creados      int `json:"creados"`
	Actualizados int `json:"actualizados"`
	Saltados     int `json:"saltados"`
	Rechazados   int `json:"rechazados"`
	Puertos      int `json:"puertos"`
	Cables       int `json:"cables"`
	Credenciales int `json:"credenciales"`
	// Recableados son los puertos que ya llevaban un cable a otro sitio y se
	// quedaron con el del archivo. Se cuentan aparte porque son el unico caso en
	// que la importacion PISA un dato que ya estaba.
	Recableados int `json:"recableados"`
}

// NombresDeEquipos devuelve los equipos vivos por su nombre normalizado.
//
// La llave es el nombre normalizado —sin acentos y en mayusculas— porque es como
// se compara: «Switch Site» y «switch site» son el mismo aparato para cualquiera
// que mire el rack, y crear los dos seria partir la red en dos mapas.
func (b *Base) NombresDeEquipos(ctx context.Context) (map[string]int64, error) {
	filas, err := b.QueryContext(ctx,
		`SELECT id, COALESCE(alias, nombre, '') FROM equipos WHERE estatus = 1`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer los nombres de los equipos: %w", err)
	}
	defer filas.Close()

	nombres := map[string]int64{}
	for filas.Next() {
		var id int64
		var nombre string
		if err := filas.Scan(&id, &nombre); err != nil {
			return nil, err
		}
		clave := importacion.Normalizar(nombre)
		if clave == "" {
			continue
		}
		nombres[clave] = id
	}
	return nombres, filas.Err()
}

// AplicarImportacion escribe el plan.
//
// [repetidos] dice que hacer con los aparatos que ya existen: actualizarlos con
// lo que trae el archivo, o dejarlos como estan.
func (b *Base) AplicarImportacion(ctx context.Context, caja *secreto.Caja,
	plan importacion.Plan, repetidos string) (ResumenImportacion, error) {
	var resumen ResumenImportacion
	momento := Ahora()

	err := b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		// Los nombres se releen aqui dentro: la vista previa se calculo antes y
		// entre medias pudo entrar un escaneo.
		porNombre, err := nombresEnTransaccion(ctx, tx)
		if err != nil {
			return err
		}

		// --- primera pasada: los aparatos.
		//
		// Van todos antes que los cables porque un cable necesita las dos puntas
		// creadas, y el archivo no viene ordenado: en la hoja de la clinica los
		// nodos estaban antes que su switch.
		for _, renglon := range plan.Renglones {
			if renglon.Accion == importacion.AccionRechazar {
				resumen.Rechazados++
				continue
			}

			clave := importacion.Normalizar(renglon.Nombre)
			equipoID, existe := porNombre[clave]

			if existe {
				if repetidos == importacion.RepetidosSaltar {
					resumen.Saltados++
					continue
				}
				if err := actualizarDesdeImportacion(ctx, tx, equipoID, renglon, momento); err != nil {
					return err
				}
				resumen.Actualizados++
			} else {
				equipoID, err = crearDesdeImportacion(ctx, tx, renglon, momento)
				if err != nil {
					return err
				}
				porNombre[clave] = equipoID
				resumen.Creados++
			}

			puestos, err := asegurarPuertos(ctx, tx, equipoID, renglon.Puertos, momento)
			if err != nil {
				return err
			}
			resumen.Puertos += puestos

			if renglon.Usuario != "" || renglon.Clave != "" || renglon.Direccion != "" {
				if err := guardarCredencialImportada(ctx, tx, caja, equipoID, renglon, momento); err != nil {
					return err
				}
				resumen.Credenciales++
			}
		}

		// --- segunda pasada: los cables.
		for _, renglon := range plan.Renglones {
			if renglon.Accion == importacion.AccionRechazar || !renglon.TieneCable() {
				continue
			}
			hijoID, hayHijo := porNombre[importacion.Normalizar(renglon.Nombre)]
			padreID, hayPadre := porNombre[importacion.Normalizar(renglon.CuelgaDe)]
			// Si el aparato se salto por repetido, su cable no tiene punta que
			// agarrar; y un padre que no esta ya se rechazo al planear.
			if !hayHijo || !hayPadre || hijoID == padreID {
				continue
			}

			puertoID, err := asegurarPuerto(ctx, tx, padreID, renglon.PuertoTipo,
				renglon.PuertoNumero, momento)
			if err != nil {
				return err
			}

			// Un puerto lleva UN cable. Si ese puerto ya iba a otro sitio, manda el
			// archivo: es lo que alguien acaba de mirar con el cable en la mano.
			// Se cuenta aparte para poder decirlo al terminar.
			var habia int
			if err := tx.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM enlaces_fisicos
				  WHERE (puerto_origen_id = ? OR puerto_destino_id = ?)
				    AND COALESCE(equipo_destino_id, -1) <> ?`,
				puertoID, puertoID, hijoID).Scan(&habia); err != nil {
				return err
			}
			resumen.Recableados += habia

			if _, err := tx.ExecContext(ctx,
				`DELETE FROM enlaces_fisicos
				  WHERE puerto_origen_id = ? OR puerto_destino_id = ?`,
				puertoID, puertoID); err != nil {
				return err
			}
			// El aparato tambien pudo estar colgado de otro sitio: se suelta, o
			// acabaria con dos cables de subida y dos padres en el mapa.
			if _, err := tx.ExecContext(ctx,
				`DELETE FROM enlaces_fisicos WHERE equipo_destino_id = ?`, hijoID); err != nil {
				return err
			}

			// El cable va al APARATO y no a un puerto suyo: el archivo dice por que
			// puerto del switch entra, no por cual del aparato sale. Suponerle uno
			// seria escribir en el mapa algo que nadie conto.
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO enlaces_fisicos (puerto_origen_id, puerto_destino_id,
				                             equipo_destino_id, origen_dato, notas, creado_en)
				VALUES (?, NULL, ?, 'manual', NULL, ?)`,
				puertoID, hijoID, momento); err != nil {
				return fmt.Errorf("no se pudo guardar el cable de %s: %w", renglon.Nombre, err)
			}
			resumen.Cables++
		}
		return nil
	})

	return resumen, err
}

func nombresEnTransaccion(ctx context.Context, tx *sql.Tx) (map[string]int64, error) {
	filas, err := tx.QueryContext(ctx,
		`SELECT id, COALESCE(alias, nombre, '') FROM equipos WHERE estatus = 1`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()

	nombres := map[string]int64{}
	for filas.Next() {
		var id int64
		var nombre string
		if err := filas.Scan(&id, &nombre); err != nil {
			return nil, err
		}
		if clave := importacion.Normalizar(nombre); clave != "" {
			nombres[clave] = id
		}
	}
	return nombres, filas.Err()
}

func crearDesdeImportacion(ctx context.Context, tx *sql.Tx,
	renglon importacion.Renglon, momento string) (int64, error) {
	resultado, err := tx.ExecContext(ctx, `
		INSERT INTO equipos (identidad, ip, mac, nombre, alias, tipo, categoria, modelo,
		                     notas, ubicacion, origen, metodo, presente, primera_vez,
		                     ultima_vez, estatus)
		VALUES ('', ?, ?, NULL, ?, ?, ?, ?, ?, ?, 'manual', 'manual', 1, ?, ?, 1)`,
		renglon.IP, nuloSiVacio(renglon.MAC), renglon.Nombre,
		nuloSiVacio(renglon.Tipo), nuloSiVacio(renglon.Categoria),
		nuloSiVacio(renglon.Modelo), nuloSiVacio(renglon.Notas),
		nuloSiVacio(renglon.Ubicacion), momento, momento)
	if err != nil {
		return 0, fmt.Errorf("no se pudo dar de alta «%s»: %w", renglon.Nombre, err)
	}
	equipoID, _ := resultado.LastInsertId()

	// La identidad lleva el id, y el id no existe hasta que existe la fila. El
	// prefijo dice de un vistazo que ese aparato no se identifica por nada
	// medido: lo puso una persona.
	_, err = tx.ExecContext(ctx, `UPDATE equipos SET identidad = ? WHERE id = ?`,
		fmt.Sprintf("manual:%d", equipoID), equipoID)
	return equipoID, err
}

// actualizarDesdeImportacion pisa SOLO lo que el archivo trae lleno.
//
// Una celda vacia significa «no lo se», no «borralo». Quien deja la columna
// MODELO en blanco no esta pidiendo que se borre el modelo que alguien tecleo
// hace un mes; es la misma regla que ya sigue la clave de una credencial.
func actualizarDesdeImportacion(ctx context.Context, tx *sql.Tx, equipoID int64,
	renglon importacion.Renglon, momento string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE equipos
		   SET tipo       = COALESCE(NULLIF(?, ''), tipo),
		       categoria  = COALESCE(NULLIF(?, ''), categoria),
		       modelo     = COALESCE(NULLIF(?, ''), modelo),
		       notas      = COALESCE(NULLIF(?, ''), notas),
		       ubicacion  = COALESCE(NULLIF(?, ''), ubicacion),
		       ip         = CASE WHEN ? <> '' THEN ? ELSE ip END,
		       mac        = COALESCE(NULLIF(?, ''), mac),
		       modificado = ?
		 WHERE id = ? AND estatus = 1`,
		renglon.Tipo, renglon.Categoria, renglon.Modelo, renglon.Notas,
		renglon.Ubicacion, renglon.IP, renglon.IP, renglon.MAC, momento, equipoID)
	if err != nil {
		return fmt.Errorf("no se pudo actualizar «%s»: %w", renglon.Nombre, err)
	}
	return nil
}

// asegurarPuertos declara los puertos 1..N que falten.
//
// Se crean los que falten y no se borra ninguno: un switch al que alguien le
// declaro ocho puertos y en el archivo dice cuatro no pierde los otros cuatro,
// que a lo mejor llevan cable.
func asegurarPuertos(ctx context.Context, tx *sql.Tx, equipoID int64, cuantos int,
	momento string) (int, error) {
	puestos := 0
	for numero := 1; numero <= cuantos; numero++ {
		var existe int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM puertos_fisicos WHERE equipo_id = ? AND numero = ? AND tipo = 'lan'`,
			equipoID, numero).Scan(&existe); err != nil {
			return puestos, err
		}
		if existe > 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO puertos_fisicos (equipo_id, numero, tipo, creado_en)
			VALUES (?, ?, 'lan', ?)`, equipoID, numero, momento); err != nil {
			return puestos, fmt.Errorf("no se pudo declarar el puerto %d: %w", numero, err)
		}
		puestos++
	}
	return puestos, nil
}

// asegurarPuerto devuelve el id del puerto, creandolo si no estaba.
//
// El archivo dice «puerto 7 del switch» y ese puerto puede no estar declarado
// todavia: rechazar el renglon por eso obligaria a capturar a mano los 24
// puertos antes de importar, que es justo lo que se quiere evitar.
func asegurarPuerto(ctx context.Context, tx *sql.Tx, equipoID int64, tipo string,
	numero int, momento string) (int64, error) {
	var puertoID int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM puertos_fisicos WHERE equipo_id = ? AND numero = ? AND tipo = ?`,
		equipoID, numero, tipo).Scan(&puertoID)
	if err == nil {
		return puertoID, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	resultado, err := tx.ExecContext(ctx, `
		INSERT INTO puertos_fisicos (equipo_id, numero, tipo, creado_en)
		VALUES (?, ?, ?, ?)`, equipoID, numero, tipo, momento)
	if err != nil {
		return 0, fmt.Errorf("no se pudo declarar el puerto %s %d: %w",
			strings.ToUpper(tipo), numero, err)
	}
	puertoID, _ = resultado.LastInsertId()
	return puertoID, nil
}

// guardarCredencialImportada deja como se entra al aparato, con la clave cifrada.
//
// La clave se cifra con la misma caja que el resto del programa: en la base
// nunca hay una contrasena en claro. Lo que si va en claro es el ARCHIVO del que
// salio, y de eso avisa la pantalla.
func guardarCredencialImportada(ctx context.Context, tx *sql.Tx, caja *secreto.Caja,
	equipoID int64, renglon importacion.Renglon, momento string) error {
	guardada := ""
	if renglon.Clave != "" {
		if caja == nil {
			return fmt.Errorf("no hay con que cifrar la clave de «%s»", renglon.Nombre)
		}
		var err error
		guardada, err = caja.Cifrar(renglon.Clave)
		if err != nil {
			return fmt.Errorf("no se pudo cifrar la clave de «%s»: %w", renglon.Nombre, err)
		}
	}

	_, err := tx.ExecContext(ctx, `
		INSERT INTO credenciales_equipo (equipo_id, tipo, usuario, clave, direccion, notas, creada)
		VALUES (?, ?, ?, ?, ?, NULL, ?)
		ON CONFLICT (equipo_id, tipo) DO UPDATE SET
			usuario    = excluded.usuario,
			clave      = COALESCE(NULLIF(excluded.clave, ''), credenciales_equipo.clave),
			direccion  = excluded.direccion,
			modificada = excluded.creada`,
		equipoID, renglon.Acceso, renglon.Usuario, guardada,
		nuloSiVacio(renglon.Direccion), momento)
	if err != nil {
		return fmt.Errorf("no se pudo guardar el acceso de «%s»: %w", renglon.Nombre, err)
	}
	return nil
}
