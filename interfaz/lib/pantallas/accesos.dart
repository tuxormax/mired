import 'package:flutter/material.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../widgets/mensajes.dart';
import 'controladoras.dart';
import 'credenciales.dart';
import 'topologia_manual.dart';

/// PantallaAccesos es **la** lista de llaves de una red.
///
/// Antes eran tres sitios distintos para lo mismo: las credenciales SNMP en su
/// pantalla, las controladoras WiFi en otra, y la clave del panel de cada
/// aparato escondida dentro de su ficha. Tres cosas que entender por separado
/// cuando en la cabeza de quien las usa son una sola: **las contrasenas de los
/// aparatos de esta red**.
///
/// Aqui van juntas y cada renglon dice DE QUE APARATO es. Se sigue llegando a la
/// de un aparato desde su ficha, que es donde tambien se busca; es la misma
/// lista vista por otra puerta.
///
/// La unica que no es de un aparato concreto es la de SNMP, y a proposito: se
/// prueba contra todos porque, cuando se captura, todavia no se sabe cuales son
/// switches administrables. Sale como «Todos», que ademas explica solo por que
/// esa fila es distinta.
class PantallaAccesos extends StatefulWidget {
  const PantallaAccesos({super.key, required this.red});

  final Red red;

  @override
  State<PantallaAccesos> createState() => _PantallaAccesosState();
}

/// _Acceso es un renglon de la lista, venga de donde venga.
///
/// Las tres cosas se guardan en sitios distintos —y con formas distintas—, pero
/// para quien mira la lista son lo mismo: un aparato, una forma de entrar y un
/// usuario. Esta clase es esa vista comun.
class _Acceso {
  const _Acceso({
    required this.deQuien,
    required this.como,
    required this.detalle,
    required this.icono,
    required this.borrar,
    this.esDeTodos = false,
  });

  /// De que aparato es. «Todos» cuando no es de ninguno en concreto.
  final String deQuien;

  /// Como se entra: panel web, SSH, SNMP, la controladora...
  final String como;

  /// El usuario, la direccion del panel: lo que ayude a reconocerla.
  final String detalle;

  final IconData icono;
  final Future<void> Function() borrar;
  final bool esDeTodos;
}

class _PantallaAccesosState extends State<PantallaAccesos> {
  late Future<List<_Acceso>> _accesos;
  List<Equipo> _equipos = const [];

  @override
  void initState() {
    super.initState();
    _recargar();
  }

  void _recargar() {
    // Con cuerpo de bloque a proposito: `setState(() => _x = algo())` DEVUELVE
    // lo asignado, y si eso es un Future, Flutter lo rechaza creyendo que el
    // callback es asincrono.
    setState(() {
      _accesos = _juntarlo();
    });
  }

  /// _juntarlo trae las tres fuentes y las pone en una sola lista.
  Future<List<_Acceso>> _juntarlo() async {
    final clave = widget.red.clave;
    final equipos = await Api.instancia.listarEquipos(clave);
    final snmp = await Api.instancia.listarCredenciales(clave);
    final controladoras = await Api.instancia.listarControladoras(clave);
    _equipos = equipos;

    final lista = <_Acceso>[
      // Primero las de «todos»: son las que hacen que el mapa de puertos exista.
      for (final credencial in snmp)
        _Acceso(
          deQuien: 'Todos los aparatos',
          como: 'SNMP ${credencial.version} · contrasena de lectura',
          detalle: credencial.usuario.isEmpty
              ? 'Para encontrar los switches y sus puertos'
              : 'usuario ${credencial.usuario}',
          icono: Icons.lan_outlined,
          esDeTodos: true,
          borrar: () => Api.instancia.borrarCredencial(clave, credencial.id),
        ),
      for (final controladora in controladoras)
        _Acceso(
          deQuien: controladora.nombre,
          como: 'Controladora WiFi (${controladora.tipo})',
          detalle: [
            controladora.url,
            if (controladora.usuario.isNotEmpty) 'usuario ${controladora.usuario}',
          ].join(' · '),
          icono: Icons.wifi_tethering,
          borrar: () => Api.instancia.borrarControladora(clave, controladora.id),
        ),
      for (final equipo in equipos)
        for (final credencial in equipo.credenciales)
          _Acceso(
            deQuien: equipo.comoSeLlama,
            como: credencial.comoSeLlamaElTipo,
            detalle: [
              if (credencial.usuario.isNotEmpty) 'usuario ${credencial.usuario}',
              if (credencial.direccion.isNotEmpty) credencial.direccion,
              if (!credencial.tieneClave) 'sin clave guardada',
            ].join(' · '),
            icono: Icons.vpn_key_outlined,
            borrar: () => Api.instancia.borrarCredencialDeEquipo(clave, credencial.id),
          ),
    ];
    return lista;
  }

