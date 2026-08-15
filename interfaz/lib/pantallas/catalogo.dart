import 'package:flutter/material.dart';

import '../servicios/api.dart';
import '../widgets/mensajes.dart';

/// PantallaCatalogo muestra que aparatos sabe reconocer esta instalacion, y deja
/// traer los que aporto la comunidad.
///
/// El catalogo es la parte del proyecto que no puede escribir una sola persona:
/// nadie tiene en su casa una camara de cada marca. Por eso las definiciones son
/// archivos y no codigo, y por eso hay dos caminos:
///
///   * lo que se declara aqui, que se guarda aparte y **gana siempre**, porque
///     quien tiene el aparato delante sabe mas que ningun repositorio;
///   * lo que publican los demas, que se baja a su propia carpeta y no pisa
///     nunca lo de aqui.
class PantallaCatalogo extends StatefulWidget {
  const PantallaCatalogo({super.key});

  @override
  State<PantallaCatalogo> createState() => _PantallaCatalogoState();
}

class _PantallaCatalogoState extends State<PantallaCatalogo> {
  late Future<Map<String, dynamic>> _catalogo;
  bool _actualizando = false;
  String _filtro = '';

  @override
  void initState() {
    super.initState();
    _catalogo = Api.instancia.catalogo();
  }

  void _recargar() => setState(() => _catalogo = Api.instancia.catalogo());

  Future<void> _actualizar() async {
    setState(() => _actualizando = true);
    try {
      final resultado = await Api.instancia.actualizarCatalogo();
      if (!mounted) return;

      final bajadas = resultado['bajadas'] as int? ?? 0;
      final nuevas = resultado['nuevas'] as int? ?? 0;
      final fallos = (resultado['fallos'] as List<dynamic>? ?? []).length;
      mensajeExito(
          context,
          'Se trajeron $bajadas definiciones, $nuevas de ellas nuevas. '
          'El catalogo tiene ahora ${resultado['definiciones']} en total'
          '${fallos > 0 ? ', y $fallos archivos no se pudieron usar' : ''}.');
      _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _actualizando = false);
    }
  }

  @override
  Widget build(BuildContext contexto) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Catalogo de dispositivos'),
        actions: [
          IconButton(
            tooltip: 'Actualizar la lista',
            icon: const Icon(Icons.refresh),
            onPressed: _recargar,
          ),
        ],
      ),
      body: FutureBuilder<Map<String, dynamic>>(
        future: _catalogo,
        builder: (_, resultado) {
          if (resultado.hasError) {
            return Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Text('No se pudo leer el catalogo: ${resultado.error}'),
              ),
            );
          }
          if (!resultado.hasData) {
            return const Center(child: CircularProgressIndicator());
          }

          final datos = resultado.data!;
          final definiciones = (datos['definiciones'] as List<dynamic>? ?? [])
              .cast<Map<String, dynamic>>();
          final problemas = (datos['problemas'] as List<dynamic>? ?? []).cast<String>();
          final lista = definiciones.where((fila) {
            if (_filtro.isEmpty) return true;
            final texto = '${fila['nombre']} ${fila['categoria']} ${fila['descripcion']} '
                    '${fila['archivo']}'
                .toLowerCase();
            return texto.contains(_filtro);
          }).toList();

          return Column(
            children: [
              Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'MiRed reconoce ${definiciones.length} tipos de aparato. Las '
                      'definiciones son archivos: las que usted escriba mandan sobre las '
                      'que trae el paquete y sobre las de la comunidad.',
                      style: Theme.of(contexto).textTheme.bodyMedium,
                    ),
                    const SizedBox(height: 12),
                    Row(
                      children: [
                        Expanded(
                          child: TextField(
                            decoration: const InputDecoration(
                              prefixIcon: Icon(Icons.search),
                              hintText: 'Buscar por nombre, categoria o archivo',
                              border: OutlineInputBorder(),
                              isDense: true,
                            ),
                            onChanged: (texto) =>
                                setState(() => _filtro = texto.toLowerCase()),
                          ),
                        ),
                        const SizedBox(width: 12),
                        FilledButton.icon(
                          icon: _actualizando
                              ? const SizedBox(
                                  width: 16,
                                  height: 16,
                                  child: CircularProgressIndicator(strokeWidth: 2))
                              : const Icon(Icons.cloud_download_outlined),
                          label: const Text('Traer las de la comunidad'),
                          onPressed: _actualizando ? null : _actualizar,
                        ),
                      ],
                    ),
                  ],
                ),
              ),

              // Un archivo que no se pudo cargar se DICE. Quien escribio una
              // definicion propia y no funciona necesita ver el error, no
              // quedarse pensando que su aparato no se puede reconocer.
              if (problemas.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  child: Card(
                    color: Theme.of(contexto).colorScheme.errorContainer,
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const Text('Definiciones que no se pudieron usar',
                              style: TextStyle(fontWeight: FontWeight.bold)),
                          const SizedBox(height: 4),
                          for (final problema in problemas) Text(problema),
                        ],
                      ),
                    ),
                  ),
                ),

              Expanded(
                child: ListView.separated(
                  padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                  itemCount: lista.length,
                  separatorBuilder: (_, __) => const Divider(height: 1),
                  itemBuilder: (_, indice) {
                    final fila = lista[indice];
                    final archivo = fila['archivo'] as String? ?? '';
                    return ListTile(
                      leading: const Icon(Icons.devices_other),
                      title: Text(fila['nombre'] as String? ?? ''),
                      subtitle: Text([
                        if ((fila['descripcion'] as String? ?? '').isNotEmpty)
                          fila['descripcion'] as String,
                        if ((fila['aporta'] as String? ?? '').isNotEmpty)
                          'aporto ${fila['aporta']}',
                        // De donde salio la definicion: no es lo mismo una que
                        // trae el paquete que una que escribio el usuario.
                        _deDondeSalio(archivo),
                      ].join(' · ')),
                      trailing: Text(fila['categoria'] as String? ?? '',
                          style: Theme.of(contexto).textTheme.labelSmall),
                    );
                  },
                ),
              ),
            ],
          );
        },
      ),
    );
  }

  String _deDondeSalio(String archivo) {
    if (archivo.contains('comunidad')) return 'de la comunidad';
    if (archivo.startsWith('/usr/share/')) return 'viene con MiRed';
    return 'escrita aqui';
  }
}
