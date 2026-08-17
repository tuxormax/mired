import 'dart:convert';
import 'dart:typed_data';
import 'dart:ui' as dibujo;

import 'package:flutter/material.dart';

import '../modelos/categorias.dart';
import '../modelos/modelos.dart';
import '../modelos/tipos_de_puerto.dart';
import '../pantallas/mapa_plano.dart';
import 'hoja_calculo.dart';

/// Este archivo convierte el plano del mapa en los formatos que se entregan:
/// PNG, SVG, PDF y las hojas de calculo (ODS, XLSX y CSV).
///
/// Los tres dibujos salen del **mismo plano** que se ve en pantalla, no de un
/// recalculo aparte, para que el archivo guardado y la pantalla no puedan
/// discrepar. Las hojas de calculo no quieren posiciones sino renglones, asi que
/// no usan el plano —pero si el **mismo arbol**, [ArbolDeclarado], que decide
/// quien cuelga de quien.
///
/// El PNG lo dibuja Flutter. El SVG y el PDF se escriben aqui a mano, byte por
/// byte, sin biblioteca de terceros: son cajas, lineas y texto, que es la parte
/// facil de los dos formatos, y asi el proyecto no arrastra una dependencia mas
/// ni su licencia.

/// EncabezadoMapa es la banda de arriba del archivo exportado.
///
/// Un mapa exportado acaba en un correo o impreso en una pared. Sin decir de que
/// sitio es y de que dia, a la semana ya no se sabe cual de los tres es el
/// bueno.
class EncabezadoMapa {
  const EncabezadoMapa({required this.titulo, required this.subtitulo});

  final String titulo;
  final String subtitulo;

  static const double alto = 58;
}

// -------------------------------------------------------------------- PNG ---

/// pngDelPlano dibuja el plano con el mismo pintor de la pantalla.
///
/// La escala 2 no es capricho: a escala 1 el texto de 11 puntos queda ilegible
/// en cuanto alguien hace zoom sobre el archivo, que es lo primero que se hace
/// con un mapa de red.
Future<Uint8List> pngDelPlano(
  Plano plano,
  EncabezadoMapa encabezado, {
  double escala = 2,
}) async {
  final ancho = plano.tamano.width;
  final alto = plano.tamano.height + EncabezadoMapa.alto;

  final grabadora = dibujo.PictureRecorder();
  final lienzo = Canvas(grabadora);
  lienzo.scale(escala);
  lienzo.drawRect(
      Rect.fromLTWH(0, 0, ancho, alto), Paint()..color = plano.colorFondo);
  _encabezadoEnLienzo(lienzo, plano, encabezado, ancho);

  lienzo.save();
  lienzo.translate(0, EncabezadoMapa.alto);
  PintorMapa(plano: plano).paint(lienzo, plano.tamano);
  lienzo.restore();

  final imagen = await grabadora
      .endRecording()
      .toImage((ancho * escala).round(), (alto * escala).round());
  final datos = await imagen.toByteData(format: dibujo.ImageByteFormat.png);
  imagen.dispose();

  if (datos == null) {
    throw StateError('El navegador no pudo convertir el mapa a PNG.');
  }
  return datos.buffer.asUint8List();
}

void _encabezadoEnLienzo(
    Canvas lienzo, Plano plano, EncabezadoMapa encabezado, double ancho) {
  void escribir(String texto, double y, double tamano, bool negrita, Color color) {
    final pintor = TextPainter(
      text: TextSpan(
        text: texto,
        style: TextStyle(
          color: color,
          fontSize: tamano,
          fontWeight: negrita ? FontWeight.w700 : FontWeight.normal,
        ),
      ),
      textDirection: TextDirection.ltr,
      maxLines: 1,
      ellipsis: '…',
    )..layout(maxWidth: ancho - 40);
    pintor.paint(lienzo, Offset(20, y));
  }

  escribir(encabezado.titulo, 14, 16, true, plano.colorTexto);
  escribir(encabezado.subtitulo, 36, 10, false, plano.colorLinea);
  lienzo.drawLine(
    Offset(20, EncabezadoMapa.alto - 6),
    Offset(ancho - 20, EncabezadoMapa.alto - 6),
    Paint()
      ..color = plano.colorLinea
      ..strokeWidth = 1,
  );
}

// -------------------------------------------------------------------- SVG ---

