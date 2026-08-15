import 'dart:math' as matematicas;

import 'package:flutter/material.dart';

import '../modelos/modelos.dart';

/// Este archivo tiene el **plano** del mapa: donde va cada caja y cada linea, y
/// como se pinta sobre un lienzo.
///
/// Vive aparte de la pantalla porque lo usan dos: la pantalla, para dibujarlo, y
/// la exportacion, para volcarlo a PNG, SVG y PDF. Si el calculo estuviera
/// dentro de la pantalla, el archivo exportado tendria que recalcular las
/// posiciones por su cuenta y acabaria distinto de lo que se ve.

/// DatosMapa junta lo que hace falta para armar el plano.
class DatosMapa {
  const DatosMapa({
    required this.mapa,
    required this.equipos,
    this.topologia = const TopologiaManual(),
  });

  final MapaPuertos mapa;
  final List<Equipo> equipos;

  /// Lo declarado a mano: los puertos que alguien conto mirando el aparato y los
  /// cables que declaro. Es la tercera fuente del mapa, junto al enlace
  /// confirmado por SNMP y al inferido por conteo de MAC.
  final TopologiaManual topologia;

  Equipo? equipoPorId(int? id) {
    if (id == null) return null;
    for (final equipo in equipos) {
      if (equipo.id == id) return equipo;
    }
    return null;
  }

  /// Los equipos que tienen puertos declarados: se dibujan como cabecera de su
  /// propio bloque, igual que un switch administrable.
  List<Equipo> get conPuertosDeclarados {
    final ids = topologia.puertos.map((puerto) => puerto.equipoId).toSet();
    final lista = equipos.where((equipo) => ids.contains(equipo.id)).toList();
    lista.sort((uno, otro) => uno.comoSeLlama.compareTo(otro.comoSeLlama));
    return lista;
  }

  /// Los equipos a los que tiene sentido tirar un cable desde este puerto: los
  /// que no cuelgan de ningun sitio, mas los que tienen algun puerto declarado
  /// libre —un switch al que todavia le queda lugar—. Nunca el propio aparato
  /// del que sale el cable.
  ///
  /// Sin la segunda mitad no se podia conectar un switch con otro sin inventar
  /// aparatos: los que ya tenian puertos declarados desaparecian de la lista.
  List<Equipo> conectablesDesde(int puertoOrigenId) {
    final duenoDelOrigen = topologia.equipoDelPuerto(puertoOrigenId);
    final ids = <int>{};
    final lista = <Equipo>[];

    for (final equipo in [...sinUbicar, ...conPuertosDeclarados]) {
      if (equipo.id == duenoDelOrigen || !ids.add(equipo.id)) continue;
      final tienePuertos = topologia.puertosDe(equipo.id).isNotEmpty;
      if (tienePuertos && topologia.puertosLibresDe(equipo.id).isEmpty) continue;
      lista.add(equipo);
    }
    lista.sort((uno, otro) => uno.comoSeLlama.compareTo(otro.comoSeLlama));
    return lista;
  }

  /// Los equipos que ningun switch reporto en ningun puerto y que tampoco
  /// aparecen en lo declarado a mano. Se muestran aparte: esconderlos daria un
  /// plano incompleto sin avisar.
  List<Equipo> get sinUbicar {
    final ubicados = mapa.puertos
        .where((puerto) => puerto.equipoId != null)
        .map((puerto) => puerto.equipoId)
        .toSet();
    final switches = mapa.puertos.map((puerto) => puerto.switchId).toSet();
    // Lo declarado tambien ubica: un equipo colgado a mano del puerto 3 del
    // modem ya no esta "sin ubicar", aunque ningun switch lo haya reportado.
    final declarados = topologia.puertos.map((puerto) => puerto.equipoId).toSet();
    for (final enlace in topologia.enlaces) {
      if (enlace.equipoDestinoId != null) declarados.add(enlace.equipoDestinoId!);
    }
    // Y lo que cuelga de una antena por el aire tambien esta ubicado: sin esto,
    // los telefonos seguirian flotando abajo aunque se sepa de que antena
    // cuelgan.
    for (final enlace in topologia.inalambricos) {
      declarados.add(enlace.equipoId);
      declarados.add(enlace.antenaId);
    }

    return equipos
        .where((equipo) =>
            !ubicados.contains(equipo.id) &&
            !switches.contains(equipo.id) &&
            !declarados.contains(equipo.id))
        .toList();
  }
}

class CajaPlano {
  CajaPlano({
    required this.rectangulo,
    required this.titulo,
    required this.subtitulo,
    required this.color,
    required this.icono,
    this.equipoId,
    this.puertoFisicoId,
    this.enlaceId,
    this.enlaceInalambricoId,
    this.puertoLibre = false,
    this.declarada = false,
  });

  final Rect rectangulo;
  final String titulo;
  final String subtitulo;
  final Color color;
  final IconData icono;

