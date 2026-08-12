package basedatos

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// ErrRedNoExiste lo devuelven las consultas cuando la red no esta o esta borrada.
var ErrRedNoExiste = errors.New("la red no existe")

// ErrNombreRepetido lo devuelve el alta cuando ya hay una red activa con ese
// nombre.
var ErrNombreRepetido = errors.New("ya existe una red con ese nombre")

// Red es una red registrada en el catalogo. Los campos de resumen se actualizan
// al terminar cada escaneo y existen para que el panel de inicio no tenga que
// abrir el archivo de cada red.
type Red struct {
	ID               int64   `json:"id"`
	Clave            string  `json:"clave"`
	Nombre           string  `json:"nombre"`
	Descripcion      string  `json:"descripcion"`
	Archivo          string  `json:"archivo"`
	Estatus          int     `json:"estatus"`
	Creada           string  `json:"creada"`
	Modificada       *string `json:"modificada"`
	Equipos          int     `json:"equipos"`
	EquiposPresentes int     `json:"equiposPresentes"`
	UltimoEscaneo    *string `json:"ultimoEscaneo"`
	AlertasAbiertas  int     `json:"alertasAbiertas"`

	// Agenda de barridos automaticos de esta instalacion.
	Programado            bool    `json:"programado"`
	PresenciaCadaSegundos int     `json:"presenciaCadaSegundos"`
	ProfundoCadaMinutos   int     `json:"profundoCadaMinutos"`
	ProximaPresencia      *string `json:"proximaPresencia"`
	ProximoProfundo       *string `json:"proximoProfundo"`
}

// columnasRed es la lista que leen todas las consultas de red, para que no se
// desincronicen entre si al agregar una columna.
const columnasRed = `
	id, clave, nombre, COALESCE(descripcion, ''), archivo, estatus,
	creada, modificada, equipos, equipos_presentes, ultimo_escaneo,
	alertas_abiertas, programado, presencia_cada_segundos, profundo_cada_minutos,
	proxima_presencia, proximo_profundo`

// leerRed llena una red desde una fila con las columnasRed.
func leerRed(destino interface {
	Scan(...any) error
}) (Red, error) {
	var r Red
	var programado int
	err := destino.Scan(&r.ID, &r.Clave, &r.Nombre, &r.Descripcion, &r.Archivo,
		&r.Estatus, &r.Creada, &r.Modificada, &r.Equipos, &r.EquiposPresentes,
		&r.UltimoEscaneo, &r.AlertasAbiertas, &programado, &r.PresenciaCadaSegundos,
		&r.ProfundoCadaMinutos, &r.ProximaPresencia, &r.ProximoProfundo)
	r.Programado = programado == 1
	return r, err
}

// ListarRedes devuelve las redes activas del catalogo, ordenadas por nombre.
func (e *Enrutador) ListarRedes(ctx context.Context) ([]Red, error) {
	filas, err := e.Catalogo.QueryContext(ctx,
		`SELECT `+columnasRed+` FROM redes WHERE estatus = 1 ORDER BY nombre`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar las redes: %w", err)
	}
	defer filas.Close()

	redes := []Red{}
	for filas.Next() {
		r, err := leerRed(filas)
		if err != nil {
			return nil, err
		}
		redes = append(redes, r)
	}
	return redes, filas.Err()
}

// BuscarRed devuelve una red activa por su clave.
func (e *Enrutador) BuscarRed(ctx context.Context, clave string) (Red, error) {
	fila := e.Catalogo.QueryRowContext(ctx,
		`SELECT `+columnasRed+` FROM redes WHERE clave = ? AND estatus >= 0`, clave)
	r, err := leerRed(fila)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrRedNoExiste
	}
	if err != nil {
		return r, fmt.Errorf("no se pudo leer la red %s: %w", clave, err)
	}
	return r, nil
}