  Future<void> _nuevo() async {
    final que = await showDialog<_QueAcceso>(
      context: context,
      builder: (_) => _DialogoQueAcceso(equipos: _equipos),
    );
    if (que == null || !mounted) return;

    final guardado = await showDialog<bool>(
      context: context,
      builder: (_) => switch (que.como) {
        'snmp' => DialogoCredencialSNMP(clave: widget.red.clave),
        'controladora' => DialogoControladora(clave: widget.red.clave),
        _ => DialogoCredencial(
            clave: widget.red.clave, equipo: que.equipo!, tipoInicial: que.como),
      },
    );
    if (guardado == true) _recargar();
  }

  Future<void> _borrar(_Acceso acceso) async {
    final confirma = await showDialog<bool>(
      context: context,
      builder: (contextoModal) => AlertDialog(
        title: const Text('Quitar este acceso'),
        content: Text('Se borra la contrasena de «${acceso.deQuien}». '
            'El aparato no se toca: solo MiRed deja de poder entrar.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(contextoModal).pop(false),
              child: const Text('Cancelar')),
          FilledButton(
              onPressed: () => Navigator.of(contextoModal).pop(true),
              child: const Text('Quitar')),
        ],
      ),
    );
    if (confirma != true) return;

    try {
      await acceso.borrar();
      _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  @override
  Widget build(BuildContext contexto) => Scaffold(
        appBar: AppBar(
          title: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              const Text('Accesos y contrasenas'),
              Text(
                'Solo de la red ${widget.red.nombre}',
                style: Theme.of(contexto).textTheme.labelSmall,
              ),
            ],
          ),
        ),
        floatingActionButton: FloatingActionButton.extended(
          onPressed: _nuevo,
          icon: const Icon(Icons.add),
          label: const Text('Nuevo acceso'),
        ),
        body: FutureBuilder<List<_Acceso>>(
          future: _accesos,
          builder: (_, resultado) {
            if (resultado.connectionState != ConnectionState.done) {
              return const Center(child: CircularProgressIndicator());
            }
            if (resultado.hasError) {
              return Center(
                child: TextButton(
                  onPressed: () => mostrarProblema(contexto, resultado.error!),
                  child: const Text('No se pudieron cargar los accesos. Ver detalles'),
                ),
              );
            }

            final accesos = resultado.data ?? [];
            return ListView(
              padding: const EdgeInsets.fromLTRB(16, 16, 16, 88),
              children: [
                _laExplicacion(contexto),
                const SizedBox(height: 16),
                if (accesos.isEmpty)
                  const Padding(
                    padding: EdgeInsets.all(24),
                    child: Text(
                      'Todavia no hay ninguno, y puede que no haga falta ninguno.\n\n'
                      'MiRed encuentra los aparatos de la red sin necesidad de contrasenas. '
                      'Estas sirven para lo que hay que PREGUNTAR: en que puerto esta cada '
                      'cosa, quien cuelga de cada antena, o para tener a mano como se entra '
                      'a un aparato.',
                      textAlign: TextAlign.center,
                    ),
                  )
                else
                  Card(
                    margin: EdgeInsets.zero,
                    child: Column(
                      children: [
                        for (final acceso in accesos) _renglon(contexto, acceso),
                      ],
                    ),
                  ),
              ],
            );
          },
        ),
      );

  Widget _laExplicacion(BuildContext contexto) => Card(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
            const Icon(Icons.info_outline),
            const SizedBox(width: 12),
            Expanded(
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text(
                  'Las contrasenas con las que MiRed le pregunta cosas a los aparatos de '
                  'esta red. Cada una dice de que aparato es.',
                  style: Theme.of(contexto).textTheme.bodyMedium,
                ),
                const SizedBox(height: 8),
                Text(
                  'Todo esto es OPCIONAL: sin ninguna, MiRed sigue encontrando los '
                  'aparatos y avisando de lo que cambia. Lo que se gana con ellas es '
                  'saber en que puerto esta cada cosa, quien cuelga de cada antena WiFi, '
                  'y tener a mano como se entra a administrar cada aparato.',
                  style: Theme.of(contexto).textTheme.bodySmall,
                ),
                const SizedBox(height: 8),
                Text(
                  'Las claves se guardan CIFRADAS y no vuelven a salir de aqui, salvo '
                  'que alguien las pida expresamente — y eso queda anotado.',
                  style: Theme.of(contexto).textTheme.bodySmall,
                ),
              ]),
            ),
          ]),
        ),
      );

  Widget _renglon(BuildContext contexto, _Acceso acceso) {
    final tema = Theme.of(contexto);
    return ListTile(
      leading: Icon(acceso.icono,
          color: acceso.esDeTodos ? tema.colorScheme.primary : null),
      title: Row(children: [
        Flexible(child: Text(acceso.deQuien, overflow: TextOverflow.ellipsis)),
        // La de «todos» se marca: es la unica que no es de un aparato, y sin
        // decirlo se lee como si faltara el nombre.
        if (acceso.esDeTodos) ...[
          const SizedBox(width: 8),
          Chip(
            label: const Text('el que conteste'),
            labelStyle: tema.textTheme.labelSmall,
            visualDensity: VisualDensity.compact,
            padding: EdgeInsets.zero,
          ),
        ],
      ]),
      subtitle: Text([acceso.como, acceso.detalle]
          .where((parte) => parte.isNotEmpty)
          .join('  ·  ')),
      trailing: IconButton(
        tooltip: 'Quitar',
        icon: const Icon(Icons.delete_outline),
        onPressed: () => _borrar(acceso),
      ),
    );
  }
}

