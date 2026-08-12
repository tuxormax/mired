package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/tuxormax/mired/internal/autenticacion"
	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/sonda"
)

// esperaEscaneo es lo que el servidor le da a la sonda para terminar un barrido
// antes de darlo por perdido. Un /24 con puertos tarda minutos, no segundos.
const esperaEscaneo = 20 * time.Minute

// enCurso lleva la cuenta de que redes se estan escaneando ahora mismo.
//
// Dos barridos simultaneos sobre la misma red no solo desperdician trabajo: se
// pisan al marcar los equipos ausentes, y el resultado seria una red donde los
// equipos parpadean entre presente y ausente sin razon.
var enCurso struct {
	sync.Mutex
	redes map[string]bool
}

func marcarEnCurso(clave string) bool {
	enCurso.Lock()
	defer enCurso.Unlock()
	if enCurso.redes == nil {
		enCurso.redes = map[string]bool{}
	}
	if enCurso.redes[clave] {
		return false
	}
	enCurso.redes[clave] = true
	return true
}

func liberarEnCurso(clave string) {
	enCurso.Lock()
	defer enCurso.Unlock()
	delete(enCurso.redes, clave)
}

type peticionEscaneo struct {
	// SoloPresencia hace el barrido rapido: quien esta, sin tocar puertos.
	SoloPresencia bool `json:"soloPresencia"`
}

// lanzarEscaneo arranca un barrido y contesta de inmediato.
//
// No se espera a que termine porque un escaneo tarda minutos: dejar la peticion
// HTTP abierta todo ese rato serviria para que cualquier corte de red la mate y
// el usuario no sepa si quedo o no.
func (a *API) lanzarEscaneo(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())
	nivel, _ := autenticacion.NivelDe(peticion.Context())
	if !autenticacion.PuedeEscribir(nivel) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Escaneo", Accion: "Lanzar", Causa: CausaPermiso,
			Codigo: http.StatusForbidden,
		}, "Necesita permiso de escritura sobre esta red.", nil)
		return
	}

	var cuerpo peticionEscaneo
	if peticion.ContentLength > 0 && !a.leerCuerpo(escritor, peticion, &cuerpo, "Escaneo", "Lanzar") {
		return
	}

	// Sin subredes no hay nada que barrer, y decirlo asi evita el escaneo vacio
	// que despues parece un fallo.
	var subredes []string
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		lista, err := base.ListarSubredes(peticion.Context())
		if err != nil {
			return err
		}
		for _, subred := range lista {
			if subred.Escanear {
				subredes = append(subredes, subred.CIDR)
			}
		}
		return nil
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Escaneo", Accion: "Lanzar", Causa: CausaBaseDatos,
			Tabla: "subredes", Codigo: http.StatusInternalServerError,
		}, "No se pudieron leer las subredes de esta red.", err)
		return
	}
	if len(subredes) == 0 {
		a.errorValidacion(escritor, peticion, "Escaneo", "Lanzar",
			"Esta red no tiene ninguna subred marcada para escanear. Agregue al menos una.")
		return
	}

	if !marcarEnCurso(clave) {
		a.errorValidacion(escritor, peticion, "Escaneo", "Lanzar",
			"Ya hay un escaneo en curso en esta red. Espere a que termine.")
		return
	}

	tipo := "profundo"
	if cuerpo.SoloPresencia {
		tipo = "presencia"
	}

	var escaneoID int64
	err = a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		escaneoID, err = base.IniciarEscaneo(peticion.Context(), tipo)
		return err
	})
	if err != nil {
		liberarEnCurso(clave)
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Escaneo", Accion: "Lanzar", Causa: CausaBaseDatos,
			Tabla: "escaneos", Codigo: http.StatusInternalServerError,
			Estado: "El escaneo NO se inicio.",
		}, "No se pudo iniciar el escaneo.", err)
		return
	}

	go a.correrEscaneo(clave, escaneoID, subredes, cuerpo.SoloPresencia)

	a.anotarActividad(peticion, "Escaneo", "Lanzar escaneo "+tipo+" en "+clave)
	responderOk(escritor, map[string]any{
		"escaneoId": escaneoID,
		"estado":    "en_curso",
		"tipo":      tipo,
		"subredes":  subredes,
	})
}

