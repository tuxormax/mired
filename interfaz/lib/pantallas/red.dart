import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../modelos/categorias.dart';
import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../servicios/navegador.dart';
import '../servicios/frescura.dart';
import '../servicios/trayectoria.dart';
import '../widgets/mensajes.dart';
import 'alertas.dart';
import 'equipo.dart';
import 'mapa.dart';
import 'propiedades_equipo.dart';
import 'topologia_manual.dart';

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
  late Future<Consumo> _consumo;
  late Future<List<ConsumoPorAplicacion>> _aplicaciones;
  late Future<ComposicionDeRed> _composicion;
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
      _consumo = Api.instancia.consumo(_red.clave);
      _aplicaciones = Api.instancia.consumoPorAplicacion(_red.clave);
      _composicion = Api.instancia.composicion(_red.clave);
    });
    _releerRed();
  }

  /// _releerRed vuelve a pedir la ficha del sitio.
  ///
  /// Sin esto, `_red` se quedaba con la foto del momento en que se abrio la
  /// pantalla, y **la campanita seguia mostrando el numero viejo** aunque las
  /// alertas ya estuvieran despachadas: habia que salir al panel de inicio y
  /// volver a entrar para verlo bien. Lo mismo valia para cuando fue el ultimo
  /// escaneo, que se lee en el titulo.
  ///
  /// Va aparte del setState de arriba porque es una peticion mas, y que falle no
  /// debe tumbar el resto de la pantalla: si no se puede releer, se conserva lo
  /// que ya se tenia.
  Future<void> _releerRed() async {
    try {
      final fresca = await Api.instancia.verRed(_red.clave);
      if (mounted) setState(() => _red = fresca);
    } catch (_) {
      // Se conserva lo que ya se sabia de la red.
    }
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

  /// _agregarAMano da de alta un aparato que ningun barrido va a encontrar: el
  /// switch no administrable, el modem que no habla SNMP hacia la LAN.
  Future<void> _agregarAMano() async {
    final creado = await showDialog<Equipo>(
      context: context,
      builder: (_) => DialogoEquipoManual(clave: _red.clave),
    );
    if (creado != null) _recargar();
  }

  Future<void> _editarFicha(Equipo equipo) async {
    final cambio = await showDialog<bool>(
      context: context,
      builder: (_) => DialogoFicha(clave: _red.clave, equipo: equipo),
    );
    if (cambio == true) _recargar();
  }

  /// _verFicha abre la ficha completa del aparato: sus propiedades, lo que
  /// tiene conectado y —si se puede administrar— sus credenciales.
  Future<void> _verFicha(Equipo equipo) async {
    final topologia = await Api.instancia.topologiaManual(_red.clave);
    final equipos = await Api.instancia.listarEquipos(_red.clave);
    if (!mounted) return;

    final cambio = await showDialog<bool>(
      context: context,
      builder: (_) => DialogoEquipo(
        clave: _red.clave,
        equipo: equipo,
        topologia: topologia,
        equipos: equipos,
      ),
    );
    if (cambio == true) _recargar();
  }

  Future<void> _editarPuertos(Equipo equipo) async {
    final cambio = await showDialog<bool>(
      context: context,
      builder: (_) => DialogoPuertos(clave: _red.clave, equipo: equipo),
    );
    if (cambio == true) _recargar();
  }

  /// _borrarManual solo sirve para lo declarado a mano. Un equipo descubierto se
  /// marca ausente cuando desaparece, pero no se borra: su historia es lo que
  /// despues permite avisar de que lleva tres dias sin aparecer.
  Future<void> _borrarManual(Equipo equipo) async {
    final confirmado = await showDialog<bool>(
      context: context,
      builder: (contextoModal) => AlertDialog(
        title: Text('Borrar ${equipo.comoSeLlama}'),
        content: const Text(
            'Se va tambien con sus puertos declarados y los cables que salian de ellas. '
            'Los equipos que colgaban de el vuelven a la zona de los que no estan ubicados.'),
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
    if (confirmado != true) return;

    try {
      await Api.instancia.borrarEquipoManual(_red.clave, equipo.id);
      _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  @override
  Widget build(BuildContext contexto) {
    return DefaultTabController(
      length: 5,
      child: Scaffold(
        appBar: AppBar(
          title: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(_red.nombre),
              // Cuando se escaneo por ultima vez, siempre a la vista: es lo que
              // dice si lo que se esta mirando sigue siendo cierto.
              Text(
                haceCuanto(_red.ultimoEscaneo).texto,
                style: Theme.of(contexto).textTheme.labelSmall,
              ),
            ],
          ),
          bottom: const TabBar(tabs: [
            Tab(icon: Icon(Icons.devices_other), text: 'Equipos'),
            Tab(icon: Icon(Icons.settings_input_component), text: 'Puertos'),
            Tab(icon: Icon(Icons.speed), text: 'Consumo'),
            Tab(icon: Icon(Icons.wifi), text: 'WiFi'),
            Tab(icon: Icon(Icons.route_outlined), text: 'Que se revisa'),
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
                      title: Text('Escanear toda la red'),
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
            PopupMenuButton<String>(
              tooltip: 'Mas',
              icon: const Icon(Icons.more_vert),
              onSelected: (opcion) {
                if (opcion == 'borrar') _borrar();
              },
              itemBuilder: (_) => const [
                PopupMenuItem(
                  value: 'borrar',
                  child: ListTile(
                    dense: true,
                    leading: Icon(Icons.delete_outline),
                    title: Text('Eliminar esta red'),
                  ),
                ),
              ],
            ),
            IconButton(
              tooltip: 'Alertas',
              icon: Badge(
                isLabelVisible: _red.alertasAbiertas > 0,
                label: Text('${_red.alertasAbiertas}'),
                child: const Icon(Icons.notifications_outlined),
              ),
              onPressed: () async {
                await Navigator.of(contexto).push(
                  MaterialPageRoute<void>(builder: (_) => PantallaAlertas(red: _red)),
                );
                _recargar();
              },
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
            _pestanaConsumo(contexto),
            const _PestanaAire(),
            _pestanaSubredes(contexto),
          ],
        ),
      ),
    );
  }

  /// _borrar quita la red, preguntando antes si tambien se van los datos.
  ///
  /// Por omision el borrado es **recuperable**: el archivo se conserva y volver
  /// a crear una red con el mismo nombre la revive con todo su historico. Quien
  /// quiera que no quede nada tiene que marcarlo a proposito, porque eso no
  /// tiene vuelta atras.
  Future<void> _borrar() async {
    final tambienLosDatos = await showDialog<bool>(
      context: context,
      builder: (_) => _DialogoBorrarRed(red: _red),
    );
    if (tambienLosDatos == null) return;

    try {
      await Api.instancia.borrarRed(_red.clave, tambienLosDatos: tambienLosDatos);
      if (!mounted) return;
      // Se vuelve al panel: la pantalla de una red que ya no existe no tiene
      // nada que mostrar.
      Navigator.of(context).pop();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
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
              const SizedBox(width: 8),
              // Un switch no administrable no tiene direccion y no sale en
              // ningun escaneo. Sin esta puerta no habria forma de meterlo.
              OutlinedButton.icon(
                icon: const Icon(Icons.add),
                label: const Text('A mano'),
                onPressed: _agregarAMano,
              ),
            ],
          ),
        ),
        _Composicion(composicion: _composicion),
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
                  alEditarFicha: () => _editarFicha(equipos[indice]),
                  alEditarPuertos: () => _editarPuertos(equipos[indice]),
                  alVerFicha: () => _verFicha(equipos[indice]),
                  alBorrar: () => _borrarManual(equipos[indice]),
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

  /// _pestanaPuertos es el mapa de puertos: que hay conectado en cada puerto.
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
        // sitio: primero el aparato, luego sus puertos.
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
            const SizedBox(height: 12),
            // El cableado se edita DESDE AQUI.
            //
            // El boton vive en esta pestana porque es donde va la mano de quien
            // quiere tocar el cableado. Estaba solo como un lapiz suelto dentro
            // de la pantalla del mapa, y ahi no lo encontraba nadie: la primera
            // persona que lo uso no lo vio.
            Row(
              children: [
                Expanded(
                  child: Text(
                    mapa.hayMapa
                        ? 'Lo que sus switches no puedan decir, declarelo usted.'
                        : 'Sus switches no pueden decir que hay en cada puerto, pero usted si: '
                            'declare sus aparatos y su cableado a mano.',
                    style: Theme.of(contexto).textTheme.bodySmall,
                  ),
                ),
                const SizedBox(width: 12),
                FilledButton.tonalIcon(
                  icon: const Icon(Icons.edit_outlined),
                  label: const Text('Editar el cableado'),
                  onPressed: () async {
                    await Navigator.of(contexto).push(
                      MaterialPageRoute<void>(
                        builder: (_) => PantallaMapa(red: _red, editarAlAbrir: true),
                      ),
                    );
                    _recargar();
                  },
                ),
              ],
            ),
            const SizedBox(height: 16),
            // Sin mapa NO se repite el mensaje de arriba: la explicacion de la
            // tarjeta ya dice por que, y decirlo dos veces suena a que falta
            // algo por hacer cuando muchas veces no falta nada.
            if (!mapa.hayMapa && mapa.capacidad == 'desconocida')
              const Padding(
                padding: EdgeInsets.all(24),
                child: Text('Corra un escaneo completo con el boton del radar.',
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
                        // MAC en un puerto, atras hay un switch no administrable
                        // y no se puede decir cual esta en que puerto.
                        trailing: renglon.confirmado
                            ? const Tooltip(
                                message: 'Unico equipo en este puerto: puerto exacto',
                                child: Chip(
                                  label: Text('Confirmado'),
                                  visualDensity: VisualDensity.compact,
                                ),
                              )
                            : Tooltip(
                                message: 'Hay ${renglon.cuantosEnPuerto} equipos en este puerto: '
                                    'atras cuelga un switch no administrable o un punto de acceso',
                                child: Chip(
                                  avatar: const Icon(Icons.hub, size: 16),
                                  label: Text('Grupo de ${renglon.cuantosEnPuerto}'),
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

  /// _pestanaConsumo responde "quien se esta comiendo el internet".
  ///
  /// El numero sale de los contadores que el switch ya llevaba, cruzados con el
  /// mapa de puertos. Cuando el puerto tiene varios equipos se dice: ese consumo
  /// es del grupo, no de uno.
  Widget _pestanaConsumo(BuildContext contexto) {
    return FutureBuilder<Consumo>(
      future: _consumo,
      builder: (_, resultado) {
        if (resultado.connectionState != ConnectionState.done) {
          return const Center(child: CircularProgressIndicator());
        }
        if (resultado.hasError) {
          return Center(
            child: TextButton(
              onPressed: () => mostrarProblema(contexto, resultado.error!),
              child: const Text('No se pudo cargar el consumo. Ver detalles'),
            ),
          );
        }

        final consumo = resultado.data!;
        final colores = Theme.of(contexto).colorScheme;
        final todos = [...consumo.puertos, ...consumo.porFlujos];
        final tope = todos.isEmpty
            ? 1
            : todos.map((p) => p.total).reduce((a, b) => a > b ? a : b);

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
                    Expanded(child: Text(consumo.explicacion)),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 16),
            if (consumo.puertos.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Text('Medido en los puertos del switch',
                    style: Theme.of(contexto).textTheme.titleSmall),
              ),
            for (final puerto in consumo.puertos)
              Card(
                margin: const EdgeInsets.only(bottom: 8),
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Expanded(
                            child: Text(puerto.quienEs,
                                style: const TextStyle(fontWeight: FontWeight.w600),
                                overflow: TextOverflow.ellipsis),
                          ),
                          Text(ConsumoDePuerto.enPalabras(puerto.total),
                              style: Theme.of(contexto).textTheme.titleMedium),
                        ],
                      ),
                      const SizedBox(height: 4),
                      Text([
                        '${puerto.switchNombre} ${puerto.puerto}',
                        if (puerto.equipoIp.isNotEmpty) puerto.equipoIp,
                        'baja ${ConsumoDePuerto.enPalabras(puerto.bpsEntrada)}',
                        'sube ${ConsumoDePuerto.enPalabras(puerto.bpsSalida)}',
                        if (!puerto.confirmado && puerto.cuantosEnPuerto > 1)
                          'del grupo entero',
                      ].join(' · '), style: Theme.of(contexto).textTheme.bodySmall),
                      const SizedBox(height: 8),
                      LinearProgressIndicator(
                        value: tope == 0 ? 0 : puerto.total / tope,
                        backgroundColor: colores.surfaceContainerHighest,
                      ),
                    ],
                  ),
                ),
              ),
            if (consumo.porFlujos.isNotEmpty) ...[
              const SizedBox(height: 8),
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Text(
                    consumo.porFlujos.any((equipo) => equipo.estimado)
                        // sFlow muestrea: decirle "medido" a eso seria hacer
                        // pasar una estimacion por una medicion.
                        ? 'Estimado en el router por muestreo (sFlow), ultimas 24 horas'
                        : 'Medido en el router, ultimas 24 horas',
                    style: Theme.of(contexto).textTheme.titleSmall),
              ),
              for (final equipo in consumo.porFlujos)
                Card(
                  margin: const EdgeInsets.only(bottom: 8),
                  child: Padding(
                    padding: const EdgeInsets.all(12),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Expanded(
                              child: Text(equipo.equipoNombre,
                                  style: const TextStyle(fontWeight: FontWeight.w600),
                                  overflow: TextOverflow.ellipsis),
                            ),
                            Text('${equipo.estimado ? '~' : ''}'
                                '${ConsumoDePuerto.enPalabras(equipo.total)}'),
                          ],
                        ),
                        const SizedBox(height: 4),
                        Text([
                          equipo.equipoIp,
                          'bajo ${ConsumoDePuerto.enPalabras(equipo.bpsEntrada)}',
                          'subio ${ConsumoDePuerto.enPalabras(equipo.bpsSalida)}',
                        ].join(' · '), style: Theme.of(contexto).textTheme.bodySmall),
                        const SizedBox(height: 8),
                        LinearProgressIndicator(
                          value: tope == 0 ? 0 : equipo.total / tope,
                          backgroundColor: colores.surfaceContainerHighest,
                        ),
                      ],
                    ),
                  ),
                ),
            ],
            if (!consumo.hayAlgo)
              const Padding(
                padding: EdgeInsets.all(24),
                child: Text('Todavia no hay mediciones de consumo.',
                    textAlign: TextAlign.center),
              ),
            _EnQueSeGasta(aplicaciones: _aplicaciones),
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
          // No se le pide nada: al escanear, MiRed averigua solo en que red esta
          // este equipo. Esto solo explica que va a pasar.
          return Center(
            child: Padding(
              padding: const EdgeInsets.all(32),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.radar, size: 40,
                      color: Theme.of(contexto).colorScheme.primary),
                  const SizedBox(height: 12),
                  const Text(
                    'Todavia no se ha escaneado nada.\n\n'
                    'Use el boton del radar y elija «Escanear toda la red»: '
                    'MiRed averigua solo en que red esta este equipo y la revisa '
                    'entera.',
                    textAlign: TextAlign.center,
                  ),
                ],
              ),
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
                        subred.escanear ? 'Se revisa' : 'No se revisa',
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
    required this.alEditarFicha,
    required this.alEditarPuertos,
    required this.alVerFicha,
    required this.alBorrar,
  });

  final Equipo equipo;
  final String clave;
  final VoidCallback alRenombrar;
  final VoidCallback alEditarFicha;
  final VoidCallback alEditarPuertos;
  /// Abre la ficha completa: propiedades, conexiones y credenciales.
  final VoidCallback alVerFicha;
  final VoidCallback alBorrar;

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
        // La IP va del tamano del renglon de abajo, no en gris chiquito: en una
        // red es LO que identifica a un aparato —el nombre puede faltar, estar
        // repetido o ser el que alguien le puso—, asi que tiene que leerse.
        //
        // Si el equipo todavia no tiene nombre, el titulo ya es su IP: repetirla
        // al lado no agrega nada.
        title: Row(
          children: [
            Flexible(
              child: Text(equipo.comoSeLlama,
                  style: const TextStyle(fontWeight: FontWeight.w600),
                  overflow: TextOverflow.ellipsis),
            ),
            if (equipo.ip.isNotEmpty && equipo.comoSeLlama != equipo.ip) ...[
              const SizedBox(width: 10),
              Text(equipo.ip,
                  style: Theme.of(contexto).textTheme.bodyMedium?.copyWith(
                      fontFamily: 'monospace', color: colores.onSurfaceVariant)),
            ],
          ],
        ),
        subtitle: Text([
          if (equipo.tipo.isNotEmpty) equipo.tipo,
          if (equipo.modelo.isNotEmpty) equipo.modelo,
          // El modelo que dijo el propio aparato, cuando nadie tecleo uno. Es
          // mas util que la MAC para saber que es esa caja.
          if (equipo.modelo.isEmpty && equipo.modeloDicho.isNotEmpty) equipo.modeloDicho,
          if (equipo.redQueEmite.isNotEmpty) 'WiFi ${equipo.redQueEmite}',
          if (equipo.fabricante.isNotEmpty) equipo.fabricante,
          if (equipo.mac.isNotEmpty) equipo.mac,
          if (equipo.puertos.isNotEmpty) '${equipo.puertos.length} puertos abiertos',
          // Se dice con todas sus letras: este renglon no lo encontro ningun
          // barrido, lo tecleo alguien.
          if (equipo.esManual) 'declarado a mano',
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
                // Lo mismo que muestra la ficha que abre el mapa: un solo sitio
                // donde se decide como se ve un aparato.
                PropiedadesDelEquipo(
                    clave: clave, equipo: equipo, alCambiar: alEditarFicha),

                const SizedBox(height: 8),
                Wrap(
                  spacing: 8,
                  children: [
                    TextButton.icon(
                      icon: const Icon(Icons.hub_outlined),
                      label: const Text('Conexiones y credenciales'),
                      onPressed: alVerFicha,
                    ),
                    TextButton.icon(
                      icon: const Icon(Icons.history),
                      label: const Text('Historial de conexiones'),
                      onPressed: () => showDialog<void>(
                        context: contexto,
                        builder: (_) => _DialogoPresencia(clave: clave, equipo: equipo),
                      ),
                    ),
                    TextButton.icon(
                      icon: const Icon(Icons.badge_outlined),
                      label: const Text('Ficha'),
                      onPressed: alEditarFicha,
                    ),
                    // Los puertos se declaran en CUALQUIER equipo, no solo en un
                    // switch tonto: un modem administrable que no habla SNMP
                    // hacia la LAN es el caso mas comun de todos.
                    TextButton.icon(
                      icon: const Icon(Icons.settings_input_hdmi),
                      label: const Text('Puertos'),
                      onPressed: alEditarPuertos,
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
                    // Borrar solo lo declarado. Lo descubierto se marca ausente
                    // y conserva su historia, que es de donde salen las alertas.
                    if (equipo.esManual)
                      TextButton.icon(
                        icon: const Icon(Icons.delete_outline),
                        label: const Text('Borrar'),
                        onPressed: alBorrar,
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

/// _Composicion dice de que esta hecha la red: cuantos aparatos hay y cuantos de
/// cada tipo.
///
/// La cuenta la hace el SERVIDOR sobre toda la red, no esta pantalla sobre lo
/// que tenga a la vista: con un filtro escrito o con «solo presentes» marcado,
/// contar aqui diria «3 equipos» en una red de treinta.
///
/// Sale de la misma tabla que la lista y que el mapa, asi que los tres no pueden
/// discrepar: un switch declarado a mano cuenta en cuanto se declara.
class _Composicion extends StatelessWidget {
  const _Composicion({required this.composicion});

  final Future<ComposicionDeRed> composicion;

  @override
  Widget build(BuildContext contexto) {
    return FutureBuilder<ComposicionDeRed>(
      future: composicion,
      builder: (_, resultado) {
        // Mientras no se sepa, no se pone nada: un cero parpadeando dice algo
        // que no es cierto.
        if (!resultado.hasData) return const SizedBox.shrink();

        final datos = resultado.data!;
        if (datos.total == 0) return const SizedBox.shrink();

        final colores = Theme.of(contexto).colorScheme;
        // El orden de la lista unica, no el que devuelva el servidor: asi lo que
        // arma la red va primero y lo que cuelga de ella despues, siempre igual.
        final cuentas = <String, CuentaPorCategoria>{
          for (final fila in datos.categorias) fila.categoria: fila,
        };
        final ordenadas = [
          for (final categoria in categoriasDeEquipo)
            if (cuentas.containsKey(categoria.clave)) cuentas[categoria.clave]!,
          // Lo que el servidor devuelva y no este en la lista tambien se
          // muestra: esconderlo daria un total que no cuadra con la suma.
          for (final fila in datos.categorias)
            if (buscarCategoria(fila.categoria) == null) fila,
        ];

        return Container(
          width: double.infinity,
          padding: const EdgeInsets.fromLTRB(16, 4, 16, 10),
          child: Wrap(
            spacing: 8,
            runSpacing: 8,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              // El total de la red, y aparte cuantos estan prendidos ahora. Un
              // equipo apagado sigue siendo parte de la red; contar solo los
              // prendidos haria bailar el numero todo el dia.
              Chip(
                avatar: Icon(Icons.lan_outlined, size: 18, color: colores.onPrimaryContainer),
                backgroundColor: colores.primaryContainer,
                label: Text(
                  datos.total == 1
                      ? '1 aparato · ${datos.presentes} prendido'
                      : '${datos.total} aparatos · ${datos.presentes} prendidos ahora',
                  style: TextStyle(
                      fontWeight: FontWeight.w600, color: colores.onPrimaryContainer),
                ),
              ),
              if (datos.declarados > 0)
                Tooltip(
                  message: 'Los declaro una persona; ningun escaneo los vio.\n'
                      'Un switch no administrable no tiene direccion y no puede verse de otro modo.',
                  child: Chip(
                    avatar: const Icon(Icons.edit_outlined, size: 16),
                    label: Text('${datos.declarados} a mano'),
                    visualDensity: VisualDensity.compact,
                  ),
                ),
              for (final fila in ordenadas)
                Tooltip(
                  message: [
                    buscarCategoria(fila.categoria)?.explicacion ?? '',
                    '${fila.presentes} de ${fila.cuantos} prendidos ahora',
                    if (fila.declarados > 0) '${fila.declarados} declarados a mano',
                  ].where((linea) => linea.isNotEmpty).join('\n'),
                  child: Chip(
                    avatar: Icon(iconoDeCategoria(fila.categoria), size: 16),
                    label: Text(
                      '${fila.cuantos} '
                      '${buscarCategoria(fila.categoria)?.cuantos(fila.cuantos) ?? fila.categoria}',
                    ),
                    visualDensity: VisualDensity.compact,
                  ),
                ),
            ],
          ),
        );
      },
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
          if (_propuesta != null) ...[
            TextButton.icon(
              icon: Icon(_copiado ? Icons.check : Icons.copy),
              label: Text(_copiado ? 'Copiado' : 'Copiar'),
              onPressed: () async {
                await Clipboard.setData(
                    ClipboardData(text: _propuesta!['contenido'] as String));
                if (contexto.mounted) setState(() => _copiado = true);
              },
            ),
            // Compartirlo es un acto de una PERSONA, con su cuenta: MiRed deja
            // el aporte escrito y quien decide publicarlo es quien lo abre.
            TextButton.icon(
              icon: const Icon(Icons.ios_share),
              label: const Text('Compartir con la comunidad'),
              onPressed: _ocupado ? null : _compartir,
            ),
            FilledButton.icon(
              icon: const Icon(Icons.save_outlined),
              label: const Text('Guardar en mi catalogo'),
              onPressed: _ocupado ? null : _guardar,
            ),
          ],
          TextButton(
            onPressed: () => Navigator.of(contexto).pop(),
            child: const Text('Cerrar'),
          ),
        ],
      );

  /// _guardar deja la definicion en el catalogo de esta instalacion.
  Future<void> _guardar() async {
    final propuesta = _propuesta;
    if (propuesta == null) return;

    setState(() => _ocupado = true);
    try {
      final guardado = await Api.instancia.guardarDefinicion(
        propuesta['archivo'] as String,
        propuesta['contenido'] as String,
      );
      if (!mounted) return;
      mensajeExito(context,
          'Guardado en ${guardado['ruta']}. El catalogo ya tiene '
          '${guardado['definiciones']} definiciones; este aparato se reconoce '
          'desde el proximo escaneo.');
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _ocupado = false);
    }
  }

  /// _compartir abre el aporte ya escrito contra el repositorio del proyecto.
  Future<void> _compartir() async {
    final propuesta = _propuesta;
    if (propuesta == null) return;

    final direccion = propuesta['urlAporte'] as String? ?? '';
    if (direccion.isEmpty) return;

    // Si no se puede abrir el navegador —un equipo sin escritorio—, al menos
    // queda la direccion en el portapapeles: quedarse callado seria peor.
    final abierto = await abrirEnElNavegador(direccion);
    if (!mounted) return;
    if (!abierto) {
      await Clipboard.setData(ClipboardData(text: direccion));
      if (mounted) {
        mensajeAviso(context,
            'No se pudo abrir el navegador. La direccion del aporte quedo copiada.');
      }
    }
  }
}

/// _EnQueSeGasta responde "en que" se va el ancho de banda, no solo "cuanto".
///
/// Lo llena el paquete OPCIONAL mired-dpi. Si no esta instalado esto no se
/// dibuja: una seccion vacia con un mensaje de error en cada pantalla seria un
/// castigo por no haber instalado algo que es opcional a proposito.
class _EnQueSeGasta extends StatelessWidget {
  const _EnQueSeGasta({required this.aplicaciones});

  final Future<List<ConsumoPorAplicacion>> aplicaciones;

  @override
  Widget build(BuildContext contexto) {
    return FutureBuilder<List<ConsumoPorAplicacion>>(
      future: aplicaciones,
      builder: (_, resultado) {
        final lista = resultado.data ?? [];
        if (lista.isEmpty) return const SizedBox.shrink();

        final colores = Theme.of(contexto).colorScheme;
        final tope = lista.first.bytes;

        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const SizedBox(height: 16),
            Padding(
              padding: const EdgeInsets.only(bottom: 4),
              child: Text('En que se gasta, ultimas 24 horas',
                  style: Theme.of(contexto).textTheme.titleSmall),
            ),
            Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: Text(
                'Sale de la inspeccion profunda. El nombre no se obtiene descifrando nada: '
                'viaja en claro en el saludo de TLS, en la cabecera Host de HTTP y en las '
                'consultas de DNS.',
                style: Theme.of(contexto).textTheme.bodySmall,
              ),
            ),
            for (final renglon in lista.take(30))
              Card(
                margin: const EdgeInsets.only(bottom: 8),
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Expanded(
                            child: Text(renglon.aplicacion,
                                style: const TextStyle(fontWeight: FontWeight.w600),
                                overflow: TextOverflow.ellipsis),
                          ),
                          Text(renglon.enPalabras),
                        ],
                      ),
                      const SizedBox(height: 4),
                      Text([
                        renglon.equipo,
                        // De donde salio el nombre se dice siempre: un nombre
                        // del saludo de TLS no vale lo mismo que una suposicion
                        // por numero de puerto.
                        renglon.procedencia,
                      ].join(' · '), style: Theme.of(contexto).textTheme.bodySmall),
                      const SizedBox(height: 8),
                      LinearProgressIndicator(
                        value: tope == 0 ? 0 : renglon.bytes / tope,
                        backgroundColor: colores.surfaceContainerHighest,
                      ),
                    ],
                  ),
                ),
              ),
          ],
        );
      },
    );
  }
}

