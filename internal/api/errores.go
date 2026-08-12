package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/tuxormax/mired/internal/autenticacion"
	"github.com/tuxormax/mired/internal/version"
)

// Respuesta es el sobre de TODA respuesta de la API. La interfaz nunca tiene que
// adivinar si algo salio bien: lo dice el campo ok.
type Respuesta struct {
	Ok    bool          `json:"ok"`
	Datos any           `json:"datos,omitempty"`
	Error *ErrorDetalle `json:"error,omitempty"`
}

// ErrorDetalle es el bloque de detalles del estandar de errores de la casa: todo
// lo que hace falta para diagnosticar sin volver a preguntarle al usuario.
//
// El mismo bloque se guarda en log_errores, asi que soporte ve el error aunque
// nadie lo reporte, y el ID permite cruzar lo que copio el usuario con la fila
// de la bitacora.
type ErrorDetalle struct {
	ID       string `json:"id"`
	Momento  string `json:"momento"`
	Modulo   string `json:"modulo"`
	Accion   string `json:"accion"`
	Causa    string `json:"causa"`
	Ruta     string `json:"ruta,omitempty"`
	Tabla    string `json:"tabla,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Red      string `json:"red,omitempty"`
	Usuario  string `json:"usuario,omitempty"`
	Version  string `json:"version"`
	Build    string `json:"build"`
	SO       string `json:"so"`
	Estado   string `json:"estado,omitempty"`
	Mensaje  string `json:"mensaje"`
	Error    string `json:"error,omitempty"`
}

// Causas normalizadas, para poder agrupar los errores en la bitacora.
const (
	CausaValidacion = "Validacion"
	CausaBaseDatos  = "Base de datos"
	CausaPermiso    = "Permiso"
	CausaSesion     = "Sesion"
	CausaArchivo    = "Archivo"
	CausaRed        = "Red"
	CausaInterno    = "Interno"
)

// contextoError describe donde ocurrio algo, para armar el detalle.
type contextoError struct {
	Modulo string
	Accion string
	Causa  string
	Tabla  string
	Estado string
	// Codigo es el HTTP que se responde.
	Codigo int
}

// responderOk envia datos con el sobre estandar.
func responderOk(escritor http.ResponseWriter, datos any) {
	escribirJSON(escritor, http.StatusOK, Respuesta{Ok: true, Datos: datos})
}

// responderError arma el detalle completo, lo guarda en la bitacora y lo envia.
//
// mensaje es lo que se le dice al usuario en lenguaje llano; err es el error
// crudo del sistema, que viaja aparte para que soporte lo vea sin que la
// pantalla muestre jerga.
func (a *API) responderError(escritor http.ResponseWriter, peticion *http.Request, ctx contextoError, mensaje string, err error) {
	detalle := ErrorDetalle{
		ID:       generarIDError(),
		Momento:  time.Now().Format(time.RFC3339),
		Modulo:   ctx.Modulo,
		Accion:   ctx.Accion,
		Causa:    ctx.Causa,
		Ruta:     origenLlamada(),
		Tabla:    ctx.Tabla,
		Endpoint: peticion.Method + " " + peticion.URL.Path,
		Version:  version.Numero + " Rev " + version.Revision,
		Build:    version.Build,
		SO:       runtime.GOOS + "/" + runtime.GOARCH,
		Estado:   ctx.Estado,
		Mensaje:  mensaje,
	}
	if err != nil {
		detalle.Error = err.Error()
	}
	if usuario, hay := autenticacion.UsuarioDe(peticion.Context()); hay {
		detalle.Usuario = usuario.Usuario
	}
	if red, hay := autenticacion.RedActivaDe(peticion.Context()); hay {
		detalle.Red = red
	}

	// Los errores de validacion y de permiso son parte del uso normal: se
	// responden, pero no ensucian la bitacora de errores del sistema.
	if ctx.Causa != CausaValidacion && ctx.Causa != CausaPermiso && ctx.Causa != CausaSesion {
		a.registrarError(peticion.Context(), detalle)
		a.Bitacora.Error(mensaje,
			"id", detalle.ID, "modulo", detalle.Modulo, "accion", detalle.Accion,
			"causa", detalle.Causa, "error", detalle.Error)
	}

	codigo := ctx.Codigo
	if codigo == 0 {
		codigo = http.StatusInternalServerError
	}
	escribirJSON(escritor, codigo, Respuesta{Ok: false, Error: &detalle})
}

// registrarError guarda el error en el catalogo. Es lo que permite ver todos los
// errores de produccion sin depender de que el usuario los reporte.
func (a *API) registrarError(ctx context.Context, detalle ErrorDetalle) {
	// El guardado no puede depender de la peticion que fallo: si el usuario
	// cancelo o se cayo la conexion, el error igual tiene que quedar anotado.
	guardar, cancelar := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancelar()

	_, err := a.Datos.Catalogo.ExecContext(guardar, `
		INSERT INTO log_errores (id, momento, modulo, accion, causa, ruta, tabla,
		                         endpoint, origen, version, build, so, error, extra)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'servidor', ?, ?, ?, ?, ?)`,
		detalle.ID, detalle.Momento, detalle.Modulo, detalle.Accion, detalle.Causa,
		nuloSiVacio(detalle.Ruta), nuloSiVacio(detalle.Tabla), detalle.Endpoint,
		detalle.Version, detalle.Build, detalle.SO,
		primerosCaracteres(detalle.Error, 4000), nuloSiVacio(detalle.Mensaje))
	if err != nil {
		// Si ni la bitacora se puede escribir, al menos que quede en el diario
		// del sistema. No se reintenta: se perderia mas de lo que se gana.
		a.Bitacora.Error("no se pudo guardar el error en la bitacora",
			"id", detalle.ID, "error", err)
	}
}

// RegistrarErrorExterno guarda un error reportado por la interfaz o por la
// sonda, con el mismo formato que los del servidor.
func (a *API) RegistrarErrorExterno(ctx context.Context, origen string, detalle ErrorDetalle, trayectoria, pila string) error {
	if detalle.ID == "" {
		detalle.ID = generarIDError()
	}
	if detalle.Momento == "" {
		detalle.Momento = time.Now().Format(time.RFC3339)
	}

	_, err := a.Datos.Catalogo.ExecContext(ctx, `
		INSERT INTO log_errores (id, momento, modulo, accion, causa, ruta, tabla,
		                         endpoint, origen, version, build, so,
		                         trayectoria, pila, error, extra)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO NOTHING`,
		detalle.ID, detalle.Momento, valorOAlterno(detalle.Modulo, "desconocido"),
		valorOAlterno(detalle.Accion, "desconocida"), valorOAlterno(detalle.Causa, CausaInterno),
		nuloSiVacio(detalle.Ruta), nuloSiVacio(detalle.Tabla), nuloSiVacio(detalle.Endpoint),
		origen, valorOAlterno(detalle.Version, "?"), valorOAlterno(detalle.Build, "?"),
		valorOAlterno(detalle.SO, "?"), nuloSiVacio(primerosCaracteres(trayectoria, 8000)),
		nuloSiVacio(primerosCaracteres(pila, 8000)),
		primerosCaracteres(valorOAlterno(detalle.Error, detalle.Mensaje), 4000),
		nuloSiVacio(detalle.Mensaje))
	return err
}

func escribirJSON(escritor http.ResponseWriter, codigo int, cuerpo any) {
	escritor.Header().Set("Content-Type", "application/json; charset=utf-8")
	escritor.WriteHeader(codigo)
	if err := json.NewEncoder(escritor).Encode(cuerpo); err != nil {
		slog.Error("no se pudo escribir la respuesta", "error", err)
	}
}

// generarIDError produce el identificador corto que el usuario copia y soporte
// busca en la bitacora: ERR-9F3A2B1C.
func generarIDError() string {
	crudo := make([]byte, 4)
	if _, err := rand.Read(crudo); err != nil {
		return "ERR-00000000"
	}
	return "ERR-" + strings.ToUpper(hex.EncodeToString(crudo))
}

// origenLlamada devuelve el archivo y la linea donde se detecto el error, para
// no tener que buscarlo a ciegas.
func origenLlamada() string {
	// 0 es esta funcion, 1 responderError, 2 quien lo llamo de verdad.
	if _, archivo, linea, ok := runtime.Caller(2); ok {
		if corte := strings.LastIndex(archivo, "/mired/"); corte >= 0 {
			archivo = archivo[corte+len("/mired/"):]
		}
		return archivo + ":" + itoa(linea)
	}
	return ""
}

func itoa(numero int) string {
	if numero == 0 {
		return "0"
	}
	var digitos []byte
	for numero > 0 {
		digitos = append([]byte{byte('0' + numero%10)}, digitos...)
		numero /= 10
	}
	return string(digitos)
}

func nuloSiVacio(texto string) any {
	if texto == "" {
		return nil
	}
	return texto
}

func valorOAlterno(texto, alterno string) string {
	if strings.TrimSpace(texto) == "" {
		return alterno
	}
	return texto
}

func primerosCaracteres(texto string, tope int) string {
	if len(texto) <= tope {
		return texto
	}
	return texto[:tope] + "…"
}
