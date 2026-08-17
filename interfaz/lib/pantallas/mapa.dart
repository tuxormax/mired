import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../servicios/descarga.dart';
import '../servicios/exportar_mapa.dart';
import '../servicios/frescura.dart';
import '../servicios/hoja_calculo.dart';
import '../widgets/mensajes.dart';
import 'equipo.dart';
import 'mapa_plano.dart';
import 'topologia_manual.dart';

/// PantallaMapa dibuja la red: los switches arriba y lo que cuelga de cada puerto
/// debajo.
///
/// Dos reglas de dibujo que son el corazon del proyecto:
///  - Lo **confirmado** (una sola MAC en el puerto) va con linea llena.
///  - Lo **inferido** (varias MAC en el mismo puerto, o sea un switch tonto atras)
///    va con linea punteada y agrupado. Dibujarlo igual que lo confirmado seria
///    mentir sobre lo que se sabe.
///  - Lo que no cuelga de ningun switch conocido NO se esconde: va aparte, en su
///    propia columna, dicho con todas sus letras.
///
/// El calculo de posiciones y el pintor viven en mapa_plano.dart, porque los
/// comparte con la exportacion.
class PantallaMapa extends StatefulWidget {
  const PantallaMapa({
    super.key,
    required this.red,
    this.editarAlAbrir = false,
    this.pausarAgendaAlEditar = true,
  });

  final Red red;

  /// Abre ya en modo edicion. Lo usa el boton «Editar el cableado» de la pestana
  /// de puertos: quien lo pulsa ya dijo a que viene, y obligarle a buscar otro
  /// boton al llegar seria pedirle que lo diga dos veces.
  final bool editarAlAbrir;

  /// Al editar, pedirle al servidor que pare los barridos automaticos de esta
  /// red. Se puede apagar para dibujar el mapa sin hablar con nadie, que es lo
  /// que hacen las pruebas: ahi no hay agenda que parar.
  final bool pausarAgendaAlEditar;

  @override
  State<PantallaMapa> createState() => _PantallaMapaState();
}

enum _Formato { png, svg, pdf, ods, xlsx, csv, portapapeles }

class _PantallaMapaState extends State<PantallaMapa> {
  late Future<DatosMapa> _datos;
  bool _exportando = false;

  /// El modo edicion es explicito a proposito.
  ///
  /// Sin el, un clic para mirar un equipo reescribiria la topologia por
  /// accidente, y el mapa es justo lo que se consulta cuando algo no funciona:
  /// el peor momento para cambiarlo sin querer.
  bool _editando = false;

  /// Mientras se edita, la agenda de esta red queda en pausa. Se apunta para
  /// poder reanudarla al salir, y se renueva con un reloj mientras dure.
  bool _pausada = false;
  Timer? _relojDePausa;

  @override
  void initState() {
    super.initState();
    _editando = widget.editarAlAbrir;
    _datos = _cargar();
    if (_editando && widget.pausarAgendaAlEditar) _pausarMientrasSeEdita();
  }

  @override
  void dispose() {
    // Al salir del mapa se reanuda SIEMPRE, aunque se haya cerrado a lo bruto.
    _relojDePausa?.cancel();
    if (_pausada) Api.instancia.reanudarAgenda(widget.red.clave).ignore();
    super.dispose();
  }

  /// Mientras se edita el mapa, MiRed no barre esta red.
  ///
  /// Editar es DECLARAR cableado, no medir: un barrido corriendo por debajo solo
  /// gasta el equipo de quien esta trabajando. Se renueva sola mientras dure la
  /// edicion, y vence si el programa se cierra de golpe.
  Future<void> _pausarMientrasSeEdita() async {
    if (!widget.pausarAgendaAlEditar) return;
    _relojDePausa?.cancel();
    try {
      await Api.instancia.pausarAgenda(widget.red.clave);
      _pausada = true;
    } catch (_) {
      // Que no se pueda pausar no impide editar: es una cortesia con el equipo,
      // no un requisito.
      return;
    }
    _relojDePausa = Timer.periodic(const Duration(minutes: 10), (_) {
      Api.instancia.pausarAgenda(widget.red.clave).ignore();
    });
  }

  Future<void> _reanudar() async {
    _relojDePausa?.cancel();
    _relojDePausa = null;
    if (!_pausada) return;
    _pausada = false;
    try {
      await Api.instancia.reanudarAgenda(widget.red.clave);
    } catch (_) {
      // Si no se pudo avisar, la pausa vence sola.
    }
  }

