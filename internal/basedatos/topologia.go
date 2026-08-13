package basedatos

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
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

// VecinoSNMP es un equipo anunciado en una boca, por LLDP o por CDP.
type VecinoSNMP struct {
	InterfazLocal string
	Nombre        string
	Descripcion   string
	PuertoRemoto  string
	ChasisID      string
	// Origen es "lldp" o "cdp"; vacio se guarda como "lldp". La tabla lo lleva en
	// su clave unica, asi que un mismo enlace visto por los dos protocolos ocupa
	// dos renglones en vez de pisarse.
	Origen string
	// DireccionIP es la que el vecino anuncia. La manda CDP; por LLDP suele venir
	// vacia.
	DireccionIP string
}

// Capacidad del mapa de puertos de una red.
const (
	CapacidadDesconocida  = "desconocida"
	CapacidadExacta       = "exacta"
	CapacidadPorGrupo     = "por_grupo"
	CapacidadNoDisponible = "no_disponible"
)

// MovimientoDePuerto es un equipo que cambio de boca entre dos consultas.
type MovimientoDePuerto struct {
	EquipoID int64
	Nombre   string
	Antes    string
	Ahora    string
}

// GuardarSNMP guarda lo que contestaron los equipos administrables y arma con
// eso el mapa de puertos.
//
// Devuelve tambien que equipos se cambiaron de boca: es un hecho que solo se
// puede ver aqui, comparando la foto anterior con la nueva, y de ahi sale la
// alerta de "se movio de lugar".
func (b *Base) GuardarSNMP(ctx context.Context, fichas []FichaSNMP) ([]MovimientoDePuerto, error) {
	momento := Ahora()
	var movimientos []MovimientoDePuerto

	err := b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		// Las MAC conocidas permiten enlazar cada boca con el equipo que ya se
		// descubrio, en vez de dejar una direccion suelta que no le dice nada a
		// nadie.
		porMAC, err := equiposPorMAC(ctx, tx)
		if err != nil {
			return err
		}

		// Donde estaba cada MAC antes de esta consulta. Se toma ANTES de tocar
		// nada: despues del borrado ya no hay con que comparar.
		antes, err := bocaPorMac(ctx, tx)
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

		despues, err := bocaPorMac(ctx, tx)
		if err != nil {
			return err
		}
		movimientos = compararBocas(ctx, tx, antes, despues, porMAC)
		return nil
	})

	return movimientos, err
}

// bocaPorMac dice en que boca esta cada MAC, como texto legible.
func bocaPorMac(ctx context.Context, tx *sql.Tx) (map[string]string, error) {
	filas, err := tx.QueryContext(ctx, `
		SELECT c.mac,
		       COALESCE(sw.alias, sw.nombre, sw.ip) || ' ' ||
		       COALESCE(i.nombre, i.descripcion, CAST(c.interfaz_indice AS TEXT))
		  FROM conexiones_puerto c
		  JOIN equipos sw ON sw.id = c.switch_id
		  LEFT JOIN interfaces i ON i.equipo_id = c.switch_id AND i.indice = c.interfaz_indice
		 WHERE c.confirmado = 1`)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer donde esta cada equipo: %w", err)
	}
	defer filas.Close()

	// Solo se miran las bocas CONFIRMADAS: en una boca con varios equipos no se
	// puede decir que alguno se movio, porque nunca se supo cual estaba donde.
	donde := map[string]string{}
	for filas.Next() {
		var mac, boca string
		if err := filas.Scan(&mac, &boca); err != nil {
			return nil, err
		}
		donde[mac] = boca
	}
	return donde, filas.Err()
}

