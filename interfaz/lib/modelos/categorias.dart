import 'package:flutter/material.dart';

/// La LISTA UNICA de lo que puede componer una red.
///
/// **Este archivo viaja en paralelo con `internal/catalogo/categorias.go`.** Hay
/// una prueba en Go que compara los dos y falla si se separan: es la unica forma
/// de que no se desincronicen en silencio, que es exactamente como se ensucian
/// los datos.
///
/// La usan dos sitios y por eso tiene que ser una sola:
///  - el desplegable de «agregar un aparato a mano», que **nunca** es texto
///    libre;
///  - el contador de la pantalla de equipos, que agrupa por esto.
///
/// Si el que se agrega a mano y el que se descubre no hablan el mismo idioma, el
/// contador muestra «AP: 1» y «Punto de acceso: 2» como si fueran cosas
/// distintas. Ya paso una vez.
class CategoriaEquipo {
  const CategoriaEquipo({
    required this.clave,
    required this.singular,
    required this.plural,
    required this.icono,
    required this.automatica,
    required this.explicacion,
    this.declaraPuertos = false,
    this.preguntaConexion = false,
  });

  /// Lo que se guarda en la base. No cambia nunca.
  final String clave;

  /// Lo que lee una persona. El plural NO se calcula pegando una «s»:
  /// «switch» da «switches» y «punto de acceso» da «puntos de acceso».
  final String singular;
  final String plural;
  final IconData icono;

  /// Si un escaneo puede reconocerla solo.
  ///
  /// Lo que es `false` **solo puede llegar declarado a mano**: un switch no
  /// administrable no tiene direccion, no contesta a nada y ningun barrido lo va
  /// a ver jamas.
  final bool automatica;

  final String explicacion;

  /// Si al declararlo a mano tiene sentido preguntarle cuantos puertos tiene.
  /// Un switch si; un celular no.
  final bool declaraPuertos;

  /// Si tiene sentido preguntar si esta por cable o por WiFi. Solo aplica a
  /// equipos terminales: en un switch no significaria nada.
  final bool preguntaConexion;

  /// Como se dice de N aparatos de esta categoria.
  String cuantos(int cuantos) => cuantos == 1 ? singular : plural;
}

