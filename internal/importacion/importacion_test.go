package importacion

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// La hoja de la clinica, tal como estaba: el titulo arriba, un renglon en
// blanco, los encabezados con OTROS nombres (PUERTO, NODO, UBICACION,
// OBSERVACIONES) y al final, tras dos renglones vacios, el unico nodo que no
// cuelga del switch.
const hojaDeLaClinica = `RED ALAMBRICA CLINICA ESQUIPULAS,,,
,,,
PUERTO,NODO,UBICACION,OBSERVACIONES
1,router,site,
2,serv1,site,servidor 1
5,D01,admon,
10,D06,dental,fortinet lan 4
19,D15,farmacia,se cambio jack
,,,
,,,
modem 2,D19,usg,antena deco
`

func TestSeLeeLaHojaDeLaClinicaTalComoEstaba(t *testing.T) {
	hoja, err := Leer([]byte(hojaDeLaClinica))
	if err != nil {
		t.Fatalf("no se pudo leer la hoja: %v", err)
	}

	// Los encabezados NO estan en el primer renglon: arriba va el titulo del
	// sitio y un renglon en blanco. Una hoja de verdad casi nunca empieza en la
	// fila 1, y lo que MiRed exporta tampoco.
	if len(hoja.Filas) != 6 {
		t.Fatalf("se esperaban 6 renglones con datos y hay %d", len(hoja.Filas))
	}

	// NODO y OBSERVACIONES son sinonimos: la hoja que lleva anos en uso se
	// importa sin que nadie renombre una sola columna.
	primero := hoja.Filas[0]
	if primero.Valor(ColNombre) != "router" {
		t.Fatalf("NODO tenia que reconocerse como NOMBRE: %q", primero.Valor(ColNombre))
	}
	if primero.Valor(ColUbicacion) != "site" {
		t.Fatalf("no se leyo la ubicacion: %q", primero.Valor(ColUbicacion))
	}
	if hoja.Filas[3].Valor(ColNotas) != "fortinet lan 4" {
		t.Fatalf("OBSERVACIONES tenia que reconocerse como NOTAS: %q",
			hoja.Filas[3].Valor(ColNotas))
	}

	// Y el renglon del archivo se conserva para poder senalarlo: el primer dato
	// esta en el cuarto renglon, no en el primero.
	if primero.Renglon != 4 {
		t.Fatalf("el numero de renglon esta mal: %d", primero.Renglon)
	}
}

func TestLaHojaDeLaClinicaAvisaDeLoQueLeFalta(t *testing.T) {
	hoja, err := Leer([]byte(hojaDeLaClinica))
	if err != nil {
		t.Fatalf("no se pudo leer: %v", err)
	}
	plan := Planear(hoja, map[string]int64{})

	// Su hoja no dice QUE ES cada nodo ni de que switch son los puertos, asi que
	// se rechaza entera **diciendo que falta**, en vez de inventarse una
	// categoria y meter 23 aparatos mal clasificados.
	if plan.Crear != 0 || plan.Rechazar != 6 {
		t.Fatalf("se esperaban 6 rechazos y 0 altas; hubo %d y %d",
			plan.Rechazar, plan.Crear)
	}
	if !strings.Contains(plan.Renglones[0].Motivo, "QUE_ES") {
		t.Fatalf("el motivo tenia que nombrar la columna que falta: %q",
			plan.Renglones[0].Motivo)
	}
}

func TestElPuertoQueEsOtroAparatoSeExplica(t *testing.T) {
	// «modem 2» en la columna PUERTO son DOS datos —de que aparato y que puerto—
	// y van en dos columnas. El mensaje tiene que decir eso, no «valor invalido».
	_, _, err := puertoDe("modem 2")
	if err == nil {
		t.Fatal("«modem 2» no es un puerto y tenia que rechazarse")
	}
	if !strings.Contains(err.Error(), "CUELGA_DE") {
		t.Fatalf("el mensaje tiene que mandar a la columna CUELGA_DE: %q", err)
	}
}

