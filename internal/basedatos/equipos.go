package basedatos

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrEquipoNoExiste lo devuelven las operaciones sobre un equipo que ya no esta.
var ErrEquipoNoExiste = errors.New("el equipo no existe")

// Equipo es un aparato descubierto en una red.
type Equipo struct {
	ID         int64    `json:"id"`
	Identidad  string   `json:"identidad"`
	IP         string   `json:"ip"`
	MAC        string   `json:"mac"`
	Fabricante string   `json:"fabricante"`
	Nombre     string   `json:"nombre"`
	Alias      string   `json:"alias"`
	Tipo       string   `json:"tipo"`
	Subred     string   `json:"subred"`
	Metodo     string   `json:"metodo"`
	Presente   bool     `json:"presente"`
	PrimeraVez string   `json:"primeraVez"`
	UltimaVez  string   `json:"ultimaVez"`
	Puertos    []Puerto `json:"puertos"`
	// Huella es lo que el aparato conto de si mismo: el titulo de su pagina, su
	// certificado, lo que anuncia por mDNS o UPnP y lo que contesta al protocolo
	// de su fabricante. Con la fuente de cada cosa, para poder decir de donde
	// salio en vez de presentarlo todo como igual de firme.
	Huella []DatoHuella `json:"huella,omitempty"`
	// Modelo y Notas los escribe una persona. No salen de ningun barrido: son lo
	// que sabe quien tiene el aparato delante.
	Modelo string `json:"modelo"`
	Notas  string `json:"notas"`
	// Origen es "descubierto" o "manual". Un switch no administrable NUNCA va a
	// salir en un escaneo —no tiene direccion—, y sin esto no habria forma de
	// distinguir "no contesto" de "no existe".
	Origen string `json:"origen"`
	// Conexion es "cable" o "wifi", y solo aplica a equipos terminales. Vacio en
	// un switch o un router, donde no significaria nada.
	Conexion string `json:"conexion"`
	// Categoria es la clave de la lista unica de MiRed (ver internal/catalogo).
	//
	// No es lo mismo que Tipo: Tipo es el nombre para leer ("Impresora HP") y
	// Categoria es para CONTAR ("impresora"). Agrupando por Tipo salen cubos
	// separados para "Impresora HP" y "Impresora de red".
	Categoria string `json:"categoria"`
}

// ComoSeLlama devuelve el nombre que conviene mostrar: manda el que puso una
// persona sobre el que se descubrio, y si no hay ninguno, la IP.
func (e Equipo) ComoSeLlama() string {
	switch {
	case e.Alias != "":
		return e.Alias
	case e.Nombre != "":
		return e.Nombre
	default:
		return e.IP
	}
}

// Puerto es un puerto abierto de un equipo.
type Puerto struct {
	Numero    int    `json:"numero"`
	Protocolo string `json:"protocolo"`
	Servicio  string `json:"servicio"`
	Banner    string `json:"banner"`
	Abierto   bool   `json:"abierto"`
	UltimaVez string `json:"ultimaVez"`
}

// EquipoDescubierto es lo que trae un barrido de la sonda, antes de guardarse.
type EquipoDescubierto struct {
	IP         string
	MAC        string
	Nombre     string
	Fabricante string
	Metodo     string
	Subred     string
	Puertos    []PuertoDescubierto
	// Huella es lo que el aparato dijo de si mismo: el titulo de su pagina, su
	// certificado, lo que anuncia por mDNS o UPnP, y lo que contesta al
	// protocolo propio de su fabricante.
	Huella []DatoHuella
}

// DatoHuella es una cosa que un aparato conto de si mismo, con la fuente de
// donde se supo.
type DatoHuella struct {
	Fuente string `json:"fuente"`
	Clave  string `json:"clave"`
	Valor  string `json:"valor"`
}

// PuertoDescubierto es un puerto que contesto durante el barrido.
type PuertoDescubierto struct {
	Numero    int
	Protocolo string
	Servicio  string
	Banner    string
}

// ResumenEscaneo es lo que dejo una corrida.
type ResumenEscaneo struct {
	EscaneoID int64 `json:"escaneoId"`
	Vistos    int   `json:"vistos"`
	Nuevos    int   `json:"nuevos"`
	Ausentes  int   `json:"ausentes"`
}

