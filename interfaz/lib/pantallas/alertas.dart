import 'package:flutter/material.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../servicios/trayectoria.dart';
import '../widgets/mensajes.dart';

/// PantallaAlertas muestra lo que cambio en la red y a donde se avisa.
///
/// El valor de MiRed no es escanear: es enterarse. Esta pantalla es donde eso
/// se ve.
class PantallaAlertas extends StatefulWidget {
  const PantallaAlertas({super.key, required this.red});

  final Red red;

  @override
  State<PantallaAlertas> createState() => _PantallaAlertasState();
}

class _PantallaAlertasState extends State<PantallaAlertas> {
  late Future<({List<Alerta> alertas, int abiertas})> _alertas;
  late Future<List<Regla>> _reglas;
  late Future<List<DestinoAlerta>> _destinos;
  bool _soloSinVer = true;

  @override
  void initState() {
    super.initState();
    _recargar();
  }

  void _recargar() {
    setState(() {
      _alertas = Api.instancia.listarAlertas(widget.red.clave, soloSinVer: _soloSinVer);
      _reglas = Api.instancia.listarReglas(widget.red.clave);
      _destinos = Api.instancia.listarDestinos(widget.red.clave);
    });
  }

  Future<void> _marcarTodas() async {
    try {
      Trayectoria.instancia.anotar('Marcar alertas vistas en ${widget.red.nombre}');
      await Api.instancia.marcarAlertasVistas(widget.red.clave);
      _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  /// _marcarUna despacha una sola alerta.
  ///
  /// Marcarlas TODAS de golpe es comodo cuando ya se revisaron, pero obliga a
  /// elegir entre despachar el monton entero o no despachar nada. Lo normal es
  /// atender una, dejar las demas para despues, y que el contador refleje eso.
  Future<void> _marcarUna(Alerta alerta) async {
    try {
      Trayectoria.instancia.anotar('Marcar leida la alerta ${alerta.id}');
      await Api.instancia.marcarAlertasVistas(widget.red.clave, ids: [alerta.id]);
      _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  Future<void> _cambiarRegla(Regla regla, {bool? activa, int? umbral}) async {
    final nueva = Regla(
      tipo: regla.tipo,
      activa: activa ?? regla.activa,
      umbral: umbral ?? regla.umbral,
    );
    try {
      await Api.instancia.guardarRegla(widget.red.clave, nueva);
      _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  Future<void> _agregarDestino() async {
    final creado = await showDialog<bool>(
      context: context,
      builder: (_) => _DialogoDestino(clave: widget.red.clave),
    );
    if (creado == true) _recargar();
  }

  Future<void> _borrarDestino(DestinoAlerta destino) async {
    try {
      await Api.instancia.borrarDestino(widget.red.clave, destino.id);
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
          title: Text('Alertas de ${widget.red.nombre}'),
          bottom: const TabBar(tabs: [
            Tab(icon: Icon(Icons.notifications_active_outlined), text: 'Avisos'),
            Tab(icon: Icon(Icons.rule), text: 'Que se vigila'),
            Tab(icon: Icon(Icons.send_outlined), text: 'A donde avisar'),
          ]),
          actions: [
            IconButton(
              tooltip: 'Actualizar',
              icon: const Icon(Icons.refresh),
              onPressed: _recargar,
            ),
          ],
        ),
        body: TabBarView(
          children: [
            _pestanaAvisos(contexto),
            _pestanaReglas(contexto),
            _pestanaDestinos(contexto),
          ],
        ),
      ),
    );
  }

  Widget _pestanaAvisos(BuildContext contexto) {
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
          child: Row(
            children: [
              FilterChip(
                label: const Text('Solo sin ver'),
                selected: _soloSinVer,
                onSelected: (valor) {
                  setState(() => _soloSinVer = valor);
                  _recargar();
                },
              ),
              const SizedBox(width: 12),
              // Cuantas quedan sin ver, siempre a la vista.
              //
              // Es la cuenta que manda el servidor, NO los renglones de esta
              // lista: con "solo sin ver" apagado la lista trae tambien las ya
              // despachadas, y contar renglones diria que hay veinte nuevas
              // cuando no queda ninguna.
              FutureBuilder<({List<Alerta> alertas, int abiertas})>(
                future: _alertas,
                builder: (_, resultado) {
                  final sinVer = resultado.data?.abiertas ?? 0;
                  if (!resultado.hasData) return const SizedBox.shrink();
                  return Chip(
                    avatar: Icon(
                      sinVer == 0 ? Icons.check_circle_outline : Icons.notifications_active_outlined,
                      size: 18,
                    ),
                    label: Text(switch (sinVer) {
                      0 => 'Ninguna sin ver',
                      1 => '1 sin ver',
                      _ => '$sinVer sin ver',
                    }),
                    visualDensity: VisualDensity.compact,
                  );
                },
              ),
              const Spacer(),
              TextButton.icon(
                icon: const Icon(Icons.done_all),
                label: const Text('Ya las vi todas'),
                onPressed: _marcarTodas,
              ),
            ],
          ),
        ),
        Expanded(
          child: FutureBuilder<({List<Alerta> alertas, int abiertas})>(
            future: _alertas,
            builder: (_, resultado) {
              if (resultado.connectionState != ConnectionState.done) {
                return const Center(child: CircularProgressIndicator());
              }
              if (resultado.hasError) {
                return Center(
                  child: TextButton(
                    onPressed: () => mostrarProblema(contexto, resultado.error!),
                    child: const Text('No se pudieron cargar las alertas. Ver detalles'),
                  ),
                );
              }

              final alertas = resultado.data!.alertas;
              if (alertas.isEmpty) {
                return const Center(
                  child: Padding(
                    padding: EdgeInsets.all(32),
                    child: Text('Nada que reportar: la red esta como la dejo.',
                        textAlign: TextAlign.center),
                  ),
                );
              }

              return ListView.separated(
                padding: const EdgeInsets.all(16),
                itemCount: alertas.length,
                separatorBuilder: (_, __) => const Divider(height: 1),
                itemBuilder: (_, indice) {
                  final alerta = alertas[indice];
                  return ListTile(
                    leading: Icon(_iconoDe(alerta.tipo), color: _colorDe(contexto, alerta.tipo)),
                    title: Text(alerta.titulo,
                        style: TextStyle(
                            fontWeight: alerta.vista ? FontWeight.normal : FontWeight.w600)),
                    subtitle: Text([alerta.detalle, alerta.momento].join(' · ')),
                    // Antes aqui habia un cartelito que solo decia "Nueva".
                    // Repetia lo que ya se veia en la negrita del titulo y no
                    // dejaba hacer nada con la alerta: para quitarla habia que
                    // despacharlas todas de golpe.
                    trailing: alerta.vista
                        ? null
                        : TextButton.icon(
                            icon: const Icon(Icons.done, size: 18),
                            label: const Text('Marcar leida'),
                            onPressed: () => _marcarUna(alerta),
                          ),
                  );
                },
              );
            },
          ),
        ),
      ],
    );
  }

  Widget _pestanaReglas(BuildContext contexto) {
    return FutureBuilder<List<Regla>>(
      future: _reglas,
      builder: (_, resultado) {
        if (resultado.connectionState != ConnectionState.done) {
          return const Center(child: CircularProgressIndicator());
        }
        if (resultado.hasError) {
          return Center(
            child: TextButton(
              onPressed: () => mostrarProblema(contexto, resultado.error!),
              child: const Text('No se pudieron cargar las reglas. Ver detalles'),
            ),
          );
        }

        final reglas = resultado.data ?? [];
        return ListView(
          padding: const EdgeInsets.all(16),
          children: [
            for (final regla in reglas)
              Card(
                child: Column(
                  children: [
                    SwitchListTile(
                      value: regla.activa,
                      onChanged: (valor) => _cambiarRegla(regla, activa: valor),
                      title: Text(regla.comoSeLlama),
                    ),
                    if (regla.usaUmbral && regla.activa)
                      Padding(
                        padding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
                        child: Row(
                          children: [
                            const Text('Avisar despues de'),
                            const SizedBox(width: 12),
                            SizedBox(
                              width: 110,
                              child: TextFormField(
                                initialValue: '${regla.umbral}',
                                keyboardType: TextInputType.number,
                                decoration: const InputDecoration(
                                  isDense: true,
                                  border: OutlineInputBorder(),
                                  suffixText: 'min',
                                ),
                                onFieldSubmitted: (valor) {
                                  final numero = int.tryParse(valor.trim());
                                  if (numero != null && numero >= 0) {
                                    _cambiarRegla(regla, umbral: numero);
                                  }
                                },
                              ),
                            ),
                            const SizedBox(width: 12),
                            Text('(${_enPalabras(regla.umbral)})',
                                style: Theme.of(contexto).textTheme.labelSmall),
                          ],
                        ),
                      ),
                  ],
                ),
              ),
          ],
        );
      },
    );
  }

  Widget _pestanaDestinos(BuildContext contexto) {
    return FutureBuilder<List<DestinoAlerta>>(
      future: _destinos,
      builder: (_, resultado) {
        if (resultado.connectionState != ConnectionState.done) {
          return const Center(child: CircularProgressIndicator());
        }
        if (resultado.hasError) {
          return Center(
            child: TextButton(
              onPressed: () => mostrarProblema(contexto, resultado.error!),
              child: const Text('No se pudieron cargar los destinos. Ver detalles'),
            ),
          );
        }

        final destinos = resultado.data ?? [];
        return ListView(
          padding: const EdgeInsets.all(16),
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
                        'Sin destinos las alertas se ven aqui y nada mas. Con uno configurado, '
                        'MiRed avisa al telefono, a un chat o al correo en cuanto pasa algo.',
                        style: Theme.of(contexto).textTheme.bodyMedium,
                      ),
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 12),
            if (destinos.isEmpty)
              const Padding(
                padding: EdgeInsets.all(24),
                child: Text('Todavia no hay a donde avisar.', textAlign: TextAlign.center),
              )
            else
              Card(
                margin: EdgeInsets.zero,
                child: Column(
                  children: [
                    for (final destino in destinos)
                      ListTile(
                        leading: Icon(_iconoDestino(destino.tipo)),
                        title: Text(destino.nombre),
                        subtitle: Text([
                          destino.tipo,
                          destino.destino,
                          if (destino.ultimoError.isNotEmpty)
                            'ultimo intento fallo: ${destino.ultimoError}',
                        ].join(' · ')),
                        trailing: IconButton(
                          tooltip: 'Borrar',
                          icon: const Icon(Icons.delete_outline),
                          onPressed: () => _borrarDestino(destino),
                        ),
                      ),
                  ],
                ),
              ),
            const SizedBox(height: 16),
            Align(
              alignment: Alignment.centerLeft,
              child: FilledButton.icon(
                icon: const Icon(Icons.add),
                label: const Text('Agregar destino'),
                onPressed: _agregarDestino,
              ),
            ),
          ],
        );
      },
    );
  }

  static String _enPalabras(int minutos) {
    if (minutos < 60) return '$minutos minutos';
    if (minutos < 1440) return '${(minutos / 60).round()} horas';
    return '${(minutos / 1440).round()} dias';
  }

  static IconData _iconoDe(String tipo) {
    switch (tipo) {
      case 'equipo_nuevo':
        return Icons.new_releases_outlined;
      case 'equipo_ausente':
        return Icons.power_off;
      case 'puerto_nuevo':
        return Icons.lock_open;
      case 'cambio_ip':
        return Icons.swap_horiz;
      default:
        return Icons.notifications;
    }
  }

  static Color? _colorDe(BuildContext contexto, String tipo) {
    final colores = Theme.of(contexto).colorScheme;
    switch (tipo) {
      case 'equipo_nuevo':
      case 'puerto_nuevo':
        return colores.error;
      default:
        return colores.primary;
    }
  }

  static IconData _iconoDestino(String tipo) {
    switch (tipo) {
      case 'correo':
        return Icons.mail_outline;
      case 'telegram':
        return Icons.send;
      case 'ntfy':
        return Icons.phone_android;
      default:
        return Icons.webhook;
    }
  }
}