/// svgDelPlano escribe el mapa como vectores.
///
/// El SVG es el formato para quien quiera **retocar** el plano: se abre en
/// Inkscape o en Illustrator, se mueven cajas y se le agregan anotaciones del
/// sitio. Por eso cada caja lleva su titulo en un elemento de texto aparte y no
/// aplanado.
String svgDelPlano(Plano plano, EncabezadoMapa encabezado) {
  final ancho = plano.tamano.width;
  final alto = plano.tamano.height + EncabezadoMapa.alto;
  final salida = StringBuffer()
    ..writeln('<?xml version="1.0" encoding="UTF-8"?>')
    ..writeln('<svg xmlns="http://www.w3.org/2000/svg" width="${_n(ancho)}" '
        'height="${_n(alto)}" viewBox="0 0 ${_n(ancho)} ${_n(alto)}" '
        'font-family="Helvetica, Arial, sans-serif">')
    ..writeln('<rect width="100%" height="100%" fill="${_color(plano.colorFondo)}"/>');

  // Encabezado.
  salida
    ..writeln('<text x="20" y="27" font-size="16" font-weight="bold" '
        'fill="${_color(plano.colorTexto)}">${_xml(encabezado.titulo)}</text>')
    ..writeln('<text x="20" y="44" font-size="10" '
        'fill="${_color(plano.colorLinea)}">${_xml(encabezado.subtitulo)}</text>')
    ..writeln('<line x1="20" y1="${_n(EncabezadoMapa.alto - 6)}" '
        'x2="${_n(ancho - 20)}" y2="${_n(EncabezadoMapa.alto - 6)}" '
        'stroke="${_color(plano.colorLinea)}" stroke-width="1"/>');

  salida.writeln('<g transform="translate(0 ${_n(EncabezadoMapa.alto)})">');

  for (final enlace in plano.enlaces) {
    salida.writeln('<path d="M ${_n(enlace.desde.dx)} ${_n(enlace.desde.dy)} '
        'Q ${_n(enlace.cima.dx)} ${_n(enlace.cima.dy)} '
        '${_n(enlace.hasta.dx)} ${_n(enlace.hasta.dy)}" fill="none" '
        'stroke="${_color(plano.colorEnlace)}" stroke-width="${enlace.porAmbos ? 3 : 2}"/>');
    salida.writeln('<text x="${_n(enlace.cima.dx)}" y="${_n(enlace.cima.dy - 4)}" '
        'font-size="11" text-anchor="middle" fill="${_color(plano.colorEnlace)}">'
        '${_xml(enlace.etiqueta)}</text>');
  }

  for (final linea in plano.lineas) {
    // El punteado del SVG lo hace el propio formato con stroke-dasharray, con
    // los mismos valores que usa el pintor de la pantalla: guion corto para el
    // grupo inferido, guion largo para lo declarado a mano.
    final punteado = linea.confirmada
        ? ''
        : (linea.declarada ? ' stroke-dasharray="12 5"' : ' stroke-dasharray="6 5"');
    // Cada enlace lleva SU color, igual que en pantalla: de un switch salen
    // tantas lineas como puertos, y todas del mismo gris no se pueden seguir.
    final tinte = linea.color ?? plano.colorLinea;

    // El mismo recorrido en codo que dibuja la pantalla, sacado de la misma
    // funcion: si aqui se trazara una recta, el archivo exportado no seria el
    // mapa que se vio.
    final camino = StringBuffer('M ${_n(linea.desde.dx)} ${_n(linea.desde.dy)}');
    for (final tramo in recorridoDeEnlace(linea)) {
      camino.write(tramo.control == null
          ? ' L ${_n(tramo.hasta.dx)} ${_n(tramo.hasta.dy)}'
          : ' Q ${_n(tramo.control!.dx)} ${_n(tramo.control!.dy)} '
              '${_n(tramo.hasta.dx)} ${_n(tramo.hasta.dy)}');
    }
    salida.writeln('<path d="$camino" fill="none" stroke="${_color(tinte)}" '
        'stroke-width="2"$punteado/>');

    // La etiqueta termina pegada a la caja a la que entra el cable. El SVG sabe
    // alinear por la derecha el solo, asi que aqui no hace falta estimar cuanto
    // mide el texto.
    final fin = finDeEtiqueta(linea);
    salida.writeln('<text x="${_n(fin.dx)}" y="${_n(fin.dy + 13)}" font-size="13" '
        'font-weight="600" text-anchor="end" '
        'fill="${_color(tinte)}">'
        '${_xml(_recortar(linea.etiqueta.toUpperCase(), anchoEtiquetaEnlace, 13, true))}'
        '</text>');
  }

  for (final caja in plano.cajas) {
    final r = caja.rectangulo;
    // La caja de lo declarado tambien va punteada: si el archivo exportado
    // dibujara igual lo tecleado y lo medido, quien lo reciba lo leeria todo
    // como comprobado.
    final bordePunteado = caja.declarada ? ' stroke-dasharray="5 4"' : '';
    salida.writeln('<rect x="${_n(r.left)}" y="${_n(r.top)}" width="${_n(r.width)}" '
        'height="${_n(r.height)}" rx="8" ry="8" fill="${_color(caja.color)}" '
        'stroke="${_color(plano.colorLinea)}" stroke-width="1"$bordePunteado/>');
    salida.writeln('<text x="${_n(r.left + 10)}" y="${_n(r.top + 21)}" font-size="13" '
        'font-weight="600" fill="${_color(plano.colorTexto)}">'
        '${_xml(_recortar(caja.titulo, anchoCaja - 20, 13, true))}</text>');
    salida.writeln('<text x="${_n(r.left + 10)}" y="${_n(r.top + 39)}" font-size="11" '
        'fill="${_color(plano.colorLinea)}">'
        '${_xml(_recortar(caja.subtitulo, anchoCaja - 20, 11, false))}</text>');
  }

  salida
    ..writeln('</g>')
    ..writeln('</svg>');
  return salida.toString();
}

// -------------------------------------------------------------------- PDF ---

