package basedatos

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Tipos de alerta que sabe generar MiRed.
const (
	AlertaEquipoNuevo        = "equipo_nuevo"
	AlertaEquipoAusente      = "equipo_ausente"
	AlertaPuertoNuevo        = "puerto_nuevo"
	AlertaCambioIP           = "cambio_ip"
	AlertaCambioPuertoSwitch = "cambio_puerto_switch"
	AlertaRedSinReportar     = "red_sin_reportar"
)

// Alerta es un aviso generado por el motor.
type Alerta struct {
	ID       int64  `json:"id"`
	Tipo     string `json:"tipo"`
	Momento  string `json:"momento"`
	EquipoID *int64 `json:"equipoId"`
	Titulo   string `json:"titulo"`
	Detalle  string `json:"detalle"`
	Vista    bool   `json:"vista"`
	Enviada  bool   `json:"enviada"`
}

// Regla dice si se vigila algo y con que umbral.
type Regla struct {
	Tipo   string `json:"tipo"`
	Activa bool   `json:"activa"`
	Umbral int    `json:"umbral"`
}

// DestinoAlerta es a donde se avisa.
type DestinoAlerta struct {
	ID          int64  `json:"id"`
	Nombre      string `json:"nombre"`
	Tipo        string `json:"tipo"`
	Destino     string `json:"destino"`
	Extra       string `json:"extra,omitempty"`
	Activo      bool   `json:"activo"`
	UltimoEnvio string `json:"ultimoEnvio,omitempty"`
	UltimoError string `json:"ultimoError,omitempty"`
}

// Reglas devuelve la configuracion de vigilancia de esta red.
func (b *Base) Reglas(ctx context.Context) (map[string]Regla, error) {
	filas, err := b.QueryContext(ctx, `SELECT tipo, activa, umbral FROM reglas_alerta`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer las reglas de alerta: %w", err)
	}
	defer filas.Close()

	reglas := map[string]Regla{}
	for filas.Next() {
		var regla Regla
		var activa int
		if err := filas.Scan(&regla.Tipo, &activa, &regla.Umbral); err != nil {
			return nil, err
		}
		regla.Activa = activa == 1
		reglas[regla.Tipo] = regla
	}
	return reglas, filas.Err()
}

// GuardarRegla enciende o apaga una vigilancia y fija su umbral.
func (b *Base) GuardarRegla(ctx context.Context, regla Regla) error {
	resultado, err := b.ExecContext(ctx,
		`UPDATE reglas_alerta SET activa = ?, umbral = ?, modificada = ? WHERE tipo = ?`,
		boolAEntero(regla.Activa), regla.Umbral, Ahora(), regla.Tipo)
	if err != nil {
		return fmt.Errorf("no se pudo guardar la regla: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return fmt.Errorf("no existe una regla de alerta llamada %q", regla.Tipo)
	}
	return nil
}

// GenerarAlertas compara la foto nueva contra lo que ya se sabia y crea los
// avisos que correspondan.
//
// Se apoya en lo que el propio escaneo ya dejo escrito (equipos nuevos, eventos
// de presencia, puertos nuevos), no vuelve a mirar la red: el motor de alertas
// no debe poder decir algo distinto de lo que quedo guardado.
func (b *Base) GenerarAlertas(ctx context.Context, escaneoID int64) ([]Alerta, error) {
	reglas, err := b.Reglas(ctx)
	if err != nil {
		return nil, err
	}

	var nuevas []Alerta

	if regla, hay := reglas[AlertaEquipoNuevo]; hay && regla.Activa {
		avisos, err := b.alertasEquipoNuevo(ctx, escaneoID)
		if err != nil {
			return nil, err
		}
		nuevas = append(nuevas, avisos...)
	}
	if regla, hay := reglas[AlertaEquipoAusente]; hay && regla.Activa {
		avisos, err := b.alertasEquipoAusente(ctx, regla.Umbral)
		if err != nil {
			return nil, err
		}
		nuevas = append(nuevas, avisos...)
	}
	if regla, hay := reglas[AlertaPuertoNuevo]; hay && regla.Activa {
		avisos, err := b.alertasPuertoNuevo(ctx, escaneoID)
		if err != nil {
			return nil, err
		}
		nuevas = append(nuevas, avisos...)
	}
	if regla, hay := reglas[AlertaCambioIP]; hay && regla.Activa {
		avisos, err := b.alertasCambioIP(ctx, escaneoID)
		if err != nil {
			return nil, err
		}
		nuevas = append(nuevas, avisos...)
	}

	return b.guardarAlertas(ctx, nuevas)
}

// guardarAlertas inserta las que no existian ya.
//
// La huella evita el problema clasico de los sistemas de avisos: repetir la
// misma alerta en cada corrida hasta que nadie les hace caso.
func (b *Base) guardarAlertas(ctx context.Context, alertas []Alerta) ([]Alerta, error) {
	if len(alertas) == 0 {
		return nil, nil
	}

	var guardadas []Alerta
	err := b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		for _, alerta := range alertas {
			resultado, err := tx.ExecContext(ctx, `
				INSERT INTO alertas (tipo, momento, equipo_id, titulo, detalle, huella)
				VALUES (?, ?, ?, ?, ?, ?)
				ON CONFLICT (huella) DO NOTHING`,
				alerta.Tipo, alerta.Momento, alerta.EquipoID, alerta.Titulo,
				nuloSiVacio(alerta.Detalle), huellaDe(alerta))
			if err != nil {
				return fmt.Errorf("no se pudo guardar la alerta: %w", err)
			}
			if filas, _ := resultado.RowsAffected(); filas > 0 {
				alerta.ID, _ = resultado.LastInsertId()
				guardadas = append(guardadas, alerta)
			}
		}
		return nil
	})
	return guardadas, err
}

