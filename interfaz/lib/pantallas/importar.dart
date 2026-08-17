import 'dart:convert';
import 'dart:typed_data';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../servicios/descarga.dart';
import '../widgets/mensajes.dart';

/// PantallaImportar sube una hoja de aparatos a una red.
///
/// Nace de un caso real: la hoja de cableado de una clinica —23 rosetas, una
/// columna por puerto, nodo, ubicacion y observaciones— que llevaba anos en una
/// hoja de calculo. Capturar eso aparato por aparato son 23 formularios.
///
/// Es una pantalla completa y no un cuadro de dialogo **porque la guia va
/// dentro**: quien llena la hoja la tiene delante mientras la llena, sin ir a
/// buscar un manual aparte. Y la guia se dibuja de lo que manda el servidor, no
/// de una copia escrita aqui: si se agrega una columna, la guia se entera sola.
///
/// **Son dos pasos y el primero no escribe nada.** Se elige el archivo, se ve
/// renglon por renglon lo que pasaria, y solo entonces se importa. Importar a
/// ciegas y descubrir despues que tres renglones estaban mal significa borrarlos
/// a mano de uno en uno.
/// anchoDeLectura es lo que mide un parrafo comodo de leer.
///
/// Unos noventa caracteres. Mas ancho y el ojo pierde el renglon al volver a la
/// izquierda; es la razon por la que un periodico va en columnas.
const double anchoDeLectura = 900;

class PantallaImportar extends StatefulWidget {
  const PantallaImportar({super.key, required this.red});

  final Red red;

  @override
  State<PantallaImportar> createState() => _PantallaImportarState();
}

class _PantallaImportarState extends State<PantallaImportar> {
  PlantillaImportacion? _plantilla;

  String _nombreArchivo = '';
  Uint8List? _archivo;
  PlanImportacion? _plan;
  ResumenImportacion? _resumen;

  /// Que hacer con los aparatos que ya existen. Arranca en actualizar: quien
  /// vuelve a subir la hoja casi siempre es porque corrigio un dato en ella.
  String _repetidos = 'actualizar';
  bool _trabajando = false;
  bool _huboCambios = false;

  @override
  void initState() {
    super.initState();
    _cargarGuia();
  }

  /// _cargarGuia trae la plantilla y como se llena.
  ///
  /// Si falla no se monta un modal: la pantalla sirve igual para subir un
  /// archivo, solo se queda sin la tabla de columnas. Un error aqui no puede
  /// impedir el trabajo.
  Future<void> _cargarGuia() async {
    try {
      final plantilla = await Api.instancia.plantillaDeImportacion(widget.red.clave);
      if (mounted) setState(() => _plantilla = plantilla);
    } catch (_) {
      // Sin guia, pero la pantalla sigue en pie.
    }
  }

