import 'dart:async';

import 'package:flutter/material.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../servicios/trayectoria.dart';
import '../widgets/mensajes.dart';

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

  final _busqueda = TextEditingController();
  String _filtro = '';
  bool _soloPresentes = false;
  bool _escaneando = false;
  Timer? _vigilante;

  @override
  void initState() {
    super.initState();
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
      _equipos = Api.instancia.listarEquipos(widget.red.clave, soloPresentes: _soloPresentes);
      _subredes = Api.instancia.listarSubredes(widget.red.clave);
    });
  }

  /// Si al abrir la pantalla ya hay un escaneo corriendo (lo lanzo otra persona,
  /// o esta pestana se recargo), hay que engancharse a el en vez de fingir que
  /// no pasa nada.
  Future<void> _revisarEscaneoEnCurso() async {
    try {
      final corridas = await Api.instancia.listarEscaneos(widget.red.clave);
      if (corridas.isNotEmpty && corridas.first.enCurso && mounted) {
        setState(() => _escaneando = true);
        _vigilar();
      }
    } catch (_) {
      // Sin escaneos que revisar no pasa nada.
    }
  }

  Future<void> _escanear({required bool soloPresencia}) async {
    setState(() => _escaneando = true);
    Trayectoria.instancia.anotar('Escanear ${widget.red.nombre}');

    try {
      await Api.instancia.lanzarEscaneo(widget.red.clave, soloPresencia: soloPresencia);
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
        final corridas = await Api.instancia.listarEscaneos(widget.red.clave);
        if (corridas.isEmpty || corridas.first.enCurso) return;

        reloj.cancel();
        if (!mounted) return;
        setState(() => _escaneando = false);
        _recargar();

        final ultima = corridas.first;
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
      builder: (_) => _DialogoNuevaSubred(clave: widget.red.clave),
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
      await Api.instancia.ponerAlias(widget.red.clave, equipo.id, nuevo.trim());
      _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  @override
  Widget build(BuildContext contexto) {
    return DefaultTabController(
      length: 2,
      child: Scaffold(
        appBar: AppBar(
          title: Text(widget.red.nombre),
          bottom: const TabBar(tabs: [
            Tab(icon: Icon(Icons.devices_other), text: 'Equipos'),
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
            Text('Archivo de esta red: ${widget.red.clave}.db',
                style: Theme.of(contexto).textTheme.labelSmall),
          ],
        );
      },
    );
  }
}

class _TarjetaEquipo extends StatelessWidget {
  const _TarjetaEquipo({required this.equipo, required this.alRenombrar});

  final Equipo equipo;
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
                _Renglon(etiqueta: 'Nombre descubierto', valor: equipo.nombre),
                _Renglon(etiqueta: 'MAC', valor: equipo.mac),
                _Renglon(etiqueta: 'Fabricante', valor: equipo.fabricante),
                _Renglon(etiqueta: 'Subred', valor: equipo.subred),
                // Se dice como se vio: no es lo mismo "esta" que "algo contesto
                // en esa direccion", y ocultarlo seria mentir sobre la certeza.
                _Renglon(etiqueta: 'Certeza', valor: equipo.certeza),
                _Renglon(etiqueta: 'Visto por primera vez', valor: equipo.primeraVez),
                _Renglon(etiqueta: 'Visto por ultima vez', valor: equipo.ultimaVez),
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