/// pdfDelPlano escribe un PDF 1.4 de una sola pagina, del tamano exacto del
/// plano.
///
/// Se usa el tamano del plano y no una hoja carta a proposito: un mapa de red no
/// se lee partido en hojas. Quien lo quiera impreso deja que el visor lo ajuste,
/// que es una casilla en el dialogo de impresion.
///
/// Las fuentes son las base 14 del formato (Helvetica y Helvetica-Bold), que
/// todo visor trae. Asi el archivo no lleva fuentes incrustadas y pesa unos
/// pocos kilobytes.
Uint8List pdfDelPlano(Plano plano, EncabezadoMapa encabezado) {
  final ancho = plano.tamano.width;
  final alto = plano.tamano.height + EncabezadoMapa.alto;

  final contenido = StringBuffer()
    // El PDF cuenta la Y desde abajo y el plano desde arriba. Esta matriz voltea
    // el eje de una vez, y asi todo lo de abajo se escribe con las mismas
    // coordenadas que usa la pantalla. El texto se vuelve a voltear en su propia
    // matriz, que si no saldria de cabeza.
    ..writeln('1 0 0 -1 0 ${_n(alto)} cm')
    ..writeln('${_colorPdf(plano.colorFondo)} rg')
    ..writeln('0 0 ${_n(ancho)} ${_n(alto)} re f');

  _textoPdf(contenido, encabezado.titulo, 20, 14, 16, true, plano.colorTexto);
  _textoPdf(contenido, encabezado.subtitulo, 20, 36, 10, false, plano.colorLinea);
  contenido
    ..writeln('${_colorPdf(plano.colorLinea)} RG')
    ..writeln('1 w')
    ..writeln('20 ${_n(EncabezadoMapa.alto - 6)} m '
        '${_n(ancho - 20)} ${_n(EncabezadoMapa.alto - 6)} l S');

  const double desplazamiento = EncabezadoMapa.alto;

  // Los cables entre switches. El PDF no tiene curva cuadratica, solo cubica:
  // los dos puntos de control de la cubica salen del unico de la cuadratica a
  // dos tercios del camino hacia cada extremo. Es una conversion exacta, no una
  // aproximacion.
  contenido.writeln('${_colorPdf(plano.colorEnlace)} RG');
  for (final enlace in plano.enlaces) {
    final desde = enlace.desde.translate(0, desplazamiento);
    final hasta = enlace.hasta.translate(0, desplazamiento);
    final cima = enlace.cima.translate(0, desplazamiento);
    final control1 = Offset(desde.dx + 2 / 3 * (cima.dx - desde.dx),
        desde.dy + 2 / 3 * (cima.dy - desde.dy));
    final control2 = Offset(hasta.dx + 2 / 3 * (cima.dx - hasta.dx),
        hasta.dy + 2 / 3 * (cima.dy - hasta.dy));

    contenido
      ..writeln('${enlace.porAmbos ? 3 : 2} w')
      ..writeln('${_n(desde.dx)} ${_n(desde.dy)} m '
          '${_n(control1.dx)} ${_n(control1.dy)} '
          '${_n(control2.dx)} ${_n(control2.dy)} '
          '${_n(hasta.dx)} ${_n(hasta.dy)} c S');
    _textoPdf(contenido, enlace.etiqueta, cima.dx - 60, cima.dy - 15, 11, false,
        plano.colorEnlace);
  }

  for (final linea in plano.lineas) {
    final tinte = linea.color ?? plano.colorLinea;
    contenido
      ..writeln('${_colorPdf(tinte)} RG')
      ..writeln('2 w')
      // Guion corto para el grupo inferido, guion largo para lo declarado a
      // mano, continuo para lo confirmado. Los mismos tres trazos de pantalla.
      ..writeln(linea.confirmada
          ? '[] 0 d'
          : (linea.declarada ? '[12 5] 0 d' : '[6 5] 0 d'));

    // El recorrido en codo, el mismo que la pantalla. El PDF no tiene curva
    // cuadratica: cada esquina se pasa a cubica con la conversion exacta de dos
    // tercios, igual que los arcos de arriba.
    var pluma = linea.desde.translate(0, desplazamiento);
    contenido.write('${_n(pluma.dx)} ${_n(pluma.dy)} m ');
    for (final tramo in recorridoDeEnlace(linea)) {
      final hasta = tramo.hasta.translate(0, desplazamiento);
      if (tramo.control == null) {
        contenido.write('${_n(hasta.dx)} ${_n(hasta.dy)} l ');
      } else {
        final control = tramo.control!.translate(0, desplazamiento);
        final uno = Offset(pluma.dx + 2 / 3 * (control.dx - pluma.dx),
            pluma.dy + 2 / 3 * (control.dy - pluma.dy));
        final dos = Offset(hasta.dx + 2 / 3 * (control.dx - hasta.dx),
            hasta.dy + 2 / 3 * (control.dy - hasta.dy));
        contenido.write('${_n(uno.dx)} ${_n(uno.dy)} ${_n(dos.dx)} ${_n(dos.dy)} '
            '${_n(hasta.dx)} ${_n(hasta.dy)} c ');
      }
      pluma = hasta;
    }
    contenido.writeln('S');

    // Aqui no hay quien alinee por la derecha: se recorta primero y se estima
    // cuanto mide para restarlo del borde de la caja.
    final etiqueta = _recortar(linea.etiqueta.toUpperCase(), anchoEtiquetaEnlace, 13, true);
    final fin = finDeEtiqueta(linea);
    _textoPdf(contenido, etiqueta, fin.dx - etiqueta.length * 13 * 0.56,
        fin.dy + desplazamiento, 13, true, tinte);
  }
  contenido.writeln('${_colorPdf(plano.colorLinea)} RG');
  contenido.writeln('[] 0 d');

  for (final caja in plano.cajas) {
    final r = caja.rectangulo.translate(0, desplazamiento);
    contenido
      ..writeln('${_colorPdf(caja.color)} rg')
      ..writeln('${_colorPdf(plano.colorLinea)} RG')
      ..writeln('1 w')
      // Borde punteado en lo declarado, por la misma razon que en pantalla: un
      // plano impreso donde lo tecleado se ve igual que lo medido se usa como si
      // todo estuviera comprobado.
      ..writeln(caja.declarada ? '[5 4] 0 d' : '[] 0 d')
      ..writeln(_cajaRedondeadaPdf(r, 8))
      // B rellena y contornea de una pasada; dos operaciones dibujarian el borde
      // dos veces sobre el mismo trazo.
      ..writeln('B');
    _textoPdf(contenido, _recortar(caja.titulo, anchoCaja - 20, 13, true),
        r.left + 10, r.top + 8, 13, true, plano.colorTexto);
    _textoPdf(contenido, _recortar(caja.subtitulo, anchoCaja - 20, 11, false),
        r.left + 10, r.top + 28, 11, false, plano.colorLinea);
  }

  return _armarPdf(contenido.toString(), ancho, alto);
}