// huellaDe identifica el hecho, no el momento: "este equipo aparecio" es el
// mismo hecho aunque se detecte en tres escaneos seguidos.
func huellaDe(alerta Alerta) string {
	equipo := "0"
	if alerta.EquipoID != nil {
		equipo = strconv.FormatInt(*alerta.EquipoID, 10)
	}
	return alerta.Tipo + "|" + equipo + "|" + alerta.Detalle
}

func (b *Base) alertasEquipoNuevo(ctx context.Context, escaneoID int64) ([]Alerta, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT e.id, e.ip, COALESCE(e.mac, ''), COALESCE(e.alias, e.nombre, ''),
		       COALESCE(e.fabricante, '')
		  FROM escaneo_equipos ee
		  JOIN equipos e ON e.id = ee.equipo_id
		 WHERE ee.escaneo_id = ? AND ee.nuevo = 1`, escaneoID)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron buscar equipos nuevos: %w", err)
	}
	defer filas.Close()

	momento := Ahora()
	var alertas []Alerta
	for filas.Next() {
		var id int64
		var ip, mac, nombre, fabricante string
		if err := filas.Scan(&id, &ip, &mac, &nombre, &fabricante); err != nil {
			return nil, err
		}
		quien := nombre
		if quien == "" {
			quien = ip
		}
		alertas = append(alertas, Alerta{
			Tipo:     AlertaEquipoNuevo,
			Momento:  momento,
			EquipoID: &id,
			Titulo:   "Se conecto un equipo desconocido: " + quien,
			Detalle: strings.TrimSpace(strings.Join([]string{
				"IP " + ip,
				textoSiHay("MAC ", mac),
				textoSiHay("fabricante ", fabricante),
			}, " ")),
		})
	}
	return alertas, filas.Err()
}

func (b *Base) alertasEquipoAusente(ctx context.Context, umbralMinutos int) ([]Alerta, error) {
	if umbralMinutos <= 0 {
		umbralMinutos = 1440
	}
	limite := time.Now().Add(-time.Duration(umbralMinutos) * time.Minute).Format(time.RFC3339)

	filas, err := b.QueryContext(ctx, `
		SELECT id, ip, COALESCE(alias, nombre, ''), ultima_vez
		  FROM equipos
		 WHERE estatus = 1 AND presente = 0 AND ultima_vez < ?`, limite)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron buscar equipos ausentes: %w", err)
	}
	defer filas.Close()

	momento := Ahora()
	var alertas []Alerta
	for filas.Next() {
		var id int64
		var ip, nombre, ultimaVez string
		if err := filas.Scan(&id, &ip, &nombre, &ultimaVez); err != nil {
			return nil, err
		}
		quien := nombre
		if quien == "" {
			quien = ip
		}
		alertas = append(alertas, Alerta{
			Tipo:     AlertaEquipoAusente,
			Momento:  momento,
			EquipoID: &id,
			Titulo:   "Lleva tiempo sin aparecer: " + quien,
			// La huella incluye la fecha en que se le vio por ultima vez: si el
			// equipo vuelve y se va de nuevo, eso es un hecho distinto y debe
			// volver a avisar.
			Detalle: "Visto por ultima vez el " + ultimaVez,
		})
	}
	return alertas, filas.Err()
}

func (b *Base) alertasPuertoNuevo(ctx context.Context, escaneoID int64) ([]Alerta, error) {
	// Un puerto nuevo en un equipo NUEVO no es noticia: la noticia era el equipo.
	// Lo que preocupa es que a algo conocido le aparezca un puerto que no tenia.
	filas, err := b.QueryContext(ctx, `
		SELECT p.equipo_id, e.ip, COALESCE(e.alias, e.nombre, ''), p.numero,
		       COALESCE(p.servicio, '')
		  FROM puertos p
		  JOIN equipos e ON e.id = p.equipo_id
		  JOIN escaneo_equipos ee ON ee.equipo_id = p.equipo_id AND ee.escaneo_id = ?
		 WHERE p.abierto = 1
		   AND ee.nuevo = 0
		   AND p.escaneo_creado = ?`, escaneoID, escaneoID)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron buscar puertos nuevos: %w", err)
	}
	defer filas.Close()

	momento := Ahora()
	var alertas []Alerta
	for filas.Next() {
		var equipoID int64
		var ip, nombre, servicio string
		var numero int
		if err := filas.Scan(&equipoID, &ip, &nombre, &numero, &servicio); err != nil {
			return nil, err
		}
		quien := nombre
		if quien == "" {
			quien = ip
		}
		alertas = append(alertas, Alerta{
			Tipo:     AlertaPuertoNuevo,
			Momento:  momento,
			EquipoID: &equipoID,
			Titulo:   fmt.Sprintf("Puerto nuevo abierto en %s: %d", quien, numero),
			Detalle: strings.TrimSpace(fmt.Sprintf("%s tiene abierto el puerto %d %s",
				ip, numero, servicio)),
		})
	}
	return alertas, filas.Err()
}

// alertasCambioIP compara la IP de este escaneo contra la del anterior.
//
// Un equipo que cambia de IP suele ser normal (DHCP), pero en una red con
// direcciones fijas es justo la senal de que algo se movio o de que hay un
// conflicto.
func (b *Base) alertasCambioIP(ctx context.Context, escaneoID int64) ([]Alerta, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT actual.equipo_id, COALESCE(e.alias, e.nombre, ''), anterior.ip, actual.ip
		  FROM escaneo_equipos actual
		  JOIN equipos e ON e.id = actual.equipo_id
		  JOIN escaneo_equipos anterior
		    ON anterior.equipo_id = actual.equipo_id
		   AND anterior.escaneo_id = (
		         SELECT MAX(escaneo_id) FROM escaneo_equipos
		          WHERE equipo_id = actual.equipo_id AND escaneo_id < ?)
		 WHERE actual.escaneo_id = ? AND anterior.ip <> actual.ip`,
		escaneoID, escaneoID)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron buscar cambios de IP: %w", err)
	}
	defer filas.Close()

	momento := Ahora()
	var alertas []Alerta
	for filas.Next() {
		var equipoID int64
		var nombre, antes, ahora string
		if err := filas.Scan(&equipoID, &nombre, &antes, &ahora); err != nil {
			return nil, err
		}
		quien := nombre
		if quien == "" {
			quien = ahora
		}
		alertas = append(alertas, Alerta{
			Tipo:     AlertaCambioIP,
			Momento:  momento,
			EquipoID: &equipoID,
			Titulo:   "Cambio de direccion: " + quien,
			Detalle:  "Antes " + antes + ", ahora " + ahora,
		})
	}
	return alertas, filas.Err()
}