/// La lista, en el orden en que conviene leerla: primero lo que arma la red,
/// despues lo que cuelga de ella.
const List<CategoriaEquipo> categoriasDeEquipo = [
  CategoriaEquipo(
    clave: 'gateway',
    singular: 'Modem o gateway',
    plural: 'Modems y gateways',
    icono: Icons.router,
    automatica: true,
    explicacion: 'La caja del proveedor de internet, la que trae el cable de la calle.',
    declaraPuertos: true,
  ),
  CategoriaEquipo(
    clave: 'router',
    singular: 'Router',
    plural: 'Routers',
    icono: Icons.router,
    automatica: true,
    explicacion:
        'Reparte entre redes distintas. En una casa suele ser el mismo aparato que el modem.',
    declaraPuertos: true,
  ),
  CategoriaEquipo(
    clave: 'switch_administrable',
    singular: 'Switch administrable',
    plural: 'Switches administrables',
    icono: Icons.settings_ethernet,
    automatica: true,
    explicacion: 'Contesta por SNMP y puede decir que hay enchufado en cada puerto.',
    declaraPuertos: true,
  ),
  CategoriaEquipo(
    clave: 'switch_simple',
    singular: 'Switch no administrable',
    plural: 'Switches no administrables',
    icono: Icons.settings_ethernet,
    automatica: false,
    explicacion: 'El switch de toda la vida: reparte y calla. No tiene direccion, '
        'asi que ningun escaneo lo ve nunca y solo puede declararse a mano.',
    declaraPuertos: true,
  ),
  CategoriaEquipo(
    clave: 'punto_de_acceso',
    singular: 'Punto de acceso WiFi',
    plural: 'Puntos de acceso WiFi',
    icono: Icons.wifi,
    automatica: true,
    explicacion: 'La antena de la que cuelga el WiFi.',
  ),
  CategoriaEquipo(
    clave: 'computadora',
    singular: 'PC o laptop',
    plural: 'PC y laptops',
    icono: Icons.computer,
    automatica: true,
    explicacion: 'Equipos de trabajo, por cable o por WiFi.',
    preguntaConexion: true,
  ),
  CategoriaEquipo(
    clave: 'telefono',
    singular: 'Telefono o celular',
    plural: 'Telefonos y celulares',
    icono: Icons.smartphone,
    automatica: true,
    explicacion: 'Celulares, tabletas y telefonos IP.',
    preguntaConexion: true,
  ),
  CategoriaEquipo(
    clave: 'impresora',
    singular: 'Impresora',
    plural: 'Impresoras',
    icono: Icons.print,
    automatica: true,
    explicacion: 'Cualquier cosa que imprima por red.',
    preguntaConexion: true,
  ),
  CategoriaEquipo(
    clave: 'camara',
    singular: 'Camara o grabador',
    plural: 'Camaras y grabadores',
    icono: Icons.videocam,
    automatica: true,
    explicacion: 'Camaras IP y los grabadores DVR o NVR de los que cuelgan.',
    declaraPuertos: true,
  ),
  CategoriaEquipo(
    clave: 'servidor',
    singular: 'Servidor',
    plural: 'Servidores',
    icono: Icons.dns,
    automatica: true,
    explicacion: 'Equipos que dan un servicio: web, base de datos, virtualizacion.',
  ),
  CategoriaEquipo(
    clave: 'almacenamiento',
    singular: 'Almacenamiento en red',
    plural: 'Almacenamientos en red',
    icono: Icons.storage,
    automatica: true,
    explicacion: 'NAS y discos de red.',
  ),
  CategoriaEquipo(
    clave: 'multimedia',
    singular: 'TV o multimedia',
    plural: 'TV y multimedia',
    icono: Icons.cast,
    automatica: true,
    explicacion: 'Televisiones, Chromecast, consolas y bocinas.',
    preguntaConexion: true,
  ),
  CategoriaEquipo(
    clave: 'otro',
    singular: 'Otro aparato',
    plural: 'Otros aparatos',
    icono: Icons.devices_other,
    automatica: true,
    explicacion: 'Lo que no encaja en ninguna de las de arriba pero se sabe que es.',
    preguntaConexion: true,
  ),
  CategoriaEquipo(
    clave: 'sin_reconocer',
    singular: 'Sin reconocer',
    plural: 'Sin reconocer',
    icono: Icons.help_outline,
    automatica: true,
    explicacion: 'Se vio en la red pero nadie supo que es. Aqui es donde el catalogo '
        'necesita crecer, y desde la ficha se puede proponer su definicion.',
  ),
];

/// Donde caen los equipos que nadie identifico.
///
/// **No se esconden ni se reparten entre las demas.** Un inventario que calla lo
/// que no supo clasificar se lee como si estuviera completo, y justamente ahi es
/// donde hay que mirar: un aparato desconocido en la red es lo primero que
/// interesa de un mapa.
const String categoriaSinReconocer = 'sin_reconocer';

/// Las que se pueden elegir al declarar un aparato a mano: todas menos «sin
/// reconocer», que es un resultado, no algo que alguien escoja.
List<CategoriaEquipo> get categoriasParaDeclarar =>
    categoriasDeEquipo.where((c) => c.clave != categoriaSinReconocer).toList();

/// buscarCategoria devuelve la categoria de una clave, o null si no se conoce.
CategoriaEquipo? buscarCategoria(String? clave) {
  if (clave == null || clave.isEmpty) return null;
  for (final categoria in categoriasDeEquipo) {
    if (categoria.clave == clave) return categoria;
  }
  return null;
}

/// nombreDeCategoria devuelve como se lee una clave. Nunca devuelve vacio: un
/// renglon en blanco no dice si es que no hay dato o si el programa fallo.
String nombreDeCategoria(String? clave) =>
    buscarCategoria(clave)?.singular ?? 'Sin reconocer';

/// iconoDeCategoria devuelve el icono que le toca a una clave.
IconData iconoDeCategoria(String? clave) =>
    buscarCategoria(clave)?.icono ?? Icons.help_outline;
