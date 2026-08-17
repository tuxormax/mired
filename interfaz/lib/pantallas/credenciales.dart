import 'package:flutter/material.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../servicios/trayectoria.dart';
import '../widgets/mensajes.dart';

/// DialogoCredencialSNMP captura la contrasena de lectura de los switches.
///
/// Es la unica credencial que NO es de un aparato concreto: se prueba contra
/// todos y se usa la que conteste, porque cuando se captura todavia no se sabe
/// cuales de los aparatos son switches administrables —eso es justo lo que MiRed
/// va a averiguar con ella—. En la lista de accesos aparece como «Todos».
///
/// Escrita para que la entienda quien no sabe que es SNMP **sin quitarsela al
/// que si**: un solo campo en cristiano, el nombre tecnico escrito al lado, lo
/// demas plegado, y un boton para PROBARLA antes de guardar. Ver [[gotchas]].
class DialogoCredencialSNMP extends StatefulWidget {
  const DialogoCredencialSNMP({super.key, required this.clave});

  final String clave;

  @override
  State<DialogoCredencialSNMP> createState() => _DialogoCredencialSNMPState();
}

class _DialogoCredencialSNMPState extends State<DialogoCredencialSNMP> {
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
  bool _probando = false;
  bool _tecnicoAbierto = false;
  bool _explicacionAbierta = false;
  PruebaDeCredencial? _resultado;

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
    Trayectoria.instancia.anotar('Crear credencial SNMP $_nombreFinal');

