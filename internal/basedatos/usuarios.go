package basedatos

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrUsuarioNoExiste lo devuelven las consultas cuando el usuario no esta o esta
// borrado.
var ErrUsuarioNoExiste = errors.New("el usuario no existe")

// ErrUsuarioRepetido lo devuelve el alta cuando ya hay alguien con ese usuario.
var ErrUsuarioRepetido = errors.New("ya existe un usuario con ese nombre de acceso")

// Usuario es quien entra a MiRed. La clave nunca sale de la base: el campo se
// llena solo al comprobarla.
type Usuario struct {
	ID           int64   `json:"id"`
	Usuario      string  `json:"usuario"`
	Nombre       string  `json:"nombre"`
	Correo       string  `json:"correo"`
	Superadmin   bool    `json:"superadmin"`
	Estatus      int     `json:"estatus"`
	Creado       string  `json:"creado"`
	UltimoAcceso *string `json:"ultimoAcceso"`

	// Clave es el hash bcrypt. No se serializa nunca a JSON.
	Clave string `json:"-"`
}

// Nivel de acceso de un usuario sobre una red.
const (
	NivelLectura    = "lectura"
	NivelEscritura  = "escritura"
	NivelAdministra = "administra"
)

// CrearUsuario da de alta a alguien. La clave llega ya hasheada: este paquete
// guarda datos, no decide como se protegen.
//
// Si existe un usuario borrado con el mismo nombre de acceso, se reactiva con la
// clave nueva en vez de fallar por el indice unico.
func (e *Enrutador) CrearUsuario(ctx context.Context, usuario, nombre, claveHash, correo string, superadmin bool) (Usuario, error) {
	usuario = strings.ToLower(strings.TrimSpace(usuario))
	nombre = strings.TrimSpace(nombre)
	if usuario == "" || nombre == "" || claveHash == "" {
		return Usuario{}, fmt.Errorf("faltan datos del usuario")
	}

	var idBorrado int64
	err := e.Catalogo.QueryRowContext(ctx,
		`SELECT id FROM usuarios WHERE usuario = ? AND estatus = -1`, usuario).Scan(&idBorrado)
	switch {
	case err == nil:
		_, err = e.Catalogo.ExecContext(ctx, `
			UPDATE usuarios
			   SET nombre = ?, clave = ?, correo = ?, superadmin = ?, estatus = 1, modificado = ?
			 WHERE id = ?`,
			nombre, claveHash, correo, boolAEntero(superadmin), Ahora(), idBorrado)
		if err != nil {
			return Usuario{}, fmt.Errorf("no se pudo reactivar el usuario: %w", err)
		}
		return e.BuscarUsuarioPorID(ctx, idBorrado)
	case !errors.Is(err, sql.ErrNoRows):
		return Usuario{}, fmt.Errorf("no se pudo comprobar el usuario: %w", err)
	}

	var repetido int
	if err := e.Catalogo.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM usuarios WHERE usuario = ? AND estatus >= 0`, usuario).Scan(&repetido); err != nil {
		return Usuario{}, fmt.Errorf("no se pudo comprobar el usuario: %w", err)
	}
	if repetido > 0 {
		return Usuario{}, ErrUsuarioRepetido
	}

	resultado, err := e.Catalogo.ExecContext(ctx, `
		INSERT INTO usuarios (usuario, nombre, clave, correo, superadmin, estatus, creado)
		VALUES (?, ?, ?, ?, ?, 1, ?)`,
		usuario, nombre, claveHash, correo, boolAEntero(superadmin), Ahora())
	if err != nil {
		return Usuario{}, fmt.Errorf("no se pudo crear el usuario: %w", err)
	}
	id, _ := resultado.LastInsertId()
	return e.BuscarUsuarioPorID(ctx, id)
}

// BuscarUsuario devuelve un usuario activo por su nombre de acceso, con su hash
// de clave, para poder comprobar la contrasena.
func (e *Enrutador) BuscarUsuario(ctx context.Context, usuario string) (Usuario, error) {
	return e.leerUsuario(ctx,
		`WHERE usuario = ? AND estatus = 1`, strings.ToLower(strings.TrimSpace(usuario)))
}

// BuscarUsuarioPorID devuelve un usuario activo por su identificador.
func (e *Enrutador) BuscarUsuarioPorID(ctx context.Context, id int64) (Usuario, error) {
	return e.leerUsuario(ctx, `WHERE id = ? AND estatus = 1`, id)
}

func (e *Enrutador) leerUsuario(ctx context.Context, condicion string, argumentos ...any) (Usuario, error) {
	consulta := `
		SELECT id, usuario, nombre, clave, COALESCE(correo, ''), superadmin,
		       estatus, creado, ultimo_acceso
		  FROM usuarios ` + condicion

	var u Usuario
	var superadmin int
	err := e.Catalogo.QueryRowContext(ctx, consulta, argumentos...).
		Scan(&u.ID, &u.Usuario, &u.Nombre, &u.Clave, &u.Correo, &superadmin,
			&u.Estatus, &u.Creado, &u.UltimoAcceso)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrUsuarioNoExiste
	}
	if err != nil {
		return u, fmt.Errorf("no se pudo leer el usuario: %w", err)
	}
	u.Superadmin = superadmin == 1
	return u, nil
}

// ListarUsuarios devuelve los usuarios activos, ordenados por nombre.
func (e *Enrutador) ListarUsuarios(ctx context.Context) ([]Usuario, error) {
	filas, err := e.Catalogo.QueryContext(ctx, `
		SELECT id, usuario, nombre, COALESCE(correo, ''), superadmin, estatus,
		       creado, ultimo_acceso
		  FROM usuarios
		 WHERE estatus = 1
		 ORDER BY nombre`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar los usuarios: %w", err)
	}
	defer filas.Close()

	usuarios := []Usuario{}
	for filas.Next() {
		var u Usuario
		var superadmin int
		if err := filas.Scan(&u.ID, &u.Usuario, &u.Nombre, &u.Correo, &superadmin,
			&u.Estatus, &u.Creado, &u.UltimoAcceso); err != nil {
			return nil, err
		}
		u.Superadmin = superadmin == 1
		usuarios = append(usuarios, u)
	}
	return usuarios, filas.Err()
}

// HayUsuarios dice si el catalogo ya tiene alguien dado de alta. Se usa en el
// arranque para sembrar el superadministrador solo la primera vez.
func (e *Enrutador) HayUsuarios(ctx context.Context) (bool, error) {
	var cuantos int
	if err := e.Catalogo.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM usuarios WHERE estatus >= 0`).Scan(&cuantos); err != nil {
		return false, fmt.Errorf("no se pudieron contar los usuarios: %w", err)
	}
	return cuantos > 0, nil
}