// AlertasDeMovimiento avisa de los equipos que se cambiaron de boca del switch.
//
// Solo se llama con movimientos entre bocas CONFIRMADAS: en una boca compartida
// nunca se supo cual equipo estaba donde, asi que decir que se movio seria
// inventar.
func (b *Base) AlertasDeMovimiento(ctx context.Context, movimientos []MovimientoDePuerto) error {
	reglas, err := b.Reglas(ctx)
	if err != nil {
		return err
	}
	if regla, hay := reglas[AlertaCambioPuertoSwitch]; !hay || !regla.Activa {
		return nil
	}

	momento := Ahora()
	alertas := make([]Alerta, 0, len(movimientos))
	for _, movimiento := range movimientos {
		id := movimiento.EquipoID
		alertas = append(alertas, Alerta{
			Tipo:     AlertaCambioPuertoSwitch,
			Momento:  momento,
			EquipoID: &id,
			Titulo:   "Se movio de lugar: " + movimiento.Nombre,
			Detalle:  "Antes en " + movimiento.Antes + ", ahora en " + movimiento.Ahora,
		})
	}

	_, err = b.guardarAlertas(ctx, alertas)
	return err
}

// AlertaSiDejoDeReportar avisa cuando la red entera lleva demasiado sin un
// escaneo terminado.
//
// Es la unica alerta que NO nace de un escaneo, por definicion: nace de que no
// hubo ninguno. La revisa el programador cada tanto.
func (b *Base) AlertaSiDejoDeReportar(ctx context.Context) ([]Alerta, error) {
	reglas, err := b.Reglas(ctx)
	if err != nil {
		return nil, err
	}
	regla, hay := reglas[AlertaRedSinReportar]
	if !hay || !regla.Activa {
		return nil, nil
	}
	umbral := regla.Umbral
	if umbral <= 0 {
		umbral = 120
	}

	var ultimo sql.NullString
	err = b.QueryRowContext(ctx,
		`SELECT MAX(terminado) FROM escaneos WHERE estado = 'terminado'`).Scan(&ultimo)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el ultimo escaneo: %w", err)
	}
	if !ultimo.Valid || ultimo.String == "" {
		// Una red que nunca se ha escaneado no "dejo de reportar": todavia no
		// empieza. Avisar aqui seria ruido el dia que se crea el sitio.
		return nil, nil
	}

	cuando, err := time.Parse(time.RFC3339, ultimo.String)
	if err != nil {
		return nil, nil
	}
	if time.Since(cuando) < time.Duration(umbral)*time.Minute {
		return nil, nil
	}

	return b.guardarAlertas(ctx, []Alerta{{
		Tipo:    AlertaRedSinReportar,
		Momento: Ahora(),
		Titulo:  "Esta red dejo de reportar",
		// La huella lleva la fecha del ultimo escaneo: si vuelve a fallar
		// despues de recuperarse, eso es un hecho nuevo y vuelve a avisar.
		Detalle: "El ultimo escaneo que termino fue el " + ultimo.String,
	}})
}