/// _cajaRedondeadaPdf traza un rectangulo de esquinas curvas.
///
/// El PDF no tiene rectangulo redondeado: se dibuja con cuatro rectas y cuatro
/// curvas de Bezier. El 0.5523 es la constante que hace que una Bezier cubica se
/// confunda con un cuarto de circulo.
String _cajaRedondeadaPdf(Rect r, double radio) {
  final k = radio * 0.5523;
  final t = StringBuffer()
    ..write('${_n(r.left + radio)} ${_n(r.top)} m ')
    ..write('${_n(r.right - radio)} ${_n(r.top)} l ')
    ..write('${_n(r.right - radio + k)} ${_n(r.top)} '
        '${_n(r.right)} ${_n(r.top + radio - k)} '
        '${_n(r.right)} ${_n(r.top + radio)} c ')
    ..write('${_n(r.right)} ${_n(r.bottom - radio)} l ')
    ..write('${_n(r.right)} ${_n(r.bottom - radio + k)} '
        '${_n(r.right - radio + k)} ${_n(r.bottom)} '
        '${_n(r.right - radio)} ${_n(r.bottom)} c ')
    ..write('${_n(r.left + radio)} ${_n(r.bottom)} l ')
    ..write('${_n(r.left + radio - k)} ${_n(r.bottom)} '
        '${_n(r.left)} ${_n(r.bottom - radio + k)} '
        '${_n(r.left)} ${_n(r.bottom - radio)} c ')
    ..write('${_n(r.left)} ${_n(r.top + radio)} l ')
    ..write('${_n(r.left)} ${_n(r.top + radio - k)} '
        '${_n(r.left + radio - k)} ${_n(r.top)} '
        '${_n(r.left + radio)} ${_n(r.top)} c h');
  return t.toString();
}

void _textoPdf(StringBuffer salida, String texto, double x, double y,
    double tamano, bool negrita, Color color) {
  if (texto.isEmpty) return;
  // Flutter apoya el texto por su borde de arriba; el PDF, por la linea base.
  // Ese 0.8 es lo que sube una Helvetica desde la base hasta la mayuscula.
  final base = y + tamano * 0.8;
  salida
    ..writeln('BT')
    ..writeln('${_colorPdf(color)} rg')
    ..writeln('/${negrita ? "F2" : "F1"} ${_n(tamano)} Tf')
    ..writeln('1 0 0 -1 ${_n(x)} ${_n(base)} Tm')
    ..writeln('${_cadenaPdf(texto)} Tj')
    ..writeln('ET');
}

/// _armarPdf pega los objetos y la tabla de referencias cruzadas.
///
/// La xref lleva el desplazamiento **en bytes** de cada objeto desde el inicio
/// del archivo, y cada renglon mide exactamente 20 bytes. Un byte de mas y el
/// visor da el archivo por corrupto: por eso se va midiendo sobre los bytes ya
/// escritos y no sobre la longitud del texto.
Uint8List _armarPdf(String contenido, double ancho, double alto) {
  final bytes = BytesBuilder();
  final desplazamientos = <int>[];

  void escribir(String texto) => bytes.add(latin1.encode(texto));

  void objeto(int numero, String cuerpo) {
    desplazamientos.add(bytes.length);
    escribir('$numero 0 obj\n$cuerpo\nendobj\n');
  }

  escribir('%PDF-1.4\n');
  // Cuatro bytes altos avisan a las herramientas de que el archivo es binario y
  // no deben tocarle los saltos de linea.
  bytes.add([0x25, 0xE2, 0xE3, 0xCF, 0xD3, 0x0A]);

  objeto(1, '<< /Type /Catalog /Pages 2 0 R >>');
  objeto(2, '<< /Type /Pages /Kids [3 0 R] /Count 1 >>');
  objeto(
      3,
      '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${_n(ancho)} ${_n(alto)}] '
      '/Resources << /Font << /F1 5 0 R /F2 6 0 R >> >> /Contents 4 0 R >>');
  objeto(4,
      '<< /Length ${latin1.encode(contenido).length} >>\nstream\n$contenido\nendstream');
  objeto(5,
      '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>');
  objeto(6,
      '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>');

  final inicioXref = bytes.length;
  final total = desplazamientos.length + 1;
  escribir('xref\n0 $total\n0000000000 65535 f \n');
  for (final desplazamiento in desplazamientos) {
    escribir('${desplazamiento.toString().padLeft(10, '0')} 00000 n \n');
  }
  escribir('trailer\n<< /Size $total /Root 1 0 R >>\nstartxref\n$inicioXref\n%%EOF\n');

  return bytes.toBytes();
}

