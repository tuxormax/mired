import 'dart:io';

/// Version de escritorio: la que de verdad arranca y mata procesos.
///
/// Todo lo delicado de este archivo se reduce a una idea: **solo se mata lo que
/// se arranco**. Si al abrir el programa ya habia un MiRed corriendo —porque
/// alguien lo dejo como servicio, o porque quedo vivo de una sesion anterior—
/// nos colgamos de el y al cerrar no se toca. Matar el servicio de otro seria
/// dejar sin vigilancia una red que si la tenia.

/// Servicios arranca y detiene los procesos de MiRed.
class Servicios {
  Servicios._();
  static final Servicios instancia = Servicios._();

  /// Donde busca los binarios, en orden.
  ///
  /// Primero lo instalado por el paquete; despues el arbol de compilacion, para
  /// poder desarrollar sin instalar nada.
  static const _dondeBuscar = [
    '/usr/bin',
    '/usr/local/bin',
    '../empaquetado/salida',
  ];

  static const _puerto = 60072;

  final List<Process> _arrancados = [];
  String? _aviso;

  /// aviso explica por que MiRed arranco a medias, si arranco a medias.
  ///
  /// Se guarda en vez de lanzarse como error porque casi nunca impide trabajar:
  /// sin la sonda, por ejemplo, el descubrimiento por ARP no va pero el resto
  /// si. Callarlo seria peor: la pantalla se veria incompleta sin explicacion.
  String? get aviso => _aviso;

  /// arranque dice si este programa levanto los servicios o se colgo de unos que
  /// ya estaban.
  bool get losArranqueYo => _arrancados.isNotEmpty;

  /// carpetaDeDatos es donde viven las bases cuando MiRed corre como programa.
  ///
  /// En el equipo del usuario, no en `/var/lib`: los servicios corren como el,
  /// y no tendria por que poder escribir en una carpeta del sistema. Cada
  /// usuario tiene sus redes, que es lo que se espera de un programa.
  static String get carpetaDeDatos {
    final casa = Platform.environment['HOME'] ?? '/tmp';
    return '$casa/.local/share/mired';
  }

  /// arrancar levanta lo que haga falta y espera a que el servidor conteste.
  ///
  /// Devuelve cuando MiRed ya esta listo para usarse, o cuando quedo claro que
  /// no va a estarlo.
  Future<void> arrancar() async {
    if (await _contesta()) return; // ya habia uno vivo: nos colgamos de el

    final servidor = _buscarBinario('mired-servidor');
    if (servidor == null) {
      _aviso = 'No se encontro el programa mired-servidor. Si MiRed corre en otro '
          'equipo, indique su direccion con el boton de abajo.';
      return;
    }

    await Directory('$carpetaDeDatos/redes').create(recursive: true);
    final entorno = _entorno();

    // La sonda primero: el servidor le habla en cuanto arranca, y si todavia no
    // esta escuchando el primer escaneo falla sin motivo aparente.
    final sonda = _buscarBinario('mired-sonda');
    if (sonda != null) {
      await _lanzar(sonda, entorno);
    } else {
      _aviso = 'No se encontro mired-sonda: el descubrimiento por ARP no va a '
          'funcionar, solo el sondeo de puertos.';
    }

    await _lanzar(servidor, entorno);

    if (!await _esperarAlServidor()) {
      _aviso = 'Los servicios de MiRed arrancaron pero el servidor no contesto. '
          'Revise la ventana de errores para ver que dijo.';
    }
  }

  /// detener mata lo que arranco este programa, y solo eso.
  Future<void> detener() async {
    for (final proceso in _arrancados) {
      // SIGTERM y no SIGKILL: los dos servicios cierran sus bases al recibirlo,
      // y una base SQLite cerrada a la brava deja su archivo de bitacora suelto.
      proceso.kill(ProcessSignal.sigterm);
    }
    // Se les da un momento para cerrar en orden antes de soltarlos.
    await Future.wait(
      _arrancados.map((proceso) => proceso.exitCode.timeout(
            const Duration(seconds: 5),
            onTimeout: () {
              proceso.kill(ProcessSignal.sigkill);
              return -1;
            },
          )),
    );
    _arrancados.clear();
  }

  Future<void> _lanzar(String binario, Map<String, String> entorno) async {
    final proceso = await Process.start(binario, const [], environment: entorno);
    _arrancados.add(proceso);

    // Lo que digan por consola se descarta a proposito: sus errores de verdad
    // viajan por la API al modal de errores de la casa, que es donde el usuario
    // los puede copiar. Pero HAY que consumirlo: un proceso cuya salida nadie
    // lee acaba bloqueado cuando se llena el buzon del sistema.
    proceso.stdout.drain<void>();
    proceso.stderr.drain<void>();
  }

  Map<String, String> _entorno() {
    final datos = carpetaDeDatos;
    return {
      ...Platform.environment,
      'MIRED_DATOS': datos,
      // Solo a este equipo: como programa, MiRed no tiene por que quedar
      // expuesto a la red sin que nadie lo haya pedido.
      'MIRED_ESCUCHA': '127.0.0.1:$_puerto',
      'MIRED_SOCKET_SONDA': '$datos/sonda.sock',
      'MIRED_SOCKET_DPI': '$datos/dpi.sock',
    };
  }

  /// _buscarBinario devuelve la ruta del programa, o null si no esta.
  String? _buscarBinario(String nombre) {
    for (final carpeta in _dondeBuscar) {
      final ruta = '$carpeta/$nombre';
      if (File(ruta).existsSync()) return ruta;
    }
    return null;
  }

  /// _esperarAlServidor da tiempo a que abra su puerto.
  ///
  /// Arrancar tarda: hay que abrir el catalogo, migrarlo y cargar el catalogo de
  /// dispositivos. En una Raspberry eso son segundos, no milisegundos.
  Future<bool> _esperarAlServidor() async {
    for (var intento = 0; intento < 40; intento++) {
      if (await _contesta()) return true;
      await Future<void>.delayed(const Duration(milliseconds: 250));
    }
    return false;
  }

  Future<bool> _contesta() async {
    try {
      final enchufe = await Socket.connect('127.0.0.1', _puerto,
          timeout: const Duration(milliseconds: 400));
      enchufe.destroy();
      return true;
    } catch (_) {
      return false;
    }
  }
}
