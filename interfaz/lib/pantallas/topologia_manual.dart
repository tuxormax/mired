import 'package:flutter/material.dart';

import '../modelos/categorias.dart';
import '../modelos/modelos.dart';
import '../modelos/tipos_de_puerto.dart';
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
  final _ubicacion = TextEditingController();

  /// Arranca en el switch no administrable: es la razon numero uno por la que
  /// alguien abre este formulario, porque es lo unico que ningun escaneo ve.
  String _clave = 'switch_simple';
  int _puertos = 8;
  String _conexion = '';
  bool _guardando = false;

  @override
  void dispose() {
    _nombre.dispose();
    _modelo.dispose();
    _notas.dispose();
    _ip.dispose();
    _ubicacion.dispose();
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
        'ubicacion': _ubicacion.text.trim(),
        'ip': _ip.text.trim(),
        'conexion': _categoria.preguntaConexion ? _conexion : '',
        'puertos': _categoria.declaraPuertos ? _puertos : 0,
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
                // Donde ESTA, que no es lo mismo que de donde cuelga. En una
                // instalacion con rosetas es el dato por el que se busca:
                // «que hay en el consultorio 4».
                TextField(
                  controller: _ubicacion,
                  maxLength: 120, // lo mismo que acepta la columna
                  decoration: const InputDecoration(
                    labelText: 'Donde esta (opcional)',
                    hintText: 'Farmacia, consultorio 5, rack del site...',
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
                if (_categoria.declaraPuertos) ...[
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      const Text('Cuantos puertos fisicos tiene'),
                      const Spacer(),
                      IconButton(
                        icon: const Icon(Icons.remove_circle_outline),
                        onPressed: _puertos > 0 ? () => setState(() => _puertos--) : null,
                      ),
                      Text('$_puertos', style: Theme.of(contexto).textTheme.titleMedium),
                      IconButton(
                        icon: const Icon(Icons.add_circle_outline),
                        // 512 es el tope de la columna: el formulario no deja
                        // capturar lo que la base va a rechazar.
                        onPressed: _puertos < 512 ? () => setState(() => _puertos++) : null,
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
  late final TextEditingController _ubicacion;
  late String _conexion;
  bool _guardando = false;

  @override
  void initState() {
    super.initState();
    _modelo = TextEditingController(text: widget.equipo.modelo);
    _notas = TextEditingController(text: widget.equipo.notas);
    _ubicacion = TextEditingController(text: widget.equipo.ubicacion);
    _conexion = widget.equipo.conexion;
  }

  @override
  void dispose() {
    _modelo.dispose();
    _notas.dispose();
    _ubicacion.dispose();
    super.dispose();
  }

  Future<void> _guardar() async {
    setState(() => _guardando = true);
    try {
      await Api.instancia.guardarFicha(widget.clave, widget.equipo.id,
          modelo: _modelo.text.trim(),
          notas: _notas.text.trim(),
          ubicacion: _ubicacion.text.trim(),
          conexion: _conexion);
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
              // El mismo campo, con el mismo tope, que el alta a mano: si un
              // formulario acepta lo que el otro no, el dato acaba sucio.
              TextField(
                controller: _ubicacion,
                maxLength: 120,
                decoration: const InputDecoration(
                  labelText: 'Donde esta',
                  hintText: 'Farmacia, consultorio 5, rack del site...',
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

/// DialogoPuertos administra los puertos declarados de un equipo.
///
/// No es solo para switches tontos: un modem administrable sin SNMP hacia la LAN
/// —el caso mas comun de todos— necesita exactamente lo mismo.
class DialogoPuertos extends StatefulWidget {
  const DialogoPuertos({super.key, required this.clave, required this.equipo});

  final String clave;
  final Equipo equipo;

  @override
  State<DialogoPuertos> createState() => _DialogoPuertosState();
}

class _DialogoPuertosState extends State<DialogoPuertos> {
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
            .where((puerto) => puerto.tipo == 'lan')
            .fold<int>(0, (mayor, puerto) => puerto.numero > mayor ? puerto.numero : mayor) +
        1;

    final nueva = await showDialog<({int numero, String tipo, int? velocidad})>(
      context: context,
      builder: (_) => _DialogoPuerto(numero: siguiente),
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

  Future<void> _borrar(PuertoFisico puerto) async {
    try {
      await Api.instancia.borrarPuerto(widget.clave, puerto.id);
      _hubocambios = true;
      _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  @override
  Widget build(BuildContext contexto) => AlertDialog(
        title: Text('Puertos fisicos de ${widget.equipo.comoSeLlama}'),
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
                    child: const Text('No se pudieron cargar los puertos. Ver detalles'),
                  ),
                );
              }

              final topologia = resultado.data!;
              final puertos = topologia.puertosDe(widget.equipo.id);

              return Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(
                    'Cuente los puertos fisicos mirando el aparato. Lo que se declare aqui se '
                    'dibuja punteado en el mapa: es lo que usted sabe, no lo que se midio.',
                    style: Theme.of(contexto).textTheme.bodySmall,
                  ),
                  const SizedBox(height: 8),
                  Expanded(
                    child: puertos.isEmpty
                        ? const Center(child: Text('Todavia no hay puertos declarados.'))
                        : ListView.builder(
                            itemCount: puertos.length,
                            itemBuilder: (_, indice) {
                              final puerto = puertos[indice];
                              final cable = topologia.enlaceDe(puerto.id);
                              return ListTile(
                                dense: true,
                                leading: Icon(iconoDePuerto(puerto.tipo)),
                                title: Text(puerto.etiqueta),
                                subtitle: Text([
                                  if (puerto.velocidadMbps != null) '${puerto.velocidadMbps} Mbps',
                                  if (cable == null)
                                    'sin conectar'
                                  else
                                    'a ${cable.puertoOrigenId == puerto.id ? cable.destinoNombre : cable.origenNombre}',
                                ].join(' · ')),
                                trailing: IconButton(
                                  tooltip: 'Quitar el puerto',
                                  icon: const Icon(Icons.delete_outline),
                                  onPressed: () => _borrar(puerto),
                                ),
                              );
                            },
                          ),
                  ),
                  const SizedBox(height: 8),
                  OutlinedButton.icon(
                    icon: const Icon(Icons.add),
                    label: const Text('Agregar puerto'),
                    onPressed: () => _agregar(puertos),
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

class _DialogoPuerto extends StatefulWidget {
  const _DialogoPuerto({required this.numero});

  final int numero;

  @override
  State<_DialogoPuerto> createState() => _DialogoPuertoState();
}

class _DialogoPuertoState extends State<_DialogoPuerto> {
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
        title: const Text('Nuevo puerto fisico'),
        content: SizedBox(
          width: 360,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              DropdownButtonFormField<String>(
                initialValue: _tipo,
                decoration: const InputDecoration(
                    labelText: 'Tipo', border: OutlineInputBorder(), isDense: true),
                // El desplegable sale de la lista unica: nunca texto libre, y
                // nunca una lista escrita a mano aqui que se separe de la que
                // valida el servidor.
                items: [
                  for (final tipo in tiposDePuerto)
                    DropdownMenuItem(
                      value: tipo.clave,
                      child: Text('${tipo.nombre} — ${tipo.explicacion}'),
                    ),
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
                mensajeAviso(contexto, 'El numero de puerto tiene que estar entre 1 y 512.');
                return;
              }
              Navigator.of(contexto).pop((numero: numero, tipo: _tipo, velocidad: _velocidad));
            },
            child: const Text('Agregar'),
          ),
        ],
      );
}

/// DialogoElegirPuerto pregunta en QUE puerto del otro aparato entra el cable.
///
/// Un cable ocupa un puerto en las dos puntas. Sin esto, un switch de 5 puertos
/// colgado del modem seguia ofreciendo sus 5 puertos como libres, y el mapa
/// contaba un lugar que en la realidad ya tiene el cable puesto.
///
/// Devuelve el id del puerto elegido, o 0 si el usuario no sabe en cual entra:
/// eso conecta contra el aparato entero, que es menos preciso pero es la verdad
/// cuando nadie fue a mirar el cable.
class DialogoElegirPuerto extends StatelessWidget {
  const DialogoElegirPuerto({super.key, required this.equipo, required this.puertos});

  final Equipo equipo;
  final List<PuertoFisico> puertos;

  @override
  Widget build(BuildContext contexto) {
    return AlertDialog(
      title: Text('¿En que puerto de ${equipo.comoSeLlama}?'),
      content: SizedBox(
        width: 420,
        height: 320,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Padding(
              padding: EdgeInsets.only(bottom: 8),
              child: Text('El cable ocupa un puerto de este lado y otro del otro. '
                  'Solo se ofrecen los que estan libres.'),
            ),
            Expanded(
              child: ListView.builder(
                itemCount: puertos.length,
                itemBuilder: (_, indice) {
                  final puerto = puertos[indice];
                  return ListTile(
                    leading: Icon(iconoDePuerto(puerto.tipo)),
                    title: Text(puerto.etiqueta),
                    subtitle: puerto.velocidadMbps != null
                        ? Text('${puerto.velocidadMbps} Mbps')
                        : null,
                    onTap: () => Navigator.of(contexto).pop(puerto.id),
                  );
                },
              ),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(contexto).pop(0),
          child: const Text('No se en cual'),
        ),
        TextButton(
          onPressed: () => Navigator.of(contexto).pop(),
          child: const Text('Cancelar'),
        ),
      ],
    );
  }
}

/// DialogoElegirEquipo lista los equipos a los que se les puede tirar un cable
/// desde el puerto que se toco.
///
/// Se ofrecen los que no cuelgan de ningun sitio y los que tienen algun puerto
/// declarado libre —un switch con lugar—. Lo que un switch YA reporto no se
/// mueve desde aqui: eso no se corrige tecleando, se corrige moviendo el cable.
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
                          'No queda ningun equipo al que conectar. Los que ya cuelgan de un '
                          'puerto no se mueven desde aqui: eso se corrige moviendo el cable.',
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
                            color: equipo.presente
                                ? Theme.of(contexto).colorScheme.primary
                                : null,
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

/// DialogoColgarPorWiFi elige que equipos cuelgan de una antena.
///
/// **Con casillas, y de una sola vez.** El WiFi no tiene puertos: de una antena
/// cuelgan uno o veinte equipos sin que haya nada que declarar por cada uno.
/// Obligar a repetir el flujo aparato por aparato seria castigar a quien declara
/// bien su red.
///
/// Devuelve los identificadores elegidos, o null si se cancelo.
class DialogoColgarPorWiFi extends StatefulWidget {
  const DialogoColgarPorWiFi({super.key, required this.antena, required this.candidatos});

  final Equipo antena;
  final List<Equipo> candidatos;

  @override
  State<DialogoColgarPorWiFi> createState() => _DialogoColgarPorWiFiState();
}

class _DialogoColgarPorWiFiState extends State<DialogoColgarPorWiFi> {
  final Set<int> _elegidos = {};
  String _filtro = '';

  @override
  Widget build(BuildContext contexto) {
    final lista = widget.candidatos
        .where((equipo) =>
            equipo.id != widget.antena.id &&
            (_filtro.isEmpty ||
                '${equipo.comoSeLlama} ${equipo.ip} ${equipo.mac} ${equipo.fabricante}'
                    .toLowerCase()
                    .contains(_filtro)))
        .toList();

    return AlertDialog(
      title: Text('Colgar de ${widget.antena.comoSeLlama} por WiFi'),
      content: SizedBox(
        width: 520,
        height: 420,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Marque todos los que entran por esta antena. Queda declarado a mano: '
              'si algun dia la antena lo dice por su cuenta, ese dato manda y se ve '
              'que cambio.',
              style: Theme.of(contexto).textTheme.bodySmall,
            ),
            const SizedBox(height: 12),
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
                          'No queda ningun equipo suelto que colgar. Los que ya cuelgan de '
                          'un puerto o de otra antena no se mueven desde aqui.',
                          textAlign: TextAlign.center,
                        ),
                      ),
                    )
                  : ListView.builder(
                      itemCount: lista.length,
                      itemBuilder: (_, indice) {
                        final equipo = lista[indice];
                        return CheckboxListTile(
                          value: _elegidos.contains(equipo.id),
                          onChanged: (marcado) => setState(() {
                            if (marcado == true) {
                              _elegidos.add(equipo.id);
                            } else {
                              _elegidos.remove(equipo.id);
                            }
                          }),
                          title: Text(equipo.comoSeLlama),
                          subtitle: Text([
                            if (equipo.ip.isNotEmpty) equipo.ip,
                            if (equipo.fabricante.isNotEmpty) equipo.fabricante,
                            if (equipo.mac.isNotEmpty) equipo.mac,
                          ].join(' · ')),
                          dense: true,
                        );
                      },
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
          onPressed: _elegidos.isEmpty
              ? null
              : () => Navigator.of(contexto).pop(_elegidos.toList()),
          child: Text(_elegidos.length == 1
              ? 'Colgar 1 equipo'
              : 'Colgar ${_elegidos.length} equipos'),
        ),
      ],
    );
  }
}

/// DialogoCredencial guarda como se entra a un aparato.
///
/// La clave se escribe oculta y **dejarla en blanco no la borra**: significa "no
/// la toques". Para quitarla esta el boton de borrar la credencial entera.
class DialogoCredencial extends StatefulWidget {
  const DialogoCredencial({
    super.key,
    required this.clave,
    required this.equipo,
    this.credencial,
  });

  final String clave;
  final Equipo equipo;

  /// La que ya estaba, si se esta corrigiendo. Nunca trae la clave: eso solo
  /// viaja cuando alguien la pide expresamente.
  final CredencialEquipo? credencial;

  @override
  State<DialogoCredencial> createState() => _DialogoCredencialState();
}

class _DialogoCredencialState extends State<DialogoCredencial> {
  late TextEditingController _usuario;
  late TextEditingController _claveTexto;
  late TextEditingController _direccion;
  late TextEditingController _notas;
  late String _tipo;
  bool _oculta = true;
  bool _guardando = false;

  @override
  void initState() {
    super.initState();
    final anterior = widget.credencial;
    _tipo = anterior?.tipo ?? 'web';
    _usuario = TextEditingController(text: anterior?.usuario ?? '');
    _claveTexto = TextEditingController();
    _direccion = TextEditingController(
        text: anterior?.direccion ??
            (widget.equipo.ip.isEmpty ? '' : 'http://${widget.equipo.ip}'));
    _notas = TextEditingController(text: anterior?.notas ?? '');
  }

  @override
  void dispose() {
    _usuario.dispose();
    _claveTexto.dispose();
    _direccion.dispose();
    _notas.dispose();
    super.dispose();
  }

  Future<void> _guardar() async {
    setState(() => _guardando = true);
    try {
      await Api.instancia.guardarCredencial(widget.clave, widget.equipo.id, {
        'tipo': _tipo,
        'usuario': _usuario.text.trim(),
        'clave': _claveTexto.text,
        'direccion': _direccion.text.trim(),
        'notas': _notas.text.trim(),
      });
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
        title: Text('Credencial de ${widget.equipo.comoSeLlama}'),
        content: SizedBox(
          width: 460,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Queda guardada con el aparato, para no buscarla en un papel. La clave '
                  'se guarda cifrada y no se muestra sola: hay que pedirla, y el servidor '
                  'anota quien la pidio. Nunca sale en un mapa exportado.',
                  style: Theme.of(contexto).textTheme.bodySmall,
                ),
                const SizedBox(height: 16),
                DropdownButtonFormField<String>(
                  initialValue: _tipo,
                  decoration: const InputDecoration(
                      labelText: 'Para que sirve', border: OutlineInputBorder(), isDense: true),
                  items: const [
                    DropdownMenuItem(value: 'web', child: Text('Panel web')),
                    DropdownMenuItem(value: 'ssh', child: Text('Consola SSH')),
                    DropdownMenuItem(value: 'consola', child: Text('Consola')),
                    DropdownMenuItem(value: 'app', child: Text('Aplicacion del fabricante')),
                    DropdownMenuItem(value: 'otro', child: Text('Otro acceso')),
                  ],
                  onChanged: (valor) => setState(() => _tipo = valor ?? 'web'),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _usuario,
                  maxLength: 120,
                  decoration: const InputDecoration(
                    labelText: 'Usuario',
                    hintText: 'admin, ubnt, root...',
                    border: OutlineInputBorder(),
                    isDense: true,
                    counterText: '',
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _claveTexto,
                  obscureText: _oculta,
                  maxLength: 200,
                  decoration: InputDecoration(
                    labelText: 'Clave',
                    hintText: widget.credencial?.tieneClave == true
                        ? 'Dejela en blanco para no cambiarla'
                        : null,
                    border: const OutlineInputBorder(),
                    isDense: true,
                    counterText: '',
                    suffixIcon: IconButton(
                      icon: Icon(_oculta ? Icons.visibility_outlined : Icons.visibility_off_outlined),
                      onPressed: () => setState(() => _oculta = !_oculta),
                    ),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _direccion,
                  maxLength: 200,
                  decoration: const InputDecoration(
                    labelText: 'Donde se entra',
                    hintText: 'http://192.168.1.254, ssh://...',
                    border: OutlineInputBorder(),
                    isDense: true,
                    counterText: '',
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _notas,
                  maxLines: 2,
                  maxLength: 500,
                  decoration: const InputDecoration(
                    labelText: 'Notas',
                    hintText: 'La del panel de invitados, la cambio Juan en mayo...',
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
            onPressed: _guardando ? null : () => Navigator.of(contexto).pop(false),
            child: const Text('Cancelar'),
          ),
          FilledButton(
            onPressed: _guardando ? null : _guardar,
            child: const Text('Guardar'),
          ),
        ],
      );
}
