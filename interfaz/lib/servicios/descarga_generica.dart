import 'dart:typed_data';

/// Version para todo objetivo que no sea web: escritorio, Android y las
/// pruebas.
///
/// No se queda callada. Un archivo que el usuario pidio y nunca aparece es peor
/// que un mensaje que explique por que: aqui se lanza la excepcion y la pantalla
/// la muestra con el modal de errores de la casa.
///
/// El dia que MiRed se compile a escritorio, este archivo es donde va el guardar
/// con el selector de archivos del sistema.
Future<void> descargarArchivo(String nombre, String tipoMime, Uint8List datos) {
  throw UnsupportedError(
    'Descargar archivos solo funciona en la interfaz web de MiRed.',
  );
}

/// abrirEnlace abre una direccion en otra pestana.
void abrirEnlace(String direccion) {
  throw UnsupportedError(
    'Abrir enlaces solo funciona en la interfaz web de MiRed.',
  );
}
