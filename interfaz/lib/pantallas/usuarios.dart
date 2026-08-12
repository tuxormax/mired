import 'package:flutter/material.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../servicios/trayectoria.dart';
import '../widgets/mensajes.dart';

/// PantallaUsuarios es solo para el administrador del sistema: dar de alta
/// gente y decidir que red ve cada quien.
class PantallaUsuarios extends StatefulWidget {
  const PantallaUsuarios({super.key});

  @override
  State<PantallaUsuarios> createState() => _PantallaUsuariosState();
}

class _PantallaUsuariosState extends State<PantallaUsuarios> {
  late Future<List<Usuario>> _usuarios;

  @override
  void initState() {
    super.initState();
    _recargar();
  }

  void _recargar() {
    setState(() => _usuarios = Api.instancia.listarUsuarios());
  }

  Future<void> _crear() async {
    final creado = await showDialog<bool>(
      context: context,
      builder: (_) => const _DialogoNuevoUsuario(),
    );
    if (creado == true) _recargar();
  }

  Future<void> _permisos(Usuario usuario) async {
    final cambio = await showDialog<bool>(
      context: context,
      builder: (_) => _DialogoPermisos(usuario: usuario),
    );
    if (cambio == true) _recargar();
  }

  Future<void> _borrar(Usuario usuario) async {
    final confirma = await showDialog<bool>(
      context: context,
      builder: (contextoModal) => AlertDialog(
        title: const Text('Borrar usuario'),
        content: Text('Se le quitara el acceso a ${usuario.nombre}. '
            'Se puede volver a dar de alta despues con el mismo usuario.'),
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
      Trayectoria.instancia.anotar('Borrar usuario ${usuario.usuario}');
      await Api.instancia.borrarUsuario(usuario.id);
      _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  @override
  Widget build(BuildContext contexto) => Scaffold(
        appBar: AppBar(title: const Text('Usuarios')),
        floatingActionButton: FloatingActionButton.extended(
          onPressed: _crear,
          icon: const Icon(Icons.person_add_outlined),
          label: const Text('Nuevo usuario'),
        ),
        body: FutureBuilder<List<Usuario>>(
          future: _usuarios,
          builder: (_, resultado) {
            if (resultado.connectionState != ConnectionState.done) {
              return const Center(child: CircularProgressIndicator());
            }
            if (resultado.hasError) {
              return Center(
                child: TextButton(
                  onPressed: () => mostrarProblema(contexto, resultado.error!),
                  child: const Text('No se pudieron cargar los usuarios. Ver detalles'),
                ),
              );
            }

            final usuarios = resultado.data ?? [];
            return ListView.separated(
              padding: const EdgeInsets.all(16),
              itemCount: usuarios.length,
              separatorBuilder: (_, __) => const Divider(height: 1),
              itemBuilder: (_, indice) {
                final usuario = usuarios[indice];
                return ListTile(
                  leading: CircleAvatar(
                    child: Text(usuario.nombre.isEmpty ? '?' : usuario.nombre[0].toUpperCase()),
                  ),
                  title: Row(
                    children: [
                      Text(usuario.nombre),
                      if (usuario.superadmin) ...[
                        const SizedBox(width: 8),
                        const Chip(
                          label: Text('Administrador'),
                          visualDensity: VisualDensity.compact,
                        ),
                      ],
                    ],
                  ),
                  subtitle: Text(usuario.usuario),
                  trailing: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (!usuario.superadmin)
                        IconButton(
                          tooltip: 'Redes que puede ver',
                          icon: const Icon(Icons.key_outlined),
                          onPressed: () => _permisos(usuario),
                        ),
                      if (!usuario.superadmin)
                        IconButton(
                          tooltip: 'Borrar',
                          icon: const Icon(Icons.delete_outline),
                          onPressed: () => _borrar(usuario),
                        ),
                    ],
                  ),
                );
              },
            );
          },
        ),
      );
}

class _DialogoNuevoUsuario extends StatefulWidget {
  const _DialogoNuevoUsuario();

  @override
  State<_DialogoNuevoUsuario> createState() => _DialogoNuevoUsuarioState();
}

class _DialogoNuevoUsuarioState extends State<_DialogoNuevoUsuario> {
  final _formulario = GlobalKey<FormState>();
  final _usuario = TextEditingController();
  final _nombre = TextEditingController();
  final _clave = TextEditingController();
  final _correo = TextEditingController();
  bool _superadmin = false;
  bool _ocupado = false;

  @override
  void dispose() {
    _usuario.dispose();
    _nombre.dispose();
    _clave.dispose();
    _correo.dispose();
    super.dispose();
  }

