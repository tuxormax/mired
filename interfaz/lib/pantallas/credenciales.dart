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
  const PantallaCredenciales({super.key, required this.red});

  /// La red de la que son estas credenciales. **No se comparten**: la comunidad
  /// de un cliente no tiene nada que hacer contra los switches de otro.
  final Red red;

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
    setState(() {
      _credenciales = Api.instancia.listarCredenciales(widget.red.clave);
    });
  }

  Future<void> _crear() async {
    final creada = await showDialog<bool>(
      context: context,
      builder: (_) => _DialogoCredencial(clave: widget.red.clave),
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
      await Api.instancia.borrarCredencial(widget.red.clave, credencial.id);
      _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  @override
  Widget build(BuildContext contexto) => Scaffold(
        appBar: AppBar(
          // De que red son, escrito: con varias instalaciones abiertas es lo
          // unico que evita meterle a un cliente la credencial de otro.
          title: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              const Text('Preguntarle a los switches'),
              Text(
                'Credenciales SNMP · solo de la red ${widget.red.nombre}',
                style: Theme.of(contexto).textTheme.labelSmall,
              ),
            ],
          ),
        ),
        floatingActionButton: FloatingActionButton.extended(
          onPressed: _crear,
          icon: const Icon(Icons.add),
          label: const Text('Agregar la contrasena de un switch'),
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
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                'Un switch «administrable» sabe que aparato tiene enchufado en '
                                'cada uno de sus puertos, y puede decirlo — pero solo si le damos '
                                'su contrasena de lectura. Eso es lo que se guarda aqui.',
                                style: Theme.of(contexto).textTheme.bodyMedium,
                              ),
                              const SizedBox(height: 8),
                              Text(
                                'Es OPCIONAL. Sin esto MiRed sigue encontrando todos los aparatos '
                                'de la red; lo unico que no podra decir es de que puerto cuelga '
                                'cada uno.',
                                style: Theme.of(contexto).textTheme.bodyMedium,
                              ),
                              const SizedBox(height: 8),
                              Text(
                                'Puede poner varias: se prueban en orden contra cada aparato y se '
                                'usa la primera que conteste, asi que no hace falta decir cual va '
                                'con cual switch.',
                                style: Theme.of(contexto).textTheme.bodySmall,
                              ),
                            ],
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
  const _DialogoCredencial({required this.clave});

  final String clave;

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
  bool _probando = false;
  bool _tecnicoAbierto = false;
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
