import 'package:flutter/material.dart';

import '../servicios/api.dart';
import '../widgets/mensajes.dart';

/// DialogoVersiones muestra que cambio en cada entrega.
///
/// Lo abre el pie de la pantalla de inicio al pulsarlo, como en el resto de los
/// proyectos de la casa. Responde lo que el numero de version a secas no puede:
/// **que cambio, cuando y en que modulo**. Cuando alguien reporta un problema,
/// lo primero que hace falta saber es que trae instalado y desde cuando.
class DialogoVersiones extends StatefulWidget {
  const DialogoVersiones({super.key});

  @override
  State<DialogoVersiones> createState() => _DialogoVersionesState();
}

class _DialogoVersionesState extends State<DialogoVersiones>
    with SingleTickerProviderStateMixin {
  late Future<Map<String, dynamic>> _historial;
  late final TabController _pestanas;

  @override
  void initState() {
    super.initState();
    _pestanas = TabController(length: 2, vsync: this);
    _historial = Api.instancia.historialVersiones();
  }

  @override
  void dispose() {
    _pestanas.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext contexto) {
    return AlertDialog(
      title: const Text('Historial de versiones'),
      content: SizedBox(
        width: 620,
        height: 460,
        child: FutureBuilder<Map<String, dynamic>>(
          future: _historial,
          builder: (_, resultado) {
            if (resultado.connectionState != ConnectionState.done) {
              return const Center(child: CircularProgressIndicator());
            }
            if (resultado.hasError) {
              return Center(
                child: TextButton(
                  onPressed: () => mostrarProblema(contexto, resultado.error!),
                  child: const Text('No se pudo cargar el historial. Ver detalles'),
                ),
              );
            }

            final datos = resultado.data ?? {};
            final actual = datos['actual'] as Map<String, dynamic>? ?? {};

            return Column(
              children: [
                // Lo primero: que trae instalado ESTE equipo. Es el dato por el
                // que se abre esta ventana nueve de cada diez veces.
                Card(
                  margin: EdgeInsets.zero,
                  child: ListTile(
                    dense: true,
                    leading: const Icon(Icons.tag),
                    title: Text('MiRed ${actual['version']} Rev ${actual['revision']}'),
                    subtitle: Text('Compilado de ${actual['build']}'),
                  ),
                ),
                TabBar(
                  controller: _pestanas,
                  tabs: const [
                    Tab(text: 'El sistema'),
                    Tab(text: 'La base de datos'),
                  ],
                ),
                Expanded(
                  child: TabBarView(
                    controller: _pestanas,
                    children: [
                      _Lista(entradas: datos['sistema'] as List<dynamic>? ?? []),
                      _Lista(entradas: datos['base'] as List<dynamic>? ?? []),
                    ],
                  ),
                ),
              ],
            );
          },
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(contexto).pop(),
          child: const Text('Cerrar'),
        ),
      ],
    );
  }
}

class _Lista extends StatelessWidget {
  const _Lista({required this.entradas});

  final List<dynamic> entradas;

  @override
  Widget build(BuildContext contexto) {
    if (entradas.isEmpty) {
      return const Center(child: Text('Todavia no hay historial.'));
    }

    return ListView.separated(
      padding: const EdgeInsets.symmetric(vertical: 8),
      itemCount: entradas.length,
      separatorBuilder: (_, __) => const Divider(height: 1),
      itemBuilder: (_, indice) {
        final entrada = entradas[indice] as Map<String, dynamic>;
        return ListTile(
          dense: true,
          leading: _Etiqueta(tipo: entrada['tipo'] as String? ?? ''),
          title: Text('Rev ${entrada['revision']} · ${entrada['modulo']}',
              style: const TextStyle(fontWeight: FontWeight.w600)),
          subtitle: Text(entrada['notas'] as String? ?? ''),
          trailing: Text(entrada['fecha'] as String? ?? '',
              style: Theme.of(contexto).textTheme.labelSmall),
        );
      },
    );
  }
}

/// _Etiqueta pinta el tipo de cambio, para poder recorrer la lista de un vistazo
/// buscando solo las correcciones.
class _Etiqueta extends StatelessWidget {
  const _Etiqueta({required this.tipo});

  final String tipo;

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;

    late final Color fondo;
    late final String texto;
    switch (tipo) {
      case 'feature':
        fondo = colores.primaryContainer;
        texto = 'nuevo';
      case 'fix':
        fondo = colores.errorContainer;
        texto = 'arreglo';
      case 'change':
        fondo = colores.tertiaryContainer;
        texto = 'cambio';
      default:
        fondo = colores.surfaceContainerHighest;
        texto = 'ajuste';
    }

    return Container(
      width: 62,
      padding: const EdgeInsets.symmetric(vertical: 4),
      decoration: BoxDecoration(
        color: fondo,
        borderRadius: BorderRadius.circular(6),
      ),
      child: Text(texto,
          textAlign: TextAlign.center,
          style: Theme.of(contexto).textTheme.labelSmall),
    );
  }
}
