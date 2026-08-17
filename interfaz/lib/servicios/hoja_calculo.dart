import 'dart:convert';
import 'dart:typed_data';

import 'zip.dart';

/// Este archivo entrega una o varias tablas como hoja de calculo, en los tres
/// formatos que la gente abre: ODS (LibreOffice Calc), XLSX (Excel) y CSV.
///
/// **Cada tabla es una hoja.** Es el motivo de que exista: mezclar en una sola
/// tabla renglones que hablan de cosas distintas —un aparato, un cable— deja
/// columnas vacias que significan cosas diferentes segun la fila, y eso no hay
/// encabezado que lo salve. En ODS y en XLSX cada tabla va en su pestana; el
/// CSV no tiene pestanas, asi que van una tras otra, separadas y con su titulo.
///
/// Los tres formatos se escriben a mano, sin biblioteca de terceros. El ODS y el
/// XLSX son un ZIP con XML adentro; el ZIP lo arma [armarZip].

/// Tabla es una hoja: su nombre, sus encabezados y sus renglones.
class Tabla {
  const Tabla({
    required this.nombre,
    required this.encabezados,
    required this.filas,
    this.explicacion = '',
  });

  /// Como se llama la pestana. Corto: Excel no admite mas de 31 caracteres ni
  /// los signos `[]:*?/\`, y [_nombreDeHoja] se encarga de recortarlo.
  final String nombre;

  /// Una linea en cristiano de que contesta esta hoja. Va encima de los
  /// encabezados: quien recibe el archivo por correo no tiene a quien
  /// preguntarle.
  final String explicacion;

  final List<String> encabezados;
  final List<List<String>> filas;
}

// -------------------------------------------------------------------- CSV ---

/// csvDeTablas pega las tablas una tras otra en un solo texto.
///
/// El CSV no tiene hojas: es un archivo, una tabla. Cuando hay mas de una se
/// escriben seguidas, cada una con su nombre encima y un renglon en blanco de
/// por medio, que es lo mas parecido a no mezclarlas que el formato permite.
String csvDeTablas(List<Tabla> tablas, {String titulo = ''}) {
  final renglones = <String>[];
  if (titulo.isNotEmpty) {
    renglones..add(_paraCsv(titulo))..add('');
  }

  for (final tabla in tablas) {
    if (tablas.length > 1) {
      renglones.add(_paraCsv(tabla.nombre.toUpperCase()));
      if (tabla.explicacion.isNotEmpty) renglones.add(_paraCsv(tabla.explicacion));
    }
    renglones.add(tabla.encabezados.map(_paraCsv).join(','));
    for (final fila in tabla.filas) {
      renglones.add(fila.map(_paraCsv).join(','));
    }
    renglones.add('');
  }

  // Sobra el ultimo renglon en blanco: un archivo que termina en dos saltos deja
  // una fila fantasma al final de la hoja.
  while (renglones.isNotEmpty && renglones.last.isEmpty) {
    renglones.removeLast();
  }
  return renglones.join('\n');
}

/// csvEnBytes le pone al CSV la marca de codificacion antes del texto.
///
/// Sin esos tres bytes, Excel en Windows abre el archivo con la codificacion
/// local del equipo y «Camara» sale como «CÃ¡mara». LibreOffice acierta solo,
/// pero el archivo acaba en el correo de cualquiera. La marca es invisible en
/// los dos.
Uint8List csvEnBytes(String csv) =>
    Uint8List.fromList([0xEF, 0xBB, 0xBF, ...utf8.encode(csv)]);

String _paraCsv(String valor) =>
    valor.contains(',') || valor.contains('"') || valor.contains('\n')
        ? '"${valor.replaceAll('"', '""')}"'
        : valor;

// -------------------------------------------------------------------- ODS ---

