import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../servicios/trayectoria.dart';

/// mensajeError muestra el modal de error con el formato de la casa.
///
/// Tres decisiones que parecen exageradas y no lo son:
///  - No se puede cerrar sin copiar. El reflejo de cerrar hace perder el
///    detalle, y despues el reporte es "salio un error" y no se puede arreglar.
///  - Todos los campos visibles y seleccionables, incluido el mensaje crudo del
///    sistema.
///  - Se manda solo al servidor apenas aparece, para que soporte lo vea aunque
///    nadie lo reporte.
Future<void> mensajeError(BuildContext contexto, ErrorMiRed problema, {String? pila}) async {
  final texto = _armarTexto(problema);

  // El envio va primero y sin esperar: si el usuario cierra rapido, el error ya
  // salio.
  Api.instancia.reportarError(problema, Trayectoria.instancia.texto, pila ?? '');

  if (!contexto.mounted) return;
  var copiado = false;

  await showDialog<void>(
    context: contexto,
    barrierDismissible: false,
    builder: (contextoModal) => PopScope(
      canPop: false,
      child: StatefulBuilder(
        builder: (contextoEstado, redibujar) => AlertDialog(
          title: const Row(
            children: [
              Icon(Icons.error_outline, color: Colors.red),
              SizedBox(width: 8),
              Text('Error'),
            ],
          ),
          content: SizedBox(
            width: 720,
            child: SingleChildScrollView(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Text(
                    'Ha habido un error, copie los detalles y notifique a seismexico.',
                    style: TextStyle(fontWeight: FontWeight.w600),
                  ),
                  const SizedBox(height: 12),
                  Container(
                    width: double.infinity,
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: Theme.of(contextoEstado).colorScheme.surfaceContainerHighest,
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: SelectableText(
                      texto,
                      style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
                    ),
                  ),
                ],
              ),
            ),
          ),
          actions: [
            FilledButton.icon(
              icon: Icon(copiado ? Icons.check : Icons.copy),
              label: Text(copiado ? 'Copiado' : 'Copiar'),
              onPressed: () async {
                await Clipboard.setData(ClipboardData(text: texto));
                redibujar(() => copiado = true);
                // Vuelve a "Copiar" por si quiere copiarlo de nuevo, pero el
                // boton de cerrar ya queda habilitado para siempre.
                await Future<void>.delayed(const Duration(seconds: 2));
                if (contextoEstado.mounted) redibujar(() {});
              },
            ),
            TextButton(
              onPressed: copiado ? () => Navigator.of(contextoModal).pop() : null,
              child: const Text('Cerrar'),
            ),
          ],
        ),
      ),
    ),
  );
}

/// mensajeAviso es para lo que el usuario puede corregir solo: capturo algo mal,
/// no tiene permiso. Un modal bloqueante ahi seria un castigo, no una ayuda.
void mensajeAviso(BuildContext contexto, String texto) {
  ScaffoldMessenger.of(contexto).showSnackBar(
    SnackBar(content: Text(texto), behavior: SnackBarBehavior.floating),
  );
}

/// mostrarProblema decide sola: aviso corto si el usuario puede corregirlo,
/// modal completo si es una falla de verdad.
Future<void> mostrarProblema(BuildContext contexto, Object problema, {String? pila}) async {
  if (problema is ErrorMiRed) {
    if (problema.esDeCaptura) {
      mensajeAviso(contexto, problema.mensaje);
      return;
    }
    await mensajeError(contexto, problema, pila: pila);
    return;
  }
  await mensajeError(
    contexto,
    ErrorMiRed(
      modulo: 'Interfaz',
      accion: 'Operacion',
      causa: 'Interno',
      mensaje: 'Ocurrio un error inesperado en la interfaz.',
      errorCrudo: problema.toString(),
    ),
    pila: pila,
  );
}

String _armarTexto(ErrorMiRed problema) {
  final lineas = <String>[
    '[Error] Ha habido un error, copie los detalles y notifique a seismexico.',
    '',
    _campo('ID', problema.id),
    _campo('Timestamp', problema.momento.isNotEmpty ? problema.momento : DateTime.now().toIso8601String()),
    _campo('Modulo', problema.modulo),
    _campo('Accion', problema.accion),
    _campo('Causa', problema.causa),
    _campo('Ruta', problema.ruta),
    _campo('Tabla', problema.tabla),
    _campo('Endpoint', problema.endpoint),
    _campo('Red', problema.red),
    _campo('Usuario', problema.usuario.isNotEmpty ? problema.usuario : (Api.instancia.usuario?.usuario ?? '')),
    _campo('Version', problema.version.isNotEmpty ? problema.version : Api.instancia.version),
    _campo('Build', problema.build),
    _campo('SO', problema.so),
    _campo('Mensaje', problema.mensaje),
    _campo('Estado', problema.estado),
    'Trayectoria:',
    Trayectoria.instancia.texto,
  ].where((linea) => linea.isNotEmpty).toList();

  if (problema.errorCrudo.isNotEmpty) {
    lineas
      ..add('')
      ..add('--- Error del sistema ---')
      ..add(problema.errorCrudo);
  }
  return lineas.join('\n');
}

String _campo(String etiqueta, String valor) =>
    valor.isEmpty ? '' : '${etiqueta.padRight(12)}$valor';
