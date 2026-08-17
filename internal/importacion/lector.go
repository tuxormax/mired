package importacion

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Este archivo saca una rejilla de texto de los tres formatos que la gente usa:
// CSV, ODS (LibreOffice) y XLSX (Excel).
//
// Los tres se leen con la biblioteca estandar de Go y nada mas: `encoding/csv`,
// `archive/zip` y `encoding/xml`. Un ODS y un XLSX son un ZIP con XML dentro, y
// leerlos es abrir el paquete y recorrer las filas. La misma decision que en la
// interfaz, donde esos dos formatos se ESCRIBEN a mano.
//
// Aceptar los tres es lo que evita el «guardar como CSV» de cada vez, que es
// justo donde la gente se equivoca de separador y acaba con todo en una columna.

// Fila es un renglon ya reconocido: cada columna de la plantilla con su valor.
type Fila struct {
	// Renglon es el numero de renglon EN EL ARCHIVO, contando desde 1 e
	// incluyendo el titulo y los encabezados. Sin esto, decir «el renglon 4 esta
	// mal» manda a la persona a mirar otro sitio.
	Renglon int
	Valores map[string]string
}

// Valor devuelve el contenido de una columna, ya sin espacios de sobra.
func (f Fila) Valor(columna string) string {
	return strings.TrimSpace(f.Valores[columna])
}

// Hoja es lo leido de un archivo: las columnas que se reconocieron, las que no,
// y los renglones.
type Hoja struct {
	// Reconocidas son las claves de columna encontradas, en el orden del archivo.
	Reconocidas []string
	// Ignoradas son los encabezados que traia el archivo y MiRed no entiende. Se
	// dicen en vez de callarlos: quien capturo una columna «RESPONSABLE» tiene
	// que enterarse de que ese dato no se guardo.
	Ignoradas []string
	Filas     []Fila
}

// Leer saca la hoja de un archivo, adivinando el formato por su contenido.
//
// Se mira el contenido y no la extension: un archivo renombrado a mano es lo
// mas normal del mundo, y fallar por el nombre seria fallar por nada.
func Leer(datos []byte) (Hoja, error) {
	switch {
	case len(datos) == 0:
		return Hoja{}, errors.New("el archivo esta vacio")
	// Todo ZIP empieza igual. ODS y XLSX son ZIP; el CSV no.
	case len(datos) > 4 && datos[0] == 'P' && datos[1] == 'K':
		return leerPaquete(datos)
	default:
		return leerCSV(datos)
	}
}

// -------------------------------------------------------------------- CSV ---

func leerCSV(datos []byte) (Hoja, error) {
	// La marca de codificacion que pone Excel al guardar. Si no se quita, el
	// primer encabezado llega con tres bytes invisibles pegados y no se reconoce.
	texto := strings.TrimPrefix(string(datos), "\uFEFF")

	lector := csv.NewReader(strings.NewReader(texto))
	lector.Comma = separadorDe(texto)
	// Un archivo de verdad tiene renglones con distinto numero de columnas: el
	// titulo de arriba ocupa una sola celda. Sin esto, el lector para en el
	// segundo renglon.
	lector.FieldsPerRecord = -1
	// Comillas sueltas en medio de un texto —«el cable de 3"»— no deben tumbar la
	// lectura de un archivo entero.
	lector.LazyQuotes = true

	rejilla, err := lector.ReadAll()
	if err != nil {
		return Hoja{}, fmt.Errorf("no se pudo leer el CSV: %w", err)
	}
	return deRejilla(rejilla)
}

// separadorDe adivina con que se separan las columnas.
//
// En un Excel en espanol de Windows el separador de lista es el punto y coma, y
// el archivo guardado asi, leido con coma, sale entero en una sola columna. Es
// la primera causa de «el importador no funciona», y se resuelve contando.
func separadorDe(texto string) rune {
	renglones := strings.SplitN(texto, "\n", 20)
	cuenta := map[rune]int{',': 0, ';': 0, '\t': 0, '|': 0}
	for _, renglon := range renglones {
		for separador := range cuenta {
			cuenta[separador] += strings.Count(renglon, string(separador))
		}
	}
	mejor, cuantos := ',', cuenta[',']
	for _, separador := range []rune{';', '\t', '|'} {
		if cuenta[separador] > cuantos {
			mejor, cuantos = separador, cuenta[separador]
		}
	}
	return mejor
}

