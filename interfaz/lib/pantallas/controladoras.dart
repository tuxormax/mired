import 'package:flutter/material.dart';

import '../servicios/api.dart';
import '../servicios/trayectoria.dart';
import '../widgets/mensajes.dart';

/// DialogoControladora captura como se entra a la controladora del WiFi.
///
/// Una controladora no es un aparato mas: es quien SABE que hay colgado de cada
/// antena. Sin ella, en una oficina la mitad de los equipos salen «sin ubicar».
class DialogoControladora extends StatefulWidget {
  const DialogoControladora({super.key, required this.clave});

  final String clave;

  @override
  State<DialogoControladora> createState() => DialogoControladoraState();
}

class DialogoControladoraState extends State<DialogoControladora> {
  final _formulario = GlobalKey<FormState>();
  final _nombre = TextEditingController();
  final _url = TextEditingController(text: 'https://');
  final _usuario = TextEditingController();
  final _clave = TextEditingController();
  final _sitio = TextEditingController(text: 'default');

  bool _verificarTls = false;
  bool _ocupado = false;

  @override
  void dispose() {
    _nombre.dispose();
    _url.dispose();
    _usuario.dispose();
    _clave.dispose();
    _sitio.dispose();
    super.dispose();
  }

  Future<void> _guardar() async {
    if (!_formulario.currentState!.validate()) return;
    setState(() => _ocupado = true);
    Trayectoria.instancia.anotar('Crear controladora ${_nombre.text}');

    try {
      await Api.instancia.crearControladora(widget.clave, {
        'nombre': _nombre.text.trim(),
        'tipo': 'unifi',
        'url': _url.text.trim(),
        'usuario': _usuario.text.trim(),
        'clave': _clave.text,
        'sitio': _sitio.text.trim().isEmpty ? 'default' : _sitio.text.trim(),
        'verificarTls': _verificarTls,
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
        title: const Text('Agregar la controladora del WiFi'),
        content: SizedBox(
          width: 460,
          child: SingleChildScrollView(
            child: Form(
              key: _formulario,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Que es una controladora, para quien nunca ha oido la
                  // palabra. Con un ejemplo de marca, que es como la reconoce
                  // quien la tiene puesta en su casa u oficina.
                  Text(
                    'Es el aparato o el programa que manda sobre sus antenas WiFi '
                    '—una UniFi, un Cloud Key—. Sin preguntarle a ella no hay forma de '
                    'saber que telefono o que laptop esta colgado de cada antena.',
                    style: Theme.of(contexto).textTheme.bodyMedium,
                  ),
                  const SizedBox(height: 16),
                  TextFormField(
                    controller: _nombre,
                    autofocus: true,
                    maxLength: 80,
                    decoration: const InputDecoration(
                      labelText: 'Nombre',
                      hintText: 'UniFi de matriz, Cloud Key bodega...',
                      border: OutlineInputBorder(),
                      counterText: '',
                    ),
                    validator: (valor) =>
                        (valor == null || valor.trim().isEmpty) ? 'Capture el nombre' : null,
                  ),
                  const SizedBox(height: 12),
                  // El tipo es un enumerado, nunca texto libre: la base solo
                  // acepta los que MiRed sabe consultar.
                  DropdownButtonFormField<String>(
                    initialValue: 'unifi',
                    decoration: const InputDecoration(
                      labelText: 'Tipo',
                      border: OutlineInputBorder(),
                    ),
                    items: const [
                      DropdownMenuItem(value: 'unifi', child: Text('UniFi (Ubiquiti)')),
                    ],
                    onChanged: null,
                  ),
                  const SizedBox(height: 12),
                  TextFormField(
                    controller: _url,
                    maxLength: 200,
                    decoration: const InputDecoration(
                      labelText: 'Direccion de su pagina',
                      hintText: 'https://192.168.1.10:8443',
                      helperText: 'La misma que escribe usted en el navegador para entrar '
                          'a administrarla',
                      border: OutlineInputBorder(),
                      counterText: '',
                    ),
                    validator: (valor) {
                      final texto = (valor ?? '').trim();
                      if (texto.isEmpty) return 'Capture la direccion';
                      // Se valida aqui igual que en el servidor: una direccion
                      // mal escrita se descubriria dentro de un mes, cuando
                      // alguien note que el WiFi no sale en el mapa.
                      final direccion = Uri.tryParse(texto);
                      if (direccion == null ||
                          !(direccion.isScheme('http') || direccion.isScheme('https')) ||
                          direccion.host.isEmpty) {
                        return 'Debe ser como https://192.168.1.10:8443';
                      }
                      return null;
                    },
                  ),
                  const SizedBox(height: 12),
                  TextFormField(
                    controller: _usuario,
                    maxLength: 80,
                    decoration: const InputDecoration(
                      labelText: 'Usuario',
                      helperText: 'Con el que entra a su pagina. Basta uno de solo lectura',
                      helperMaxLines: 2,
                      border: OutlineInputBorder(),
                      counterText: '',
                    ),
                    validator: (valor) =>
                        (valor == null || valor.trim().isEmpty) ? 'Capture el usuario' : null,
                  ),
                  const SizedBox(height: 12),
                  TextFormField(
                    controller: _clave,
                    obscureText: true,
                    maxLength: 200,
                    decoration: const InputDecoration(
                      labelText: 'Clave',
                      border: OutlineInputBorder(),
                      counterText: '',
                    ),
                    validator: (valor) =>
                        (valor == null || valor.isEmpty) ? 'Capture la clave' : null,
                  ),
                  const SizedBox(height: 12),
                  TextFormField(
                    controller: _sitio,
                    maxLength: 80,
                    decoration: const InputDecoration(
                      labelText: 'Sitio',
                      helperText: 'Una controladora puede llevar varias instalaciones; si '
                          'solo lleva la suya, dejelo en «default»',
                      helperMaxLines: 3,
                      border: OutlineInputBorder(),
                      counterText: '',
                    ),
                  ),
                  const SizedBox(height: 4),
                  SwitchListTile(
                    contentPadding: EdgeInsets.zero,
                    value: _verificarTls,
                    onChanged: (valor) => setState(() => _verificarTls = valor),
                    title: const Text('Exigir certificado valido'),
                    // Casi toda controladora casera usa un certificado que ella
                    // misma se firmo. Exigirlo dejaria la funcion inservible en
                    // la mayoria de las instalaciones.
                    subtitle: const Text('Dejelo apagado si la controladora usa su propio '
                        'certificado, que es lo normal'),
                  ),
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
