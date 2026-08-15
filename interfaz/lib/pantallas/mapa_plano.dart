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
  });

  final Offset desde;
  final Offset hasta;
  final bool confirmada;
  final String etiqueta;

  /// El cable lo declaro una persona. Se dibuja distinto del confirmado y del
  /// inferido: son tres cosas distintas y presentarlas igual seria hacer pasar
  /// lo tecleado por medido.
  final bool declarada;
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
const double separacionX = 30;
const double separacionY = 130;

/// Espacio que se reserva arriba para los arcos de los enlaces entre switches.
/// Solo se reserva si hay enlaces que dibujar: si no, seria un hueco en blanco.
const double altoEnlaces = 90;

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

    for (final cable in datos.topologia.enlaces) {
      final padre = datos.topologia.equipoDelPuerto(cable.puertoOrigenId);
      final hijo = cable.puertoDestinoId != null
          ? datos.topologia.equipoDelPuerto(cable.puertoDestinoId!)
          : cable.equipoDestinoId;
      if (padre == null || hijo == null || padre == hijo) continue;
      if (!_tienePuertos.contains(hijo)) continue; // sin puertos es una hoja
      if (_padreDe.containsKey(hijo)) continue; // ya cuelga de otro
      if (_desciendeDe(padre, hijo)) continue; // cerraria un circulo
      _padreDe[hijo] = padre;
    }

    raices = conPuertos.where((equipo) => !_padreDe.containsKey(equipo.id)).toList();
  }

  final DatosMapa datos;
  final Map<int, int> _padreDe = {};
  final Set<int> _tienePuertos = {};
  final Map<int, double> _anchos = {};
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

  /// Cuanto mide en horizontal el bloque de un aparato con todo lo que le
  /// cuelga. Se mide antes de colocar nada: sin esto los subarboles se pisan.
  double medir(Equipo equipo) {
    final guardado = _anchos[equipo.id];
    if (guardado != null) return guardado;
    _anchos[equipo.id] = anchoCaja + separacionX; // provisional, corta circulos

    double total = 0;
    for (final puerto in datos.topologia.puertosDe(equipo.id)) {
      final hijo = hijoEn(equipo.id, puerto);
      total += hijo == null ? anchoCaja + separacionX : medir(hijo);
    }

    final ancho = matematicas.max(total, anchoCaja + separacionX);
    _anchos[equipo.id] = ancho;
    return ancho;
  }

  /// colocar dibuja el aparato, sus puertos y todo lo que cuelga de ellos.
  /// Devuelve cuanto ocupo de ancho.
  double colocar({
    required Equipo equipo,
    required double x,
    required double y,
    required ColorScheme colores,
    required List<CajaPlano> cajas,
    required List<LineaPlano> lineas,
    required void Function(double) alBajar,
  }) {
    alBajar(y);
    final anchoBloque = medir(equipo);
    final centroEquipo = Offset(x + anchoBloque / 2, y);

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

    double xPuerto = x;
    for (final puerto in datos.topologia.puertosDe(equipo.id)) {
      final hijo = hijoEn(equipo.id, puerto);
      final cable = datos.topologia.enlaceDe(puerto.id);
      final anchoPuerto = hijo == null ? anchoCaja + separacionX : medir(hijo);
      final centroPuerto = Offset(xPuerto + anchoPuerto / 2, y + separacionY);

      if (hijo != null) {
        // Lo que cuelga de este puerto es otro aparato con puertos propios: se
        // dibuja aqui mismo, con los suyos debajo. Es la unica vez que sale.
        colocar(
          equipo: hijo,
          x: xPuerto,
          y: y + separacionY,
          colores: colores,
          cajas: cajas,
          lineas: lineas,
          alBajar: alBajar,
        );
      } else if (cable == null) {
        // Un puerto libre NO se esconde: es justo donde el modo edicion ofrece
        // conectar algo, y verlo vacio dice cuanto falta por declarar.
        cajas.add(CajaPlano(
          rectangulo: Rect.fromCenter(center: centroPuerto, width: anchoCaja, height: altoCaja),
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
        // Hay cable, pero el otro extremo no cuelga de aqui: o es una hoja sin
        // puertos declarados, o es el aparato de arriba y el cable SUBE. En los
        // dos casos el puerto esta ocupado, y eso es justo lo que hay que ver.
        final otro = _otroExtremo(cable, equipo.id);
        final sube = otro != null && _padreDe[equipo.id] == otro;
        final soyOrigen = cable.puertoOrigenId == puerto.id;
        final nombreOtro = soyOrigen ? cable.destinoNombre : cable.origenNombre;

        cajas.add(CajaPlano(
          rectangulo: Rect.fromCenter(center: centroPuerto, width: anchoCaja, height: altoCaja),
          titulo: nombreOtro.isEmpty ? 'Conectado' : (sube ? '↑ $nombreOtro' : nombreOtro),
          subtitulo: sube ? 'sube al aparato de arriba' : 'declarado a mano',
          color: colores.surfaceContainerHighest,
          icono: sube ? Icons.arrow_upward : Icons.devices,
          equipoId: otro,
          puertoFisicoId: puerto.id,
          enlaceId: cable.id,
          declarada: true,
        ));
      }

      lineas.add(LineaPlano(
        desde: centroEquipo + const Offset(0, altoCaja / 2),
        hasta: centroPuerto - const Offset(0, altoCaja / 2),
        confirmada: false,
        declarada: true,
        etiqueta: puerto.tipo == 'wan' ? 'WAN' : 'puerto ${puerto.numero}',
      ));

      xPuerto += anchoPuerto;
    }

    alBajar(y + separacionY);
    return anchoBloque;
  }
}

/// armarPlano coloca todo: una columna por switch, y debajo de cada uno sus
/// puertos con lo que cuelga. Es un arbol por niveles, que para un plano de sitio
/// se lee mucho mejor que una maraña de nodos flotando.
Plano armarPlano(DatosMapa datos, ColorScheme colores) {
  final cajas = <CajaPlano>[];
  final lineas = <LineaPlano>[];

  // Agrupar por switch y, dentro, por puerto.
  final porSwitch = <int, Map<int, List<PuertoDeSwitch>>>{};
  for (final puerto in datos.mapa.puertos) {
    porSwitch
        .putIfAbsent(puerto.switchId, () => {})
        .putIfAbsent(puerto.indice, () => [])
        .add(puerto);
  }

  // Los cables entre switches solo se pueden dibujar si se conocen los dos
  // extremos. Se calcula antes de colocar nada porque de esto depende cuanto
  // espacio hay que dejar arriba: reservarlo siempre seria una franja en blanco
  // en las redes de un solo switch, que son la mayoria.
  final cables = datos.mapa.enlacesUnicos
      .where((cable) =>
          cable.enlace.vecinoId != null &&
          porSwitch.containsKey(cable.enlace.equipoId) &&
          porSwitch.containsKey(cable.enlace.vecinoId))
      .toList();
  final arriba = cables.isEmpty ? 0.0 : altoEnlaces;

  double x = separacionX;
  double anchoMaximo = 0;
  double altoMaximo = 0;
  final centrosDeSwitch = <int, Offset>{};

  porSwitch.forEach((switchId, puertos) {
    final ejemplo = puertos.values.first.first;
    final anchoBloque =
        matematicas.max(puertos.length * (anchoCaja + separacionX), anchoCaja + separacionX);

    final centroSwitch = Offset(x + anchoBloque / 2, arriba + separacionY / 2);
    centrosDeSwitch[switchId] = centroSwitch;
    cajas.add(CajaPlano(
      rectangulo: Rect.fromCenter(
          center: centroSwitch, width: anchoCaja, height: altoCaja),
      titulo: ejemplo.switchNombre,
      subtitulo: ejemplo.switchIp,
      color: colores.primaryContainer,
      icono: Icons.router,
      equipoId: switchId,
    ));

    double xPuerto = x;
    puertos.forEach((indice, enElPuerto) {
      final confirmado = enElPuerto.length == 1 && enElPuerto.first.confirmado;
      final centroPuerto = Offset(xPuerto + anchoCaja / 2, arriba + separacionY / 2 + separacionY);

      if (confirmado) {
        final unico = enElPuerto.first;
        cajas.add(CajaPlano(
          rectangulo:
              Rect.fromCenter(center: centroPuerto, width: anchoCaja, height: altoCaja),
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
          rectangulo:
              Rect.fromCenter(center: centroPuerto, width: anchoCaja, height: altoCaja),
          titulo: '$cuantos ${cuantos == 1 ? 'equipo' : 'equipos'}',
          subtitulo: 'tras algo no administrable',
          color: colores.tertiaryContainer,
          icono: Icons.hub,
        ));
      }

      lineas.add(LineaPlano(
        desde: centroSwitch + const Offset(0, altoCaja / 2),
        hasta: centroPuerto - const Offset(0, altoCaja / 2),
        confirmada: confirmado,
        etiqueta: enElPuerto.first.puerto,
      ));

      xPuerto += anchoCaja + separacionX;
    });

    x += anchoBloque + separacionX * 2;
    anchoMaximo = matematicas.max(anchoMaximo, x);
    altoMaximo = matematicas.max(altoMaximo, arriba + separacionY * 2 + altoCaja);
  });

  // Los cables entre switches, en arco por encima de la fila.
  final enlaces = <EnlacePlano>[];
  for (final cable in cables) {
    final desde = centrosDeSwitch[cable.enlace.equipoId];
    final hasta = centrosDeSwitch[cable.enlace.vecinoId];
    if (desde == null || hasta == null || desde == hasta) continue;

    // Cuanto mas separados esten los dos switches, mas alto sube el arco: asi
    // dos cables distintos no se encinan en la misma curva.
    final separacion = (hasta.dx - desde.dx).abs();
    final altura = matematicas.min(arriba - 12, 24 + separacion / 12);

    enlaces.add(EnlacePlano(
      desde: desde - const Offset(0, altoCaja / 2),
      hasta: hasta - const Offset(0, altoCaja / 2),
      cima: Offset((desde.dx + hasta.dx) / 2, desde.dy - altoCaja / 2 - altura),
      etiqueta: cable.enlace.vecinoPuerto.isEmpty
          ? cable.enlace.interfazLocal
          : '${cable.enlace.interfazLocal} ↔ ${cable.enlace.vecinoPuerto}',
      porAmbos: cable.porAmbos,
    ));
  }

  // ------------------------------------------- lo que se declaro a mano ---
  //
  // Va en su propia franja, debajo de lo que dijeron los switches, y con el
  // mismo reparto: el aparato arriba y sus puertos debajo. Se dibuja punteado de
  // punta a punta —caja y cable— porque es lo que alguien TECLEO, y presentarlo
  // igual que lo confirmado seria hacer pasar una declaracion por una medicion.
  // **Cada aparato se dibuja UNA vez.** Un switch colgado del modem es hijo del
  // modem: va debajo de su puerto, con sus propios puertos debajo. Antes salia
  // dos veces —una como caja bajo el puerto del modem y otra como bloque suelto
  // con sus puertos—, y el cable que sube al modem no ocupaba ninguno de sus
  // puertos, asi que un switch de 5 con el uplink puesto seguia ofreciendo 5.
  final arbol = _ArbolDeclarado(datos);
  if (arbol.raices.isNotEmpty) {
    if (altoMaximo == 0) altoMaximo = arriba + separacionY / 2;
    double xManual = separacionX;
    final yCabecera = altoMaximo + separacionY / 2;
    double fondo = yCabecera;

    for (final raiz in arbol.raices) {
      final ancho = arbol.colocar(
        equipo: raiz,
        x: xManual,
        y: yCabecera,
        colores: colores,
        cajas: cajas,
        lineas: lineas,
        alBajar: (y) => fondo = matematicas.max(fondo, y),
      );
      xManual += ancho + separacionX * 2;
      anchoMaximo = matematicas.max(anchoMaximo, xManual);
    }

    altoMaximo = matematicas.max(altoMaximo, fondo + altoCaja);
  }

  // Los que no cuelgan de ningun switch conocido: en su propia zona, abajo.
  final sinUbicar = datos.sinUbicar;
  if (sinUbicar.isNotEmpty) {
    if (altoMaximo == 0) altoMaximo = arriba + separacionY / 2;
    final yBase = altoMaximo + separacionY;
    double xSuelto = separacionX;
    double filaY = yBase;
    final porFila = matematicas.max(1, (anchoMaximo / (anchoCaja + separacionX)).floor());

    for (var i = 0; i < sinUbicar.length; i++) {
      final equipo = sinUbicar[i];
      if (i > 0 && i % porFila == 0) {
        xSuelto = separacionX;
        filaY += altoCaja + 20;
      }
      cajas.add(CajaPlano(
        rectangulo: Rect.fromLTWH(xSuelto, filaY, anchoCaja, altoCaja),
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
      xSuelto += anchoCaja + separacionX;
      altoMaximo = matematicas.max(altoMaximo, filaY + altoCaja + separacionY / 2);
    }
  }

  return Plano(
    cajas: cajas,
    lineas: lineas,
    enlaces: enlaces,
    tamano: Size(matematicas.max(anchoMaximo + separacionX, 800),
        matematicas.max(altoMaximo + separacionY, 600)),
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
      if (linea.confirmada) {
        lienzo.drawLine(linea.desde, linea.hasta, trazo);
      } else if (linea.declarada) {
        // Guion largo: se distingue a simple vista del punteado corto del grupo
        // inferido. Son tres cosas distintas —medido, deducido y tecleado— y el
        // plano tiene que dejar ver cual es cual sin leer la leyenda.
        _lineaPunteada(lienzo, linea.desde, linea.hasta, trazo, largo: 12, hueco: 5);
      } else {
        _lineaPunteada(lienzo, linea.desde, linea.hasta, trazo);
      }
      _texto(lienzo, linea.etiqueta,
          Offset((linea.desde.dx + linea.hasta.dx) / 2 + 6,
              (linea.desde.dy + linea.hasta.dy) / 2 - 8),
          11, plano.colorLinea);
    }

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
