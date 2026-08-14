import 'package:flutter/material.dart';

import '../modelos/categorias.dart';
import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../widgets/mensajes.dart';

/// Los formularios de lo que se declara a mano.
///
/// Viven aparte porque los usan dos pantallas: la ficha del equipo (pestana de
/// equipos) y el modo edicion del mapa. Si estuvieran dentro de una de las dos,
/// la otra acabaria con una copia que se desincroniza.
///
/// Todos devuelven `true` cuando algo cambio, para que quien los abrio recargue.

/// DialogoEquipoManual da de alta un aparato que ningun barrido va a encontrar.
///
/// El caso que lo justifica: un switch no administrable no tiene direccion, no
/// contesta a nada y no existe para ningun escaneo. Sin esto, el cable que pasa
/// por el no se puede dibujar y el mapa muestra ocho equipos flotando donde en
/// realidad hay un arbol.
class DialogoEquipoManual extends StatefulWidget {
  const DialogoEquipoManual({super.key, required this.clave});

  final String clave;

  @override
  State<DialogoEquipoManual> createState() => _DialogoEquipoManualState();
}

class _DialogoEquipoManualState extends State<DialogoEquipoManual> {
  final _nombre = TextEditingController();
  final _modelo = TextEditingController();
  final _notas = TextEditingController();
  final _ip = TextEditingController();

  /// Arranca en el switch no administrable: es la razon numero uno por la que
  /// alguien abre este formulario, porque es lo unico que ningun escaneo ve.
  String _clave = 'switch_simple';
  int _bocas = 8;
  String _conexion = '';
  bool _guardando = false;

  @override
  void dispose() {
    _nombre.dispose();
    _modelo.dispose();
    _notas.dispose();
    _ip.dispose();
    super.dispose();
  }

  CategoriaEquipo get _categoria =>
      buscarCategoria(_clave) ?? categoriasParaDeclarar.first;

  Future<void> _guardar() async {
    if (_nombre.text.trim().isEmpty) {
      mensajeAviso(context, 'Pongale un nombre para poder reconocerlo en el mapa.');
      return;
    }
    setState(() => _guardando = true);
    try {
      final creado = await Api.instancia.crearEquipoManual(widget.clave, {
        'nombre': _nombre.text.trim(),
        // La clave para contar y el nombre para leer, los dos de la MISMA
        // lista: es lo que hace que un aparato declarado y uno descubierto
        // cuenten juntos en vez de sacar dos cubos para la misma cosa.
        'categoria': _categoria.clave,
        'tipo': _categoria.singular,
        'modelo': _modelo.text.trim(),
        'notas': _notas.text.trim(),
        'ip': _ip.text.trim(),
        'conexion': _categoria.preguntaConexion ? _conexion : '',
        'puertos': _categoria.declaraBocas ? _bocas : 0,
      });
      if (mounted) Navigator.of(context).pop(creado);
    } catch (problema, pila) {
      if (mounted) {
        setState(() => _guardando = false);
        await mostrarProblema(context, problema, pila: pila.toString());
      }
    }
  }

