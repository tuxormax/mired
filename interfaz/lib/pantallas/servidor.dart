import 'package:flutter/material.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';

/// DialogoServidor deja apuntar el programa a otro MiRed.
///
/// **Solo tiene sentido en el programa de escritorio.** Cuando MiRed se abre
/// desde el navegador, el servidor es siempre el que sirvio la pagina y no hay
/// nada que elegir; el programa, en cambio, esta instalado en un equipo y puede
/// querer hablarle al MiRed de otro sitio —la Raspberry de la sucursal, el
/// servidor de la matriz—. Eso es justo lo que gana teniendo un programa y no
/// solo la pagina.
///
/// Cambiar de servidor **cierra la sesion**: una sesion es de un servidor, no
/// del programa, y arrastrarla seria mandar el token de uno al otro.
class DialogoServidor extends StatefulWidget {
  const DialogoServidor({super.key});

  @override
  State<DialogoServidor> createState() => _DialogoServidorState();
}

class _DialogoServidorState extends State<DialogoServidor> {
  final _formulario = GlobalKey<FormState>();
  late final TextEditingController _direccion;
  bool _probando = false;
  String? _resultado;
  bool _contesto = false;

  @override
  void initState() {
    super.initState();
    _direccion = TextEditingController(text: Api.instancia.servidor);
  }

  @override
  void dispose() {
    _direccion.dispose();
    super.dispose();
  }

  /// _probar comprueba que ahi hay un MiRed ANTES de guardar la direccion.
  ///
  /// Sin esto, una direccion mal escrita se guarda, cierra la sesion y deja el
  /// programa sin poder entrar a ningun lado ni volver atras facilmente.
  Future<void> _probar() async {
    if (!_formulario.currentState!.validate()) return;
    setState(() {
      _probando = true;
      _resultado = null;
      _contesto = false;
    });

    final anterior = Api.instancia.servidor;
    try {
      Api.instancia.servidor = _direccion.text.trim().replaceAll(RegExp(r'/+$'), '');
      final estado = await Api.instancia.estado();
      if (estado['servicio'] != 'MiRed') {
        throw const FormatException('ahi contesta algo, pero no es MiRed');
      }
      setState(() {
        _contesto = true;
        _resultado = 'Contesta MiRed ${estado['version']} Rev ${estado['revision']}';
      });
    } catch (problema) {
      setState(() {
        _contesto = false;
        _resultado = 'No contesta: ${problema is ErrorMiRed ? problema.mensaje : problema}';
      });
    } finally {
      // Se deja como estaba hasta que el usuario acepte: probar no es cambiar.
      Api.instancia.servidor = anterior;
      if (mounted) setState(() => _probando = false);
    }
  }

  Future<void> _guardar() async {
    if (!_formulario.currentState!.validate()) return;
    await Api.instancia.guardarServidor(_direccion.text);
    if (mounted) Navigator.of(context).pop(true);
  }

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;

    return AlertDialog(
      title: const Text('A que MiRed conectarse'),
      content: SizedBox(
        width: 440,
        child: Form(
          key: _formulario,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                'Este programa habla con un servidor MiRed. Por omision, el de este '
                'mismo equipo. Puede apuntarlo al de otro sitio poniendo su direccion.',
                style: Theme.of(contexto).textTheme.bodySmall,
              ),
              const SizedBox(height: 16),
              TextFormField(
                controller: _direccion,
                autofocus: true,
                maxLength: 200,
                decoration: const InputDecoration(
                  labelText: 'Direccion del servidor',
                  hintText: Api.servidorPorOmision,
                  border: OutlineInputBorder(),
                  counterText: '',
                ),
                validator: (valor) {
                  final texto = (valor ?? '').trim();
                  if (texto.isEmpty) return 'Capture la direccion';
                  final direccion = Uri.tryParse(texto);
                  if (direccion == null ||
                      !(direccion.isScheme('http') || direccion.isScheme('https')) ||
                      direccion.host.isEmpty) {
                    return 'Debe ser como http://192.168.1.10:60072';
                  }
                  return null;
                },
              ),
              const SizedBox(height: 8),
              if (_resultado != null)
                Row(
                  children: [
                    Icon(_contesto ? Icons.check_circle_outline : Icons.error_outline,
                        size: 18, color: _contesto ? colores.primary : colores.error),
                    const SizedBox(width: 6),
                    Expanded(
                      child: Text(_resultado!,
                          style: Theme.of(contexto).textTheme.bodySmall?.copyWith(
                              color: _contesto ? null : colores.error)),
                    ),
                  ],
                ),
              const SizedBox(height: 8),
              Text(
                'Al cambiar de servidor se cierra la sesion: una sesion es de un '
                'servidor, no del programa.',
                style: Theme.of(contexto).textTheme.labelSmall,
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _probando ? null : () => Navigator.of(contexto).pop(false),
          child: const Text('Cancelar'),
        ),
        TextButton(
          onPressed: _probando ? null : _probar,
          child: _probando
              ? const SizedBox(
                  height: 16, width: 16, child: CircularProgressIndicator(strokeWidth: 2))
              : const Text('Probar'),
        ),
        FilledButton(
          onPressed: _probando ? null : _guardar,
          child: const Text('Conectar'),
        ),
      ],
    );
  }
}
