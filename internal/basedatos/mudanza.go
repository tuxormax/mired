package basedatos

import (
	"context"
	"database/sql"
	"fmt"
)

// Aqui vive la mudanza de las credenciales SNMP y las controladoras WiFi desde
// el catalogo —donde las compartian todas las redes— a la base de cada red.
//
// **Por que no la hace una migracion.** Las migraciones son SQL sobre UNA base,
// y aqui hay que leer de un archivo y escribir en otros: el catalogo y la base
// de cada red son archivos distintos. Asi que la hace el servidor al arrancar,
// una sola vez, en cuanto ve que el catalogo todavia tiene algo.
//
// **Por que se copia a TODAS las redes.** Porque eso es lo que habia: esas
// credenciales SE USABAN en todas. Repartirlas a mano —adivinando cual era de
// que sitio— seria inventarse un dato; dejarlas en una sola red le quitaria el
// SNMP a las demas sin avisar. Se copian a todas y quien sobre se borra desde
// la pantalla de su red, que es una decision de quien conoce la instalacion.
//
// **Por que se vacia el catalogo despues.** Es lo que hace la mudanza
// irrepetible sin necesidad de una marca aparte: si el catalogo esta vacio, no
// hay nada que copiar. Vaciarlo es lo ultimo, y solo si todas las redes ya
// recibieron su copia.

// Las dos formas de abajo llevan el estatus, que los structs publicos no
// exponen: la mudanza tiene que copiar tambien lo borrado con borrado suave,
// porque una credencial que alguien quito sigue siendo suya y su nombre hace
// falta para poder reactivarla.
type credencialMudada struct {
	CredencialSNMP
	estatus int
}

type controladoraMudada struct {
	Controladora
	estatus int
}

// MudarCredencialesALasRedes copia a la base de cada red lo que estaba
// compartido en el catalogo, y despues lo quita de alli.
//
// Devuelve cuantas credenciales y controladoras se movieron. Si no habia nada,
// no hace nada y no es un error: es lo normal en toda instalacion posterior al
// cambio.
func (e *Enrutador) MudarCredencialesALasRedes(ctx context.Context) (int, int, error) {
	credenciales, controladoras, err := e.loQueQuedaEnElCatalogo(ctx)
	if err != nil {
		return 0, 0, err
	}
	if len(credenciales) == 0 && len(controladoras) == 0 {
		return 0, 0, nil
	}

	redes, err := e.ListarRedes(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("no se pudieron listar las redes para la mudanza: %w", err)
	}

	for _, red := range redes {
		err := e.ConRed(ctx, red.Clave, func(base *Base) error {
			return base.recibirDelCatalogo(ctx, credenciales, controladoras)
		})
		if err != nil {
			// Si una red falla se para todo y NO se vacia el catalogo: al
			// siguiente arranque se vuelve a intentar entero. Media mudanza es
			// peor que ninguna.
			return 0, 0, fmt.Errorf("no se pudo mudar a la red %s: %w", red.Clave, err)
		}
	}

	if _, err := e.Catalogo.ExecContext(ctx, `DELETE FROM credenciales_snmp`); err != nil {
		return 0, 0, fmt.Errorf("no se pudo vaciar el catalogo de credenciales: %w", err)
	}
	if _, err := e.Catalogo.ExecContext(ctx, `DELETE FROM controladoras`); err != nil {
		return 0, 0, fmt.Errorf("no se pudo vaciar el catalogo de controladoras: %w", err)
	}
	return len(credenciales), len(controladoras), nil
}

