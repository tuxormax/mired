package basedatos

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/tuxormax/mired/internal/catalogo"
)

// Aqui vive la tercera fuente del mapa: lo que una persona DECLARA.
//
// MiRed ya sabia distinguir el enlace confirmado (se lo dijo el switch) del
// inferido (varias MAC en un puerto). Ninguno de los dos sirve en la red mas
// comun que existe: modem del ISP, switch tonto, PC, antena y grabador de
// camaras. Ahi no hay a quien preguntarle nada, pero el dueno tiene el cable
// delante y lo sabe. Esto es para que pueda decirlo, **sin que despues se
// confunda con lo que se midio**: el origen del dato viaja con el dato.

var (
	// ErrPuertoRepetido: ese puerto ya estaba declarada en ese equipo.
	ErrPuertoRepetido = errors.New("ese puerto ya esta declarado en el equipo")
	// ErrPuertoNoExiste lo devuelven las operaciones sobre un puerto que ya no esta.
	ErrPuertoNoExiste = errors.New("el puerto no existe")
	// ErrEnlaceNoExiste lo devuelven las operaciones sobre un cable que ya no esta.
	ErrEnlaceNoExiste = errors.New("el enlace no existe")
	// ErrEnlaceAlReves: nadie se conecta consigo mismo.
	ErrEnlaceAlReves = errors.New("un equipo no puede conectarse a si mismo")
	// ErrEquipoRepetido: ya hay un equipo con ese nombre en la red.
	ErrEquipoRepetido = errors.New("ya existe un equipo con ese nombre")
)

// Origenes posibles de un cable del mapa. El orden de esta lista NO es
// decorativo: es la prioridad al reconciliar, de mayor a menor.
const (
	OrigenSNMP     = "snmp"
	OrigenLLDP     = "lldp"
	OrigenCDP      = "cdp"
	OrigenManual   = "manual"
	OrigenInferido = "inferido"
)

// PuertoFisico es un puerto de un equipo, contada mirando el aparato.
//
// No es lo mismo que `interfaces`: aquellas son las que el equipo ANUNCIA por
// SNMP con su indice interno, y solo existen si el equipo habla. Un switch de
// ocho puertos de cien pesos no habla, no tiene direccion, y aun asi tiene ocho
// puertos.
type PuertoFisico struct {
	ID       int64 `json:"id"`
	EquipoID int64 `json:"equipoId"`
	Numero   int   `json:"numero"`
	// Tipo es uno de TiposDePuerto. Con el numero forma el NOMBRE del puerto
	// —"LAN 3", "WAN 1"—, que es como esta rotulado en el aparato y como hay que
	// nombrarlo en el mapa: "puerto 3" a secas no dice por donde sale el cable en
	// un modem que tiene LAN, WAN y DMZ.
	Tipo string `json:"tipo"`
	// VelocidadMbps va en NULL cuando no se sabe: mejor vacio que un 100
	// inventado que despues alguien lea como medido.
	VelocidadMbps *int   `json:"velocidadMbps"`
	Notas         string `json:"notas"`
	CreadoEn      string `json:"creadoEn"`
}

// EnlaceFisico es un cable ya resuelto: las dos puntas apuntan a filas de la
// base, no a un nombre suelto que anuncio un vecino.
type EnlaceFisico struct {
	ID             int64 `json:"id"`
	PuertoOrigenID int64 `json:"puertoOrigenId"`
	// EquipoOrigenID y NumeroOrigen se resuelven al leer, para que la interfaz no
	// tenga que cruzar tablas para pintar una linea.
	EquipoOrigenID  int64  `json:"equipoOrigenId"`
	NumeroOrigen    int    `json:"numeroOrigen"`
	OrigenNombre    string `json:"origenNombre"`
	PuertoDestinoID *int64 `json:"puertoDestinoId"`
	EquipoDestinoID *int64 `json:"equipoDestinoId"`
	NumeroDestino   int    `json:"numeroDestino"`
	DestinoNombre   string `json:"destinoNombre"`
	// OrigenDato dice de donde salio: manual, snmp, lldp, cdp o inferido. Es lo
	// que permite dibujar distinto lo tecleado de lo medido en vez de mezclarlo.
	OrigenDato string `json:"origenDato"`
	Notas      string `json:"notas"`
	CreadoEn   string `json:"creadoEn"`
}

