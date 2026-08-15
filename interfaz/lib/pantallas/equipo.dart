import 'package:flutter/material.dart';

import '../modelos/modelos.dart';
import '../modelos/tipos_de_puerto.dart';
import '../servicios/api.dart';
import '../widgets/mensajes.dart';
import 'propiedades_equipo.dart';
import 'topologia_manual.dart';

/// DialogoEquipo es la ficha de un aparato: lo que se sabe de el y lo que tiene
/// conectado.
///
/// Tocar un aparato —en la lista o en el mapa— abre ESTO. No un menu de
/// acciones: en el mapa, un clic es para mirar, y quitar un cable por accidente
/// mientras se consulta la red es justo lo que no puede pasar.
///
/// Los enlaces se administran aqui dentro, en la pestana **Conexiones**, donde
/// se ve de un vistazo que cuelga de este aparato y **por que medio**: por cable
/// —con su puerto— o por WiFi.
class DialogoEquipo extends StatefulWidget {
  const DialogoEquipo({
    super.key,
    required this.clave,
    required this.equipo,
    required this.topologia,
    required this.equipos,
    this.enConexiones = false,
  });

  final String clave;
  final Equipo equipo;
  final TopologiaManual topologia;

  /// Todos los equipos de la red: hacen falta para ofrecer que colgar.
  final List<Equipo> equipos;

  /// Abre directamente en Conexiones. Lo usa el mapa: quien toca un aparato ahi
  /// casi siempre viene a mirar como esta conectado.
  final bool enConexiones;

  @override
  State<DialogoEquipo> createState() => _DialogoEquipoState();
}

class _DialogoEquipoState extends State<DialogoEquipo> {
  /// Se avisa al cerrar si algo cambio, para que quien abrio recargue.
  bool _huboCambios = false;

  void _cambio() => setState(() => _huboCambios = true);

  @override
  Widget build(BuildContext contexto) {
    // La pestana de credenciales solo aparece si el aparato se puede
    // administrar: un switch no administrable no tiene panel al que entrar, y
    // ofrecerle un formulario de usuario y clave seria ofrecer algo que no
    // existe.
    final conCredenciales = widget.equipo.puedeAdministrarse;

    return DefaultTabController(
      length: conCredenciales ? 3 : 2,
      initialIndex: widget.enConexiones ? 1 : 0,
      child: AlertDialog(
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(widget.equipo.comoSeLlama),
            if (widget.equipo.ip.isNotEmpty)
              Text(widget.equipo.ip,
                  style: Theme.of(contexto).textTheme.bodyMedium?.copyWith(
                      fontFamily: 'monospace',
                      color: Theme.of(contexto).colorScheme.onSurfaceVariant)),
          ],
        ),
        content: SizedBox(
          width: 620,
          height: 520,
          child: Column(
            children: [
              TabBar(tabs: [
                const Tab(icon: Icon(Icons.info_outline), text: 'Propiedades'),
                const Tab(icon: Icon(Icons.hub_outlined), text: 'Conexiones'),
                if (conCredenciales)
                  const Tab(icon: Icon(Icons.key_outlined), text: 'Credenciales'),
              ]),
              Expanded(
                child: TabBarView(
                  children: [
                    SingleChildScrollView(
                      padding: const EdgeInsets.only(top: 12),
                      child: PropiedadesDelEquipo(
                          clave: widget.clave, equipo: widget.equipo, alCambiar: _cambio),
                    ),
                    _Conexiones(
                      clave: widget.clave,
                      equipo: widget.equipo,
                      topologia: widget.topologia,
                      equipos: widget.equipos,
                      alCambiar: _cambio,
                    ),
                    if (conCredenciales)
                      CredencialesDelEquipo(
                          clave: widget.clave, equipo: widget.equipo, alCambiar: _cambio),
                  ],
                ),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(contexto).pop(_huboCambios),
            child: const Text('Cerrar'),
          ),
        ],
      ),
    );
  }
}

