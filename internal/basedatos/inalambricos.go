package basedatos

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Lo que cuelga de una antena por el aire.
//
// Es el hermano de enlaces_fisicos: la misma idea —el dato dice de donde salio—
// aplicada a lo que no lleva cable. Un telefono no entra por un puerto, y por eso
// no se le puede inventar uno.

// Origenes propios de un enlace inalambrico. Los demas —manual y snmp— ya
// existen para los cables y son los mismos: el dato dice de donde salio, sin
// importar si vino por aire o por cobre.
const (
	// OrigenControladora lo dijo la controladora WiFi: sabe de que antena y de
	// que SSID cuelga cada cliente.
	OrigenControladora = "controladora"
	// OrigenPanel se leyo del panel del propio aparato, entrando con la
	// credencial que el usuario guardo en su ficha.
	OrigenPanel = "panel"
)

// ErrAntenaNoExiste la devuelve el alta cuando la antena ya no esta.
var ErrAntenaNoExiste = errors.New("la antena no existe")

// EnlaceInalambrico es un equipo colgado de una antena.
type EnlaceInalambrico struct {
	ID       int64 `json:"id"`
	EquipoID int64 `json:"equipoId"`
	AntenaID int64 `json:"antenaId"`

	// Resueltos al leer, para que la interfaz no tenga que cruzarlos.
	EquipoNombre string `json:"equipoNombre"`
	AntenaNombre string `json:"antenaNombre"`

	// Red es el SSID, cuando se sabe.
	Red string `json:"red,omitempty"`
	// SenalDbm solo viene de lo medido: quien declara a mano sabe de que antena
	// cuelga su telefono, no con cuantos dBm llega.
	SenalDbm *int `json:"senalDbm,omitempty"`

	OrigenDato string `json:"origenDato"`
	Notas      string `json:"notas,omitempty"`
	UltimaVez  string `json:"ultimaVez"`
}

// EsManual dice si lo tecleo una persona.
func (e EnlaceInalambrico) EsManual() bool { return e.OrigenDato == OrigenManual }

// GuardarEnlaceInalambrico cuelga un equipo de una antena.
//
// Un cliente esta asociado a UNA antena a la vez —eso no es una regla de MiRed,
// es como funciona el WiFi—, asi que volver a colgarlo reemplaza.
//
// **Lo medido no pisa lo declarado a la ligera, ni al reves**: si una persona
// declaro que el telefono cuelga de la antena del patio y despues la antena dice
// otra cosa, gana lo MEDIDO —la antena sabe quien esta asociado ahora mismo— y
// la contradiccion se ve porque el origen cambia. Al reves no: un escaneo que no
// ve nada no borra lo que alguien declaro.
func (b *Base) GuardarEnlaceInalambrico(ctx context.Context, enlace EnlaceInalambrico) (EnlaceInalambrico, error) {
	if enlace.EquipoID <= 0 || enlace.AntenaID <= 0 {
		return EnlaceInalambrico{}, errors.New("hace falta decir que equipo cuelga de que antena")
	}
	if enlace.EquipoID == enlace.AntenaID {
		return EnlaceInalambrico{}, ErrEnlaceAlReves
	}
	if enlace.OrigenDato == "" {
		enlace.OrigenDato = OrigenManual
	}

	momento := Ahora()
	var creado EnlaceInalambrico

	err := b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		for _, caso := range []struct {
			id  int64
			err error
		}{{enlace.EquipoID, ErrEquipoNoExiste}, {enlace.AntenaID, ErrAntenaNoExiste}} {
			var existe int
			if err := tx.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM equipos WHERE id = ? AND estatus = 1`, caso.id).Scan(&existe); err != nil {
				return err
			}
			if existe == 0 {
				return caso.err
			}
		}

		resultado, err := tx.ExecContext(ctx, `
			INSERT INTO enlaces_inalambricos (equipo_id, antena_id, red, senal_dbm,
			                                  origen_dato, notas, ultima_vez, creado_en)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (equipo_id) DO UPDATE SET
				antena_id   = excluded.antena_id,
				red         = COALESCE(NULLIF(excluded.red, ''), enlaces_inalambricos.red),
				senal_dbm   = COALESCE(excluded.senal_dbm, enlaces_inalambricos.senal_dbm),
				origen_dato = excluded.origen_dato,
				notas       = COALESCE(NULLIF(excluded.notas, ''), enlaces_inalambricos.notas),
				ultima_vez  = excluded.ultima_vez`,
			enlace.EquipoID, enlace.AntenaID, nuloSiVacio(enlace.Red), enlace.SenalDbm,
			enlace.OrigenDato, nuloSiVacio(enlace.Notas), momento, momento)
		if err != nil {
			return fmt.Errorf("no se pudo guardar el enlace inalambrico: %w", err)
		}

		id, _ := resultado.LastInsertId()
		if id == 0 {
			// Fue una actualizacion: hay que ir por el identificador de verdad.
			if err := tx.QueryRowContext(ctx,
				`SELECT id FROM enlaces_inalambricos WHERE equipo_id = ?`,
				enlace.EquipoID).Scan(&id); err != nil {
				return err
			}
		}
		creado = enlace
		creado.ID = id
		creado.UltimaVez = momento
		return nil
	})

	return creado, err
}

// ListarEnlacesInalambricos devuelve todo lo que cuelga por el aire, con los
// nombres ya resueltos.
func (b *Base) ListarEnlacesInalambricos(ctx context.Context) ([]EnlaceInalambrico, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT i.id, i.equipo_id, i.antena_id,
		       COALESCE(NULLIF(c.alias, ''), NULLIF(c.nombre, ''), c.ip, 'equipo ' || c.id),
		       COALESCE(NULLIF(a.alias, ''), NULLIF(a.nombre, ''), a.ip, 'antena ' || a.id),
		       COALESCE(i.red, ''), i.senal_dbm, i.origen_dato, COALESCE(i.notas, ''), i.ultima_vez
		  FROM enlaces_inalambricos i
		  JOIN equipos c ON c.id = i.equipo_id AND c.estatus = 1
		  JOIN equipos a ON a.id = i.antena_id AND a.estatus = 1
		 ORDER BY a.id, c.id`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar los enlaces inalambricos: %w", err)
	}
	defer filas.Close()

	enlaces := []EnlaceInalambrico{}
	for filas.Next() {
		var enlace EnlaceInalambrico
		var senal sql.NullInt64
		if err := filas.Scan(&enlace.ID, &enlace.EquipoID, &enlace.AntenaID,
			&enlace.EquipoNombre, &enlace.AntenaNombre, &enlace.Red, &senal,
			&enlace.OrigenDato, &enlace.Notas, &enlace.UltimaVez); err != nil {
			return nil, err
		}
		if senal.Valid {
			valor := int(senal.Int64)
			enlace.SenalDbm = &valor
		}
		enlaces = append(enlaces, enlace)
	}
	return enlaces, filas.Err()
}

