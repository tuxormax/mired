library;

/// descarga guarda un archivo y abre enlaces, en web y en escritorio.
///
/// Son dos cosas distintas de verdad, no una con dos nombres: en web se le
/// entrega el archivo al navegador y es el quien decide donde ponerlo; en
/// escritorio hay que escribirlo y **decir donde quedo**. Por eso cada objetivo
/// tiene su archivo y `descargarArchivo` devuelve la ruta cuando la hay.
export 'descarga_generica.dart' if (dart.library.js_interop) 'descarga_web.dart';