  @override
  Widget build(BuildContext contexto) => AlertDialog(
        title: const Text('Agregar un aparato a mano'),
        content: SizedBox(
          width: 460,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Para lo que ningun escaneo puede ver: un switch no administrable, '
                  'un modem sin SNMP hacia la LAN. Queda marcado como declarado a mano '
                  'y se dibuja punteado, para que no se confunda con lo medido.',
                  style: Theme.of(contexto).textTheme.bodySmall,
                ),
                const SizedBox(height: 16),
                // Sale de la lista unica de MiRed, **nunca es texto libre**: es
                // la misma con la que el catalogo clasifica lo que descubre.
                DropdownButtonFormField<String>(
                  initialValue: _clave,
                  isExpanded: true,
                  decoration: const InputDecoration(
                      labelText: 'Que es', border: OutlineInputBorder(), isDense: true),
                  items: [
                    for (final categoria in categoriasParaDeclarar)
                      DropdownMenuItem(
                        value: categoria.clave,
                        child: Row(children: [
                          Icon(categoria.icono, size: 18),
                          const SizedBox(width: 8),
                          Flexible(
                              child: Text(categoria.singular,
                                  overflow: TextOverflow.ellipsis)),
                          // Lo que ningun escaneo puede ver se dice aqui: es
                          // exactamente para eso que existe este formulario.
                          if (!categoria.automatica) ...[
                            const SizedBox(width: 6),
                            const Icon(Icons.edit_outlined, size: 14),
                          ],
                        ]),
                      ),
                  ],
                  onChanged: (valor) => setState(() => _clave = valor ?? 'switch_simple'),
                ),
                Padding(
                  padding: const EdgeInsets.only(top: 6),
                  child: Text(_categoria.explicacion,
                      style: Theme.of(contexto).textTheme.bodySmall),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _nombre,
                  autofocus: true,
                  maxLength: 120, // lo mismo que acepta la columna
                  decoration: const InputDecoration(
                    labelText: 'Como se le conoce',
                    hintText: 'Switch del rack, Modem de la sala...',
                    border: OutlineInputBorder(),
                    isDense: true,
                    counterText: '',
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _modelo,
                  maxLength: 120,
                  decoration: const InputDecoration(
                    labelText: 'Modelo (opcional)',
                    hintText: 'TP-Link SG108, Router Telmex HG6145F3...',
                    border: OutlineInputBorder(),
                    isDense: true,
                    counterText: '',
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _ip,
                  maxLength: 45,
                  decoration: const InputDecoration(
                    labelText: 'Direccion IP (si tiene)',
                    hintText: 'Un switch tonto no tiene: dejelo vacio',
                    border: OutlineInputBorder(),
                    isDense: true,
                    counterText: '',
                  ),
                ),
                if (_categoria.declaraBocas) ...[
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      const Text('Cuantas bocas tiene'),
                      const Spacer(),
                      IconButton(
                        icon: const Icon(Icons.remove_circle_outline),
                        onPressed: _bocas > 0 ? () => setState(() => _bocas--) : null,
                      ),
                      Text('$_bocas', style: Theme.of(contexto).textTheme.titleMedium),
                      IconButton(
                        icon: const Icon(Icons.add_circle_outline),
                        // 512 es el tope de la columna: el formulario no deja
                        // capturar lo que la base va a rechazar.
                        onPressed: _bocas < 512 ? () => setState(() => _bocas++) : null,
                      ),
                    ],
                  ),
                ],
                if (_categoria.preguntaConexion) ...[
                  const SizedBox(height: 12),
                  DropdownButtonFormField<String>(
                    initialValue: _conexion,
                    decoration: const InputDecoration(
                        labelText: 'Como se conecta', border: OutlineInputBorder(), isDense: true),
                    items: const [
                      DropdownMenuItem(value: '', child: Text('No se sabe')),
                      DropdownMenuItem(value: 'cable', child: Text('Por cable')),
                      DropdownMenuItem(value: 'wifi', child: Text('Por WiFi')),
                    ],
                    onChanged: (valor) => setState(() => _conexion = valor ?? ''),
                  ),
                ],
                const SizedBox(height: 12),
                TextField(
                  controller: _notas,
                  maxLines: 2,
                  maxLength: 2000,
                  decoration: const InputDecoration(
                    labelText: 'Notas (opcional)',
                    border: OutlineInputBorder(),
                    isDense: true,
                    counterText: '',
                  ),
                ),
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
              onPressed: _guardando ? null : () => Navigator.of(contexto).pop(),
              child: const Text('Cancelar')),
          FilledButton(
            onPressed: _guardando ? null : _guardar,
            child: _guardando
                ? const SizedBox(
                    height: 16, width: 16, child: CircularProgressIndicator(strokeWidth: 2))
                : const Text('Agregar'),
          ),
        ],
      );
}

/// DialogoFicha guarda lo que una persona sabe del equipo y ninguna herramienta
/// puede averiguar: el modelo, una nota y si esta por cable o por WiFi.
///
/// Sirve igual para lo descubierto y para lo declarado. Por eso NO es "editar un
/// equipo manual": el modelo de una laptop descubierta tampoco sale de ningun
/// escaneo.
class DialogoFicha extends StatefulWidget {
  const DialogoFicha({super.key, required this.clave, required this.equipo});

  final String clave;
  final Equipo equipo;

  @override
  State<DialogoFicha> createState() => _DialogoFichaState();
}

