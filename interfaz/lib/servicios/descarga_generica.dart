import 'dart:io';
import 'dart:typed_data';

import 'package:file_selector/file_selector.dart';

/// Version para el programa de escritorio.
///
/// MiRed es un programa de escritorio, asi que guardar un archivo se hace como
/// en cualquier otro programa del sistema: **preguntando al usuario donde**, con
/// el cuadro de guardar del propio escritorio. Antes se escribia derecho en la
/// carpeta de descargas y solo se avisaba de la ruta; funcionaba, pero obligaba
/// a ir a buscar el archivo y a moverlo a mano al sitio donde de verdad iba.
///
/// El cuadro es el nativo de GTK, el mismo que abre cualquier programa de Linux,
/// asi que trae gratis los marcadores del usuario, los sitios de red montados y
/// la confirmacion de sobrescritura.

/// descargarArchivo pregunta donde guardar, escribe el archivo y devuelve donde
/// quedo.
///
/// Devuelve cadena vacia si el usuario cerro el cuadro sin elegir: cancelar no
/// es un error y no debe acabar en un modal de problema.
Future<String> descargarArchivo(String nombre, String tipoMime, Uint8List datos) async {
  final elegido = await getSaveLocation(
    suggestedName: nombre,
    // Se propone la carpeta de descargas, pero solo como punto de partida: es
    // donde la gente espera encontrarse parada al abrir un cuadro de guardar.
    initialDirectory: (await _carpetaDeDescargas()).path,
    acceptedTypeGroups: [_grupoDeTipo(nombre, tipoMime)],
    confirmButtonText: 'Guardar',
  );
  if (elegido == null) return '';

  final destino = File(_conSuExtension(elegido.path, nombre));
  await destino.writeAsBytes(datos);
  return destino.path;
}

/// _grupoDeTipo arma el filtro del cuadro con el nombre del formato en cristiano.
///
/// Sin filtro, el cuadro lista todos los archivos de la carpeta y el usuario no
/// ve cual de los suyos va a pisar. La etiqueta se escribe en espanol porque es
/// lo unico de ese cuadro que decide MiRed; el resto lo pone el escritorio.
XTypeGroup _grupoDeTipo(String nombre, String tipoMime) {
  final punto = nombre.lastIndexOf('.');
  final extension = punto > 0 ? nombre.substring(punto + 1).toLowerCase() : '';
  const nombres = {
    'png': 'Imagen PNG',
    'svg': 'Dibujo vectorial SVG',
    'pdf': 'Documento PDF',
    'ods': 'Hoja de calculo de LibreOffice',
    'xlsx': 'Hoja de calculo de Excel',
    'csv': 'Tabla de texto CSV',
  };
  return XTypeGroup(
    label: nombres[extension] ?? 'Archivo ${extension.toUpperCase()}',
    extensions: extension.isEmpty ? null : [extension],
    // El tipo MIME va sin los parametros de detras: "text/csv;charset=utf-8" no
    // es un tipo que GTK reconozca, "text/csv" si.
    mimeTypes: [tipoMime.split(';').first],
  );
}

/// _conSuExtension le devuelve la extension al nombre si el usuario la quito.
///
/// El cuadro de GTK no la pone solo: quien teclee "mapa de la oficina" acaba con
/// un archivo sin extension que su escritorio ya no sabe abrir. Si el usuario la
/// escribio, se respeta la suya, aunque no sea la propuesta.
String _conSuExtension(String rutaElegida, String nombrePropuesto) {
  final punto = nombrePropuesto.lastIndexOf('.');
  if (punto <= 0) return rutaElegida;
  final extension = nombrePropuesto.substring(punto);

  final ultimaBarra = rutaElegida.lastIndexOf('/');
  final soloNombre = rutaElegida.substring(ultimaBarra + 1);
  if (soloNombre.contains('.')) return rutaElegida;

  return '$rutaElegida$extension';
}

/// _carpetaDeDescargas busca donde guarda las descargas este equipo.
///
/// Se respeta lo que el usuario tenga configurado en su escritorio antes que
/// suponer un nombre: en un sistema en espanol la carpeta se llama "Descargas" y
/// en uno en ingles "Downloads", y dar por hecho uno de los dos abriria el
/// cuadro en una carpeta que no existe.
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

/// abrirEnlace abre una direccion en el navegador del sistema.
void abrirEnlace(String direccion) {
  // xdg-open es el estandar de escritorio en Linux: respeta el navegador que el
  // usuario haya elegido en vez de imponer uno.
  Process.run('xdg-open', [direccion]).ignore();
}