// Contradiccion es un tramo donde lo declarado a mano y lo que reporta el equipo
// no coinciden.
//
// **No se pisa ninguno de los dos.** Se muestran los dos y se pregunta: pisar en
// silencio el dato tecleado borraria trabajo de una persona, y pisar el medido
// dejaria el mapa mintiendo sobre lo que el switch acaba de decir.
type Contradiccion struct {
	EnlaceID     int64  `json:"enlaceId"`
	EquipoID     int64  `json:"equipoId"`
	EquipoNombre string `json:"equipoNombre"`
	Numero       int    `json:"numero"`
	// Declarado es lo que puso la persona; Medido, lo que contesto el equipo.
	Declarado string `json:"declarado"`
	Medido    string `json:"medido"`
	Fuente    string `json:"fuente"`
	Momento   string `json:"momento"`
}

// EquipoManual es lo que hace falta para dar de alta un aparato que ningun
// barrido va a encontrar nunca.
type EquipoManual struct {
	Nombre string `json:"nombre"`
	// Categoria es una clave de la lista unica de MiRed, elegida de un
	// desplegable. **Nunca texto libre**: si el que se declara a mano y el que se
	// descubre no hablan el mismo idioma, el contador saca dos cubos para la
	// misma cosa.
	Categoria string `json:"categoria"`
	// Tipo es como se lee. Lo manda la interfaz junto con la categoria, sacado de
	// la misma lista, para que la ficha no muestre la clave cruda.
	Tipo     string `json:"tipo"`
	Modelo   string `json:"modelo"`
	Notas    string `json:"notas"`
	IP       string `json:"ip"`
	MAC      string `json:"mac"`
	Conexion string `json:"conexion"`
	// Puertos permite crear el switch con sus puertos de una vez, que es como lo
	// piensa quien lo esta capturando: "es un switch de ocho".
	Puertos int `json:"puertos"`
}

// FichaEquipo son los campos que una persona puede corregir de cualquier equipo,
// venga de un escaneo o no.
type FichaEquipo struct {
	Modelo   string `json:"modelo"`
	Notas    string `json:"notas"`
	Conexion string `json:"conexion"`
}

// ------------------------------------------------------- equipos a mano --