  Future<DatosMapa> _cargar() async {
    final mapa = await Api.instancia.mapaDePuertos(widget.red.clave);
    final equipos = await Api.instancia.listarEquipos(widget.red.clave);
    final topologia = await Api.instancia.topologiaManual(widget.red.clave);
    return DatosMapa(mapa: mapa, equipos: equipos, topologia: topologia);
  }

  void _recargar() => setState(() => _datos = _cargar());

  /// _alTocar resuelve un clic sobre el plano en modo edicion.
  ///
  /// Se eligio clic-clic y no arrastre libre: arrastrar equipos sobre un lienzo
  /// exige deteccion de colisiones en el pintor para un beneficio marginal.
  /// Tocar un puerto y elegir del menu resuelve lo mismo y se entiende solo.
  Future<void> _alTocar(Offset donde, Plano plano, DatosMapa datos) async {
    // De la ultima a la primera: las cajas se pintan en orden, asi que la de
    // encima es la ultima que se dibujo.
    for (final caja in plano.cajas.reversed) {
      if (!caja.rectangulo.contains(donde)) continue;

      if (caja.puertoLibre && caja.puertoFisicoId != null) {
        await _conectarPuerto(caja.puertoFisicoId!, datos);
        return;
      }
      // Tocar un aparato abre SU FICHA. Los enlaces —el cable que lo trae, lo
      // que le cuelga por WiFi— se administran ahi dentro, en la pestana de
      // conexiones.
      final equipo = datos.equipoPorId(caja.equipoId);
      if (equipo != null) {
        await _abrirFicha(equipo, datos);
        return;
      }
      return;
    }
  }

