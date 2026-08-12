package basedatos

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrEquipoNoExiste lo devuelven las operaciones sobre un equipo que ya no esta.
var ErrEquipoNoExiste = errors.New("el equipo no existe")

// Equipo es un aparato descubierto en una red.
type Equipo struct {
	ID         int64    `json:"id"`
	Identidad  string   `json:"identidad"`
	IP         string   `json:"ip"`
	MAC        string   `json:"mac"`
	Fabricante string   `json:"fabricante"`
	Nombre     string   `json:"nombre"`
	Alias      string   `json:"alias"`
	Tipo       string   `json:"tipo"`
	Subred     string   `json:"subred"`
	Metodo     string   `json:"metodo"`
	Presente   bool     `json:"presente"`
	PrimeraVez string   `json:"primeraVez"`
	UltimaVez  string   `json:"ultimaVez"`
	Puertos    []Puerto `json:"puertos"`
}

// ComoSeLlama devuelve el nombre que conviene mostrar: manda el que puso una
// persona sobre el que se descubrio, y si no hay ninguno, la IP.
func (e Equipo) ComoSeLlama() string {
	switch {
	case e.Alias != "":
		return e.Alias
	case e.Nombre != "":
		return e.Nombre
	default:
		return e.IP
	}
}

// Puerto es un puerto abierto de un equipo.
type Puerto struct {
	Numero    int    `json:"numero"`
	Protocolo string `json:"protocolo"`
	Servicio  string `json:"servicio"`
	Banner    string `json:"banner"`
	Abierto   bool   `json:"abierto"`
	UltimaVez string `json:"ultimaVez"`
}

// EquipoDescubierto es lo que trae un barrido de la sonda, antes de guardarse.
type EquipoDescubierto struct {
	IP         string
	MAC        string
	Nombre     string
	Fabricante string
	Metodo     string
	Subred     string
	Puertos    []PuertoDescubierto
}

// PuertoDescubierto es un puerto que contesto durante el barrido.
type PuertoDescubierto struct {
	Numero    int
	Protocolo string
	Servicio  string
	Banner    string
}

// ResumenEscaneo es lo que dejo una corrida.
type ResumenEscaneo struct {
	EscaneoID int64 `json:"escaneoId"`
	Vistos    int   `json:"vistos"`
	Nuevos    int   `json:"nuevos"`
	Ausentes  int   `json:"ausentes"`
}

// IniciarEscaneo deja constancia de que empezo un barrido, antes de que termine.
//
// Se anota al empezar y no al terminar para que un escaneo que se cae a la mitad
// deje rastro: si solo se anotara al final, un barrido que nunca termina seria
// invisible.
func (b *Base) IniciarEscaneo(ctx context.Context, tipo string) (int64, error) {
	resultado, err := b.ExecContext(ctx,
		`INSERT INTO escaneos (tipo, iniciado, estado) VALUES (?, ?, 'en_curso')`,
		tipo, Ahora())
	if err != nil {
		return 0, fmt.Errorf("no se pudo registrar el escaneo: %w", err)
	}
	return resultado.LastInsertId()
}

// FallarEscaneo marca un barrido que no pudo completarse.
func (b *Base) FallarEscaneo(ctx context.Context, escaneoID int64, motivo string) error {
	_, err := b.ExecContext(ctx,
		`UPDATE escaneos SET estado = 'fallido', terminado = ?, detalle = ? WHERE id = ?`,
		Ahora(), motivo, escaneoID)
	return err
}

