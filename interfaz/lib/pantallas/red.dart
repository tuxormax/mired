import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../servicios/trayectoria.dart';
import '../widgets/mensajes.dart';
import 'mapa.dart';

/// PantallaRed es lo de un sitio: sus equipos y las subredes que se escanean.
class PantallaRed extends StatefulWidget {
  const PantallaRed({super.key, required this.red});

  final Red red;

  @override
  State<PantallaRed> createState() => _PantallaRedState();
}

class _PantallaRedState extends State<PantallaRed> {
  late Future<List<Equipo>> _equipos;
  late Future<List<Subred>> _subredes;
  late Future<MapaPuertos> _mapa;
  late Red _red;

  final _busqueda = TextEditingController();
  String _filtro = '';
  bool _soloPresentes = false;
  bool _escaneando = false;
  Timer? _vigilante;

  @override
  void initState() {
    super.initState();
    _red = widget.red;
    _recargar();
    _revisarEscaneoEnCurso();
  }

  @override
  void dispose() {
    _vigilante?.cancel();
    _busqueda.dispose();
    super.dispose();
  }

  void _recargar() {
    setState(() {
      _equipos = Api.instancia.listarEquipos(_red.clave, soloPresentes: _soloPresentes);
      _subredes = Api.instancia.listarSubredes(_red.clave);
      _mapa = Api.instancia.mapaDePuertos(_red.clave);
    });
  }

  /// Si al abrir la pantalla ya hay un escaneo corriendo (lo lanzo otra persona,
  /// el programador, o esta pestana se recargo), hay que engancharse a el en vez
  /// de fingir que no pasa nada.
  Future<void> _revisarEscaneoEnCurso() async {
    try {
      final estado = await Api.instancia.listarEscaneos(_red.clave);
      if (estado.enCurso && mounted) {
        setState(() => _escaneando = true);
        _vigilar();
      }
    } catch (_) {
      // Sin escaneos que revisar no pasa nada.
    }
  }

  Future<void> _configurarAgenda() async {
    final nueva = await showDialog<Red>(
      context: context,
      builder: (_) => _DialogoAgenda(red: _red),
    );
    if (nueva != null && mounted) {
      setState(() => _red = nueva);
      mensajeAviso(context,
          nueva.programado ? 'Barridos automaticos encendidos.' : 'Barridos automaticos apagados.');
    }
  }

  Future<void> _escanear({required bool soloPresencia}) async {
    setState(() => _escaneando = true);
    Trayectoria.instancia.anotar('Escanear ${_red.nombre}');

    try {
      await Api.instancia.lanzarEscaneo(_red.clave, soloPresencia: soloPresencia);
      _vigilar();
    } catch (problema, pila) {
      if (mounted) {
        setState(() => _escaneando = false);
        await mostrarProblema(context, problema, pila: pila.toString());
      }
    }
  }

  /// _vigilar pregunta cada tres segundos si el escaneo termino. Se consulta en
  /// vez de esperar la respuesta porque un barrido tarda minutos y la peticion
  /// no puede quedarse abierta todo ese rato.
  void _vigilar() {
    _vigilante?.cancel();
    _vigilante = Timer.periodic(const Duration(seconds: 3), (reloj) async {
      try {
        final estado = await Api.instancia.listarEscaneos(_red.clave);
        if (estado.enCurso || estado.escaneos.isEmpty) return;

        reloj.cancel();
        if (!mounted) return;
        setState(() => _escaneando = false);
        _recargar();

        final ultima = estado.escaneos.first;
        if (!mounted) return;
        if (ultima.estado == 'fallido') {
          mensajeAviso(context, 'El escaneo fallo: ${ultima.detalle ?? "sin detalle"}');
        } else {
          mensajeAviso(context,
              'Escaneo terminado: ${ultima.vistos} equipos, ${ultima.nuevos} nuevos.');
        }
      } catch (_) {
        // Un tropiezo consultando no debe cortar la vigilancia.
      }
    });
  }

  Future<void> _agregarSubred() async {
    final agregada = await showDialog<bool>(
      context: context,
      builder: (_) => _DialogoNuevaSubred(clave: _red.clave),
    );
    if (agregada == true) _recargar();
  }

