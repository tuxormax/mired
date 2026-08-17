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
/// **Son dos pasos y el primero no escribe nada.** Se elige el archivo, se ve
/// renglon por renglon lo que pasaria, y solo entonces se importa. Importar a
/// ciegas y descubrir despues que tres renglones estaban mal significa borrarlos
/// a mano de uno en uno.
class PantallaImportar extends StatefulWidget {
  const PantallaImportar({super.key, required this.red});

  final Red red;

  @override
  State<PantallaImportar> createState() => _PantallaImportarState();
}

class _PantallaImportarState extends State<PantallaImportar> {
  String _nombreArchivo = '';
  Uint8List? _archivo;
  PlanImportacion? _plan;
  ResumenImportacion? _resumen;

  /// Que hacer con los aparatos que ya existen. Arranca en actualizar: quien
  /// vuelve a subir la hoja casi siempre es porque corrigio un dato en ella.
  String _repetidos = 'actualizar';
  bool _trabajando = false;
  bool _huboCambios = false;

  Future<void> _descargarPlantilla() async {
    setState(() => _trabajando = true);
    try {
      final plantilla = await Api.instancia.plantillaDeImportacion(widget.red.clave);
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
        body: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            _laExplicacion(contexto),
            const SizedBox(height: 16),
            Row(children: [
              OutlinedButton.icon(
                onPressed: _trabajando ? null : _descargarPlantilla,
                icon: const Icon(Icons.download_outlined),
                label: const Text('Descargar la plantilla'),
              ),
              const SizedBox(width: 12),
              FilledButton.icon(
                onPressed: _trabajando ? null : _elegirArchivo,
                icon: const Icon(Icons.upload_file_outlined),
                label: const Text('Elegir un archivo'),
              ),
              if (_trabajando) ...[
                const SizedBox(width: 16),
                const SizedBox(
                    width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2)),
              ],
            ]),
            if (_resumen != null) ...[
              const SizedBox(height: 20),
              _elResumen(contexto, _resumen!),
            ],
            if (plan != null) ...[
              const SizedBox(height: 20),
              _elPlan(contexto, plan),
            ],
          ],
        ),
      ),
    );
  }

  Widget _laExplicacion(BuildContext contexto) => Card(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Como se llena la hoja',
                  style: Theme.of(contexto).textTheme.titleMedium),
              const SizedBox(height: 8),
              const Text(
                'Un renglon por aparato. El switch tambien lleva su renglon, y los '
                'demas cuelgan de el poniendo su nombre en CUELGA_DE y el numero de '
                'puerto en PUERTO. El orden de los renglones da igual.',
              ),
              const SizedBox(height: 8),
              const Text(
                'Se aceptan CSV, ODS y XLSX. Si su hoja ya viene de otro sitio no hace '
                'falta reescribirla: MiRed reconoce tambien encabezados como NODO, '
                'OBSERVACIONES o CONECTADO_A.',
              ),
              const SizedBox(height: 8),
              Text(
                'Nada se guarda hasta que usted lo diga: al elegir el archivo se '
                'ensena antes, renglon por renglon, lo que se va a hacer.',
                style: Theme.of(contexto).textTheme.bodySmall,
              ),
            ],
          ),
        ),
      );

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
            onPressed: _trabajando
                ? null
                : () => setState(() {
                      _plan = null;
                      _archivo = null;
                    }),
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
                    ? WidgetStatePropertyAll(tema.colorScheme.errorContainer.withValues(alpha: 0.4))
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
      return Tooltip(
        message: renglon.motivo,
        child: Row(mainAxisSize: MainAxisSize.min, children: [
          Icon(Icons.error_outline, size: 16, color: tema.colorScheme.error),
          const SizedBox(width: 6),
          // El motivo va a la vista y no solo en el globito: si hay que
          // corregir el archivo, hay que poder leerlo todo de un tiron.
          ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: Text(renglon.motivo,
                style: tema.textTheme.bodySmall?.copyWith(color: tema.colorScheme.error)),
          ),
        ]),
      );
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
}