class _DialogoDestino extends StatefulWidget {
  const _DialogoDestino({required this.clave});

  final String clave;

  @override
  State<_DialogoDestino> createState() => _DialogoDestinoState();
}

class _DialogoDestinoState extends State<_DialogoDestino> {
  final _formulario = GlobalKey<FormState>();
  final _nombre = TextEditingController();
  final _destino = TextEditingController();
  final _extra = TextEditingController();
  String _tipo = 'ntfy';
  bool _ocupado = false;

  @override
  void dispose() {
    _nombre.dispose();
    _destino.dispose();
    _extra.dispose();
    super.dispose();
  }

  String get _pista {
    switch (_tipo) {
      case 'ntfy':
        return 'https://ntfy.sh/mi-red-casa';
      case 'telegram':
        return 'Identificador del chat (por ejemplo 123456789)';
      case 'correo':
        return 'a-quien@ejemplo.mx';
      default:
        return 'https://mi-servidor/aviso';
    }
  }

  String get _pistaExtra {
    switch (_tipo) {
      case 'ntfy':
        return 'Token, solo si el servidor lo pide';
      case 'telegram':
        return 'Token del bot (obligatorio)';
      case 'correo':
        return 'Ajustes SMTP en JSON: {"servidor":"","puerto":587,"usuario":"","clave":"","de":""}';
      default:
        return 'No hace falta';
    }
  }