// IniciarEscaneo deja constancia de que empezo un barrido, antes de que termine.
//
// Se anota al empezar y no al terminar para que un escaneo que se cae a la mitad
// deje rastro: si solo se anotara al final, un barrido que nunca termina seria
// invisible.
func (b *Base) IniciarEscaneo(ctx context.Context, tipo string) (int64, error) {
	resultado, err := b.ExecContext(ctx,
		`INSERT INTO escaneos (tipo, iniciado, estado) VALUES (?, ?, 'en_curso')`,
		tipo, Ahora())
	if err != nil {
		return 0, fmt.Errorf("no se pudo registrar el escaneo: %w", err)
	}
	return resultado.LastInsertId()
}

// FallarEscaneo marca un barrido que no pudo completarse.
func (b *Base) FallarEscaneo(ctx context.Context, escaneoID int64, motivo string) error {
	_, err := b.ExecContext(ctx,
		`UPDATE escaneos SET estado = 'fallido', terminado = ?, detalle = ? WHERE id = ?`,
		Ahora(), motivo, escaneoID)
	return err
}

// GuardarDescubrimiento guarda lo que vio un barrido: da de alta lo nuevo,
// actualiza lo conocido y marca como ausente lo que ya no contesto.
//
// Va entero en una transaccion: media foto de la red es peor que ninguna, porque
// las alertas de la fase 7 comparan contra la anterior y una foto a medias
// avisaria de equipos "desaparecidos" que solo se quedaron fuera del corte.
func (b *Base) GuardarDescubrimiento(ctx context.Context, escaneoID int64, profundo bool, equipos []EquipoDescubierto) (ResumenEscaneo, error) {
	resumen := ResumenEscaneo{EscaneoID: escaneoID, Vistos: len(equipos)}
	momento := Ahora()

	err := b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		vistos := make(map[int64]bool, len(equipos))

		for _, visto := range equipos {
			identidad := identidadDe(visto)
			equipoID, esNuevo, err := guardarEquipo(ctx, tx, identidad, visto, momento)
			if err != nil {
				return err
			}
			vistos[equipoID] = true
			if esNuevo {
				resumen.Nuevos++
			}

			if profundo {
				if err := guardarPuertos(ctx, tx, escaneoID, equipoID, visto.Puertos, momento); err != nil {
					return err
				}
				if err := guardarHuella(ctx, tx, equipoID, visto.Huella, momento); err != nil {
					return err
				}
			}

			_, err = tx.ExecContext(ctx, `
				INSERT INTO escaneo_equipos (escaneo_id, equipo_id, ip, nuevo)
				VALUES (?, ?, ?, ?)
				ON CONFLICT (escaneo_id, equipo_id) DO NOTHING`,
				escaneoID, equipoID, visto.IP, boolAEntero(esNuevo))
			if err != nil {
				return fmt.Errorf("no se pudo anotar el equipo en el escaneo: %w", err)
			}
		}

		// Lo que no contesto queda como ausente, no se borra: que un equipo se
		// apague no significa que deje de existir, y su historia es justamente
		// lo que despues permite avisar de que lleva tres dias sin aparecer.
		ausentes, err := marcarAusentes(ctx, tx, escaneoID, momento)
		if err != nil {
			return err
		}
		resumen.Ausentes = ausentes

		_, err = tx.ExecContext(ctx, `
			UPDATE escaneos
			   SET estado = 'terminado', terminado = ?, equipos_vistos = ?, equipos_nuevos = ?
			 WHERE id = ?`,
			momento, resumen.Vistos, resumen.Nuevos, escaneoID)
		if err != nil {
			return fmt.Errorf("no se pudo cerrar el escaneo: %w", err)
		}
		return nil
	})

	return resumen, err
}

func identidadDe(visto EquipoDescubierto) string {
	mac := strings.ToLower(strings.TrimSpace(visto.MAC))
	if mac != "" {
		return mac
	}
	// Sin MAC, la IP es lo unico que hay. Se marca con prefijo para que se vea
	// de un vistazo que ese equipo se identifica por algo que puede cambiar.
	return "ip:" + visto.IP
}