  Future<void> _descargarPlantilla() async {
    setState(() => _trabajando = true);
    try {
      final plantilla =
          _plantilla ?? await Api.instancia.plantillaDeImportacion(widget.red.clave);
      final donde = await descargarArchivo(plantilla.nombre, 'text/csv;charset=utf-8',
          Uint8List.fromList(utf8.encode(plantilla.contenido)));
      if (donde.isNotEmpty && mounted) {
        mensajeAviso(context, 'Plantilla guardada en $donde');
      }
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _trabajando = false);
    }
  }

  Future<void> _elegirArchivo() async {
    // Se aceptan los tres formatos que la gente tiene: el CSV y las dos hojas de
    // calculo. Asi nadie tiene que pasar por «guardar como CSV», que es donde se
    // equivoca de separador y acaba con todo en una columna.
    const tipos = XTypeGroup(
      label: 'Hojas de aparatos',
      extensions: ['csv', 'ods', 'xlsx'],
    );
    final elegido = await openFile(acceptedTypeGroups: const [tipos]);
    if (elegido == null) return;

    setState(() {
      _trabajando = true;
      _resumen = null;
    });
    try {
      final datos = await elegido.readAsBytes();
      final plan = await Api.instancia
          .vistaPreviaImportacion(widget.red.clave, elegido.name, datos);
      if (!mounted) return;
      setState(() {
        _nombreArchivo = elegido.name;
        _archivo = datos;
        _plan = plan;
        _repetidos = 'actualizar';
      });
    } catch (problema, pila) {
      if (mounted) {
        setState(() {
          _plan = null;
          _archivo = null;
        });
        await mostrarProblema(context, problema, pila: pila.toString());
      }
    } finally {
      if (mounted) setState(() => _trabajando = false);
    }
  }

  Future<void> _importar() async {
    final archivo = _archivo;
    if (archivo == null) return;

    setState(() => _trabajando = true);
    try {
      final resumen = await Api.instancia.importar(
          widget.red.clave, _nombreArchivo, archivo,
          repetidos: _repetidos);
      if (!mounted) return;
      setState(() {
        _resumen = resumen;
        _plan = null;
        _archivo = null;
        _huboCambios = true;
      });
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _trabajando = false);
    }
  }

  void _olvidarArchivo() => setState(() {
        _plan = null;
        _archivo = null;
      });

  @override
  Widget build(BuildContext contexto) {
    final plan = _plan;
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (salio, _) {
        if (!salio) Navigator.of(contexto).pop(_huboCambios);
      },
      child: Scaffold(
        appBar: AppBar(title: Text('Importar aparatos en ${widget.red.nombre}')),
        // Ocupa TODO el ancho de la ventana. Se probo con un tope de 1080 —para
        // que los parrafos no cruzaran la pantalla entera— y salio mal: la tabla
        // del ejemplo tiene doce columnas y quedaba cortada justo donde empieza
        // a servir. Aqui manda la tabla; los parrafos se defienden solos, que
        // para eso llevan su propio ancho maximo.
        body: ListView(
          padding: const EdgeInsets.all(24),
          children: [
            _paraQueSirve(contexto),
            const SizedBox(height: 20),
            _lasInstrucciones(contexto),
            const SizedBox(height: 20),
            _elCampoDeSubida(contexto),
            if (_resumen != null) ...[
              const SizedBox(height: 20),
              _elResumen(contexto, _resumen!),
            ],
            if (plan != null) ...[
              const SizedBox(height: 24),
              _elPlan(contexto, plan),
            ],
            const SizedBox(height: 32),
            _laGuia(contexto),
            const SizedBox(height: 40),
          ],
        ),
      ),
    );
  }

  // ------------------------------------------------------- las instrucciones --

  Widget _paraQueSirve(BuildContext contexto) {
    final tema = Theme.of(contexto);
    return Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
      Icon(Icons.upload_file_outlined, size: 32, color: tema.colorScheme.primary),
      const SizedBox(width: 16),
      Expanded(
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text('Suba la instalacion que ya tiene documentada',
              style: tema.textTheme.headlineSmall),
          const SizedBox(height: 6),
          _parrafo(
            'Si el sitio esta apuntado en una hoja de calculo —lo normal en algo '
            'cableado por alguien— no hace falta capturarlo aparato por aparato. '
            'De cada renglon salen el aparato, sus puertos, el cable que lo cuelga '
            'de su switch y hasta la clave de su panel.',
            tema.textTheme.bodyMedium,
          ),
        ]),
      ),
    ]);
  }

  Widget _lasInstrucciones(BuildContext contexto) {
    final tema = Theme.of(contexto);
    Widget paso(int numero, String titulo, String texto) => Padding(
          padding: const EdgeInsets.only(bottom: 12),
          child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
            CircleAvatar(
              radius: 13,
              backgroundColor: tema.colorScheme.primaryContainer,
              child: Text('$numero',
                  style: tema.textTheme.labelLarge
                      ?.copyWith(color: tema.colorScheme.onPrimaryContainer)),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text(titulo, style: tema.textTheme.titleSmall),
                _parrafo(texto, tema.textTheme.bodyMedium),
              ]),
            ),
          ]),
        );

    return Card(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 20, 20, 8),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text('Como se hace', style: tema.textTheme.titleMedium),
          const SizedBox(height: 16),
          paso(1, 'Descargue la plantilla',
              'Trae los encabezados, la ayuda de cada columna y dos renglones de '
              'ejemplo. Abrala en LibreOffice o en Excel.'),
          paso(2, 'Llenela: un renglon por aparato',
              'El switch tambien lleva su renglon; los demas cuelgan de el '
              'poniendo su nombre en CUELGA_DE. El orden de los renglones da igual. '
              'La guia de aqui abajo explica columna por columna.'),
          paso(3, 'Subala y revise antes de guardar',
              'MiRed le ensena renglon por renglon lo que va a hacer, con el numero '
              'de renglon de SU hoja. Nada se guarda hasta que usted lo diga.'),
        ]),
      ),
    );
  }

  // ---------------------------------------------------- el campo de subida --

  Widget _elCampoDeSubida(BuildContext contexto) {
    final tema = Theme.of(contexto);
    return Card(
      color: tema.colorScheme.surfaceContainerHighest,
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(children: [
          Icon(Icons.table_view_outlined, size: 40, color: tema.colorScheme.primary),
          const SizedBox(height: 12),
          Text('El archivo', style: tema.textTheme.titleMedium),
          const SizedBox(height: 4),
          Text('Se aceptan CSV, ODS y XLSX',
              style: tema.textTheme.bodySmall, textAlign: TextAlign.center),
          const SizedBox(height: 16),
          Wrap(spacing: 12, runSpacing: 12, alignment: WrapAlignment.center, children: [
            OutlinedButton.icon(
              onPressed: _trabajando ? null : _descargarPlantilla,
              icon: const Icon(Icons.download_outlined),
              label: const Text('Descargar la plantilla'),
            ),
            FilledButton.icon(
              onPressed: _trabajando ? null : _elegirArchivo,
              icon: const Icon(Icons.folder_open_outlined),
              label: Text(_archivo == null ? 'Elegir un archivo' : 'Elegir otro archivo'),
            ),
          ]),
          if (_nombreArchivo.isNotEmpty && _archivo != null) ...[
            const SizedBox(height: 12),
            Chip(
              avatar: const Icon(Icons.description_outlined, size: 18),
              label: Text(_nombreArchivo),
              onDeleted: _trabajando ? null : _olvidarArchivo,
              deleteButtonTooltipMessage: 'Quitar este archivo',
            ),
          ],
          if (_trabajando) ...[
            const SizedBox(height: 16),
            const SizedBox(
                width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2)),
          ],
        ]),
      ),
    );
  }

  // ------------------------------------------------------------- el plan ----

  Widget _elResumen(BuildContext contexto, ResumenImportacion resumen) => Card(
        color: Theme.of(contexto).colorScheme.secondaryContainer,
        child: ListTile(
          leading: const Icon(Icons.check_circle_outline),
          title: const Text('Importado'),
          subtitle: Text(resumen.enPalabras),
        ),
      );

  Widget _elPlan(BuildContext contexto, PlanImportacion plan) {
    final tema = Theme.of(contexto);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('Esto es lo que se haria con «$_nombreArchivo»',
            style: tema.textTheme.titleMedium),
        const SizedBox(height: 8),
        Wrap(spacing: 8, runSpacing: 8, children: [
          if (plan.crear > 0)
            Chip(
                avatar: const Icon(Icons.add_circle_outline, size: 18),
                label: Text('${plan.crear} aparatos nuevos')),
          if (plan.actualizar > 0)
            Chip(
                avatar: const Icon(Icons.sync, size: 18),
                label: Text('${plan.actualizar} ya estaban')),
          if (plan.cables > 0)
            Chip(
                avatar: const Icon(Icons.cable, size: 18),
                label: Text('${plan.cables} cables')),
          if (plan.rechazar > 0)
            Chip(
              avatar: Icon(Icons.error_outline, size: 18, color: tema.colorScheme.error),
              label: Text('${plan.rechazar} sin poder importar'),
            ),
        ]),

        // Las columnas que MiRed no entiende se dicen, no se callan: quien
        // capturo «RESPONSABLE» tiene que enterarse de que ese dato no se guarda.
        if (plan.ignoradas.isNotEmpty) ...[
          const SizedBox(height: 12),
          _aviso(
            contexto,
            Icons.info_outline,
            'Estas columnas del archivo no se guardan en ningun sitio: '
            '${plan.ignoradas.join(', ')}.',
          ),
        ],

        // Un archivo con contrasenas es un archivo con contrasenas. En la base
        // quedan cifradas; en el archivo, no.
        if (plan.conClave > 0) ...[
          const SizedBox(height: 12),
          _aviso(
            contexto,
            Icons.key_outlined,
            'El archivo trae ${plan.conClave} '
            '${plan.conClave == 1 ? 'contrasena' : 'contrasenas'} escritas en claro. '
            'En MiRed quedan cifradas, pero el archivo no: convendria borrarlo '
            'despues de importar.',
            grave: true,
          ),
        ],

        if (plan.actualizar > 0) ...[
          const SizedBox(height: 16),
          Text('${plan.actualizar} de los aparatos del archivo ya estan en la red',
              style: tema.textTheme.titleSmall),
          RadioGroup<String>(
            groupValue: _repetidos,
            onChanged: (valor) => setState(() => _repetidos = valor ?? 'actualizar'),
            child: const Column(children: [
              RadioListTile<String>(
                value: 'actualizar',
                dense: true,
                title: Text('Actualizarlos con lo que trae el archivo'),
                subtitle: Text('Las celdas vacias no borran nada: lo que no venga '
                    'lleno se queda como estaba'),
              ),
              RadioListTile<String>(
                value: 'saltar',
                dense: true,
                title: Text('Dejarlos como estan'),
                subtitle: Text('Solo entran los aparatos nuevos'),
              ),
            ]),
          ),
        ],

        const SizedBox(height: 16),
        _laTabla(contexto, plan),
        const SizedBox(height: 16),
        Row(children: [
          FilledButton.icon(
            onPressed: _trabajando || !plan.hayAlgoQueImportar ? null : _importar,
            icon: const Icon(Icons.save_outlined),
            label: const Text('Importar'),
          ),
          const SizedBox(width: 12),
          TextButton(
            onPressed: _trabajando ? null : _olvidarArchivo,
            child: const Text('Cancelar'),
          ),
        ]),
        if (!plan.hayAlgoQueImportar)
          Padding(
            padding: const EdgeInsets.only(top: 8),
            child: Text(
              'No hay ni un renglon que se pueda importar. Corrija el archivo '
              'mirando los motivos de aqui abajo y vuelva a elegirlo.',
              style: tema.textTheme.bodySmall?.copyWith(color: tema.colorScheme.error),
            ),
          ),
      ],
    );
  }

  /// _parrafo le pone al texto su ancho de lectura.
  ///
  /// La pantalla ocupa toda la ventana porque las tablas lo necesitan —el
  /// ejemplo de llenado tiene doce columnas—, pero un renglon de texto que cruza
  /// 1 900 pixeles no lo sigue el ojo de nadie.
  Widget _parrafo(String texto, TextStyle? estilo) => ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: anchoDeLectura),
        child: Text(texto, style: estilo),
      );

  Widget _aviso(BuildContext contexto, IconData icono, String texto,
      {bool grave = false}) {
    final tema = Theme.of(contexto);
    final color = grave ? tema.colorScheme.error : tema.colorScheme.onSurfaceVariant;
    return Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
      Icon(icono, size: 18, color: color),
      const SizedBox(width: 8),
      Expanded(child: Text(texto, style: tema.textTheme.bodySmall?.copyWith(color: color))),
    ]);
  }

  /// _laTabla ensena renglon por renglon lo que va a pasar.
  ///
  /// Con el numero de renglon **del archivo**, para que quien tenga que corregir
  /// algo sepa exactamente donde mirar en su hoja de calculo.
  Widget _laTabla(BuildContext contexto, PlanImportacion plan) {
    final tema = Theme.of(contexto);
    return Card(
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: DataTable(
          columns: const [
            DataColumn(label: Text('Renglon')),
            DataColumn(label: Text('Aparato')),
            DataColumn(label: Text('Que es')),
            DataColumn(label: Text('Cuelga de')),
            DataColumn(label: Text('Donde esta')),
            DataColumn(label: Text('Que se hace')),
          ],
          rows: [
            for (final renglon in plan.renglones)
              DataRow(
                color: renglon.seRechaza
                    ? WidgetStatePropertyAll(
                        tema.colorScheme.errorContainer.withValues(alpha: 0.4))
                    : null,
                cells: [
                  DataCell(Text('${renglon.renglon}')),
                  DataCell(Text(renglon.nombre)),
                  DataCell(Text(renglon.tipo)),
                  DataCell(Text(renglon.cuelgaDe.isEmpty
                      ? ''
                      : '${renglon.cuelgaDe}  ·  ${renglon.puerto}')),
                  DataCell(Text(renglon.ubicacion)),
                  DataCell(_queSeHace(contexto, renglon)),
                ],
              ),
          ],
        ),
      ),
    );
  }

  Widget _queSeHace(BuildContext contexto, RenglonImportado renglon) {
    final tema = Theme.of(contexto);
    if (renglon.seRechaza) {
      return Row(mainAxisSize: MainAxisSize.min, children: [
        Icon(Icons.error_outline, size: 16, color: tema.colorScheme.error),
        const SizedBox(width: 6),
        // El motivo va a la vista y no en un globito: si hay que corregir el
        // archivo, hay que poder leerlo todo de un tiron.
        ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 420),
          child: Text(renglon.motivo,
              style: tema.textTheme.bodySmall?.copyWith(color: tema.colorScheme.error)),
        ),
      ]);
    }

    final texto = renglon.seActualiza ? 'Ya existe: se actualiza' : 'Se crea';
    if (renglon.aviso.isEmpty) return Text(texto);
    return Row(mainAxisSize: MainAxisSize.min, children: [
      const Icon(Icons.warning_amber_outlined, size: 16),
      const SizedBox(width: 6),
      ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 420),
        child: Text('$texto — ${renglon.aviso}', style: tema.textTheme.bodySmall),
      ),
    ]);
  }

  // ------------------------------------------------------------- la guia ----

  Widget _laGuia(BuildContext contexto) {
    final tema = Theme.of(contexto);
    final plantilla = _plantilla;

    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      Row(children: [
        const Icon(Icons.menu_book_outlined),
        const SizedBox(width: 8),
        Text('Guia para llenar la hoja', style: tema.textTheme.titleLarge),
      ]),
      const SizedBox(height: 4),
      _parrafo(
        'Esta guia sale de lo que el servidor acepta de verdad, no de un manual '
        'aparte: si un dia cambia una columna, cambia aqui sola.',
        tema.textTheme.bodySmall,
      ),
      const SizedBox(height: 16),
      _lasReglas(contexto),
      const SizedBox(height: 20),
      if (plantilla == null)
        const Padding(
          padding: EdgeInsets.symmetric(vertical: 24),
          child: Center(child: CircularProgressIndicator()),
        )
      else ...[
        Text('Las columnas', style: tema.textTheme.titleMedium),
        const SizedBox(height: 8),
        _tablaDeColumnas(contexto, plantilla),
        const SizedBox(height: 24),
        Text('Que se puede poner en QUE_ES', style: tema.textTheme.titleMedium),
        const SizedBox(height: 4),
        _parrafo(
          'Es una lista cerrada, la misma con la que MiRed clasifica lo que '
          'descubre. Lo que no este aqui se rechaza en vez de inventar una '
          'categoria, que dejaria el contador de la red diciendo dos cosas.',
          tema.textTheme.bodySmall,
        ),
        const SizedBox(height: 12),
        _lasCategorias(contexto, plantilla),
        const SizedBox(height: 24),
        Text('Un ejemplo lleno', style: tema.textTheme.titleMedium),
        const SizedBox(height: 4),
        Text('Los mismos renglones que trae la plantilla descargable.',
            style: tema.textTheme.bodySmall),
        const SizedBox(height: 12),
        _elEjemplo(contexto, plantilla),
      ],
    ]);
  }

  Widget _lasReglas(BuildContext contexto) {
    final tema = Theme.of(contexto);
    Widget regla(IconData icono, String titulo, String texto) => Padding(
          padding: const EdgeInsets.only(bottom: 10),
          child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Icon(icono, size: 18, color: tema.colorScheme.primary),
            const SizedBox(width: 10),
            Expanded(
              // El Expanded manda un ancho FIJO, asi que un ConstrainedBox a
              // secas no encoge nada: hay que alinear primero a la izquierda.
              child: Align(
                alignment: Alignment.centerLeft,
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: anchoDeLectura),
                  child: Text.rich(TextSpan(children: [
                    TextSpan(text: '$titulo ', style: tema.textTheme.titleSmall),
                    TextSpan(text: texto, style: tema.textTheme.bodyMedium),
                  ])),
                ),
              ),
            ),
          ]),
        );

    return Card(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 20, 20, 10),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          regla(Icons.hub_outlined, 'El switch tambien es un renglon.',
              'Los demas cuelgan de el poniendo su NOMBRE en CUELGA_DE y el numero '
              'de puerto en PUERTO. Asi tambien se dice que un aparato cuelga del '
              'modem y no del switch: se cambia el nombre y ya.'),
          regla(Icons.swap_vert, 'El orden de los renglones da igual.',
              'Primero se crean todos los aparatos y despues se tiran los cables, '
              'asi que un nodo puede ir antes que su switch.'),
          regla(Icons.badge_outlined, 'El nombre no se puede repetir.',
              'Es como se reconoce un aparato en la red. Si ya existe uno con ese '
              'nombre, usted decide si se actualiza o se deja como esta.'),
          regla(Icons.backspace_outlined, 'Una celda vacia no borra nada.',
              'Significa «no lo se», no «borralo»: lo que deje en blanco se queda '
              'como estaba.'),
          regla(Icons.place_outlined, 'UBICACION no es CUELGA_DE.',
              'Una cosa es donde ESTA el aparato —farmacia, consultorio 5— y otra de '
              'que puerto cuelga. Las dos se guardan.'),
          regla(Icons.translate, 'No hace falta reescribir su hoja.',
              'Se reconocen encabezados como NODO, OBSERVACIONES o CONECTADO_A, el '
              'titulo del sitio arriba, los renglones en blanco, el punto y coma que '
              'pone Excel en espanol y los acentos.'),
          regla(Icons.key_outlined, 'Las claves se guardan cifradas.',
              'En MiRed si; en el archivo no. Si la hoja lleva contrasenas, '
              'convendria borrarla despues de importar.'),
        ]),
      ),
    );
  }

  Widget _tablaDeColumnas(BuildContext contexto, PlantillaImportacion plantilla) {
    final tema = Theme.of(contexto);
    return Card(
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: DataTable(
          columnSpacing: 24,
          columns: const [
            DataColumn(label: Text('Columna')),
            DataColumn(label: Text('Que se escribe')),
            DataColumn(label: Text('Ejemplo')),
            DataColumn(label: Text('Tambien se acepta')),
          ],
          rows: [
            for (final columna in plantilla.columnas)
              DataRow(cells: [
                DataCell(Row(mainAxisSize: MainAxisSize.min, children: [
                  Text(columna.clave,
                      style: tema.textTheme.bodyMedium
                          ?.copyWith(fontFeatures: const [FontFeature.tabularFigures()])),
                  if (columna.obligatoria) ...[
                    const SizedBox(width: 6),
                    Tooltip(
                      message: 'Sin esta columna el archivo no se puede importar',
                      child: Text('obligatoria',
                          style: tema.textTheme.labelSmall
                              ?.copyWith(color: tema.colorScheme.error)),
                    ),
                  ],
                ])),
                DataCell(ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 420),
                  child: Text(columna.ayuda),
                )),
                DataCell(Text(columna.ejemplo,
                    style: tema.textTheme.bodySmall
                        ?.copyWith(color: tema.colorScheme.primary))),
                DataCell(ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 260),
                  child: Text(columna.otrosNombres.join(', '),
                      style: tema.textTheme.bodySmall),
                )),
              ]),
          ],
        ),
      ),
    );
  }

  Widget _lasCategorias(BuildContext contexto, PlantillaImportacion plantilla) {
    final tema = Theme.of(contexto);
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Wrap(spacing: 10, runSpacing: 10, children: [
          for (final categoria in plantilla.categorias)
            Tooltip(
              message: categoria.apodos.isEmpty
                  ? categoria.comoSeLee
                  : '${categoria.comoSeLee}\nTambien: ${categoria.apodos.join(', ')}',
              child: Chip(
                label: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(categoria.clave, style: tema.textTheme.bodyMedium),
                    Text(categoria.comoSeLee, style: tema.textTheme.labelSmall),
                  ],
                ),
              ),
            ),
        ]),
      ),
    );
  }

  Widget _elEjemplo(BuildContext contexto, PlantillaImportacion plantilla) {
    // Solo las columnas que el ejemplo usa: ensenar catorce columnas donde diez
    // van vacias no ensena nada.
    final usadas = <String>[];
    for (final columna in plantilla.columnas) {
      if (plantilla.ejemplo.any((fila) => (fila[columna.clave] ?? '').isNotEmpty)) {
        usadas.add(columna.clave);
      }
    }
    if (usadas.isEmpty) return const SizedBox.shrink();

    return Card(
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: DataTable(
          columnSpacing: 20,
          columns: [for (final clave in usadas) DataColumn(label: Text(clave))],
          rows: [
            for (final fila in plantilla.ejemplo)
              DataRow(cells: [
                for (final clave in usadas) DataCell(Text(fila[clave] ?? '')),
              ]),
          ],
        ),
      ),
    );
  }
}