  /// A que equipo corresponde la caja, cuando se sabe. Lo usa el modo edicion
  /// para saber en que se hizo clic; el plano exportado lo ignora.
  final int? equipoId;

  /// El puerto declarado que representa esta caja, si es un puerto.
  final int? puertoFisicoId;

  /// El cable declarado que la ocupa, si hay alguno.
  final int? enlaceId;

  /// El enlace INALAMBRICO que representa, si cuelga por el aire. Va aparte del
  /// cable a proposito: quitar un cable y descolgar un equipo del WiFi son dos
  /// cosas distintas, y confundirlas borraria la que no es.
  final int? enlaceInalambricoId;

  /// Un puerto declarado sin nada conectado. Es donde el modo edicion ofrece
  /// conectar algo.
  final bool puertoLibre;

  /// La caja sale de lo que tecleo una persona, no de lo que se midio. Se dibuja
  /// con borde punteado para que no se confunda con lo confirmado.
  final bool declarada;
}

class LineaPlano {
  LineaPlano({
    required this.desde,
    required this.hasta,
    required this.confirmada,
    required this.etiqueta,
    this.declarada = false,
    this.inalambrica = false,
    this.color,
  });

  final Offset desde;
  final Offset hasta;
  final bool confirmada;
  final String etiqueta;

  /// El cable lo declaro una persona. Se dibuja distinto del confirmado y del
  /// inferido: son tres cosas distintas y presentarlas igual seria hacer pasar
  /// lo tecleado por medido.
  final bool declarada;

  /// No es un cable: es WiFi. Se dibuja con puntos finos, distinto de los tres
  /// anteriores, porque tampoco es lo mismo: por el aire no hay puerto que
  /// senalar, y el mapa no puede sugerir que lo haya.
  final bool inalambrica;

  /// Color de ESTE enlace. Cada uno lleva el suyo, y su etiqueta va del mismo
  /// color: en un switch de ocho puertos, ocho lineas grises que salen del mismo
  /// sitio son ocho lineas que no se pueden seguir con la vista.
  final Color? color;
}

/// EnlacePlano es un cable de switch a switch, dibujado como arco por encima de
/// la fila de switches.
///
/// Va en arco y no en linea recta porque los switches estan todos en la misma
/// fila: una recta entre el primero y el tercero cruzaria por encima del
/// segundo y se leeria como si lo tocara.
class EnlacePlano {
  EnlacePlano({
    required this.desde,
    required this.hasta,
    required this.cima,
    required this.etiqueta,
    required this.porAmbos,
  });

  final Offset desde;
  final Offset hasta;

  /// Punto de control del arco: por donde tira la curva hacia arriba.
  final Offset cima;
  final String etiqueta;

  /// Lo vieron LLDP y CDP. Es el dato mas firme que hay de un cable, y se dibuja
  /// mas grueso para que se note a simple vista.
  final bool porAmbos;
}

class Plano {
  Plano({
    required this.cajas,
    required this.lineas,
    required this.enlaces,
    required this.tamano,
    required this.colorLinea,
    required this.colorTexto,
    required this.colorFondo,
    required this.colorEnlace,
  });

  final List<CajaPlano> cajas;
  final List<LineaPlano> lineas;
  final List<EnlacePlano> enlaces;
  final Size tamano;
  final Color colorLinea;
  final Color colorTexto;
  final Color colorFondo;
  final Color colorEnlace;
}

const double anchoCaja = 190;
const double altoCaja = 54;

/// El mapa crece hacia la DERECHA: cada nivel es una columna y los hermanos van
/// en lista hacia abajo.
///
/// Se dibujaba de arriba abajo, y en cuanto un switch tenia ocho puertos el
/// dibujo se abria en abanico: las lineas se cruzaban, las cajas se salian de la
/// pantalla y habia que buscar el aparato. Una red se lee como se cablea: la
/// entrada a la izquierda y lo que cuelga, a su derecha.

/// separacionX es el hueco entre una columna y la siguiente.
const double separacionX = 70;

/// separacionY es el hueco entre dos hermanos de la misma columna.
const double separacionY = 18;

/// paso de una columna a la siguiente, y de una fila a la siguiente.
const double anchoColumna = anchoCaja + separacionX;
const double altoFila = altoCaja + separacionY;

/// margen es el aire que queda alrededor del dibujo.
const double margen = 24;

/// Los colores con los que se distinguen los enlaces.
///
/// Cada enlace lleva el suyo y su etiqueta va igual, porque de un switch salen
/// tantas lineas como puertos tenga y todas del mismo gris no se pueden seguir
/// con la vista. Son colores elegidos para verse sobre fondo claro Y sobre fondo
/// oscuro: el mapa se mira en la pantalla —que puede estar en cualquiera de los
/// dos temas— y se exporta siempre sobre blanco.
const List<Color> coloresDeEnlace = [
  Color(0xFF4FC3F7), // azul cielo
  Color(0xFF81C784), // verde
  Color(0xFFFFB74D), // ambar
  Color(0xFFBA68C8), // morado
  Color(0xFF4DD0E1), // turquesa
  Color(0xFFF06292), // rosa
  Color(0xFFAED581), // lima
  Color(0xFFFF8A65), // terracota
  Color(0xFF9575CD), // lavanda
  Color(0xFFDCE775), // oliva
];

