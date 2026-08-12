import 'dart:math' as matematicas;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../widgets/mensajes.dart';

/// PantallaMapa dibuja la red: los switches arriba y lo que cuelga de cada boca
/// debajo.
///
/// Dos reglas de dibujo que son el corazon del proyecto:
///  - Lo **confirmado** (una sola MAC en la boca) va con linea llena.
///  - Lo **inferido** (varias MAC en la misma boca, o sea un switch tonto atras)
///    va con linea punteada y agrupado. Dibujarlo igual que lo confirmado seria
///    mentir sobre lo que se sabe.
///  - Lo que no cuelga de ningun switch conocido NO se esconde: va aparte, en su
///    propia columna, dicho con todas sus letras.
class PantallaMapa extends StatefulWidget {
  const PantallaMapa({super.key, required this.red});

  final Red red;

  @override
  State<PantallaMapa> createState() => _PantallaMapaState();
}

class _PantallaMapaState extends State<PantallaMapa> {
  late Future<_DatosMapa> _datos;

  @override
  void initState() {
    super.initState();
    _datos = _cargar();
  }

  Future<_DatosMapa> _cargar() async {
    final mapa = await Api.instancia.mapaDePuertos(widget.red.clave);
    final equipos = await Api.instancia.listarEquipos(widget.red.clave);
    return _DatosMapa(mapa: mapa, equipos: equipos);
  }

