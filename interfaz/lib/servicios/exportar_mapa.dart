import 'dart:convert';
import 'dart:typed_data';
import 'dart:ui' as dibujo;

import 'package:flutter/material.dart';

import '../pantallas/mapa_plano.dart';

/// Este archivo convierte el plano del mapa en los cuatro formatos que se
/// entregan: PNG, SVG, PDF y CSV.
///
/// Los tres primeros salen del **mismo plano** que se ve en pantalla, no de un
/// recalculo aparte, para que el archivo guardado y la pantalla no puedan
/// discrepar. El CSV es el unico que sale de los datos crudos, porque una hoja
/// de calculo no quiere posiciones sino renglones.
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

// -------------------------------------------------------------------- CSV ---

/// csvDelMapa saca los renglones para una hoja de calculo.
///
/// Los equipos sin ubicar van tambien, marcados como tales. Un inventario que
/// calla lo que no supo ubicar se lee como si estuviera completo.
///
/// **Empieza diciendo de cuando son los datos**, igual que los otros tres
/// formatos. Es la regla del proyecto: todo reporte que salga de MiRed dice de
/// que momento es, porque un archivo suelto sin fecha, a la semana, ya no se
/// sabe si sirve. Va en su propio renglon y con un renglon en blanco detras, para
/// que la hoja de calculo siga encontrando los encabezados como una fila.
String csvDelMapa(DatosMapa datos, [EncabezadoMapa? encabezado]) {
  final renglones = <String>[
    if (encabezado != null) ...[
      _paraCsv('${encabezado.titulo} — ${encabezado.subtitulo}'),
      '',
    ],
    'switch,ip_switch,puerto,equipo,ip_equipo,mac,certeza,equipos_en_el_puerto,origen_del_dato',
  ];
  for (final puerto in datos.mapa.puertos) {
    renglones.add([
      puerto.switchNombre,
      puerto.switchIp,
      puerto.puerto,
      puerto.equipoNombre,
      puerto.equipoIp,
      puerto.mac,
      puerto.confirmado ? 'confirmado' : 'grupo',
      '${puerto.cuantosEnPuerto}',
      'snmp',
    ].map(_paraCsv).join(','));
  }

  // Lo declarado a mano va con su propia columna de origen. Mezclarlo con lo
  // medido sin decir cual es cual convertiria la hoja en un inventario que
  // parece comprobado y no lo esta.
  for (final puerto in datos.topologia.puertos) {
    final equipo = datos.equipoPorId(puerto.equipoId);
    final cable = datos.topologia.enlaceDe(puerto.id);
    final soyOrigen = cable != null && cable.puertoOrigenId == puerto.id;
    final otroLado = cable == null
        ? ''
        : (soyOrigen ? cable.destinoNombre : cable.origenNombre);
    final destino = datos.equipoPorId(
        cable == null ? null : (soyOrigen ? cable.equipoDestinoId : cable.equipoOrigenId));

    renglones.add([
      equipo?.comoSeLlama ?? '',
      equipo?.ip ?? '',
      puerto.tipo == 'wan' ? 'WAN' : 'puerto ${puerto.numero}',
      otroLado,
      destino?.ip ?? '',
      destino?.mac ?? '',
      cable == null ? 'puerto libre' : 'declarado a mano',
      '',
      cable?.origenDato ?? 'manual',
    ].map(_paraCsv).join(','));
  }

  for (final equipo in datos.sinUbicar) {
    renglones.add([
      '', '', '', equipo.comoSeLlama, equipo.ip, equipo.mac, 'sin ubicar', '', '',
    ].map(_paraCsv).join(','));
  }
  return renglones.join('\n');
}

String _paraCsv(String valor) => valor.contains(',') || valor.contains('"')
    ? '"${valor.replaceAll('"', '""')}"'
    : valor;

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