// GuardarDescubrimiento guarda lo que vio un barrido: da de alta lo nuevo,
// actualiza lo conocido y marca como ausente lo que ya no contesto.
//
// Va entero en una transaccion: media foto de la red es peor que ninguna, porque
// las alertas de la fase 7 comparan contra la anterior y una foto a medias
// avisaria de equipos "desaparecidos" que solo se quedaron fuera del corte.
func (b *Base) GuardarDescubrimiento(ctx context.Context, escaneoID int64, profundo bool, equipos []EquipoDescubierto) (ResumenEscaneo, error) {
	resumen := ResumenEscaneo{EscaneoID: escaneoID, Vistos: len(equipos)}
	momento := Ahora()

	err := b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		vistos := make(map[int64]bool, len(equipos))

		for _, visto := range equipos {
			identidad := identidadDe(visto)
			equipoID, esNuevo, err := guardarEquipo(ctx, tx, identidad, visto, momento)
			if err != nil {
				return err
			}
			vistos[equipoID] = true
			if esNuevo {
				resumen.Nuevos++
			}

			if profundo {
				if err := guardarPuertos(ctx, tx, equipoID, visto.Puertos, momento); err != nil {
					return err
				}
			}

			_, err = tx.ExecContext(ctx, `
				INSERT INTO escaneo_equipos (escaneo_id, equipo_id, ip, nuevo)
				VALUES (?, ?, ?, ?)
				ON CONFLICT (escaneo_id, equipo_id) DO NOTHING`,
				escaneoID, equipoID, visto.IP, boolAEntero(esNuevo))
			if err != nil {
				return fmt.Errorf("no se pudo anotar el equipo en el escaneo: %w", err)
			}
		}

		// Lo que no contesto queda como ausente, no se borra: que un equipo se
		// apague no significa que deje de existir, y su historia es justamente
		// lo que despues permite avisar de que lleva tres dias sin aparecer.
		ausentes, err := marcarAusentes(ctx, tx, escaneoID, momento)
		if err != nil {
			return err
		}
		resumen.Ausentes = ausentes

		_, err = tx.ExecContext(ctx, `
			UPDATE escaneos
			   SET estado = 'terminado', terminado = ?, equipos_vistos = ?, equipos_nuevos = ?
			 WHERE id = ?`,
			momento, resumen.Vistos, resumen.Nuevos, escaneoID)
		if err != nil {
			return fmt.Errorf("no se pudo cerrar el escaneo: %w", err)
		}
		return nil
	})

	return resumen, err
}

func identidadDe(visto EquipoDescubierto) string {
	mac := strings.ToLower(strings.TrimSpace(visto.MAC))
	if mac != "" {
		return mac
	}
	// Sin MAC, la IP es lo unico que hay. Se marca con prefijo para que se vea
	// de un vistazo que ese equipo se identifica por algo que puede cambiar.
	return "ip:" + visto.IP
}

