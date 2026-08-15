package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/tuxormax/mired/internal/autenticacion"
	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/catalogo"
)

// listarCatalogo muestra que dispositivos sabe reconocer esta instalacion.
//
// Incluye los archivos que no se pudieron cargar: quien escribio una definicion
// propia y no funciona necesita ver el error, no quedarse adivinando.
func (a *API) listarCatalogo(escritor http.ResponseWriter, peticion *http.Request) {
	if a.Catalogo == nil {
		responderOk(escritor, map[string]any{
			"definiciones": []any{},
			"problemas":    []string{},
			"aviso":        "Esta instalacion no tiene catalogo de dispositivos cargado.",
		})
		return
	}

	definiciones := a.Catalogo.Definiciones()
	lista := make([]map[string]any, 0, len(definiciones))
	for _, definicion := range definiciones {
		lista = append(lista, map[string]any{
			"nombre":      definicion.Nombre,
			"categoria":   definicion.Categoria,
			"icono":       definicion.Icono,
			"descripcion": definicion.Descripcion,
			"aporta":      definicion.Aporta,
			"archivo":     definicion.Archivo,
		})
	}

	// Los problemas se mandan siempre como lista, aunque este vacia: un null
	// obliga a la interfaz a defenderse de dos formas distintas del mismo dato.
	problemas := a.Catalogo.Problemas()
	if problemas == nil {
		problemas = []string{}
	}

	responderOk(escritor, map[string]any{
		"definiciones": lista,
		"problemas":    problemas,
	})
}

// proponerDefinicion genera el `.toml` de un equipo que nadie reconocio, ya
// relleno con lo que se vio.
//
// Es la puerta de entrada del catalogo comunitario: el usuario revisa, le pone
// nombre y lo manda. Sin esto, aportar exigiria entender el formato desde cero.
func (a *API) proponerDefinicion(escritor http.ResponseWriter, peticion *http.Request) {
	clave, _ := autenticacion.RedActivaDe(peticion.Context())

	id, err := strconv.ParseInt(peticion.PathValue("equipo"), 10, 64)
	if err != nil || id <= 0 {
		a.errorValidacion(escritor, peticion, "Catalogo", "Proponer definicion",
			"El equipo no es valido.")
		return
	}

	var encontrado *basedatos.DatosParaReconocer
	err = a.Datos.ConRed(peticion.Context(), clave, func(base *basedatos.Base) error {
		equipos, err := base.ParaReconocer(peticion.Context())
		if err != nil {
			return err
		}
		for i := range equipos {
			if equipos[i].ID == id {
				encontrado = &equipos[i]
				return nil
			}
		}
		return nil
	})
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Catalogo", Accion: "Proponer definicion", Causa: CausaBaseDatos,
			Tabla: "equipos", Codigo: http.StatusInternalServerError,
		}, "No se pudieron leer los datos del equipo.", err)
		return
	}
	if encontrado == nil {
		a.errorValidacion(escritor, peticion, "Catalogo", "Proponer definicion",
			"Ese equipo no existe en esta red.")
		return
	}

	nombre := peticion.URL.Query().Get("nombre")
	propuesta := catalogo.Proponer(catalogo.Equipo{
		IP:         encontrado.IP,
		MAC:        encontrado.MAC,
		Fabricante: encontrado.Fabricante,
		Nombre:     encontrado.Nombre,
		Puertos:    encontrado.Puertos,
		Banners:    encontrado.Banners,
		SnmpDescr:  encontrado.SnmpDescr,
		Huella:     encontrado.Huella,
		Modelo:     encontrado.Modelo,
	}, nombre)

	archivo := sugerirNombreArchivo(encontrado)
	responderOk(escritor, map[string]any{
		"archivo":   archivo,
		"contenido": propuesta,
		// La direccion donde aportarlo, ya escrita. Aportar es un acto de una
		// PERSONA con su cuenta: el servicio no publica por su cuenta, prepara
		// el aporte y quien decide es quien lo abre.
		"urlAporte": catalogo.URLParaAportar(nombre, archivo, propuesta),
	})
}

