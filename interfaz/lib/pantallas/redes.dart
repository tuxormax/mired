import 'package:flutter/material.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../servicios/trayectoria.dart';
import '../widgets/mensajes.dart';
import 'credenciales.dart';
import 'entrar.dart';
import 'red.dart';
import 'usuarios.dart';

/// PantallaRedes es el panel de inicio: una tarjeta por sitio.
///
/// Se arma con el resumen que el catalogo guarda de cada red, no abriendo el
/// archivo de cada una. Con treinta redes, esa diferencia es la que separa una
/// pantalla instantanea de una que tarda segundos.
class PantallaRedes extends StatefulWidget {
  const PantallaRedes({super.key});

  @override
  State<PantallaRedes> createState() => _PantallaRedesState();
}

class _PantallaRedesState extends State<PantallaRedes> {
  late Future<List<Red>> _redes;
  Map<String, dynamic>? _sonda;

  @override
  void initState() {
    super.initState();
    _recargar();
    _consultarSonda();
  }

  void _recargar() {
    setState(() => _redes = Api.instancia.listarRedes());
  }

  Future<void> _consultarSonda() async {
    try {
      final estado = await Api.instancia.estadoSonda();
      if (mounted) setState(() => _sonda = estado);
    } catch (_) {
      // Que no se pueda consultar la sonda no debe estorbar el panel.
    }
  }

  Future<void> _salir() async {
    await Api.instancia.salir();
    if (!mounted) return;
    Navigator.of(context).pushReplacement(
      MaterialPageRoute<void>(builder: (_) => const PantallaEntrar()),
    );
  }

  Future<void> _crearRed() async {
    final creada = await showDialog<bool>(
      context: context,
      builder: (_) => const _DialogoNuevaRed(),
    );
    if (creada == true) _recargar();
  }

  @override
  Widget build(BuildContext contexto) {
    final usuario = Api.instancia.usuario;

    return Scaffold(
      appBar: AppBar(
        title: const Text('MiRed'),
        actions: [
          if (usuario?.superadmin == true)
            IconButton(
              tooltip: 'Credenciales SNMP',
              icon: const Icon(Icons.vpn_key_outlined),
              onPressed: () => Navigator.of(contexto).push(
                MaterialPageRoute<void>(builder: (_) => const PantallaCredenciales()),
              ),
            ),
          if (usuario?.superadmin == true)
            IconButton(
              tooltip: 'Usuarios',
              icon: const Icon(Icons.people_outline),
              onPressed: () => Navigator.of(contexto).push(
                MaterialPageRoute<void>(builder: (_) => const PantallaUsuarios()),
              ),
            ),
          IconButton(
            tooltip: 'Actualizar',
            icon: const Icon(Icons.refresh),
            onPressed: () {
              _recargar();
              _consultarSonda();
            },
          ),
          PopupMenuButton<String>(
            tooltip: usuario?.nombre ?? '',
            icon: const Icon(Icons.account_circle_outlined),
            onSelected: (opcion) {
              if (opcion == 'salir') _salir();
            },
            itemBuilder: (_) => [
              PopupMenuItem(
                enabled: false,
                child: Text(usuario?.nombre ?? '', style: const TextStyle(fontWeight: FontWeight.bold)),
              ),
              const PopupMenuDivider(),
              const PopupMenuItem(value: 'salir', child: Text('Cerrar sesion')),
            ],
          ),
        ],
      ),
      floatingActionButton: usuario?.superadmin == true
          ? FloatingActionButton.extended(
              onPressed: _crearRed,
              icon: const Icon(Icons.add),
              label: const Text('Nueva red'),
            )
          : null,
      body: Column(
        children: [
          _AvisoSonda(estado: _sonda),
          Expanded(
            child: FutureBuilder<List<Red>>(
              future: _redes,
              builder: (contextoLista, resultado) {
                if (resultado.connectionState != ConnectionState.done) {
                  return const Center(child: CircularProgressIndicator());
                }
                if (resultado.hasError) {
                  return _Problema(
                    problema: resultado.error!,
                    reintentar: _recargar,
                  );
                }

                final redes = resultado.data ?? [];
                if (redes.isEmpty) {
                  return const _Vacio();
                }

                return LayoutBuilder(
                  builder: (_, medidas) {
                    final columnas = (medidas.maxWidth / 340).floor().clamp(1, 4);
                    return GridView.builder(
                      padding: const EdgeInsets.all(16),
                      gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
                        crossAxisCount: columnas,
                        childAspectRatio: 1.9,
                        crossAxisSpacing: 12,
                        mainAxisSpacing: 12,
                      ),
                      itemCount: redes.length,
                      itemBuilder: (_, indice) => _TarjetaRed(
                        red: redes[indice],
                        alAbrir: () async {
                          Trayectoria.instancia.anotar('Abrir red ${redes[indice].nombre}');
                          await Navigator.of(contexto).push(MaterialPageRoute<void>(
                            builder: (_) => PantallaRed(red: redes[indice]),
                          ));
                          _recargar();
                        },
                      ),
                    );
                  },
                );
              },
            ),
          ),
          _PieVersion(),
        ],
      ),
    );
  }
}