  Future<void> _conectarPuerto(int puertoId, DatosMapa datos) async {
    final eleccion = await showModalBottomSheet<String>(
      context: context,
      builder: (contextoHoja) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.add_box_outlined),
              title: const Text('Agregar un aparato nuevo'),
              subtitle: const Text('Un switch, un modem, algo que no sale en el escaneo'),
              onTap: () => Navigator.of(contextoHoja).pop('nuevo'),
            ),
            ListTile(
              leading: const Icon(Icons.link),
              title: const Text('Conectar uno que ya se descubrio'),
              subtitle: const Text('De los que todavia no estan en ningun puerto'),
              onTap: () => Navigator.of(contextoHoja).pop('existente'),
            ),
          ],
        ),
      ),
    );
    if (eleccion == null || !mounted) return;

    Equipo? destino;
    if (eleccion == 'nuevo') {
      destino = await showDialog<Equipo>(
        context: context,
        builder: (_) => DialogoEquipoManual(clave: widget.red.clave),
      );
    } else {
      destino = await showDialog<Equipo>(
        context: context,
        builder: (_) => DialogoElegirEquipo(candidatos: datos.conectablesDesde(puertoId)),
      );
    }
    if (destino == null || !mounted) return;

    // Si el otro extremo tiene puertos declarados, hay que decir en CUAL entra
    // el cable. Un cable ocupa un puerto en las dos puntas: sin esto, un switch
    // de 5 puertos colgado del modem seguia diciendo que tenia 5 libres.
    int? puertoDestinoId;
    final libres = datos.topologia.puertosLibresDe(destino.id);
    if (libres.isNotEmpty) {
      final elegido = await showDialog<int>(
        context: context,
        builder: (_) => DialogoElegirPuerto(equipo: destino!, puertos: libres),
      );
      if (elegido == null) return;
      // 0 es "no se en cual": se conecta contra el aparato, como antes.
      if (elegido > 0) puertoDestinoId = elegido;
    }

    try {
      await Api.instancia.conectar(widget.red.clave,
          puertoOrigenId: puertoId,
          puertoDestinoId: puertoDestinoId,
          equipoDestinoId: puertoDestinoId == null ? destino.id : null);
      _recargar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }


  /// _abrirFicha muestra las propiedades del aparato y sus conexiones.
  ///
  /// **Un clic en el mapa es para MIRAR.** Antes salia un menu de acciones, y
  /// con el, quitar un cable por accidente mientras se consulta la red estaba a
  /// un toque de distancia. Los enlaces se administran dentro de la ficha, en la
  /// pestana de conexiones, donde ademas se ve por que medio entra cada uno.
  Future<void> _abrirFicha(Equipo equipo, DatosMapa datos) async {
    final cambio = await showDialog<bool>(
      context: context,
      builder: (_) => DialogoEquipo(
        clave: widget.red.clave,
        equipo: equipo,
        topologia: datos.topologia,
        equipos: datos.equipos,
        // Quien toca un aparato en el mapa casi siempre viene a ver como esta
        // conectado.
        enConexiones: _editando,
      ),
    );
    if (cambio == true) _recargar();
  }


  Future<void> _agregarAparato() async {
    final creado = await showDialog<Equipo>(
      context: context,
      builder: (_) => DialogoEquipoManual(clave: widget.red.clave),
    );
    if (creado != null) _recargar();
  }

  /// _exportar arma el archivo y pregunta al usuario donde guardarlo.
  ///
  /// El sitio lo elige quien exporta, en el cuadro de guardar del escritorio: un
  /// mapa suele ir a la carpeta del cliente o del sitio, no a la de descargas.
  ///
  /// El plano de la exportacion se arma con los colores fijos de exportar, no
  /// con los del tema: lo que se guarda o se imprime va siempre sobre blanco,
  /// aunque quien lo exporto tenga la pantalla en oscuro.
  Future<void> _exportar(_Formato formato, DatosMapa datos) async {
    setState(() => _exportando = true);
    try {
      // Las contrasenas de los aparatos van DENTRO de lo exportado. Es lo que
      // convierte la hoja en un respaldo de verdad y en algo que se puede
      // entregar o mudar a otro equipo; sin ellas, la instalacion queda
      // documentada a medias. Van en claro, y se sabe: el archivo hay que
      // tratarlo como se trata un llavero.
      //
      // Si el servidor no las da —permisos, o una llave que no descifra— la
      // exportacion NO se cae: sale sin ellas. Un mapa sin claves sigue siendo
      // util; no tener mapa, no.
      var credenciales = const <CredencialEquipo>[];
      if (formato != _Formato.png && formato != _Formato.svg &&
          formato != _Formato.pdf) {
        try {
          credenciales =
              await Api.instancia.credencialesParaExportar(widget.red.clave);
        } catch (_) {
          // Sin claves, pero con mapa.
        }
      }

      final momento = DateTime.now();
      final sello = momento.toIso8601String().substring(0, 16).replaceAll(':', '');
      final base = 'mapa-${widget.red.clave}-$sello';

      // Las DOS fechas, y la de los datos primero: un mapa exportado hoy con
      // datos de hace tres semanas no es un mapa de hoy, y quien lo reciba tiene
      // que poder saberlo sin preguntar. Va en los CUATRO formatos, CSV incluido.
      // La frase se arma entera, no pegando un "del" delante de lo que salga:
      // sin escaneo previo quedaba "Datos del sin escanear todavia".
      final deCuandoSonLosDatos = datos.mapa.momento.isEmpty
          ? 'Todavia sin escanear'
          : 'Datos del ${_enPalabras(datos.mapa.momento)}';
      final encabezado = EncabezadoMapa(
        titulo: 'Mapa de ${widget.red.nombre}',
        subtitulo: '$deCuandoSonLosDatos'
            '  ·  Exportado el '
            '${momento.toIso8601String().substring(0, 19).replaceFirst('T', ' ')}'
            '  ·  ${Api.instancia.version}',
      );

      if (formato == _Formato.portapapeles) {
        await Clipboard.setData(
            ClipboardData(text: csvDelMapa(datos, encabezado, credenciales)));
        if (mounted) {
          mensajeAviso(context,
              'Mapa copiado como CSV: ya se puede pegar en una hoja de calculo.');
        }
        return;
      }

      var donde = '';
      switch (formato) {
        case _Formato.png:
          donde = await descargarArchivo('$base.png', 'image/png',
              await pngDelPlano(_planoParaExportar(datos), encabezado));
        case _Formato.svg:
          donde = await descargarArchivo(
              '$base.svg',
              'image/svg+xml',
              Uint8List.fromList(
                  utf8.encode(svgDelPlano(_planoParaExportar(datos), encabezado))));
        case _Formato.pdf:
          donde = await descargarArchivo('$base.pdf', 'application/pdf',
              pdfDelPlano(_planoParaExportar(datos), encabezado));
        case _Formato.ods:
          donde = await descargarArchivo(
              '$base.ods',
              'application/vnd.oasis.opendocument.spreadsheet',
              odsDelMapa(datos, encabezado, momento, credenciales: credenciales));
        case _Formato.xlsx:
          donde = await descargarArchivo(
              '$base.xlsx',
              'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
              xlsxDelMapa(datos, encabezado, momento, credenciales: credenciales));
        case _Formato.csv:
          // Con marca de codificacion: sin ella Excel abre el archivo con la
          // codificacion local del equipo y los acentos salen rotos.
          donde = await descargarArchivo('$base.csv', 'text/csv;charset=utf-8',
              csvEnBytes(csvDelMapa(datos, encabezado, credenciales)));
        case _Formato.portapapeles:
          break; // Resuelto arriba.
      }

      // Se dice DONDE quedo aunque lo haya elegido el usuario: el cuadro pudo
      // haberse abierto en otra carpeta de la que creia. Vacio significa que
      // cerro el cuadro sin guardar, y cancelar no se avisa: ya lo sabe.
      if (donde.isNotEmpty && mounted) {
        mensajeAviso(context, 'Guardado en $donde');
      }
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _exportando = false);
    }
  }

  /// _planoParaExportar arma el dibujo con los colores fijos de exportar.
  ///
  /// Solo lo necesitan los tres formatos que dibujan. Las hojas de calculo no
  /// llevan posiciones, y calcularselas seria trabajo tirado.
  Plano _planoParaExportar(DatosMapa datos) =>
      armarPlano(datos, coloresParaExportar);

  /// _enPalabras deja una fecha ISO legible, sin la T ni la zona horaria.
  static String _enPalabras(String iso) {
    final momento = DateTime.tryParse(iso);
    if (momento == null) return iso;
    return momento.toIso8601String().substring(0, 19).replaceFirst('T', ' ');
  }

  @override
  Widget build(BuildContext contexto) {
    return Scaffold(
      appBar: AppBar(
        title: Text('Mapa de ${widget.red.nombre}'),
        actions: [
          FutureBuilder<DatosMapa>(
            future: _datos,
            builder: (_, resultado) => PopupMenuButton<_Formato>(
              tooltip: 'Exportar el mapa',
              enabled: resultado.hasData && !_exportando,
              icon: _exportando
                  ? const SizedBox(
                      height: 18, width: 18,
                      child: CircularProgressIndicator(strokeWidth: 2))
                  : const Icon(Icons.download_outlined),
              onSelected: (formato) => _exportar(formato, resultado.data!),
              itemBuilder: (_) => const [
                PopupMenuItem(
                  value: _Formato.png,
                  child: ListTile(
                    dense: true,
                    leading: Icon(Icons.image_outlined),
                    title: Text('PNG'),
                    subtitle: Text('Imagen, para pegar en un documento'),
                  ),
                ),
                PopupMenuItem(
                  value: _Formato.svg,
                  child: ListTile(
                    dense: true,
                    leading: Icon(Icons.polyline_outlined),
                    title: Text('SVG'),
                    subtitle: Text('Vectorial, para retocar el plano'),
                  ),
                ),
                PopupMenuItem(
                  value: _Formato.pdf,
                  child: ListTile(
                    dense: true,
                    leading: Icon(Icons.picture_as_pdf_outlined),
                    title: Text('PDF'),
                    subtitle: Text('Para imprimir o mandar por correo'),
                  ),
                ),
                PopupMenuDivider(),
                PopupMenuItem(
                  value: _Formato.ods,
                  child: ListTile(
                    dense: true,
                    leading: Icon(Icons.table_chart_outlined),
                    title: Text('ODS'),
                    subtitle: Text('LibreOffice Calc: aparatos y conexiones, '
                        'en dos hojas'),
                  ),
                ),
                PopupMenuItem(
                  value: _Formato.xlsx,
                  child: ListTile(
                    dense: true,
                    leading: Icon(Icons.grid_on_outlined),
                    title: Text('XLSX'),
                    subtitle: Text('Excel: aparatos y conexiones, en dos hojas'),
                  ),
                ),
                PopupMenuItem(
                  value: _Formato.csv,
                  child: ListTile(
                    dense: true,
                    leading: Icon(Icons.table_view_outlined),
                    title: Text('CSV'),
                    subtitle: Text('Texto plano: las dos tablas, una tras otra'),
                  ),
                ),
                PopupMenuItem(
                  value: _Formato.portapapeles,
                  child: ListTile(
                    dense: true,
                    leading: Icon(Icons.copy_outlined),
                    title: Text('Copiar CSV'),
                    subtitle: Text('Al portapapeles, sin bajar archivo'),
                  ),
                ),
              ],
            ),
          ),
          // El modo edicion se enciende a proposito: un clic de navegacion no
          // debe poder reescribir la topologia.
          //
          // Va CON SU NOMBRE, no como un lapiz suelto. Era un icono mas entre
          // otros dos y la primera persona que uso el programa no lo encontro:
          // un icono solo se reconoce cuando ya sabes que estas buscando.
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 4),
            child: FilledButton.tonalIcon(
              icon: Icon(_editando ? Icons.done : Icons.edit_outlined, size: 18),
              label: Text(_editando ? 'Terminar' : 'Editar el cableado'),
              onPressed: () {
                setState(() => _editando = !_editando);
                // Entrar a editar para la agenda de esta red; salir la reanuda.
                if (_editando) {
                  _pausarMientrasSeEdita();
                } else {
                  _reanudar();
                }
              },
            ),
          ),
          IconButton(
            tooltip: 'Actualizar',
            icon: const Icon(Icons.refresh),
            onPressed: _recargar,
          ),
        ],
      ),
      floatingActionButton: _editando
          ? FloatingActionButton.extended(
              onPressed: _agregarAparato,
              icon: const Icon(Icons.add),
              label: const Text('Agregar aparato'),
            )
          : null,
      body: FutureBuilder<DatosMapa>(
        future: _datos,
        builder: (_, resultado) {
          if (resultado.connectionState != ConnectionState.done) {
            return const Center(child: CircularProgressIndicator());
          }
          if (resultado.hasError) {
            return Center(
              child: TextButton(
                onPressed: () => mostrarProblema(contexto, resultado.error!),
                child: const Text('No se pudo armar el mapa. Ver detalles'),
              ),
            );
          }

          final datos = resultado.data!;
          if (datos.equipos.isEmpty) {
            return const Center(
              child: Padding(
                padding: EdgeInsets.all(32),
                child: Text('Todavia no hay equipos que dibujar. Corra un escaneo.',
                    textAlign: TextAlign.center),
              ),
            );
          }

          final plano = armarPlano(datos, Theme.of(contexto).colorScheme);

          // El lienzo va dentro del InteractiveViewer, asi que el gesto llega
          // con las coordenadas ya convertidas al espacio del plano: no hay que
          // deshacer el zoom ni el desplazamiento a mano.
          Widget lienzo = CustomPaint(
            size: plano.tamano,
            painter: PintorMapa(plano: plano),
          );
          if (_editando) {
            lienzo = GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTapUp: (detalle) => _alTocar(detalle.localPosition, plano, datos),
              child: lienzo,
            );
          }

          return Column(
            children: [
              _Leyenda(mapa: datos.mapa, topologia: datos.topologia),
              if (datos.topologia.contradicciones.isNotEmpty)
                _AvisoContradicciones(contradicciones: datos.topologia.contradicciones),
              if (_editando) const _BarraEdicion(),
              Expanded(
                child: InteractiveViewer(
                  constrained: false,
                  minScale: 0.2,
                  maxScale: 3,
                  boundaryMargin: const EdgeInsets.all(200),
                  child: lienzo,
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

/// _BarraEdicion explica que hace cada clic mientras se edita.
///
/// Sin esto el modo edicion es un lienzo mudo donde no se sabe donde hay que
/// tocar, y la gente prueba a lo tonto sobre el mapa de su propia red.
class _BarraEdicion extends StatelessWidget {
  const _BarraEdicion();

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      color: colores.tertiaryContainer,
      child: Row(
        children: [
          Icon(Icons.edit_outlined, size: 18, color: colores.onTertiaryContainer),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              'Modo edicion: toque un puerto libre para conectarle algo, un equipo '
              'para declarar sus puertos, o un cable declarado para quitarlo.',
              style: Theme.of(contexto)
                  .textTheme
                  .labelMedium
                  ?.copyWith(color: colores.onTertiaryContainer),
            ),
          ),
        ],
      ),
    );
  }
}

/// _AvisoContradicciones muestra donde lo declarado a mano y lo que reporta el
/// equipo dicen cosas distintas.
///
/// No se pisa ninguno de los dos: pisar en silencio lo tecleado borraria trabajo
/// de una persona, y pisar lo medido dejaria el mapa mintiendo sobre lo que el
/// switch acaba de decir.
class _AvisoContradicciones extends StatelessWidget {
  const _AvisoContradicciones({required this.contradicciones});

  final List<Contradiccion> contradicciones;

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      color: colores.errorContainer,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.warning_amber, size: 18, color: colores.onErrorContainer),
              const SizedBox(width: 8),
              Text(
                contradicciones.length == 1
                    ? 'Un tramo declarado a mano ya no coincide con lo que reporta el equipo'
                    : '${contradicciones.length} tramos declarados a mano ya no coinciden '
                        'con lo que reportan los equipos',
                style: Theme.of(contexto)
                    .textTheme
                    .labelLarge
                    ?.copyWith(color: colores.onErrorContainer),
              ),
            ],
          ),
          const SizedBox(height: 4),
          for (final choque in contradicciones.take(5))
            Padding(
              padding: const EdgeInsets.only(left: 26, top: 2),
              child: Text(
                '${choque.equipoNombre}, puerto ${choque.numero}: usted declaro '
                '"${choque.declarado}" y ${choque.fuente.toUpperCase()} dice '
                '"${choque.medido}".',
                style: Theme.of(contexto)
                    .textTheme
                    .bodySmall
                    ?.copyWith(color: colores.onErrorContainer),
              ),
            ),
        ],
      ),
    );
  }
}