/// _QueAcceso es lo que se eligio en el primer paso: de que aparato y como.
class _QueAcceso {
  const _QueAcceso({required this.como, this.equipo});
  final String como;
  final Equipo? equipo;
}

/// _DialogoQueAcceso pregunta primero lo que una persona piensa primero.
///
/// «¿De que aparato?» y «¿como se entra?», en ese orden. Antes habia que saber
/// de antemano en cual de las tres pantallas estaba lo que se queria guardar, y
/// eso solo lo sabe quien ya conoce el programa por dentro.
class _DialogoQueAcceso extends StatefulWidget {
  const _DialogoQueAcceso({required this.equipos});

  final List<Equipo> equipos;

  @override
  State<_DialogoQueAcceso> createState() => _DialogoQueAccesoState();
}

class _DialogoQueAccesoState extends State<_DialogoQueAcceso> {
  /// -1 es «todos los aparatos», que es como se guarda la de SNMP.
  int _equipoId = -1;
  String _como = 'snmp';

  Equipo? get _equipo {
    for (final equipo in widget.equipos) {
      if (equipo.id == _equipoId) return equipo;
    }
    return null;
  }

  bool get _esTodos => _equipoId == -1;

  @override
  Widget build(BuildContext contexto) {
    final tema = Theme.of(contexto);
    // Con «todos» solo cabe SNMP: es la unica que se prueba a ciegas contra la
    // red entera. Con un aparato concreto, todo lo demas.
    final formas = _esTodos
        ? const [('snmp', 'Contrasena de lectura de los switches (SNMP)')]
        : const [
            ('web', 'Su pagina de administracion'),
            ('ssh', 'Consola SSH'),
            ('consola', 'Consola'),
            ('app', 'Aplicacion del fabricante'),
            ('controladora', 'Es la controladora del WiFi'),
          ];
    if (!formas.any((forma) => forma.$1 == _como)) _como = formas.first.$1;

    return AlertDialog(
      title: const Text('Nuevo acceso'),
      content: SizedBox(
        width: 520,
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          DropdownButtonFormField<int>(
            initialValue: _equipoId,
            isExpanded: true,
            decoration: const InputDecoration(
              labelText: '¿De que aparato es?',
              border: OutlineInputBorder(),
            ),
            items: [
              const DropdownMenuItem(
                value: -1,
                child: Text('Todos — para encontrar los switches y sus puertos'),
              ),
              for (final equipo in widget.equipos)
                DropdownMenuItem(
                  value: equipo.id,
                  child: Text(equipo.comoSeLlama, overflow: TextOverflow.ellipsis),
                ),
            ],
            onChanged: (valor) => setState(() => _equipoId = valor ?? -1),
          ),
          const SizedBox(height: 8),
          Align(
            alignment: Alignment.centerLeft,
            child: Text(
              _esTodos
                  ? 'La contrasena de lectura se prueba contra todos los aparatos y se '
                      'usa la que conteste: cuando se captura todavia no se sabe cuales '
                      'son switches administrables.'
                  : 'Se guarda como la llave de ese aparato, y aparece tambien en su ficha.',
              style: tema.textTheme.bodySmall,
            ),
          ),
          const SizedBox(height: 16),
          DropdownButtonFormField<String>(
            initialValue: _como,
            isExpanded: true,
            decoration: const InputDecoration(
              labelText: '¿Como se entra?',
              border: OutlineInputBorder(),
            ),
            items: [
              for (final forma in formas)
                DropdownMenuItem(value: forma.$1, child: Text(forma.$2)),
            ],
            onChanged: (valor) => setState(() => _como = valor ?? 'web'),
          ),
        ]),
      ),
      actions: [
        TextButton(
            onPressed: () => Navigator.of(contexto).pop(),
            child: const Text('Cancelar')),
        FilledButton(
          onPressed: () => Navigator.of(contexto)
              .pop(_QueAcceso(como: _como, equipo: _equipo)),
          child: const Text('Siguiente'),
        ),
      ],
    );
  }
}