  Future<void> _guardar() async {
    if (!_formulario.currentState!.validate()) return;
    setState(() => _ocupado = true);
    Trayectoria.instancia.anotar('Agregar destino de alertas ${_nombre.text}');

    try {
      await Api.instancia.crearDestino(widget.clave, {
        'nombre': _nombre.text.trim(),
        'tipo': _tipo,
        'destino': _destino.text.trim(),
        'extra': _extra.text.trim(),
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
        title: const Text('A donde avisar'),
        content: SizedBox(
          width: 480,
          child: Form(
            key: _formulario,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextFormField(
                  controller: _nombre,
                  autofocus: true,
                  maxLength: 80,
                  decoration: const InputDecoration(
                    labelText: 'Nombre',
                    hintText: 'Mi telefono, Grupo de soporte...',
                    border: OutlineInputBorder(),
                    counterText: '',
                  ),
                  validator: (valor) =>
                      (valor == null || valor.trim().isEmpty) ? 'Capture el nombre' : null,
                ),
                const SizedBox(height: 12),
                DropdownButtonFormField<String>(
                  initialValue: _tipo,
                  decoration: const InputDecoration(
                    labelText: 'Por donde',
                    border: OutlineInputBorder(),
                  ),
                  items: const [
                    DropdownMenuItem(value: 'ntfy', child: Text('ntfy (al telefono)')),
                    DropdownMenuItem(value: 'telegram', child: Text('Telegram')),
                    DropdownMenuItem(value: 'correo', child: Text('Correo')),
                    DropdownMenuItem(value: 'webhook', child: Text('Webhook')),
                  ],
                  onChanged: (valor) => setState(() => _tipo = valor ?? 'ntfy'),
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _destino,
                  decoration: InputDecoration(
                    labelText: 'Destino',
                    hintText: _pista,
                    border: const OutlineInputBorder(),
                  ),
                  validator: (valor) =>
                      (valor == null || valor.trim().isEmpty) ? 'Capture el destino' : null,
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _extra,
                  maxLines: _tipo == 'correo' ? 3 : 1,
                  decoration: InputDecoration(
                    labelText: 'Ajustes',
                    helperText: _pistaExtra,
                    helperMaxLines: 3,
                    border: const OutlineInputBorder(),
                  ),
                  // Telegram sin token no puede mandar nada: mejor decirlo aqui
                  // que dejar un destino que falla en silencio.
                  validator: (valor) =>
                      (_tipo == 'telegram' && (valor == null || valor.trim().isEmpty))
                          ? 'Telegram necesita el token del bot'
                          : null,
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
                : const Text('Guardar'),
          ),
        ],
      );
}