class _Leyenda extends StatelessWidget {
  const _Leyenda({required this.mapa, this.topologia = const TopologiaManual()});

  final MapaPuertos mapa;
  final TopologiaManual topologia;

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      color: colores.surfaceContainerHighest,
      child: Wrap(
        spacing: 20,
        runSpacing: 8,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          Text(mapa.explicacion, style: Theme.of(contexto).textTheme.bodySmall),
          _DeCuandoEs(momento: mapa.momento),
          const _Marca(texto: 'Linea llena: puerto confirmado', punteada: false),
          const _Marca(texto: 'Linea punteada: grupo tras algo no administrable', punteada: true),
          // La tercera fuente del mapa. Va en la leyenda por la misma razon que
          // las otras dos: un plano donde no se distingue lo medido de lo
          // tecleado acaba usandose como si todo estuviera comprobado.
          if (topologia.hayAlgo)
            _Marca(
              texto: 'Punteado largo y caja punteada: declarado a mano',
              punteada: true,
              color: colores.secondary,
            ),
          if (mapa.enlacesUnicos.isNotEmpty)
            _Marca(
              texto: 'Arco: cable entre switches, anunciado por '
                  '${mapa.enlaces.map((e) => e.origen.toUpperCase()).toSet().join(' y ')}',
              punteada: false,
              color: colores.primary,
            ),
        ],
      ),
    );
  }
}

