import 'package:flutter/material.dart';

import '../servicios/api.dart';
import '../servicios/trayectoria.dart';
import '../widgets/mensajes.dart';
import 'redes.dart';

/// PantallaPrimerAcceso crea el administrador de una instalacion recien hecha.
///
/// **MiRed no trae ningun usuario ni clave de fabrica.** Unas credenciales
/// iguales en todas las instalaciones son, en un proyecto publico, una puerta
/// que cualquiera puede buscar: basta rastrear el puerto y probarlas. Que la
/// persona elija las suyas desde el primer momento es la unica forma de que no
/// se queden sin cambiar.
///
/// Esta pantalla solo aparece mientras no haya ningun usuario. En cuanto existe
/// el primero, la puerta se cierra tambien en el servidor.
class PantallaPrimerAcceso extends StatefulWidget {
  const PantallaPrimerAcceso({super.key, required this.operadores});

  /// Los signos que acepta el algoritmo TUXOR, ya separados para leerlos.
  final String operadores;

  @override
  State<PantallaPrimerAcceso> createState() => _PantallaPrimerAccesoState();
}

class _PantallaPrimerAccesoState extends State<PantallaPrimerAcceso> {
  final _formulario = GlobalKey<FormState>();
  final _usuario = TextEditingController();
  final _nombre = TextEditingController();
  final _clave = TextEditingController();
  final _repetida = TextEditingController();
  bool _ocupado = false;
  bool _verClave = false;

  @override
  void initState() {
    super.initState();
    // El aviso del operador tiene que aparecer en cuanto se teclea, no al
    // enviar: descubrir la regla despues de escribir una clave larga es de las
    // cosas que hacen abandonar un alta.
    _usuario.addListener(_revisar);
    _clave.addListener(_revisar);
  }

  void _revisar() => setState(() {});

  @override
  void dispose() {
    _usuario.dispose();
    _nombre.dispose();
    _clave.dispose();
    _repetida.dispose();
    super.dispose();
  }

  /// _tieneOperador replica la regla del algoritmo: el texto tiene que empezar o
  /// terminar con uno de esos signos y dejar algo mas.
  ///
  /// Se comprueba aqui Y en el servidor. Que el formulario avise no exime al
  /// servidor: un navegador viejo o una peticion hecha por fuera se saltan esto.
  static bool _tieneOperador(String texto) {
    const signos = '+-*%^&|<>#';
    var letras = texto;
    // El modificador @ va por fuera de los operadores.
    if (letras.startsWith('@@') || letras.endsWith('@@')) {
      letras = letras.replaceAll(RegExp(r'^@@|@@$'), '');
    } else if (letras.startsWith('@') || letras.endsWith('@')) {
      letras = letras.replaceAll(RegExp(r'^@|@$'), '');
    }
    if (letras.isEmpty) return false;

    final alPrincipio = signos.contains(letras[0]);
    final alFinal = signos.contains(letras[letras.length - 1]);
    if (!alPrincipio && !alFinal) return false;

    // Y tiene que quedar algo que no sea un signo.
    final resto = letras.replaceAll(RegExp('^[${RegExp.escape(signos)}]+'), '')
        .replaceAll(RegExp('[${RegExp.escape(signos)}]+\$'), '');
    return resto.isNotEmpty;
  }

  bool get _cumpleTuxor =>
      _tieneOperador(_usuario.text.trim()) || _tieneOperador(_clave.text);

  Future<void> _crear() async {
    if (!_formulario.currentState!.validate()) return;
    setState(() => _ocupado = true);
    Trayectoria.instancia.anotar('Primer acceso → crear administrador');

    try {
      await Api.instancia.crearPrimerAdministrador(
        _usuario.text.trim(),
        _nombre.text.trim(),
        _clave.text,
      );
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
            constraints: const BoxConstraints(maxWidth: 420),
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
                      Text('Primer acceso',
                          textAlign: TextAlign.center,
                          style: Theme.of(contexto).textTheme.titleSmall),
                      const SizedBox(height: 16),
                      Text(
                        'MiRed no trae ningun usuario de fabrica: elija usted el suyo. '
                        'Sera el administrador, y desde el se dan de alta los demas.',
                        textAlign: TextAlign.center,
                        style: Theme.of(contexto).textTheme.bodySmall,
                      ),
                      const SizedBox(height: 24),
                      TextFormField(
                        controller: _usuario,
                        autofocus: true,
                        maxLength: 60, // lo mismo que acepta la columna
                        decoration: const InputDecoration(
                          labelText: 'Usuario',
                          hintText: '+admin',
                          border: OutlineInputBorder(),
                          counterText: '',
                        ),
                        validator: (valor) => (valor == null || valor.trim().isEmpty)
                            ? 'Capture el usuario'
                            : null,
                      ),
                      const SizedBox(height: 16),
                      TextFormField(
                        controller: _nombre,
                        maxLength: 120, // lo mismo que acepta la columna
                        decoration: const InputDecoration(
                          labelText: 'Nombre de la persona',
                          border: OutlineInputBorder(),
                          counterText: '',
                        ),
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
                        validator: (valor) => (valor == null || valor.length < 8)
                            ? 'La clave debe tener al menos 8 caracteres'
                            : null,
                      ),
                      const SizedBox(height: 16),
                      TextFormField(
                        controller: _repetida,
                        obscureText: !_verClave,
                        decoration: const InputDecoration(
                          labelText: 'Repita la clave',
                          border: OutlineInputBorder(),
                        ),
                        validator: (valor) =>
                            valor != _clave.text ? 'Las dos claves no coinciden' : null,
                        onFieldSubmitted: (_) => _crear(),
                      ),
                      const SizedBox(height: 16),
                      _AvisoOperadores(
                        operadores: widget.operadores,
                        cumple: _cumpleTuxor,
                      ),
                      const SizedBox(height: 20),
                      FilledButton(
                        onPressed: _ocupado || !_cumpleTuxor ? null : _crear,
                        child: _ocupado
                            ? const SizedBox(
                                height: 18, width: 18,
                                child: CircularProgressIndicator(strokeWidth: 2))
                            : const Text('Crear administrador y entrar'),
                      ),
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

/// _AvisoOperadores explica la regla del algoritmo TUXOR y dice si ya se cumple.
///
/// Es la regla que mas sorprende de todo el alta, asi que se explica antes y se
/// marca en verde en cuanto se cumple, en vez de rechazar al final sin decir por
/// que.
class _AvisoOperadores extends StatelessWidget {
  const _AvisoOperadores({required this.operadores, required this.cumple});

  final String operadores;
  final bool cumple;

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: cumple ? colores.secondaryContainer : colores.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(cumple ? Icons.check_circle_outline : Icons.info_outline,
              size: 20,
              color: cumple ? colores.primary : colores.onSurfaceVariant),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              cumple
                  ? 'Listo: ya lleva uno de los signos que hacen falta.'
                  : 'El usuario o la clave deben empezar o terminar con uno de estos '
                      'signos:  $operadores\n\nPor ejemplo: «+admin» o «*miclave#».',
              style: Theme.of(contexto).textTheme.bodySmall,
            ),
          ),
        ],
      ),
    );
  }
}