// BorrarEnlaceInalambrico descuelga un equipo de su antena.
func (b *Base) BorrarEnlaceInalambrico(ctx context.Context, id int64) error {
	resultado, err := b.ExecContext(ctx, `DELETE FROM enlaces_inalambricos WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("no se pudo quitar el enlace inalambrico: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return ErrEnlaceNoExiste
	}
	return nil
}

// GuardarAsociadosDeAntena guarda de golpe lo que dijo una antena.
//
// Se usa desde el escaneo, con lo que contesto por SNMP o por su panel. **No
// borra lo declarado a mano de otras antenas**: solo reemplaza lo que esta
// antena reporta, que es lo unico sobre lo que tiene autoridad.
func (b *Base) GuardarAsociadosDeAntena(ctx context.Context, antenaID int64,
	origen string, asociados []EnlaceInalambrico) (int, error) {
	guardados := 0
	for _, asociado := range asociados {
		asociado.AntenaID = antenaID
		asociado.OrigenDato = origen
		if _, err := b.GuardarEnlaceInalambrico(ctx, asociado); err != nil {
			if errors.Is(err, ErrEquipoNoExiste) || errors.Is(err, ErrEnlaceAlReves) {
				// Un asociado que no esta en el inventario todavia no es un
				// error: aparecera en el proximo barrido.
				continue
			}
			return guardados, err
		}
		guardados++
	}
	return guardados, nil
}

// EquipoPorMAC busca un equipo por su direccion fisica.
//
// Lo necesita el cruce de asociados: la antena contesta con MAC, no con la IP ni
// con el identificador de MiRed.
func (b *Base) EquipoPorMAC(ctx context.Context, mac string) (int64, error) {
	mac = normalizarMACGuardada(mac)
	if mac == "" {
		return 0, ErrEquipoNoExiste
	}

	var id int64
	err := b.QueryRowContext(ctx,
		`SELECT id FROM equipos WHERE LOWER(REPLACE(REPLACE(mac, ':', ''), '-', '')) = ?
		   AND estatus = 1`, mac).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrEquipoNoExiste
	}
	return id, err
}

func normalizarMACGuardada(mac string) string {
	limpia := make([]rune, 0, 12)
	for _, letra := range mac {
		switch {
		case letra >= '0' && letra <= '9', letra >= 'a' && letra <= 'f':
			limpia = append(limpia, letra)
		case letra >= 'A' && letra <= 'F':
			limpia = append(limpia, letra+32)
		}
	}
	if len(limpia) != 12 {
		return ""
	}
	return string(limpia)
}