func compararBocas(ctx context.Context, tx *sql.Tx, antes, despues map[string]string, porMAC map[string]int64) []MovimientoDePuerto {
	var movimientos []MovimientoDePuerto

	for mac, ahora := range despues {
		anterior, habia := antes[mac]
		if !habia || anterior == ahora {
			continue
		}

		equipoID, hay := porMAC[mac]
		if !hay {
			continue
		}
		nombre := mac
		tx.QueryRowContext(ctx,
			`SELECT COALESCE(alias, nombre, ip) FROM equipos WHERE id = ?`, equipoID).Scan(&nombre)

		movimientos = append(movimientos, MovimientoDePuerto{
			EquipoID: equipoID,
			Nombre:   nombre,
			Antes:    anterior,
			Ahora:    ahora,
		})
	}
	return movimientos
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
		// CDP no manda el chasis pero si la IP del vecino, que es lo unico con lo
		// que se le puede poner cara: sin esto, un enlace visto solo por CDP se
		// quedaria como un nombre suelto sin equipo al que colgarse.
		if vecinoID == nil && vecino.DireccionIP != "" {
			if id, hay := porIP(ctx, tx, vecino.DireccionIP); hay {
				vecinoID = id
			}
		}

		origen := vecino.Origen
		if origen == "" {
			origen = "lldp"
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO enlaces (equipo_id, interfaz_local, vecino_nombre, vecino_puerto,
			                     vecino_chasis, vecino_equipo_id, origen, ultima_vez)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (equipo_id, interfaz_local, origen) DO UPDATE SET
				vecino_nombre = excluded.vecino_nombre,
				vecino_puerto = excluded.vecino_puerto,
				vecino_chasis = excluded.vecino_chasis,
				vecino_equipo_id = excluded.vecino_equipo_id,
				ultima_vez = excluded.ultima_vez`,
			equipoID, vecino.InterfazLocal, nuloSiVacio(vecino.Nombre),
			nuloSiVacio(vecino.PuertoRemoto), nuloSiVacio(vecino.ChasisID), vecinoID,
			origen, momento)
		if err != nil {
			return fmt.Errorf("no se pudo guardar el enlace de %s: %w", vecino.InterfazLocal, err)
		}
	}
	return nil
}

// EnlaceEntreEquipos es un cable de switch a switch, anunciado por LLDP o CDP.
//
// Es lo que convierte una lista de switches sueltos en un arbol: sin esto no se
// sabe cual cuelga de cual, solo que existen.
type EnlaceEntreEquipos struct {
	EquipoID      int64  `json:"equipoId"`
	EquipoNombre  string `json:"equipoNombre"`
	InterfazLocal string `json:"interfazLocal"`
	VecinoNombre  string `json:"vecinoNombre"`
	VecinoPuerto  string `json:"vecinoPuerto"`
	VecinoID      *int64 `json:"vecinoId"`
	Origen        string `json:"origen"`
	UltimaVez     string `json:"ultimaVez"`
}

// Enlaces devuelve los cables entre equipos administrables.
//
// Un mismo cable puede venir dos veces —una por LLDP y otra por CDP— porque cada
// protocolo lo describe a su manera. Se devuelven los dos y se prefiere el
// confirmado por los dos lados: **el origen se conserva** para que la interfaz
// pueda decir de donde salio el dato en vez de presentarlo como verdad sin
// procedencia.
func (b *Base) Enlaces(ctx context.Context) ([]EnlaceEntreEquipos, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT e.equipo_id,
		       COALESCE(eq.alias, eq.nombre, eq.ip),
		       e.interfaz_local,
		       COALESCE(e.vecino_nombre, ''),
		       COALESCE(e.vecino_puerto, ''),
		       e.vecino_equipo_id,
		       e.origen,
		       e.ultima_vez
		  FROM enlaces e
		  JOIN equipos eq ON eq.id = e.equipo_id
		 ORDER BY eq.ip, e.interfaz_local, e.origen`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer los enlaces: %w", err)
	}
	defer filas.Close()

	enlaces := []EnlaceEntreEquipos{}
	for filas.Next() {
		var enlace EnlaceEntreEquipos
		if err := filas.Scan(&enlace.EquipoID, &enlace.EquipoNombre, &enlace.InterfazLocal,
			&enlace.VecinoNombre, &enlace.VecinoPuerto, &enlace.VecinoID,
			&enlace.Origen, &enlace.UltimaVez); err != nil {
			return nil, err
		}
		enlaces = append(enlaces, enlace)
	}
	return enlaces, filas.Err()
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

// ContadorPuerto son los bytes acumulados de una boca en un momento dado.
type ContadorPuerto struct {
	Indice         int
	Entrada        uint64
	Salida         uint64
	SesentaYCuatro bool
}

// PuntoTrafico es un renglon de la grafica de consumo de una boca.
type PuntoTrafico struct {
	Momento    string `json:"momento"`
	BpsEntrada int64  `json:"bpsEntrada"`
	BpsSalida  int64  `json:"bpsSalida"`
}

// ConsumoDePuerto es lo que gasta una boca, con quien cuelga de ella.
type ConsumoDePuerto struct {
	SwitchID      int64  `json:"switchId"`
	SwitchNombre  string `json:"switchNombre"`
	Indice        int    `json:"indice"`
	Puerto        string `json:"puerto"`
	EquipoNombre  string `json:"equipoNombre"`
	EquipoIP      string `json:"equipoIp"`
	Confirmado    bool   `json:"confirmado"`
	CuantosEnBoca int    `json:"cuantosEnBoca"`
	BpsEntrada    int64  `json:"bpsEntrada"`
	BpsSalida     int64  `json:"bpsSalida"`
	Momento       string `json:"momento"`
	// Estimado dice que la cifra sale de un muestreo (sFlow) y no de una cuenta.
	// Los contadores del switch y NetFlow cuentan; sFlow estima. Presentar las
	// dos igual seria hacer pasar una estimacion por una medicion.
	Estimado bool `json:"estimado"`
}

// GuardarTrafico anota los contadores y calcula la tasa contra la muestra
// anterior.
//
// Los contadores son acumulados y se desbordan; el dato util es la RESTA entre
// dos lecturas dividida por el tiempo. Una resta negativa significa que el
// contador se reinicio (el switch se reinicio, o el de 32 bits dio la vuelta):
// esa muestra se guarda sin tasa en vez de inventar un pico que no existio.
func (b *Base) GuardarTrafico(ctx context.Context, switchIP string, contadores []ContadorPuerto) error {
	if len(contadores) == 0 {
		return nil
	}
	momento := Ahora()

	return b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		switchID, hay := porIP(ctx, tx, switchIP)
		if !hay {
			return nil
		}

		for _, contador := range contadores {
			var antesMomento string
			var antesEntrada, antesSalida int64
			err := tx.QueryRowContext(ctx, `
				SELECT momento, bytes_entrada, bytes_salida
				  FROM muestras_trafico
				 WHERE switch_id = ? AND interfaz_indice = ?
				 ORDER BY id DESC LIMIT 1`, switchID, contador.Indice).
				Scan(&antesMomento, &antesEntrada, &antesSalida)

			var bpsEntrada, bpsSalida any
			if err == nil {
				entrada, salida, ok := calcularTasa(antesMomento, momento,
					antesEntrada, antesSalida, contador)
				if ok {
					bpsEntrada, bpsSalida = entrada, salida
				}
			}

			_, err = tx.ExecContext(ctx, `
				INSERT INTO muestras_trafico (switch_id, interfaz_indice, momento,
				                              bytes_entrada, bytes_salida, bps_entrada, bps_salida)
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				switchID, contador.Indice, momento,
				int64(contador.Entrada), int64(contador.Salida), bpsEntrada, bpsSalida)
			if err != nil {
				return fmt.Errorf("no se pudo guardar el trafico del puerto %d: %w", contador.Indice, err)
			}
		}
		return nil
	})
}

// calcularTasa convierte dos lecturas en bits por segundo.
func calcularTasa(antes, ahora string, antesEntrada, antesSalida int64, contador ContadorPuerto) (int64, int64, bool) {
	desde, err := time.Parse(time.RFC3339, antes)
	if err != nil {
		return 0, 0, false
	}
	hasta, err := time.Parse(time.RFC3339, ahora)
	if err != nil {
		return 0, 0, false
	}

	segundos := hasta.Sub(desde).Seconds()
	// Menos de un segundo entre muestras da divisiones absurdas; mas de un dia
	// promedia tanto que el numero deja de significar algo.
	if segundos < 1 || segundos > 86400 {
		return 0, 0, false
	}

	deltaEntrada := int64(contador.Entrada) - antesEntrada
	deltaSalida := int64(contador.Salida) - antesSalida
	if deltaEntrada < 0 || deltaSalida < 0 {
		// El contador se reinicio: no se inventa un pico.
		return 0, 0, false
	}

	return int64(float64(deltaEntrada) * 8 / segundos),
		int64(float64(deltaSalida) * 8 / segundos), true
}

// ConsumoActual devuelve cuanto gasta cada boca en su ultima medicion, con el
// equipo que cuelga de ella.
//
// Esto es lo que responde "quien se esta comiendo el internet" sin capturar un
// solo paquete: el switch ya llevaba la cuenta y MiRed ya sabia quien esta en
// cada boca.
func (b *Base) ConsumoActual(ctx context.Context) ([]ConsumoDePuerto, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT m.switch_id,
		       COALESCE(sw.alias, sw.nombre, sw.ip),
		       m.interfaz_indice,
		       COALESCE(i.nombre, i.descripcion, CAST(m.interfaz_indice AS TEXT)),
		       COALESCE(m.bps_entrada, 0), COALESCE(m.bps_salida, 0), m.momento,
		       COALESCE(eq.alias, eq.nombre, eq.ip, ''), COALESCE(eq.ip, ''),
		       COALESCE(MAX(c.confirmado), 0), COALESCE(COUNT(c.id), 0)
		  FROM muestras_trafico m
		  JOIN equipos sw ON sw.id = m.switch_id
		  LEFT JOIN interfaces i ON i.equipo_id = m.switch_id AND i.indice = m.interfaz_indice
		  LEFT JOIN conexiones_puerto c
		         ON c.switch_id = m.switch_id AND c.interfaz_indice = m.interfaz_indice
		  LEFT JOIN equipos eq ON eq.id = c.equipo_id
		 WHERE m.id IN (
		       SELECT MAX(id) FROM muestras_trafico GROUP BY switch_id, interfaz_indice)
		   AND m.bps_entrada IS NOT NULL
		 GROUP BY m.id
		 ORDER BY (COALESCE(m.bps_entrada, 0) + COALESCE(m.bps_salida, 0)) DESC`)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el consumo: %w", err)
	}
	defer filas.Close()

	consumo := []ConsumoDePuerto{}
	for filas.Next() {
		var c ConsumoDePuerto
		var confirmado int
		if err := filas.Scan(&c.SwitchID, &c.SwitchNombre, &c.Indice, &c.Puerto,
			&c.BpsEntrada, &c.BpsSalida, &c.Momento, &c.EquipoNombre, &c.EquipoIP,
			&confirmado, &c.CuantosEnBoca); err != nil {
			return nil, err
		}
		c.Confirmado = confirmado == 1
		consumo = append(consumo, c)
	}
	return consumo, filas.Err()
}

// HistorialTrafico devuelve los puntos de la grafica de una boca.
func (b *Base) HistorialTrafico(ctx context.Context, switchID int64, indice, limite int) ([]PuntoTrafico, error) {
	if limite <= 0 || limite > 1000 {
		limite = 200
	}

	filas, err := b.QueryContext(ctx, `
		SELECT momento, COALESCE(bps_entrada, 0), COALESCE(bps_salida, 0)
		  FROM muestras_trafico
		 WHERE switch_id = ? AND interfaz_indice = ? AND bps_entrada IS NOT NULL
		 ORDER BY id DESC LIMIT ?`, switchID, indice, limite)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el historial de trafico: %w", err)
	}
	defer filas.Close()

	puntos := []PuntoTrafico{}
	for filas.Next() {
		var punto PuntoTrafico
		if err := filas.Scan(&punto.Momento, &punto.BpsEntrada, &punto.BpsSalida); err != nil {
			return nil, err
		}
		puntos = append(puntos, punto)
	}
	return puntos, filas.Err()
}

