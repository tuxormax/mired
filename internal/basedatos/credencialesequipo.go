package basedatos

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/tuxormax/mired/internal/secreto"
)

// Las credenciales de cada equipo, guardadas donde se buscan: en el equipo.
//
// El caso es el de cualquier red que uno administra: hay que entrar al panel del
// modem, del switch o de la antena, y la clave esta en un papel, en una nota del
// telefono, o en la cabeza de alguien que hoy no vino.
//
// Tres reglas que sostienen esto y **no se negocian**:
//
//  1. La clave se guarda **cifrada**, y la llave del cifrado no vive con los
//     datos: asi un respaldo de la base, copiado a cualquier lado, no se abre.
//  2. La clave **no viaja en el listado de equipos**. Se manda el usuario y la
//     direccion del panel —que es lo que se quiere ver de un vistazo— y la clave
//     solo cuando alguien la pide expresamente.
//  3. **Nunca sale en un mapa exportado.** Un PNG con las claves de la red
//     adentro, mandado por correo, es la peor forma de perderlas.

// ErrCredencialNoExiste la devuelven las operaciones sobre una que ya no esta.
var ErrCredencialNoExiste = errors.New("ese equipo no tiene esa credencial guardada")

// TiposDeCredencial son los usos que se pueden guardar. Es una lista cerrada:
// texto libre aqui acabaria con "web", "Web", "panel" y "pagina" conviviendo.
var TiposDeCredencial = []string{"web", "ssh", "consola", "app", "otro"}

// CredencialEquipo es como se entra a un aparato.
type CredencialEquipo struct {
	ID       int64  `json:"id"`
	EquipoID int64  `json:"equipoId"`
	Tipo     string `json:"tipo"`
	Usuario  string `json:"usuario"`

	// Clave solo viaja cuando alguien la pide. En los listados va vacia y
	// TieneClave dice si hay algo guardado.
	Clave      string `json:"clave,omitempty"`
	TieneClave bool   `json:"tieneClave"`

	Direccion  string `json:"direccion,omitempty"`
	Notas      string `json:"notas,omitempty"`
	Creada     string `json:"creada"`
	Modificada string `json:"modificada,omitempty"`
}

// GuardarCredencialEquipo guarda o actualiza como se entra a un aparato.
//
// Una clave vacia NO borra la que ya estaba: en un formulario donde la clave se
// muestra oculta, dejarla en blanco significa "no la toques", no "quitala". Para
// quitarla esta BorrarCredencialEquipo.
func (b *Base) GuardarCredencialEquipo(ctx context.Context, caja *secreto.Caja,
	credencial CredencialEquipo) (CredencialEquipo, error) {
	if credencial.EquipoID <= 0 {
		return CredencialEquipo{}, errors.New("hace falta decir de que equipo es la credencial")
	}
	credencial.Tipo = strings.ToLower(strings.TrimSpace(credencial.Tipo))
	if credencial.Tipo == "" {
		credencial.Tipo = "web"
	}
	if !esTipoDeCredencial(credencial.Tipo) {
		return CredencialEquipo{}, fmt.Errorf("no se guardan credenciales de tipo %q", credencial.Tipo)
	}

	guardada := ""
	if credencial.Clave != "" {
		var err error
		guardada, err = caja.Cifrar(credencial.Clave)
		if err != nil {
			return CredencialEquipo{}, fmt.Errorf("no se pudo cifrar la clave: %w", err)
		}
	}

	momento := Ahora()
	err := b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		var existe int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM equipos WHERE id = ? AND estatus = 1`,
			credencial.EquipoID).Scan(&existe); err != nil {
			return err
		}
		if existe == 0 {
			return ErrEquipoNoExiste
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO credenciales_equipo (equipo_id, tipo, usuario, clave, direccion, notas, creada)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (equipo_id, tipo) DO UPDATE SET
				usuario    = excluded.usuario,
				-- La clave vacia conserva la que habia: en el formulario, dejarla
				-- en blanco significa "no la toques".
				clave      = COALESCE(NULLIF(excluded.clave, ''), credenciales_equipo.clave),
				direccion  = excluded.direccion,
				notas      = excluded.notas,
				modificada = excluded.creada`,
			credencial.EquipoID, credencial.Tipo, nuloSiVacio(credencial.Usuario),
			nuloSiVacio(guardada), nuloSiVacio(credencial.Direccion),
			nuloSiVacio(credencial.Notas), momento)
		return err
	})
	if err != nil {
		return CredencialEquipo{}, err
	}

	return b.CredencialDeEquipo(ctx, caja, credencial.EquipoID, credencial.Tipo, false)
}