/// colorDeEnlace reparte la paleta. Se pide por numero de enlace, no al azar:
/// el mismo mapa tiene que salir igual dos veces seguidas.
Color colorDeEnlace(int cual) => coloresDeEnlace[cual % coloresDeEnlace.length];

/// coloresParaExportar son los que usa todo archivo exportado, sin importar si
/// la pantalla esta en claro o en oscuro.
///
/// Un mapa en tema oscuro impreso en papel sale casi negro y se lleva el toner
/// de media oficina. El archivo que se guarda o se manda por correo va siempre
/// sobre blanco.
final ColorScheme coloresParaExportar =
    ColorScheme.fromSeed(seedColor: const Color(0xFF1565C0), brightness: Brightness.light);

/// _ArbolDeclarado ordena lo que se declaro a mano como lo que es: un arbol.
///
/// El cable que alguien tecleo dice quien cuelga de quien. **El padre es el
/// aparato del que SALE el cable**, que es como lo declaro la persona: se para
/// en el modem, toca su puerto y elige el switch. Con eso alcanza para dibujar
/// cada aparato una sola vez, y para que el puerto por donde sube el cable se
/// vea ocupado en los dos extremos.
class _ArbolDeclarado {
  _ArbolDeclarado(this.datos) {
    final conPuertos = datos.conPuertosDeclarados;
    for (final equipo in conPuertos) {
      _tienePuertos.add(equipo.id);
    }

    // Una antena tambien es cabecera de bloque aunque no tenga NINGUN puerto
    // declarado: el WiFi no tiene puertos, y aun asi de ella cuelgan equipos.
    final antenas = <int>{};
    for (final enlace in datos.topologia.inalambricos) {
      antenas.add(enlace.antenaId);
    }

    for (final cable in datos.topologia.enlaces) {
      final padre = datos.topologia.equipoDelPuerto(cable.puertoOrigenId);
      final hijo = cable.puertoDestinoId != null
          ? datos.topologia.equipoDelPuerto(cable.puertoDestinoId!)
          : cable.equipoDestinoId;
      if (padre == null || hijo == null || padre == hijo) continue;
      // Es hoja solo si no es cabecera de nada: ni tiene puertos declarados ni
      // le cuelga algo por el aire. Una antena sin un solo puerto sigue siendo
      // cabecera de su bloque, y si se tratara como hoja se dibujaria dos veces.
      if (!_tienePuertos.contains(hijo) && !antenas.contains(hijo)) continue;
      if (_padreDe.containsKey(hijo)) continue; // ya cuelga de otro
      if (_desciendeDe(padre, hijo)) continue; // cerraria un circulo
      _padreDe[hijo] = padre;
    }

    // Lo que cuelga por el aire tambien hace padre a la antena. Se hace DESPUES
    // de los cables para que un aparato conectado por cable no acabe colgando
    // del WiFi: el cable es el dato mas firme de los dos.
    for (final enlace in datos.topologia.inalambricos) {
      if (enlace.equipoId == enlace.antenaId) continue;
      if (_padreDe.containsKey(enlace.equipoId)) continue;
      if (_desciendeDe(enlace.antenaId, enlace.equipoId)) continue;
      _padreDe[enlace.equipoId] = enlace.antenaId;
      _clientes.putIfAbsent(enlace.antenaId, () => []).add(enlace);
    }

    final cabeceras = <int>{..._tienePuertos, ...antenas};
    raices = datos.equipos
        .where((equipo) =>
            cabeceras.contains(equipo.id) && !_padreDe.containsKey(equipo.id))
        .toList()
      ..sort((uno, otro) => uno.comoSeLlama.compareTo(otro.comoSeLlama));
  }

  final DatosMapa datos;
  final Map<int, int> _padreDe = {};
  final Set<int> _tienePuertos = {};
  /// Lo que cuelga de cada antena por el aire.
  final Map<int, List<EnlaceInalambrico>> _clientes = {};
  final Map<int, double> _altos = {};
  late final List<Equipo> raices;

  bool _desciendeDe(int posibleHijo, int posibleAncestro) {
    var actual = _padreDe[posibleHijo];
    var vueltas = 0;
    while (actual != null && vueltas < 64) {
      if (actual == posibleAncestro) return true;
      actual = _padreDe[actual];
      vueltas++;
    }
    return false;
  }

  /// esCabecera dice si a ese aparato hay que dibujarle su propio bloque: o
  /// tiene puertos declarados, o le cuelga algo por el aire.
  bool esCabecera(int equipoId) =>
      _tienePuertos.contains(equipoId) || (_clientes[equipoId]?.isNotEmpty ?? false);