func guardarEquipo(ctx context.Context, tx *sql.Tx, identidad string, visto EquipoDescubierto, momento string) (int64, bool, error) {
	var equipoID int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM equipos WHERE identidad = ? AND estatus >= 0`, identidad).Scan(&equipoID)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		resultado, err := tx.ExecContext(ctx, `
			INSERT INTO equipos (identidad, ip, mac, fabricante, nombre, subred, metodo,
			                     presente, primera_vez, ultima_vez, estatus)
			VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?, 1)`,
			identidad, visto.IP, nuloSiVacio(visto.MAC), nuloSiVacio(visto.Fabricante),
			nuloSiVacio(visto.Nombre), nuloSiVacio(visto.Subred), visto.Metodo,
			momento, momento)
		if err != nil {
			return 0, false, fmt.Errorf("no se pudo guardar el equipo %s: %w", visto.IP, err)
		}
		nuevoID, _ := resultado.LastInsertId()
		if err := anotarPresencia(ctx, tx, nuevoID, momento, true, visto.IP); err != nil {
			return 0, false, err
		}
		return nuevoID, true, nil

	case err != nil:
		return 0, false, fmt.Errorf("no se pudo buscar el equipo %s: %w", visto.IP, err)
	}

	// Si estaba ausente y volvio, eso es un evento: es la mitad de lo que
	// despues responde "¿a que hora llega el celular de fulano?".
	var estaba int
	if err := tx.QueryRowContext(ctx, `SELECT presente FROM equipos WHERE id = ?`, equipoID).Scan(&estaba); err == nil && estaba == 0 {
		if err := anotarPresencia(ctx, tx, equipoID, momento, true, visto.IP); err != nil {
			return 0, false, err
		}
	}

	// COALESCE conserva lo que ya se sabia cuando este barrido no lo trae: un
	// escaneo rapido no debe borrar el nombre que encontro uno profundo.
	_, err = tx.ExecContext(ctx, `
		UPDATE equipos
		   SET ip = ?,
		       mac = COALESCE(NULLIF(?, ''), mac),
		       fabricante = COALESCE(NULLIF(?, ''), fabricante),
		       nombre = COALESCE(NULLIF(?, ''), nombre),
		       subred = COALESCE(NULLIF(?, ''), subred),
		       metodo = ?,
		       presente = 1,
		       ultima_vez = ?,
		       modificado = ?
		 WHERE id = ?`,
		visto.IP, visto.MAC, visto.Fabricante, visto.Nombre, visto.Subred,
		visto.Metodo, momento, momento, equipoID)
	if err != nil {
		return 0, false, fmt.Errorf("no se pudo actualizar el equipo %s: %w", visto.IP, err)
	}
	return equipoID, false, nil
}

// guardarHuella suma lo que el aparato conto de si mismo.
//
// Se SUMA, no se reemplaza: cada fuente contesta cuando quiere —el aparato pudo
// no estar de humor para el mDNS de este barrido— y borrar lo de ayer porque hoy
// no volvio a decirlo dejaria la ficha parpadeando. La fecha dice de cuando es
// cada cosa.
func guardarHuella(ctx context.Context, tx *sql.Tx, equipoID int64, datos []DatoHuella, momento string) error {
	for _, dato := range datos {
		if dato.Fuente == "" || dato.Clave == "" || dato.Valor == "" {
			continue
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO huellas (equipo_id, fuente, clave, valor, ultima_vez)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (equipo_id, fuente, clave, valor) DO UPDATE SET
				ultima_vez = excluded.ultima_vez`,
			equipoID, dato.Fuente, dato.Clave, dato.Valor, momento)
		if err != nil {
			return fmt.Errorf("no se pudo guardar la huella %s/%s: %w", dato.Fuente, dato.Clave, err)
		}
	}
	return nil
}

