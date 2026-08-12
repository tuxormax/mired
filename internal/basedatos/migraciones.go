package basedatos

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

// esquemas trae los archivos SQL dentro del binario, para que el .deb no tenga
// que instalarlos aparte ni el servicio dependa de rutas del sistema.
//
//go:embed esquema/catalogo/*.sql esquema/red/*.sql
var esquemas embed.FS

// TipoEsquema distingue los dos mundos de MiRed: la base global y la de cada red.
type TipoEsquema string

const (
	// EsquemaCatalogo es la base global: usuarios, permisos y registro de redes.
	EsquemaCatalogo TipoEsquema = "catalogo"
	// EsquemaRed es la base propia de una red: todo lo operativo de ese sitio.
	EsquemaRed TipoEsquema = "red"
)

// migracion es un archivo SQL numerado.
type migracion struct {
	Numero  int
	Nombre  string
	Cuerpo  string
	Archivo string
}

// Migrar aplica al archivo abierto las migraciones que le falten.
//
// A diferencia de un servidor de base de datos, aqui las migraciones NO se
// aplican una sola vez: se aplican a cada archivo. Una base de red restaurada de
// un respaldo viejo llega con esquema atrasado, y hay que actualizarla antes de
// tocarla. Por eso esto se llama al abrir, no al instalar.
func (b *Base) Migrar(ctx context.Context, tipo TipoEsquema) error {
	if err := b.crearTablaMigraciones(ctx); err != nil {
		return err
	}

	aplicadas, err := b.migracionesAplicadas(ctx)
	if err != nil {
		return err
	}

	pendientes, err := leerMigraciones(tipo)
	if err != nil {
		return err
	}

	for _, m := range pendientes {
		if aplicadas[m.Numero] {
			continue
		}
		if err := b.aplicar(ctx, m); err != nil {
			return fmt.Errorf("migracion %s de %s: %w", m.Archivo, b.Archivo, err)
		}
	}
	return nil
}

// VersionEsquema devuelve el numero de la ultima migracion aplicada.
func (b *Base) VersionEsquema(ctx context.Context) (int, error) {
	var version sql.NullInt64
	err := b.QueryRowContext(ctx, `SELECT MAX(numero) FROM migraciones`).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("no se pudo leer la version del esquema: %w", err)
	}
	return int(version.Int64), nil
}

func (b *Base) crearTablaMigraciones(ctx context.Context) error {
	_, err := b.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS migraciones (
			numero   INTEGER PRIMARY KEY,
			nombre   TEXT NOT NULL,
			aplicada TEXT NOT NULL
		)`)
	if err != nil {
		return fmt.Errorf("no se pudo preparar el control de migraciones: %w", err)
	}
	return nil
}

func (b *Base) migracionesAplicadas(ctx context.Context) (map[int]bool, error) {
	filas, err := b.QueryContext(ctx, `SELECT numero FROM migraciones`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer las migraciones aplicadas: %w", err)
	}
	defer filas.Close()

	aplicadas := map[int]bool{}
	for filas.Next() {
		var numero int
		if err := filas.Scan(&numero); err != nil {
			return nil, err
		}
		aplicadas[numero] = true
	}
	return aplicadas, filas.Err()
}

// aplicar corre una migracion completa dentro de una transaccion: o entra
// entera o no entra. SQLite si admite DDL transaccional, a diferencia de otros
// motores, asi que no hay riesgo de quedarse a medias.
func (b *Base) aplicar(ctx context.Context, m migracion) error {
	return b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, m.Cuerpo); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO migraciones (numero, nombre, aplicada) VALUES (?, ?, ?)`,
			m.Numero, m.Nombre, Ahora())
		return err
	})
}

// leerMigraciones lee del binario los archivos del esquema pedido, ordenados por
// su numero. El nombre debe ser NNNN_descripcion.sql.
func leerMigraciones(tipo TipoEsquema) ([]migracion, error) {
	carpeta := path.Join("esquema", string(tipo))
	entradas, err := fs.ReadDir(esquemas, carpeta)
	if err != nil {
		return nil, fmt.Errorf("no se encontro el esquema %s: %w", tipo, err)
	}

	var lista []migracion
	for _, entrada := range entradas {
		if entrada.IsDir() || !strings.HasSuffix(entrada.Name(), ".sql") {
			continue
		}
		numero, nombre, err := partirNombre(entrada.Name())
		if err != nil {
			return nil, err
		}
		cuerpo, err := fs.ReadFile(esquemas, path.Join(carpeta, entrada.Name()))
		if err != nil {
			return nil, err
		}
		lista = append(lista, migracion{
			Numero:  numero,
			Nombre:  nombre,
			Cuerpo:  string(cuerpo),
			Archivo: entrada.Name(),
		})
	}

	sort.Slice(lista, func(i, j int) bool { return lista[i].Numero < lista[j].Numero })
	return lista, nil
}

func partirNombre(archivo string) (int, string, error) {
	base := strings.TrimSuffix(archivo, ".sql")
	partes := strings.SplitN(base, "_", 2)
	if len(partes) != 2 {
		return 0, "", fmt.Errorf("migracion mal nombrada %q: se espera NNNN_descripcion.sql", archivo)
	}
	numero, err := strconv.Atoi(partes[0])
	if err != nil {
		return 0, "", fmt.Errorf("migracion mal nombrada %q: %w", archivo, err)
	}
	return numero, partes[1], nil
}
