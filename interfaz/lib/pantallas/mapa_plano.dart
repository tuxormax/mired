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

  /// Lo declarado a mano: las bocas que alguien conto mirando el aparato y los
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

  /// Los equipos que tienen bocas declaradas: se dibujan como cabecera de su
  /// propio bloque, igual que un switch administrable.
  List<Equipo> get conBocasDeclaradas {
    final ids = topologia.puertos.map((puerto) => puerto.equipoId).toSet();
    final lista = equipos.where((equipo) => ids.contains(equipo.id)).toList();
    lista.sort((uno, otro) => uno.comoSeLlama.compareTo(otro.comoSeLlama));
    return lista;
  }

  /// Los equipos que ningun switch reporto en ninguna boca y que tampoco
  /// aparecen en lo declarado a mano. Se muestran aparte: esconderlos daria un
  /// plano incompleto sin avisar.
  List<Equipo> get sinUbicar {
    final ubicados = mapa.puertos
        .where((puerto) => puerto.equipoId != null)
        .map((puerto) => puerto.equipoId)
        .toSet();
    final switches = mapa.puertos.map((puerto) => puerto.switchId).toSet();
    // Lo declarado tambien ubica: un equipo colgado a mano de la boca 3 del
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
    this.bocaLibre = false,
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

  /// La boca declarada que representa esta caja, si es una boca.
  final int? puertoFisicoId;

  /// El cable declarado que la ocupa, si hay alguno.
  final int? enlaceId;

  /// Una boca declarada sin nada conectado. Es donde el modo edicion ofrece
  /// conectar algo.
  final bool bocaLibre;

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

/// armarPlano coloca todo: una columna por switch, y debajo de cada uno sus
/// bocas con lo que cuelga. Es un arbol por niveles, que para un plano de sitio
/// se lee mucho mejor que una maraña de nodos flotando.
Plano armarPlano(DatosMapa datos, ColorScheme colores) {
  final cajas = <CajaPlano>[];
  final lineas = <LineaPlano>[];

  // Agrupar por switch y, dentro, por boca.
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

  porSwitch.forEach((switchId, bocas) {
    final ejemplo = bocas.values.first.first;
    final anchoBloque =
        matematicas.max(bocas.length * (anchoCaja + separacionX), anchoCaja + separacionX);

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

    double xBoca = x;
    bocas.forEach((indice, enLaBoca) {
      final confirmado = enLaBoca.length == 1 && enLaBoca.first.confirmado;
      final centroBoca = Offset(xBoca + anchoCaja / 2, arriba + separacionY / 2 + separacionY);

      if (confirmado) {
        final unico = enLaBoca.first;
        cajas.add(CajaPlano(
          rectangulo:
              Rect.fromCenter(center: centroBoca, width: anchoCaja, height: altoCaja),
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
        // llegaron a esta pantalla: el servidor conto las MAC de la boca, y el
        // dia que la respuesta venga resumida o recortada, contar renglones
        // diria "1 equipos" donde hay nueve.
        final cuantos = matematicas.max(enLaBoca.length, enLaBoca.first.cuantosEnBoca);
        cajas.add(CajaPlano(
          rectangulo:
              Rect.fromCenter(center: centroBoca, width: anchoCaja, height: altoCaja),
          titulo: '$cuantos ${cuantos == 1 ? 'equipo' : 'equipos'}',
          subtitulo: 'tras algo no administrable',
          color: colores.tertiaryContainer,
          icono: Icons.hub,
        ));
      }

      lineas.add(LineaPlano(
        desde: centroSwitch + const Offset(0, altoCaja / 2),
        hasta: centroBoca - const Offset(0, altoCaja / 2),
        confirmada: confirmado,
        etiqueta: enLaBoca.first.puerto,
      ));

      xBoca += anchoCaja + separacionX;
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
  // mismo reparto: el aparato arriba y sus bocas debajo. Se dibuja punteado de
  // punta a punta —caja y cable— porque es lo que alguien TECLEO, y presentarlo
  // igual que lo confirmado seria hacer pasar una declaracion por una medicion.
  final conBocas = datos.conBocasDeclaradas;
  if (conBocas.isNotEmpty) {
    if (altoMaximo == 0) altoMaximo = arriba + separacionY / 2;
    double xManual = separacionX;
    final yCabecera = altoMaximo + separacionY / 2;

    for (final equipo in conBocas) {
      final bocas = datos.topologia.puertosDe(equipo.id);
      final anchoBloque = matematicas.max(
          bocas.length * (anchoCaja + separacionX), anchoCaja + separacionX);
      final centroEquipo = Offset(xManual + anchoBloque / 2, yCabecera);

      cajas.add(CajaPlano(
        rectangulo: Rect.fromCenter(
            center: centroEquipo, width: anchoCaja, height: altoCaja),
        titulo: equipo.comoSeLlama,
        subtitulo: equipo.modelo.isNotEmpty
            ? equipo.modelo
            : (equipo.ip.isNotEmpty ? equipo.ip : 'declarado a mano'),
        color: colores.secondaryContainer,
        icono: Icons.settings_ethernet,
        equipoId: equipo.id,
        declarada: equipo.esManual,
      ));

      double xBoca = xManual;
      for (final boca in bocas) {
        final centroBoca = Offset(xBoca + anchoCaja / 2, yCabecera + separacionY);
        final cable = datos.topologia.enlaceDe(boca.id);

        if (cable == null) {
          // Una boca libre NO se esconde: es justo donde el modo edicion
          // ofrece conectar algo, y verla vacia dice cuanto falta por declarar.
          cajas.add(CajaPlano(
            rectangulo:
                Rect.fromCenter(center: centroBoca, width: anchoCaja, height: altoCaja),
            titulo: 'Boca ${boca.etiqueta} libre',
            subtitulo: boca.velocidadMbps != null ? '${boca.velocidadMbps} Mbps' : 'sin conectar',
            color: colores.surfaceContainerLow,
            icono: Icons.add_circle_outline,
            equipoId: equipo.id,
            puertoFisicoId: boca.id,
            bocaLibre: true,
            declarada: true,
          ));
        } else {
          // El nombre del otro extremo, mirado desde ESTA boca: un cable
          // declarado desde la otra punta apunta hacia aca.
          final soyOrigen = cable.puertoOrigenId == boca.id;
          final otroLado = soyOrigen ? cable.destinoNombre : cable.origenNombre;
          cajas.add(CajaPlano(
            rectangulo:
                Rect.fromCenter(center: centroBoca, width: anchoCaja, height: altoCaja),
            titulo: otroLado.isEmpty ? 'Conectado' : otroLado,
            subtitulo: 'declarado a mano',
            color: colores.surfaceContainerHighest,
            icono: Icons.devices,
            equipoId: soyOrigen ? cable.equipoDestinoId : cable.equipoOrigenId,
            puertoFisicoId: boca.id,
            enlaceId: cable.id,
            declarada: true,
          ));
        }

        lineas.add(LineaPlano(
          desde: centroEquipo + const Offset(0, altoCaja / 2),
          hasta: centroBoca - const Offset(0, altoCaja / 2),
          confirmada: false,
          declarada: true,
          etiqueta: boca.tipo == 'wan' ? 'WAN' : 'boca ${boca.numero}',
        ));

        xBoca += anchoCaja + separacionX;
      }

      xManual += anchoBloque + separacionX * 2;
      anchoMaximo = matematicas.max(anchoMaximo, xManual);
    }

    altoMaximo = matematicas.max(altoMaximo, yCabecera + separacionY + altoCaja);
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
        subtitulo: equipo.ip,
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
