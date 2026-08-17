library;

/// descarga pregunta donde guardar un archivo y abre enlaces en el navegador.
///
/// Antes esto era una exportacion condicional, porque la interfaz se compilaba
/// tambien a web y ahi guardar un archivo es cosa del navegador. **MiRed ya no
/// tiene interfaz web**: es un programa de escritorio, asi que hay una sola
/// forma de hacerlo y no hace falta elegir.
export 'descarga_generica.dart';
