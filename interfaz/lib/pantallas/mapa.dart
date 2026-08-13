import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../servicios/descarga.dart';
import '../servicios/exportar_mapa.dart';
import '../widgets/mensajes.dart';
import 'mapa_plano.dart';

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
///
/// El calculo de posiciones y el pintor viven en mapa_plano.dart, porque los
/// comparte con la exportacion.
class PantallaMapa extends StatefulWidget {
  const PantallaMapa({super.key, required this.red});

  final Red red;

  @override
  State<PantallaMapa> createState() => _PantallaMapaState();
}

enum _Formato { png, svg, pdf, csv, portapapeles }

class _PantallaMapaState extends State<PantallaMapa> {
  late Future<DatosMapa> _datos;
  bool _exportando = false;

  @override
  void initState() {
    super.initState();
    _datos = _cargar();
  }

  Future<DatosMapa> _cargar() async {
    final mapa = await Api.instancia.mapaDePuertos(widget.red.clave);
    final equipos = await Api.instancia.listarEquipos(widget.red.clave);
    return DatosMapa(mapa: mapa, equipos: equipos);
  }

  /// _exportar arma el archivo y se lo entrega al navegador.
  ///
  /// El plano de la exportacion se arma con los colores fijos de exportar, no
  /// con los del tema: lo que se guarda o se imprime va siempre sobre blanco,
  /// aunque quien lo exporto tenga la pantalla en oscuro.
  Future<void> _exportar(_Formato formato, DatosMapa datos) async {
    setState(() => _exportando = true);
    try {
      final momento = DateTime.now();
      final sello = momento.toIso8601String().substring(0, 16).replaceAll(':', '');
      final base = 'mapa-${widget.red.clave}-$sello';

      if (formato == _Formato.portapapeles) {
        await Clipboard.setData(ClipboardData(text: csvDelMapa(datos)));
        if (mounted) {
          mensajeAviso(context,
              'Mapa copiado como CSV: ya se puede pegar en una hoja de calculo.');
        }
        return;
      }

      final plano = armarPlano(datos, coloresParaExportar);
      final encabezado = EncabezadoMapa(
        titulo: 'Mapa de ${widget.red.nombre}',
        subtitulo: '${datos.mapa.explicacion}  ·  Exportado el '
            '${momento.toIso8601String().substring(0, 19).replaceFirst('T', ' ')}'
            '  ·  ${Api.instancia.version}',
      );

      var donde = '';
      switch (formato) {
        case _Formato.png:
          donde = await descargarArchivo(
              '$base.png', 'image/png', await pngDelPlano(plano, encabezado));
        case _Formato.svg:
          donde = await descargarArchivo('$base.svg', 'image/svg+xml',
              Uint8List.fromList(utf8.encode(svgDelPlano(plano, encabezado))));
        case _Formato.pdf:
          donde = await descargarArchivo(
              '$base.pdf', 'application/pdf', pdfDelPlano(plano, encabezado));
        case _Formato.csv:
          donde = await descargarArchivo('$base.csv', 'text/csv;charset=utf-8',
              Uint8List.fromList(utf8.encode(csvDelMapa(datos))));
        case _Formato.portapapeles:
          break; // Resuelto arriba.
      }

      // En el programa de escritorio se dice DONDE quedo. Un archivo guardado en
      // un sitio que el usuario no sabe cual es no sirve de nada; en web no hace
      // falta, porque el navegador ya lo anuncia a su manera.
      if (donde.isNotEmpty && mounted) {
        mensajeAviso(context, 'Guardado en $donde');
      }
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _exportando = false);
    }
  }

  @override
  Widget build(BuildContext contexto) {
    return Scaffold(
      appBar: AppBar(
        title: Text('Mapa de ${widget.red.nombre}'),
        actions: [
          FutureBuilder<DatosMapa>(
            future: _datos,
            builder: (_, resultado) => PopupMenuButton<_Formato>(
              tooltip: 'Exportar el mapa',
              enabled: resultado.hasData && !_exportando,
              icon: _exportando
                  ? const SizedBox(
                      height: 18, width: 18,
                      child: CircularProgressIndicator(strokeWidth: 2))
                  : const Icon(Icons.download_outlined),
              onSelected: (formato) => _exportar(formato, resultado.data!),
              itemBuilder: (_) => const [
                PopupMenuItem(
                  value: _Formato.png,
                  child: ListTile(
                    dense: true,
                    leading: Icon(Icons.image_outlined),
                    title: Text('PNG'),
                    subtitle: Text('Imagen, para pegar en un documento'),
                  ),
                ),
                PopupMenuItem(
                  value: _Formato.svg,
                  child: ListTile(
                    dense: true,
                    leading: Icon(Icons.polyline_outlined),
                    title: Text('SVG'),
                    subtitle: Text('Vectorial, para retocar el plano'),
                  ),
                ),
                PopupMenuItem(
                  value: _Formato.pdf,
                  child: ListTile(
                    dense: true,
                    leading: Icon(Icons.picture_as_pdf_outlined),
                    title: Text('PDF'),
                    subtitle: Text('Para imprimir o mandar por correo'),
                  ),
                ),
                PopupMenuDivider(),
                PopupMenuItem(
                  value: _Formato.csv,
                  child: ListTile(
                    dense: true,
                    leading: Icon(Icons.table_view_outlined),
                    title: Text('CSV'),
                    subtitle: Text('Para una hoja de calculo'),
                  ),
                ),
                PopupMenuItem(
                  value: _Formato.portapapeles,
                  child: ListTile(
                    dense: true,
                    leading: Icon(Icons.copy_outlined),
                    title: Text('Copiar CSV'),
                    subtitle: Text('Al portapapeles, sin bajar archivo'),
                  ),
                ),
              ],
            ),
          ),
          IconButton(
            tooltip: 'Actualizar',
            icon: const Icon(Icons.refresh),
            onPressed: () => setState(() {
              _datos = _cargar();
            }),
          ),
        ],
      ),
      body: FutureBuilder<DatosMapa>(
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

          final plano = armarPlano(datos, Theme.of(contexto).colorScheme);

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
                    painter: PintorMapa(plano: plano),
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
          if (mapa.enlacesUnicos.isNotEmpty)
            _Marca(
              texto: 'Arco: cable entre switches, anunciado por '
                  '${mapa.enlaces.map((e) => e.origen.toUpperCase()).toSet().join(' y ')}',
              punteada: false,
              color: colores.primary,
            ),
        ],
      ),
    );
  }
}

class _Marca extends StatelessWidget {
  const _Marca({required this.texto, required this.punteada, this.color});

  final String texto;
  final bool punteada;
  final Color? color;

  @override
  Widget build(BuildContext contexto) => Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(punteada ? Icons.more_horiz : Icons.remove, size: 18, color: color),
          const SizedBox(width: 4),
          Text(texto,
              style: Theme.of(contexto).textTheme.labelSmall?.copyWith(color: color)),
        ],
      );
}
