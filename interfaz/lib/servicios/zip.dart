import 'dart:convert';
import 'dart:typed_data';

/// Este archivo arma un ZIP a mano, sin biblioteca de terceros.
///
/// Existe porque un ODS y un XLSX **son un ZIP con XML adentro**: sin poder
/// escribir el ZIP no hay hoja de calculo de verdad, solo CSV. Es la misma
/// decision que ya se tomo con el PDF —escribirlo byte por byte en vez de
/// arrastrar una dependencia y su licencia— y aqui sale todavia mas barata,
/// porque los archivos se guardan **sin comprimir**.
///
/// Sin comprimir (metodo 0, «guardado») es legal y todo lector lo acepta. Un
/// mapa de red son unas decenas de kilobytes de texto: comprimirlos ahorraria
/// unos pocos KB a cambio de traerse un compresor entero.

/// EntradaZip es un archivo dentro del paquete.
class EntradaZip {
  const EntradaZip(this.nombre, this.datos);

  /// La ruta dentro del ZIP, con barras normales: `xl/worksheets/hoja1.xml`.
  final String nombre;
  final Uint8List datos;

  /// Atajo para las entradas de texto, que son todas menos ninguna aqui.
  factory EntradaZip.texto(String nombre, String contenido) =>
      EntradaZip(nombre, Uint8List.fromList(utf8.encode(contenido)));
}

/// armarZip pega las entradas en un paquete ZIP.
///
/// [fecha] va en cada entrada como fecha de modificacion. Se recibe en vez de
/// preguntarla al reloj para que dos exportaciones del mismo mapa den el mismo
/// archivo byte por byte: asi las pruebas pueden comparar el resultado.
Uint8List armarZip(List<EntradaZip> entradas, DateTime fecha) {
  final salida = BytesBuilder();
  final directorio = BytesBuilder();
  final hora = _horaDos(fecha);
  final dia = _fechaDos(fecha);

  for (final entrada in entradas) {
    final nombre = Uint8List.fromList(utf8.encode(entrada.nombre));
    final crc = _crc32(entrada.datos);
    final donde = salida.length;

    // Cabecera local, delante de los datos.
    salida
      ..add(_de4(0x04034b50))
      ..add(_de2(20)) // version necesaria para abrirlo
      ..add(_de2(0)) // sin banderas: los nombres son ASCII
      ..add(_de2(0)) // metodo 0: guardado, sin comprimir
      ..add(_de2(hora))
      ..add(_de2(dia))
      ..add(_de4(crc))
      ..add(_de4(entrada.datos.length))
      ..add(_de4(entrada.datos.length))
      ..add(_de2(nombre.length))
      ..add(_de2(0)) // sin campo extra
      ..add(nombre)
      ..add(entrada.datos);

    // La misma ficha, otra vez, en el indice del final. El ZIP se lee de atras
    // hacia adelante: sin este indice el archivo no existe para un lector.
    directorio
      ..add(_de4(0x02014b50))
      ..add(_de2(20)) // version del que lo escribio
      ..add(_de2(20))
      ..add(_de2(0))
      ..add(_de2(0))
      ..add(_de2(hora))
      ..add(_de2(dia))
      ..add(_de4(crc))
      ..add(_de4(entrada.datos.length))
      ..add(_de4(entrada.datos.length))
      ..add(_de2(nombre.length))
      ..add(_de2(0)) // extra
      ..add(_de2(0)) // comentario
      ..add(_de2(0)) // en que disco empieza; solo hay uno
      ..add(_de2(0)) // atributos internos
      ..add(_de4(0)) // atributos externos
      ..add(_de4(donde)) // donde empieza su cabecera local
      ..add(nombre);
  }

  final inicioDirectorio = salida.length;
  final indice = directorio.toBytes();
  salida
    ..add(indice)
    ..add(_de4(0x06054b50))
    ..add(_de2(0)) // numero de disco
    ..add(_de2(0)) // disco donde empieza el indice
    ..add(_de2(entradas.length))
    ..add(_de2(entradas.length))
    ..add(_de4(indice.length))
    ..add(_de4(inicioDirectorio))
    ..add(_de2(0)); // sin comentario

  return salida.toBytes();
}

/// _horaDos y _fechaDos empaquetan la fecha como la guardaba el MS-DOS de 1980,
/// que es lo que el formato ZIP pide hasta hoy.
///
/// Los segundos van a la mitad porque solo hay cinco bits para ellos: el formato
/// guarda la hora con precision de dos segundos. No es un error de redondeo, es
/// el formato.
int _horaDos(DateTime fecha) =>
    (fecha.hour << 11) | (fecha.minute << 5) | (fecha.second ~/ 2);

int _fechaDos(DateTime fecha) {
  // Antes de 1980 no hay como escribirlo; se sujeta al primer dia valido en vez
  // de escribir un numero negativo que dejaria el archivo ilegible.
  final ano = fecha.year < 1980 ? 1980 : fecha.year;
  return ((ano - 1980) << 9) | (fecha.month << 5) | fecha.day;
}

Uint8List _de2(int valor) =>
    Uint8List.fromList([valor & 0xFF, (valor >> 8) & 0xFF]);

Uint8List _de4(int valor) => Uint8List.fromList([
      valor & 0xFF,
      (valor >> 8) & 0xFF,
      (valor >> 16) & 0xFF,
      (valor >> 24) & 0xFF,
    ]);

/// La tabla del CRC-32, armada una sola vez.
///
/// El ZIP guarda de cada archivo una suma de comprobacion; si no cuadra, el
/// lector da el paquete por corrupto y no lo abre. Es el mismo CRC-32 de
/// siempre, con el polinomio invertido 0xEDB88320.
final List<int> _tabla = _armarTabla();

List<int> _armarTabla() {
  final tabla = List<int>.filled(256, 0);
  for (var i = 0; i < 256; i++) {
    var valor = i;
    for (var vuelta = 0; vuelta < 8; vuelta++) {
      valor = (valor & 1) == 1 ? (valor >> 1) ^ 0xEDB88320 : valor >> 1;
    }
    tabla[i] = valor;
  }
  return tabla;
}

int _crc32(Uint8List datos) {
  var crc = 0xFFFFFFFF;
  for (final byte in datos) {
    crc = _tabla[(crc ^ byte) & 0xFF] ^ (crc >> 8);
  }
  return (crc ^ 0xFFFFFFFF) & 0xFFFFFFFF;
}
