// Paquete basedatos concentra TODO el acceso a SQLite de MiRed: el catalogo
// global y la base propia de cada red.
//
// Regla que no se negocia: el enrutado de conexiones vive aqui y en ningun otro
// paquete. Ningun modulo decide por su cuenta a que archivo escribe; le pide al
// enrutador la base de la red con la que trabaja. Si esa decision se reparte,
// tarde o temprano una funcion nueva escribira en el archivo equivocado.
package basedatos

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // driver de SQLite en Go puro, sin cgo
)

// nombreDriver es el driver de SQLite en Go puro. Sin cgo el binario queda
// estatico y cruza a arm64 sin toolchain cruzado, que es media razon por la que
// se eligio Go.
const nombreDriver = "sqlite"

// Base es una base de datos abierta, sea el catalogo o la de una red.
type Base struct {
	*sql.DB
	// Archivo es la ruta del .db, util para mensajes de error y respaldos.
	Archivo string
}

// opcionesConexion son los PRAGMA con los que se abre toda base de MiRed.
//
//   - WAL deja leer mientras se escribe, que es lo que vuelve a SQLite usable en
//     un servicio con interfaz web.
//   - busy_timeout evita que una escritura simultanea falle de inmediato: espera.
//   - foreign_keys las apaga SQLite por omision; sin esto las llaves foraneas
//     serian decorativas.
//   - synchronous NORMAL es lo recomendado junto con WAL: seguro ante caida del
//     proceso, y solo arriesga la ultima transaccion ante corte de corriente.
var opcionesConexion = map[string]string{
	"journal_mode": "WAL",
	"busy_timeout": "5000",
	"foreign_keys": "1",
	"synchronous":  "NORMAL",
}

// Abrir abre (o crea) una base en la ruta indicada, con los PRAGMA de la casa.
func Abrir(archivo string) (*Base, error) {
	if err := os.MkdirAll(filepath.Dir(archivo), 0o750); err != nil {
		return nil, fmt.Errorf("no se pudo crear la carpeta de %s: %w", archivo, err)
	}

	parametros := url.Values{}
	for pragma, valor := range opcionesConexion {
		parametros.Add("_pragma", fmt.Sprintf("%s(%s)", pragma, valor))
	}
	dsn := "file:" + archivo + "?" + parametros.Encode()

	db, err := sql.Open(nombreDriver, dsn)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir %s: %w", archivo, err)
	}

	// Con WAL varias conexiones pueden leer a la vez; las escrituras se
	// serializan solas y esperan gracias a busy_timeout. Un tope modesto evita
	// que un pico de peticiones abra decenas de conexiones por base, que con
	// muchas redes abiertas suma.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("no se pudo conectar a %s: %w", archivo, err)
	}

	return &Base{DB: db, Archivo: archivo}, nil
}

// EnTransaccion ejecuta la funcion dentro de una transaccion y confirma solo si
// no devolvio error. Si la funcion entra en panico, deshace y vuelve a lanzarlo.
func (b *Base) EnTransaccion(ctx context.Context, hacer func(*sql.Tx) error) error {
	tx, err := b.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("no se pudo iniciar la transaccion: %w", err)
	}
	defer func() {
		if problema := recover(); problema != nil {
			tx.Rollback()
			panic(problema)
		}
	}()

	if err := hacer(tx); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("no se pudo confirmar la transaccion: %w", err)
	}
	return nil
}

// Ahora devuelve el momento actual en el formato en que MiRed guarda las fechas:
// texto ISO 8601 con zona horaria. SQLite no tiene tipo fecha, asi que el
// formato es el contrato, y se respeta en todo el proyecto.
func Ahora() string {
	return time.Now().Format(time.RFC3339)
}