  Future<void> _exportar(_DatosMapa datos) async {
    final renglones = <String>[
      'switch,ip_switch,puerto,equipo,ip_equipo,mac,certeza,equipos_en_la_boca',
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
        '${puerto.cuantosEnBoca}',
      ].map(_paraCsv).join(','));
    }
    for (final equipo in datos.sinUbicar) {
      renglones.add([
        '', '', '', equipo.comoSeLlama, equipo.ip, equipo.mac, 'sin ubicar', '',
      ].map(_paraCsv).join(','));
    }

    final csv = renglones.join('\n');
    await Clipboard.setData(ClipboardData(text: csv));
    if (mounted) {
      mensajeAviso(context, 'Mapa copiado como CSV: ya se puede pegar en una hoja de calculo.');
    }
  }

  static String _paraCsv(String valor) =>
      valor.contains(',') || valor.contains('"')
          ? '"${valor.replaceAll('"', '""')}"'
          : valor;

  @override
  Widget build(BuildContext contexto) {
    return Scaffold(
      appBar: AppBar(
        title: Text('Mapa de ${widget.red.nombre}'),
        actions: [
          FutureBuilder<_DatosMapa>(
            future: _datos,
            builder: (_, resultado) => IconButton(
              tooltip: 'Copiar como CSV',
              icon: const Icon(Icons.table_view_outlined),
              onPressed: resultado.hasData ? () => _exportar(resultado.data!) : null,
            ),
          ),
          IconButton(
            tooltip: 'Actualizar',
            icon: const Icon(Icons.refresh),
            onPressed: () => setState(() => _datos = _cargar()),
          ),
        ],
      ),
      body: FutureBuilder<_DatosMapa>(
        future: _datos,
        builder: (_, resultado) {
          if (resultado.connectionState != ConnectionState.done) {
            return const Center(child: CircularProgressIndicator());
          }
          if (resultado.hasError) {
            return Center(
              child: TextButton(
                onPressed: () => mostrarProblema(contexto, resultado.error!),
                child: const Text('No se pudo armar el mapa. Ver detalles'),
              ),
            );
          }

          final datos = resultado.data!;
          if (datos.equipos.isEmpty) {
            return const Center(
              child: Padding(
                padding: EdgeInsets.all(32),
                child: Text('Todavia no hay equipos que dibujar. Corra un escaneo.',
                    textAlign: TextAlign.center),
              ),
            );
          }

          final plano = _armarPlano(datos, Theme.of(contexto).colorScheme);

          return Column(
            children: [
              _Leyenda(mapa: datos.mapa),
              Expanded(
                child: InteractiveViewer(
                  constrained: false,
                  minScale: 0.2,
                  maxScale: 3,
                  boundaryMargin: const EdgeInsets.all(200),
                  child: CustomPaint(
                    size: plano.tamano,
                    painter: _PintorMapa(plano: plano),
                  ),
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _DatosMapa {
  const _DatosMapa({required this.mapa, required this.equipos});

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

// --------------------------------------------------------------- el plano ---

class _Caja {
  _Caja({
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

class _Linea {
  _Linea({required this.desde, required this.hasta, required this.confirmada, required this.etiqueta});

  final Offset desde;
  final Offset hasta;
  final bool confirmada;
  final String etiqueta;
}

class _Plano {
  _Plano({
    required this.cajas,
    required this.lineas,
    required this.tamano,
    required this.colorLinea,
    required this.colorTexto,
  });

  final List<_Caja> cajas;
  final List<_Linea> lineas;
  final Size tamano;
  final Color colorLinea;
  final Color colorTexto;
}

const double _anchoCaja = 190;
const double _altoCaja = 54;
const double _separacionX = 30;
const double _separacionY = 130;

/// _armarPlano coloca todo: una columna por switch, y debajo de cada uno sus
/// bocas con lo que cuelga. Es un arbol por niveles, que para un plano de sitio
/// se lee mucho mejor que una maraña de nodos flotando.
_Plano _armarPlano(_DatosMapa datos, ColorScheme colores) {
  final cajas = <_Caja>[];
  final lineas = <_Linea>[];

  // Agrupar por switch y, dentro, por boca.
  final porSwitch = <int, Map<int, List<PuertoDeSwitch>>>{};
  for (final puerto in datos.mapa.puertos) {
    porSwitch
        .putIfAbsent(puerto.switchId, () => {})
        .putIfAbsent(puerto.indice, () => [])
        .add(puerto);
  }

  double x = _separacionX;
  double anchoMaximo = 0;
  double altoMaximo = 0;

  porSwitch.forEach((switchId, bocas) {
    final ejemplo = bocas.values.first.first;
    final anchoBloque =
        matematicas.max(bocas.length * (_anchoCaja + _separacionX), _anchoCaja + _separacionX);

    final centroSwitch = Offset(x + anchoBloque / 2, _separacionY / 2);
    cajas.add(_Caja(
      rectangulo: Rect.fromCenter(
          center: centroSwitch, width: _anchoCaja, height: _altoCaja),
      titulo: ejemplo.switchNombre,
      subtitulo: ejemplo.switchIp,
      color: colores.primaryContainer,
      icono: Icons.router,
    ));

    double xBoca = x;
    bocas.forEach((indice, enLaBoca) {
      final confirmado = enLaBoca.length == 1 && enLaBoca.first.confirmado;
      final centroBoca = Offset(xBoca + _anchoCaja / 2, _separacionY / 2 + _separacionY);

      if (confirmado) {
        final unico = enLaBoca.first;
        cajas.add(_Caja(
          rectangulo:
              Rect.fromCenter(center: centroBoca, width: _anchoCaja, height: _altoCaja),
          titulo: unico.quienEs,
          subtitulo: unico.equipoIp.isNotEmpty ? unico.equipoIp : unico.mac,
          color: colores.surfaceContainerHighest,
          icono: Icons.devices,
        ));
      } else {
        // El grupo se dibuja como UNA caja que dice cuantos hay: es exactamente
        // lo que se sabe, ni mas ni menos.
        cajas.add(_Caja(
          rectangulo:
              Rect.fromCenter(center: centroBoca, width: _anchoCaja, height: _altoCaja),
          titulo: '${enLaBoca.length} equipos',
          subtitulo: 'tras algo no administrable',
          color: colores.tertiaryContainer,
          icono: Icons.hub,
        ));
      }

      lineas.add(_Linea(
        desde: centroSwitch + const Offset(0, _altoCaja / 2),
        hasta: centroBoca - const Offset(0, _altoCaja / 2),
        confirmada: confirmado,
        etiqueta: enLaBoca.first.puerto,
      ));

      xBoca += _anchoCaja + _separacionX;
    });

    x += anchoBloque + _separacionX * 2;
    anchoMaximo = matematicas.max(anchoMaximo, x);
    altoMaximo = matematicas.max(altoMaximo, _separacionY * 2 + _altoCaja);
  });

  // Los que no cuelgan de ningun switch conocido: en su propia zona, abajo.
  final sinUbicar = datos.sinUbicar;
  if (sinUbicar.isNotEmpty) {
    final yBase = altoMaximo + _separacionY;
    double xSuelto = _separacionX;
    double filaY = yBase;
    final porFila = matematicas.max(1, (anchoMaximo / (_anchoCaja + _separacionX)).floor());

    for (var i = 0; i < sinUbicar.length; i++) {
      final equipo = sinUbicar[i];
      if (i > 0 && i % porFila == 0) {
        xSuelto = _separacionX;
        filaY += _altoCaja + 20;
      }
      cajas.add(_Caja(
        rectangulo: Rect.fromLTWH(xSuelto, filaY, _anchoCaja, _altoCaja),
        titulo: equipo.comoSeLlama,
        subtitulo: equipo.ip,
        color: colores.surfaceContainerLow,
        icono: equipo.presente ? Icons.help_outline : Icons.power_off,
      ));
      xSuelto += _anchoCaja + _separacionX;
      altoMaximo = matematicas.max(altoMaximo, filaY + _altoCaja + _separacionY / 2);
    }
  }

  return _Plano(
    cajas: cajas,
    lineas: lineas,
    tamano: Size(matematicas.max(anchoMaximo + _separacionX, 800),
        matematicas.max(altoMaximo + _separacionY, 600)),
    colorLinea: colores.outline,
    colorTexto: colores.onSurface,
  );
}

class _PintorMapa extends CustomPainter {
  _PintorMapa({required this.plano});

  final _Plano plano;

  @override
  void paint(Canvas lienzo, Size tamano) {
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
          negrita: true, ancho: _anchoCaja - 44);
      _texto(lienzo, caja.subtitulo, caja.rectangulo.topLeft + const Offset(34, 28), 11,
          plano.colorLinea,
          ancho: _anchoCaja - 44);
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
  bool shouldRepaint(covariant _PintorMapa anterior) => anterior.plano != plano;
}

class _Leyenda extends StatelessWidget {
  const _Leyenda({required this.mapa});

  final MapaPuertos mapa;

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      color: colores.surfaceContainerHighest,
      child: Wrap(
        spacing: 20,
        runSpacing: 8,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          Text(mapa.explicacion, style: Theme.of(contexto).textTheme.bodySmall),
          const _Marca(texto: 'Linea llena: puerto confirmado', punteada: false),
          const _Marca(texto: 'Linea punteada: grupo tras algo no administrable', punteada: true),
        ],
      ),
    );
  }
}

class _Marca extends StatelessWidget {
  const _Marca({required this.texto, required this.punteada});

  final String texto;
  final bool punteada;

  @override
  Widget build(BuildContext contexto) => Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(punteada ? Icons.more_horiz : Icons.remove, size: 18),
          const SizedBox(width: 4),
          Text(texto, style: Theme.of(contexto).textTheme.labelSmall),
        ],
      );
}
