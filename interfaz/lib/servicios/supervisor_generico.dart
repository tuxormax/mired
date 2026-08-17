import 'dart:convert';
import 'dart:io';

/// Version de escritorio: la que de verdad arranca y mata procesos.
///
/// Dos ideas sostienen todo este archivo:
///
///  1. **Solo se toca lo de este equipo.** Si el programa esta apuntando al
///     MiRed de otro sitio, aqui no se arranca ni se mata nada: ese servicio es
///     de alguien mas y puede estar vigilando una red que no es la nuestra.
///  2. **Un servidor de otra version no sirve.** Si al abrir ya hay un MiRed
///     corriendo en este equipo pero es de una version distinta —tipico despues
///     de actualizar el paquete sin cerrar el programa—, se detiene y se levanta
///     el instalado. Colgarse de el daria una interfaz nueva hablando con un
///     motor viejo: la mitad de las cosas no funcionarian y nada lo explicaria.
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

  /// aviso explica por que MiRed arranco a medias, o que tuvo que hacer.
  ///
  /// Se guarda en vez de lanzarse como error porque casi nunca impide trabajar.
  /// Callarlo seria peor: la pantalla se veria rara sin explicacion.
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

  /// carpetaDeConfiguracion es donde va lo que NO son datos: hoy, la llave con
  /// la que se cifran las contrasenas de los aparatos.
  ///
  /// **Separada de los datos a proposito.** La carpeta de datos es lo unico que
  /// hay que respaldar, y esos respaldos acaban en discos, en la nube y en
  /// correos: si la llave viajara ahi dentro, cifrar no protegeria de nada.
  ///
  /// Sin esto el servidor cae a `/etc/mired/llave-secretos`, que es lo correcto
  /// cuando corre como servicio del sistema y **imposible** cuando lo lanza el
  /// programa, porque esa carpeta es de root. Guardar la contrasena de un
  /// aparato fallaba siempre con «permission denied», y solo se notaba la
  /// primera vez que hacia falta cifrar algo.
  static String get carpetaDeConfiguracion {
    final casa = Platform.environment['HOME'] ?? '/tmp';
    return '$casa/.config/mired';
  }

  /// arrancar deja MiRed listo para usarse, o deja claro por que no lo esta.
  ///
  /// El parametro dice a que servidor apunta el programa. Si no es el de este
  /// equipo, aqui no se hace nada.
  Future<void> arrancar({String servidor = 'http://localhost:60072'}) async {
    if (!_esDeEsteEquipo(servidor)) {
      // El programa mira el MiRed de otro sitio: alla no mandamos nosotros.
      return;
    }

    final binarioServidor = _buscarBinario('mired-servidor');
    final instalada = binarioServidor == null
        ? null
        : await _versionDelBinario(binarioServidor);
    final viva = await _versionDelQueEstaCorriendo();

    if (viva != null) {
      // Ya hay un MiRed vivo aqui. Solo sirve si es el mismo que esta instalado.
      if (instalada == null || viva == instalada) return;

      _aviso = 'Habia un MiRed $viva corriendo de antes. Se detuvo y se arranco '
          'el instalado, $instalada.';
      await _detenerLosDeAntes();
    }

    if (binarioServidor == null) {
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

    await _lanzar(binarioServidor, entorno);

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

  /// _detenerLosDeAntes para los servicios que quedaron de una version anterior.
  ///
  /// Solo se llama cuando ya se sabe que la version no coincide **y** que son de
  /// este equipo. Se les manda SIGTERM por su ruta exacta —no por un patron
  /// suelto— para no llevarse por delante nada que solo se le parezca.
  Future<void> _detenerLosDeAntes() async {
    for (final nombre in ['mired-servidor', 'mired-sonda', 'mired-dpi']) {
      final ruta = _buscarBinario(nombre);
      if (ruta == null) continue;
      await Process.run('pkill', ['-TERM', '-f', '^$ruta\$']);
    }

    // Esperar a que suelten el puerto: arrancar el nuestro mientras el viejo lo
    // tiene tomado fallaria con un "direccion en uso" que no explica nada.
    for (var intento = 0; intento < 40; intento++) {
      if (!await _contesta()) return;
      await Future<void>.delayed(const Duration(milliseconds: 250));
    }
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
    final configuracion = carpetaDeConfiguracion;
    return {
      ...Platform.environment,
      'MIRED_DATOS': datos,
      // Solo a este equipo: como programa, MiRed no tiene por que quedar
      // expuesto a la red sin que nadie lo haya pedido.
      'MIRED_ESCUCHA': '127.0.0.1:$_puerto',
      // La llave del cifrado, en la carpeta de configuracion y NO en la de
      // datos: respaldar los datos no puede llevarse con que descifrarlos.
      'MIRED_LLAVE_SECRETOS': '$configuracion/llave-secretos',
      'MIRED_SOCKET_SONDA': '$datos/sonda.sock',
      'MIRED_SOCKET_DPI': '$datos/dpi.sock',
      // El catalogo de dispositivos: se LEE lo que instalo el paquete, pero lo
      // que se declara aqui y lo que se baja de la comunidad se ESCRIBE en la
      // carpeta del usuario. Corriendo como programa no hay permiso para tocar
      // /etc ni /var/lib, y sin esto guardar una definicion fallaria siempre.
      'MIRED_DISPOSITIVOS':
          '/usr/share/mired/dispositivos:$datos/dispositivos-comunidad:$datos/dispositivos',
      'MIRED_DISPOSITIVOS_PROPIOS': '$datos/dispositivos',
      'MIRED_DISPOSITIVOS_COMUNIDAD': '$datos/dispositivos-comunidad',
    };
  }

  /// Las dos decisiones de esta clase se exponen para poder probarlas: a que
  /// servidor se le puede tocar, y cuando dos versiones son la misma. Todo lo
  /// demas de aqui arranca procesos de verdad y no se puede probar sin ellos.
  static bool esDeEsteEquipoParaPruebas(String direccion) => _esDeEsteEquipo(direccion);
  static String? versionParaPruebas(String texto) => _soloVersionYRevision(texto);

  /// _esDeEsteEquipo dice si esa direccion apunta aqui.
  ///
  /// Se exige una direccion completa y con nombre de equipo. Un texto cualquiera
  /// **no** cuenta como "este equipo": `Uri.tryParse` acepta casi todo y devuelve
  /// un nombre vacio, asi que tomarlo por bueno haria arrancar y matar servicios
  /// locales por una direccion mal escrita.
  static bool _esDeEsteEquipo(String direccion) {
    final url = Uri.tryParse(direccion.trim());
    if (url == null) return false;
    if (!url.isScheme('http') && !url.isScheme('https')) return false;

    return url.host == 'localhost' || url.host == '127.0.0.1' || url.host == '::1';
  }

  /// _buscarBinario devuelve la ruta del programa, o null si no esta.
  String? _buscarBinario(String nombre) {
    for (final carpeta in _dondeBuscar) {
      final ruta = '$carpeta/$nombre';
      if (File(ruta).existsSync()) return ruta;
    }
    return null;
  }

  /// _versionDelBinario pregunta al ejecutable instalado cual es.
  ///
  /// Se le pregunta a EL y no se compila el numero dentro de esta interfaz: asi
  /// la comparacion es entre lo que hay instalado y lo que esta corriendo, que
  /// es justo el desajuste que se quiere cazar.
  Future<String?> _versionDelBinario(String ruta) async {
    try {
      final resultado = await Process.run(ruta, ['--version']);
      return _soloVersionYRevision(resultado.stdout.toString());
    } catch (_) {
      return null;
    }
  }

  /// _versionDelQueEstaCorriendo pregunta al servidor vivo, si lo hay.
  Future<String?> _versionDelQueEstaCorriendo() async {
    try {
      final cliente = HttpClient()..connectionTimeout = const Duration(seconds: 2);
      final peticion = await cliente.getUrl(
          Uri.parse('http://127.0.0.1:$_puerto/api/estado'));
      final respuesta = await peticion.close().timeout(const Duration(seconds: 3));
      final cuerpo = await respuesta.transform(utf8.decoder).join();
      cliente.close();

      final sobre = jsonDecode(cuerpo) as Map<String, dynamic>;
      final datos = sobre['datos'] as Map<String, dynamic>?;
      if (datos == null) return null;
      return _soloVersionYRevision('${datos['version']} Rev ${datos['revision']}');
    } catch (_) {
      // No contesta, o contesta algo que no es MiRed: para el caso es lo mismo.
      return null;
    }
  }

  /// _soloVersionYRevision deja "v1.14 Rev 15" de cualquiera de las dos formas.
  ///
  /// El binario dice `mired-servidor MiRed v1.14 Rev 15 (4ea57e5)` y la API
  /// devuelve los campos sueltos. **El hash del build se descarta a proposito**:
  /// dos compilaciones de la misma entrega son la misma version, y compararlo
  /// haria reiniciar el servidor cada vez que alguien recompila sin cambiar nada.
  static String? _soloVersionYRevision(String texto) {
    final encontrado =
        RegExp(r'(v?[\d.]+)\s+Rev\s+(\d+)', caseSensitive: false).firstMatch(texto);
    if (encontrado == null) return null;

    var numero = encontrado.group(1)!;
    if (!numero.startsWith('v')) numero = 'v$numero';
    return '$numero Rev ${encontrado.group(2)}';
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
