package basedatos

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// FichaSNMP es lo que un equipo administrable contesto por SNMP, listo para
// guardarse.
type FichaSNMP struct {
	IP            string
	Nombre        string
	Descripcion   string
	Contacto      string
	Ubicacion     string
	ObjectID      string
	EncendidoMs   int64
	EsSwitch      bool
	Credencial    string
	Interfaces    []InterfazSNMP
	MacsPorPuerto map[string][]string
	Vecinos       []VecinoSNMP
}

// InterfazSNMP es una boca de un equipo administrable.
type InterfazSNMP struct {
	Indice        int
	Nombre        string
	Descripcion   string
	Alias         string
	MAC           string
	Tipo          int
	Activa        bool
	VelocidadMbps int
}

// VecinoSNMP es un equipo anunciado por LLDP en una boca.
type VecinoSNMP struct {
	InterfazLocal string
	Nombre        string
	Descripcion   string
	PuertoRemoto  string
	ChasisID      string
}

// Capacidad del mapa de puertos de una red.
const (
	CapacidadDesconocida  = "desconocida"
	CapacidadExacta       = "exacta"
	CapacidadPorGrupo     = "por_grupo"
	CapacidadNoDisponible = "no_disponible"
)

// GuardarSNMP guarda lo que contestaron los equipos administrables y arma con
// eso el mapa de puertos.
func (b *Base) GuardarSNMP(ctx context.Context, fichas []FichaSNMP) error {
	momento := Ahora()

	return b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		// Las MAC conocidas permiten enlazar cada boca con el equipo que ya se
		// descubrio, en vez de dejar una direccion suelta que no le dice nada a
		// nadie.
		porMAC, err := equiposPorMAC(ctx, tx)
		if err != nil {
			return err
		}

		for _, ficha := range fichas {
			equipoID, hay := porIP(ctx, tx, ficha.IP)
			if !hay {
				// El equipo tiene que existir: la ficha SNMP se cuelga de el.
				// Si no esta es que el barrido no lo vio, y eso ya es un dato
				// raro que no conviene tapar creando un equipo a medias.
				continue
			}

			if err := guardarFichaSNMP(ctx, tx, equipoID, ficha, momento); err != nil {
				return err
			}
			if err := guardarInterfaces(ctx, tx, equipoID, ficha.Interfaces, momento); err != nil {
				return err
			}
			if err := guardarConexiones(ctx, tx, equipoID, ficha.MacsPorPuerto, porMAC, momento); err != nil {
				return err
			}
			if err := guardarEnlaces(ctx, tx, equipoID, ficha.Vecinos, porMAC, momento); err != nil {
				return err
			}
		}
		return nil
	})
}

func equiposPorMAC(ctx context.Context, tx *sql.Tx) (map[string]int64, error) {
	filas, err := tx.QueryContext(ctx,
		`SELECT id, LOWER(mac) FROM equipos WHERE mac IS NOT NULL AND mac <> '' AND estatus = 1`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer las MAC conocidas: %w", err)
	}
	defer filas.Close()

	porMAC := map[string]int64{}
	for filas.Next() {
		var id int64
		var mac string
		if err := filas.Scan(&id, &mac); err != nil {
			return nil, err
		}
		porMAC[mac] = id
	}
	return porMAC, filas.Err()
}

func porIP(ctx context.Context, tx *sql.Tx, ip string) (int64, bool) {
	var id int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM equipos WHERE ip = ? AND estatus = 1 ORDER BY id LIMIT 1`, ip).Scan(&id)
	return id, err == nil
}

func guardarFichaSNMP(ctx context.Context, tx *sql.Tx, equipoID int64, ficha FichaSNMP, momento string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO equipos_snmp (equipo_id, nombre, descripcion, contacto, ubicacion,
		                          object_id, encendido_ms, es_switch, credencial, consultado)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (equipo_id) DO UPDATE SET
			nombre = excluded.nombre, descripcion = excluded.descripcion,
			contacto = excluded.contacto, ubicacion = excluded.ubicacion,
			object_id = excluded.object_id, encendido_ms = excluded.encendido_ms,
			es_switch = excluded.es_switch, credencial = excluded.credencial,
			consultado = excluded.consultado`,
		equipoID, nuloSiVacio(ficha.Nombre), nuloSiVacio(ficha.Descripcion),
		nuloSiVacio(ficha.Contacto), nuloSiVacio(ficha.Ubicacion),
		nuloSiVacio(ficha.ObjectID), ficha.EncendidoMs, boolAEntero(ficha.EsSwitch),
		nuloSiVacio(ficha.Credencial), momento)
	if err != nil {
		return fmt.Errorf("no se pudo guardar la ficha SNMP: %w", err)
	}

	// El nombre que el equipo dice tener es mejor que el del DNS inverso, pero
	// nunca pisa el que puso una persona.
	if ficha.Nombre != "" {
		_, err = tx.ExecContext(ctx,
			`UPDATE equipos SET nombre = ? WHERE id = ? AND (nombre IS NULL OR nombre = '')`,
			ficha.Nombre, equipoID)
		if err != nil {
			return err
		}
	}
	return nil
}

