package basedatos

import (
	"context"
	"database/sql"
	"fmt"
)

// El historial de versiones, que vive en el catalogo.
//
// Responde lo que el numero de version a secas no puede: **que cambio, cuando y
// en que modulo**. Cuando alguien reporta un problema, lo primero que hace falta
// saber es que trae instalado y desde cuando.

// EntradaDeVersion es una entrega y lo que trajo.
type EntradaDeVersion struct {
	Version  string `json:"version"`
	Revision int    `json:"revision"`
	Fecha    string `json:"fecha"`
	Tipo     string `json:"tipo"`
	Modulo   string `json:"modulo"`
	Notas    string `json:"notas"`
}

// SembrarVersiones deja en las tablas el historial que trae el binario.
//
// Se llama en cada arranque y es **idempotente**: las filas se identifican por
// version y revision, asi que reinstalar o reiniciar no duplica nada. Esa es la
// razon de sembrar aqui y no en una migracion — una migracion corre una vez, y
// el historial crece con cada entrega.
func (e *Enrutador) SembrarVersiones(ctx context.Context, sistema, base []EntradaDeVersion) error {
	return e.Catalogo.EnTransaccion(ctx, func(tx *sql.Tx) error {
		if err := sembrarEn(ctx, tx, "versionessistema", sistema); err != nil {
			return err
		}
		return sembrarEn(ctx, tx, "versiondb", base)
	})
}

func sembrarEn(ctx context.Context, tx *sql.Tx, tabla string, entradas []EntradaDeVersion) error {
	for _, entrada := range entradas {
		_, err := tx.ExecContext(ctx, fmt.Sprintf(`
			INSERT INTO %s (version, revision, fecha, tipo, modulo, notas)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (version, revision) DO UPDATE SET
				fecha = excluded.fecha, tipo = excluded.tipo,
				modulo = excluded.modulo, notas = excluded.notas`, tabla),
			entrada.Version, entrada.Revision, entrada.Fecha,
			entrada.Tipo, entrada.Modulo, entrada.Notas)
		if err != nil {
			return fmt.Errorf("no se pudo sembrar la version %s Rev %d en %s: %w",
				entrada.Version, entrada.Revision, tabla, err)
		}
	}
	return nil
}

// HistorialDeVersiones devuelve las dos listas, de lo mas nuevo a lo mas viejo.
func (e *Enrutador) HistorialDeVersiones(ctx context.Context) ([]EntradaDeVersion, []EntradaDeVersion, error) {
	sistema, err := leerVersiones(ctx, e.Catalogo, "versionessistema")
	if err != nil {
		return nil, nil, err
	}
	base, err := leerVersiones(ctx, e.Catalogo, "versiondb")
	if err != nil {
		return nil, nil, err
	}
	return sistema, base, nil
}

func leerVersiones(ctx context.Context, base *Base, tabla string) ([]EntradaDeVersion, error) {
	filas, err := base.QueryContext(ctx, fmt.Sprintf(`
		SELECT version, revision, fecha, tipo, modulo, notas
		  FROM %s
		 ORDER BY revision DESC`, tabla))
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer %s: %w", tabla, err)
	}
	defer filas.Close()

	entradas := []EntradaDeVersion{}
	for filas.Next() {
		var entrada EntradaDeVersion
		if err := filas.Scan(&entrada.Version, &entrada.Revision, &entrada.Fecha,
			&entrada.Tipo, &entrada.Modulo, &entrada.Notas); err != nil {
			return nil, err
		}
		entradas = append(entradas, entrada)
	}
	return entradas, filas.Err()
}