/// _cadenaPdf convierte un texto de Dart en una cadena literal de PDF.
///
/// Las fuentes van declaradas con WinAnsiEncoding, que abajo de 256 coincide con
/// Latin-1 salvo en un puñado de simbolos. Todo lo que no cabe ahi —un emoji en
/// el alias de un equipo, por ejemplo— se sustituye en vez de romper el archivo.
String _cadenaPdf(String texto) {
  const equivalencias = {
    0x2026: 0x85, // …
    0x2018: 0x91, 0x2019: 0x92, // comillas simples curvas
    0x201C: 0x93, 0x201D: 0x94, // comillas dobles curvas
    0x2022: 0x95, // viñeta
    0x2013: 0x96, 0x2014: 0x97, // guiones largos
    0x20AC: 0x80, // euro
    // Las flechas NO existen en WinAnsi. Sin esto, la etiqueta de un cable
    // saldria "Gi0/1 ? Gi0/24" en el PDF, que parece un dato dudoso cuando en
    // realidad es un caracter que la fuente no tiene. Un guion dice lo mismo.
    0x2192: 0x2D, 0x2190: 0x2D, 0x2194: 0x2D, // → ← ↔
  };

  final salida = StringBuffer('(');
  for (final punto in texto.runes) {
    var byte = punto;
    if (punto > 0xFF) byte = equivalencias[punto] ?? 0x3F; // ?
    if (byte < 32) byte = 0x20;

    if (byte == 0x28 || byte == 0x29 || byte == 0x5C) {
      salida.write('\\${String.fromCharCode(byte)}');
    } else if (byte > 126) {
      salida.write('\\${byte.toRadixString(8).padLeft(3, '0')}');
    } else {
      salida.write(String.fromCharCode(byte));
    }
  }
  return (salida..write(')')).toString();
}

String _colorPdf(Color color) {
  final entero = color.toARGB32();
  final rojo = ((entero >> 16) & 0xFF) / 255;
  final verde = ((entero >> 8) & 0xFF) / 255;
  final azul = (entero & 0xFF) / 255;
  return '${_n(rojo)} ${_n(verde)} ${_n(azul)}';
}

// ------------------------------------------------------- hoja de calculo ---

/// tablasDelMapa arma las DOS tablas del mapa: los aparatos y las conexiones.
///
/// Son dos y no una porque hablan de cosas distintas. Una sola tabla obligaba a
/// meter en el mismo renglon un aparato, un puerto y un cable, y a dejar en
/// blanco lo que no aplicara: la mitad de las celdas vacias querian decir «no
/// aplica» y la otra mitad «no se sabe», sin forma de distinguirlas. Ademas cada
/// cable salia DOS veces, una por cada punta, porque las dos puntas son puertos.
///
/// Las dos salen del mismo [ArbolDeclarado] que dibuja el mapa y **en el mismo
/// orden en que el mapa se lee**: de la raiz hacia afuera, y lo que no se supo
/// ubicar al final. Asi la hoja y el dibujo se recorren igual.
List<Tabla> tablasDelMapa(DatosMapa datos) {
  final arbol = ArbolDeclarado(datos);
  final orden = _aparatosEnOrdenDelMapa(datos, arbol);
  return [
    _tablaDeAparatos(datos, arbol, orden),
    _tablaDeConexiones(datos, arbol, orden),
  ];
}

/// csvDelMapa deja las dos tablas en un texto plano, una tras otra.
///
/// **Empieza diciendo de cuando son los datos**, igual que los otros formatos.
/// Es la regla del proyecto: todo reporte que salga de MiRed dice de que momento
/// es, porque un archivo suelto sin fecha, a la semana, ya no se sabe si sirve.
String csvDelMapa(DatosMapa datos, [EncabezadoMapa? encabezado]) =>
    csvDeTablas(tablasDelMapa(datos), titulo: _tituloDeUnaLinea(encabezado));

/// odsDelMapa y xlsxDelMapa entregan lo mismo con una pestana por tabla.
Uint8List odsDelMapa(DatosMapa datos, EncabezadoMapa encabezado, DateTime momento) =>
    odsDeTablas(tablasDelMapa(datos), momento, titulo: _tituloDeUnaLinea(encabezado));

Uint8List xlsxDelMapa(DatosMapa datos, EncabezadoMapa encabezado, DateTime momento) =>
    xlsxDeTablas(tablasDelMapa(datos), momento, titulo: _tituloDeUnaLinea(encabezado));

String _tituloDeUnaLinea(EncabezadoMapa? encabezado) =>
    encabezado == null ? '' : '${encabezado.titulo} — ${encabezado.subtitulo}';

// --------------------------------------------------- la tabla de aparatos ---

Tabla _tablaDeAparatos(DatosMapa datos, ArbolDeclarado arbol, List<Equipo> orden) =>
    Tabla(
      nombre: 'Aparatos',
      explicacion: 'Un renglon por aparato: que es, de donde cuelga y que tan '
          'seguro es ese dato.',
      encabezados: const [
        'Aparato',
        'Que es',
        'IP',
        'MAC',
        'Ultimo barrido',
        'Conexion',
        'Cuelga de',
        'Puerto',
        'Que tan seguro',
        'Como se supo',
      ],
      filas: [
        for (final equipo in orden) _filaDeAparato(datos, arbol, equipo),
      ],
    );

List<String> _filaDeAparato(DatosMapa datos, ArbolDeclarado arbol, Equipo equipo) {
  final donde = _dondeCuelga(datos, arbol, equipo);
  return [
    equipo.comoSeLlama,
    _queEs(equipo),
    equipo.ip,
    equipo.mac,
    _ultimoBarrido(equipo),
    _comoSeConecta(equipo, donde),
    donde.cuelgaDe,
    donde.puerto,
    donde.seguridad,
    donde.comoSeSupo,
  ];
}

/// _Ubicacion es de donde cuelga un aparato, con la procedencia del dato pegada.
///
/// Van juntos a proposito: decir «cuelga del puerto LAN 2» sin decir si eso lo
/// midio un switch o lo tecleo alguien convierte una suposicion razonable en un
/// hecho, que es como se ensucia un inventario.
class _Ubicacion {
  const _Ubicacion({
    this.cuelgaDe = '',
    this.puerto = '',
    this.seguridad = '',
    this.comoSeSupo = '',
    this.porElAire = false,
    this.porCable = false,
  });