func TestLaPlantillaSeLeeASiMisma(t *testing.T) {
	// Si la plantilla que se descarga no se pudiera volver a subir, no seria una
	// plantilla. Los renglones de ayuda empiezan por # y no cuentan como datos.
	hoja, err := Leer([]byte(PlantillaCSV()))
	if err != nil {
		t.Fatalf("la plantilla no se puede leer: %v", err)
	}
	if len(hoja.Filas) != 2 {
		t.Fatalf("la plantilla trae 2 ejemplos y se leyeron %d", len(hoja.Filas))
	}
	if len(hoja.Ignoradas) != 0 {
		t.Fatalf("la plantilla no puede traer columnas que MiRed no entienda: %v",
			hoja.Ignoradas)
	}

	plan := Planear(hoja, map[string]int64{})
	if plan.Crear != 2 || plan.Rechazar != 0 {
		t.Fatalf("los dos ejemplos tienen que poder importarse: %+v", plan)
	}
	// El segundo ejemplo cuelga del primero, que esta en el mismo archivo.
	if !plan.Renglones[1].TieneCable() {
		t.Fatal("el ejemplo tenia que dejar un cable dibujado")
	}
}

func TestLaHojaDeLaClinicaCompletaSeImporta(t *testing.T) {
	// La misma hoja, con las dos columnas que le faltaban. Es el caso que de
	// verdad importa: 24 aparatos, 24 cables y un puerto libre.
	completa := `RED ALAMBRICA CLINICA ESQUIPULAS,,,,,
,,,,,
NOMBRE,QUE_ES,PUERTOS,CUELGA_DE,PUERTO,UBICACION
switch site,switch,24,,,site
router,modem,,switch site,1,site
serv1,servidor,,switch site,2,site
D01,pc,,switch site,5,admon
D19,otro,,router,2,usg
`
	hoja, err := Leer([]byte(completa))
	if err != nil {
		t.Fatalf("no se pudo leer: %v", err)
	}
	plan := Planear(hoja, map[string]int64{})

	if plan.Crear != 5 || plan.Rechazar != 0 {
		t.Fatalf("se esperaban 5 altas sin rechazos: %+v", plan)
	}
	if plan.Cables != 4 {
		t.Fatalf("se esperaban 4 cables y hay %d", plan.Cables)
	}
	// El switch se declara con sus 24 puertos aunque solo se usen unos pocos: los
	// libres son la mitad de la utilidad del mapa cuando hay que conectar algo.
	if plan.Renglones[0].Puertos != 24 {
		t.Fatalf("el switch tenia que quedar con 24 puertos: %d", plan.Renglones[0].Puertos)
	}
	// «switch» a secas es el NO administrable: el que se captura a mano es
	// justamente el que ningun escaneo ve.
	if plan.Renglones[0].Categoria != "switch_simple" {
		t.Fatalf("«switch» tenia que ser switch_simple: %q", plan.Renglones[0].Categoria)
	}
	if plan.Renglones[1].Categoria != "gateway" {
		t.Fatalf("«modem» tenia que ser gateway: %q", plan.Renglones[1].Categoria)
	}
	// D19 cuelga del router, no del switch. Es el renglon raro de la hoja
	// original y con CUELGA_DE deja de ser un caso especial.
	ultimo := plan.Renglones[4]
	if ultimo.CuelgaDe != "router" || ultimo.PuertoNumero != 2 {
		t.Fatalf("D19 tenia que colgar del router por su puerto 2: %+v", ultimo)
	}
}

