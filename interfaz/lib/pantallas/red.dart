import 'package:flutter/material.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../servicios/trayectoria.dart';
import '../widgets/mensajes.dart';

/// PantallaRed es lo de un sitio: sus subredes y, a partir de la fase 2, sus
/// equipos y su mapa.
class PantallaRed extends StatefulWidget {
  const PantallaRed({super.key, required this.red});

  final Red red;

  @override
  State<PantallaRed> createState() => _PantallaRedState();
}

class _PantallaRedState extends State<PantallaRed> {
  late Future<List<Subred>> _subredes;

  @override
  void initState() {
    super.initState();
    _recargar();
  }

  void _recargar() {
    setState(() => _subredes = Api.instancia.listarSubredes(widget.red.clave));
  }

  Future<void> _agregarSubred() async {
    final agregada = await showDialog<bool>(
      context: context,
      builder: (_) => _DialogoNuevaSubred(clave: widget.red.clave),
    );
    if (agregada == true) _recargar();
  }

  @override
  Widget build(BuildContext contexto) {
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.red.nombre),
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(20),
          child: Padding(
            padding: const EdgeInsets.only(left: 16, bottom: 8),
            child: Align(
              alignment: Alignment.centerLeft,
              child: Text(
                'Archivo: ${widget.red.clave}.db',
                style: Theme.of(contexto).textTheme.labelSmall,
              ),
            ),
          ),
        ),
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _agregarSubred,
        icon: const Icon(Icons.add),
        label: const Text('Agregar subred'),
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  const Icon(Icons.info_outline),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text(
                      'El descubrimiento de equipos llega en la fase 2. Por ahora aqui se '
                      'define que rangos se van a escanear en este sitio.',
                      style: Theme.of(contexto).textTheme.bodyMedium,
                    ),
                  ),
                ],
              ),
            ),
          ),
          const SizedBox(height: 20),
          Text('Subredes', style: Theme.of(contexto).textTheme.titleMedium),
          const SizedBox(height: 8),
          FutureBuilder<List<Subred>>(
            future: _subredes,
            builder: (_, resultado) {
              if (resultado.connectionState != ConnectionState.done) {
                return const Padding(
                  padding: EdgeInsets.all(32),
                  child: Center(child: CircularProgressIndicator()),
                );
              }
              if (resultado.hasError) {
                return ListTile(
                  leading: const Icon(Icons.error_outline),
                  title: const Text('No se pudieron cargar las subredes'),
                  trailing: TextButton(
                    onPressed: () => mostrarProblema(contexto, resultado.error!),
                    child: const Text('Ver detalles'),
                  ),
                );
              }

              final subredes = resultado.data ?? [];
              if (subredes.isEmpty) {
                return const Padding(
                  padding: EdgeInsets.all(24),
                  child: Text('Este sitio todavia no tiene subredes que escanear.'),
                );
              }

              return Card(
                child: Column(
                  children: [
                    for (final subred in subredes)
                      ListTile(
                        leading: const Icon(Icons.route_outlined),
                        title: Text(subred.cidr, style: const TextStyle(fontFamily: 'monospace')),
                        subtitle: Text([
                          if (subred.nombre.isNotEmpty) subred.nombre,
                          if (subred.vlan != null) 'VLAN ${subred.vlan}',
                          subred.escanear ? 'Se escanea' : 'No se escanea',
                        ].join(' · ')),
                      ),
                  ],
                ),
              );
            },
          ),
        ],
      ),
    );
  }
}

class _DialogoNuevaSubred extends StatefulWidget {
  const _DialogoNuevaSubred({required this.clave});

  final String clave;

  @override
  State<_DialogoNuevaSubred> createState() => _DialogoNuevaSubredState();
}

class _DialogoNuevaSubredState extends State<_DialogoNuevaSubred> {
  final _formulario = GlobalKey<FormState>();
  final _cidr = TextEditingController();
  final _nombre = TextEditingController();
  final _vlan = TextEditingController();
  bool _ocupado = false;

  @override
  void dispose() {
    _cidr.dispose();
    _nombre.dispose();
    _vlan.dispose();
    super.dispose();
  }

  Future<void> _guardar() async {
    if (!_formulario.currentState!.validate()) return;
    setState(() => _ocupado = true);
    Trayectoria.instancia.anotar('Agregar subred ${_cidr.text}');

    try {
      await Api.instancia.crearSubred(
        widget.clave,
        _cidr.text.trim(),
        _nombre.text.trim(),
        _vlan.text.trim().isEmpty ? null : int.parse(_vlan.text.trim()),
      );
      if (mounted) Navigator.of(context).pop(true);
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _ocupado = false);
    }
  }

  @override
  Widget build(BuildContext contexto) => AlertDialog(
        title: const Text('Agregar subred'),
        content: SizedBox(
          width: 420,
          child: Form(
            key: _formulario,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextFormField(
                  controller: _cidr,
                  autofocus: true,
                  decoration: const InputDecoration(
                    labelText: 'Rango',
                    hintText: '192.168.1.0/24',
                    border: OutlineInputBorder(),
                  ),
                  validator: (valor) {
                    if (valor == null || valor.trim().isEmpty) return 'Capture el rango';
                    // Lo mismo que valida el servidor: si aqui pasa, alla pasa.
                    if (!RegExp(r'^[0-9a-fA-F:.]+/\d{1,3}$').hasMatch(valor.trim())) {
                      return 'Se espera algo como 192.168.1.0/24';
                    }
                    return null;
                  },
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _nombre,
                  maxLength: 80,
                  decoration: const InputDecoration(
                    labelText: 'Nombre (opcional)',
                    hintText: 'Camaras, Oficinas, WiFi...',
                    border: OutlineInputBorder(),
                    counterText: '',
                  ),
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _vlan,
                  keyboardType: TextInputType.number,
                  decoration: const InputDecoration(
                    labelText: 'VLAN (opcional)',
                    border: OutlineInputBorder(),
                  ),
                  validator: (valor) {
                    if (valor == null || valor.trim().isEmpty) return null;
                    final numero = int.tryParse(valor.trim());
                    // El rango real de una VLAN, que es lo que acepta la columna.
                    if (numero == null || numero < 1 || numero > 4094) {
                      return 'Debe estar entre 1 y 4094';
                    }
                    return null;
                  },
                ),
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
            onPressed: _ocupado ? null : () => Navigator.of(contexto).pop(false),
            child: const Text('Cancelar'),
          ),
          FilledButton(
            onPressed: _ocupado ? null : _guardar,
            child: _ocupado
                ? const SizedBox(height: 16, width: 16, child: CircularProgressIndicator(strokeWidth: 2))
                : const Text('Agregar'),
          ),
        ],
      );
}
