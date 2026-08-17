import 'package:flutter/material.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../servicios/trayectoria.dart';
import '../widgets/mensajes.dart';

/// PantallaControladoras administra las controladoras WiFi.
///
/// Un punto de acceso no tiene puertos: tiene antenas, y quien sabe que aparato
/// esta colgado de cual es la controladora. Sin dar de alta una, en una oficina
/// moderna la mitad de los equipos —telefonos, portatiles, camaras— salen como
/// "sin ubicar" en el mapa.
class PantallaControladoras extends StatefulWidget {
  const PantallaControladoras({super.key, required this.red});

  /// La red de la que es esta controladora. **No se comparte**: la de un cliente
  /// no atiende la red de otro.
  final Red red;

  @override
  State<PantallaControladoras> createState() => _PantallaControladorasState();
}

class _PantallaControladorasState extends State<PantallaControladoras> {
  late Future<List<Controladora>> _controladoras;

  @override
  void initState() {
    super.initState();
    _recargar();
  }

  void _recargar() {
    setState(() {
      _controladoras = Api.instancia.listarControladoras(widget.red.clave);
    });
  }

  Future<void> _crear() async {
    final creada = await showDialog<bool>(
      context: context,
      builder: (_) => _DialogoControladora(clave: widget.red.clave),
    );
    if (creada == true) _recargar();
  }

  Future<void> _borrar(Controladora controladora) async {
    final confirma = await showDialog<bool>(
      context: context,
      builder: (contextoModal) => AlertDialog(
        title: const Text('Borrar controladora'),
        content: Text('Se dejara de preguntarle a "${controladora.nombre}". '
            'Los equipos que solo se ubicaban por WiFi pasaran a "sin ubicar" en el mapa.'),
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
      await Api.instancia.borrarControladora(widget.red.clave, controladora.id);
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
              const Text('Quien manda el WiFi'),
              Text(
                'Controladoras WiFi · solo de la red ${widget.red.nombre}',
                style: Theme.of(contexto).textTheme.labelSmall,
              ),
            ],
          ),
        ),
        floatingActionButton: FloatingActionButton.extended(
          onPressed: _crear,
          icon: const Icon(Icons.add),
          label: const Text('Nueva controladora'),
        ),
        body: FutureBuilder<List<Controladora>>(
          future: _controladoras,
          builder: (_, resultado) {
            if (resultado.connectionState != ConnectionState.done) {
              return const Center(child: CircularProgressIndicator());
            }
            if (resultado.hasError) {
              return Center(
                child: TextButton(
                  onPressed: () => mostrarProblema(contexto, resultado.error!),
                  child: const Text('No se pudieron cargar las controladoras. Ver detalles'),
                ),
              );
            }

            final controladoras = resultado.data ?? [];
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
                            'Lo que va por cable se sabe mirando los puertos del switch. Lo que '
                            'va por WiFi no: una antena no tiene puertos, y quien sabe que '
                            'telefono o que laptop esta colgado de cual es la controladora que '
                            'las manda —una UniFi, un Cloud Key—.\n\n'
                            'Es OPCIONAL, y solo hace falta si tiene una. Dandola de alta, el '
                            'WiFi sale en el mapa igual que lo cableado: cada aparato bajo su '
                            'antena y con el nombre de la red a la que se conecto. Sin ella, esos '
                            'aparatos salen igual, pero como «sin ubicar».',
                            style: Theme.of(contexto).textTheme.bodyMedium,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
                const SizedBox(height: 16),
                if (controladoras.isEmpty)
                  const Padding(
                    padding: EdgeInsets.all(24),
                    child: Text(
                      'Todavia no hay ninguna controladora.\n'
                      'Sin ella, los equipos por WiFi salen como "sin ubicar".',
                      textAlign: TextAlign.center,
                    ),
                  )
                else
                  Card(
                    margin: EdgeInsets.zero,
                    child: Column(
                      children: [
                        for (final controladora in controladoras)
                          _Renglon(
                            controladora: controladora,
                            alBorrar: () => _borrar(controladora),
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

/// _Renglon muestra una controladora y, sobre todo, si esta contestando.
///
/// Que una controladora lleve dias sin contestar es un dato, no un silencio: sin
/// decirlo aqui, el WiFi se iria del mapa poco a poco y nadie sabria por que.
class _Renglon extends StatelessWidget {
  const _Renglon({required this.controladora, required this.alBorrar});

  final Controladora controladora;
  final VoidCallback alBorrar;

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;
    final fallando = controladora.ultimoError.isNotEmpty;

    return ListTile(
      leading: Icon(
        fallando ? Icons.wifi_off : Icons.wifi_tethering,
        color: fallando ? colores.error : null,
      ),
      title: Text(controladora.nombre),
      subtitle: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text([
            controladora.url,
            'sitio ${controladora.sitio}',
            'usuario ${controladora.usuario}',
          ].join(' · ')),
          if (fallando)
            Text('No contesto: ${controladora.ultimoError}',
                style: TextStyle(color: colores.error))
          else if (controladora.ultimoExito.isNotEmpty)
            Text('Contesto por ultima vez el ${controladora.ultimoExito}')
          else
            const Text('Todavia no se le ha preguntado. Corra un escaneo completo.'),
        ],
      ),
      isThreeLine: true,
      trailing: IconButton(
        tooltip: 'Borrar',
        icon: const Icon(Icons.delete_outline),
        onPressed: alBorrar,
      ),
    );
  }
}

class _DialogoControladora extends StatefulWidget {
  const _DialogoControladora({required this.clave});

  final String clave;

  @override
  State<_DialogoControladora> createState() => _DialogoControladoraState();
}

class _DialogoControladoraState extends State<_DialogoControladora> {
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