  /// El equipo que cuelga de este puerto, si es un aparato con puertos propios.
  Equipo? hijoEn(int equipoId, PuertoFisico puerto) {
    final cable = datos.topologia.enlaceDe(puerto.id);
    if (cable == null) return null;
    final otro = _otroExtremo(cable, equipoId);
    if (otro == null || _padreDe[otro] != equipoId) return null;
    return datos.equipoPorId(otro);
  }

  int? _otroExtremo(EnlaceFisico cable, int equipoId) {
    final origen = datos.topologia.equipoDelPuerto(cable.puertoOrigenId);
    if (origen == equipoId) {
      return cable.puertoDestinoId != null
          ? datos.topologia.equipoDelPuerto(cable.puertoDestinoId!)
          : cable.equipoDestinoId;
    }
    return origen;
  }

  /// Cuanto ALTO ocupa un aparato con todo lo que le cuelga.
  ///
  /// Se mide antes de colocar nada: sin esto, dos ramas se pisarian. Ahora se
  /// mide en vertical porque el arbol crece hacia la derecha, y lo que compite
  /// por sitio son las filas.
  double medir(Equipo equipo) {
    final guardado = _altos[equipo.id];
    if (guardado != null) return guardado;
    _altos[equipo.id] = altoFila; // provisional, corta circulos

    double total = 0;
    for (final puerto in datos.topologia.puertosDe(equipo.id)) {
      // El puerto por donde SUBE al aparato de la izquierda no ocupa fila: esa
      // conexion ya se ve en la linea que llega desde el padre.
      if (subePorAqui(equipo.id, puerto)) continue;
      final hijo = hijoEn(equipo.id, puerto);
      total += hijo == null ? altoFila : medir(hijo);
    }
    for (final cliente in _clientes[equipo.id] ?? const <EnlaceInalambrico>[]) {
      final hijo = datos.equipoPorId(cliente.equipoId);
      total += hijo != null && esCabecera(hijo.id) ? medir(hijo) : altoFila;
    }

    final alto = matematicas.max(total, altoFila);
    _altos[equipo.id] = alto;
    return alto;
  }

  /// subePorAqui dice si ese puerto es por el que el aparato cuelga del de la
  /// izquierda. Ese puerto esta ocupado —y hay que contarlo como tal— pero no se
  /// dibuja como una caja mas: seria repetir la misma conexion dos veces.
  bool subePorAqui(int equipoId, PuertoFisico puerto) {
    final padre = _padreDe[equipoId];
    if (padre == null) return false;
    final cable = datos.topologia.enlaceDe(puerto.id);
    if (cable == null) return false;
    return _otroExtremo(cable, equipoId) == padre;
  }