/// odsDeTablas arma la hoja de calculo de LibreOffice.
///
/// El ODS es el formato abierto y documentado del sector, y lo abren tanto
/// LibreOffice como Excel. Son cuatro archivos XML dentro de un ZIP; el primero
/// tiene que ser `mimetype`, sin comprimir, o el lector no reconoce el paquete.
Uint8List odsDeTablas(List<Tabla> tablas, DateTime momento, {String titulo = ''}) {
  final hojas = StringBuffer();
  for (final tabla in tablas) {
    final columnas = _cuantasColumnas(tabla);
    hojas
      ..writeln('<table:table table:name="${_xml(_nombreDeHoja(tabla.nombre))}">')
      ..writeln('<table:table-column table:number-columns-repeated="$columnas"/>');

    for (final fila in _filasConTitulo(tabla, titulo)) {
      hojas.writeln('<table:table-row>');
      for (final celda in fila) {
        hojas.writeln(celda.isEmpty
            ? '<table:table-cell/>'
            : '<table:table-cell office:value-type="string">'
                '<text:p>${_xml(celda)}</text:p></table:table-cell>');
      }
      hojas.writeln('</table:table-row>');
    }
    hojas.writeln('</table:table>');
  }

  final contenido = '<?xml version="1.0" encoding="UTF-8"?>\n'
      '<office:document-content '
      'xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" '
      'xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" '
      'xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" '
      'office:version="1.3">\n'
      '<office:body><office:spreadsheet>\n$hojas'
      '</office:spreadsheet></office:body></office:document-content>\n';

  final manifiesto = '<?xml version="1.0" encoding="UTF-8"?>\n'
      '<manifest:manifest '
      'xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" '
      'manifest:version="1.3">\n'
      '<manifest:file-entry manifest:full-path="/" '
      'manifest:media-type="$_tipoOds"/>\n'
      '<manifest:file-entry manifest:full-path="content.xml" '
      'manifest:media-type="text/xml"/>\n'
      '<manifest:file-entry manifest:full-path="styles.xml" '
      'manifest:media-type="text/xml"/>\n'
      '</manifest:manifest>\n';

  // Un ODS sin estilos abre igual, pero LibreOffice se queja al guardarlo. Uno
  // vacio y bien formado le basta.
  const estilos = '<?xml version="1.0" encoding="UTF-8"?>\n'
      '<office:document-styles '
      'xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" '
      'office:version="1.3"><office:styles/></office:document-styles>\n';

  return armarZip([
    // El mimetype va PRIMERO y tal cual: es como se reconoce un ODS sin
    // descomprimirlo entero.
    EntradaZip.texto('mimetype', _tipoOds),
    EntradaZip.texto('META-INF/manifest.xml', manifiesto),
    EntradaZip.texto('styles.xml', estilos),
    EntradaZip.texto('content.xml', contenido),
  ], momento);
}

const String _tipoOds = 'application/vnd.oasis.opendocument.spreadsheet';

// ------------------------------------------------------------------- XLSX ---

