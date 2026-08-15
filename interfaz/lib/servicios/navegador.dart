import 'dart:io';

/// Abrir una direccion en el navegador del sistema.
///
/// Se hace con el abridor del propio escritorio y no con una dependencia: es una
/// llamada por sistema operativo, y meter un paquete de terceros —con su cadena
/// de dependencias y su mantenimiento— para lanzar un comando seria pagar caro
/// algo que cabe en veinte lineas.
///
/// Devuelve si se pudo abrir. Un equipo sin escritorio no tiene con que abrirlo,
/// y quien llame tiene que poder ofrecer otra cosa —copiar la direccion— en vez
/// de quedarse callado.
Future<bool> abrirEnElNavegador(String direccion) async {
  if (direccion.isEmpty) return false;

  // Solo se abre lo que de verdad es una direccion web. Sin esto, un texto
  // cualquiera acabaria pasado a un comando del sistema.
  final url = Uri.tryParse(direccion);
  if (url == null || (url.scheme != 'http' && url.scheme != 'https')) return false;

  final (comando, argumentos) = switch (Platform.operatingSystem) {
    'linux' => ('xdg-open', [direccion]),
    'macos' => ('open', [direccion]),
    'windows' => ('cmd', ['/c', 'start', '', direccion]),
    _ => ('', <String>[]),
  };
  if (comando.isEmpty) return false;

  try {
    final resultado = await Process.run(comando, argumentos);
    return resultado.exitCode == 0;
  } on ProcessException {
    return false;
  }
}