  Future<void> _renombrar(Equipo equipo) async {
    final control = TextEditingController(text: equipo.alias);
    final nuevo = await showDialog<String>(
      context: context,
      builder: (contextoModal) => AlertDialog(
        title: Text('Nombre de ${equipo.ip}'),
        content: TextField(
          controller: control,
          autofocus: true,
          maxLength: 120, // lo mismo que acepta la columna
          decoration: const InputDecoration(
            labelText: 'Como se le conoce',
            hintText: 'Impresora de contabilidad, NVR de camaras...',
            border: OutlineInputBorder(),
            counterText: '',
          ),
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(contextoModal).pop(),
              child: const Text('Cancelar')),
          FilledButton(
              onPressed: () => Navigator.of(contextoModal).pop(control.text),
              child: const Text('Guardar')),
        ],
      ),
    );
    if (nuevo == null) return;

    try {
      await Api.instancia.ponerAlias(_red.clave, equipo.id, nuevo.trim());
      _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  @override
  Widget build(BuildContext contexto) {
    return DefaultTabController(
      length: 3,
      child: Scaffold(
        appBar: AppBar(
          title: Text(_red.nombre),
          bottom: const TabBar(tabs: [
            Tab(icon: Icon(Icons.devices_other), text: 'Equipos'),
            Tab(icon: Icon(Icons.settings_input_component), text: 'Puertos'),
            Tab(icon: Icon(Icons.route_outlined), text: 'Subredes'),
          ]),
          actions: [
            if (_escaneando)
              const Padding(
                padding: EdgeInsets.symmetric(horizontal: 16, vertical: 18),
                child: SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2)),
              )
            else
              PopupMenuButton<String>(
                tooltip: 'Escanear',
                icon: const Icon(Icons.radar),
                onSelected: (opcion) => _escanear(soloPresencia: opcion == 'presencia'),
                itemBuilder: (_) => const [
                  PopupMenuItem(
                    value: 'completo',
                    child: ListTile(
                      leading: Icon(Icons.travel_explore),
                      title: Text('Escaneo completo'),
                      subtitle: Text('Equipos, nombres y puertos'),
                    ),
                  ),
                  PopupMenuItem(
                    value: 'presencia',
                    child: ListTile(
                      leading: Icon(Icons.bolt),
                      title: Text('Solo presencia'),
                      subtitle: Text('Rapido: quien esta ahora'),
                    ),
                  ),
                ],
              ),
            IconButton(
              tooltip: 'Ver el mapa de la red',
              icon: const Icon(Icons.account_tree_outlined),
              onPressed: () => Navigator.of(contexto).push(
                MaterialPageRoute<void>(builder: (_) => PantallaMapa(red: _red)),
              ),
            ),
            IconButton(
              tooltip: _red.programado
                  ? 'Barridos automaticos encendidos'
                  : 'Programar barridos automaticos',
              icon: Icon(_red.programado ? Icons.schedule : Icons.schedule_outlined),
              color: _red.programado ? Colors.lightGreenAccent : null,
              onPressed: _configurarAgenda,
            ),
            IconButton(
              tooltip: 'Actualizar',
              icon: const Icon(Icons.refresh),
              onPressed: _recargar,
            ),
          ],
        ),
        floatingActionButton: FloatingActionButton.extended(
          onPressed: _agregarSubred,
          icon: const Icon(Icons.add),
          label: const Text('Agregar subred'),
        ),
        body: TabBarView(
          children: [
            _pestanaEquipos(contexto),
            _pestanaPuertos(contexto),
            _pestanaSubredes(contexto),
          ],
        ),
      ),
    );
  }

  Widget _pestanaEquipos(BuildContext contexto) {
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
          child: Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _busqueda,
                  decoration: const InputDecoration(
                    prefixIcon: Icon(Icons.search),
                    hintText: 'Buscar por nombre, IP, MAC, fabricante o puerto',
                    border: OutlineInputBorder(),
                    isDense: true,
                  ),
                  onChanged: (texto) => setState(() => _filtro = texto.toLowerCase()),
                ),
              ),
              const SizedBox(width: 12),
              FilterChip(
                label: const Text('Solo presentes'),
                selected: _soloPresentes,
                onSelected: (valor) {
                  setState(() => _soloPresentes = valor);
                  _recargar();
                },
              ),
            ],
          ),
        ),
        Expanded(
          child: FutureBuilder<List<Equipo>>(
            future: _equipos,
            builder: (_, resultado) {
              if (resultado.connectionState != ConnectionState.done) {
                return const Center(child: CircularProgressIndicator());
              }
              if (resultado.hasError) {
                return Center(
                  child: TextButton(
                    onPressed: () => mostrarProblema(contexto, resultado.error!),
                    child: const Text('No se pudieron cargar los equipos. Ver detalles'),
                  ),
                );
              }

              final equipos = (resultado.data ?? []).where(_coincide).toList();
              if (equipos.isEmpty) {
                return _SinEquipos(escaneando: _escaneando);
              }

              return ListView.separated(
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 88),
                itemCount: equipos.length,
                separatorBuilder: (_, __) => const SizedBox(height: 6),
                itemBuilder: (_, indice) => _TarjetaEquipo(
                  equipo: equipos[indice],
                  clave: _red.clave,
                  alRenombrar: () => _renombrar(equipos[indice]),
                ),
              );
            },
          ),
        ),
      ],
    );
  }

  bool _coincide(Equipo equipo) {
    if (_filtro.isEmpty) return true;
    final donde = [
      equipo.ip,
      equipo.mac,
      equipo.nombre,
      equipo.alias,
      equipo.fabricante,
      equipo.subred,
      ...equipo.puertos.map((puerto) => puerto.etiqueta),
    ].join(' ').toLowerCase();
    return donde.contains(_filtro);
  }

  /// _pestanaPuertos es el mapa de puertos: que hay conectado en cada boca.
  ///
  /// Cuando no hay mapa NO se deja la pantalla vacia: se explica por que no lo
  /// hay y que haria falta para tenerlo. Una pantalla vacia sin explicacion hace
  /// que la gente crea que la herramienta esta rota.
  Widget _pestanaPuertos(BuildContext contexto) {
    return FutureBuilder<MapaPuertos>(
      future: _mapa,
      builder: (_, resultado) {
        if (resultado.connectionState != ConnectionState.done) {
          return const Center(child: CircularProgressIndicator());
        }
        if (resultado.hasError) {
          return Center(
            child: TextButton(
              onPressed: () => mostrarProblema(contexto, resultado.error!),
              child: const Text('No se pudo cargar el mapa de puertos. Ver detalles'),
            ),
          );
        }

        final mapa = resultado.data!;
        final colores = Theme.of(contexto).colorScheme;

        // Los renglones se agrupan por switch, que es como se mira un plano de
        // sitio: primero el aparato, luego sus bocas.
        final porSwitch = <String, List<PuertoDeSwitch>>{};
        for (final renglon in mapa.puertos) {
          porSwitch.putIfAbsent('${renglon.switchNombre}|${renglon.switchIp}', () => []).add(renglon);
        }

        return ListView(
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 88),
          children: [
            Card(
              color: mapa.capacidad == 'exacta'
                  ? colores.primaryContainer
                  : colores.surfaceContainerHighest,
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(mapa.capacidad == 'exacta'
                        ? Icons.check_circle_outline
                        : Icons.info_outline),
                    const SizedBox(width: 12),
                    Expanded(child: Text(mapa.explicacion)),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 16),
            if (!mapa.hayMapa)
              const Padding(
                padding: EdgeInsets.all(24),
                child: Text('Todavia no hay ninguna boca de switch registrada.',
                    textAlign: TextAlign.center),
              ),
            for (final entrada in porSwitch.entries) ...[
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 8),
                child: Row(
                  children: [
                    const Icon(Icons.router_outlined),
                    const SizedBox(width: 8),
                    Text(entrada.key.split('|').first,
                        style: Theme.of(contexto).textTheme.titleMedium),
                    const SizedBox(width: 8),
                    Text(entrada.key.split('|').last,
                        style: TextStyle(fontFamily: 'monospace', color: colores.outline)),
                  ],
                ),
              ),
              Card(
                margin: EdgeInsets.zero,
                child: Column(
                  children: [
                    for (final renglon in entrada.value)
                      ListTile(
                        dense: true,
                        leading: SizedBox(
                          width: 84,
                          child: Text(renglon.puerto,
                              style: const TextStyle(fontFamily: 'monospace'),
                              overflow: TextOverflow.ellipsis),
                        ),
                        title: Text(renglon.quienEs),
                        subtitle: Text([
                          renglon.mac,
                          if (renglon.velocidadMbps > 0) '${renglon.velocidadMbps} Mbps',
                          if (renglon.alias.isNotEmpty) renglon.alias,
                        ].join(' · ')),
                        // Se distingue lo confirmado de lo inferido: con varias
                        // MAC en una boca, atras hay un switch no administrable
                        // y no se puede decir cual esta en que puerto.
                        trailing: renglon.confirmado
                            ? const Tooltip(
                                message: 'Unico equipo en esta boca: puerto exacto',
                                child: Chip(
                                  label: Text('Confirmado'),
                                  visualDensity: VisualDensity.compact,
                                ),
                              )
                            : Tooltip(
                                message: 'Hay ${renglon.cuantosEnBoca} equipos en esta boca: '
                                    'atras cuelga un switch no administrable o un punto de acceso',
                                child: Chip(
                                  avatar: const Icon(Icons.hub, size: 16),
                                  label: Text('Grupo de ${renglon.cuantosEnBoca}'),
                                  visualDensity: VisualDensity.compact,
                                ),
                              ),
                      ),
                  ],
                ),
              ),
              const SizedBox(height: 12),
            ],
          ],
        );
      },
    );
  }

  Widget _pestanaSubredes(BuildContext contexto) {
    return FutureBuilder<List<Subred>>(
      future: _subredes,
      builder: (_, resultado) {
        if (resultado.connectionState != ConnectionState.done) {
          return const Center(child: CircularProgressIndicator());
        }
        if (resultado.hasError) {
          return Center(
            child: TextButton(
              onPressed: () => mostrarProblema(contexto, resultado.error!),
              child: const Text('No se pudieron cargar las subredes. Ver detalles'),
            ),
          );
        }

        final subredes = resultado.data ?? [];
        if (subredes.isEmpty) {
          return const Center(
            child: Padding(
              padding: EdgeInsets.all(32),
              child: Text('Este sitio todavia no tiene subredes que escanear.\n'
                  'Agregue al menos una para poder descubrir equipos.',
                  textAlign: TextAlign.center),
            ),
          );
        }

        return ListView(
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 88),
          children: [
            Card(
              child: Column(
                children: [
                  for (final subred in subredes)
                    ListTile(
                      leading: const Icon(Icons.route_outlined),
                      title: Text(subred.cidr, style: const TextStyle(fontFamily: 'monospace')),
                      subtitle: Text([
                        if (subred.nombre.isNotEmpty) subred.nombre,
                        if (subred.vlan != null) 'VLAN ${subred.vlan}',
                        subred.escanear ? 'Se escanea' : 'No se escanea',
                      ].join(' · ')),
                    ),
                ],
              ),
            ),
            const SizedBox(height: 16),
            Text('Archivo de esta red: ${_red.clave}.db',
                style: Theme.of(contexto).textTheme.labelSmall),
          ],
        );
      },
    );
  }
}

