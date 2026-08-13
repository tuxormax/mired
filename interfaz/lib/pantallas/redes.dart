import 'package:flutter/material.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../servicios/descarga.dart';
import '../servicios/subredes.dart' as subredes;
import '../servicios/trayectoria.dart';
import '../widgets/mensajes.dart';
import 'controladoras.dart';
import 'credenciales.dart';
import 'entrar.dart';
import 'red.dart';
import 'usuarios.dart';
import 'versiones.dart';

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
    // El cuerpo va entre llaves a proposito: con flecha, la expresion devuelve
    // el Future y setState revienta en compilacion de depuracion.
    setState(() {
      _redes = Api.instancia.listarRedes();
    });
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
              tooltip: 'Controladoras WiFi',
              icon: const Icon(Icons.wifi_tethering),
              onPressed: () => Navigator.of(contexto).push(
                MaterialPageRoute<void>(builder: (_) => const PantallaControladoras()),
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

/// _PieVersion muestra la version y el enlace al codigo fuente.
///
/// El enlace no es cortesia: MiRed es AGPL-3.0 y se usa por red, asi que la
/// seccion 13 de la licencia obliga a ofrecerle el codigo a quien lo use desde
/// el navegador. El pie de la interfaz es donde eso se cumple.
class _PieVersion extends StatelessWidget {
  static const codigo = 'https://github.com/tuxormax/mired';

  @override
  Widget build(BuildContext contexto) {
    final estilo = Theme.of(contexto).textTheme.labelSmall;
    return Padding(
      padding: const EdgeInsets.all(8),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          // El pie es clickable, como en el resto de los proyectos de la casa:
          // abre que cambio en cada entrega.
          InkWell(
            onTap: () => showDialog<void>(
              context: contexto,
              builder: (_) => const DialogoVersiones(),
            ),
            child: Text(
              Api.instancia.version,
              style: estilo?.copyWith(
                decoration: TextDecoration.underline,
                color: Theme.of(contexto).colorScheme.primary,
              ),
            ),
          ),
          Text('  ·  AGPL-3.0  ·  ', style: estilo),
          InkWell(
            onTap: () => abrirEnlace(codigo),
            child: Text(
              'Codigo fuente',
              style: estilo?.copyWith(
                decoration: TextDecoration.underline,
                color: Theme.of(contexto).colorScheme.primary,
              ),
            ),
          ),
        ],
      ),
    );
  }
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
  final _acotado = TextEditingController();
  bool _ocupado = false;

  /// La red que MiRed detecto en este equipo, si la detecto.
  ///
  /// **Es la respuesta al 95% de los casos**, y el programa ya la sabe: se la
  /// pregunta a la sonda en vez de pedirle al usuario que la calcule.
  subredes.Interpretacion? _miRed;
  bool _buscando = true;

  /// Si el usuario quiere acotar en vez de escanear su red entera.
  bool _acotar = false;

  @override
  void initState() {
    super.initState();
    _buscarMiRed();
    _acotado.addListener(() => setState(() {}));
  }

  /// _buscarMiRed le pregunta a la sonda que tarjetas de red ve.
  ///
  /// La sonda ya reportaba esto —se usa para avisar si le faltan permisos—, solo
  /// que nadie lo estaba aprovechando para lo mas util: rellenar el formulario.
  Future<void> _buscarMiRed() async {
    try {
      final estado = await Api.instancia.estadoSonda();
      final detalle = estado['estado'] as Map<String, dynamic>?;
      for (final cruda in (detalle?['interfaces'] as List<dynamic>?) ?? []) {
        final interfaz = cruda as Map<String, dynamic>;
        if (interfaz['activa'] != true) continue;
        for (final direccion in (interfaz['direcciones'] as List<dynamic>?) ?? []) {
          final leida = subredes.deLaRedDetectada(direccion as String);
          if (leida != null) {
            if (mounted) setState(() => _miRed = leida);
            return;
          }
        }
      }
    } catch (_) {
      // Que no se pueda preguntar no rompe nada: se cae al campo de siempre.
    } finally {
      // Si no se detecto nada, la unica opcion util es escribirlo: se deja ya
      // elegida en vez de dejar el formulario sin nada marcado.
      if (mounted) {
        setState(() {
          _buscando = false;
          if (_miRed == null) _acotar = true;
        });
      }
    }
  }

  /// _loQueSeVaAEscanear es lo unico que decide que se manda al servidor.
  subredes.Interpretacion? get _loQueSeVaAEscanear =>
      _acotar ? subredes.interpretar(_acotado.text) : _miRed;

  @override
  void dispose() {
    _nombre.dispose();
    _descripcion.dispose();
    _acotado.dispose();
    super.dispose();
  }

  Future<void> _guardar() async {
    if (!_formulario.currentState!.validate()) return;
    setState(() => _ocupado = true);
    Trayectoria.instancia.anotar('Crear red ${_nombre.text}');

    try {
      final queEscanear = _loQueSeVaAEscanear;
      await Api.instancia.crearRed(_nombre.text.trim(), _descripcion.text.trim(),
          queEscanear?.subredes ?? const []);
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
                const SizedBox(height: 16),
                _QueEscanear(
                  miRed: _miRed,
                  buscando: _buscando,
                  acotar: _acotar,
                  campo: _acotado,
                  interpretado: _loQueSeVaAEscanear,
                  alCambiarAcotar: (valor) => setState(() => _acotar = valor),
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
            onPressed: _ocupado || _loQueSeVaAEscanear?.seEntendio != true
                ? null
                : _guardar,
            child: _ocupado
                ? const SizedBox(height: 16, width: 16, child: CircularProgressIndicator(strokeWidth: 2))
                : const Text('Crear'),
          ),
        ],
      );
}