func guardarEquipo(ctx context.Context, tx *sql.Tx, identidad string, visto EquipoDescubierto, momento string) (int64, bool, error) {
	var equipoID int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM equipos WHERE identidad = ? AND estatus >= 0`, identidad).Scan(&equipoID)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		resultado, err := tx.ExecContext(ctx, `
			INSERT INTO equipos (identidad, ip, mac, fabricante, nombre, subred, metodo,
			                     presente, primera_vez, ultima_vez, estatus)
			VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?, 1)`,
			identidad, visto.IP, nuloSiVacio(visto.MAC), nuloSiVacio(visto.Fabricante),
			nuloSiVacio(visto.Nombre), nuloSiVacio(visto.Subred), visto.Metodo,
			momento, momento)
		if err != nil {
			return 0, false, fmt.Errorf("no se pudo guardar el equipo %s: %w", visto.IP, err)
		}
		nuevoID, _ := resultado.LastInsertId()
		return nuevoID, true, nil

	case err != nil:
		return 0, false, fmt.Errorf("no se pudo buscar el equipo %s: %w", visto.IP, err)
	}

	// COALESCE conserva lo que ya se sabia cuando este barrido no lo trae: un
	// escaneo rapido no debe borrar el nombre que encontro uno profundo.
	_, err = tx.ExecContext(ctx, `
		UPDATE equipos
		   SET ip = ?,
		       mac = COALESCE(NULLIF(?, ''), mac),
		       fabricante = COALESCE(NULLIF(?, ''), fabricante),
		       nombre = COALESCE(NULLIF(?, ''), nombre),
		       subred = COALESCE(NULLIF(?, ''), subred),
		       metodo = ?,
		       presente = 1,
		       ultima_vez = ?,
		       modificado = ?
		 WHERE id = ?`,
		visto.IP, visto.MAC, visto.Fabricante, visto.Nombre, visto.Subred,
		visto.Metodo, momento, momento, equipoID)
	if err != nil {
		return 0, false, fmt.Errorf("no se pudo actualizar el equipo %s: %w", visto.IP, err)
	}
	return equipoID, false, nil
}

func guardarPuertos(ctx context.Context, tx *sql.Tx, equipoID int64, puertos []PuertoDescubierto, momento string) error {
	for _, puerto := range puertos {
		protocolo := puerto.Protocolo
		if protocolo == "" {
			protocolo = "tcp"
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO puertos (equipo_id, numero, protocolo, servicio, banner,
			                     abierto, primera_vez, ultima_vez)
			VALUES (?, ?, ?, ?, ?, 1, ?, ?)
			ON CONFLICT (equipo_id, numero, protocolo) DO UPDATE SET
				servicio = COALESCE(NULLIF(excluded.servicio, ''), puertos.servicio),
				banner = COALESCE(NULLIF(excluded.banner, ''), puertos.banner),
				abierto = 1,
				ultima_vez = excluded.ultima_vez`,
			equipoID, puerto.Numero, protocolo, nuloSiVacio(puerto.Servicio),
			nuloSiVacio(puerto.Banner), momento, momento)
		if err != nil {
			return fmt.Errorf("no se pudo guardar el puerto %d: %w", puerto.Numero, err)
		}
	}

	// Un puerto que estaba abierto y ya no contesta se marca cerrado, pero no se
	// borra: "este equipo tenia abierto el 3389 y ya no" es informacion, y en la
	// fase 7 se vuelve alerta.
	//
	// Se cierra por LISTA de lo que se vio, no comparando marcas de tiempo: dos
	// escaneos dentro del mismo segundo tienen la misma marca y la comparacion
	// no cerraria nada. La lista siempre dice la verdad.
	argumentos := []any{equipoID}
	consulta := `UPDATE puertos SET abierto = 0 WHERE equipo_id = ? AND abierto = 1`
	if len(puertos) > 0 {
		consulta += ` AND (numero, protocolo) NOT IN (VALUES `
		for i, puerto := range puertos {
			if i > 0 {
				consulta += `, `
			}
			consulta += `(?, ?)`
			protocolo := puerto.Protocolo
			if protocolo == "" {
				protocolo = "tcp"
			}
			argumentos = append(argumentos, puerto.Numero, protocolo)
		}
		consulta += `)`
	}

	_, err := tx.ExecContext(ctx, consulta, argumentos...)
	return err
}

func marcarAusentes(ctx context.Context, tx *sql.Tx, escaneoID int64, momento string) (int, error) {
	resultado, err := tx.ExecContext(ctx, `
		UPDATE equipos
		   SET presente = 0, modificado = ?
		 WHERE estatus = 1
		   AND presente = 1
		   AND id NOT IN (SELECT equipo_id FROM escaneo_equipos WHERE escaneo_id = ?)`,
		momento, escaneoID)
	if err != nil {
		return 0, fmt.Errorf("no se pudieron marcar los equipos ausentes: %w", err)
	}
	ausentes, _ := resultado.RowsAffected()
	return int(ausentes), nil
}