class _TarjetaEquipo extends StatelessWidget {
  const _TarjetaEquipo({
    required this.equipo,
    required this.clave,
    required this.alRenombrar,
  });

  final Equipo equipo;
  final String clave;
  final VoidCallback alRenombrar;

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;

    return Card(
      margin: EdgeInsets.zero,
      child: ExpansionTile(
        leading: Icon(
          equipo.presente ? Icons.circle : Icons.circle_outlined,
          size: 14,
          color: equipo.presente ? Colors.green : colores.outline,
        ),
        title: Row(
          children: [
            Flexible(
              child: Text(equipo.comoSeLlama,
                  style: const TextStyle(fontWeight: FontWeight.w600),
                  overflow: TextOverflow.ellipsis),
            ),
            const SizedBox(width: 10),
            Text(equipo.ip,
                style: TextStyle(
                    fontFamily: 'monospace', fontSize: 12, color: colores.outline)),
          ],
        ),
        subtitle: Text([
          if (equipo.tipo.isNotEmpty) equipo.tipo,
          if (equipo.fabricante.isNotEmpty) equipo.fabricante,
          if (equipo.mac.isNotEmpty) equipo.mac,
          if (equipo.puertos.isNotEmpty) '${equipo.puertos.length} puertos abiertos',
        ].join(' · ')),
        trailing: IconButton(
          tooltip: 'Ponerle nombre',
          icon: const Icon(Icons.edit_outlined),
          onPressed: alRenombrar,
        ),
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(56, 0, 16, 16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (equipo.puertos.isNotEmpty) ...[
                  Wrap(
                    spacing: 6,
                    runSpacing: 6,
                    children: [
                      for (final puerto in equipo.puertos)
                        Tooltip(
                          message: puerto.banner.isEmpty ? puerto.etiqueta : puerto.banner,
                          child: Chip(
                            label: Text(puerto.etiqueta),
                            visualDensity: VisualDensity.compact,
                          ),
                        ),
                    ],
                  ),
                  const SizedBox(height: 12),
                ],
                _Renglon(etiqueta: 'Reconocido como', valor: equipo.tipo),
                _Renglon(etiqueta: 'Nombre descubierto', valor: equipo.nombre),
                _Renglon(etiqueta: 'MAC', valor: equipo.mac),
                _Renglon(etiqueta: 'Fabricante', valor: equipo.fabricante),
                _Renglon(etiqueta: 'Subred', valor: equipo.subred),
                // Se dice como se vio: no es lo mismo "esta" que "algo contesto
                // en esa direccion", y ocultarlo seria mentir sobre la certeza.
                _Renglon(etiqueta: 'Certeza', valor: equipo.certeza),
                _Renglon(etiqueta: 'Visto por primera vez', valor: equipo.primeraVez),
                _Renglon(etiqueta: 'Visto por ultima vez', valor: equipo.ultimaVez),
                const SizedBox(height: 8),
                Wrap(
                  spacing: 8,
                  children: [
                    TextButton.icon(
                      icon: const Icon(Icons.history),
                      label: const Text('Historial de conexiones'),
                      onPressed: () => showDialog<void>(
                        context: contexto,
                        builder: (_) => _DialogoPresencia(clave: clave, equipo: equipo),
                      ),
                    ),
                    // Solo se ofrece cuando nadie lo reconocio: es exactamente
                    // ahi donde el catalogo necesita crecer.
                    if (equipo.tipo.isEmpty)
                      TextButton.icon(
                        icon: const Icon(Icons.add_box_outlined),
                        label: const Text('Proponer definicion'),
                        onPressed: () => showDialog<void>(
                          context: contexto,
                          builder: (_) => _DialogoPropuesta(clave: clave, equipo: equipo),
                        ),
                      ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _Renglon extends StatelessWidget {
  const _Renglon({required this.etiqueta, required this.valor});

  final String etiqueta;
  final String valor;

  @override
  Widget build(BuildContext contexto) {
    if (valor.isEmpty) return const SizedBox.shrink();
    return Padding(
      padding: const EdgeInsets.only(bottom: 2),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 170,
            child: Text(etiqueta, style: Theme.of(contexto).textTheme.labelMedium),
          ),
          Expanded(child: SelectableText(valor)),
        ],
      ),
    );
  }
}

class _SinEquipos extends StatelessWidget {
  const _SinEquipos({required this.escaneando});

  final bool escaneando;

  @override
  Widget build(BuildContext contexto) => Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(escaneando ? Icons.radar : Icons.devices_other, size: 48),
              const SizedBox(height: 12),
              Text(
                escaneando
                    ? 'Escaneando la red. Puede tardar unos minutos.'
                    : 'Todavia no se ha descubierto ningun equipo.',
                textAlign: TextAlign.center,
                style: Theme.of(contexto).textTheme.titleMedium,
              ),
              if (!escaneando) ...[
                const SizedBox(height: 6),
                const Text('Use el boton del radar, arriba, para escanear el sitio.',
                    textAlign: TextAlign.center),
              ],
            ],
          ),
        ),
      );
}