// MarcarAcceso deja constancia de la ultima entrada del usuario.
func (e *Enrutador) MarcarAcceso(ctx context.Context, id int64) error {
	_, err := e.Catalogo.ExecContext(ctx,
		`UPDATE usuarios SET ultimo_acceso = ? WHERE id = ?`, Ahora(), id)
	return err
}

// CambiarClave guarda un hash nuevo.
func (e *Enrutador) CambiarClave(ctx context.Context, id int64, claveHash string) error {
	resultado, err := e.Catalogo.ExecContext(ctx,
		`UPDATE usuarios SET clave = ?, modificado = ? WHERE id = ? AND estatus = 1`,
		claveHash, Ahora(), id)
	if err != nil {
		return fmt.Errorf("no se pudo cambiar la clave: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return ErrUsuarioNoExiste
	}
	return nil
}

// BorrarUsuario aplica borrado suave. Al superadministrador no se le borra: es
// el unico que puede volver a entrar si todo lo demas se rompe.
func (e *Enrutador) BorrarUsuario(ctx context.Context, id int64) error {
	resultado, err := e.Catalogo.ExecContext(ctx,
		`UPDATE usuarios SET estatus = -1, modificado = ? WHERE id = ? AND superadmin = 0 AND estatus >= 0`,
		Ahora(), id)
	if err != nil {
		return fmt.Errorf("no se pudo borrar el usuario: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return ErrUsuarioNoExiste
	}
	_, err = e.Catalogo.ExecContext(ctx, `DELETE FROM sesiones WHERE usuario_id = ?`, id)
	return err
}

// RedesDeUsuario devuelve las claves de las redes que ese usuario puede ver. El
// superadministrador ve todas.
func (e *Enrutador) RedesDeUsuario(ctx context.Context, usuario Usuario) (map[string]string, error) {
	permisos := map[string]string{}

	if usuario.Superadmin {
		redes, err := e.ListarRedes(ctx)
		if err != nil {
			return nil, err
		}
		for _, red := range redes {
			permisos[red.Clave] = NivelAdministra
		}
		return permisos, nil
	}

	filas, err := e.Catalogo.QueryContext(ctx, `
		SELECT r.clave, p.nivel
		  FROM permisos_red p
		  JOIN redes r ON r.id = p.red_id
		 WHERE p.usuario_id = ? AND r.estatus = 1`, usuario.ID)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer los permisos: %w", err)
	}
	defer filas.Close()

	for filas.Next() {
		var clave, nivel string
		if err := filas.Scan(&clave, &nivel); err != nil {
			return nil, err
		}
		permisos[clave] = nivel
	}
	return permisos, filas.Err()
}

// OtorgarPermiso da o cambia el acceso de un usuario a una red.
func (e *Enrutador) OtorgarPermiso(ctx context.Context, usuarioID int64, claveRed, nivel string) error {
	if nivel != NivelLectura && nivel != NivelEscritura && nivel != NivelAdministra {
		return fmt.Errorf("nivel de permiso invalido: %s", nivel)
	}
	red, err := e.BuscarRed(ctx, claveRed)
	if err != nil {
		return err
	}
	_, err = e.Catalogo.ExecContext(ctx, `
		INSERT INTO permisos_red (usuario_id, red_id, nivel, otorgado)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (usuario_id, red_id) DO UPDATE SET nivel = excluded.nivel`,
		usuarioID, red.ID, nivel, Ahora())
	if err != nil {
		return fmt.Errorf("no se pudo otorgar el permiso: %w", err)
	}
	return nil
}

// QuitarPermiso retira el acceso de un usuario a una red.
func (e *Enrutador) QuitarPermiso(ctx context.Context, usuarioID int64, claveRed string) error {
	red, err := e.BuscarRed(ctx, claveRed)
	if err != nil {
		return err
	}
	_, err = e.Catalogo.ExecContext(ctx,
		`DELETE FROM permisos_red WHERE usuario_id = ? AND red_id = ?`, usuarioID, red.ID)
	return err
}

func boolAEntero(valor bool) int {
	if valor {
		return 1
	}
	return 0
}