/// xlsxDeTablas arma la hoja de calculo de Excel.
///
/// Se escribe con **cadenas en la propia celda** (`inlineStr`) en vez de la
/// tabla compartida de textos que usa Excel al guardar. Es igual de valido, lo
/// abren Excel y LibreOffice, y ahorra un archivo entero y el indice que habria
/// que llevar. Aqui no compensa: un mapa de red no repite tanto texto.
Uint8List xlsxDeTablas(List<Tabla> tablas, DateTime momento, {String titulo = ''}) {
  final entradas = <EntradaZip>[];
  final tipos = StringBuffer()
    ..write('<?xml version="1.0" encoding="UTF-8"?>')
    ..write('<Types xmlns="http://schemas.openxmlformats.org/package/2006/'
        'content-types">')
    ..write('<Default Extension="rels" ContentType="application/'
        'vnd.openxmlformats-package.relationships+xml"/>')
    ..write('<Default Extension="xml" ContentType="application/xml"/>')
    ..write('<Override PartName="/xl/libro.xml" ContentType="application/'
        'vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>');

  final indiceHojas = StringBuffer();
  final enlaces = StringBuffer()
    ..write('<?xml version="1.0" encoding="UTF-8"?>')
    ..write('<Relationships xmlns="http://schemas.openxmlformats.org/package/'
        '2006/relationships">');

  for (var numero = 1; numero <= tablas.length; numero++) {
    final tabla = tablas[numero - 1];
    final ruta = 'xl/hojas/hoja$numero.xml';

    final celdas = StringBuffer();
    var renglon = 0;
    for (final fila in _filasConTitulo(tabla, titulo)) {
      renglon++;
      celdas.write('<row r="$renglon">');
      for (var columna = 0; columna < fila.length; columna++) {
        if (fila[columna].isEmpty) continue; // una celda vacia no se escribe
        final donde = '${_letraDeColumna(columna)}$renglon';
        celdas.write('<c r="$donde" t="inlineStr"><is><t xml:space="preserve">'
            '${_xml(fila[columna])}</t></is></c>');
      }
      celdas.write('</row>');
    }

    entradas.add(EntradaZip.texto(
        ruta,
        '<?xml version="1.0" encoding="UTF-8"?>'
        '<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/'
        '2006/main"><sheetData>$celdas</sheetData></worksheet>'));

    tipos.write('<Override PartName="/$ruta" ContentType="application/'
        'vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>');
    indiceHojas.write('<sheet name="${_xml(_nombreDeHoja(tabla.nombre))}" '
        'sheetId="$numero" r:id="rId$numero"/>');
    enlaces.write('<Relationship Id="rId$numero" Type="http://schemas.'
        'openxmlformats.org/officeDocument/2006/relationships/worksheet" '
        'Target="hojas/hoja$numero.xml"/>');
  }

  tipos.write('</Types>');
  enlaces.write('</Relationships>');

  return armarZip([
    EntradaZip.texto('[Content_Types].xml', tipos.toString()),
    EntradaZip.texto(
        '_rels/.rels',
        '<?xml version="1.0" encoding="UTF-8"?>'
        '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/'
        'relationships"><Relationship Id="rIdLibro" Type="http://schemas.'
        'openxmlformats.org/officeDocument/2006/relationships/officeDocument" '
        'Target="xl/libro.xml"/></Relationships>'),
    EntradaZip.texto(
        'xl/libro.xml',
        '<?xml version="1.0" encoding="UTF-8"?>'
        '<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/'
        'main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/'
        'relationships"><sheets>$indiceHojas</sheets></workbook>'),
    EntradaZip.texto('xl/_rels/libro.xml.rels', enlaces.toString()),
    ...entradas,
  ], momento);
}

/// _letraDeColumna pasa 0 a «A», 25 a «Z» y 26 a «AA».
///
/// Excel nombra cada celda por letra de columna y numero de fila, y no hay
/// atajo: hay que contar en base 26 con las letras como digitos.
String _letraDeColumna(int indice) {
  var nombre = '';
  var resto = indice;
  while (resto >= 0) {
    nombre = String.fromCharCode(65 + resto % 26) + nombre;
    resto = resto ~/ 26 - 1;
  }
  return nombre;
}

// ------------------------------------------------------------------ apoyo ---

/// _filasConTitulo pone arriba de la hoja de que es y de cuando.
///
/// Es la regla del proyecto: todo reporte que salga de MiRed dice de que momento
/// son los datos. Va en su propio renglon y con uno en blanco detras, para que
/// la hoja de calculo siga viendo los encabezados como una fila y no como parte
/// del titulo.
List<List<String>> _filasConTitulo(Tabla tabla, String titulo) => [
      if (titulo.isNotEmpty) ...[
        [titulo],
        const <String>[],
      ],
      if (tabla.explicacion.isNotEmpty) ...[
        [tabla.explicacion],
        const <String>[],
      ],
      tabla.encabezados,
      ...tabla.filas,
    ];

int _cuantasColumnas(Tabla tabla) {
  var ancho = tabla.encabezados.length;
  for (final fila in tabla.filas) {
    if (fila.length > ancho) ancho = fila.length;
  }
  return ancho < 1 ? 1 : ancho;
}

/// _nombreDeHoja deja el nombre de la pestana como Excel lo admite.
///
/// Excel no abre el archivo si una hoja pasa de 31 caracteres o lleva alguno de
/// `[]:*?/\`. Es un limite suyo, no del contenido, asi que se recorta aqui en
/// vez de andar cuidandolo en cada tabla.
String _nombreDeHoja(String nombre) {
  final limpio = nombre.replaceAll(RegExp(r'[\[\]:*?/\\]'), ' ').trim();
  if (limpio.isEmpty) return 'Hoja';
  return limpio.length <= 31 ? limpio : limpio.substring(0, 31);
}

String _xml(String texto) => texto
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;');