class _DialogoFichaState extends State<DialogoFicha> {
  late final TextEditingController _modelo;
  late final TextEditingController _notas;
  late String _conexion;
  bool _guardando = false;

  @override
  void initState() {
    super.initState();
    _modelo = TextEditingController(text: widget.equipo.modelo);
    _notas = TextEditingController(text: widget.equipo.notas);
    _conexion = widget.equipo.conexion;
  }

  @override
  void dispose() {
    _modelo.dispose();
    _notas.dispose();
    super.dispose();
  }

  Future<void> _guardar() async {
    setState(() => _guardando = true);
    try {
      await Api.instancia.guardarFicha(widget.clave, widget.equipo.id,
          modelo: _modelo.text.trim(), notas: _notas.text.trim(), conexion: _conexion);
      if (mounted) Navigator.of(context).pop(true);
    } catch (problema, pila) {
      if (mounted) {
        setState(() => _guardando = false);
        await mostrarProblema(context, problema, pila: pila.toString());
      }
    }
  }

  @override
  Widget build(BuildContext contexto) => AlertDialog(
        title: Text('Ficha de ${widget.equipo.comoSeLlama}'),
        content: SizedBox(
          width: 460,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: _modelo,
                autofocus: true,
                maxLength: 120, // lo mismo que acepta la columna
                decoration: const InputDecoration(
                  labelText: 'Modelo',
                  hintText: 'TP-Link SG108, Dell Latitude 5420...',
                  border: OutlineInputBorder(),
                  isDense: true,
                  counterText: '',
                ),
              ),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                initialValue: _conexion,
                decoration: const InputDecoration(
                    labelText: 'Como se conecta', border: OutlineInputBorder(), isDense: true),
                items: const [
                  DropdownMenuItem(value: '', child: Text('No se sabe / no aplica')),
                  DropdownMenuItem(value: 'cable', child: Text('Por cable')),
                  DropdownMenuItem(value: 'wifi', child: Text('Por WiFi')),
                ],
                onChanged: (valor) => setState(() => _conexion = valor ?? ''),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _notas,
                maxLines: 3,
                maxLength: 2000, // lo mismo que acepta la columna
                decoration: const InputDecoration(
                  labelText: 'Notas',
                  border: OutlineInputBorder(),
                  isDense: true,
                  counterText: '',
                ),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
              onPressed: _guardando ? null : () => Navigator.of(contexto).pop(),
              child: const Text('Cancelar')),
          FilledButton(
              onPressed: _guardando ? null : _guardar, child: const Text('Guardar')),
        ],
      );
}

/// DialogoBocas administra las bocas declaradas de un equipo.
///
/// No es solo para switches tontos: un modem administrable sin SNMP hacia la LAN
/// —el caso mas comun de todos— necesita exactamente lo mismo.
class DialogoBocas extends StatefulWidget {
  const DialogoBocas({super.key, required this.clave, required this.equipo});

  final String clave;
  final Equipo equipo;

  @override
  State<DialogoBocas> createState() => _DialogoBocasState();
}

class _DialogoBocasState extends State<DialogoBocas> {
  late Future<TopologiaManual> _topologia;
  bool _hubocambios = false;

  @override
  void initState() {
    super.initState();
    _recargar();
  }

  void _recargar() {
    setState(() => _topologia = Api.instancia.topologiaManual(widget.clave));
  }