// ListarAlertas devuelve las mas recientes.
func (b *Base) ListarAlertas(ctx context.Context, soloSinVer bool, limite int) ([]Alerta, error) {
	if limite <= 0 || limite > 500 {
		limite = 200
	}

	consulta := `SELECT id, tipo, momento, equipo_id, titulo, COALESCE(detalle, ''), vista, enviada
	               FROM alertas`
	if soloSinVer {
		consulta += ` WHERE vista = 0`
	}
	consulta += ` ORDER BY id DESC LIMIT ?`

	filas, err := b.QueryContext(ctx, consulta, limite)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar las alertas: %w", err)
	}
	defer filas.Close()

	alertas := []Alerta{}
	for filas.Next() {
		var a Alerta
		var vista, enviada int
		if err := filas.Scan(&a.ID, &a.Tipo, &a.Momento, &a.EquipoID, &a.Titulo,
			&a.Detalle, &vista, &enviada); err != nil {
			return nil, err
		}
		a.Vista = vista == 1
		a.Enviada = enviada == 1
		alertas = append(alertas, a)
	}
	return alertas, filas.Err()
}

// MarcarAlertasVistas da por atendidas las alertas indicadas, o todas.
func (b *Base) MarcarAlertasVistas(ctx context.Context, ids []int64) (int, error) {
	if len(ids) == 0 {
		resultado, err := b.ExecContext(ctx, `UPDATE alertas SET vista = 1 WHERE vista = 0`)
		if err != nil {
			return 0, fmt.Errorf("no se pudieron marcar las alertas: %w", err)
		}
		filas, _ := resultado.RowsAffected()
		return int(filas), nil
	}

	marcadas := 0
	err := b.EnTransaccion(ctx, func(tx *sql.Tx) error {
		for _, id := range ids {
			resultado, err := tx.ExecContext(ctx,
				`UPDATE alertas SET vista = 1 WHERE id = ? AND vista = 0`, id)
			if err != nil {
				return err
			}
			if filas, _ := resultado.RowsAffected(); filas > 0 {
				marcadas++
			}
		}
		return nil
	})
	return marcadas, err
}