// CrearEquipoManual da de alta un aparato que nadie va a descubrir.
//
// Un switch no administrable no tiene direccion: no contesta ping, no sale en la
// tabla ARP y no existe para ningun escaneo. Sin esto, el cable que pasa por el
// no se puede dibujar, y el mapa muestra ocho equipos flotando donde en realidad
// hay un arbol.
func (b *Base) CrearEquipoManual(ctx context.Context, datos EquipoManual) (Equipo, error) {
	nombre := strings.TrimSpace(datos.Nombre)
	if nombre == "" {
		return Equipo{}, errors.New("hay que ponerle un nombre al equipo")
	}
	if len(nombre) > 120 {
		return Equipo{}, errors.New("el nombre no puede pasar de 120 caracteres")
	}
	if datos.Puertos < 0 || datos.Puertos > 512 {
		return Equipo{}, errors.New("el numero de puertos tiene que estar entre 0 y 512")
	}
	if err := validarConexion(datos.Conexion); err != nil {
		return Equipo{}, err
	}
	// El servidor comprueba la categoria aunque el formulario ya la ofrezca en un
	// desplegable: un cliente viejo, o una peticion hecha fuera del formulario,
	// se saltan la interfaz. Y una categoria inventada rompe el contador en
	// silencio, que es la peor forma de romperse.
	if !catalogo.EsCategoriaValida(datos.Categoria) {
		return Equipo{}, fmt.Errorf(
			"«%s» no es una categoria de MiRed: elija una de la lista", datos.Categoria)
	}
	if datos.Categoria == catalogo.SinReconocer {
		return Equipo{}, errors.New(
			"«sin reconocer» es donde caen los que nadie identifico, no algo que se pueda elegir")
	}

	momento := Ahora()
	var creado Equipo

	err := b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		var repetidos int
		err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM equipos
			  WHERE estatus = 1 AND LOWER(COALESCE(alias, nombre, '')) = LOWER(?)`,
			nombre).Scan(&repetidos)
		if err != nil {
			return err
		}
		if repetidos > 0 {
			return ErrEquipoRepetido
		}

		// La identidad se completa despues del INSERT porque lleva el id, y el id
		// no existe hasta que la fila existe. Se marca con prefijo por la misma
		// razon que "ip:": para que se vea de un vistazo que ese equipo no se
		// identifica por nada que se haya medido.
		resultado, err := tx.ExecContext(ctx, `
			INSERT INTO equipos (identidad, ip, mac, nombre, alias, tipo, categoria, modelo,
			                     notas, origen, conexion, metodo, presente, primera_vez,
			                     ultima_vez, estatus)
			VALUES ('', ?, ?, NULL, ?, ?, ?, ?, ?, 'manual', ?, 'manual', 1, ?, ?, 1)`,
			strings.TrimSpace(datos.IP), nuloSiVacio(normalizarMAC(datos.MAC)), nombre,
			nuloSiVacio(strings.TrimSpace(datos.Tipo)), nuloSiVacio(strings.TrimSpace(datos.Categoria)),
			nuloSiVacio(strings.TrimSpace(datos.Modelo)),
			nuloSiVacio(strings.TrimSpace(datos.Notas)), nuloSiVacio(datos.Conexion),
			momento, momento)
		if err != nil {
			return fmt.Errorf("no se pudo dar de alta el equipo: %w", err)
		}
		equipoID, _ := resultado.LastInsertId()

		if _, err := tx.ExecContext(ctx,
			`UPDATE equipos SET identidad = ? WHERE id = ?`,
			fmt.Sprintf("manual:%d", equipoID), equipoID); err != nil {
			return err
		}

		// Los puertos se numeran 1..N y son LAN. La WAN de un modem se agrega
		// aparte: casi ningun aparato tiene la WAN en medio de la fila.
		for numero := 1; numero <= datos.Puertos; numero++ {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO puertos_fisicos (equipo_id, numero, tipo, creado_en)
				VALUES (?, ?, 'lan', ?)`, equipoID, numero, momento); err != nil {
				return fmt.Errorf("no se pudo crear el puerto %d: %w", numero, err)
			}
		}

		creado, err = leerEquipo(ctx, tx, equipoID)
		return err
	})

	return creado, err
}