// loQueQuedaEnElCatalogo lee lo compartido que todavia no se ha mudado.
//
// Lee TODO, incluido lo borrado con borrado suave: una credencial que alguien
// quito sigue siendo suya, y perder su nombre haria que reactivarla por nombre
// dejara de funcionar.
func (e *Enrutador) loQueQuedaEnElCatalogo(ctx context.Context) ([]credencialMudada, []controladoraMudada, error) {
	// Las tablas del catalogo siguen existiendo pero pueden estar vacias. Si
	// alguna no existe —una instalacion nueva creada despues de quitarlas del
	// esquema— tampoco es un error: no hay nada que mudar.
	credenciales := []credencialMudada{}
	filas, err := e.Catalogo.QueryContext(ctx, `
		SELECT nombre, version, COALESCE(comunidad, ''), COALESCE(usuario, ''),
		       COALESCE(autenticacion_protocolo, ''), COALESCE(autenticacion_clave, ''),
		       COALESCE(privacidad_protocolo, ''), COALESCE(privacidad_clave, ''),
		       estatus, creada
		  FROM credenciales_snmp`)
	if err != nil {
		return nil, nil, nil // la tabla ya no esta: nada que mudar
	}
	for filas.Next() {
		var c credencialMudada
		if err := filas.Scan(&c.Nombre, &c.Version, &c.Comunidad, &c.Usuario,
			&c.AutenticacionProtocolo, &c.AutenticacionClave,
			&c.PrivacidadProtocolo, &c.PrivacidadClave, &c.estatus, &c.Creada); err != nil {
			filas.Close()
			return nil, nil, err
		}
		credenciales = append(credenciales, c)
	}
	filas.Close()

	controladoras := []controladoraMudada{}
	filasC, err := e.Catalogo.QueryContext(ctx, `
		SELECT nombre, tipo, url, usuario, clave, sitio, verificar_tls, estatus, creada
		  FROM controladoras`)
	if err != nil {
		return credenciales, controladoras, nil
	}
	defer filasC.Close()
	for filasC.Next() {
		var c controladoraMudada
		var verificar int
		if err := filasC.Scan(&c.Nombre, &c.Tipo, &c.URL, &c.Usuario, &c.Clave,
			&c.Sitio, &verificar, &c.estatus, &c.Creada); err != nil {
			return nil, nil, err
		}
		c.VerificarTLS = verificar == 1
		controladoras = append(controladoras, c)
	}
	return credenciales, controladoras, filasC.Err()
}

// recibirDelCatalogo mete en ESTA red lo que venia del catalogo.
//
// Si la red ya tiene una con ese nombre no se toca: puede haberla creado alguien
// despues del cambio, y pisarla con la vieja seria deshacer su trabajo.
func (b *Base) recibirDelCatalogo(ctx context.Context,
	credenciales []credencialMudada, controladoras []controladoraMudada) error {
	return b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		for _, c := range credenciales {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO credenciales_snmp (nombre, version, comunidad, usuario,
				                               autenticacion_protocolo, autenticacion_clave,
				                               privacidad_protocolo, privacidad_clave,
				                               estatus, creada)
				SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
				 WHERE NOT EXISTS (
				       SELECT 1 FROM credenciales_snmp WHERE nombre = ? AND estatus >= 0)`,
				c.Nombre, c.Version, nuloSiVacio(c.Comunidad), nuloSiVacio(c.Usuario),
				nuloSiVacio(c.AutenticacionProtocolo), nuloSiVacio(c.AutenticacionClave),
				nuloSiVacio(c.PrivacidadProtocolo), nuloSiVacio(c.PrivacidadClave),
				c.estatus, c.Creada, c.Nombre); err != nil {
				return fmt.Errorf("no se pudo copiar la credencial %s: %w", c.Nombre, err)
			}
		}
		for _, c := range controladoras {
			verificar := 0
			if c.VerificarTLS {
				verificar = 1
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO controladoras (nombre, tipo, url, usuario, clave, sitio,
				                           verificar_tls, estatus, creada)
				SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?
				 WHERE NOT EXISTS (
				       SELECT 1 FROM controladoras WHERE nombre = ? AND estatus >= 0)`,
				c.Nombre, c.Tipo, c.URL, c.Usuario, c.Clave, c.Sitio, verificar,
				c.estatus, c.Creada, c.Nombre); err != nil {
				return fmt.Errorf("no se pudo copiar la controladora %s: %w", c.Nombre, err)
			}
		}
		return nil
	})
}