// ListarEquipos devuelve los equipos de la red con sus puertos abiertos.
func (b *Base) ListarEquipos(ctx context.Context, soloPresentes bool) ([]Equipo, error) {
	consulta := `
		SELECT id, identidad, ip, COALESCE(mac, ''), COALESCE(fabricante, ''),
		       COALESCE(nombre, ''), COALESCE(alias, ''), COALESCE(tipo, ''),
		       COALESCE(subred, ''), COALESCE(metodo, ''), presente,
		       primera_vez, ultima_vez
		  FROM equipos
		 WHERE estatus = 1`
	if soloPresentes {
		consulta += ` AND presente = 1`
	}
	consulta += ` ORDER BY presente DESC, ip`

	filas, err := b.QueryContext(ctx, consulta)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar los equipos: %w", err)
	}
	defer filas.Close()

	equipos := []Equipo{}
	porID := map[int64]int{}
	for filas.Next() {
		var e Equipo
		var presente int
		if err := filas.Scan(&e.ID, &e.Identidad, &e.IP, &e.MAC, &e.Fabricante,
			&e.Nombre, &e.Alias, &e.Tipo, &e.Subred, &e.Metodo, &presente,
			&e.PrimeraVez, &e.UltimaVez); err != nil {
			return nil, err
		}
		e.Presente = presente == 1
		e.Puertos = []Puerto{}
		porID[e.ID] = len(equipos)
		equipos = append(equipos, e)
	}
	if err := filas.Err(); err != nil {
		return nil, err
	}
	if len(equipos) == 0 {
		return equipos, nil
	}

	// Los puertos se traen de una sola consulta y se reparten en memoria: una
	// consulta por equipo serian cientos de idas a la base para pintar una
	// pantalla.
	filasPuertos, err := b.QueryContext(ctx, `
		SELECT equipo_id, numero, protocolo, COALESCE(servicio, ''), COALESCE(banner, ''),
		       abierto, ultima_vez
		  FROM puertos
		 WHERE abierto = 1
		 ORDER BY numero`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar los puertos: %w", err)
	}
	defer filasPuertos.Close()

	for filasPuertos.Next() {
		var equipoID int64
		var p Puerto
		var abierto int
		if err := filasPuertos.Scan(&equipoID, &p.Numero, &p.Protocolo, &p.Servicio,
			&p.Banner, &abierto, &p.UltimaVez); err != nil {
			return nil, err
		}
		p.Abierto = abierto == 1
		if indice, hay := porID[equipoID]; hay {
			equipos[indice].Puertos = append(equipos[indice].Puertos, p)
		}
	}
	return equipos, filasPuertos.Err()
}

// PonerAlias le pone a un equipo el nombre que una persona quiera.
func (b *Base) PonerAlias(ctx context.Context, equipoID int64, alias string) error {
	resultado, err := b.ExecContext(ctx,
		`UPDATE equipos SET alias = ?, modificado = ? WHERE id = ? AND estatus = 1`,
		nuloSiVacio(strings.TrimSpace(alias)), Ahora(), equipoID)
	if err != nil {
		return fmt.Errorf("no se pudo guardar el nombre: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return ErrEquipoNoExiste
	}
	return nil
}

// ResumenDeRed cuenta lo que el catalogo necesita para el panel de inicio, sin
// que este tenga que abrir el archivo de cada red al pintarlo.
func (b *Base) ResumenDeRed(ctx context.Context) (equipos, presentes int, ultimoEscaneo string, err error) {
	err = b.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(presente), 0) FROM equipos WHERE estatus = 1`).
		Scan(&equipos, &presentes)
	if err != nil {
		return 0, 0, "", fmt.Errorf("no se pudo contar los equipos: %w", err)
	}

	var ultimo sql.NullString
	err = b.QueryRowContext(ctx,
		`SELECT MAX(terminado) FROM escaneos WHERE estado = 'terminado'`).Scan(&ultimo)
	if err != nil {
		return equipos, presentes, "", fmt.Errorf("no se pudo leer el ultimo escaneo: %w", err)
	}
	return equipos, presentes, ultimo.String, nil
}

// ActualizarResumen guarda en el catalogo el resumen de una red.
func (e *Enrutador) ActualizarResumen(ctx context.Context, clave string, equipos, presentes int, ultimoEscaneo string) error {
	_, err := e.Catalogo.ExecContext(ctx, `
		UPDATE redes
		   SET equipos = ?, equipos_presentes = ?, ultimo_escaneo = ?, resumen_al = ?
		 WHERE clave = ?`,
		equipos, presentes, nuloSiVacio(ultimoEscaneo), Ahora(), clave)
	if err != nil {
		return fmt.Errorf("no se pudo actualizar el resumen de la red: %w", err)
	}
	return nil
}

func nuloSiVacio(texto string) any {
	if strings.TrimSpace(texto) == "" {
		return nil
	}
	return texto
}