    try {
      await Api.instancia.crearCredencial(widget.clave, _comoQuedaria());
      if (mounted) Navigator.of(context).pop(true);
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _ocupado = false);
    }
  }

  /// _probar pregunta a los equipos de la red con lo que hay escrito, sin
  /// guardar nada.
  ///
  /// Es la pieza que hace usable esta pantalla para quien no sabe que es SNMP:
  /// sin ella se guarda una contrasena a ciegas, y si estaba mal nadie se entera
  /// —el mapa simplemente se queda sin puertos, tres dias despues, sin decir por
  /// que—. Con esto se pulsa un boton y contesta cuantos aparatos respondieron.
  Future<void> _probar() async {
    if (!_formulario.currentState!.validate()) return;
    setState(() {
      _probando = true;
      _resultado = null;
    });
    try {
      final resultado =
          await Api.instancia.probarCredencial(widget.clave, _comoQuedaria());
      if (mounted) setState(() => _resultado = resultado);
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _probando = false);
    }
  }

  /// _comoQuedaria arma lo que se mandaria al guardar. Lo usan la prueba y el
  /// guardado, para que no puedan diferir.
  Map<String, dynamic> _comoQuedaria() => {
        'nombre': _nombreFinal,
        'version': _version,
        if (!_esV3) 'comunidad': _comunidad.text,
        if (_esV3) 'usuario': _usuario.text.trim(),
        if (_esV3 && _claveAuth.text.isNotEmpty) 'autenticacionProtocolo': _protocoloAuth,
        if (_esV3 && _claveAuth.text.isNotEmpty) 'autenticacionClave': _claveAuth.text,
        if (_esV3 && _clavePriv.text.isNotEmpty) 'privacidadProtocolo': _protocoloPriv,
        if (_esV3 && _clavePriv.text.isNotEmpty) 'privacidadClave': _clavePriv.text,
      };

  /// _nombreFinal: si no le pusieron nombre, se le pone uno.
  ///
  /// Pedir un nombre para algo de lo que casi siempre hay UNO es pedir trabajo
  /// por nada. El experto que lleva cuatro se lo pone; el que lleva una, no
  /// tiene ni que pensarlo.
  String get _nombreFinal {
    final escrito = _nombre.text.trim();
    if (escrito.isNotEmpty) return escrito;
    return _esV3 ? 'Acceso SNMP v3' : 'Contrasena de lectura';
  }

  @override
  Widget build(BuildContext contexto) {
    final tema = Theme.of(contexto);
    return AlertDialog(
      title: const Text('Agregar la contrasena de un switch'),
      content: SizedBox(
        width: 520,
        child: SingleChildScrollView(
          child: Form(
            key: _formulario,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'La que trae el switch para DEJARSE LEER. No es la de entrar a '
                  'configurarlo: con esta solo se le puede preguntar, no cambiar nada.',
                  style: tema.textTheme.bodyMedium,
                ),
                const SizedBox(height: 16),

                if (!_esV3) ...[
                  TextFormField(
                    controller: _comunidad,
                    autofocus: true,
                    decoration: const InputDecoration(
                      labelText: 'Contrasena de lectura del switch',
                      hintText: 'public',
                      // El nombre tecnico va escrito, no escondido: quien sabe
                      // SNMP tiene que reconocer el campo de un vistazo, y quien
                      // no sabe se entera de como se llama esto en el manual del
                      // switch, que es donde va a tener que buscarlo.
                      helperText: 'En el switch aparece como «community» de solo lectura. '
                          'Si nadie la cambio, casi siempre es: public',
                      helperMaxLines: 3,
                      border: OutlineInputBorder(),
                    ),
                    validator: (valor) => (!_esV3 && (valor == null || valor.isEmpty))
                        ? 'Escriba la contrasena de lectura'
                        : null,
                  ),
                  const SizedBox(height: 8),
                  Text(
                    'Donde encontrarla: entre a la pagina del switch escribiendo su '
                    'direccion IP en el navegador, y busque la seccion «SNMP».',
                    style: tema.textTheme.bodySmall,
                  ),
                  _queEsEsto(contexto),
                ] else ...[
                  TextFormField(
                    controller: _usuario,
                    autofocus: true,
                    decoration: const InputDecoration(
                      labelText: 'Usuario',
                      helperText: 'El que le dieron para consultar el switch',
                      border: OutlineInputBorder(),
                    ),
                    validator: (valor) => (_esV3 && (valor == null || valor.trim().isEmpty))
                        ? 'Escriba el usuario'
                        : null,
                  ),
                  const SizedBox(height: 12),
                  Row(children: [
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
                          labelText: 'Clave', border: OutlineInputBorder()),
                      ),
                    ),
                  ]),
                  const SizedBox(height: 12),
                  Row(children: [
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
                          labelText: 'Clave', border: OutlineInputBorder()),
                      ),
                    ),
                  ]),
                ],

                // Probar ANTES de guardar. Es lo unico que convierte esta
                // pantalla en algo que se puede usar sin saber que es SNMP.
                const SizedBox(height: 16),
                Row(children: [
                  OutlinedButton.icon(
                    onPressed: _probando || _ocupado ? null : _probar,
                    icon: _probando
                        ? const SizedBox(
                            width: 16, height: 16,
                            child: CircularProgressIndicator(strokeWidth: 2))
                        : const Icon(Icons.wifi_tethering),
                    label: Text(_probando ? 'Preguntando...' : 'Probar ahora'),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text(
                      'Le pregunta a los aparatos de esta red y dice cuantos contestan. '
                      'No guarda nada.',
                      style: tema.textTheme.bodySmall,
                    ),
                  ),
                ]),
                if (_resultado != null) ...[
                  const SizedBox(height: 12),
                  _elResultado(contexto, _resultado!),
                ],

                // Lo tecnico, plegado: quien lleva veinte anos en redes lo abre y
                // encuentra la version y el v3 donde espera; a quien no le suena
                // de nada no le estorba.
                const SizedBox(height: 8),
                ExpansionTile(
                  initiallyExpanded: _tecnicoAbierto,
                  onExpansionChanged: (abierto) => _tecnicoAbierto = abierto,
                  tilePadding: EdgeInsets.zero,
                  childrenPadding: const EdgeInsets.only(bottom: 8),
                  title: Text('Opciones tecnicas (version de SNMP)',
                      style: tema.textTheme.bodyMedium),
                  children: [
                    DropdownButtonFormField<String>(
                      initialValue: _version,
                      decoration: const InputDecoration(
                        labelText: 'Version de SNMP',
                        border: OutlineInputBorder(),
                      ),
                      // Enumerado, nunca texto libre: son las tres versiones que
                      // existen y la base solo acepta esas.
                      items: const [
                        DropdownMenuItem(
                            value: 'v2c',
                            child: Text('v2c — con contrasena de lectura (lo normal)')),
                        DropdownMenuItem(
                            value: 'v1', child: Text('v1 — igual, pero de equipos viejos')),
                        DropdownMenuItem(
                            value: 'v3', child: Text('v3 — con usuario y cifrado')),
                      ],
                      onChanged: (valor) => setState(() {
                        _version = valor ?? 'v2c';
                        _resultado = null;
                      }),
                    ),
                    const SizedBox(height: 12),
                    TextFormField(
                      controller: _nombre,
                      maxLength: 80,
                      decoration: InputDecoration(
                        labelText: 'Como llamarla en la lista',
                        hintText: _nombreFinal,
                        helperText: 'Solo para reconocerla si llega a haber varias',
                        border: const OutlineInputBorder(),
                        counterText: '',
                      ),
                    ),
                  ],
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
              ? const SizedBox(
                  height: 16, width: 16, child: CircularProgressIndicator(strokeWidth: 2))
              : const Text('Guardar'),
        ),
      ],
    );
  }

  /// _queEsEsto contesta la pregunta que se hace cualquiera al llegar aqui.
  ///
  /// La pantalla ya decia COMO se llama el campo en el switch («community») y
  /// DONDE buscarlo, pero no QUE ES. El usuario lo pregunto en cuanto lo vio, y
  /// tenia razon: saber el nombre de algo no es entenderlo.
  ///
  /// Va plegado porque quien ya lo sabe no tiene por que leerlo cada vez, y va
  /// AQUI y no en un manual aparte porque es donde se hace la pregunta.
  Widget _queEsEsto(BuildContext contexto) {
    final tema = Theme.of(contexto);

    Widget parrafo(String texto) => Padding(
          padding: const EdgeInsets.only(bottom: 10),
          child: Text(texto, style: tema.textTheme.bodySmall),
        );

    return ExpansionTile(
      initiallyExpanded: _explicacionAbierta,
      onExpansionChanged: (abierto) => _explicacionAbierta = abierto,
      tilePadding: EdgeInsets.zero,
      childrenPadding: EdgeInsets.zero,
      title: Text('¿Que es esto de la «community»?',
          style: tema.textTheme.bodyMedium?.copyWith(color: tema.colorScheme.primary)),
      children: [
        Align(
          alignment: Alignment.centerLeft,
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            parrafo(
                'Es la contrasena de lectura del switch, y nada mas. El nombre es lo '
                'unico raro: cuando se invento SNMP, en los ochenta, se llamaba '
                '«comunidad» al grupo de aparatos que compartian la misma clave. Se '
                'quedo el nombre.'),
            parrafo(
                'Casi siempre hay dos: la de LECTURA —suele venir como «public»— para '
                'preguntarle cosas, y la de ESCRITURA —«private»— para cambiarle la '
                'configuracion. MiRed solo necesita la de lectura: con ella se le puede '
                'preguntar, no tocar nada. No le ponga la de escritura, que seria darle '
                'permiso para cambiarle el switch sin ninguna necesidad.'),
            parrafo(
                'Si nadie la ha cambiado nunca, es «public». Escribalo tal cual y pulse '
                '«Probar ahora»: si contesta, era esa.'),
            parrafo(
                'Ojo: NO es una contrasena seria. En SNMP v1 y v2c viaja en claro por el '
                'cable y cualquiera que escuche la red la ve, asi que nunca reutilice '
                'aqui una que use para otra cosa. Eso es justo lo que arregla el v3, el '
                'de usuario y cifrado, que esta en las opciones tecnicas de abajo.'),
            parrafo(
                'Y si su switch es de los sencillos —de los que se enchufan y ya, sin '
                'pagina de configuracion— no tiene community ninguna: no hay nada que '
                'poner aqui, y MiRed funciona igual sin el mapa de puertos.'),
          ]),
        ),
      ],
    );
  }

  /// _elResultado cuenta como fue la prueba, en una frase y con color.
  ///
  /// Verde si contesto alguien, ambar si no. **Que no conteste nadie no es un
  /// error**: puede ser que los switches de esa red sean sencillos, y eso hay
  /// que decirlo asi en vez de pintar un fallo rojo que asusta.
  Widget _elResultado(BuildContext contexto, PruebaDeCredencial prueba) {
    final tema = Theme.of(contexto);
    final bien = prueba.contestaron > 0;
    final color = bien ? tema.colorScheme.primary : tema.colorScheme.tertiary;

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: tema.colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: color),
      ),
      child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Icon(bien ? Icons.check_circle_outline : Icons.info_outline, color: color),
        const SizedBox(width: 10),
        Expanded(
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text(prueba.explicacion, style: tema.textTheme.bodyMedium),
            for (final aparato in prueba.switches)
              Padding(
                padding: const EdgeInsets.only(top: 4),
                child: Text(
                  '· ${aparato.nombre.isEmpty ? aparato.ip : aparato.nombre}'
                  '${aparato.puertos > 0 ? ' — ${aparato.puertos} puertos' : ''}',
                  style: tema.textTheme.bodySmall,
                ),
              ),
          ]),
        ),
      ]),
    );
  }
}
