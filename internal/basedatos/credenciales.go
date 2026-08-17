package basedatos

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrCredencialRepetida la devuelve el alta cuando ya hay una con ese nombre.
var ErrCredencialRepetida = errors.New("ya existe una credencial con ese nombre")

// CredencialSNMP es como se le habla a los equipos administrables.
//
// Viven en el catalogo y no en cada red porque en la practica la misma
// comunidad o el mismo usuario v3 sirve para varios sitios, y repetirla red por
// red es la forma segura de que una quede desactualizada.
type CredencialSNMP struct {
	ID      int64  `json:"id"`
	Nombre  string `json:"nombre"`
	Version string `json:"version"`

	Comunidad string `json:"comunidad,omitempty"`

	Usuario                string `json:"usuario,omitempty"`
	AutenticacionProtocolo string `json:"autenticacionProtocolo,omitempty"`
	AutenticacionClave     string `json:"autenticacionClave,omitempty"`
	PrivacidadProtocolo    string `json:"privacidadProtocolo,omitempty"`
	PrivacidadClave        string `json:"privacidadClave,omitempty"`

	Creada string `json:"creada"`
}

// SinSecretos devuelve una copia sin claves ni comunidades, para mandarla a la
// interfaz. La comunidad SNMP es, en la practica, una contrasena: no tiene por
// que viajar de vuelta al navegador solo para pintar una lista.
func (c CredencialSNMP) SinSecretos() CredencialSNMP {
	c.Comunidad = ""
	c.AutenticacionClave = ""
	c.PrivacidadClave = ""
	return c
}

// ListarCredencialesSNMP devuelve las credenciales activas, CON sus secretos.
// Solo la usa la sonda; para la interfaz se pasa antes por SinSecretos.
func (b *Base) ListarCredencialesSNMP(ctx context.Context) ([]CredencialSNMP, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT id, nombre, version, COALESCE(comunidad, ''), COALESCE(usuario, ''),
		       COALESCE(autenticacion_protocolo, ''), COALESCE(autenticacion_clave, ''),
		       COALESCE(privacidad_protocolo, ''), COALESCE(privacidad_clave, ''), creada
		  FROM credenciales_snmp
		 WHERE estatus = 1
		 ORDER BY nombre`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar las credenciales SNMP: %w", err)
	}
	defer filas.Close()

	credenciales := []CredencialSNMP{}
	for filas.Next() {
		var c CredencialSNMP
		if err := filas.Scan(&c.ID, &c.Nombre, &c.Version, &c.Comunidad, &c.Usuario,
			&c.AutenticacionProtocolo, &c.AutenticacionClave,
			&c.PrivacidadProtocolo, &c.PrivacidadClave, &c.Creada); err != nil {
			return nil, err
		}
		credenciales = append(credenciales, c)
	}
	return credenciales, filas.Err()
}

// CrearCredencialSNMP da de alta una credencial. Si habia una borrada con el
// mismo nombre, se reactiva con los datos nuevos.
func (b *Base) CrearCredencialSNMP(ctx context.Context, credencial CredencialSNMP) (CredencialSNMP, error) {
	credencial.Nombre = strings.TrimSpace(credencial.Nombre)
	if credencial.Nombre == "" {
		return CredencialSNMP{}, fmt.Errorf("la credencial necesita un nombre")
	}
	switch credencial.Version {
	case "v1", "v2c":
		if credencial.Comunidad == "" {
			return CredencialSNMP{}, fmt.Errorf("SNMP %s necesita una comunidad", credencial.Version)
		}
	case "v3":
		if credencial.Usuario == "" {
			return CredencialSNMP{}, fmt.Errorf("SNMP v3 necesita un usuario")
		}
	default:
		return CredencialSNMP{}, fmt.Errorf("version de SNMP invalida: %s", credencial.Version)
	}

	var borradaID int64
	err := b.QueryRowContext(ctx,
		`SELECT id FROM credenciales_snmp WHERE nombre = ? AND estatus = -1`,
		credencial.Nombre).Scan(&borradaID)
	if err == nil {
		_, err = b.ExecContext(ctx, `
			UPDATE credenciales_snmp
			   SET version = ?, comunidad = ?, usuario = ?, autenticacion_protocolo = ?,
			       autenticacion_clave = ?, privacidad_protocolo = ?, privacidad_clave = ?,
			       estatus = 1, modificada = ?
			 WHERE id = ?`,
			credencial.Version, nuloSiVacio(credencial.Comunidad), nuloSiVacio(credencial.Usuario),
			nuloSiVacio(credencial.AutenticacionProtocolo), nuloSiVacio(credencial.AutenticacionClave),
			nuloSiVacio(credencial.PrivacidadProtocolo), nuloSiVacio(credencial.PrivacidadClave),
			Ahora(), borradaID)
		if err != nil {
			return CredencialSNMP{}, fmt.Errorf("no se pudo reactivar la credencial: %w", err)
		}
		credencial.ID = borradaID
		return credencial, nil
	}

	var repetida int
	if err := b.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM credenciales_snmp WHERE nombre = ? AND estatus >= 0`,
		credencial.Nombre).Scan(&repetida); err != nil {
		return CredencialSNMP{}, fmt.Errorf("no se pudo comprobar el nombre: %w", err)
	}
	if repetida > 0 {
		return CredencialSNMP{}, ErrCredencialRepetida
	}

	resultado, err := b.ExecContext(ctx, `
		INSERT INTO credenciales_snmp (nombre, version, comunidad, usuario,
		                               autenticacion_protocolo, autenticacion_clave,
		                               privacidad_protocolo, privacidad_clave, estatus, creada)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
		credencial.Nombre, credencial.Version, nuloSiVacio(credencial.Comunidad),
		nuloSiVacio(credencial.Usuario), nuloSiVacio(credencial.AutenticacionProtocolo),
		nuloSiVacio(credencial.AutenticacionClave), nuloSiVacio(credencial.PrivacidadProtocolo),
		nuloSiVacio(credencial.PrivacidadClave), Ahora())
	if err != nil {
		return CredencialSNMP{}, fmt.Errorf("no se pudo crear la credencial: %w", err)
	}
	credencial.ID, _ = resultado.LastInsertId()
	return credencial, nil
}

// BorrarCredencialSNMP aplica borrado suave.
func (b *Base) BorrarCredencialSNMP(ctx context.Context, id int64) error {
	resultado, err := b.ExecContext(ctx,
		`UPDATE credenciales_snmp SET estatus = -1, modificada = ? WHERE id = ? AND estatus >= 0`,
		Ahora(), id)
	if err != nil {
		return fmt.Errorf("no se pudo borrar la credencial: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return fmt.Errorf("esa credencial no existe")
	}
	return nil
}