// ------------------------------------------------------------ ODS y XLSX ---

func leerPaquete(datos []byte) (Hoja, error) {
	paquete, err := zip.NewReader(bytes.NewReader(datos), int64(len(datos)))
	if err != nil {
		return Hoja{}, fmt.Errorf("el archivo parece una hoja de calculo pero no se puede abrir: %w", err)
	}

	dentro := map[string]*zip.File{}
	for _, archivo := range paquete.File {
		dentro[archivo.Name] = archivo
	}

	if _, esOds := dentro["content.xml"]; esOds {
		return leerOds(dentro)
	}
	if _, esXlsx := dentro["xl/workbook.xml"]; esXlsx {
		return leerXlsx(dentro)
	}
	return Hoja{}, errors.New(
		"el archivo es un paquete pero no es un ODS ni un XLSX: guarde la hoja como CSV")
}

func contenidoDe(dentro map[string]*zip.File, nombre string) ([]byte, error) {
	archivo, hay := dentro[nombre]
	if !hay {
		return nil, fmt.Errorf("a la hoja de calculo le falta %s", nombre)
	}
	abierto, err := archivo.Open()
	if err != nil {
		return nil, err
	}
	defer abierto.Close()
	return io.ReadAll(abierto)
}

// odsContenido es lo minimo del content.xml de un ODS: sus hojas.
type odsContenido struct {
	Hojas []odsTabla `xml:"body>spreadsheet>table"`
}

type odsTabla struct {
	Nombre string    `xml:"name,attr"`
	Filas  []odsFila `xml:"table-row"`
}

type odsFila struct {
	// Repetidas: LibreOffice no escribe mil filas vacias iguales, escribe una y
	// dice «esta va mil veces». Lo mismo con las celdas. Ignorarlo desplaza
	// TODAS las columnas a partir del primer hueco.
	Repetidas int        `xml:"number-rows-repeated,attr"`
	Celdas    []odsCelda `xml:"table-cell"`
}

type odsCelda struct {
	Repetidas int      `xml:"number-columns-repeated,attr"`
	Parrafos  []string `xml:"p"`
}

func leerOds(dentro map[string]*zip.File) (Hoja, error) {
	crudo, err := contenidoDe(dentro, "content.xml")
	if err != nil {
		return Hoja{}, err
	}

	var contenido odsContenido
	if err := xml.Unmarshal(crudo, &contenido); err != nil {
		return Hoja{}, fmt.Errorf("no se pudo leer el ODS: %w", err)
	}

	// Se prueban TODAS las hojas y se usa la primera con encabezados que MiRed
	// reconozca: en un libro con varias pestanas, la buena no tiene por que ser
	// la primera.
	var ultimoError error
	for _, tabla := range contenido.Hojas {
		rejilla := [][]string{}
		for _, fila := range tabla.Filas {
			renglon := []string{}
			for _, celda := range fila.Celdas {
				texto := strings.Join(celda.Parrafos, " ")
				for vez := 0; vez < repeticiones(celda.Repetidas); vez++ {
					renglon = append(renglon, texto)
					// Una celda vacia repetida mil veces al final del renglon no
					// aporta nada y solo infla la rejilla.
					if texto == "" && len(renglon) > 64 {
						break
					}
				}
			}
			for vez := 0; vez < repeticiones(fila.Repetidas); vez++ {
				rejilla = append(rejilla, renglon)
				if len(sinVacios(renglon)) == 0 && len(rejilla) > 4096 {
					break
				}
			}
		}
		hoja, err := deRejilla(rejilla)
		if err == nil {
			return hoja, nil
		}
		ultimoError = err
	}
	if ultimoError == nil {
		ultimoError = errors.New("el ODS no tiene ninguna hoja con datos")
	}
	return Hoja{}, ultimoError
}

// xlsxLibro es el indice de hojas del XLSX, en su orden.
type xlsxLibro struct {
	Hojas []struct {
		Nombre string `xml:"name,attr"`
		ID     string `xml:"id,attr"`
	} `xml:"sheets>sheet"`
}

type xlsxEnlaces struct {
	Enlaces []struct {
		ID     string `xml:"Id,attr"`
		Target string `xml:"Target,attr"`
	} `xml:"Relationship"`
}