func TestUnNombreRepetidoEnElArchivoSeRechazaSenalandoElOtro(t *testing.T) {
	hoja, err := Leer([]byte("NOMBRE,QUE_ES\nD01,pc\nD02,pc\nd01,pc\n"))
	if err != nil {
		t.Fatalf("no se pudo leer: %v", err)
	}
	plan := Planear(hoja, map[string]int64{})

	if plan.Rechazar != 1 {
		t.Fatalf("se esperaba un rechazo y hubo %d", plan.Rechazar)
	}
	// Se compara sin distinguir mayusculas: «D01» y «d01» son el mismo aparato
	// para cualquiera que mire el rack.
	if !strings.Contains(plan.Renglones[2].Motivo, "renglon 2") {
		t.Fatalf("el motivo tenia que senalar el renglon anterior: %q",
			plan.Renglones[2].Motivo)
	}
}

func TestLoQueYaEstaEnLaRedSaleComoActualizar(t *testing.T) {
	hoja, err := Leer([]byte("NOMBRE,QUE_ES,UBICACION\nD01,pc,farmacia\nD02,pc,dental\n"))
	if err != nil {
		t.Fatalf("no se pudo leer: %v", err)
	}
	plan := Planear(hoja, map[string]int64{"D01": 7})

	if plan.Crear != 1 || plan.Actualizar != 1 {
		t.Fatalf("se esperaba 1 alta y 1 actualizacion: %+v", plan)
	}
	if plan.Renglones[0].ExisteID != 7 {
		t.Fatalf("no se reconocio el equipo que ya estaba: %+v", plan.Renglones[0])
	}
}

func TestUnPadreQueNoExisteSeRechaza(t *testing.T) {
	hoja, _ := Leer([]byte("NOMBRE,QUE_ES,CUELGA_DE,PUERTO\nD01,pc,switch fantasma,3\n"))
	plan := Planear(hoja, map[string]int64{})

	if plan.Rechazar != 1 {
		t.Fatalf("colgar de un aparato que no existe tiene que rechazarse: %+v", plan)
	}
	if !strings.Contains(plan.Renglones[0].Motivo, "switch fantasma") {
		t.Fatalf("el motivo tiene que decir cual falta: %q", plan.Renglones[0].Motivo)
	}
}

func TestElPuntoYComaDeExcelSeReconoce(t *testing.T) {
	// Un Excel en espanol guarda el CSV con punto y coma. Leido con coma, el
	// archivo entero cae en una sola columna: es la primera causa de «el
	// importador no funciona».
	hoja, err := Leer([]byte("NOMBRE;QUE_ES;UBICACION\nD01;pc;farmacia\n"))
	if err != nil {
		t.Fatalf("no se pudo leer con punto y coma: %v", err)
	}
	if hoja.Filas[0].Valor(ColUbicacion) != "farmacia" {
		t.Fatalf("no se separaron las columnas: %+v", hoja.Filas[0])
	}
}

func TestLaMarcaDeCodificacionDeExcelNoRompeElPrimerEncabezado(t *testing.T) {
	hoja, err := Leer([]byte("\uFEFFNOMBRE,QUE_ES\nD01,pc\n"))
	if err != nil {
		t.Fatalf("no se pudo leer con marca de codificacion: %v", err)
	}
	if hoja.Filas[0].Valor(ColNombre) != "D01" {
		t.Fatalf("los tres bytes invisibles se comieron el encabezado: %+v", hoja.Filas[0])
	}
}

func TestSeAvisaDeLasColumnasQueMiRedNoEntiende(t *testing.T) {
	// Callarlas seria peor que rechazarlas: quien capturo «RESPONSABLE» tiene que
	// enterarse de que ese dato no se guardo en ningun sitio.
	hoja, _ := Leer([]byte("NOMBRE,QUE_ES,RESPONSABLE\nD01,pc,Juan\n"))
	if len(hoja.Ignoradas) != 1 || hoja.Ignoradas[0] != "RESPONSABLE" {
		t.Fatalf("no se aviso de la columna desconocida: %v", hoja.Ignoradas)
	}
}