class _TarjetaRed extends StatelessWidget {
  const _TarjetaRed({required this.red, required this.alAbrir});

  final Red red;
  final VoidCallback alAbrir;

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;

    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: alAbrir,
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(Icons.hub_outlined, color: colores.primary),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      red.nombre,
                      style: Theme.of(contexto).textTheme.titleMedium,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  if (red.alertasAbiertas > 0)
                    Badge(label: Text('${red.alertasAbiertas}'), backgroundColor: colores.error),
                ],
              ),
              if (red.descripcion.isNotEmpty) ...[
                const SizedBox(height: 4),
                Text(red.descripcion,
                    style: Theme.of(contexto).textTheme.bodySmall,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis),
              ],
              const Spacer(),
              Row(
                children: [
                  _Dato(etiqueta: 'Equipos', valor: '${red.equipos}'),
                  const SizedBox(width: 20),
                  _Dato(etiqueta: 'Presentes', valor: '${red.equiposPresentes}'),
                ],
              ),
              const SizedBox(height: 6),
              Text(
                red.ultimoEscaneo == null
                    ? 'Sin escanear todavia'
                    : 'Ultimo escaneo: ${red.ultimoEscaneo}',
                style: Theme.of(contexto).textTheme.labelSmall,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _Dato extends StatelessWidget {
  const _Dato({required this.etiqueta, required this.valor});

  final String etiqueta;
  final String valor;

  @override
  Widget build(BuildContext contexto) => Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(valor, style: Theme.of(contexto).textTheme.titleLarge),
          Text(etiqueta, style: Theme.of(contexto).textTheme.labelSmall),
        ],
      );
}

/// _AvisoSonda dice sin rodeos si la sonda no esta disponible. Es la regla del
/// proyecto: antes que una pantalla vacia sin explicacion, una linea que diga
/// que falta.
class _AvisoSonda extends StatelessWidget {
  const _AvisoSonda({required this.estado});

  final Map<String, dynamic>? estado;

  @override
  Widget build(BuildContext contexto) {
    if (estado == null) return const SizedBox.shrink();

    final viva = estado!['viva'] == true;
    String? aviso;
    if (!viva) {
      aviso = estado!['motivo'] as String? ?? 'La sonda de escaneo no responde.';
    } else {
      final detalle = estado!['estado'] as Map<String, dynamic>?;
      final advertencia = detalle?['advertencia'] as String?;
      if (advertencia != null && advertencia.isNotEmpty) aviso = advertencia;
    }
    if (aviso == null) return const SizedBox.shrink();

    final colores = Theme.of(contexto).colorScheme;
    return Container(
      width: double.infinity,
      color: colores.errorContainer,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      child: Row(
        children: [
          Icon(Icons.warning_amber_outlined, color: colores.onErrorContainer, size: 20),
          const SizedBox(width: 8),
          Expanded(child: Text(aviso, style: TextStyle(color: colores.onErrorContainer))),
        ],
      ),
    );
  }
}

