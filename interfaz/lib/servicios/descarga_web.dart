import 'dart:js_interop';
import 'dart:typed_data';

import 'package:web/web.dart' as navegador;

/// Version web: la unica que de verdad baja archivos.
///
/// El navegador no deja escribir en el disco, asi que el camino es siempre el
/// mismo: se arma un Blob con los bytes en memoria, se le pide una URL
/// temporal, se fabrica un enlace invisible con el atributo download y se le da
/// click. Al terminar hay que **soltar la URL**: si no, los bytes del PNG se
/// quedan en memoria hasta que se recargue la pagina, y un mapa grande son
/// varios megas por cada exportacion.

/// descargarArchivo entrega el archivo al navegador.
///
/// Devuelve cadena vacia: en web no hay una ruta que decir, porque cada
/// navegador guarda donde tenga configurado y ni siquiera puede consultarse.
Future<String> descargarArchivo(String nombre, String tipoMime, Uint8List datos) async {
  final partes = <JSAny>[datos.toJS].toJS;
  final blob = navegador.Blob(partes, navegador.BlobPropertyBag(type: tipoMime));
  final direccion = navegador.URL.createObjectURL(blob);

  final enlace = navegador.document.createElement('a') as navegador.HTMLAnchorElement
    ..href = direccion
    ..download = nombre;
  navegador.document.body?.appendChild(enlace);
  enlace.click();
  enlace.remove();

  navegador.URL.revokeObjectURL(direccion);
  return '';
}

/// abrirEnlace abre una direccion en otra pestana.
///
/// noopener no es adorno: sin el, la pagina que se abre puede manipular la de
/// MiRed a traves de window.opener.
void abrirEnlace(String direccion) {
  navegador.window.open(direccion, '_blank', 'noopener,noreferrer');
}