  /// colocar dibuja el aparato y, a su DERECHA, todo lo que le cuelga.
  ///
  /// `y` es donde empieza su franja; el aparato se centra en ella, y sus hijos
  /// se apilan hacia abajo en la columna siguiente. Devuelve cuanto alto ocupo.
  double colocar({
    required Equipo equipo,
    required double x,
    required double y,
    required ColorScheme colores,
    required List<CajaPlano> cajas,
    required List<LineaPlano> lineas,
    required void Function(double, double) alCrecer,
    required Color Function() siguienteColor,
  }) {
    final altoBloque = medir(equipo);
    final centroEquipo = Offset(x + anchoCaja / 2, y + altoBloque / 2);
    alCrecer(x + anchoCaja, y + altoBloque);

    cajas.add(CajaPlano(
      rectangulo: Rect.fromCenter(center: centroEquipo, width: anchoCaja, height: altoCaja),
      titulo: equipo.comoSeLlama,
      subtitulo: equipo.modelo.isNotEmpty
          ? equipo.modelo
          : (equipo.ip.isNotEmpty ? equipo.ip : 'declarado a mano'),
      color: colores.secondaryContainer,
      icono: Icons.settings_ethernet,
      equipoId: equipo.id,
      declarada: equipo.esManual,
    ));

    final xHijos = x + anchoColumna;
    double yHijo = y;

    void tirarLinea(Offset centroHijo, String etiqueta,
        {bool inalambrica = false, bool declarada = true}) {
      lineas.add(LineaPlano(
        desde: centroEquipo + const Offset(anchoCaja / 2, 0),
        hasta: centroHijo - const Offset(anchoCaja / 2, 0),
        confirmada: false,
        declarada: declarada,
        inalambrica: inalambrica,
        etiqueta: etiqueta,
        color: siguienteColor(),
      ));
    }

    for (final puerto in datos.topologia.puertosDe(equipo.id)) {
      // Por aqui sube al aparato de la izquierda: no se dibuja otra vez.
      if (subePorAqui(equipo.id, puerto)) continue;

      final hijo = hijoEn(equipo.id, puerto);
      final cable = datos.topologia.enlaceDe(puerto.id);
      final altoHijo = hijo == null ? altoFila : medir(hijo);
      final centroHijo = Offset(xHijos + anchoCaja / 2, yHijo + altoHijo / 2);
      final etiqueta = puerto.tipo == 'wan' ? 'WAN' : 'puerto ${puerto.numero}';

      if (hijo != null) {
        colocar(
          equipo: hijo,
          x: xHijos,
          y: yHijo,
          colores: colores,
          cajas: cajas,
          lineas: lineas,
          alCrecer: alCrecer,
          siguienteColor: siguienteColor,
        );
      } else if (cable == null) {
        // Un puerto libre NO se esconde: es justo donde el modo edicion ofrece
        // conectar algo, y verlo vacio dice cuanto falta por declarar.
        cajas.add(CajaPlano(
          rectangulo: Rect.fromCenter(center: centroHijo, width: anchoCaja, height: altoCaja),
          titulo: 'Puerto ${puerto.etiqueta} libre',
          subtitulo:
              puerto.velocidadMbps != null ? '${puerto.velocidadMbps} Mbps' : 'sin conectar',
          color: colores.surfaceContainerLow,
          icono: Icons.add_circle_outline,
          equipoId: equipo.id,
          puertoFisicoId: puerto.id,
          puertoLibre: true,
          declarada: true,
        ));
      } else {
        // Hay cable y el otro extremo no tiene bloque propio: es una hoja.
        final soyOrigen = cable.puertoOrigenId == puerto.id;
        final otro = _otroExtremo(cable, equipo.id);
        final nombreOtro = soyOrigen ? cable.destinoNombre : cable.origenNombre;
        cajas.add(CajaPlano(
          rectangulo: Rect.fromCenter(center: centroHijo, width: anchoCaja, height: altoCaja),
          titulo: nombreOtro.isEmpty ? 'Conectado' : nombreOtro,
          subtitulo: 'declarado a mano',
          color: colores.surfaceContainerHighest,
          icono: Icons.devices,
          equipoId: otro,
          puertoFisicoId: puerto.id,
          enlaceId: cable.id,
          declarada: true,
        ));
      }

      tirarLinea(centroHijo, etiqueta);
      alCrecer(xHijos + anchoCaja, yHijo + altoHijo);
      yHijo += altoHijo;
    }

    // Y lo que cuelga por el aire. **No ocupa puerto**: el WiFi no tiene, y
    // dibujarlo como si lo tuviera sugeriria un cable que no existe.
    for (final cliente in _clientes[equipo.id] ?? const <EnlaceInalambrico>[]) {
      final hijo = datos.equipoPorId(cliente.equipoId);
      final esBloque = hijo != null && esCabecera(hijo.id);
      final altoHijo = esBloque ? medir(hijo) : altoFila;
      final centroHijo = Offset(xHijos + anchoCaja / 2, yHijo + altoHijo / 2);

      if (esBloque) {
        colocar(
          equipo: hijo,
          x: xHijos,
          y: yHijo,
          colores: colores,
          cajas: cajas,
          lineas: lineas,
          alCrecer: alCrecer,
          siguienteColor: siguienteColor,
        );
      } else {
        cajas.add(CajaPlano(
          rectangulo: Rect.fromCenter(center: centroHijo, width: anchoCaja, height: altoCaja),
          titulo: hijo?.comoSeLlama ?? cliente.equipoNombre,
          // De donde salio se dice SIEMPRE: no es lo mismo que lo declarara una
          // persona a que lo dijera la propia antena.
          subtitulo: [
            if (cliente.red.isNotEmpty) cliente.red,
            if (cliente.senalDbm != null) '${cliente.senalDbm} dBm',
            cliente.comoSeSupo,
          ].join(' · '),
          color: colores.surfaceContainerHighest,
          icono: Icons.wifi,
          equipoId: cliente.equipoId,
          enlaceInalambricoId: cliente.id,
          declarada: cliente.esManual,
        ));
      }

      tirarLinea(centroHijo, cliente.red.isEmpty ? 'WiFi' : cliente.red,
          inalambrica: true, declarada: false);
      alCrecer(xHijos + anchoCaja, yHijo + altoHijo);
      yHijo += altoHijo;
    }

    return altoBloque;
  }
}