// HuellaDe devuelve lo que un equipo conto de si mismo.
func (b *Base) HuellaDe(ctx context.Context, equipoID int64) ([]DatoHuella, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT fuente, clave, valor FROM huellas
		 WHERE equipo_id = ?
		 ORDER BY fuente, clave, valor`, equipoID)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer la huella: %w", err)
	}
	defer filas.Close()

	datos := []DatoHuella{}
	for filas.Next() {
		var dato DatoHuella
		if err := filas.Scan(&dato.Fuente, &dato.Clave, &dato.Valor); err != nil {
			return nil, err
		}
		datos = append(datos, dato)
	}
	return datos, filas.Err()
}

func guardarPuertos(ctx context.Context, tx *sql.Tx, escaneoID, equipoID int64, puertos []PuertoDescubierto, momento string) error {
	for _, puerto := range puertos {
		protocolo := puerto.Protocolo
		if protocolo == "" {
			protocolo = "tcp"
		}
		// escaneo_creado NO se toca al actualizar: dice cuando aparecio por
		// primera vez, y eso es lo que despues distingue un puerto nuevo de uno
		// que lleva ahi meses.
		_, err := tx.ExecContext(ctx, `
			INSERT INTO puertos (equipo_id, numero, protocolo, servicio, banner,
			                     abierto, primera_vez, ultima_vez, escaneo_creado)
			VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?)
			ON CONFLICT (equipo_id, numero, protocolo) DO UPDATE SET
				servicio = COALESCE(NULLIF(excluded.servicio, ''), puertos.servicio),
				banner = COALESCE(NULLIF(excluded.banner, ''), puertos.banner),
				abierto = 1,
				ultima_vez = excluded.ultima_vez`,
			equipoID, puerto.Numero, protocolo, nuloSiVacio(puerto.Servicio),
			nuloSiVacio(puerto.Banner), momento, momento, escaneoID)
		if err != nil {
			return fmt.Errorf("no se pudo guardar el puerto %d: %w", puerto.Numero, err)
		}
	}

	// Un puerto que estaba abierto y ya no contesta se marca cerrado, pero no se
	// borra: "este equipo tenia abierto el 3389 y ya no" es informacion, y en la
	// fase 7 se vuelve alerta.
	//
	// Se cierra por LISTA de lo que se vio, no comparando marcas de tiempo: dos
	// escaneos dentro del mismo segundo tienen la misma marca y la comparacion
	// no cerraria nada. La lista siempre dice la verdad.
	argumentos := []any{equipoID}
	consulta := `UPDATE puertos SET abierto = 0 WHERE equipo_id = ? AND abierto = 1`
	if len(puertos) > 0 {
		consulta += ` AND (numero, protocolo) NOT IN (VALUES `
		for i, puerto := range puertos {
			if i > 0 {
				consulta += `, `
			}
			consulta += `(?, ?)`
			protocolo := puerto.Protocolo
			if protocolo == "" {
				protocolo = "tcp"
			}
			argumentos = append(argumentos, puerto.Numero, protocolo)
		}
		consulta += `)`
	}

	_, err := tx.ExecContext(ctx, consulta, argumentos...)
	return err
}

func marcarAusentes(ctx context.Context, tx *sql.Tx, escaneoID int64, momento string) (int, error) {
	// Primero se ve QUIENES se van a marcar ausentes, porque despues del UPDATE
	// ya no hay forma de saber cuales cambiaron, y cada uno necesita su evento.
	// Los equipos DECLARADOS a mano quedan fuera: un switch no administrable no
	// tiene direccion, no contesta a nada y ningun barrido lo va a ver nunca.
	// Sin esta linea se marcaria ausente en cada escaneo, generaria un evento de
	// presencia cada vez y la alerta de "lleva dias sin aparecer" avisaria de un
	// aparato que esta ahi, atornillado a la pared.
	filas, err := tx.QueryContext(ctx, `
		SELECT id, ip FROM equipos
		 WHERE estatus = 1 AND presente = 1 AND origen <> 'manual'
		   AND id NOT IN (SELECT equipo_id FROM escaneo_equipos WHERE escaneo_id = ?)`,
		escaneoID)
	if err != nil {
		return 0, fmt.Errorf("no se pudieron buscar los equipos ausentes: %w", err)
	}

	type ausente struct {
		id int64
		ip string
	}
	var ausentes []ausente
	for filas.Next() {
		var quien ausente
		if err := filas.Scan(&quien.id, &quien.ip); err != nil {
			filas.Close()
			return 0, err
		}
		ausentes = append(ausentes, quien)
	}
	filas.Close()
	if err := filas.Err(); err != nil {
		return 0, err
	}
	if len(ausentes) == 0 {
		return 0, nil
	}

	for _, quien := range ausentes {
		if _, err := tx.ExecContext(ctx,
			`UPDATE equipos SET presente = 0, modificado = ? WHERE id = ?`,
			momento, quien.id); err != nil {
			return 0, fmt.Errorf("no se pudo marcar ausente el equipo %d: %w", quien.id, err)
		}
		if err := anotarPresencia(ctx, tx, quien.id, momento, false, quien.ip); err != nil {
			return 0, err
		}
	}
	return len(ausentes), nil
}

// anotarPresencia guarda un cambio de estado. Solo se llama cuando el estado
// cambia de verdad: guardar en cada barrido serian mil renglones diarios por
// equipo para decir lo mismo.
func anotarPresencia(ctx context.Context, tx *sql.Tx, equipoID int64, momento string, presente bool, ip string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO eventos_presencia (equipo_id, momento, presente, ip) VALUES (?, ?, ?, ?)`,
		equipoID, momento, boolAEntero(presente), nuloSiVacio(ip))
	if err != nil {
		return fmt.Errorf("no se pudo anotar el cambio de presencia: %w", err)
	}
	return nil
}