/// _Conexiones dice que cuelga de este aparato y por que medio.
///
/// Las dos mitades no son lo mismo y por eso van separadas:
///
///   * **Por cable**: cada puerto fisico con lo que lleva. Un puerto lleva un
///     cable, y por eso se ve puerto por puerto.
///   * **Por WiFi**: el aire no tiene puertos, asi que aqui hay una lista sin
///     numeros, a la que se le agregan uno o VARIOS equipos de una vez.
class _Conexiones extends StatefulWidget {
  const _Conexiones({
    required this.clave,
    required this.equipo,
    required this.topologia,
    required this.equipos,
    required this.alCambiar,
  });

  final String clave;
  final Equipo equipo;
  final TopologiaManual topologia;
  final List<Equipo> equipos;
  final VoidCallback alCambiar;

  @override
  State<_Conexiones> createState() => _ConexionesState();
}

class _ConexionesState extends State<_Conexiones> {
  late TopologiaManual _topologia;
  bool _ocupado = false;

  @override
  void initState() {
    super.initState();
    _topologia = widget.topologia;
  }

  Future<void> _recargar() async {
    setState(() => _ocupado = true);
    try {
      final topologia = await Api.instancia.topologiaManual(widget.clave);
      if (mounted) setState(() => _topologia = topologia);
      widget.alCambiar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _ocupado = false);
    }
  }

  Equipo? _equipoPorId(int? id) {
    if (id == null) return null;
    for (final equipo in widget.equipos) {
      if (equipo.id == id) return equipo;
    }
    return null;
  }

  /// Los que se pueden colgar: los que no cuelgan de ningun sitio todavia.
  List<Equipo> get _sueltos {
    final ubicados = <int>{};
    for (final enlace in _topologia.enlaces) {
      if (enlace.equipoDestinoId != null) ubicados.add(enlace.equipoDestinoId!);
      if (enlace.puertoDestinoId != null) {
        final dueno = _topologia.equipoDelPuerto(enlace.puertoDestinoId!);
        if (dueno != null) ubicados.add(dueno);
      }
    }
    for (final enlace in _topologia.inalambricos) {
      ubicados.add(enlace.equipoId);
    }
    return widget.equipos
        .where((equipo) => equipo.id != widget.equipo.id && !ubicados.contains(equipo.id))
        .toList();
  }

  Future<void> _colgarPorWiFi() async {
    final elegidos = await showDialog<List<int>>(
      context: context,
      builder: (_) => DialogoColgarPorWiFi(antena: widget.equipo, candidatos: _sueltos),
    );
    if (elegidos == null || elegidos.isEmpty) return;

    try {
      final resultado = await Api.instancia.colgarPorWiFi(widget.clave,
          antenaId: widget.equipo.id, equipos: elegidos);
      final rechazados = (resultado['rechazados'] as List<dynamic>? ?? []);
      if (mounted && rechazados.isNotEmpty) {
        mensajeAviso(context, 'No se pudieron colgar: ${rechazados.join('; ')}');
      }
      await _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  Future<void> _descolgar(EnlaceInalambrico enlace) async {
    final quitar = await _confirmar(
      'Descolgar del WiFi',
      '${enlace.equipoNombre} deja de colgar de ${widget.equipo.comoSeLlama}. '
          'El equipo no se borra: vuelve a quedar suelto.',
      'Descolgar',
    );
    if (quitar != true) return;

    try {
      await Api.instancia.descolgarDeWiFi(widget.clave, enlace.id);
      await _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  Future<void> _quitarCable(EnlaceFisico cable) async {
    final quitar = await _confirmar(
      'Quitar el cable',
      'Se borra solo lo que se declaro a mano. El puerto queda libre y el equipo '
          'vuelve a quedar suelto.',
      'Quitar',
    );
    if (quitar != true) return;

    try {
      await Api.instancia.desconectar(widget.clave, cable.id);
      await _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  Future<void> _conectarPuerto(PuertoFisico puerto) async {
    final destino = await showDialog<Equipo>(
      context: context,
      builder: (_) => DialogoElegirEquipo(candidatos: _sueltos),
    );
    if (destino == null || !mounted) return;

    // Si el otro extremo tiene puertos declarados, hay que decir en cual entra:
    // un cable ocupa un puerto en las DOS puntas.
    int? puertoDestinoId;
    final libres = _topologia.puertosLibresDe(destino.id);
    if (libres.isNotEmpty) {
      final elegido = await showDialog<int>(
        context: context,
        builder: (_) => DialogoElegirPuerto(equipo: destino, puertos: libres),
      );
      if (elegido == null) return;
      if (elegido > 0) puertoDestinoId = elegido;
    }

    try {
      await Api.instancia.conectar(widget.clave,
          puertoOrigenId: puerto.id,
          puertoDestinoId: puertoDestinoId,
          equipoDestinoId: puertoDestinoId == null ? destino.id : null);
      await _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  Future<void> _declararPuertos() async {
    final cambio = await showDialog<bool>(
      context: context,
      builder: (_) => DialogoPuertos(clave: widget.clave, equipo: widget.equipo),
    );
    if (cambio == true) await _recargar();
  }

  Future<bool?> _confirmar(String titulo, String texto, String boton) => showDialog<bool>(
        context: context,
        builder: (contextoModal) => AlertDialog(
          title: Text(titulo),
          content: Text(texto),
          actions: [
            TextButton(
                onPressed: () => Navigator.of(contextoModal).pop(false),
                child: const Text('Cancelar')),
            FilledButton(
                onPressed: () => Navigator.of(contextoModal).pop(true),
                child: Text(boton)),
          ],
        ),
      );

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;
    final puertos = _topologia.puertosDe(widget.equipo.id);
    final porAire = _topologia.inalambricosDe(widget.equipo.id);
    final deQuienCuelga = _topologia.antenaDe(widget.equipo.id);

    return Stack(
      children: [
        ListView(
          padding: const EdgeInsets.symmetric(vertical: 12),
          children: [
            // De donde cuelga ESTE aparato. Se dice primero porque es lo que
            // orienta: antes de ver que le cuelga, hay que saber de que cuelga.
            if (deQuienCuelga != null)
              Card(
                margin: const EdgeInsets.only(bottom: 12),
                child: ListTile(
                  leading: const Icon(Icons.wifi),
                  title: Text('Cuelga de ${deQuienCuelga.antenaNombre}'),
                  subtitle: Text([
                    'por WiFi',
                    if (deQuienCuelga.red.isNotEmpty) deQuienCuelga.red,
                    deQuienCuelga.comoSeSupo,
                  ].join(' · ')),
                  trailing: IconButton(
                    tooltip: 'Descolgar',
                    icon: const Icon(Icons.link_off),
                    onPressed: () async {
                      final quitar = await _confirmar('Descolgar del WiFi',
                          '${widget.equipo.comoSeLlama} deja de colgar de '
                              '${deQuienCuelga.antenaNombre}.',
                          'Descolgar');
                      if (quitar != true) return;
                      try {
                        await Api.instancia.descolgarDeWiFi(widget.clave, deQuienCuelga.id);
                        await _recargar();
                      } catch (problema, pila) {
                        if (mounted) {
                          await mostrarProblema(context, problema, pila: pila.toString());
                        }
                      }
                    },
                  ),
                ),
              ),

            // ------------------------------------------------ por cable --
            Row(
              children: [
                const Icon(Icons.settings_ethernet, size: 18),
                const SizedBox(width: 8),
                Text('Por cable', style: Theme.of(contexto).textTheme.titleSmall),
                const Spacer(),
                TextButton.icon(
                  icon: const Icon(Icons.edit_outlined, size: 16),
                  label: Text(puertos.isEmpty ? 'Declarar puertos' : 'Editar puertos'),
                  onPressed: _declararPuertos,
                ),
              ],
            ),
            if (puertos.isEmpty)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 8),
                child: Text(
                  'Este aparato no tiene puertos declarados. Cuentelos mirandolo y '
                  'declarelos para poder decir que entra por cada uno.',
                  style: Theme.of(contexto).textTheme.bodySmall?.copyWith(color: colores.outline),
                ),
              ),
            for (final puerto in puertos) _renglonDePuerto(contexto, puerto),

            const SizedBox(height: 16),

            // ------------------------------------------------- por WiFi --
            Row(
              children: [
                const Icon(Icons.wifi, size: 18),
                const SizedBox(width: 8),
                Text('Por WiFi', style: Theme.of(contexto).textTheme.titleSmall),
                const Spacer(),
                TextButton.icon(
                  icon: const Icon(Icons.add, size: 16),
                  label: const Text('Colgar equipos'),
                  onPressed: _colgarPorWiFi,
                ),
              ],
            ),
            Padding(
              padding: const EdgeInsets.only(bottom: 4),
              child: Text(
                'El WiFi no tiene puertos: se cuelgan uno o varios equipos de una vez.',
                style: Theme.of(contexto).textTheme.bodySmall?.copyWith(color: colores.outline),
              ),
            ),
            if (porAire.isEmpty)
              Text('Nadie cuelga de aqui por el aire.',
                  style: Theme.of(contexto).textTheme.bodySmall?.copyWith(color: colores.outline)),
            for (final enlace in porAire)
              ListTile(
                dense: true,
                leading: const Icon(Icons.smartphone),
                title: Text(enlace.equipoNombre),
                subtitle: Text([
                  'por WiFi',
                  if (enlace.red.isNotEmpty) enlace.red,
                  if (enlace.senalDbm != null) '${enlace.senalDbm} dBm',
                  // De donde salio el dato: no es lo mismo que lo declarara una
                  // persona a que lo dijera la propia antena.
                  enlace.comoSeSupo,
                ].join(' · ')),
                trailing: IconButton(
                  tooltip: 'Descolgar',
                  icon: const Icon(Icons.link_off, size: 18),
                  onPressed: () => _descolgar(enlace),
                ),
              ),
          ],
        ),
        if (_ocupado)
          const Positioned(
            top: 0, right: 0,
            child: Padding(
              padding: EdgeInsets.all(8),
              child: SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2)),
            ),
          ),
      ],
    );
  }

  Widget _renglonDePuerto(BuildContext contexto, PuertoFisico puerto) {
    final cable = _topologia.enlaceDe(puerto.id);
    if (cable == null) {
      return ListTile(
        dense: true,
        leading: const Icon(Icons.add_circle_outline),
        title: Text(puerto.etiqueta),
        subtitle: const Text('libre'),
        trailing: TextButton(
          onPressed: () => _conectarPuerto(puerto),
          child: const Text('Conectar'),
        ),
      );
    }

    // El otro extremo, mirado desde ESTE puerto: un cable declarado desde la
    // otra punta apunta hacia aca.
    final soyOrigen = cable.puertoOrigenId == puerto.id;
    final otroId = soyOrigen ? cable.equipoDestinoId : cable.equipoOrigenId;
    final otro = _equipoPorId(otroId);
    final nombre = otro?.comoSeLlama ??
        (soyOrigen ? cable.destinoNombre : cable.origenNombre);

    return ListTile(
      dense: true,
      leading: Icon(iconoDePuerto(puerto.tipo)),
      title: Text('${puerto.etiqueta} → ${nombre.isEmpty ? 'conectado' : nombre}'),
      subtitle: Text([
        'por cable',
        if (cable.numeroDestino > 0 && soyOrigen) 'a su puerto ${cable.numeroDestino}',
        if (cable.origenDato == 'manual') 'declarado a mano' else cable.origenDato,
      ].join(' · ')),
      trailing: IconButton(
        tooltip: 'Quitar el cable',
        icon: const Icon(Icons.link_off, size: 18),
        onPressed: () => _quitarCable(cable),
      ),
    );
  }
}