/// armarPlano coloca todo: una columna por switch, y debajo de cada uno sus
/// puertos con lo que cuelga. Es un arbol por niveles, que para un plano de sitio
/// se lee mucho mejor que una maraña de nodos flotando.
Plano armarPlano(DatosMapa datos, ColorScheme colores) {
  final cajas = <CajaPlano>[];
  final lineas = <LineaPlano>[];
  final enlaces = <EnlacePlano>[];

  // Hasta donde llego el dibujo. Se va apuntando segun se coloca, en vez de
  // calcularlo al final: cada rama sabe cuanto ocupo y nadie mas tiene que
  // volver a recorrerlo.
  double anchoMaximo = 0;
  double altoMaximo = 0;
  void alCrecer(double x, double y) {
    anchoMaximo = matematicas.max(anchoMaximo, x);
    altoMaximo = matematicas.max(altoMaximo, y);
  }

  double y = margen;

  // Un color por enlace, repartido segun se van colocando. Se lleva la cuenta
  // aqui —y no en cada rama— para que dos ramas distintas no repitan color
  // seguido.
  var cuantosEnlaces = 0;
  Color siguienteColor() => colorDeEnlace(cuantosEnlaces++);

  // ------------------------------------------ lo que dijeron los switches ---
  //
  // Un switch a la izquierda y, a su derecha, lo que cuelga de cada puerto, en
  // lista hacia abajo. Es el mismo reparto que el resto del mapa: una red se lee
  // como se cablea.
  final porSwitch = <int, Map<int, List<PuertoDeSwitch>>>{};
  for (final puerto in datos.mapa.puertos) {
    porSwitch
        .putIfAbsent(puerto.switchId, () => {})
        .putIfAbsent(puerto.indice, () => [])
        .add(puerto);
  }

  final centrosDeSwitch = <int, Offset>{};

  porSwitch.forEach((switchId, puertos) {
    final ejemplo = puertos.values.first.first;
    final altoBloque = matematicas.max(puertos.length * altoFila, altoFila);
    final centroSwitch = Offset(margen + anchoCaja / 2, y + altoBloque / 2);
    centrosDeSwitch[switchId] = centroSwitch;

    cajas.add(CajaPlano(
      rectangulo: Rect.fromCenter(center: centroSwitch, width: anchoCaja, height: altoCaja),
      titulo: ejemplo.switchNombre,
      subtitulo: ejemplo.switchIp,
      color: colores.primaryContainer,
      icono: Icons.router,
      equipoId: switchId,
    ));
    alCrecer(margen + anchoCaja, y + altoBloque);

    double yPuerto = y;
    puertos.forEach((indice, enElPuerto) {
      final confirmado = enElPuerto.length == 1 && enElPuerto.first.confirmado;
      final centroPuerto =
          Offset(margen + anchoColumna + anchoCaja / 2, yPuerto + altoFila / 2);

      if (confirmado) {
        final unico = enElPuerto.first;
        cajas.add(CajaPlano(
          rectangulo: Rect.fromCenter(center: centroPuerto, width: anchoCaja, height: altoCaja),
          titulo: unico.quienEs,
          subtitulo: unico.equipoIp.isNotEmpty ? unico.equipoIp : unico.mac,
          color: colores.surfaceContainerHighest,
          icono: Icons.devices,
          equipoId: unico.equipoId,
        ));
      } else {
        // El grupo se dibuja como UNA caja que dice cuantos hay: es exactamente
        // lo que se sabe, ni mas ni menos.
        //
        // La cuenta buena es la que trae el servidor, no la de renglones que le
        // llegaron a esta pantalla: el servidor conto las MAC del puerto, y el
        // dia que la respuesta venga resumida o recortada, contar renglones
        // diria "1 equipos" donde hay nueve.
        final cuantos = matematicas.max(enElPuerto.length, enElPuerto.first.cuantosEnPuerto);
        cajas.add(CajaPlano(
          rectangulo: Rect.fromCenter(center: centroPuerto, width: anchoCaja, height: altoCaja),
          titulo: '$cuantos ${cuantos == 1 ? 'equipo' : 'equipos'}',
          subtitulo: 'tras algo no administrable',
          color: colores.tertiaryContainer,
          icono: Icons.hub,
        ));
      }

      lineas.add(LineaPlano(
        desde: centroSwitch + const Offset(anchoCaja / 2, 0),
        hasta: centroPuerto - const Offset(anchoCaja / 2, 0),
        confirmada: confirmado,
        etiqueta: enElPuerto.first.puerto,
        color: siguienteColor(),
      ));
      alCrecer(margen + anchoColumna + anchoCaja, yPuerto + altoFila);
      yPuerto += altoFila;
    });

    y += altoBloque + separacionY * 2;
  });

  // Los cables entre switches, en arco por la IZQUIERDA: ahora los switches
  // estan uno debajo de otro, y una recta entre el primero y el tercero pasaria
  // por encima del segundo y se leeria como si lo tocara.
  for (final cable in datos.mapa.enlacesUnicos) {
    if (cable.enlace.vecinoId == null) continue;
    final desde = centrosDeSwitch[cable.enlace.equipoId];
    final hasta = centrosDeSwitch[cable.enlace.vecinoId];
    if (desde == null || hasta == null || desde == hasta) continue;

    // Cuanto mas separados esten, mas se abre el arco: asi dos cables distintos
    // no se encinan en la misma curva.
    final separacion = (hasta.dy - desde.dy).abs();
    final desvio = 24 + separacion / 12;

    enlaces.add(EnlacePlano(
      desde: desde - const Offset(anchoCaja / 2, 0),
      hasta: hasta - const Offset(anchoCaja / 2, 0),
      cima: Offset(desde.dx - anchoCaja / 2 - desvio, (desde.dy + hasta.dy) / 2),
      etiqueta: cable.enlace.vecinoPuerto.isEmpty
          ? cable.enlace.interfazLocal
          : '${cable.enlace.interfazLocal} ↔ ${cable.enlace.vecinoPuerto}',
      porAmbos: cable.porAmbos,
    ));
  }

  // ------------------------------------------- lo que se declaro a mano ---
  //
  // El aparato de entrada a la izquierda y, a su derecha, lo que cuelga de el;
  // y a la derecha de eso, lo que cuelga de aquello. Se dibuja punteado de punta
  // a punta —caja y cable— porque es lo que alguien TECLEO, y presentarlo igual
  // que lo confirmado seria hacer pasar una declaracion por una medicion.
  //
  // **Cada aparato se dibuja UNA vez** y el puerto por donde sube al de la
  // izquierda no se repite como caja: esa conexion ya es la linea que llega.
  final arbol = _ArbolDeclarado(datos);
  if (arbol.raices.isNotEmpty) {
    if (y > margen) y += separacionY * 2;
    for (final raiz in arbol.raices) {
      final alto = arbol.colocar(
        equipo: raiz,
        x: margen,
        y: y,
        colores: colores,
        cajas: cajas,
        lineas: lineas,
        alCrecer: alCrecer,
        siguienteColor: siguienteColor,
      );
      y += alto + separacionY * 2;
    }
  }

  // Los que no cuelgan de ningun sitio conocido: en su propia zona, abajo del
  // todo, en columnas. No se esconden: un plano incompleto sin avisar es peor
  // que uno que dice "de estos no se sabe donde estan".
  final sinUbicar = datos.sinUbicar;
  if (sinUbicar.isNotEmpty) {
    y = altoMaximo + altoFila;
    final porColumna = matematicas.max(1, sinUbicar.length > 6 ? (sinUbicar.length / 3).ceil() : sinUbicar.length);

    for (var i = 0; i < sinUbicar.length; i++) {
      final equipo = sinUbicar[i];
      final columna = i ~/ porColumna;
      final fila = i % porColumna;
      final donde = Offset(margen + columna * anchoColumna, y + fila * altoFila);

      cajas.add(CajaPlano(
        rectangulo: Rect.fromLTWH(donde.dx, donde.dy, anchoCaja, altoCaja),
        titulo: equipo.comoSeLlama,
        // Sin nombre, el titulo YA es la IP: repetirla debajo desperdicia el
        // unico renglon que queda para decir algo del aparato.
        subtitulo: equipo.comoSeLlama == equipo.ip
            ? (equipo.fabricante.isNotEmpty ? equipo.fabricante : equipo.tipo)
            : equipo.ip,
        color: colores.surfaceContainerLow,
        icono: equipo.presente ? Icons.help_outline : Icons.power_off,
        equipoId: equipo.id,
      ));
      alCrecer(donde.dx + anchoCaja, donde.dy + altoCaja);
    }
  }

  return Plano(
    cajas: cajas,
    lineas: lineas,
    enlaces: enlaces,
    tamano: Size(matematicas.max(anchoMaximo + margen, 800),
        matematicas.max(altoMaximo + margen, 600)),
    colorLinea: colores.outline,
    colorTexto: colores.onSurface,
    colorFondo: colores.surface,
    colorEnlace: colores.primary,
  );
}