// CrearRed da de alta una red: registra la ficha en el catalogo, crea su archivo
// SQLite, le aplica el esquema y guarda sus subredes.
//
// Si ya hubo una red con ese nombre y quedo borrada, se reactiva con su archivo
// original en vez de crear una nueva. Asi no se pierde el historico de un sitio
// por un borrado accidental, que es la razon del borrado suave.
func (e *Enrutador) CrearRed(ctx context.Context, nombre, descripcion string, subredes []string) (Red, error) {
	nombre = strings.TrimSpace(nombre)
	if nombre == "" {
		return Red{}, fmt.Errorf("la red necesita un nombre")
	}

	if reactivada, hay, err := e.reactivarRed(ctx, nombre, descripcion); err != nil {
		return Red{}, err
	} else if hay {
		return reactivada, nil
	}

	var repetida int
	if err := e.Catalogo.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM redes WHERE nombre = ? AND estatus = 1`, nombre).Scan(&repetida); err != nil {
		return Red{}, fmt.Errorf("no se pudo comprobar el nombre: %w", err)
	}
	if repetida > 0 {
		return Red{}, ErrNombreRepetido
	}

	clave, err := generarClave(nombre)
	if err != nil {
		return Red{}, err
	}
	archivo := clave + ".db"
	creada := Ahora()

	resultado, err := e.Catalogo.ExecContext(ctx, `
		INSERT INTO redes (clave, nombre, descripcion, archivo, estatus, creada)
		VALUES (?, ?, ?, ?, 1, ?)`, clave, nombre, descripcion, archivo, creada)
	if err != nil {
		return Red{}, fmt.Errorf("no se pudo registrar la red: %w", err)
	}
	id, _ := resultado.LastInsertId()

	// Si el archivo no se puede crear, la fila del catalogo no debe quedarse:
	// seria una red fantasma que aparece en la lista y no abre.
	if err := e.prepararArchivoDeRed(ctx, clave, nombre, descripcion, creada, subredes); err != nil {
		e.Catalogo.ExecContext(ctx, `DELETE FROM redes WHERE id = ?`, id)
		e.Olvidar(clave)
		os.Remove(e.ArchivoDeRed(clave))
		return Red{}, err
	}

	return e.BuscarRed(ctx, clave)
}

// reactivarRed revive una red borrada con el mismo nombre, conservando su
// archivo y todo su historico.
func (e *Enrutador) reactivarRed(ctx context.Context, nombre, descripcion string) (Red, bool, error) {
	var clave string
	err := e.Catalogo.QueryRowContext(ctx,
		`SELECT clave FROM redes WHERE nombre = ? AND estatus = -1 ORDER BY id DESC LIMIT 1`,
		nombre).Scan(&clave)
	if errors.Is(err, sql.ErrNoRows) {
		return Red{}, false, nil
	}
	if err != nil {
		return Red{}, false, fmt.Errorf("no se pudo buscar una red borrada: %w", err)
	}

	_, err = e.Catalogo.ExecContext(ctx,
		`UPDATE redes SET estatus = 1, descripcion = ?, modificada = ? WHERE clave = ?`,
		descripcion, Ahora(), clave)
	if err != nil {
		return Red{}, false, fmt.Errorf("no se pudo reactivar la red: %w", err)
	}

	red, err := e.BuscarRed(ctx, clave)
	return red, true, err
}

// prepararArchivoDeRed crea el .db de la red, le aplica el esquema, escribe su
// ficha autodescriptiva y guarda sus subredes.
func (e *Enrutador) prepararArchivoDeRed(ctx context.Context, clave, nombre, descripcion, creada string, subredes []string) error {
	return e.ConRed(ctx, clave, func(base *Base) error {
		return base.EnTransaccion(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO ficha (id, clave, nombre, descripcion, creada)
				VALUES (1, ?, ?, ?, ?)
				ON CONFLICT (id) DO UPDATE SET
					clave = excluded.clave,
					nombre = excluded.nombre,
					descripcion = excluded.descripcion,
					modificada = excluded.creada`,
				clave, nombre, descripcion, creada)
			if err != nil {
				return fmt.Errorf("no se pudo escribir la ficha de la red: %w", err)
			}

			for _, cidr := range subredes {
				cidr = strings.TrimSpace(cidr)
				if cidr == "" {
					continue
				}
				_, err := tx.ExecContext(ctx, `
					INSERT INTO subredes (cidr, escanear, estatus, creada)
					VALUES (?, 1, 1, ?)
					ON CONFLICT DO NOTHING`, cidr, creada)
				if err != nil {
					return fmt.Errorf("no se pudo guardar la subred %s: %w", cidr, err)
				}
			}
			return nil
		})
	})
}