func guardarInterfaces(ctx context.Context, tx *sql.Tx, equipoID int64, interfaces []InterfazSNMP, momento string) error {
	for _, puerto := range interfaces {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO interfaces (equipo_id, indice, nombre, descripcion, alias, mac,
			                        tipo, activa, velocidad_mbps, ultima_vez)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (equipo_id, indice) DO UPDATE SET
				nombre = excluded.nombre, descripcion = excluded.descripcion,
				alias = excluded.alias, mac = excluded.mac, tipo = excluded.tipo,
				activa = excluded.activa, velocidad_mbps = excluded.velocidad_mbps,
				ultima_vez = excluded.ultima_vez`,
			equipoID, puerto.Indice, nuloSiVacio(puerto.Nombre), nuloSiVacio(puerto.Descripcion),
			nuloSiVacio(puerto.Alias), nuloSiVacio(puerto.MAC), puerto.Tipo,
			boolAEntero(puerto.Activa), puerto.VelocidadMbps, momento)
		if err != nil {
			return fmt.Errorf("no se pudo guardar la interfaz %d: %w", puerto.Indice, err)
		}
	}
	return nil
}

// guardarConexiones arma el mapa de puertos.
//
// Aqui esta la decision honesta del proyecto: una boca con UNA MAC es un enlace
// confirmado; una boca con varias es un grupo detras de algo no administrable, y
// se guarda como tal. Inventar un puerto para cada equipo de ese grupo seria
// mentir, y un plano de sitio con datos inventados es peor que no tenerlo.
func guardarConexiones(ctx context.Context, tx *sql.Tx, switchID int64, macsPorPuerto map[string][]string, porMAC map[string]int64, momento string) error {
	if len(macsPorPuerto) == 0 {
		return nil
	}

	// Se borra lo anterior de este switch: la tabla de reenvio es una foto del
	// momento, y conservar bocas viejas dejaria equipos colgando de puertos
	// donde ya no estan.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM conexiones_puerto WHERE switch_id = ?`, switchID); err != nil {
		return fmt.Errorf("no se pudieron limpiar las conexiones anteriores: %w", err)
	}

	for puerto, macs := range macsPorPuerto {
		indice := 0
		fmt.Sscanf(puerto, "%d", &indice)
		if indice == 0 {
			continue
		}

		unicos := map[string]bool{}
		for _, mac := range macs {
			unicos[strings.ToLower(mac)] = true
		}
		cuantos := len(unicos)

		for mac := range unicos {
			var equipoID any
			if id, hay := porMAC[mac]; hay {
				equipoID = id
			}
			_, err := tx.ExecContext(ctx, `
				INSERT INTO conexiones_puerto (switch_id, interfaz_indice, mac, equipo_id,
				                               confirmado, cuantos_en_boca, ultima_vez)
				VALUES (?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (switch_id, interfaz_indice, mac) DO UPDATE SET
					equipo_id = excluded.equipo_id,
					confirmado = excluded.confirmado,
					cuantos_en_boca = excluded.cuantos_en_boca,
					ultima_vez = excluded.ultima_vez`,
				switchID, indice, mac, equipoID, boolAEntero(cuantos == 1), cuantos, momento)
			if err != nil {
				return fmt.Errorf("no se pudo guardar la conexion del puerto %d: %w", indice, err)
			}
		}
	}
	return nil
}