class _Vacio extends StatelessWidget {
  const _Vacio();

  @override
  Widget build(BuildContext contexto) => Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.hub_outlined, size: 56),
            const SizedBox(height: 12),
            Text('Todavia no hay ninguna red', style: Theme.of(contexto).textTheme.titleMedium),
            const SizedBox(height: 4),
            const Text('Cree una por cada sitio: Matriz, Sucursal 1, Bodega...'),
          ],
        ),
      );
}

class _Problema extends StatelessWidget {
  const _Problema({required this.problema, required this.reintentar});

  final Object problema;
  final VoidCallback reintentar;

  @override
  Widget build(BuildContext contexto) => Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.cloud_off, size: 48),
            const SizedBox(height: 12),
            Text(problema is ErrorMiRed
                ? (problema as ErrorMiRed).mensaje
                : 'No se pudieron cargar las redes.'),
            const SizedBox(height: 12),
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                FilledButton(onPressed: reintentar, child: const Text('Reintentar')),
                const SizedBox(width: 8),
                TextButton(
                  onPressed: () => mostrarProblema(contexto, problema),
                  child: const Text('Ver detalles'),
                ),
              ],
            ),
          ],
        ),
      );
}

class _PieVersion extends StatelessWidget {
  @override
  Widget build(BuildContext contexto) => Padding(
        padding: const EdgeInsets.all(8),
        child: Text(
          Api.instancia.version,
          style: Theme.of(contexto).textTheme.labelSmall,
        ),
      );
}

class _DialogoNuevaRed extends StatefulWidget {
  const _DialogoNuevaRed();

  @override
  State<_DialogoNuevaRed> createState() => _DialogoNuevaRedState();
}

class _DialogoNuevaRedState extends State<_DialogoNuevaRed> {
  final _formulario = GlobalKey<FormState>();
  final _nombre = TextEditingController();
  final _descripcion = TextEditingController();
  final _subredes = TextEditingController();
  bool _ocupado = false;

  @override
  void dispose() {
    _nombre.dispose();
    _descripcion.dispose();
    _subredes.dispose();
    super.dispose();
  }

  Future<void> _guardar() async {
    if (!_formulario.currentState!.validate()) return;
    setState(() => _ocupado = true);
    Trayectoria.instancia.anotar('Crear red ${_nombre.text}');

    try {
      final rangos = _subredes.text
          .split(RegExp(r'[\n,]'))
          .map((texto) => texto.trim())
          .where((texto) => texto.isNotEmpty)
          .toList();
      await Api.instancia.crearRed(_nombre.text.trim(), _descripcion.text.trim(), rangos);
      if (mounted) Navigator.of(context).pop(true);
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _ocupado = false);
    }
  }

  @override
  Widget build(BuildContext contexto) => AlertDialog(
        title: const Text('Nueva red'),
        content: SizedBox(
          width: 460,
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
                    labelText: 'Nombre del sitio',
                    hintText: 'Matriz, Sucursal 1, Bodega...',
                    border: OutlineInputBorder(),
                    counterText: '',
                  ),
                  validator: (valor) =>
                      (valor == null || valor.trim().isEmpty) ? 'Capture el nombre' : null,
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _descripcion,
                  maxLength: 200,
                  decoration: const InputDecoration(
                    labelText: 'Descripcion (opcional)',
                    border: OutlineInputBorder(),
                    counterText: '',
                  ),
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _subredes,
                  minLines: 2,
                  maxLines: 4,
                  decoration: const InputDecoration(
                    labelText: 'Subredes a escanear',
                    hintText: '192.168.1.0/24\n10.0.0.0/16',
                    helperText: 'Una por linea. Se pueden agregar despues.',
                    border: OutlineInputBorder(),
                  ),
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
                : const Text('Crear'),
          ),
        ],
      );
}