  final String cuelgaDe;
  final String puerto;
  final String seguridad;
  final String comoSeSupo;
  final bool porElAire;
  final bool porCable;
}

_Ubicacion _dondeCuelga(DatosMapa datos, ArbolDeclarado arbol, Equipo equipo) {
  // 1. Por el aire. El WiFi no tiene puertos: se dice de que antena cuelga y por
  //    que red, que es todo lo que hay que saber.
  final aire = datos.topologia.antenaDe(equipo.id);
  if (aire != null) {
    final antena = datos.equipoPorId(aire.antenaId);
    return _Ubicacion(
      cuelgaDe: antena?.comoSeLlama ?? aire.antenaNombre,
      puerto: aire.red.isEmpty ? 'Por el aire' : 'WiFi «${aire.red}»',
      seguridad: aire.senalDbm == null
          ? 'Colgado de la antena'
          : 'Colgado de la antena (${aire.senalDbm} dBm)',
      comoSeSupo: _conMayuscula(aire.comoSeSupo),
      porElAire: true,
    );
  }

  // 2. Por un cable declarado. Se busca el cable que lo une con su padre en el
  //    arbol, que es el mismo que dibuja la linea en el mapa.
  final cable = _cableHaciaSuPadre(datos, arbol, equipo);
  if (cable != null) {
    final padreId = arbol.padreDe(equipo.id) ??
        arbol.otroExtremo(cable.cable, equipo.id);
    final padre = datos.equipoPorId(padreId);
    return _Ubicacion(
      cuelgaDe: padre?.comoSeLlama ?? cable.cable.origenNombre,
      puerto: cable.puertoDelPadre,
      seguridad: 'Puerto exacto',
      comoSeSupo: _comoSeSupoElCable(cable.cable.origenDato),
      porCable: true,
    );
  }

  // 3. Lo que reporto un switch por SNMP.
  for (final puerto in datos.mapa.puertos) {
    if (puerto.equipoId != equipo.id) continue;
    return _Ubicacion(
      cuelgaDe: puerto.switchIp.isEmpty
          ? puerto.switchNombre
          : '${puerto.switchNombre} (${puerto.switchIp})',
      puerto: puerto.puerto,
      seguridad: puerto.confirmado
          ? 'Puerto exacto'
          : 'En grupo con ${puerto.cuantosEnPuerto - 1} mas',
      comoSeSupo: 'SNMP',
      porCable: true,
    );
  }

  // 4. Es la raiz: de el cuelga la red, no cuelga de nadie. No es lo mismo que
  //    «sin ubicar», y confundirlos haria parecer perdido al modem de la casa.
  if (arbol.raices.any((raiz) => raiz.id == equipo.id)) {
    return _Ubicacion(
      cuelgaDe: 'Raiz de la red',
      seguridad: 'Es por donde entra la red',
      comoSeSupo: equipo.origen == 'manual' ? 'Declarado a mano' : 'Escaneo',
    );
  }

  // 5. Se sabe que existe, no de donde cuelga. Se dice, no se esconde: un
  //    inventario que calla lo que no supo ubicar se lee como si estuviera
  //    completo.
  return const _Ubicacion(
    cuelgaDe: 'Sin ubicar',
    seguridad: 'No se sabe donde esta conectado',
  );
}

/// _CableConPuerto es un cable y como se llama el puerto del padre.
class _CableConPuerto {
  const _CableConPuerto(this.cable, this.puertoDelPadre);
  final EnlaceFisico cable;
  final String puertoDelPadre;
}

/// _cableHaciaSuPadre busca por que cable cuelga el aparato del de arriba.
///
/// Un aparato con puertos propios cuelga por uno de los suyos; uno de punta —una
/// PC, una TV— no declaro ninguno, y entonces el cable lo declaro el del otro
/// lado apuntandole al aparato entero.
_CableConPuerto? _cableHaciaSuPadre(
    DatosMapa datos, ArbolDeclarado arbol, Equipo equipo) {
  final padreId = arbol.padreDe(equipo.id);

  for (final cable in datos.topologia.enlaces) {
    final origenEquipo = datos.topologia.equipoDelPuerto(cable.puertoOrigenId);
    final destinoEquipo = cable.puertoDestinoId != null
        ? datos.topologia.equipoDelPuerto(cable.puertoDestinoId!)
        : cable.equipoDestinoId;

    final soyDestino = destinoEquipo == equipo.id;
    final soyOrigen = origenEquipo == equipo.id;
    if (!soyDestino && !soyOrigen) continue;

    final otro = soyDestino ? origenEquipo : destinoEquipo;
    if (padreId != null) {
      // Con padre conocido se exige que sea EL cable que lo une a el.
      if (otro != padreId) continue;
    } else if (!soyDestino) {
      // Sin padre en el arbol solo cuelga de un cable que le APUNTE: es un
      // aparato de punta y el cable lo declaro el del otro lado. Un cable que
      // SALE de el no lo cuelga de nadie —lo cuelga a el del otro—, y tomarlo
      // por bueno dejaba al modem de la casa «colgando» del switch que alimenta,
      // con el mapa dibujando exactamente lo contrario.
      continue;
    }

    final puertoDelPadre = datos.topologia
        .puertoPorId(soyDestino ? cable.puertoOrigenId : cable.puertoDestinoId);
    if (puertoDelPadre == null) continue;
    return _CableConPuerto(cable, puertoDelPadre.etiqueta);
  }
  return null;
}