  Future<void> _agregar(List<PuertoFisico> actuales) async {
    final siguiente = actuales
            .where((boca) => boca.tipo == 'lan')
            .fold<int>(0, (mayor, boca) => boca.numero > mayor ? boca.numero : mayor) +
        1;

    final nueva = await showDialog<({int numero, String tipo, int? velocidad})>(
      context: context,
      builder: (_) => _DialogoBoca(numero: siguiente),
    );
    if (nueva == null) return;

    try {
      await Api.instancia.agregarPuerto(widget.clave, widget.equipo.id,
          numero: nueva.numero, tipo: nueva.tipo, velocidadMbps: nueva.velocidad);
      _hubocambios = true;
      _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  Future<void> _borrar(PuertoFisico boca) async {
    try {
      await Api.instancia.borrarPuerto(widget.clave, boca.id);
      _hubocambios = true;
      _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  @override
  Widget build(BuildContext contexto) => AlertDialog(
        title: Text('Bocas de ${widget.equipo.comoSeLlama}'),
        content: SizedBox(
          width: 460,
          height: 380,
          child: FutureBuilder<TopologiaManual>(
            future: _topologia,
            builder: (_, resultado) {
              if (resultado.connectionState != ConnectionState.done) {
                return const Center(child: CircularProgressIndicator());
              }
              if (resultado.hasError) {
                return Center(
                  child: TextButton(
                    onPressed: () => mostrarProblema(contexto, resultado.error!),
                    child: const Text('No se pudieron cargar las bocas. Ver detalles'),
                  ),
                );
              }

              final topologia = resultado.data!;
              final bocas = topologia.puertosDe(widget.equipo.id);

              return Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(
                    'Cuente las bocas mirando el aparato. Lo que se declare aqui se '
                    'dibuja punteado en el mapa: es lo que usted sabe, no lo que se midio.',
                    style: Theme.of(contexto).textTheme.bodySmall,
                  ),
                  const SizedBox(height: 8),
                  Expanded(
                    child: bocas.isEmpty
                        ? const Center(child: Text('Todavia no hay bocas declaradas.'))
                        : ListView.builder(
                            itemCount: bocas.length,
                            itemBuilder: (_, indice) {
                              final boca = bocas[indice];
                              final cable = topologia.enlaceDe(boca.id);
                              return ListTile(
                                dense: true,
                                leading: Icon(boca.tipo == 'wan'
                                    ? Icons.public
                                    : Icons.settings_input_hdmi),
                                title: Text(boca.tipo == 'wan'
                                    ? 'Boca WAN'
                                    : 'Boca ${boca.numero}'),
                                subtitle: Text([
                                  if (boca.velocidadMbps != null) '${boca.velocidadMbps} Mbps',
                                  if (cable == null)
                                    'sin conectar'
                                  else
                                    'a ${cable.puertoOrigenId == boca.id ? cable.destinoNombre : cable.origenNombre}',
                                ].join(' · ')),
                                trailing: IconButton(
                                  tooltip: 'Quitar la boca',
                                  icon: const Icon(Icons.delete_outline),
                                  onPressed: () => _borrar(boca),
                                ),
                              );
                            },
                          ),
                  ),
                  const SizedBox(height: 8),
                  OutlinedButton.icon(
                    icon: const Icon(Icons.add),
                    label: const Text('Agregar boca'),
                    onPressed: () => _agregar(bocas),
                  ),
                ],
              );
            },
          ),
        ),
        actions: [
          FilledButton(
              onPressed: () => Navigator.of(contexto).pop(_hubocambios),
              child: const Text('Listo')),
        ],
      );
}

class _DialogoBoca extends StatefulWidget {
  const _DialogoBoca({required this.numero});

  final int numero;

  @override
  State<_DialogoBoca> createState() => _DialogoBocaState();
}

class _DialogoBocaState extends State<_DialogoBoca> {
  late final TextEditingController _numero;
  String _tipo = 'lan';
  int? _velocidad;

  @override
  void initState() {
    super.initState();
    _numero = TextEditingController(text: '${widget.numero}');
  }

  @override
  void dispose() {
    _numero.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext contexto) => AlertDialog(
        title: const Text('Nueva boca'),
        content: SizedBox(
          width: 360,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              DropdownButtonFormField<String>(
                initialValue: _tipo,
                decoration: const InputDecoration(
                    labelText: 'Tipo', border: OutlineInputBorder(), isDense: true),
                items: const [
                  DropdownMenuItem(value: 'lan', child: Text('LAN')),
                  DropdownMenuItem(value: 'wan', child: Text('WAN (hacia el proveedor)')),
                ],
                onChanged: (valor) => setState(() => _tipo = valor ?? 'lan'),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _numero,
                keyboardType: TextInputType.number,
                decoration: const InputDecoration(
                  labelText: 'Numero',
                  helperText: 'Entre 1 y 512, como viene rotulado en el aparato',
                  border: OutlineInputBorder(),
                  isDense: true,
                ),
              ),
              const SizedBox(height: 12),
              DropdownButtonFormField<int?>(
                initialValue: _velocidad,
                decoration: const InputDecoration(
                    labelText: 'Velocidad', border: OutlineInputBorder(), isDense: true),
                // Sin dato se guarda vacio: mejor eso que un 100 inventado que
                // despues alguien lea como medido.
                items: const [
                  DropdownMenuItem(value: null, child: Text('No se sabe')),
                  DropdownMenuItem(value: 10, child: Text('10 Mbps')),
                  DropdownMenuItem(value: 100, child: Text('100 Mbps')),
                  DropdownMenuItem(value: 1000, child: Text('1 Gbps')),
                  DropdownMenuItem(value: 2500, child: Text('2.5 Gbps')),
                  DropdownMenuItem(value: 10000, child: Text('10 Gbps')),
                ],
                onChanged: (valor) => setState(() => _velocidad = valor),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(contexto).pop(), child: const Text('Cancelar')),
          FilledButton(
            onPressed: () {
              final numero = int.tryParse(_numero.text.trim()) ?? 0;
              if (numero < 1 || numero > 512) {
                mensajeAviso(contexto, 'El numero de boca tiene que estar entre 1 y 512.');
                return;
              }
              Navigator.of(contexto).pop((numero: numero, tipo: _tipo, velocidad: _velocidad));
            },
            child: const Text('Agregar'),
          ),
        ],
      );
}

/// DialogoElegirEquipo lista los equipos que todavia no estan en ninguna boca,
/// para colgar uno de la que se toco.
///
/// Solo se ofrecen los que no estan ubicados: ofrecer todos dejaria mover por
/// accidente un equipo que un switch ya reporto, y eso no se corrige tecleando,
/// se corrige moviendo el cable.
class DialogoElegirEquipo extends StatefulWidget {
  const DialogoElegirEquipo({super.key, required this.candidatos});

  final List<Equipo> candidatos;

  @override
  State<DialogoElegirEquipo> createState() => _DialogoElegirEquipoState();
}

class _DialogoElegirEquipoState extends State<DialogoElegirEquipo> {
  String _filtro = '';

  @override
  Widget build(BuildContext contexto) {
    final lista = widget.candidatos
        .where((equipo) => _filtro.isEmpty ||
            '${equipo.comoSeLlama} ${equipo.ip} ${equipo.mac} ${equipo.fabricante}'
                .toLowerCase()
                .contains(_filtro))
        .toList();

    return AlertDialog(
      title: const Text('Conectar un equipo ya conocido'),
      content: SizedBox(
        width: 460,
        height: 380,
        child: Column(
          children: [
            TextField(
              decoration: const InputDecoration(
                prefixIcon: Icon(Icons.search),
                hintText: 'Buscar por nombre, IP, MAC o fabricante',
                border: OutlineInputBorder(),
                isDense: true,
              ),
              onChanged: (texto) => setState(() => _filtro = texto.toLowerCase()),
            ),
            const SizedBox(height: 8),
            Expanded(
              child: lista.isEmpty
                  ? const Center(
                      child: Padding(
                        padding: EdgeInsets.all(24),
                        child: Text(
                          'No queda ningun equipo sin ubicar. Los que ya cuelgan de una '
                          'boca no se mueven desde aqui: eso se corrige moviendo el cable.',
                          textAlign: TextAlign.center,
                        ),
                      ),
                    )
                  : ListView.builder(
                      itemCount: lista.length,
                      itemBuilder: (_, indice) {
                        final equipo = lista[indice];
                        return ListTile(
                          dense: true,
                          leading: Icon(
                            equipo.presente ? Icons.circle : Icons.circle_outlined,
                            size: 12,
                            color: equipo.presente ? Colors.green : null,
                          ),
                          title: Text(equipo.comoSeLlama),
                          subtitle: Text([
                            if (equipo.ip.isNotEmpty) equipo.ip,
                            if (equipo.tipo.isNotEmpty) equipo.tipo,
                            if (equipo.fabricante.isNotEmpty) equipo.fabricante,
                          ].join(' · ')),
                          onTap: () => Navigator.of(contexto).pop(equipo),
                        );
                      },
                    ),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
            onPressed: () => Navigator.of(contexto).pop(), child: const Text('Cancelar')),
      ],
    );
  }
}
