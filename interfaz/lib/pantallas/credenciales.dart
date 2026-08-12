import 'package:flutter/material.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../servicios/trayectoria.dart';
import '../widgets/mensajes.dart';

/// PantallaCredenciales administra las credenciales SNMP.
///
/// Son lo unico que le falta a MiRed para poder decir en que puerto de que
/// switch esta cada aparato: sin ellas el mapa de puertos no existe, por mas
/// administrable que sea el switch.
class PantallaCredenciales extends StatefulWidget {
  const PantallaCredenciales({super.key});

  @override
  State<PantallaCredenciales> createState() => _PantallaCredencialesState();
}

class _PantallaCredencialesState extends State<PantallaCredenciales> {
  late Future<List<CredencialSNMP>> _credenciales;

  @override
  void initState() {
    super.initState();
    _recargar();
  }

  void _recargar() {
    setState(() => _credenciales = Api.instancia.listarCredenciales());
  }

  Future<void> _crear() async {
    final creada = await showDialog<bool>(
      context: context,
      builder: (_) => const _DialogoCredencial(),
    );
    if (creada == true) _recargar();
  }

  Future<void> _borrar(CredencialSNMP credencial) async {
    final confirma = await showDialog<bool>(
      context: context,
      builder: (contextoModal) => AlertDialog(
        title: const Text('Borrar credencial'),
        content: Text('Se dejara de usar "${credencial.nombre}" para consultar switches. '
            'Las redes que dependan de ella se quedaran sin mapa de puertos.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(contextoModal).pop(false),
              child: const Text('Cancelar')),
          FilledButton(
              onPressed: () => Navigator.of(contextoModal).pop(true),
              child: const Text('Borrar')),
        ],
      ),
    );
    if (confirma != true) return;