/// _aparatosEnOrdenDelMapa recorre el arbol como se lee el dibujo.
///
/// De la raiz hacia afuera y, en cada aparato, primero lo que cuelga de sus
/// puertos y despues lo que cuelga por el aire, que es el mismo orden en que el
/// plano apila las cajas. Lo que no esta en el arbol va detras, y lo que no se
/// supo ubicar, al final del todo.
List<Equipo> _aparatosEnOrdenDelMapa(DatosMapa datos, ArbolDeclarado arbol) {
  final vistos = <int>{};
  final orden = <Equipo>[];

  void recorrer(Equipo equipo) {
    if (!vistos.add(equipo.id)) return;
    orden.add(equipo);

    for (final puerto in datos.topologia.puertosDe(equipo.id)) {
      final cable = datos.topologia.enlaceDe(puerto.id);
      if (cable == null) continue;
      final otro = arbol.otroExtremo(cable, equipo.id);
      if (otro == null) continue;
      // Cuelga de este si el arbol lo dice, o si es un aparato de punta —sin
      // puertos propios— que no es cabecera de nada.
      if (arbol.padreDe(otro) != equipo.id && arbol.esCabecera(otro)) continue;
      final hijo = datos.equipoPorId(otro);
      if (hijo != null) recorrer(hijo);
    }

    for (final cliente in arbol.clientesDe(equipo.id)) {
      final hijo = datos.equipoPorId(cliente.equipoId);
      if (hijo != null) recorrer(hijo);
    }
  }

  for (final raiz in arbol.raices) {
    recorrer(raiz);
  }

  final sinUbicar = datos.sinUbicar.map((equipo) => equipo.id).toSet();
  for (final equipo in datos.equipos) {
    if (vistos.contains(equipo.id) || sinUbicar.contains(equipo.id)) continue;
    vistos.add(equipo.id);
    orden.add(equipo);
  }
  for (final equipo in datos.sinUbicar) {
    if (vistos.add(equipo.id)) orden.add(equipo);
  }
  return orden;
}

// ------------------------------------------------- la tabla de conexiones ---

/// _tablaDeConexiones saca un renglon por conexion, **cada cable una sola vez**.
///
/// Antes cada cable salia dos veces, una por punta, porque la tabla recorria
/// puertos: de cinco cables salian ocho renglones y no habia como saber que
/// «dvr → switch» y «switch → dvr» eran el mismo. Aqui el cable se apunta cuando
/// se emite y no se repite; como el recorrido va de la raiz hacia afuera, la
/// punta que queda a la izquierda es siempre la de arriba, igual que en el mapa.
///
/// Los puertos libres tambien salen: son la mitad de la utilidad de esta hoja
/// cuando hay que conectar algo nuevo.
Tabla _tablaDeConexiones(
    DatosMapa datos, ArbolDeclarado arbol, List<Equipo> orden) {
  final filas = <List<String>>[];
  final cablesPuestos = <int>{};
  final puertosSnmpPuestos = <String>{};

  void puertosSnmpDe(int equipoId) {
    for (final puerto in datos.mapa.puertos) {
      if (puerto.switchId != equipoId) continue;
      final clave = '${puerto.switchId}|${puerto.indice}|${puerto.mac}';
      if (!puertosSnmpPuestos.add(clave)) continue;
      filas.add(_filaSnmp(puerto));
    }
  }

  for (final equipo in orden) {
    // Los puertos que alguien conto mirando el aparato.
    for (final puerto in datos.topologia.puertosDe(equipo.id)) {
      final cable = datos.topologia.enlaceDe(puerto.id);
      if (cable == null) {
        filas.add([
          equipo.comoSeLlama,
          puerto.etiqueta,
          '',
          '',
          'Libre',
          _velocidad(puerto.velocidadMbps),
          'Declarado a mano',
        ]);
        continue;
      }
      if (!cablesPuestos.add(cable.id)) continue; // ya salio por la otra punta

      final otroId = arbol.otroExtremo(cable, equipo.id);
      final otro = datos.equipoPorId(otroId);
      final puertoDelOtro = datos.topologia.puertoPorId(
          cable.puertoOrigenId == puerto.id
              ? cable.puertoDestinoId
              : cable.puertoOrigenId);
      filas.add([
        equipo.comoSeLlama,
        puerto.etiqueta,
        otro?.comoSeLlama ??
            (cable.puertoOrigenId == puerto.id
                ? cable.destinoNombre
                : cable.origenNombre),
        // Sin puerto declarado al otro lado, un aparato de punta entra por su
        // unica toma. No es suponer: se conecta por un cable.
        puertoDelOtro?.etiqueta ??
            (otroId != null && datos.topologia.puertosDe(otroId).isEmpty
                ? puertoUnicoDeUnEquipoFinal
                : ''),
        'Ocupado',
        _velocidad(puerto.velocidadMbps),
        _comoSeSupoElCable(cable.origenDato),
      ]);
    }

    // Lo que cuelga de este aparato por el aire.
    for (final cliente in arbol.clientesDe(equipo.id)) {
      final colgado = datos.equipoPorId(cliente.equipoId);
      filas.add([
        equipo.comoSeLlama,
        cliente.red.isEmpty ? 'Por el aire' : 'WiFi «${cliente.red}»',
        colgado?.comoSeLlama ?? cliente.equipoNombre,
        '',
        cliente.senalDbm == null
            ? 'Por el aire'
            : 'Por el aire (${cliente.senalDbm} dBm)',
        '',
        _conMayuscula(cliente.comoSeSupo),
      ]);
    }

    puertosSnmpDe(equipo.id);
  }

  // Los switches que solo conoce el SNMP y que no entraron en el recorrido.
  for (final puerto in datos.mapa.puertos) {
    final clave = '${puerto.switchId}|${puerto.indice}|${puerto.mac}';
    if (puertosSnmpPuestos.contains(clave)) continue;
    puertosSnmpPuestos.add(clave);
    filas.add(_filaSnmp(puerto));
  }

  // Los cables entre switches que se anunciaron por LLDP o CDP.
  for (final enlace in datos.mapa.enlacesUnicos) {
    filas.add([
      enlace.enlace.equipoNombre,
      enlace.enlace.interfazLocal,
      enlace.enlace.vecinoNombre,
      enlace.enlace.vecinoPuerto,
      'Enlace entre switches',
      '',
      enlace.origenes.map((origen) => origen.toUpperCase()).join(' y '),
    ]);
  }

  return Tabla(
    nombre: 'Conexiones',
    explicacion: 'Un renglon por conexion, cada cable una sola vez. Los puertos '
        'libres tambien salen.',
    encabezados: const [
      'De',
      'Por',
      'A',
      'Entra por',
      'Estado',
      'Velocidad',
      'Como se supo',
    ],
    filas: filas,
  );
}

