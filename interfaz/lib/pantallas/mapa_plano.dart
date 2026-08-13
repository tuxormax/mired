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
  const DatosMapa({required this.mapa, required this.equipos});

  final MapaPuertos mapa;
  final List<Equipo> equipos;

  /// Los equipos que ningun switch reporto en ninguna boca. Se muestran aparte:
  /// esconderlos daria un plano incompleto sin avisar.
  List<Equipo> get sinUbicar {
    final ubicados = mapa.puertos
        .where((puerto) => puerto.equipoId != null)
        .map((puerto) => puerto.equipoId)
        .toSet();
    final switches = mapa.puertos.map((puerto) => puerto.switchId).toSet();
    return equipos
        .where((equipo) => !ubicados.contains(equipo.id) && !switches.contains(equipo.id))
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
  });

  final Rect rectangulo;
  final String titulo;
  final String subtitulo;
  final Color color;
  final IconData icono;
}

class LineaPlano {
  LineaPlano({
    required this.desde,
    required this.hasta,
    required this.confirmada,
    required this.etiqueta,
  });

  final Offset desde;
  final Offset hasta;
  final bool confirmada;
  final String etiqueta;
}

class Plano {
  Plano({
    required this.cajas,
    required this.lineas,
    required this.tamano,
    required this.colorLinea,
    required this.colorTexto,
    required this.colorFondo,
  });

  final List<CajaPlano> cajas;
  final List<LineaPlano> lineas;
  final Size tamano;
  final Color colorLinea;
  final Color colorTexto;
  final Color colorFondo;
}

const double anchoCaja = 190;
const double altoCaja = 54;
const double separacionX = 30;
const double separacionY = 130;

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

  double x = separacionX;
  double anchoMaximo = 0;
  double altoMaximo = 0;

  porSwitch.forEach((switchId, bocas) {
    final ejemplo = bocas.values.first.first;
    final anchoBloque =
        matematicas.max(bocas.length * (anchoCaja + separacionX), anchoCaja + separacionX);

    final centroSwitch = Offset(x + anchoBloque / 2, separacionY / 2);
    cajas.add(CajaPlano(
      rectangulo: Rect.fromCenter(
          center: centroSwitch, width: anchoCaja, height: altoCaja),
      titulo: ejemplo.switchNombre,
      subtitulo: ejemplo.switchIp,
      color: colores.primaryContainer,
      icono: Icons.router,
    ));

    double xBoca = x;
    bocas.forEach((indice, enLaBoca) {
      final confirmado = enLaBoca.length == 1 && enLaBoca.first.confirmado;
      final centroBoca = Offset(xBoca + anchoCaja / 2, separacionY / 2 + separacionY);

      if (confirmado) {
        final unico = enLaBoca.first;
        cajas.add(CajaPlano(
          rectangulo:
              Rect.fromCenter(center: centroBoca, width: anchoCaja, height: altoCaja),
          titulo: unico.quienEs,
          subtitulo: unico.equipoIp.isNotEmpty ? unico.equipoIp : unico.mac,
          color: colores.surfaceContainerHighest,
          icono: Icons.devices,
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
    altoMaximo = matematicas.max(altoMaximo, separacionY * 2 + altoCaja);
  });

  // Los que no cuelgan de ningun switch conocido: en su propia zona, abajo.
  final sinUbicar = datos.sinUbicar;
  if (sinUbicar.isNotEmpty) {
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
      ));
      xSuelto += anchoCaja + separacionX;
      altoMaximo = matematicas.max(altoMaximo, filaY + altoCaja + separacionY / 2);
    }
  }

  return Plano(
    cajas: cajas,
    lineas: lineas,
    tamano: Size(matematicas.max(anchoMaximo + separacionX, 800),
        matematicas.max(altoMaximo + separacionY, 600)),
    colorLinea: colores.outline,
    colorTexto: colores.onSurface,
    colorFondo: colores.surface,
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

    final trazo = Paint()
      ..color = plano.colorLinea
      ..strokeWidth = 2
      ..style = PaintingStyle.stroke;

    for (final linea in plano.lineas) {
      if (linea.confirmada) {
        lienzo.drawLine(linea.desde, linea.hasta, trazo);
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
      lienzo.drawRRect(redondeado, borde);

      _icono(lienzo, caja.icono, caja.rectangulo.topLeft + const Offset(10, 12), plano.colorTexto);
      _texto(lienzo, caja.titulo, caja.rectangulo.topLeft + const Offset(34, 8), 13,
          plano.colorTexto,
          negrita: true, ancho: anchoCaja - 44);
      _texto(lienzo, caja.subtitulo, caja.rectangulo.topLeft + const Offset(34, 28), 11,
          plano.colorLinea,
          ancho: anchoCaja - 44);
    }
  }

  void _lineaPunteada(Canvas lienzo, Offset desde, Offset hasta, Paint trazo) {
    const largo = 6.0;
    const hueco = 5.0;
    final total = (hasta - desde).distance;
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