/// _DialogoBorrarRed pregunta que tan a fondo hay que borrar.
///
/// Se separan las dos cosas porque son muy distintas: quitar una red de la vista
/// se deshace creandola otra vez con el mismo nombre; borrar sus datos, no. Un
/// solo boton que hiciera lo segundo sin avisar seria una trampa.
class _DialogoBorrarRed extends StatefulWidget {
  const _DialogoBorrarRed({required this.red});

  final Red red;

  @override
  State<_DialogoBorrarRed> createState() => _DialogoBorrarRedState();
}

class _DialogoBorrarRedState extends State<_DialogoBorrarRed> {
  bool _tambienLosDatos = false;

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;

    return AlertDialog(
      title: Text('Eliminar «${widget.red.nombre}»'),
      content: SizedBox(
        width: 460,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Se quitara del panel con sus ${widget.red.equipos} equipos, '
                'su historico y sus alertas.'),
            const SizedBox(height: 12),
            CheckboxListTile(
              contentPadding: EdgeInsets.zero,
              value: _tambienLosDatos,
              onChanged: (valor) => setState(() => _tambienLosDatos = valor ?? false),
              title: const Text('Borrar tambien los datos del disco'),
              subtitle: const Text('No se puede deshacer'),
            ),
            const SizedBox(height: 4),
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: _tambienLosDatos
                    ? colores.errorContainer
                    : colores.surfaceContainerHighest,
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(
                    _tambienLosDatos ? Icons.warning_amber_outlined : Icons.info_outline,
                    size: 18,
                    color: _tambienLosDatos ? colores.onErrorContainer : null,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      _tambienLosDatos
                          ? 'Se borrara el archivo de datos. Todo lo descubierto en '
                              'este sitio se pierde para siempre.'
                          : 'El archivo de datos se conserva: puede recuperar este '
                              'sitio creando una red con el mismo nombre.',
                      style: Theme.of(contexto).textTheme.bodySmall?.copyWith(
                          color: _tambienLosDatos ? colores.onErrorContainer : null),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(contexto).pop(),
          child: const Text('Cancelar'),
        ),
        FilledButton(
          style: _tambienLosDatos
              ? FilledButton.styleFrom(backgroundColor: colores.error)
              : null,
          onPressed: () => Navigator.of(contexto).pop(_tambienLosDatos),
          child: Text(_tambienLosDatos ? 'Borrar todo' : 'Eliminar'),
        ),
      ],
    );
  }
}