func TestSeLeenLosDatosDeAccesoYSeCuentanLasClaves(t *testing.T) {
	hoja, _ := Leer([]byte(
		"NOMBRE,QUE_ES,IP,USUARIO,CLAVE\nswitch site,switch,192.168.1.2,admin,secreta\n"))
	plan := Planear(hoja, map[string]int64{})

	renglon := plan.Renglones[0]
	if renglon.Usuario != "admin" || renglon.Clave != "secreta" {
		t.Fatalf("no se leyo el acceso: %+v", renglon)
	}
	// Sin columna DIRECCION, el panel de un aparato con IP es su IP.
	if renglon.Direccion != "http://192.168.1.2" {
		t.Fatalf("la direccion del panel tenia que salir de la IP: %q", renglon.Direccion)
	}
	if renglon.Acceso != "web" {
		t.Fatalf("por omision se entra por el panel web: %q", renglon.Acceso)
	}
	// Se cuentan para poder avisar de que el ARCHIVO lleva claves en claro.
	if plan.ConClave != 1 {
		t.Fatalf("no se conto la clave: %d", plan.ConClave)
	}
}

func TestUnaIPOUnaMACInventadaSeRechaza(t *testing.T) {
	hoja, _ := Leer([]byte("NOMBRE,QUE_ES,IP\nD01,pc,192.168.1.999\n"))
	plan := Planear(hoja, map[string]int64{})
	if plan.Rechazar != 1 {
		t.Fatalf("una IP imposible tiene que rechazarse: %+v", plan.Renglones[0])
	}

	// Y una MAC de doce digitos seguidos, que es como la escribe media
	// herramienta del mundo, se acepta y se normaliza.
	hoja, _ = Leer([]byte("NOMBRE,QUE_ES,MAC\nD01,pc,B42E993BE376\n"))
	plan = Planear(hoja, map[string]int64{})
	if plan.Renglones[0].MAC != "b4:2e:99:3b:e3:76" {
		t.Fatalf("la MAC no se normalizo: %q", plan.Renglones[0].MAC)
	}
}

// ------------------------------------------------------------ ODS y XLSX ---