// EventoPresencia es una conexion o una desconexion de un equipo.
type EventoPresencia struct {
	Momento  string `json:"momento"`
	Presente bool   `json:"presente"`
	IP       string `json:"ip"`
}

// ListarPresencia devuelve el historial de un equipo, del mas reciente al mas
// viejo.
func (b *Base) ListarPresencia(ctx context.Context, equipoID int64, limite int) ([]EventoPresencia, error) {
	if limite <= 0 || limite > 500 {
		limite = 100
	}

	filas, err := b.QueryContext(ctx, `
		SELECT momento, presente, COALESCE(ip, '')
		  FROM eventos_presencia
		 WHERE equipo_id = ?
		 ORDER BY id DESC
		 LIMIT ?`, equipoID, limite)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el historial de presencia: %w", err)
	}
	defer filas.Close()

	eventos := []EventoPresencia{}
	for filas.Next() {
		var evento EventoPresencia
		var presente int
		if err := filas.Scan(&evento.Momento, &presente, &evento.IP); err != nil {
			return nil, err
		}
		evento.Presente = presente == 1
		eventos = append(eventos, evento)
	}
	return eventos, filas.Err()
}

// ListarEquipos devuelve los equipos de la red con sus puertos abiertos.
func (b *Base) ListarEquipos(ctx context.Context, soloPresentes bool) ([]Equipo, error) {
	consulta := `
		SELECT id, identidad, ip, COALESCE(mac, ''), COALESCE(fabricante, ''),
		       COALESCE(nombre, ''), COALESCE(alias, ''), COALESCE(tipo, ''),
		       COALESCE(subred, ''), COALESCE(metodo, ''), presente,
		       primera_vez, ultima_vez, COALESCE(modelo, ''), COALESCE(notas, ''),
		       origen, COALESCE(conexion, ''), COALESCE(categoria, '')
		  FROM equipos
		 WHERE estatus = 1`
	if soloPresentes {
		consulta += ` AND presente = 1`
	}
	consulta += ` ORDER BY presente DESC, ip`

	filas, err := b.QueryContext(ctx, consulta)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar los equipos: %w", err)
	}
	defer filas.Close()

	equipos := []Equipo{}
	porID := map[int64]int{}
	for filas.Next() {
		var e Equipo
		var presente int
		if err := filas.Scan(&e.ID, &e.Identidad, &e.IP, &e.MAC, &e.Fabricante,
			&e.Nombre, &e.Alias, &e.Tipo, &e.Subred, &e.Metodo, &presente,
			&e.PrimeraVez, &e.UltimaVez, &e.Modelo, &e.Notas, &e.Origen,
			&e.Conexion, &e.Categoria); err != nil {
			return nil, err
		}
		e.Presente = presente == 1
		e.Puertos = []Puerto{}
		porID[e.ID] = len(equipos)
		equipos = append(equipos, e)
	}
	if err := filas.Err(); err != nil {
		return nil, err
	}
	if len(equipos) == 0 {
		return equipos, nil
	}

	// Los puertos se traen de una sola consulta y se reparten en memoria: una
	// consulta por equipo serian cientos de idas a la base para pintar una
	// pantalla.
	filasPuertos, err := b.QueryContext(ctx, `
		SELECT equipo_id, numero, protocolo, COALESCE(servicio, ''), COALESCE(banner, ''),
		       abierto, ultima_vez
		  FROM puertos
		 WHERE abierto = 1
		 ORDER BY numero`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar los puertos: %w", err)
	}
	defer filasPuertos.Close()

	for filasPuertos.Next() {
		var equipoID int64
		var p Puerto
		var abierto int
		if err := filasPuertos.Scan(&equipoID, &p.Numero, &p.Protocolo, &p.Servicio,
			&p.Banner, &abierto, &p.UltimaVez); err != nil {
			return nil, err
		}
		p.Abierto = abierto == 1
		if indice, hay := porID[equipoID]; hay {
			equipos[indice].Puertos = append(equipos[indice].Puertos, p)
		}
	}
	if err := filasPuertos.Err(); err != nil {
		return nil, err
	}

	// Lo mismo con las huellas: una sola consulta y se reparten en memoria.
	filasHuella, err := b.QueryContext(ctx, `
		SELECT equipo_id, fuente, clave, valor
		  FROM huellas
		 ORDER BY fuente, clave, valor`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar las huellas: %w", err)
	}
	defer filasHuella.Close()

	for filasHuella.Next() {
		var equipoID int64
		var dato DatoHuella
		if err := filasHuella.Scan(&equipoID, &dato.Fuente, &dato.Clave, &dato.Valor); err != nil {
			return nil, err
		}
		if indice, hay := porID[equipoID]; hay {
			equipos[indice].Huella = append(equipos[indice].Huella, dato)
		}
	}
	return equipos, filasHuella.Err()
}

