library;

/// descarga entrega un archivo al navegador y abre enlaces externos.
///
/// La interfaz de MiRed se compila a web, pero el mismo codigo compila tambien
/// a escritorio y a Android, y las pruebas corren en la maquina virtual de
/// Dart. Bajar un archivo es cosa del navegador, asi que la implementacion de
/// verdad vive en descarga_web.dart y solo se enlaza cuando el objetivo es web.
/// En cualquier otro objetivo entra descarga_generica.dart, que lo dice en vez
/// de fallar en silencio.
export 'descarga_generica.dart' if (dart.library.js_interop) 'descarga_web.dart';
