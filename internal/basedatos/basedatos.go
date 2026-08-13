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

// EsperaAlAbrir es cuanto se le da a una base para contestar la primera vez.
//
// Es generoso a proposito. SQLite aqui es Go puro y la primera conexion escribe
// en el disco —crea el archivo, lo pone en modo WAL—, asi que en un equipo
// ocupado eso puede tardar segundos: una Raspberry con tarjeta SD escribiendo un
// escaneo es el caso normal, no el raro. Con la espera corta que habia antes, el
// sintoma era que una red desaparecia de la interfaz sin explicacion mientras el
// equipo estaba ocupado. Esperar de mas cuesta una pantalla lenta; esperar de
// menos cuesta un dato que no aparece.
const EsperaAlAbrir = 30 * time.Second

// Abrir abre (o crea) una base en la ruta indicada, con los PRAGMA de la casa.
//
// Respeta el contexto del que llama: si la peticion se cancela o el servicio se
// esta apagando, esto se entera. Cuando el contexto no trae plazo propio se le
// pone EsperaAlAbrir, para no quedarse esperando para siempre.
func Abrir(ctx context.Context, archivo string) (*Base, error) {
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

	if _, hayPlazo := ctx.Deadline(); !hayPlazo {
		var cancelar context.CancelFunc
		ctx, cancelar = context.WithTimeout(ctx, EsperaAlAbrir)
		defer cancelar()
	}

	inicio := time.Now()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		// Se distingue "tardo demasiado" de "no se pudo": son problemas
		// distintos y se arreglan en sitios distintos. Sin esta distincion, un
		// equipo saturado y un archivo corrupto dan el mismo mensaje, y el
		// primero manda a buscar donde no hay nada.
		if ctx.Err() != nil {
			return nil, fmt.Errorf(
				"%s no contesto en %s; el equipo puede estar saturado de disco o de CPU: %w",
				archivo, time.Since(inicio).Round(time.Second), err)
		}
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