// PonerAlias le pone a un equipo el nombre que una persona quiera.
func (b *Base) PonerAlias(ctx context.Context, equipoID int64, alias string) error {
	resultado, err := b.ExecContext(ctx,
		`UPDATE equipos SET alias = ?, modificado = ? WHERE id = ? AND estatus = 1`,
		nuloSiVacio(strings.TrimSpace(alias)), Ahora(), equipoID)
	if err != nil {
		return fmt.Errorf("no se pudo guardar el nombre: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return ErrEquipoNoExiste
	}
	return nil
}

// ResumenDeRed cuenta lo que el catalogo necesita para el panel de inicio, sin
// que este tenga que abrir el archivo de cada red al pintarlo.
func (b *Base) ResumenDeRed(ctx context.Context) (equipos, presentes int, ultimoEscaneo string, err error) {
	err = b.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(presente), 0) FROM equipos WHERE estatus = 1`).
		Scan(&equipos, &presentes)
	if err != nil {
		return 0, 0, "", fmt.Errorf("no se pudo contar los equipos: %w", err)
	}

	var ultimo sql.NullString
	err = b.QueryRowContext(ctx,
		`SELECT MAX(terminado) FROM escaneos WHERE estado = 'terminado'`).Scan(&ultimo)
	if err != nil {
		return equipos, presentes, "", fmt.Errorf("no se pudo leer el ultimo escaneo: %w", err)
	}
	return equipos, presentes, ultimo.String, nil
}

// ActualizarResumen guarda en el catalogo el resumen de una red.
func (e *Enrutador) ActualizarResumen(ctx context.Context, clave string, equipos, presentes int, ultimoEscaneo string) error {
	_, err := e.Catalogo.ExecContext(ctx, `
		UPDATE redes
		   SET equipos = ?, equipos_presentes = ?, ultimo_escaneo = ?, resumen_al = ?
		 WHERE clave = ?`,
		equipos, presentes, nuloSiVacio(ultimoEscaneo), Ahora(), clave)
	if err != nil {
		return fmt.Errorf("no se pudo actualizar el resumen de la red: %w", err)
	}
	return nil
}

func nuloSiVacio(texto string) any {
	if strings.TrimSpace(texto) == "" {
		return nil
	}
	return texto
}

// DatosParaReconocer es todo lo que se sabe de un equipo, para que el catalogo
// decida que es.
type DatosParaReconocer struct {
	ID         int64
	IP         string
	MAC        string
	Fabricante string
	Nombre     string
	Puertos    []int
	Banners    []string
	SnmpDescr  string
	// Huella es todo lo que el aparato conto de si mismo, junto en una linea.
	Huella string
	// Modelo es lo que alguna de esas fuentes dijo que era el modelo.
	Modelo string
}

// ParaReconocer junta lo que el catalogo necesita de cada equipo.
//
// Se arma desde la base y no desde el resultado del escaneo a proposito: asi el
// reconocimiento usa TODO lo que se sabe del equipo (incluido lo que dijo por
// SNMP y lo que trajo un barrido anterior), no solo lo de la ultima corrida.
func (b *Base) ParaReconocer(ctx context.Context) ([]DatosParaReconocer, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT e.id, e.ip, COALESCE(e.mac, ''), COALESCE(e.fabricante, ''),
		       COALESCE(e.nombre, ''), COALESCE(s.descripcion, '')
		  FROM equipos e
		  LEFT JOIN equipos_snmp s ON s.equipo_id = e.id
		 WHERE e.estatus = 1`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer los equipos a reconocer: %w", err)
	}
	defer filas.Close()

	equipos := []DatosParaReconocer{}
	porID := map[int64]int{}
	for filas.Next() {
		var d DatosParaReconocer
		if err := filas.Scan(&d.ID, &d.IP, &d.MAC, &d.Fabricante, &d.Nombre, &d.SnmpDescr); err != nil {
			return nil, err
		}
		porID[d.ID] = len(equipos)
		equipos = append(equipos, d)
	}
	if err := filas.Err(); err != nil {
		return nil, err
	}
	if len(equipos) == 0 {
		return equipos, nil
	}

	puertos, err := b.QueryContext(ctx,
		`SELECT equipo_id, numero, COALESCE(banner, '') FROM puertos WHERE abierto = 1`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer los puertos: %w", err)
	}
	defer puertos.Close()

	for puertos.Next() {
		var equipoID int64
		var numero int
		var banner string
		if err := puertos.Scan(&equipoID, &numero, &banner); err != nil {
			return nil, err
		}
		if indice, hay := porID[equipoID]; hay {
			equipos[indice].Puertos = append(equipos[indice].Puertos, numero)
			if banner != "" {
				equipos[indice].Banners = append(equipos[indice].Banners, banner)
			}
		}
	}
	if err := puertos.Err(); err != nil {
		return nil, err
	}

	// Y lo que cada aparato conto de si mismo. Es la senal que distingue un
	// modem de una television cuando los dos solo tienen el 80 abierto.
	huellas, err := b.QueryContext(ctx, `SELECT equipo_id, clave, valor FROM huellas`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer las huellas: %w", err)
	}
	defer huellas.Close()

	for huellas.Next() {
		var equipoID int64
		var clave, valor string
		if err := huellas.Scan(&equipoID, &clave, &valor); err != nil {
			return nil, err
		}
		indice, hay := porID[equipoID]
		if !hay {
			continue
		}
		if equipos[indice].Huella != "" {
			equipos[indice].Huella += " "
		}
		equipos[indice].Huella += valor
		if clave == "modelo" && equipos[indice].Modelo == "" {
			equipos[indice].Modelo = valor
		}
	}
	return equipos, huellas.Err()
}