class _Marca extends StatelessWidget {
  const _Marca({required this.texto, required this.punteada, this.color});

  final String texto;
  final bool punteada;
  final Color? color;

  @override
  Widget build(BuildContext contexto) => Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(punteada ? Icons.more_horiz : Icons.remove, size: 18, color: color),
          const SizedBox(width: 4),
          Text(texto,
              style: Theme.of(contexto).textTheme.labelSmall?.copyWith(color: color)),
        ],
      );
}

/// _DeCuandoEs dice de cuando son los datos del mapa que se esta viendo.
///
/// Lo mismo que va en el archivo exportado, porque la pregunta es la misma:
/// ¿esto que veo sigue siendo cierto?
class _DeCuandoEs extends StatelessWidget {
  const _DeCuandoEs({required this.momento});

  final String momento;

  @override
  Widget build(BuildContext contexto) {
    final cuando = haceCuanto(momento.isEmpty ? null : momento);
    final colores = Theme.of(contexto).colorScheme;
    final alarmante = cuando.frescura == Frescura.viejo ||
        cuando.frescura == Frescura.muyViejo ||
        cuando.frescura == Frescura.nunca;

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(Icons.schedule, size: 16, color: alarmante ? colores.error : null),
        const SizedBox(width: 4),
        Text(
          momento.isEmpty ? 'Sin datos todavia' : 'Datos de ${cuando.texto.toLowerCase()}',
          style: Theme.of(contexto).textTheme.labelSmall
              ?.copyWith(color: alarmante ? colores.error : null),
        ),
      ],
    );
  }
}