/// _PestanaAire muestra que redes inalambricas se oyen desde este equipo.
///
/// Es la unica medicion de MiRed que no pasa por un cable, y contesta lo que
/// ningun barrido de IP puede: que SSID emite cada antena y con que **MAC de
/// radio**, que no es la misma que la del cable.
///
/// No se barre solo al abrir la pestana: recorrer los canales corta el WiFi de
/// este equipo unos segundos, y hacerlo sin que nadie lo pida seria cortarle la
/// conexion a alguien por dibujar una lista.
class _PestanaAire extends StatefulWidget {
  const _PestanaAire();

  @override
  State<_PestanaAire> createState() => _PestanaAireState();
}

class _PestanaAireState extends State<_PestanaAire> {
  Map<String, dynamic>? _resultado;
  bool _barriendo = false;

  Future<void> _barrer() async {
    setState(() => _barriendo = true);
    try {
      final clave = ModalRoute.of(context)?.settings.arguments as String? ?? '';
      final datos = await Api.instancia.barrerAire(clave.isEmpty ? _claveDeLaRuta() : clave);
      if (mounted) setState(() => _resultado = datos);
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _barriendo = false);
    }
  }

  /// La clave de la red se saca de la pantalla que contiene esta pestana.
  String _claveDeLaRuta() {
    final pantalla = context.findAncestorStateOfType<_PantallaRedState>();
    return pantalla?.widget.red.clave ?? '';
  }

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;
    final redes = (_resultado?['redes'] as List<dynamic>? ?? []).cast<Map<String, dynamic>>();
    final explicacion = _resultado?['explicacion'] as String? ?? '';

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('El aire', style: Theme.of(contexto).textTheme.titleMedium),
                const SizedBox(height: 6),
                const Text(
                  'Que redes inalambricas se oyen desde este equipo: el nombre que emite '
                  'cada antena, la MAC de su radio —que no es la del cable—, en que canal '
                  'esta y con que fuerza llega. Hace falta una tarjeta WiFi en este equipo: '
                  'una PC conectada solo por cable no puede oir nada.',
                ),
                const SizedBox(height: 12),
                Row(
                  children: [
                    FilledButton.icon(
                      icon: _barriendo
                          ? const SizedBox(
                              width: 16, height: 16,
                              child: CircularProgressIndicator(strokeWidth: 2))
                          : const Icon(Icons.wifi_find),
                      label: Text(_barriendo ? 'Escuchando...' : 'Escuchar el aire'),
                      onPressed: _barriendo ? null : _barrer,
                    ),
                    if (_resultado != null) ...[
                      const SizedBox(width: 12),
                      Text('${redes.length} redes · ${_resultado!['duracionMs']} ms',
                          style: TextStyle(color: colores.outline)),
                    ],
                  ],
                ),
              ],
            ),
          ),
        ),

        // Cuando no se puede oir nada se DICE por que. Una lista vacia sin
        // explicacion se leeria como "aqui no hay redes WiFi".
        if (explicacion.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(top: 12),
            child: Card(
              color: colores.secondaryContainer,
              child: Padding(padding: const EdgeInsets.all(16), child: Text(explicacion)),
            ),
          ),

        for (final red in redes)
          Card(
            margin: const EdgeInsets.only(top: 8),
            child: ListTile(
              leading: Icon(_iconoDeSenal(red['senal'] as int? ?? -100)),
              title: Text((red['ssid'] as String? ?? '').isEmpty
                  ? 'Red oculta'
                  : red['ssid'] as String),
              subtitle: Text([
                red['bssid'] as String? ?? '',
                if ((red['banda'] as String? ?? '').isNotEmpty)
                  '${red['banda']} canal ${red['canal']}',
                '${red['senal']} dBm',
                if ((red['seguridad'] as String? ?? '').isNotEmpty) red['seguridad'] as String,
                // De quien es la antena, y COMO se supo: medido no es lo mismo
                // que deducido por la cercania de la MAC.
                if (red['equipoNombre'] != null)
                  'es ${red['equipoNombre']} (${red['comoSeSupo']})',
              ].join(' · ')),
            ),
          ),
      ],
    );
  }

  IconData _iconoDeSenal(int dbm) {
    if (dbm >= -55) return Icons.wifi;
    if (dbm >= -70) return Icons.wifi_2_bar;
    return Icons.wifi_1_bar;
  }
}