// Reconocido es lo que el catalogo averiguo de un equipo.
//
// Son dos datos distintos y hacen falta los dos: Tipo es el nombre para LEER
// ("Impresora HP") y Categoria es la clave para CONTAR ("impresora"). Con solo
// el nombre, el contador saca cubos separados para "Impresora HP" y "Impresora
// de red"; con solo la categoria, la ficha del equipo pierde el detalle.
type Reconocido struct {
	Tipo      string
	Categoria string
}

// PonerTipos guarda lo que el catalogo reconocio.
//
// Solo se escribe cuando algo CAMBIA: reescribir la misma fila en cada escaneo
// ensucia la columna de modificado y no aporta nada.
//
// **Nunca toca lo declarado a mano.** Si una persona dijo que ese aparato es un
// switch no administrable, el catalogo no tiene nada que corregirle: el catalogo
// deduce por puertos abiertos, y quien lo declaro lo tenia delante.
func (b *Base) PonerTipos(ctx context.Context, tipos map[int64]Reconocido) (int, error) {
	if len(tipos) == 0 {
		return 0, nil
	}

	cambiados := 0
	momento := Ahora()
	err := b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		for id, reconocido := range tipos {
			resultado, err := tx.ExecContext(ctx,
				`UPDATE equipos SET tipo = ?, categoria = ?, modificado = ?
				  WHERE id = ? AND origen <> 'manual'
				    AND (COALESCE(tipo, '') <> ? OR COALESCE(categoria, '') <> ?)`,
				nuloSiVacio(reconocido.Tipo), nuloSiVacio(reconocido.Categoria), momento,
				id, reconocido.Tipo, reconocido.Categoria)
			if err != nil {
				return fmt.Errorf("no se pudo guardar el tipo del equipo %d: %w", id, err)
			}
			if filas, _ := resultado.RowsAffected(); filas > 0 {
				cambiados++
			}
		}
		return nil
	})
	return cambiados, err
}