type xlsxTextos struct {
	Textos []xlsxTexto `xml:"si"`
}

// xlsxTexto es una entrada de la tabla compartida. El texto puede venir suelto
// en <t> o partido en varios <r><t>, cuando lleva formato en medio.
type xlsxTexto struct {
	Suelto string   `xml:"t"`
	Trozos []string `xml:"r>t"`
}

func (t xlsxTexto) String() string {
	if t.Suelto != "" {
		return t.Suelto
	}
	return strings.Join(t.Trozos, "")
}

type xlsxHoja struct {
	Filas []struct {
		Celdas []struct {
			Ref    string `xml:"r,attr"`
			Tipo   string `xml:"t,attr"`
			Valor  string `xml:"v"`
			Enline string `xml:"is>t"`
		} `xml:"c"`
	} `xml:"sheetData>row"`
}

func leerXlsx(dentro map[string]*zip.File) (Hoja, error) {
	crudoLibro, err := contenidoDe(dentro, "xl/workbook.xml")
	if err != nil {
		return Hoja{}, err
	}
	var libro xlsxLibro
	if err := xml.Unmarshal(crudoLibro, &libro); err != nil {
		return Hoja{}, fmt.Errorf("no se pudo leer el XLSX: %w", err)
	}

	// La hoja no dice su archivo: dice un identificador que hay que buscar en el
	// indice de enlaces. Si no esta, se cae a la convencion de nombres.
	rutas := map[string]string{}
	if crudoEnlaces, err := contenidoDe(dentro, "xl/_rels/workbook.xml.rels"); err == nil {
		var enlaces xlsxEnlaces
		if err := xml.Unmarshal(crudoEnlaces, &enlaces); err == nil {
			for _, enlace := range enlaces.Enlaces {
				rutas[enlace.ID] = "xl/" + strings.TrimPrefix(enlace.Target, "/xl/")
			}
		}
	}

	// La tabla compartida de textos: Excel no repite un texto en cada celda, lo
	// guarda una vez y en la celda pone su numero.
	compartidos := []string{}
	if crudoTextos, err := contenidoDe(dentro, "xl/sharedStrings.xml"); err == nil {
		var textos xlsxTextos
		if err := xml.Unmarshal(crudoTextos, &textos); err == nil {
			for _, texto := range textos.Textos {
				compartidos = append(compartidos, texto.String())
			}
		}
	}

	var ultimoError error
	for numero, hoja := range libro.Hojas {
		ruta, hay := rutas[hoja.ID]
		if !hay {
			ruta = fmt.Sprintf("xl/worksheets/sheet%d.xml", numero+1)
		}
		crudoHoja, err := contenidoDe(dentro, ruta)
		if err != nil {
			ultimoError = err
			continue
		}
		var contenido xlsxHoja
		if err := xml.Unmarshal(crudoHoja, &contenido); err != nil {
			ultimoError = err
			continue
		}

		rejilla := [][]string{}
		for _, fila := range contenido.Filas {
			renglon := []string{}
			for _, celda := range fila.Celdas {
				// La celda dice EN QUE COLUMNA esta («D7»), y las vacias no se
				// escriben. Sin colocarlas por su letra, un hueco corre todo lo
				// que viene detras una columna a la izquierda.
				columna := columnaDeReferencia(celda.Ref)
				for len(renglon) < columna {
					renglon = append(renglon, "")
				}
				valor := celda.Enline
				if valor == "" {
					valor = celda.Valor
				}
				if celda.Tipo == "s" {
					indice, err := strconv.Atoi(strings.TrimSpace(celda.Valor))
					if err == nil && indice >= 0 && indice < len(compartidos) {
						valor = compartidos[indice]
					}
				}
				if columna < len(renglon) {
					renglon[columna] = valor
					continue
				}
				renglon = append(renglon, valor)
			}
			rejilla = append(rejilla, renglon)
		}

		leida, err := deRejilla(rejilla)
		if err == nil {
			return leida, nil
		}
		ultimoError = err
	}
	if ultimoError == nil {
		ultimoError = errors.New("el XLSX no tiene ninguna hoja con datos")
	}
	return Hoja{}, ultimoError
}