// CredencialDeEquipo devuelve una credencial. Con `conClave` en falso, la clave
// va vacia: eso es lo que se manda a las pantallas.
func (b *Base) CredencialDeEquipo(ctx context.Context, caja *secreto.Caja,
	equipoID int64, tipo string, conClave bool) (CredencialEquipo, error) {
	var credencial CredencialEquipo
	var clave, modificada sql.NullString

	err := b.QueryRowContext(ctx, `
		SELECT id, equipo_id, tipo, COALESCE(usuario, ''), clave,
		       COALESCE(direccion, ''), COALESCE(notas, ''), creada, modificada
		  FROM credenciales_equipo
		 WHERE equipo_id = ? AND tipo = ?`, equipoID, tipo).
		Scan(&credencial.ID, &credencial.EquipoID, &credencial.Tipo, &credencial.Usuario,
			&clave, &credencial.Direccion, &credencial.Notas, &credencial.Creada, &modificada)
	if errors.Is(err, sql.ErrNoRows) {
		return CredencialEquipo{}, ErrCredencialNoExiste
	}
	if err != nil {
		return CredencialEquipo{}, fmt.Errorf("no se pudo leer la credencial: %w", err)
	}

	credencial.Modificada = modificada.String
	credencial.TieneClave = clave.Valid && clave.String != ""
	if conClave && credencial.TieneClave {
		abierta, err := caja.Descifrar(clave.String)
		if err != nil {
			return CredencialEquipo{}, err
		}
		credencial.Clave = abierta
	}
	return credencial, nil
}

// CredencialesDeRed devuelve todas las credenciales de la red, SIN claves.
//
// Es lo que la lista de equipos y el mapa necesitan: poder decir "este aparato
// tiene credencial guardada, y el usuario es admin" sin repartir claves por
// pantallas que no las pidieron.
func (b *Base) CredencialesDeRed(ctx context.Context) (map[int64][]CredencialEquipo, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT id, equipo_id, tipo, COALESCE(usuario, ''), clave,
		       COALESCE(direccion, ''), COALESCE(notas, ''), creada
		  FROM credenciales_equipo
		 ORDER BY equipo_id, tipo`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar las credenciales: %w", err)
	}
	defer filas.Close()

	porEquipo := map[int64][]CredencialEquipo{}
	for filas.Next() {
		var credencial CredencialEquipo
		var clave sql.NullString
		if err := filas.Scan(&credencial.ID, &credencial.EquipoID, &credencial.Tipo,
			&credencial.Usuario, &clave, &credencial.Direccion, &credencial.Notas,
			&credencial.Creada); err != nil {
			return nil, err
		}
		credencial.TieneClave = clave.Valid && clave.String != ""
		porEquipo[credencial.EquipoID] = append(porEquipo[credencial.EquipoID], credencial)
	}
	return porEquipo, filas.Err()
}

// BorrarCredencialEquipo quita una credencial guardada.
func (b *Base) BorrarCredencialEquipo(ctx context.Context, id int64) error {
	resultado, err := b.ExecContext(ctx, `DELETE FROM credenciales_equipo WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("no se pudo borrar la credencial: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return ErrCredencialNoExiste
	}
	return nil
}

func esTipoDeCredencial(tipo string) bool {
	for _, valido := range TiposDeCredencial {
		if tipo == valido {
			return true
		}
	}
	return false
}

// TodasLasCredencialesConClave devuelve las credenciales de la red **con sus
// contrasenas en claro**.
//
// Existe para una sola cosa: exportar la red. El usuario lo pidio asi —«si se
// exporta el mapa de la red tambien va la contrasena, no importa que se vea»—
// porque una instalacion documentada a medias no sirve para mudarla a otro
// equipo ni para entregarsela a nadie.
//
// **Es la unica lectura masiva de secretos que hay**, y por eso: no la usa
// ningun listado, solo la exportacion, y quien la llama deja constancia en la
// bitacora. Todo lo demas manda las credenciales SIN clave.
func (b *Base) TodasLasCredencialesConClave(ctx context.Context,
	caja *secreto.Caja) ([]CredencialEquipo, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT id, equipo_id, tipo, COALESCE(usuario, ''), clave,
		       COALESCE(direccion, ''), COALESCE(notas, ''), creada
		  FROM credenciales_equipo
		 ORDER BY equipo_id, tipo`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer las credenciales: %w", err)
	}
	defer filas.Close()

	credenciales := []CredencialEquipo{}
	for filas.Next() {
		var c CredencialEquipo
		var guardada sql.NullString
		if err := filas.Scan(&c.ID, &c.EquipoID, &c.Tipo, &c.Usuario, &guardada,
			&c.Direccion, &c.Notas, &c.Creada); err != nil {
			return nil, err
		}
		c.TieneClave = guardada.Valid && guardada.String != ""
		if c.TieneClave && caja != nil {
			// Una clave que no se puede descifrar NO tumba la exportacion: se
			// deja vacia y las demas salen. Perder el archivo entero por una
			// fila seria peor que entregarlo con un hueco.
			if claro, err := caja.Descifrar(guardada.String); err == nil {
				c.Clave = claro
			}
		}
		credenciales = append(credenciales, c)
	}
	return credenciales, filas.Err()
}