class PintorMapa extends CustomPainter {
  PintorMapa({required this.plano, this.conFondo = false});

  final Plano plano;

  /// En pantalla el fondo lo pone el Scaffold. En un PNG no hay Scaffold: sin
  /// esto el archivo sale con fondo transparente y quien lo abra sobre negro no
  /// vera ni una letra.
  final bool conFondo;

  @override
  void paint(Canvas lienzo, Size tamano) {
    if (conFondo) {
      lienzo.drawRect(Offset.zero & tamano, Paint()..color = plano.colorFondo);
    }

    // Los cables entre switches van primero: son el esqueleto, y lo demas se
    // dibuja encima.
    for (final enlace in plano.enlaces) {
      final arco = Path()
        ..moveTo(enlace.desde.dx, enlace.desde.dy)
        ..quadraticBezierTo(
            enlace.cima.dx, enlace.cima.dy, enlace.hasta.dx, enlace.hasta.dy);
      lienzo.drawPath(
        arco,
        Paint()
          ..color = plano.colorEnlace
          ..strokeWidth = enlace.porAmbos ? 3 : 2
          ..style = PaintingStyle.stroke,
      );
      _texto(lienzo, enlace.etiqueta,
          Offset(enlace.cima.dx - 60, enlace.cima.dy - 4), 11, plano.colorEnlace,
          ancho: 160);
    }

    final trazo = Paint()
      ..color = plano.colorLinea
      ..strokeWidth = 2
      ..style = PaintingStyle.stroke;

    for (final linea in plano.lineas) {
      final tinte = linea.color ?? plano.colorLinea;
      trazo.color = tinte;

      if (linea.inalambrica) {
        // Puntos finos: por el aire no hay cable ni puerto que senalar.
        _lineaPunteada(lienzo, linea.desde, linea.hasta, trazo, largo: 2, hueco: 4);
      } else if (linea.confirmada) {
        lienzo.drawLine(linea.desde, linea.hasta, trazo);
      } else if (linea.declarada) {
        // Guion largo: se distingue a simple vista del punteado corto del grupo
        // inferido. Son tres cosas distintas —medido, deducido y tecleado— y el
        // plano tiene que dejar ver cual es cual sin leer la leyenda.
        _lineaPunteada(lienzo, linea.desde, linea.hasta, trazo, largo: 12, hueco: 5);
      } else {
        _lineaPunteada(lienzo, linea.desde, linea.hasta, trazo);
      }
      // La etiqueta va EN MAYUSCULAS, del tamano del nombre del aparato y del
      // color de su enlace: antes era un gris de 11 puntos que apenas se veia,
      // y es justo el dato que dice por donde entra cada cosa.
      _texto(lienzo, linea.etiqueta.toUpperCase(),
          Offset((linea.desde.dx + linea.hasta.dx) / 2 - 40,
              (linea.desde.dy + linea.hasta.dy) / 2 - 20),
          13, tinte,
          negrita: true, ancho: separacionX + 20);
    }
    trazo.color = plano.colorLinea;

    for (final caja in plano.cajas) {
      final fondo = Paint()..color = caja.color;
      final borde = Paint()
        ..color = plano.colorLinea
        ..style = PaintingStyle.stroke
        ..strokeWidth = 1;
      final redondeado = RRect.fromRectAndRadius(caja.rectangulo, const Radius.circular(8));

      lienzo.drawRRect(redondeado, fondo);
      if (caja.declarada) {
        _rectanguloPunteado(lienzo, redondeado, borde);
      } else {
        lienzo.drawRRect(redondeado, borde);
      }

      _icono(lienzo, caja.icono, caja.rectangulo.topLeft + const Offset(10, 12), plano.colorTexto);
      _texto(lienzo, caja.titulo, caja.rectangulo.topLeft + const Offset(34, 8), 13,
          plano.colorTexto,
          negrita: true, ancho: anchoCaja - 44);
      _texto(lienzo, caja.subtitulo, caja.rectangulo.topLeft + const Offset(34, 28), 11,
          plano.colorLinea,
          ancho: anchoCaja - 44);
    }
  }