/// _QueEscanear pregunta que hay que revisar, sin pedir que nadie sepa de redes.
///
/// # Por que quedo asi
///
/// Antes este sitio pedia "subredes a escanear" y esperaba `192.168.1.0/24`.
/// Quien no conoce esa notacion no tiene forma de adivinarla — y **el programa ya
/// sabe la respuesta**, porque la sonda le reporta las tarjetas del equipo.
/// Pedirle al usuario una cuenta que el programa puede hacer solo es trabajo
/// regalado.
///
/// Asi que lo normal es no escribir nada: sale la red detectada, ya elegida, y
/// se le da a Crear. El que quiera acotar abre la otra opcion, y ahi vale
/// cualquiera de las formas en que la gente lo dice de verdad — incluida "de
/// aqui hasta aca", que es la que sale sola cuando alguien piensa en un rango.
class _QueEscanear extends StatelessWidget {
  const _QueEscanear({
    required this.miRed,
    required this.buscando,
    required this.acotar,
    required this.campo,
    required this.interpretado,
    required this.alCambiarAcotar,
  });

  final subredes.Interpretacion? miRed;
  final bool buscando;
  final bool acotar;
  final TextEditingController campo;
  final subredes.Interpretacion? interpretado;
  final ValueChanged<bool> alCambiarAcotar;

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;

    if (buscando) {
      return const Padding(
        padding: EdgeInsets.symmetric(vertical: 12),
        child: Row(
          children: [
            SizedBox(height: 16, width: 16, child: CircularProgressIndicator(strokeWidth: 2)),
            SizedBox(width: 10),
            Text('Buscando su red...'),
          ],
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('Que revisar', style: Theme.of(contexto).textTheme.titleSmall),
        const SizedBox(height: 4),

        RadioGroup<bool>(
          groupValue: acotar,
          onChanged: (valor) => alCambiarAcotar(valor ?? false),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (miRed != null)
                RadioListTile<bool>(
                  value: false,
                  contentPadding: EdgeInsets.zero,
                  title: const Text('Toda mi red'),
                  subtitle: Text(miRed!.explicacion),
                ),
              const RadioListTile<bool>(
                value: true,
                contentPadding: EdgeInsets.zero,
                title: Text('Solo una parte, u otra red'),
              ),
            ],
          ),
        ),

        if (miRed == null)
          // Sin sonda no hay nada que proponer. Se dice, en vez de dejar una
          // opcion vacia que no se entiende por que no funciona.
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 8),
            child: Text(
              'No se pudo detectar su red. Escriba abajo que revisar.',
              style: Theme.of(contexto).textTheme.bodySmall
                  ?.copyWith(color: colores.error),
            ),
          ),

        if (acotar) ...[
          TextFormField(
            controller: campo,
            autofocus: true,
            maxLength: 200,
            decoration: const InputDecoration(
              labelText: 'De donde a donde',
              hintText: '192.168.1.10 - 192.168.1.50',
              helperText: 'Tambien vale una direccion suelta, o 192.168.1.0/24',
              border: OutlineInputBorder(),
              counterText: '',
            ),
          ),
          const SizedBox(height: 8),
          // Se dice lo que se entendio ANTES de crear nada. Sin esto, aceptar
          // varias formas de escribirlo seria magia: el usuario pone una cosa,
          // el programa entiende otra y nadie se entera hasta que el escaneo
          // sale raro.
          if (campo.text.trim().isNotEmpty)
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(
                  interpretado?.seEntendio == true
                      ? Icons.check_circle_outline
                      : Icons.error_outline,
                  size: 18,
                  color: interpretado?.seEntendio == true ? colores.primary : colores.error,
                ),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    interpretado?.seEntendio == true
                        ? interpretado!.explicacion
                        : 'No se entiende. Pruebe con algo como '
                            '192.168.1.10 - 192.168.1.50',
                    style: Theme.of(contexto).textTheme.bodySmall?.copyWith(
                        color: interpretado?.seEntendio == true ? null : colores.error),
                  ),
                ),
              ],
            ),
        ],
      ],
    );
  }
}