func guardarEnlaces(ctx context.Context, tx *sql.Tx, equipoID int64, vecinos []VecinoSNMP, porMAC map[string]int64, momento string) error {
	for _, vecino := range vecinos {
		if vecino.InterfazLocal == "" {
			continue
		}
		var vecinoID any
		if id, hay := porMAC[strings.ToLower(vecino.ChasisID)]; hay {
			vecinoID = id
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO enlaces (equipo_id, interfaz_local, vecino_nombre, vecino_puerto,
			                     vecino_chasis, vecino_equipo_id, origen, ultima_vez)
			VALUES (?, ?, ?, ?, ?, ?, 'lldp', ?)
			ON CONFLICT (equipo_id, interfaz_local, origen) DO UPDATE SET
				vecino_nombre = excluded.vecino_nombre,
				vecino_puerto = excluded.vecino_puerto,
				vecino_chasis = excluded.vecino_chasis,
				vecino_equipo_id = excluded.vecino_equipo_id,
				ultima_vez = excluded.ultima_vez`,
			equipoID, vecino.InterfazLocal, nuloSiVacio(vecino.Nombre),
			nuloSiVacio(vecino.PuertoRemoto), nuloSiVacio(vecino.ChasisID), vecinoID, momento)
		if err != nil {
			return fmt.Errorf("no se pudo guardar el enlace de %s: %w", vecino.InterfazLocal, err)
		}
	}
	return nil
}

// PuertoDeSwitch es un renglon del mapa de puertos.
type PuertoDeSwitch struct {
	SwitchID      int64  `json:"switchId"`
	SwitchNombre  string `json:"switchNombre"`
	SwitchIP      string `json:"switchIp"`
	Indice        int    `json:"indice"`
	Puerto        string `json:"puerto"`
	Alias         string `json:"alias"`
	Activa        bool   `json:"activa"`
	VelocidadMbps int    `json:"velocidadMbps"`
	MAC           string `json:"mac"`
	EquipoID      *int64 `json:"equipoId"`
	EquipoNombre  string `json:"equipoNombre"`
	EquipoIP      string `json:"equipoIp"`
	Confirmado    bool   `json:"confirmado"`
	CuantosEnBoca int    `json:"cuantosEnBoca"`
}

// MapaDePuertos devuelve que hay conectado en cada boca de cada switch.
func (b *Base) MapaDePuertos(ctx context.Context) ([]PuertoDeSwitch, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT c.switch_id,
		       COALESCE(sw.alias, sw.nombre, sw.ip),
		       sw.ip,
		       c.interfaz_indice,
		       COALESCE(i.nombre, i.descripcion, CAST(c.interfaz_indice AS TEXT)),
		       COALESCE(i.alias, ''),
		       COALESCE(i.activa, 0),
		       COALESCE(i.velocidad_mbps, 0),
		       c.mac,
		       c.equipo_id,
		       COALESCE(eq.alias, eq.nombre, eq.ip, ''),
		       COALESCE(eq.ip, ''),
		       c.confirmado,
		       c.cuantos_en_boca
		  FROM conexiones_puerto c
		  JOIN equipos sw ON sw.id = c.switch_id
		  LEFT JOIN interfaces i ON i.equipo_id = c.switch_id AND i.indice = c.interfaz_indice
		  LEFT JOIN equipos eq ON eq.id = c.equipo_id
		 ORDER BY sw.ip, c.interfaz_indice, c.mac`)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el mapa de puertos: %w", err)
	}
	defer filas.Close()

	mapa := []PuertoDeSwitch{}
	for filas.Next() {
		var p PuertoDeSwitch
		var activa, confirmado int
		if err := filas.Scan(&p.SwitchID, &p.SwitchNombre, &p.SwitchIP, &p.Indice,
			&p.Puerto, &p.Alias, &activa, &p.VelocidadMbps, &p.MAC, &p.EquipoID,
			&p.EquipoNombre, &p.EquipoIP, &confirmado, &p.CuantosEnBoca); err != nil {
			return nil, err
		}
		p.Activa = activa == 1
		p.Confirmado = confirmado == 1
		mapa = append(mapa, p)
	}
	return mapa, filas.Err()
}

// CalcularCapacidades averigua que puede saberse en esta red y lo guarda en su
// ficha, para que la interfaz lo diga sin rodeos en vez de mostrar pantallas
// vacias sin explicacion.
func (b *Base) CalcularCapacidades(ctx context.Context) (string, error) {
	var switches, bocasConfirmadas, bocasGrupo int

	if err := b.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM equipos_snmp WHERE es_switch = 1`).Scan(&switches); err != nil {
		return "", fmt.Errorf("no se pudieron contar los switches: %w", err)
	}
	if err := b.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(confirmado), 0), COALESCE(SUM(1 - confirmado), 0) FROM conexiones_puerto`).
		Scan(&bocasConfirmadas, &bocasGrupo); err != nil {
		return "", fmt.Errorf("no se pudieron contar las conexiones: %w", err)
	}

	capacidad := CapacidadNoDisponible
	switch {
	case switches == 0:
		capacidad = CapacidadNoDisponible
	case bocasConfirmadas > 0:
		capacidad = CapacidadExacta
	case bocasGrupo > 0:
		capacidad = CapacidadPorGrupo
	}

	_, err := b.ExecContext(ctx,
		`UPDATE ficha SET capacidad_mapa_puertos = ?, capacidades_al = ? WHERE id = 1`,
		capacidad, Ahora())
	if err != nil {
		return "", fmt.Errorf("no se pudo guardar el perfil de capacidades: %w", err)
	}
	return capacidad, nil
}