// PodarTrafico borra las muestras viejas.
//
// Sin esto la tabla crece para siempre: una medicion por boca cada seis horas,
// con cuarenta y ocho bocas, son siete mil renglones al mes por switch. En una
// Raspberry eso importa.
func (b *Base) PodarTrafico(ctx context.Context, diasAConservar int) error {
	if diasAConservar <= 0 {
		diasAConservar = 90
	}
	limite := time.Now().AddDate(0, 0, -diasAConservar).Format(time.RFC3339)

	_, err := b.ExecContext(ctx, `DELETE FROM muestras_trafico WHERE momento < ?`, limite)
	return err
}

// ConsumoPorFlujo es lo que una direccion movio en un periodo, segun el router.
type ConsumoPorFlujo struct {
	IP             string
	BytesSube      uint64
	BytesBaja      uint64
	Conversaciones int
	// Estimado dice que la cifra viene de un muestreo (sFlow) y no de una cuenta
	// exacta. Se guarda con el dato porque despues ya no hay forma de saberlo.
	Estimado bool
}

// GuardarFlujos anota lo que el router reporto y lo enlaza con el equipo que ya
// se conocia.
//
// A diferencia de los contadores del switch, aqui NO hay que restar: el router
// manda lo que paso en el periodo, no un acumulado.
func (b *Base) GuardarFlujos(ctx context.Context, consumos []ConsumoPorFlujo) error {
	if len(consumos) == 0 {
		return nil
	}
	momento := Ahora()

	return b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		for _, consumo := range consumos {
			var equipoID any
			if id, hay := porIP(ctx, tx, consumo.IP); hay {
				equipoID = id
			}
			_, err := tx.ExecContext(ctx, `
				INSERT INTO trafico_flujos (equipo_id, ip, momento, bytes_sube, bytes_baja,
				                            conversaciones, estimado)
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				equipoID, consumo.IP, momento, int64(consumo.BytesSube),
				int64(consumo.BytesBaja), consumo.Conversaciones,
				boolAEntero(consumo.Estimado))
			if err != nil {
				return fmt.Errorf("no se pudo guardar el flujo de %s: %w", consumo.IP, err)
			}
		}
		return nil
	})
}

// ConsumoPorEquipo suma lo que cada equipo movio en las ultimas horas, segun los
// flujos del router.
//
// Es lo que responde "quien se esta comiendo el internet" en un sitio SIN
// switches administrables: no dice en que puerto esta, pero si cuanto gasta.
func (b *Base) ConsumoPorEquipo(ctx context.Context, horas int) ([]ConsumoDePuerto, error) {
	if horas <= 0 || horas > 720 {
		horas = 24
	}
	desde := time.Now().Add(-time.Duration(horas) * time.Hour).Format(time.RFC3339)

	filas, err := b.QueryContext(ctx, `
		SELECT f.ip,
		       COALESCE(e.alias, e.nombre, f.ip),
		       SUM(f.bytes_sube), SUM(f.bytes_baja), MAX(f.momento),
		       MAX(f.estimado)
		  FROM trafico_flujos f
		  LEFT JOIN equipos e ON e.id = f.equipo_id
		 WHERE f.momento >= ?
		 GROUP BY f.ip
		 ORDER BY (SUM(f.bytes_sube) + SUM(f.bytes_baja)) DESC
		 LIMIT 100`, desde)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el consumo por flujos: %w", err)
	}
	defer filas.Close()

	// Se reusa la misma forma que el consumo por puerto para que la interfaz no
	// tenga que pintar dos cosas distintas: lo que cambia es de donde salio el
	// numero, no que significa.
	consumo := []ConsumoDePuerto{}
	for filas.Next() {
		var c ConsumoDePuerto
		var sube, baja int64
		var estimado int
		if err := filas.Scan(&c.EquipoIP, &c.EquipoNombre, &sube, &baja, &c.Momento,
			&estimado); err != nil {
			return nil, err
		}
		// Basta con que UNA de las mediciones del periodo venga de un muestreo
		// para que la suma sea una estimacion. Redondear hacia "es exacto" seria
		// mentir por comodidad.
		c.Estimado = estimado == 1
		// Los flujos son bytes del periodo; se muestran como bits para poder
		// compararlos con lo del switch.
		c.BpsSalida = sube * 8
		c.BpsEntrada = baja * 8
		c.Puerto = "por el router"
		c.SwitchNombre = "Router"
		if c.Estimado {
			c.Puerto = "por el router (muestreo)"
		}
		consumo = append(consumo, c)
	}
	return consumo, filas.Err()
}

// PodarFlujos borra las mediciones viejas de flujos.
func (b *Base) PodarFlujos(ctx context.Context, diasAConservar int) error {
	if diasAConservar <= 0 {
		diasAConservar = 30
	}
	limite := time.Now().AddDate(0, 0, -diasAConservar).Format(time.RFC3339)

	_, err := b.ExecContext(ctx, `DELETE FROM trafico_flujos WHERE momento < ?`, limite)
	return err
}

// SubredesDeTodasLasRedes devuelve, para el receptor de flujos, que rangos
// pertenecen a que red.
//
// El receptor escucha un solo puerto para TODAS las redes: el router no dice a
// que red de MiRed pertenece un flujo, asi que hay que resolverlo por la
// direccion.
func (e *Enrutador) SubredesDeTodasLasRedes(ctx context.Context) (map[string][]string, error) {
	redes, err := e.ListarRedes(ctx)
	if err != nil {
		return nil, err
	}

	porRed := map[string][]string{}
	for _, red := range redes {
		clave := red.Clave
		err := e.ConRed(ctx, clave, func(base *Base) error {
			subredes, err := base.ListarSubredes(ctx)
			if err != nil {
				return err
			}
			for _, subred := range subredes {
				porRed[clave] = append(porRed[clave], subred.CIDR)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return porRed, nil
}