  Future<void> _guardar() async {
    if (!_formulario.currentState!.validate()) return;
    setState(() => _ocupado = true);
    Trayectoria.instancia.anotar('Crear usuario ${_usuario.text}');

    try {
      await Api.instancia.crearUsuario(
        _usuario.text.trim(),
        _nombre.text.trim(),
        _clave.text,
        _correo.text.trim(),
        _superadmin,
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
        title: const Text('Nuevo usuario'),
        content: SizedBox(
          width: 420,
          child: Form(
            key: _formulario,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextFormField(
                  controller: _usuario,
                  autofocus: true,
                  maxLength: 60, // lo mismo que acepta la columna
                  decoration: const InputDecoration(
                    labelText: 'Usuario de acceso',
                    border: OutlineInputBorder(),
                    counterText: '',
                  ),
                  validator: (valor) =>
                      (valor == null || valor.trim().isEmpty) ? 'Capture el usuario' : null,
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _nombre,
                  maxLength: 120,
                  decoration: const InputDecoration(
                    labelText: 'Nombre de la persona',
                    border: OutlineInputBorder(),
                    counterText: '',
                  ),
                  validator: (valor) =>
                      (valor == null || valor.trim().isEmpty) ? 'Capture el nombre' : null,
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _clave,
                  obscureText: true,
                  decoration: const InputDecoration(
                    labelText: 'Clave',
                    helperText: 'Al menos 8 caracteres',
                    border: OutlineInputBorder(),
                  ),
                  // La misma regla que el servidor, para que el rechazo no
                  // llegue despues de mandar el formulario.
                  validator: (valor) =>
                      (valor == null || valor.length < 8) ? 'Al menos 8 caracteres' : null,
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _correo,
                  maxLength: 120,
                  keyboardType: TextInputType.emailAddress,
                  decoration: const InputDecoration(
                    labelText: 'Correo (opcional)',
                    border: OutlineInputBorder(),
                    counterText: '',
                  ),
                ),
                SwitchListTile(
                  value: _superadmin,
                  onChanged: (valor) => setState(() => _superadmin = valor),
                  title: const Text('Administrador del sistema'),
                  subtitle: const Text('Ve todas las redes y administra usuarios'),
                  contentPadding: EdgeInsets.zero,
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
                : const Text('Crear'),
          ),
        ],
      );
}

/// _DialogoPermisos decide que redes ve un usuario. Es la contraparte de la base
/// por red: el acceso tambien se otorga sitio por sitio.
class _DialogoPermisos extends StatefulWidget {
  const _DialogoPermisos({required this.usuario});

  final Usuario usuario;

  @override
  State<_DialogoPermisos> createState() => _DialogoPermisosState();
}

class _DialogoPermisosState extends State<_DialogoPermisos> {
  late Future<List<Red>> _redes;
  final Map<String, String?> _niveles = {};
  bool _huboCambio = false;

  @override
  void initState() {
    super.initState();
    _redes = Api.instancia.listarRedes();
  }

  Future<void> _cambiar(String clave, String? nivel) async {
    try {
      if (nivel == null) {
        await Api.instancia.quitarPermiso(widget.usuario.id, clave);
      } else {
        await Api.instancia.otorgarPermiso(widget.usuario.id, clave, nivel);
      }
      setState(() {
        _niveles[clave] = nivel;
        _huboCambio = true;
      });
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  @override
  Widget build(BuildContext contexto) => AlertDialog(
        title: Text('Redes de ${widget.usuario.nombre}'),
        content: SizedBox(
          width: 480,
          child: FutureBuilder<List<Red>>(
            future: _redes,
            builder: (_, resultado) {
              if (resultado.connectionState != ConnectionState.done) {
                return const SizedBox(height: 120, child: Center(child: CircularProgressIndicator()));
              }
              final redes = resultado.data ?? [];
              if (redes.isEmpty) {
                return const Text('Todavia no hay redes que asignar.');
              }
              return SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    for (final red in redes)
                      ListTile(
                        contentPadding: EdgeInsets.zero,
                        title: Text(red.nombre),
                        trailing: DropdownButton<String?>(
                          value: _niveles[red.clave],
                          hint: const Text('Sin acceso'),
                          items: const [
                            DropdownMenuItem<String?>(value: null, child: Text('Sin acceso')),
                            DropdownMenuItem<String?>(value: 'lectura', child: Text('Lectura')),
                            DropdownMenuItem<String?>(value: 'escritura', child: Text('Escritura')),
                            DropdownMenuItem<String?>(value: 'administra', child: Text('Administra')),
                          ],
                          onChanged: (nivel) => _cambiar(red.clave, nivel),
                        ),
                      ),
                  ],
                ),
              );
            },
          ),
        ),
        actions: [
          FilledButton(
            onPressed: () => Navigator.of(contexto).pop(_huboCambio),
            child: const Text('Listo'),
          ),
        ],
      );
}
