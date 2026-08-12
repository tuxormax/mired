package basedatos

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// ErrSesionInvalida la devuelve la busqueda cuando el token no existe o ya
// expiro.
var ErrSesionInvalida = errors.New("la sesion no es valida")

// Sesion es una entrada activa a MiRed.
type Sesion struct {
	ID        string
	UsuarioID int64
	Creada    string
	UltimoUso string
	Expira    string
}

// CrearSesion genera un token nuevo para el usuario.
func (e *Enrutador) CrearSesion(ctx context.Context, usuarioID int64, duracion time.Duration, direccion, agente string) (Sesion, error) {
	token, err := generarToken()
	if err != nil {
		return Sesion{}, err
	}

	ahora := time.Now()
	sesion := Sesion{
		ID:        token,
		UsuarioID: usuarioID,
		Creada:    ahora.Format(time.RFC3339),
		UltimoUso: ahora.Format(time.RFC3339),
		Expira:    ahora.Add(duracion).Format(time.RFC3339),
	}

	_, err = e.Catalogo.ExecContext(ctx, `
		INSERT INTO sesiones (id, usuario_id, creada, ultimo_uso, expira, direccion, agente)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sesion.ID, sesion.UsuarioID, sesion.Creada, sesion.UltimoUso, sesion.Expira,
		direccion, agente)
	if err != nil {
		return Sesion{}, fmt.Errorf("no se pudo crear la sesion: %w", err)
	}
	return sesion, nil
}

// BuscarSesion valida un token y devuelve a quien pertenece. De paso renueva la
// vigencia: la sesion cuenta desde el ultimo uso, no desde que se entro, para
// que a nadie se le caiga la pantalla trabajando.
func (e *Enrutador) BuscarSesion(ctx context.Context, token string, duracion time.Duration) (Usuario, error) {
	var usuarioID int64
	var expira string
	err := e.Catalogo.QueryRowContext(ctx,
		`SELECT usuario_id, expira FROM sesiones WHERE id = ?`, token).
		Scan(&usuarioID, &expira)
	if errors.Is(err, sql.ErrNoRows) {
		return Usuario{}, ErrSesionInvalida
	}
	if err != nil {
		return Usuario{}, fmt.Errorf("no se pudo leer la sesion: %w", err)
	}

	vencimiento, err := time.Parse(time.RFC3339, expira)
	if err != nil || time.Now().After(vencimiento) {
		e.BorrarSesion(ctx, token)
		return Usuario{}, ErrSesionInvalida
	}

	ahora := time.Now()
	_, err = e.Catalogo.ExecContext(ctx,
		`UPDATE sesiones SET ultimo_uso = ?, expira = ? WHERE id = ?`,
		ahora.Format(time.RFC3339), ahora.Add(duracion).Format(time.RFC3339), token)
	if err != nil {
		return Usuario{}, fmt.Errorf("no se pudo renovar la sesion: %w", err)
	}

	usuario, err := e.BuscarUsuarioPorID(ctx, usuarioID)
	if errors.Is(err, ErrUsuarioNoExiste) {
		// El usuario se borro o se desactivo con la sesion abierta.
		e.BorrarSesion(ctx, token)
		return Usuario{}, ErrSesionInvalida
	}
	return usuario, err
}

// BorrarSesion cierra una sesion.
func (e *Enrutador) BorrarSesion(ctx context.Context, token string) error {
	_, err := e.Catalogo.ExecContext(ctx, `DELETE FROM sesiones WHERE id = ?`, token)
	return err
}

// LimpiarSesiones borra las sesiones vencidas. El servidor la llama cada tanto
// para que la tabla no crezca sola.
func (e *Enrutador) LimpiarSesiones(ctx context.Context) error {
	_, err := e.Catalogo.ExecContext(ctx,
		`DELETE FROM sesiones WHERE expira < ?`, time.Now().Format(time.RFC3339))
	return err
}

// generarToken produce 32 bytes al azar en hexadecimal. Es un token opaco: no
// lleva datos dentro, asi que no hay nada que falsificar ni que caduque solo.
func generarToken() (string, error) {
	crudo := make([]byte, 32)
	if _, err := rand.Read(crudo); err != nil {
		return "", fmt.Errorf("no se pudo generar el token de sesion: %w", err)
	}
	return hex.EncodeToString(crudo), nil
}
