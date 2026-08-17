package api

import (
	"encoding/base64"
	"net/http"

	"github.com/tuxormax/mired/internal/autenticacion"
	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/importacion"
)

// Subir una hoja de aparatos y meterla en la red de una vez.
//
// Son DOS pasos a proposito: primero se pide la vista previa, que no escribe
// nada y devuelve renglon por renglon lo que se haria, y solo despues se aplica.
// Importar a ciegas 23 aparatos y descubrir luego que tres estaban mal significa
// borrarlos a mano de uno en uno.
//
// El archivo llega **entero y en cada paso**, en vez de guardarse a medias entre
// los dos: asi el servidor no tiene que recordar nada entre peticiones y dos
// personas pueden importar a la vez sin pisarse.

// archivoImportado es lo que manda el programa: el archivo tal cual, en base64.
//
// Va en base64 y no como formulario porque el resto de la API habla JSON y no
// hay razon para que esta ruta sea la rara. Los archivos son de kilobytes.
type archivoImportado struct {
	Nombre    string `json:"nombre"`
	Contenido string `json:"contenido"`
	// Repetidos solo lo usa el paso de aplicar: "actualizar" o "saltar".
	Repetidos string `json:"repetidos"`
}

// plantillaDeImportacion entrega el archivo de ejemplo y la guia para llenarlo.
//
// Las dos cosas juntas y de la MISMA fuente: la guia que se lee en pantalla sale
// de la definicion con la que el servidor lee el archivo, asi que el dia que se
// agregue una columna no puede quedarse diciendo lo que ya no es.
func (a *API) plantillaDeImportacion(escritor http.ResponseWriter, peticion *http.Request) {
	responderOk(escritor, map[string]any{
		"nombre":    "mired-plantilla-aparatos.csv",
		"contenido": importacion.PlantillaCSV(),
		"guia":      importacion.Guia(),
	})
}

// vistaPreviaImportacion dice que pasaria, sin tocar la base.
func (a *API) vistaPreviaImportacion(escritor http.ResponseWriter, peticion *http.Request) {
	if !a.exigeEscritura(escritor, peticion, "Importar aparatos") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	plan, listo := a.planearArchivo(escritor, peticion, clave)
	if !listo {
		return
	}
	responderOk(escritor, plan)
}

// aplicarImportacion escribe lo que la vista previa enseno.
func (a *API) aplicarImportacion(escritor http.ResponseWriter, peticion *http.Request) {
	if !a.exigeEscritura(escritor, peticion, "Importar aparatos") {
		return
	}
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	var cuerpo archivoImportado
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Importacion", "Importar aparatos") {
		return
	}
	repetidos := cuerpo.Repetidos
	if repetidos != importacion.RepetidosSaltar {
		repetidos = importacion.RepetidosActualizar
	}

	plan, listo := a.planDelCuerpo(escritor, peticion, clave, cuerpo)
	if !listo {
		return
	}
	// Un archivo entero rechazado no se aplica: no hay nada que escribir y
	// responder "listo, 0 aparatos" se lee como si hubiera funcionado.
	if plan.Crear == 0 && plan.Actualizar == 0 {
		a.errorValidacion(escritor, peticion, "Importacion", "Importar aparatos",
			"No hay ni un renglon que se pueda importar. Mire los motivos de la vista previa.")
		return
	}

	var resumen basedatos.ResumenImportacion
	err := a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		resumen, err = base.AplicarImportacion(peticion.Context(), a.Secretos, plan, repetidos)
		return err
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Importacion", Accion: "Importar aparatos", Causa: CausaBaseDatos,
			Tabla: "equipos", Codigo: http.StatusInternalServerError,
		}, "No se pudo importar el archivo. No se guardo nada: la importacion entera "+
			"va en una sola transaccion.", err)
		return
	}

	a.anotarActividad(peticion, "Importacion", nombreDeArchivo(cuerpo.Nombre)+
		" en "+clave+": "+resumenEnPalabras(resumen))
	responderOk(escritor, resumen)
}

// planearArchivo lee el cuerpo y arma el plan.
func (a *API) planearArchivo(escritor http.ResponseWriter, peticion *http.Request,
	clave string) (importacion.Plan, bool) {
	var cuerpo archivoImportado
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Importacion", "Vista previa") {
		return importacion.Plan{}, false
	}
	return a.planDelCuerpo(escritor, peticion, clave, cuerpo)
}

func (a *API) planDelCuerpo(escritor http.ResponseWriter, peticion *http.Request,
	clave string, cuerpo archivoImportado) (importacion.Plan, bool) {
	datos, err := base64.StdEncoding.DecodeString(cuerpo.Contenido)
	if err != nil {
		a.errorValidacion(escritor, peticion, "Importacion", "Vista previa",
			"El archivo no llego completo. Vuelva a elegirlo.")
		return importacion.Plan{}, false
	}

	hoja, err := importacion.Leer(datos)
	if err != nil {
		// Es un error de lo que trae el archivo, no una averia: se dice tal cual,
		// que para eso el lector explica en cristiano lo que le falta.
		a.errorValidacion(escritor, peticion, "Importacion", "Vista previa", err.Error())
		return importacion.Plan{}, false
	}

	var existentes map[string]int64
	err = a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		var err error
		existentes, err = base.NombresDeEquipos(peticion.Context())
		return err
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Importacion", Accion: "Vista previa", Causa: CausaBaseDatos,
			Tabla: "equipos", Codigo: http.StatusInternalServerError,
		}, "No se pudo mirar que aparatos hay ya en la red.", err)
		return importacion.Plan{}, false
	}

	return importacion.Planear(hoja, existentes), true
}

// nombreDeArchivo deja solo el nombre, sin la ruta del equipo de quien importa.
//
// La bitacora no tiene por que guardar «/home/fulano/Documentos/...»: es el
// equipo de una persona y no aporta nada a lo que se hizo en la red.
func nombreDeArchivo(ruta string) string {
	if ruta == "" {
		return "archivo"
	}
	for i := len(ruta) - 1; i >= 0; i-- {
		if ruta[i] == '/' || ruta[i] == '\\' {
			return ruta[i+1:]
		}
	}
	return ruta
}

func resumenEnPalabras(resumen basedatos.ResumenImportacion) string {
	return itoa(resumen.Creados) + " creados, " +
		itoa(resumen.Actualizados) + " actualizados, " +
		itoa(resumen.Cables) + " cables"
}