// correrEscaneo le pide el barrido a la sonda y guarda lo que traiga.
func (a *API) correrEscaneo(clave string, escaneoID int64, subredes []string, soloPresencia bool) {
	defer liberarEnCurso(clave)

	// Contexto propio: la peticion HTTP que lo lanzo ya termino hace rato.
	ctx, cancelar := context.WithTimeout(context.Background(), esperaEscaneo+time.Minute)
	defer cancelar()

	resultado, err := sonda.PedirEscaneo(a.SocketSonda, sonda.PeticionEscaneo{
		Subredes:      subredes,
		SoloPresencia: soloPresencia,
	}, esperaEscaneo)
	if err != nil {
		a.Bitacora.Error("el escaneo fallo", "red", clave, "escaneo", escaneoID, "error", err)
		a.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
			return base.FallarEscaneo(ctx, escaneoID, err.Error())
		})
		return
	}
	for _, advertencia := range resultado.Advertencias {
		a.Bitacora.Warn("aviso del escaneo", "red", clave, "aviso", advertencia)
	}

	equipos := make([]basedatos.EquipoDescubierto, 0, len(resultado.Equipos))
	for _, visto := range resultado.Equipos {
		puertos := make([]basedatos.PuertoDescubierto, 0, len(visto.Puertos))
		for _, puerto := range visto.Puertos {
			puertos = append(puertos, basedatos.PuertoDescubierto{
				Numero:    puerto.Numero,
				Protocolo: puerto.Protocolo,
				Servicio:  puerto.Servicio,
				Banner:    puerto.Banner,
			})
		}
		equipos = append(equipos, basedatos.EquipoDescubierto{
			IP:         visto.IP,
			MAC:        visto.MAC,
			Nombre:     visto.Nombre,
			Fabricante: visto.Fabricante,
			Metodo:     visto.Metodo,
			Subred:     visto.Subred,
			Puertos:    puertos,
		})
	}

	err = a.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
		resumen, err := base.GuardarDescubrimiento(ctx, escaneoID, !soloPresencia, equipos)
		if err != nil {
			return err
		}
		a.Bitacora.Info("escaneo terminado", "red", clave, "vistos", resumen.Vistos,
			"nuevos", resumen.Nuevos, "ausentes", resumen.Ausentes, "ms", resultado.DuracionMs)

		// El resumen del catalogo se actualiza aqui, al terminar cada escaneo:
		// es lo que permite que el panel de inicio no abra el archivo de cada red.
		total, presentes, ultimo, err := base.ResumenDeRed(ctx)
		if err != nil {
			return err
		}
		return a.Datos.ActualizarResumen(ctx, clave, total, presentes, ultimo)
	})
	if err != nil {
		a.Bitacora.Error("no se pudo guardar el escaneo", "red", clave, "error", err)
		a.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
			return base.FallarEscaneo(ctx, escaneoID, err.Error())
		})
	}
}

// listarEscaneos devuelve las ultimas corridas, para que la interfaz sepa si hay
// una en curso y como termino la anterior.
func (a *API) listarEscaneos(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var corridas []map[string]any
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		filas, err := base.QueryContext(peticion.Context(), `
			SELECT id, tipo, iniciado, terminado, estado, equipos_vistos, equipos_nuevos, detalle
			  FROM escaneos
			 ORDER BY id DESC
			 LIMIT 20`)
		if err != nil {
			return err
		}
		defer filas.Close()

		for filas.Next() {
			var id, vistos, nuevos int64
			var tipo, iniciado, estado string
			var terminado, detalle *string
			if err := filas.Scan(&id, &tipo, &iniciado, &terminado, &estado, &vistos, &nuevos, &detalle); err != nil {
				return err
			}
			corridas = append(corridas, map[string]any{
				"id": id, "tipo": tipo, "iniciado": iniciado, "terminado": terminado,
				"estado": estado, "vistos": vistos, "nuevos": nuevos, "detalle": detalle,
			})
		}
		return filas.Err()
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Escaneo", Accion: "Listar", Causa: CausaBaseDatos,
			Tabla: "escaneos", Codigo: http.StatusInternalServerError,
		}, "No se pudieron leer los escaneos.", err)
		return
	}
	if corridas == nil {
		corridas = []map[string]any{}
	}
	responderOk(escritor, corridas)
}

// ------------------------------------------------------------- equipos ----

func (a *API) listarEquipos(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())
	soloPresentes := peticion.URL.Query().Get("presentes") == "1"

	var equipos []basedatos.Equipo
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		equipos, err = base.ListarEquipos(peticion.Context(), soloPresentes)
		return err
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Equipos", Accion: "Listar", Causa: CausaBaseDatos,
			Tabla: "equipos", Codigo: http.StatusInternalServerError,
		}, "No se pudieron listar los equipos.", err)
		return
	}
	responderOk(escritor, equipos)
}

type peticionAlias struct {
	Alias string `json:"alias"`
}

// ponerAlias le pone al equipo el nombre con el que lo conoce la gente. Es lo
// que convierte "192.168.1.47" en "Impresora de contabilidad".
func (a *API) ponerAlias(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())
	nivel, _ := autenticacion.NivelDe(peticion.Context())
	if !autenticacion.PuedeEscribir(nivel) {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Equipos", Accion: "Renombrar", Causa: CausaPermiso,
			Codigo: http.StatusForbidden,
		}, "Necesita permiso de escritura sobre esta red.", nil)
		return
	}

	id, err := strconv.ParseInt(peticion.PathValue("equipo"), 10, 64)
	if err != nil || id <= 0 {
		a.errorValidacion(escritor, peticion, "Equipos", "Renombrar", "El equipo no es valido.")
		return
	}

	var cuerpo peticionAlias
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Equipos", "Renombrar") {
		return
	}
	if len(cuerpo.Alias) > 120 {
		a.errorValidacion(escritor, peticion, "Equipos", "Renombrar",
			"El nombre no puede pasar de 120 caracteres.")
		return
	}

	err = a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		return base.PonerAlias(peticion.Context(), id, cuerpo.Alias)
	})
	if err != nil {
		if errors.Is(err, basedatos.ErrEquipoNoExiste) {
			a.errorValidacion(escritor, peticion, "Equipos", "Renombrar", "Ese equipo no existe.")
			return
		}
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Equipos", Accion: "Renombrar", Causa: CausaBaseDatos,
			Tabla: "equipos", Codigo: http.StatusInternalServerError,
		}, "No se pudo guardar el nombre.", err)
		return
	}

	a.anotarActividad(peticion, "Equipos", fmt.Sprintf("Renombrar equipo %d de %s", id, clave))
	responderOk(escritor, map[string]any{"guardado": true})
}