func TestSeLeeUnOdsDeLibreOffice(t *testing.T) {
	// Con `number-columns-repeated`, que es como LibreOffice guarda las celdas
	// vacias seguidas. Sin tenerlo en cuenta, TODAS las columnas a partir del
	// primer hueco salen corridas.
	contenido := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content
 xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
 xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0"
 xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
<office:body><office:spreadsheet>
<table:table table:name="Hoja1">
<table:table-row><table:table-cell office:value-type="string"><text:p>RED DE LA CLINICA</text:p></table:table-cell></table:table-row>
<table:table-row><table:table-cell table:number-columns-repeated="4"/></table:table-row>
<table:table-row>
 <table:table-cell><text:p>NOMBRE</text:p></table:table-cell>
 <table:table-cell><text:p>QUE_ES</text:p></table:table-cell>
 <table:table-cell><text:p>CUELGA_DE</text:p></table:table-cell>
 <table:table-cell><text:p>PUERTO</text:p></table:table-cell>
 <table:table-cell><text:p>UBICACION</text:p></table:table-cell>
</table:table-row>
<table:table-row>
 <table:table-cell><text:p>switch site</text:p></table:table-cell>
 <table:table-cell><text:p>switch</text:p></table:table-cell>
 <table:table-cell table:number-columns-repeated="2"/>
 <table:table-cell><text:p>site</text:p></table:table-cell>
</table:table-row>
<table:table-row>
 <table:table-cell><text:p>D01</text:p></table:table-cell>
 <table:table-cell><text:p>pc</text:p></table:table-cell>
 <table:table-cell><text:p>switch site</text:p></table:table-cell>
 <table:table-cell><text:p>5</text:p></table:table-cell>
 <table:table-cell><text:p>admon</text:p></table:table-cell>
</table:table-row>
</table:table>
</office:spreadsheet></office:body></office:document-content>`

	paquete := armarPaquete(t, map[string]string{
		"mimetype":    "application/vnd.oasis.opendocument.spreadsheet",
		"content.xml": contenido,
	})

	hoja, err := Leer(paquete)
	if err != nil {
		t.Fatalf("no se pudo leer el ODS: %v", err)
	}
	if len(hoja.Filas) != 2 {
		t.Fatalf("se esperaban 2 renglones y hay %d", len(hoja.Filas))
	}
	// La ubicacion del switch esta en la QUINTA columna, con dos celdas vacias
	// comprimidas delante. Si el repetido se ignora, aqui saldria «site» en
	// CUELGA_DE.
	if hoja.Filas[0].Valor(ColUbicacion) != "site" {
		t.Fatalf("las celdas vacias comprimidas corrieron las columnas: %+v", hoja.Filas[0])
	}
	if hoja.Filas[0].Valor(ColCuelgaDe) != "" {
		t.Fatalf("el switch no cuelga de nadie: %+v", hoja.Filas[0])
	}
	if hoja.Filas[1].Valor(ColPuerto) != "5" {
		t.Fatalf("no se leyo el puerto: %+v", hoja.Filas[1])
	}
}

func TestSeLeeUnXlsxDeExcel(t *testing.T) {
	// Con tabla compartida de textos y celdas vacias omitidas, que es como
	// guarda Excel. La celda dice en que columna esta («C2»), y sin colocarla por
	// su letra el hueco corre todo lo que viene detras.
	libro := `<?xml version="1.0"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"
 xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="Aparatos" sheetId="1" r:id="rId1"/></sheets></workbook>`
	enlaces := `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`
	textos := `<?xml version="1.0"?><sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<si><t>NOMBRE</t></si><si><t>QUE_ES</t></si><si><t>UBICACION</t></si>
<si><t>D01</t></si><si><t>pc</t></si><si><t>farmacia</t></si></sst>`
	hojaXML := `<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>
<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c><c r="C1" t="s"><v>2</v></c></row>
<row r="2"><c r="A2" t="s"><v>3</v></c><c r="B2" t="s"><v>4</v></c><c r="C2" t="s"><v>5</v></c></row>
</sheetData></worksheet>`

	paquete := armarPaquete(t, map[string]string{
		"xl/workbook.xml":            libro,
		"xl/_rels/workbook.xml.rels": enlaces,
		"xl/sharedStrings.xml":       textos,
		"xl/worksheets/sheet1.xml":   hojaXML,
	})

	hoja, err := Leer(paquete)
	if err != nil {
		t.Fatalf("no se pudo leer el XLSX: %v", err)
	}
	if len(hoja.Filas) != 1 {
		t.Fatalf("se esperaba 1 renglon y hay %d", len(hoja.Filas))
	}
	if hoja.Filas[0].Valor(ColNombre) != "D01" ||
		hoja.Filas[0].Valor(ColUbicacion) != "farmacia" {
		t.Fatalf("no se resolvio la tabla compartida de textos: %+v", hoja.Filas[0])
	}
}

func TestUnArchivoQueNoEsUnaHojaSeExplica(t *testing.T) {
	if _, err := Leer(nil); err == nil {
		t.Fatal("un archivo vacio tiene que dar error")
	}
	_, err := Leer([]byte("hola que tal\nesto no es una hoja\n"))
	if err == nil || !strings.Contains(err.Error(), "encabezados") {
		t.Fatalf("tenia que decir que no encontro los encabezados: %v", err)
	}
}

// armarPaquete arma un ZIP en memoria con los archivos dados.
func armarPaquete(t *testing.T, archivos map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	escritor := zip.NewWriter(&buffer)
	for nombre, contenido := range archivos {
		dentro, err := escritor.Create(nombre)
		if err != nil {
			t.Fatalf("no se pudo armar el paquete: %v", err)
		}
		if _, err := dentro.Write([]byte(contenido)); err != nil {
			t.Fatalf("no se pudo escribir %s: %v", nombre, err)
		}
	}
	if err := escritor.Close(); err != nil {
		t.Fatalf("no se pudo cerrar el paquete: %v", err)
	}
	return buffer.Bytes()
}