List<String> _filaSnmp(PuertoDeSwitch puerto) => [
      puerto.switchNombre,
      puerto.puerto,
      // Cuando el switch reporta una MAC que no es de ningun aparato conocido se
      // dice asi, con la MAC a la vista: es el hilo del que se tira para
      // averiguar que hay ahi.
      puerto.equipoNombre.isNotEmpty
          ? puerto.equipoNombre
          : (puerto.mac.isEmpty ? '' : 'Sin identificar (${puerto.mac})'),
      '',
      puerto.confirmado
          ? 'Ocupado'
          : 'Compartido por ${puerto.cuantosEnPuerto} aparatos',
      _velocidad(puerto.velocidadMbps),
      'SNMP',
    ];

// ------------------------------------------------------------------ apoyo ---

/// _queEs dice que clase de aparato es, en cristiano.
///
/// Primero lo que se reconocio («Impresora HP»), y si no, la categoria de la
/// lista unica («Impresora»). Nunca la clave interna: `punto_acceso` no es
/// castellano.
String _queEs(Equipo equipo) {
  if (equipo.tipo.isNotEmpty) return equipo.tipo;
  final categoria = buscarCategoria(equipo.categoria);
  if (categoria != null) return _conMayuscula(categoria.singular);
  return equipo.fabricante;
}

/// _ultimoBarrido no dice «en linea»: dice si contesto la ultima vez que se
/// miro.
///
/// La diferencia importa cuando el archivo se lee tres semanas despues. Y un
/// aparato declarado a mano al que nunca se le paso un barrido no «no
/// responde»: es que nadie le ha preguntado.
String _ultimoBarrido(Equipo equipo) {
  if (equipo.presente) return 'Respondio';
  if (equipo.origen == 'manual' && equipo.ultimaVez.isEmpty) return 'Sin comprobar';
  return 'No respondio';
}

String _comoSeConecta(Equipo equipo, _Ubicacion donde) {
  if (donde.porElAire || equipo.conexion == 'wifi') return 'WiFi';
  if (donde.porCable || equipo.conexion == 'cable') return 'Cable';
  return '';
}

/// _comoSeSupoElCable traduce la procedencia del dato.
///
/// Lo tecleado y lo medido no se presentan igual: eso es lo que separa un
/// inventario de una suposicion.
String _comoSeSupoElCable(String origenDato) {
  switch (origenDato) {
    case 'manual':
      return 'Declarado a mano';
    case 'snmp':
      return 'SNMP';
    case 'lldp':
      return 'LLDP';
    case 'cdp':
      return 'CDP';
    case 'inferido':
      return 'Deducido por conteo de MAC';
    default:
      return _conMayuscula(origenDato);
  }
}

String _velocidad(int? mbps) {
  if (mbps == null || mbps <= 0) return '';
  if (mbps % 1000 == 0) return '${mbps ~/ 1000} Gbps';
  return '$mbps Mbps';
}

String _conMayuscula(String texto) =>
    texto.isEmpty ? texto : texto[0].toUpperCase() + texto.substring(1);

// ------------------------------------------------------------------ apoyo ---

/// _n escribe un numero sin cola de decimales inutiles.
///
/// Sin esto, un PDF de cien cajas se llena de "190.00000000000003" y crece de
/// balde.
String _n(double valor) {
  final redondeado = (valor * 100).round() / 100;
  if (redondeado == redondeado.roundToDouble()) return redondeado.toInt().toString();
  return redondeado.toString();
}

String _color(Color color) =>
    '#${(color.toARGB32() & 0xFFFFFF).toRadixString(16).padLeft(6, '0')}';

String _xml(String texto) => texto
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;');

/// _recortar corta el texto que no cabe en la caja y le pone puntos suspensivos.
///
/// En pantalla lo hace Flutter, que mide la fuente de verdad. Aqui no hay quien
/// mida —el SVG lo pinta el visor y el PDF, la Helvetica del lector—, asi que se
/// estima por ancho medio de caracter. Sobra para que un nombre largo no se
/// derrame sobre la caja de al lado, que es de lo que se trata.
String _recortar(String texto, double anchoDisponible, double tamano, bool negrita) {
  final anchoPorLetra = tamano * (negrita ? 0.56 : 0.52);
  final caben = (anchoDisponible / anchoPorLetra).floor();
  if (texto.length <= caben) return texto;
  if (caben <= 1) return '…';
  return '${texto.substring(0, caben - 1)}…';
}
