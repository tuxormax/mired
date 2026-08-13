import 'dart:io';
import 'dart:typed_data';

/// Version para el programa de escritorio.
///
/// En web el navegador se encarga de todo: pide el archivo y lo deja en la
/// carpeta de descargas del usuario. En escritorio no hay navegador que lo haga,
/// asi que hay que escribirlo a mano — y sobre todo, **hay que decir donde
/// quedo**: un archivo que se guarda en un sitio que el usuario no sabe cual es
/// no sirve de nada.

/// descargarArchivo escribe el archivo y devuelve donde quedo.
Future<String> descargarArchivo(String nombre, String tipoMime, Uint8List datos) async {
  final carpeta = await _carpetaDeDescargas();
  final destino = File('${carpeta.path}/${_sinChocar(carpeta, nombre)}');
  await destino.writeAsBytes(datos);
  return destino.path;
}

/// _carpetaDeDescargas busca donde guarda las descargas este equipo.
///
/// Se respeta lo que el usuario tenga configurado en su escritorio antes que
/// suponer un nombre: en un sistema en espanol la carpeta se llama "Descargas" y
/// en uno en ingles "Downloads", y dar por hecho uno de los dos crearia una
/// carpeta duplicada al lado de la de verdad.
Future<Directory> _carpetaDeDescargas() async {
  final casa = Platform.environment['HOME'];
  if (casa == null || casa.isEmpty) return Directory.systemTemp;

  // Lo que dice el propio escritorio, si lo dice.
  final configuracion = File('$casa/.config/user-dirs.dirs');
  if (configuracion.existsSync()) {
    for (final renglon in await configuracion.readAsLines()) {
      if (!renglon.startsWith('XDG_DOWNLOAD_DIR')) continue;
      final entrecomillado = RegExp(r'"(.*)"').firstMatch(renglon);
      if (entrecomillado == null) continue;
      final ruta = entrecomillado.group(1)!.replaceFirst(r'$HOME', casa);
      final carpeta = Directory(ruta);
      if (carpeta.existsSync()) return carpeta;
    }
  }

  for (final nombre in ['Descargas', 'Downloads']) {
    final carpeta = Directory('$casa/$nombre');
    if (carpeta.existsSync()) return carpeta;
  }
  return Directory(casa);
}

/// _sinChocar le pone un numero al nombre si ya hay un archivo asi.
///
/// Exportar el mapa dos veces el mismo minuto no debe pisar el primero sin
/// avisar: el segundo sale como "mapa (2).pdf", que es lo que hace cualquier
/// navegador y lo que la gente espera.
String _sinChocar(Directory carpeta, String nombre) {
  if (!File('${carpeta.path}/$nombre').existsSync()) return nombre;

  final punto = nombre.lastIndexOf('.');
  final base = punto > 0 ? nombre.substring(0, punto) : nombre;
  final extension = punto > 0 ? nombre.substring(punto) : '';

  for (var numero = 2; numero < 1000; numero++) {
    final intento = '$base ($numero)$extension';
    if (!File('${carpeta.path}/$intento').existsSync()) return intento;
  }
  return nombre;
}

/// abrirEnlace abre una direccion en el navegador del sistema.
void abrirEnlace(String direccion) {
  // xdg-open es el estandar de escritorio en Linux: respeta el navegador que el
  // usuario haya elegido en vez de imponer uno.
  Process.run('xdg-open', [direccion]).ignore();
}