  /// _rectanguloPunteado marca una caja que sale de lo que alguien tecleo.
  ///
  /// El borde punteado hace lo mismo que la linea punteada en los cables: dice
  /// de un vistazo que ese dato no se midio. Un plano donde lo declarado se ve
  /// igual que lo confirmado acaba usandose como si todo estuviera comprobado.
  void _rectanguloPunteado(Canvas lienzo, RRect caja, Paint trazo) {
    final esquinas = [
      caja.outerRect.topLeft,
      caja.outerRect.topRight,
      caja.outerRect.bottomRight,
      caja.outerRect.bottomLeft,
    ];
    for (var i = 0; i < esquinas.length; i++) {
      _lineaPunteada(lienzo, esquinas[i], esquinas[(i + 1) % esquinas.length], trazo,
          largo: 5, hueco: 4);
    }
  }

  void _lineaPunteada(Canvas lienzo, Offset desde, Offset hasta, Paint trazo,
      {double largo = 6, double hueco = 5}) {
    final total = (hasta - desde).distance;
    if (total == 0) return;
    final paso = (hasta - desde) / total;

    var recorrido = 0.0;
    while (recorrido < total) {
      final fin = matematicas.min(recorrido + largo, total);
      lienzo.drawLine(desde + paso * recorrido, desde + paso * fin, trazo);
      recorrido = fin + hueco;
    }
  }

  void _texto(Canvas lienzo, String contenido, Offset donde, double tamano, Color color,
      {bool negrita = false, double? ancho}) {
    final pintor = TextPainter(
      text: TextSpan(
        text: contenido,
        style: TextStyle(
          color: color,
          fontSize: tamano,
          fontWeight: negrita ? FontWeight.w600 : FontWeight.normal,
        ),
      ),
      textDirection: TextDirection.ltr,
      maxLines: 1,
      ellipsis: '…',
    )..layout(maxWidth: ancho ?? 200);
    pintor.paint(lienzo, donde);
  }

  void _icono(Canvas lienzo, IconData icono, Offset donde, Color color) {
    final pintor = TextPainter(
      text: TextSpan(
        text: String.fromCharCode(icono.codePoint),
        style: TextStyle(
          fontSize: 18,
          fontFamily: icono.fontFamily,
          package: icono.fontPackage,
          color: color,
        ),
      ),
      textDirection: TextDirection.ltr,
    )..layout();
    pintor.paint(lienzo, donde);
  }

  @override
  bool shouldRepaint(covariant PintorMapa anterior) =>
      anterior.plano != plano || anterior.conFondo != conFondo;
}