// columnaDeReferencia pasa «D7» a 3: la D es la cuarta columna.
func columnaDeReferencia(referencia string) int {
	columna := 0
	for _, letra := range strings.ToUpper(referencia) {
		if letra < 'A' || letra > 'Z' {
			break
		}
		columna = columna*26 + int(letra-'A'+1)
	}
	if columna <= 0 {
		return 0
	}
	return columna - 1
}

// ------------------------------------------------------------- la rejilla ---

// deRejilla busca los encabezados y arma los renglones.
//
// El renglon de encabezados **no tiene por que ser el primero**: las hojas de
// verdad traen arriba el titulo del sitio y algun renglon en blanco —la de la
// clinica los traia—, y lo que MiRed exporta tambien. Asi que se busca el primer
// renglon donde se reconozcan al menos dos columnas.
func deRejilla(rejilla [][]string) (Hoja, error) {
	dondeEmpieza := -1
	var columnas []string
	var ignoradas []string

	for numero, renglon := range rejilla {
		if esComentario(renglon) {
			continue
		}
		posibles := make([]string, len(renglon))
		reconocidas, desconocidas := 0, []string{}
		for i, celda := range renglon {
			clave, hay := columnaPorEncabezado(celda)
			if !hay {
				if strings.TrimSpace(celda) != "" {
					desconocidas = append(desconocidas, strings.TrimSpace(celda))
				}
				continue
			}
			// Una columna repetida se queda con la primera: dos columnas NOMBRE
			// no se pueden guardar las dos en el mismo campo.
			if yaEsta(posibles, clave) {
				continue
			}
			posibles[i] = clave
			reconocidas++
		}
		if reconocidas >= 2 {
			dondeEmpieza = numero
			columnas = posibles
			ignoradas = desconocidas
			break
		}
	}

	if dondeEmpieza < 0 {
		return Hoja{}, errors.New(
			"no se encontro el renglon de encabezados: la hoja tiene que traer al menos " +
				"NOMBRE y QUE_ES. Descargue la plantilla desde el mismo boton")
	}

	hoja := Hoja{Ignoradas: ignoradas}
	for _, clave := range columnas {
		if clave != "" {
			hoja.Reconocidas = append(hoja.Reconocidas, clave)
		}
	}

	for numero := dondeEmpieza + 1; numero < len(rejilla); numero++ {
		renglon := rejilla[numero]
		if esComentario(renglon) || len(sinVacios(renglon)) == 0 {
			continue
		}
		valores := map[string]string{}
		for i, clave := range columnas {
			if clave == "" || i >= len(renglon) {
				continue
			}
			valores[clave] = strings.TrimSpace(renglon[i])
		}
		if len(sinVacios(valoresDe(valores))) == 0 {
			continue // el renglon traia datos, pero ninguno en columnas conocidas
		}
		hoja.Filas = append(hoja.Filas, Fila{Renglon: numero + 1, Valores: valores})
	}

	if len(hoja.Filas) == 0 {
		return Hoja{}, errors.New("la hoja tiene encabezados pero ningun renglon con datos")
	}
	return hoja, nil
}

// esComentario dice si el renglon es de los que empiezan por # en la plantilla.
func esComentario(renglon []string) bool {
	for _, celda := range renglon {
		limpia := strings.TrimSpace(celda)
		if limpia == "" {
			continue
		}
		return strings.HasPrefix(limpia, "#")
	}
	return false
}

func repeticiones(cuantas int) int {
	if cuantas < 1 {
		return 1
	}
	// Una hoja de calculo declara al final «esta celda vacia va 16 384 veces».
	// Copiarlas todas no aporta un dato y llena la memoria de nada.
	if cuantas > 4096 {
		return 1
	}
	return cuantas
}

func sinVacios(renglon []string) []string {
	salida := []string{}
	for _, celda := range renglon {
		if strings.TrimSpace(celda) != "" {
			salida = append(salida, celda)
		}
	}
	return salida
}

func valoresDe(mapa map[string]string) []string {
	salida := make([]string, 0, len(mapa))
	for _, valor := range mapa {
		salida = append(salida, valor)
	}
	return salida
}

func yaEsta(lista []string, buscado string) bool {
	for _, valor := range lista {
		if valor == buscado {
			return true
		}
	}
	return false
}