// AlertasSinEnviar devuelve lo que falta avisar hacia afuera.
func (b *Base) AlertasSinEnviar(ctx context.Context) ([]Alerta, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT id, tipo, momento, equipo_id, titulo, COALESCE(detalle, '')
		  FROM alertas WHERE enviada = 0 ORDER BY id LIMIT 50`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer las alertas pendientes: %w", err)
	}
	defer filas.Close()

	alertas := []Alerta{}
	for filas.Next() {
		var a Alerta
		if err := filas.Scan(&a.ID, &a.Tipo, &a.Momento, &a.EquipoID, &a.Titulo, &a.Detalle); err != nil {
			return nil, err
		}
		alertas = append(alertas, a)
	}
	return alertas, filas.Err()
}

// MarcarEnviada anota que una alerta ya salio hacia afuera.
func (b *Base) MarcarEnviada(ctx context.Context, id int64) error {
	_, err := b.ExecContext(ctx, `UPDATE alertas SET enviada = 1 WHERE id = ?`, id)
	return err
}

// ContarAlertasAbiertas cuenta las que nadie ha visto, para el panel de inicio.
func (b *Base) ContarAlertasAbiertas(ctx context.Context) (int, error) {
	var cuantas int
	err := b.QueryRowContext(ctx, `SELECT COUNT(*) FROM alertas WHERE vista = 0`).Scan(&cuantas)
	if err != nil {
		return 0, fmt.Errorf("no se pudieron contar las alertas: %w", err)
	}
	return cuantas, nil
}

// ActualizarAlertasEnCatalogo lleva la cuenta al catalogo, para que el panel de
// inicio no tenga que abrir el archivo de cada red.
func (e *Enrutador) ActualizarAlertasEnCatalogo(ctx context.Context, clave string, abiertas int) error {
	_, err := e.Catalogo.ExecContext(ctx,
		`UPDATE redes SET alertas_abiertas = ? WHERE clave = ?`, abiertas, clave)
	return err
}

// ListarDestinos devuelve a donde avisa esta red.
func (b *Base) ListarDestinos(ctx context.Context) ([]DestinoAlerta, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT id, nombre, tipo, destino, COALESCE(extra, ''), activo,
		       COALESCE(ultimo_envio, ''), COALESCE(ultimo_error, '')
		  FROM destinos_alerta
		 WHERE estatus = 1
		 ORDER BY nombre`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar los destinos: %w", err)
	}
	defer filas.Close()

	destinos := []DestinoAlerta{}
	for filas.Next() {
		var d DestinoAlerta
		var activo int
		if err := filas.Scan(&d.ID, &d.Nombre, &d.Tipo, &d.Destino, &d.Extra,
			&activo, &d.UltimoEnvio, &d.UltimoError); err != nil {
			return nil, err
		}
		d.Activo = activo == 1
		destinos = append(destinos, d)
	}
	return destinos, filas.Err()
}

// CrearDestino da de alta a donde avisar.
func (b *Base) CrearDestino(ctx context.Context, destino DestinoAlerta) (DestinoAlerta, error) {
	destino.Nombre = strings.TrimSpace(destino.Nombre)
	destino.Destino = strings.TrimSpace(destino.Destino)
	if destino.Nombre == "" || destino.Destino == "" {
		return DestinoAlerta{}, fmt.Errorf("el destino necesita nombre y direccion")
	}
	switch destino.Tipo {
	case "webhook", "ntfy", "telegram", "correo":
	default:
		return DestinoAlerta{}, fmt.Errorf("tipo de destino invalido: %s", destino.Tipo)
	}

	resultado, err := b.ExecContext(ctx, `
		INSERT INTO destinos_alerta (nombre, tipo, destino, extra, activo, estatus, creado)
		VALUES (?, ?, ?, ?, 1, 1, ?)
		ON CONFLICT (nombre) WHERE estatus >= 0 DO UPDATE SET
			tipo = excluded.tipo, destino = excluded.destino,
			extra = excluded.extra, activo = 1, estatus = 1`,
		destino.Nombre, destino.Tipo, destino.Destino, nuloSiVacio(destino.Extra), Ahora())
	if err != nil {
		return DestinoAlerta{}, fmt.Errorf("no se pudo guardar el destino: %w", err)
	}
	destino.ID, _ = resultado.LastInsertId()
	destino.Activo = true
	return destino, nil
}

// BorrarDestino deja de avisar ahi.
func (b *Base) BorrarDestino(ctx context.Context, id int64) error {
	resultado, err := b.ExecContext(ctx,
		`UPDATE destinos_alerta SET estatus = -1 WHERE id = ? AND estatus >= 0`, id)
	if err != nil {
		return fmt.Errorf("no se pudo borrar el destino: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return fmt.Errorf("ese destino no existe")
	}
	return nil
}

// AnotarEnvio deja constancia de como le fue al ultimo aviso.
func (b *Base) AnotarEnvio(ctx context.Context, id int64, problema string) error {
	_, err := b.ExecContext(ctx,
		`UPDATE destinos_alerta SET ultimo_envio = ?, ultimo_error = ? WHERE id = ?`,
		Ahora(), nuloSiVacio(problema), id)
	return err
}

func textoSiHay(prefijo, valor string) string {
	if valor == "" {
		return ""
	}
	return prefijo + valor
}