// MarcarSwitchesAdministrables pone la categoria de los equipos que contestaron
// por SNMP que son switches.
//
// Esto NO lo puede hacer el catalogo: el catalogo deduce por puertos abiertos y
// fabricante, y un switch administrable **lo dijo el mismo**. Un dato que da el
// aparato manda sobre uno deducido, asi que se aplica despues del catalogo y lo
// pisa.
func (b *Base) MarcarSwitchesAdministrables(ctx context.Context, categoria string) (int, error) {
	resultado, err := b.ExecContext(ctx, `
		UPDATE equipos SET categoria = ?, modificado = ?
		 WHERE estatus = 1 AND origen <> 'manual'
		   AND COALESCE(categoria, '') <> ?
		   AND id IN (SELECT equipo_id FROM equipos_snmp WHERE es_switch = 1)`,
		categoria, Ahora(), categoria)
	if err != nil {
		return 0, fmt.Errorf("no se pudieron marcar los switches administrables: %w", err)
	}
	cambiados, _ := resultado.RowsAffected()
	return int(cambiados), nil
}

// CuentaPorCategoria es cuantos aparatos de un tipo hay en la red.
type CuentaPorCategoria struct {
	Categoria string `json:"categoria"`
	Cuantos   int    `json:"cuantos"`
	Presentes int    `json:"presentes"`
	// Declarados son los que puso una persona a mano. Se dice aparte porque no
	// los vio ningun escaneo: la cuenta es igual de real, pero no viene de una
	// medicion.
	Declarados int `json:"declarados"`
}

// ResumenDeCategorias cuenta de que esta hecha la red.
//
// Sale de la MISMA tabla que la lista de equipos y que el mapa, asi que los tres
// no pueden discrepar: un switch declarado a mano cuenta aqui en cuanto se
// declara, sin nada que sincronizar.
//
// Los equipos sin categoria se devuelven agrupados bajo la clave que se le pase
// —"sin reconocer"—, **nunca se omiten**: un inventario que calla lo que no supo
// clasificar se lee como si estuviera completo.
func (b *Base) ResumenDeCategorias(ctx context.Context, claveSinReconocer string) ([]CuentaPorCategoria, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT CASE WHEN COALESCE(categoria, '') = '' THEN ? ELSE categoria END AS grupo,
		       COUNT(*),
		       COALESCE(SUM(presente), 0),
		       COALESCE(SUM(CASE WHEN origen = 'manual' THEN 1 ELSE 0 END), 0)
		  FROM equipos
		 WHERE estatus = 1
		 GROUP BY grupo
		 ORDER BY COUNT(*) DESC, grupo`, claveSinReconocer)
	if err != nil {
		return nil, fmt.Errorf("no se pudo contar de que esta hecha la red: %w", err)
	}
	defer filas.Close()

	resumen := []CuentaPorCategoria{}
	for filas.Next() {
		var cuenta CuentaPorCategoria
		if err := filas.Scan(&cuenta.Categoria, &cuenta.Cuantos, &cuenta.Presentes,
			&cuenta.Declarados); err != nil {
			return nil, err
		}
		resumen = append(resumen, cuenta)
	}
	return resumen, filas.Err()
}