class _DialogoNuevaSubred extends StatefulWidget {
  const _DialogoNuevaSubred({required this.clave});

  final String clave;

  @override
  State<_DialogoNuevaSubred> createState() => _DialogoNuevaSubredState();
}

class _DialogoNuevaSubredState extends State<_DialogoNuevaSubred> {
  final _formulario = GlobalKey<FormState>();
  final _cidr = TextEditingController();
  final _nombre = TextEditingController();
  final _vlan = TextEditingController();
  bool _ocupado = false;

  @override
  void dispose() {
    _cidr.dispose();
    _nombre.dispose();
    _vlan.dispose();
    super.dispose();
  }

  Future<void> _guardar() async {
    if (!_formulario.currentState!.validate()) return;
    setState(() => _ocupado = true);
    Trayectoria.instancia.anotar('Agregar subred ${_cidr.text}');

    try {
      await Api.instancia.crearSubred(
        widget.clave,
        _cidr.text.trim(),
        _nombre.text.trim(),
        _vlan.text.trim().isEmpty ? null : int.parse(_vlan.text.trim()),
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
        title: const Text('Agregar subred'),
        content: SizedBox(
          width: 420,
          child: Form(
            key: _formulario,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextFormField(
                  controller: _cidr,
                  autofocus: true,
                  decoration: const InputDecoration(
                    labelText: 'Rango',
                    hintText: '192.168.1.0/24',
                    border: OutlineInputBorder(),
                  ),
                  validator: (valor) {
                    if (valor == null || valor.trim().isEmpty) return 'Capture el rango';
                    // Lo mismo que valida el servidor: si aqui pasa, alla pasa.
                    if (!RegExp(r'^[0-9a-fA-F:.]+/\d{1,3}$').hasMatch(valor.trim())) {
                      return 'Se espera algo como 192.168.1.0/24';
                    }
                    return null;
                  },
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _nombre,
                  maxLength: 80,
                  decoration: const InputDecoration(
                    labelText: 'Nombre (opcional)',
                    hintText: 'Camaras, Oficinas, WiFi...',
                    border: OutlineInputBorder(),
                    counterText: '',
                  ),
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _vlan,
                  keyboardType: TextInputType.number,
                  decoration: const InputDecoration(
                    labelText: 'VLAN (opcional)',
                    border: OutlineInputBorder(),
                  ),
                  validator: (valor) {
                    if (valor == null || valor.trim().isEmpty) return null;
                    final numero = int.tryParse(valor.trim());
                    // El rango real de una VLAN, que es lo que acepta la columna.
                    if (numero == null || numero < 1 || numero > 4094) {
                      return 'Debe estar entre 1 y 4094';
                    }
                    return null;
                  },
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
                : const Text('Agregar'),
          ),
        ],
      );
}

/// _DialogoPresencia muestra cuando aparecio y cuando se fue un equipo.
///
/// Solo se guardan los CAMBIOS de estado, asi que esta lista es corta y legible
/// aunque el barrido corra cada minuto.
class _DialogoPresencia extends StatefulWidget {
  const _DialogoPresencia({required this.clave, required this.equipo});

  final String clave;
  final Equipo equipo;

  @override
  State<_DialogoPresencia> createState() => _DialogoPresenciaState();
}

class _DialogoPresenciaState extends State<_DialogoPresencia> {
  late Future<List<EventoPresencia>> _eventos;

  @override
  void initState() {
    super.initState();
    _eventos = Api.instancia.listarPresencia(widget.clave, widget.equipo.id);
  }

  @override
  Widget build(BuildContext contexto) => AlertDialog(
        title: Text('Conexiones de ${widget.equipo.comoSeLlama}'),
        content: SizedBox(
          width: 460,
          height: 420,
          child: FutureBuilder<List<EventoPresencia>>(
            future: _eventos,
            builder: (_, resultado) {
              if (resultado.connectionState != ConnectionState.done) {
                return const Center(child: CircularProgressIndicator());
              }
              if (resultado.hasError) {
                return Center(
                  child: TextButton(
                    onPressed: () => mostrarProblema(contexto, resultado.error!),
                    child: const Text('No se pudo leer el historial. Ver detalles'),
                  ),
                );
              }

              final eventos = resultado.data ?? [];
              if (eventos.isEmpty) {
                return const Center(
                  child: Text('Todavia no hay historial para este equipo.',
                      textAlign: TextAlign.center),
                );
              }

              return ListView.separated(
                itemCount: eventos.length,
                separatorBuilder: (_, __) => const Divider(height: 1),
                itemBuilder: (_, indice) {
                  final evento = eventos[indice];
                  return ListTile(
                    dense: true,
                    leading: Icon(
                      evento.presente ? Icons.login : Icons.logout,
                      color: evento.presente ? Colors.green : Colors.grey,
                    ),
                    title: Text(evento.presente ? 'Se conecto' : 'Dejo de responder'),
                    subtitle: Text(evento.momento),
                    trailing: evento.ip.isEmpty
                        ? null
                        : Text(evento.ip, style: const TextStyle(fontFamily: 'monospace')),
                  );
                },
              );
            },
          ),
        ),
        actions: [
          FilledButton(
            onPressed: () => Navigator.of(contexto).pop(),
            child: const Text('Cerrar'),
          ),
        ],
      );
}

/// _DialogoAgenda enciende los barridos automaticos y fija sus dos ritmos.
class _DialogoAgenda extends StatefulWidget {
  const _DialogoAgenda({required this.red});

  final Red red;

  @override
  State<_DialogoAgenda> createState() => _DialogoAgendaState();
}

class _DialogoAgendaState extends State<_DialogoAgenda> {
  final _formulario = GlobalKey<FormState>();
  late bool _programado;
  late TextEditingController _presencia;
  late TextEditingController _profundo;
  bool _ocupado = false;

  @override
  void initState() {
    super.initState();
    _programado = widget.red.programado;
    _presencia = TextEditingController(text: '${widget.red.presenciaCadaSegundos}');
    _profundo = TextEditingController(text: '${widget.red.profundoCadaMinutos}');
  }

  @override
  void dispose() {
    _presencia.dispose();
    _profundo.dispose();
    super.dispose();
  }

  Future<void> _guardar() async {
    if (!_formulario.currentState!.validate()) return;
    setState(() => _ocupado = true);
    Trayectoria.instancia.anotar('Configurar agenda de ${widget.red.nombre}');

    try {
      final nueva = await Api.instancia.configurarAgenda(
        widget.red.clave,
        programado: _programado,
        presenciaCadaSegundos: int.parse(_presencia.text.trim()),
        profundoCadaMinutos: int.parse(_profundo.text.trim()),
      );
      if (mounted) Navigator.of(context).pop(nueva);
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _ocupado = false);
    }
  }

  @override
  Widget build(BuildContext contexto) => AlertDialog(
        title: const Text('Barridos automaticos'),
        content: SizedBox(
          width: 460,
          child: Form(
            key: _formulario,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'Son dos ritmos distintos sobre la misma red. El de presencia es corto y '
                  'frecuente: solo dice quien esta. El profundo revisa puertos, nombres y '
                  'servicios, cuesta mucho mas y por eso va espaciado.',
                ),
                const SizedBox(height: 16),
                SwitchListTile(
                  value: _programado,
                  onChanged: (valor) => setState(() => _programado = valor),
                  title: const Text('Escanear esta red automaticamente'),
                  contentPadding: EdgeInsets.zero,
                ),
                const SizedBox(height: 8),
                TextFormField(
                  controller: _presencia,
                  enabled: _programado,
                  keyboardType: TextInputType.number,
                  decoration: const InputDecoration(
                    labelText: 'Presencia cada (segundos)',
                    helperText: 'De 15 a 3600. Con 60 basta para casi todo.',
                    border: OutlineInputBorder(),
                  ),
                  // Los mismos limites que impone el servidor.
                  validator: (valor) {
                    final numero = int.tryParse((valor ?? '').trim());
                    if (numero == null || numero < 15 || numero > 3600) {
                      return 'Debe estar entre 15 y 3600 segundos';
                    }
                    return null;
                  },
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _profundo,
                  enabled: _programado,
                  keyboardType: TextInputType.number,
                  decoration: const InputDecoration(
                    labelText: 'Escaneo profundo cada (minutos)',
                    helperText: 'De 5 a 10080. Con 360 (seis horas) suele sobrar.',
                    border: OutlineInputBorder(),
                  ),
                  validator: (valor) {
                    final numero = int.tryParse((valor ?? '').trim());
                    if (numero == null || numero < 5 || numero > 10080) {
                      return 'Debe estar entre 5 y 10080 minutos';
                    }
                    return null;
                  },
                ),
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
            onPressed: _ocupado ? null : () => Navigator.of(contexto).pop(),
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

/// _DialogoPropuesta genera el `.toml` de un equipo sin identificar.
///
/// Es la puerta de entrada del catalogo comunitario: quien tenga un aparato que
/// MiRed no reconoce le pone nombre, copia el archivo y lo manda. No hace falta
/// saber Go ni entender el formato: ya viene relleno con lo que se vio.
class _DialogoPropuesta extends StatefulWidget {
  const _DialogoPropuesta({required this.clave, required this.equipo});

  final String clave;
  final Equipo equipo;

  @override
  State<_DialogoPropuesta> createState() => _DialogoPropuestaState();
}

class _DialogoPropuestaState extends State<_DialogoPropuesta> {
  late TextEditingController _nombre;
  Map<String, dynamic>? _propuesta;
  bool _ocupado = false;
  bool _copiado = false;

  @override
  void initState() {
    super.initState();
    _nombre = TextEditingController(
        text: widget.equipo.fabricante.isNotEmpty
            ? '${widget.equipo.fabricante} sin identificar'
            : '');
    _generar();
  }

  @override
  void dispose() {
    _nombre.dispose();
    super.dispose();
  }

  Future<void> _generar() async {
    setState(() => _ocupado = true);
    try {
      final propuesta = await Api.instancia
          .proponerDefinicion(widget.clave, widget.equipo.id, _nombre.text.trim());
      if (mounted) setState(() => _propuesta = propuesta);
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _ocupado = false);
    }
  }

  @override
  Widget build(BuildContext contexto) => AlertDialog(
        title: const Text('Proponer definicion'),
        content: SizedBox(
          width: 620,
          child: SingleChildScrollView(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                const Text(
                  'MiRed no supo que es este aparato. Pongale nombre, revise el archivo y '
                  'mandelo al repositorio para que lo reconozca todo el mundo. Tambien puede '
                  'guardarlo en /etc/mired/dispositivos/ para usarlo solo aqui.',
                ),
                const SizedBox(height: 16),
                Row(
                  children: [
                    Expanded(
                      child: TextField(
                        controller: _nombre,
                        maxLength: 80,
                        decoration: const InputDecoration(
                          labelText: 'Como se llama este tipo de aparato',
                          hintText: 'Camara Acme, Termostato Zeta...',
                          border: OutlineInputBorder(),
                          counterText: '',
                        ),
                        onSubmitted: (_) => _generar(),
                      ),
                    ),
                    const SizedBox(width: 8),
                    FilledButton(
                      onPressed: _ocupado ? null : _generar,
                      child: const Text('Generar'),
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                if (_ocupado)
                  const Center(child: Padding(
                    padding: EdgeInsets.all(24),
                    child: CircularProgressIndicator(),
                  ))
                else if (_propuesta != null) ...[
                  Text('Archivo sugerido: ${_propuesta!['archivo']}',
                      style: Theme.of(contexto).textTheme.labelMedium),
                  const SizedBox(height: 8),
                  Container(
                    width: double.infinity,
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: Theme.of(contexto).colorScheme.surfaceContainerHighest,
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: SelectableText(
                      _propuesta!['contenido'] as String,
                      style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
                    ),
                  ),
                ],
              ],
            ),
          ),
        ),
        actions: [
          if (_propuesta != null)
            FilledButton.icon(
              icon: Icon(_copiado ? Icons.check : Icons.copy),
              label: Text(_copiado ? 'Copiado' : 'Copiar archivo'),
              onPressed: () async {
                await Clipboard.setData(
                    ClipboardData(text: _propuesta!['contenido'] as String));
                if (contexto.mounted) setState(() => _copiado = true);
              },
            ),
          TextButton(
            onPressed: () => Navigator.of(contexto).pop(),
            child: const Text('Cerrar'),
          ),
        ],
      );
}