// BorrarRed marca la red como borrada y cierra su archivo. El .db NO se elimina:
// queda para respaldo y para poder reactivar la red por su nombre.
func (e *Enrutador) BorrarRed(ctx context.Context, clave string) error {
	resultado, err := e.Catalogo.ExecContext(ctx,
		`UPDATE redes SET estatus = -1, modificada = ? WHERE clave = ? AND estatus >= 0`,
		Ahora(), clave)
	if err != nil {
		return fmt.Errorf("no se pudo borrar la red: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return ErrRedNoExiste
	}
	e.Olvidar(clave)
	return nil
}

// RenombrarRed cambia el nombre visible de la red. La clave y el archivo no
// cambian nunca: renombrar un archivo abierto es justo lo que se quiso evitar.
func (e *Enrutador) RenombrarRed(ctx context.Context, clave, nombre, descripcion string) (Red, error) {
	nombre = strings.TrimSpace(nombre)
	if nombre == "" {
		return Red{}, fmt.Errorf("la red necesita un nombre")
	}

	var repetida int
	if err := e.Catalogo.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM redes WHERE nombre = ? AND clave <> ? AND estatus = 1`,
		nombre, clave).Scan(&repetida); err != nil {
		return Red{}, fmt.Errorf("no se pudo comprobar el nombre: %w", err)
	}
	if repetida > 0 {
		return Red{}, ErrNombreRepetido
	}

	momento := Ahora()
	resultado, err := e.Catalogo.ExecContext(ctx,
		`UPDATE redes SET nombre = ?, descripcion = ?, modificada = ? WHERE clave = ? AND estatus = 1`,
		nombre, descripcion, momento, clave)
	if err != nil {
		return Red{}, fmt.Errorf("no se pudo renombrar la red: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return Red{}, ErrRedNoExiste
	}

	// La ficha de dentro del archivo tambien se actualiza: el archivo tiene que
	// seguir siendo autodescriptivo si alguien lo copia a otra instalacion.
	err = e.ConRed(ctx, clave, func(base *Base) error {
		_, err := base.ExecContext(ctx,
			`UPDATE ficha SET nombre = ?, descripcion = ?, modificada = ? WHERE id = 1`,
			nombre, descripcion, momento)
		return err
	})
	if err != nil {
		return Red{}, fmt.Errorf("no se pudo actualizar la ficha de la red: %w", err)
	}

	return e.BuscarRed(ctx, clave)
}

// generarClave arma el nombre de archivo de una red: parte legible del nombre
// mas cuatro caracteres al azar.
//
// Se usa la clave y no el nombre porque el nombre lleva acentos y espacios, se
// repite entre sitios y cambia con el tiempo, y el nombre del archivo no puede
// depender de nada de eso.
func generarClave(nombre string) (string, error) {
	base := aClaveLegible(nombre)
	if base == "" {
		base = "red"
	}
	if len(base) > 24 {
		base = strings.Trim(base[:24], "-")
	}

	azar := make([]byte, 2)
	if _, err := rand.Read(azar); err != nil {
		return "", fmt.Errorf("no se pudo generar la clave de la red: %w", err)
	}
	return base + "-" + hex.EncodeToString(azar), nil
}

// aClaveLegible pasa "Sucursal Ñuñoa 2" a "sucursal-nunoa-2".
func aClaveLegible(texto string) string {
	sinAcentos := transform.Chain(
		norm.NFD,
		runes.Remove(runes.In(unicode.Mn)),
		norm.NFC,
	)
	limpio, _, err := transform.String(sinAcentos, texto)
	if err != nil {
		limpio = texto
	}

	var salida strings.Builder
	guionPendiente := false
	for _, letra := range strings.ToLower(limpio) {
		switch {
		case letra >= 'a' && letra <= 'z', letra >= '0' && letra <= '9':
			if guionPendiente && salida.Len() > 0 {
				salida.WriteByte('-')
			}
			guionPendiente = false
			salida.WriteRune(letra)
		default:
			guionPendiente = true
		}
	}
	return salida.String()
}