    try {
      await Api.instancia.borrarCredencial(credencial.id);
      _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  @override
  Widget build(BuildContext contexto) => Scaffold(
        appBar: AppBar(title: const Text('Credenciales SNMP')),
        floatingActionButton: FloatingActionButton.extended(
          onPressed: _crear,
          icon: const Icon(Icons.add),
          label: const Text('Nueva credencial'),
        ),
        body: FutureBuilder<List<CredencialSNMP>>(
          future: _credenciales,
          builder: (_, resultado) {
            if (resultado.connectionState != ConnectionState.done) {
              return const Center(child: CircularProgressIndicator());
            }
            if (resultado.hasError) {
              return Center(
                child: TextButton(
                  onPressed: () => mostrarProblema(contexto, resultado.error!),
                  child: const Text('No se pudieron cargar las credenciales. Ver detalles'),
                ),
              );
            }

            final credenciales = resultado.data ?? [];
            return ListView(
              padding: const EdgeInsets.fromLTRB(16, 16, 16, 88),
              children: [
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Icon(Icons.info_outline),
                        const SizedBox(width: 12),
                        Expanded(
                          child: Text(
                            'Con estas credenciales MiRed le pregunta a los switches que hay '
                            'conectado en cada boca. Se prueban en orden contra cada equipo y se '
                            'usa la primera que conteste, asi que no hace falta decir cual va con '
                            'cual switch.',
                            style: Theme.of(contexto).textTheme.bodyMedium,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
                const SizedBox(height: 16),
                if (credenciales.isEmpty)
                  const Padding(
                    padding: EdgeInsets.all(24),
                    child: Text(
                      'Todavia no hay ninguna credencial.\\n'
                      'Sin ellas no se puede armar el mapa de puertos.',
                      textAlign: TextAlign.center,
                    ),
                  )
                else
                  Card(
                    margin: EdgeInsets.zero,
                    child: Column(
                      children: [
                        for (final credencial in credenciales)
                          ListTile(
                            leading: const Icon(Icons.vpn_key_outlined),
                            title: Text(credencial.nombre),
                            subtitle: Text([
                              'SNMP ${credencial.version}',
                              if (credencial.usuario.isNotEmpty) 'usuario ${credencial.usuario}',
                            ].join(' · ')),
                            trailing: IconButton(
                              tooltip: 'Borrar',
                              icon: const Icon(Icons.delete_outline),
                              onPressed: () => _borrar(credencial),
                            ),
                          ),
                      ],
                    ),
                  ),
              ],
            );
          },
        ),
      );
}

class _DialogoCredencial extends StatefulWidget {
  const _DialogoCredencial();

  @override
  State<_DialogoCredencial> createState() => _DialogoCredencialState();
}

class _DialogoCredencialState extends State<_DialogoCredencial> {
  final _formulario = GlobalKey<FormState>();
  final _nombre = TextEditingController();
  final _comunidad = TextEditingController();
  final _usuario = TextEditingController();
  final _claveAuth = TextEditingController();
  final _clavePriv = TextEditingController();

  String _version = 'v2c';
  String _protocoloAuth = 'SHA';
  String _protocoloPriv = 'AES';
  bool _ocupado = false;

  @override
  void dispose() {
    _nombre.dispose();
    _comunidad.dispose();
    _usuario.dispose();
    _claveAuth.dispose();
    _clavePriv.dispose();
    super.dispose();
  }

  bool get _esV3 => _version == 'v3';

  Future<void> _guardar() async {
    if (!_formulario.currentState!.validate()) return;
    setState(() => _ocupado = true);
    Trayectoria.instancia.anotar('Crear credencial SNMP ${_nombre.text}');

    try {
      await Api.instancia.crearCredencial({
        'nombre': _nombre.text.trim(),
        'version': _version,
        if (!_esV3) 'comunidad': _comunidad.text,
        if (_esV3) 'usuario': _usuario.text.trim(),
        if (_esV3 && _claveAuth.text.isNotEmpty) 'autenticacionProtocolo': _protocoloAuth,
        if (_esV3 && _claveAuth.text.isNotEmpty) 'autenticacionClave': _claveAuth.text,
        if (_esV3 && _clavePriv.text.isNotEmpty) 'privacidadProtocolo': _protocoloPriv,
        if (_esV3 && _clavePriv.text.isNotEmpty) 'privacidadClave': _clavePriv.text,
      });
      if (mounted) Navigator.of(context).pop(true);
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _ocupado = false);
    }
  }

  @override
  Widget build(BuildContext contexto) => AlertDialog(
        title: const Text('Nueva credencial SNMP'),
        content: SizedBox(
          width: 460,
          child: SingleChildScrollView(
            child: Form(
              key: _formulario,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  TextFormField(
                    controller: _nombre,
                    autofocus: true,
                    maxLength: 80,
                    decoration: const InputDecoration(
                      labelText: 'Nombre',
                      hintText: 'Switches de matriz, Lectura general...',
                      border: OutlineInputBorder(),
                      counterText: '',
                    ),
                    validator: (valor) =>
                        (valor == null || valor.trim().isEmpty) ? 'Capture el nombre' : null,
                  ),
                  const SizedBox(height: 12),
                  DropdownButtonFormField<String>(
                    initialValue: _version,
                    decoration: const InputDecoration(
                      labelText: 'Version de SNMP',
                      border: OutlineInputBorder(),
                    ),
                    // Enumerado, nunca texto libre: son las tres versiones que
                    // existen y la base solo acepta esas.
                    items: const [
                      DropdownMenuItem(value: 'v1', child: Text('v1 (vieja)')),
                      DropdownMenuItem(value: 'v2c', child: Text('v2c (la mas comun)')),
                      DropdownMenuItem(value: 'v3', child: Text('v3 (con usuario y cifrado)')),
                    ],
                    onChanged: (valor) => setState(() => _version = valor ?? 'v2c'),
                  ),
                  const SizedBox(height: 12),
                  if (!_esV3)
                    TextFormField(
                      controller: _comunidad,
                      decoration: const InputDecoration(
                        labelText: 'Comunidad',
                        hintText: 'public',
                        helperText: 'Es la contrasena de lectura del switch',
                        border: OutlineInputBorder(),
                      ),
                      validator: (valor) => (!_esV3 && (valor == null || valor.isEmpty))
                          ? 'Capture la comunidad'
                          : null,
                    )
                  else ...[
                    TextFormField(
                      controller: _usuario,
                      decoration: const InputDecoration(
                        labelText: 'Usuario',
                        border: OutlineInputBorder(),
                      ),
                      validator: (valor) => (_esV3 && (valor == null || valor.trim().isEmpty))
                          ? 'Capture el usuario'
                          : null,
                    ),
                    const SizedBox(height: 12),
                    Row(
                      children: [
                        Expanded(
                          child: DropdownButtonFormField<String>(
                            initialValue: _protocoloAuth,
                            decoration: const InputDecoration(
                              labelText: 'Autenticacion',
                              border: OutlineInputBorder(),
                            ),
                            items: const [
                              DropdownMenuItem(value: 'MD5', child: Text('MD5')),
                              DropdownMenuItem(value: 'SHA', child: Text('SHA')),
                              DropdownMenuItem(value: 'SHA224', child: Text('SHA224')),
                              DropdownMenuItem(value: 'SHA256', child: Text('SHA256')),
                              DropdownMenuItem(value: 'SHA384', child: Text('SHA384')),
                              DropdownMenuItem(value: 'SHA512', child: Text('SHA512')),
                            ],
                            onChanged: (valor) => setState(() => _protocoloAuth = valor ?? 'SHA'),
                          ),
                        ),
                        const SizedBox(width: 12),
                        Expanded(
                          child: TextFormField(
                            controller: _claveAuth,
                            obscureText: true,
                            decoration: const InputDecoration(
                              labelText: 'Clave',
                              border: OutlineInputBorder(),
                            ),
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 12),
                    Row(
                      children: [
                        Expanded(
                          child: DropdownButtonFormField<String>(
                            initialValue: _protocoloPriv,
                            decoration: const InputDecoration(
                              labelText: 'Privacidad',
                              border: OutlineInputBorder(),
                            ),
                            items: const [
                              DropdownMenuItem(value: 'DES', child: Text('DES')),
                              DropdownMenuItem(value: 'AES', child: Text('AES')),
                              DropdownMenuItem(value: 'AES192', child: Text('AES192')),
                              DropdownMenuItem(value: 'AES256', child: Text('AES256')),
                            ],
                            onChanged: (valor) => setState(() => _protocoloPriv = valor ?? 'AES'),
                          ),
                        ),
                        const SizedBox(width: 12),
                        Expanded(
                          child: TextFormField(
                            controller: _clavePriv,
                            obscureText: true,
                            decoration: const InputDecoration(
                              labelText: 'Clave',
                              border: OutlineInputBorder(),
                            ),
                          ),
                        ),
                      ],
                    ),
                  ],
                ],
              ),
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
                : const Text('Guardar'),
          ),
        ],
      );
}
