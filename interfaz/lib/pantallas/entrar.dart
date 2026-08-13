import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';

import '../servicios/api.dart';
import '../servicios/trayectoria.dart';
import '../widgets/mensajes.dart';
import 'redes.dart';
import 'servidor.dart';

class PantallaEntrar extends StatefulWidget {
  const PantallaEntrar({super.key});

  @override
  State<PantallaEntrar> createState() => _PantallaEntrarState();
}

class _PantallaEntrarState extends State<PantallaEntrar> {
  final _formulario = GlobalKey<FormState>();
  final _usuario = TextEditingController();
  final _clave = TextEditingController();
  bool _ocupado = false;
  bool _verClave = false;

  @override
  void dispose() {
    _usuario.dispose();
    _clave.dispose();
    super.dispose();
  }

  /// _cambiarServidor apunta el programa a otro MiRed.
  ///
  /// Vive en esta pantalla porque es donde hace falta: si el servidor de este
  /// equipo no esta corriendo, o se quiere ver el de otro sitio, aqui es donde
  /// uno se da cuenta.
  Future<void> _cambiarServidor() async {
    final cambio = await showDialog<bool>(
      context: context,
      builder: (_) => const DialogoServidor(),
    );
    if (cambio == true && mounted) setState(() {});
  }

  Future<void> _entrar() async {
    if (!_formulario.currentState!.validate()) return;
    setState(() => _ocupado = true);
    Trayectoria.instancia.anotar('Entrar → ${_usuario.text}');

    try {
      await Api.instancia.entrar(_usuario.text.trim(), _clave.text);
      await Api.instancia.recuperarSesion();
      if (!mounted) return;
      Navigator.of(context).pushReplacement(
        MaterialPageRoute<void>(builder: (_) => const PantallaRedes()),
      );
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _ocupado = false);
    }
  }

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;

    return Scaffold(
      body: Center(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 380),
            child: Card(
              child: Padding(
                padding: const EdgeInsets.all(28),
                child: Form(
                  key: _formulario,
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Icon(Icons.lan_outlined, size: 48, color: colores.primary),
                      const SizedBox(height: 12),
                      Text('MiRed',
                          textAlign: TextAlign.center,
                          style: Theme.of(contexto).textTheme.headlineMedium),
                      const SizedBox(height: 4),
                      Text('Mapeo de redes',
                          textAlign: TextAlign.center,
                          style: Theme.of(contexto).textTheme.bodySmall),
                      const SizedBox(height: 28),
                      TextFormField(
                        controller: _usuario,
                        autofocus: true,
                        maxLength: 60, // lo mismo que acepta la columna
                        decoration: const InputDecoration(
                          labelText: 'Usuario',
                          border: OutlineInputBorder(),
                          counterText: '',
                        ),
                        validator: (valor) =>
                            (valor == null || valor.trim().isEmpty) ? 'Capture su usuario' : null,
                        onFieldSubmitted: (_) => _entrar(),
                      ),
                      const SizedBox(height: 16),
                      TextFormField(
                        controller: _clave,
                        obscureText: !_verClave,
                        decoration: InputDecoration(
                          labelText: 'Clave',
                          border: const OutlineInputBorder(),
                          suffixIcon: IconButton(
                            icon: Icon(_verClave ? Icons.visibility_off : Icons.visibility),
                            onPressed: () => setState(() => _verClave = !_verClave),
                          ),
                        ),
                        validator: (valor) =>
                            (valor == null || valor.isEmpty) ? 'Capture su clave' : null,
                        onFieldSubmitted: (_) => _entrar(),
                      ),
                      const SizedBox(height: 24),
                      FilledButton(
                        onPressed: _ocupado ? null : _entrar,
                        child: _ocupado
                            ? const SizedBox(
                                height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2))
                            : const Text('Entrar'),
                      ),
                      // Solo en el programa: en el navegador el servidor es
                      // siempre el que sirvio la pagina y no hay nada que elegir.
                      if (!kIsWeb) ...[
                        const SizedBox(height: 8),
                        TextButton.icon(
                          onPressed: _ocupado ? null : _cambiarServidor,
                          icon: const Icon(Icons.dns_outlined, size: 18),
                          label: Text(Api.instancia.servidor,
                              style: Theme.of(contexto).textTheme.labelSmall),
                        ),
                      ],
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