// guardarDefinicion deja una definicion en el catalogo DE ESTA instalacion.
//
// Es el primer paso del catalogo comunitario y el que sirve aunque nunca se
// comparta nada: el aparato queda reconocido aqui desde el proximo escaneo.
func (a *API) guardarDefinicion(escritor http.ResponseWriter, peticion *http.Request) {
	var cuerpo struct {
		Archivo   string `json:"archivo"`
		Contenido string `json:"contenido"`
	}
	if !a.leerCuerpo(escritor, peticion, &cuerpo, "Catalogo", "Guardar definicion") {
		return
	}

	carpeta := a.CarpetaPropia
	if carpeta == "" {
		carpeta = catalogo.CarpetaPropia
	}
	guardado, err := catalogo.Guardar(carpeta, cuerpo.Archivo, cuerpo.Contenido)
	if err != nil {
		a.errorValidacion(escritor, peticion, "Catalogo", "Guardar definicion", err.Error())
		return
	}

	recargado, problemas := a.recargarCatalogo()
	a.anotarActividad(peticion, "Catalogo", "Guardar la definicion "+guardado.Archivo)
	responderOk(escritor, map[string]any{
		"archivo":      guardado.Archivo,
		"ruta":         guardado.Ruta,
		"definiciones": recargado,
		"problemas":    problemas,
	})
}

// actualizarCatalogo trae las definiciones que publico la comunidad.
//
// Se guardan en su propia carpeta: actualizar NUNCA pisa una definicion escrita
// aqui, porque quien tiene el aparato delante sabe mas que el repositorio.
func (a *API) actualizarCatalogo(escritor http.ResponseWriter, peticion *http.Request) {
	carpeta := a.CarpetaComunidad
	if carpeta == "" {
		carpeta = catalogo.CarpetaComunidad
	}
	traidas, err := catalogo.Actualizar(peticion.Context(), carpeta)
	if err != nil {
		a.responderError(escritor, peticion, contextoError{
			Modulo: "Catalogo", Accion: "Actualizar catalogo", Causa: CausaRed,
			Codigo: http.StatusBadGateway,
		}, "No se pudieron traer las definiciones de la comunidad.", err)
		return
	}

	recargado, problemas := a.recargarCatalogo()
	a.anotarActividad(peticion, "Catalogo",
		fmt.Sprintf("Actualizar el catalogo de la comunidad (%d definiciones)", traidas.Bajadas))
	responderOk(escritor, map[string]any{
		"bajadas":      traidas.Bajadas,
		"nuevas":       traidas.Nuevas,
		"fallos":       traidas.Fallos,
		"definiciones": recargado,
		"problemas":    problemas,
	})
}

// recargarCatalogo vuelve a leer las tres carpetas sin reiniciar el servicio.
//
// Se reemplaza el CONTENIDO del catalogo y no el puntero: el servidor y el
// programador guardan el suyo por separado, y cambiar uno solo dejaria los
// escaneos reconociendo con el catalogo viejo.
func (a *API) recargarCatalogo() (int, []string) {
	if a.Catalogo == nil {
		return 0, []string{}
	}
	nuevo, err := catalogo.Cargar(a.CarpetasCatalogo)
	if err != nil || nuevo == nil {
		return len(a.Catalogo.Definiciones()), a.Catalogo.Problemas()
	}
	a.Catalogo.Reemplazar(nuevo)

	problemas := a.Catalogo.Problemas()
	if problemas == nil {
		problemas = []string{}
	}
	return len(a.Catalogo.Definiciones()), problemas
}

// sugerirNombreArchivo propone como llamar al `.toml`, para que quien aporte no
// tenga que inventarlo.
func sugerirNombreArchivo(equipo *basedatos.DatosParaReconocer) string {
	base := equipo.Fabricante
	if base == "" {
		base = equipo.Nombre
	}
	if base == "" {
		base = "dispositivo"
	}

	limpio := make([]rune, 0, len(base))
	for _, letra := range base {
		switch {
		case letra >= 'a' && letra <= 'z', letra >= '0' && letra <= '9':
			limpio = append(limpio, letra)
		case letra >= 'A' && letra <= 'Z':
			limpio = append(limpio, letra+32)
		case len(limpio) > 0 && limpio[len(limpio)-1] != '-':
			limpio = append(limpio, '-')
		}
	}
	if len(limpio) == 0 {
		return "dispositivo.toml"
	}
	return string(limpio) + ".toml"
}
