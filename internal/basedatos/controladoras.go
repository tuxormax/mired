package basedatos

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrControladoraRepetida la devuelve el alta cuando ya hay una con ese nombre.
var ErrControladoraRepetida = errors.New("ya existe una controladora con ese nombre")

// Controladora es una controladora WiFi (UniFi y compatibles).
//
// Vive en el catalogo y no en cada red por la misma razon que las credenciales
// SNMP: una controladora suele atender varios sitios a la vez.
type Controladora struct {
	ID           int64  `json:"id"`
	Nombre       string `json:"nombre"`
	Tipo         string `json:"tipo"`
	URL          string `json:"url"`
	Usuario      string `json:"usuario"`
	Clave        string `json:"clave,omitempty"`
	Sitio        string `json:"sitio"`
	VerificarTLS bool   `json:"verificarTls"`
	Creada       string `json:"creada"`
	UltimoExito  string `json:"ultimoExito,omitempty"`
	UltimoError  string `json:"ultimoError,omitempty"`
}

// SinSecretos quita la clave antes de mandar la controladora a la interfaz.
func (c Controladora) SinSecretos() Controladora {
	c.Clave = ""
	return c
}

// ListarControladoras devuelve las activas, CON su clave. Para la interfaz se
// pasa antes por SinSecretos.
func (b *Base) ListarControladoras(ctx context.Context) ([]Controladora, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT id, nombre, tipo, url, usuario, clave, sitio, verificar_tls, creada,
		       COALESCE(ultimo_exito, ''), COALESCE(ultimo_error, '')
		  FROM controladoras
		 WHERE estatus = 1
		 ORDER BY nombre`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar las controladoras: %w", err)
	}
	defer filas.Close()

	controladoras := []Controladora{}
	for filas.Next() {
		var c Controladora
		var verificar int
		if err := filas.Scan(&c.ID, &c.Nombre, &c.Tipo, &c.URL, &c.Usuario, &c.Clave,
			&c.Sitio, &verificar, &c.Creada, &c.UltimoExito, &c.UltimoError); err != nil {
			return nil, err
		}
		c.VerificarTLS = verificar == 1
		controladoras = append(controladoras, c)
	}
	return controladoras, filas.Err()
}

// CrearControladora da de alta una controladora. Si habia una borrada con el
// mismo nombre, se reactiva con los datos nuevos.
func (b *Base) CrearControladora(ctx context.Context, c Controladora) (Controladora, error) {
	c.Nombre = strings.TrimSpace(c.Nombre)
	c.URL = strings.TrimRight(strings.TrimSpace(c.URL), "/")
	c.Usuario = strings.TrimSpace(c.Usuario)
	c.Sitio = strings.TrimSpace(c.Sitio)

	if c.Nombre == "" {
		return Controladora{}, fmt.Errorf("la controladora necesita un nombre")
	}
	if c.Tipo == "" {
		c.Tipo = "unifi"
	}
	if c.Tipo != "unifi" {
		return Controladora{}, fmt.Errorf("tipo de controladora no soportado: %s", c.Tipo)
	}
	// Se valida aqui y no solo en el formulario: una direccion mal escrita se
	// descubriria dentro de un mes, cuando alguien note que el WiFi no sale en el
	// mapa y nadie recuerde que se dio de alta esa controladora.
	direccion, err := url.Parse(c.URL)
	if err != nil || direccion.Host == "" ||
		(direccion.Scheme != "http" && direccion.Scheme != "https") {
		return Controladora{}, fmt.Errorf(
			"la direccion debe ser como https://192.168.1.10:8443 y se capturo: %s", c.URL)
	}
	if c.Usuario == "" || c.Clave == "" {
		return Controladora{}, fmt.Errorf("la controladora necesita usuario y clave")
	}
	if c.Sitio == "" {
		c.Sitio = "default"
	}

	var borradaID int64
	err = b.QueryRowContext(ctx,
		`SELECT id FROM controladoras WHERE nombre = ? AND estatus = -1`, c.Nombre).Scan(&borradaID)
	if err == nil {
		_, err = b.ExecContext(ctx, `
			UPDATE controladoras
			   SET tipo = ?, url = ?, usuario = ?, clave = ?, sitio = ?, verificar_tls = ?,
			       estatus = 1, modificada = ?, ultimo_exito = NULL, ultimo_error = NULL
			 WHERE id = ?`,
			c.Tipo, c.URL, c.Usuario, c.Clave, c.Sitio, boolAEntero(c.VerificarTLS),
			Ahora(), borradaID)
		if err != nil {
			return Controladora{}, fmt.Errorf("no se pudo reactivar la controladora: %w", err)
		}
		c.ID = borradaID
		return c, nil
	}

	var repetida int
	if err := b.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM controladoras WHERE nombre = ? AND estatus >= 0`,
		c.Nombre).Scan(&repetida); err != nil {
		return Controladora{}, fmt.Errorf("no se pudo comprobar el nombre: %w", err)
	}
	if repetida > 0 {
		return Controladora{}, ErrControladoraRepetida
	}

	resultado, err := b.ExecContext(ctx, `
		INSERT INTO controladoras (nombre, tipo, url, usuario, clave, sitio,
		                           verificar_tls, estatus, creada)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)`,
		c.Nombre, c.Tipo, c.URL, c.Usuario, c.Clave, c.Sitio,
		boolAEntero(c.VerificarTLS), Ahora())
	if err != nil {
		return Controladora{}, fmt.Errorf("no se pudo crear la controladora: %w", err)
	}
	c.ID, _ = resultado.LastInsertId()
	return c, nil
}

// BorrarControladora aplica borrado suave.
func (b *Base) BorrarControladora(ctx context.Context, id int64) error {
	resultado, err := b.ExecContext(ctx,
		`UPDATE controladoras SET estatus = -1, modificada = ? WHERE id = ? AND estatus >= 0`,
		Ahora(), id)
	if err != nil {
		return fmt.Errorf("no se pudo borrar la controladora: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return fmt.Errorf("esa controladora no existe")
	}
	return nil
}

// AnotarConsultaAControladora deja constancia de si contesto o no.
//
// Que una controladora lleve dias sin contestar es un dato, no un silencio: sin
// esto, el WiFi desapareceria del mapa poco a poco y nadie sabria por que.
func (b *Base) AnotarConsultaAControladora(ctx context.Context, id int64, problema error) error {
	if problema == nil {
		_, err := b.ExecContext(ctx,
			`UPDATE controladoras SET ultimo_exito = ?, ultimo_error = NULL WHERE id = ?`,
			Ahora(), id)
		return err
	}
	mensaje := problema.Error()
	if len(mensaje) > 500 {
		mensaje = mensaje[:500]
	}
	_, err := b.ExecContext(ctx,
		`UPDATE controladoras SET ultimo_error = ? WHERE id = ?`, mensaje, id)
	return err
}