// ActualizarFicha guarda lo que una persona sabe del equipo y ninguna
// herramienta puede averiguar: el modelo, una nota y si esta por cable o por
// WiFi.
func (b *Base) ActualizarFicha(ctx context.Context, equipoID int64, ficha FichaEquipo) error {
	if len(ficha.Modelo) > 120 {
		return errors.New("el modelo no puede pasar de 120 caracteres")
	}
	if len(ficha.Notas) > 2000 {
		return errors.New("las notas no pueden pasar de 2000 caracteres")
	}
	if err := validarConexion(ficha.Conexion); err != nil {
		return err
	}

	resultado, err := b.ExecContext(ctx, `
		UPDATE equipos
		   SET modelo = ?, notas = ?, conexion = ?, modificado = ?
		 WHERE id = ? AND estatus = 1`,
		nuloSiVacio(strings.TrimSpace(ficha.Modelo)),
		nuloSiVacio(strings.TrimSpace(ficha.Notas)),
		nuloSiVacio(ficha.Conexion), Ahora(), equipoID)
	if err != nil {
		return fmt.Errorf("no se pudo guardar la ficha del equipo: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return ErrEquipoNoExiste
	}
	return nil
}

func validarConexion(conexion string) error {
	switch conexion {
	case "", "cable", "wifi":
		return nil
	default:
		return errors.New("la conexion solo puede ser por cable o por WiFi")
	}
}

func normalizarMAC(mac string) string {
	return strings.ToLower(strings.TrimSpace(mac))
}

func leerEquipo(ctx context.Context, tx *sql.Tx, equipoID int64) (Equipo, error) {
	var e Equipo
	var presente int
	err := tx.QueryRowContext(ctx, `
		SELECT id, identidad, ip, COALESCE(mac, ''), COALESCE(fabricante, ''),
		       COALESCE(nombre, ''), COALESCE(alias, ''), COALESCE(tipo, ''),
		       COALESCE(subred, ''), COALESCE(metodo, ''), presente,
		       primera_vez, ultima_vez, COALESCE(modelo, ''), COALESCE(notas, ''),
		       origen, COALESCE(conexion, ''), COALESCE(categoria, '')
		  FROM equipos WHERE id = ?`, equipoID).
		Scan(&e.ID, &e.Identidad, &e.IP, &e.MAC, &e.Fabricante, &e.Nombre, &e.Alias,
			&e.Tipo, &e.Subred, &e.Metodo, &presente, &e.PrimeraVez, &e.UltimaVez,
			&e.Modelo, &e.Notas, &e.Origen, &e.Conexion, &e.Categoria)
	if err != nil {
		return Equipo{}, fmt.Errorf("no se pudo releer el equipo: %w", err)
	}
	e.Presente = presente == 1
	e.Puertos = []Puerto{}
	return e, nil
}

// BorrarEquipoManual quita un equipo declarado a mano.
//
// Solo los declarados: un equipo descubierto se marca ausente cuando desaparece,
// pero no se borra, porque su historia es lo que despues permite avisar de que
// lleva tres dias sin aparecer.
func (b *Base) BorrarEquipoManual(ctx context.Context, equipoID int64) error {
	resultado, err := b.ExecContext(ctx,
		`UPDATE equipos SET estatus = -1, modificado = ?
		  WHERE id = ? AND estatus = 1 AND origen = 'manual'`,
		Ahora(), equipoID)
	if err != nil {
		return fmt.Errorf("no se pudo borrar el equipo: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return ErrEquipoNoExiste
	}
	// Los puertos y sus cables se van con el: dejarlos colgando de un equipo
	// borrado pondria lineas hacia la nada en el mapa.
	_, err = b.ExecContext(ctx, `DELETE FROM puertos_fisicos WHERE equipo_id = ?`, equipoID)
	return err
}

// --------------------------------------------------------------- puertos --

// AgregarPuertoFisico declara un puerto de un equipo.
func (b *Base) AgregarPuertoFisico(ctx context.Context, puerto PuertoFisico) (PuertoFisico, error) {
	if err := validarPuerto(puerto); err != nil {
		return PuertoFisico{}, err
	}

	resultado, err := b.ExecContext(ctx, `
		INSERT INTO puertos_fisicos (equipo_id, numero, tipo, velocidad_mbps, notas, creado_en)
		VALUES (?, ?, ?, ?, ?, ?)`,
		puerto.EquipoID, puerto.Numero, puerto.Tipo, puerto.VelocidadMbps,
		nuloSiVacio(strings.TrimSpace(puerto.Notas)), Ahora())
	if err != nil {
		if esRepetido(err) {
			return PuertoFisico{}, ErrPuertoRepetido
		}
		return PuertoFisico{}, fmt.Errorf("no se pudo declarar el puerto: %w", err)
	}

	puerto.ID, _ = resultado.LastInsertId()
	return puerto, nil
}

// EditarPuertoFisico corrige un puerto ya declarada.
func (b *Base) EditarPuertoFisico(ctx context.Context, puerto PuertoFisico) error {
	if err := validarPuerto(puerto); err != nil {
		return err
	}

	resultado, err := b.ExecContext(ctx, `
		UPDATE puertos_fisicos
		   SET numero = ?, tipo = ?, velocidad_mbps = ?, notas = ?
		 WHERE id = ?`,
		puerto.Numero, puerto.Tipo, puerto.VelocidadMbps,
		nuloSiVacio(strings.TrimSpace(puerto.Notas)), puerto.ID)
	if err != nil {
		if esRepetido(err) {
			return ErrPuertoRepetido
		}
		return fmt.Errorf("no se pudo guardar el puerto: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return ErrPuertoNoExiste
	}
	return nil
}

// BorrarPuertoFisico quita un puerto. El cable que colgaba de ella se va con
// ella: un cable sin punta no es medio dato, es un dibujo falso.
func (b *Base) BorrarPuertoFisico(ctx context.Context, id int64) error {
	resultado, err := b.ExecContext(ctx, `DELETE FROM puertos_fisicos WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("no se pudo borrar el puerto: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return ErrPuertoNoExiste
	}
	return nil
}

// TiposDePuerto es la lista cerrada de lo que puede ser un puerto fisico.
//
// Es la MISMA lista que el CHECK de la tabla (red 0018) y que el desplegable de
// la interfaz. Cerrada a proposito: con texto libre acabarian conviviendo "Lan",
// "LAN1" y "lan ", y el mapa nombraria el mismo puerto de tres maneras.
var TiposDePuerto = []string{"lan", "wan", "dmz", "sfp", "consola"}

// TipoDePuertoValido dice si ese tipo esta en la lista.
func TipoDePuertoValido(tipo string) bool {
	for _, valido := range TiposDePuerto {
		if tipo == valido {
			return true
		}
	}
	return false
}

func validarPuerto(puerto PuertoFisico) error {
	if puerto.Numero < 1 || puerto.Numero > 512 {
		return errors.New("el numero de puerto tiene que estar entre 1 y 512")
	}
	if !TipoDePuertoValido(puerto.Tipo) {
		return fmt.Errorf("un puerto solo puede ser %s", strings.Join(TiposDePuerto, ", "))
	}
	if puerto.VelocidadMbps != nil && (*puerto.VelocidadMbps < 1 || *puerto.VelocidadMbps > 400000) {
		return errors.New("la velocidad no es de un puerto de red")
	}
	if len(puerto.Notas) > 500 {
		return errors.New("la nota del puerto no puede pasar de 500 caracteres")
	}
	return nil
}

func esRepetido(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique")
}

// -------------------------------------------------------------- cables --

// CrearEnlaceManual declara que de este puerto sale un cable hacia alla.
//
// Reemplaza el cable anterior de ese puerto si lo habia: un puerto lleva UN cable, y
// guardar dos dejaria dos verdades incompatibles colgando del mismo sitio.
func (b *Base) CrearEnlaceManual(ctx context.Context, enlace EnlaceFisico) (EnlaceFisico, error) {
	if enlace.PuertoOrigenID <= 0 {
		return EnlaceFisico{}, errors.New("falta decir de que puerto sale el cable")
	}
	tienePuerto := enlace.PuertoDestinoID != nil && *enlace.PuertoDestinoID > 0
	tieneEquipo := enlace.EquipoDestinoID != nil && *enlace.EquipoDestinoID > 0
	if tienePuerto == tieneEquipo {
		return EnlaceFisico{}, errors.New(
			"el cable va a un puerto o a un equipo, no a los dos ni a ninguno")
	}

	momento := Ahora()
	var creado EnlaceFisico

	err := b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		var equipoOrigen int64
		if err := tx.QueryRowContext(ctx,
			`SELECT equipo_id FROM puertos_fisicos WHERE id = ?`,
			enlace.PuertoOrigenID).Scan(&equipoOrigen); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrPuertoNoExiste
			}
			return err
		}

		var equipoDestino int64
		if tienePuerto {
			if *enlace.PuertoDestinoID == enlace.PuertoOrigenID {
				return ErrEnlaceAlReves
			}
			if err := tx.QueryRowContext(ctx,
				`SELECT equipo_id FROM puertos_fisicos WHERE id = ?`,
				*enlace.PuertoDestinoID).Scan(&equipoDestino); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return ErrPuertoNoExiste
				}
				return err
			}
		} else {
			equipoDestino = *enlace.EquipoDestinoID
			var existe int
			if err := tx.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM equipos WHERE id = ? AND estatus = 1`,
				equipoDestino).Scan(&existe); err != nil {
				return err
			}
			if existe == 0 {
				return ErrEquipoNoExiste
			}
		}
		if equipoDestino == equipoOrigen {
			return ErrEnlaceAlReves
		}

		// El puerto de destino tampoco puede llevar dos cables.
		if tienePuerto {
			if _, err := tx.ExecContext(ctx,
				`DELETE FROM enlaces_fisicos WHERE puerto_origen_id = ? OR puerto_destino_id = ?`,
				*enlace.PuertoDestinoID, *enlace.PuertoDestinoID); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM enlaces_fisicos WHERE puerto_origen_id = ? OR puerto_destino_id = ?`,
			enlace.PuertoOrigenID, enlace.PuertoOrigenID); err != nil {
			return err
		}

		resultado, err := tx.ExecContext(ctx, `
			INSERT INTO enlaces_fisicos (puerto_origen_id, puerto_destino_id, equipo_destino_id,
			                             origen_dato, notas, creado_en)
			VALUES (?, ?, ?, 'manual', ?, ?)`,
			enlace.PuertoOrigenID, enlace.PuertoDestinoID, enlace.EquipoDestinoID,
			nuloSiVacio(strings.TrimSpace(enlace.Notas)), momento)
		if err != nil {
			return fmt.Errorf("no se pudo guardar el cable: %w", err)
		}

		id, _ := resultado.LastInsertId()
		creado = enlace
		creado.ID = id
		creado.EquipoOrigenID = equipoOrigen
		creado.OrigenDato = OrigenManual
		creado.CreadoEn = momento
		return nil
	})

	return creado, err
}

// BorrarEnlaceFisico quita un cable declarado.
func (b *Base) BorrarEnlaceFisico(ctx context.Context, id int64) error {
	resultado, err := b.ExecContext(ctx, `DELETE FROM enlaces_fisicos WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("no se pudo borrar el cable: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return ErrEnlaceNoExiste
	}
	return nil
}

// -------------------------------------------------------------- lectura --

// TopologiaManual es todo lo declarado a mano en una red, mas los tramos donde
// eso no cuadra con lo que reportan los equipos.
type TopologiaManual struct {
	Puertos         []PuertoFisico  `json:"puertos"`
	Enlaces         []EnlaceFisico  `json:"enlaces"`
	Contradicciones []Contradiccion `json:"contradicciones"`
	// Inalambricos es lo que cuelga de una antena por el aire. Va aqui y no en
	// otra llamada porque el mapa necesita las dos cosas a la vez: sin esto,
	// dibujaria los cables y dejaria los telefonos flotando.
	Inalambricos []EnlaceInalambrico `json:"inalambricos"`
	// Momento es de cuando es lo mas reciente que se declaro. Va con los datos
	// por la misma regla que el resto de MiRed: todo reporte dice de cuando es.
	Momento string `json:"momento"`
}

// LeerTopologiaManual devuelve los puertos declarados, sus cables y las
// contradicciones contra lo medido.
func (b *Base) LeerTopologiaManual(ctx context.Context) (TopologiaManual, error) {
	topologia := TopologiaManual{
		Puertos:         []PuertoFisico{},
		Enlaces:         []EnlaceFisico{},
		Contradicciones: []Contradiccion{},
		Inalambricos:    []EnlaceInalambrico{},
	}

	puertos, err := b.ListarPuertosFisicos(ctx)
	if err != nil {
		return topologia, err
	}
	topologia.Puertos = puertos

	enlaces, err := b.ListarEnlacesFisicos(ctx)
	if err != nil {
		return topologia, err
	}
	topologia.Enlaces = enlaces

	contradicciones, err := b.Contradicciones(ctx)
	if err != nil {
		return topologia, err
	}
	topologia.Contradicciones = contradicciones

	// Lo que cuelga por el aire va en el mismo viaje: el mapa necesita los
	// cables y las antenas a la vez, y en dos llamadas se dibujaria a medias.
	inalambricos, err := b.ListarEnlacesInalambricos(ctx)
	if err != nil {
		return topologia, err
	}
	topologia.Inalambricos = inalambricos

	var momento sql.NullString
	if err := b.QueryRowContext(ctx, `
		SELECT MAX(cuando) FROM (
			SELECT MAX(creado_en) AS cuando FROM puertos_fisicos
			UNION ALL
			SELECT MAX(creado_en) FROM enlaces_fisicos
			UNION ALL
			SELECT MAX(creado_en) FROM enlaces_inalambricos)`).Scan(&momento); err != nil {
		return topologia, fmt.Errorf("no se pudo leer de cuando es lo declarado: %w", err)
	}
	topologia.Momento = momento.String

	return topologia, nil
}

// ListarPuertosFisicos devuelve los puertos declarados de todos los equipos de la
// red.
func (b *Base) ListarPuertosFisicos(ctx context.Context) ([]PuertoFisico, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT p.id, p.equipo_id, p.numero, p.tipo, p.velocidad_mbps,
		       COALESCE(p.notas, ''), p.creado_en
		  FROM puertos_fisicos p
		  JOIN equipos e ON e.id = p.equipo_id AND e.estatus = 1
		 ORDER BY p.equipo_id, p.tipo DESC, p.numero`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer los puertos declarados: %w", err)
	}
	defer filas.Close()

	puertos := []PuertoFisico{}
	for filas.Next() {
		var p PuertoFisico
		var velocidad sql.NullInt64
		if err := filas.Scan(&p.ID, &p.EquipoID, &p.Numero, &p.Tipo, &velocidad,
			&p.Notas, &p.CreadoEn); err != nil {
			return nil, err
		}
		if velocidad.Valid {
			mbps := int(velocidad.Int64)
			p.VelocidadMbps = &mbps
		}
		puertos = append(puertos, p)
	}
	return puertos, filas.Err()
}

// ListarEnlacesFisicos devuelve los cables con las dos puntas ya resueltas.
func (b *Base) ListarEnlacesFisicos(ctx context.Context) ([]EnlaceFisico, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT f.id, f.puerto_origen_id, po.equipo_id, po.numero,
		       COALESCE(eo.alias, eo.nombre, eo.ip, ''),
		       f.puerto_destino_id, f.equipo_destino_id,
		       COALESCE(pd.numero, 0),
		       COALESCE(ed.alias, ed.nombre, ed.ip, ep.alias, ep.nombre, ep.ip, ''),
		       f.origen_dato, COALESCE(f.notas, ''), f.creado_en
		  FROM enlaces_fisicos f
		  JOIN puertos_fisicos po ON po.id = f.puerto_origen_id
		  JOIN equipos eo ON eo.id = po.equipo_id AND eo.estatus = 1
		  LEFT JOIN puertos_fisicos pd ON pd.id = f.puerto_destino_id
		  LEFT JOIN equipos ep ON ep.id = pd.equipo_id
		  LEFT JOIN equipos ed ON ed.id = f.equipo_destino_id
		 ORDER BY po.equipo_id, po.numero`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer los cables declarados: %w", err)
	}
	defer filas.Close()

	enlaces := []EnlaceFisico{}
	for filas.Next() {
		var e EnlaceFisico
		if err := filas.Scan(&e.ID, &e.PuertoOrigenID, &e.EquipoOrigenID, &e.NumeroOrigen,
			&e.OrigenNombre, &e.PuertoDestinoID, &e.EquipoDestinoID, &e.NumeroDestino,
			&e.DestinoNombre, &e.OrigenDato, &e.Notas, &e.CreadoEn); err != nil {
			return nil, err
		}
		enlaces = append(enlaces, e)
	}
	return enlaces, filas.Err()
}

// Contradicciones busca los tramos donde lo declarado a mano y lo que reporta el
// equipo dicen cosas distintas.
//
// El dia que un switch tonto se cambia por uno administrable, lo que se tecleo
// hace meses puede haber dejado de ser cierto. Lo peligroso no es que cambie: es
// que cambie **en silencio**. Aqui se detecta para que la interfaz pueda
// preguntar, con las dos versiones a la vista.
//
// La prioridad al reconciliar es snmp/lldp/cdp > manual > inferido, pero la
// decision NO se toma aqui: esto solo dice donde no cuadran.
func (b *Base) Contradicciones(ctx context.Context) ([]Contradiccion, error) {
	// Contra la tabla de reenvio del switch: solo puertos CONFIRMADOS. En un puerto
	// con varios equipos nunca se supo cual estaba ahi, asi que no hay nada que
	// contradecir.
	const consulta = `
		WITH manuales AS (
			SELECT f.id AS enlace_id, po.equipo_id AS equipo_id, po.numero AS numero,
			       COALESCE(pd.equipo_id, f.equipo_destino_id) AS destino_id
			  FROM enlaces_fisicos f
			  JOIN puertos_fisicos po ON po.id = f.puerto_origen_id
			  LEFT JOIN puertos_fisicos pd ON pd.id = f.puerto_destino_id
			 WHERE f.origen_dato = 'manual'
		)
		SELECT m.enlace_id, m.equipo_id, COALESCE(eq.alias, eq.nombre, eq.ip), m.numero,
		       COALESCE(declarado.alias, declarado.nombre, declarado.ip, ''),
		       COALESCE(medido.alias, medido.nombre, medido.ip, c.mac),
		       'snmp', c.ultima_vez
		  FROM manuales m
		  JOIN conexiones_puerto c
		    ON c.switch_id = m.equipo_id AND c.interfaz_indice = m.numero AND c.confirmado = 1
		  JOIN equipos eq ON eq.id = m.equipo_id
		  LEFT JOIN equipos declarado ON declarado.id = m.destino_id
		  LEFT JOIN equipos medido ON medido.id = c.equipo_id
		 WHERE c.equipo_id IS NOT m.destino_id

		UNION ALL

		SELECT m.enlace_id, m.equipo_id, COALESCE(eq.alias, eq.nombre, eq.ip), m.numero,
		       COALESCE(declarado.alias, declarado.nombre, declarado.ip, ''),
		       COALESCE(vec.alias, vec.nombre, vec.ip, l.vecino_nombre, ''),
		       l.origen, l.ultima_vez
		  FROM manuales m
		  JOIN enlaces l ON l.equipo_id = m.equipo_id
		  JOIN equipos eq ON eq.id = m.equipo_id
		  LEFT JOIN interfaces i ON i.equipo_id = m.equipo_id AND i.indice = m.numero
		  LEFT JOIN equipos declarado ON declarado.id = m.destino_id
		  LEFT JOIN equipos vec ON vec.id = l.vecino_equipo_id
		 WHERE (l.interfaz_local = CAST(m.numero AS TEXT) OR l.interfaz_local = i.nombre)
		   AND l.vecino_equipo_id IS NOT NULL
		   AND l.vecino_equipo_id IS NOT m.destino_id`

	filas, err := b.QueryContext(ctx, consulta)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron comparar los cables declarados: %w", err)
	}
	defer filas.Close()

	contradicciones := []Contradiccion{}
	for filas.Next() {
		var c Contradiccion
		if err := filas.Scan(&c.EnlaceID, &c.EquipoID, &c.EquipoNombre, &c.Numero,
			&c.Declarado, &c.Medido, &c.Fuente, &c.Momento); err != nil {
			return nil, err
		}
		contradicciones = append(contradicciones, c)
	}
	return contradicciones, filas.Err()
}
