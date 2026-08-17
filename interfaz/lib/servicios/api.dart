import 'dart:convert';
import 'dart:typed_data';

import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';

import '../modelos/modelos.dart';

/// Api es el unico punto por donde la interfaz habla con el servidor.
///
/// Toda respuesta trae el sobre {ok, datos, error}: si ok es falso, aqui se
/// convierte en una excepcion ErrorMiRed con el bloque de detalles completo, y
/// las pantallas no tienen que revisar codigos HTTP a mano.
class Api {
  Api._();
  static final Api instancia = Api._();

  static const _claveToken = 'mired.token';
  static const _claveServidor = 'mired.servidor';

  /// ServidorPorOmision es a donde apunta el programa de escritorio recien
  /// instalado: al MiRed de este mismo equipo.
  ///
  /// Se puede cambiar para conectarse al de otro sitio —la Raspberry de la
  /// sucursal, por ejemplo—, que es justo para lo que sirve tener un programa y
  /// no solo la pagina que sirve cada servidor.
  static const servidorPorOmision = 'http://localhost:60072';

  /// En produccion la interfaz la sirve el mismo binario, asi que el servidor
  /// es el mismo origen. Para desarrollar con recarga en caliente se compila
  /// con --dart-define=MIRED_API=http://127.0.0.1:60072
  static const String _apiDefinida = String.fromEnvironment('MIRED_API');

  /// baseDePrueba deja apuntar el cliente a un servidor de mentira. Solo lo usan
  /// las pruebas: en produccion la interfaz la sirve el mismo binario.
  String? baseDePrueba;

  String? _token;
  Usuario? usuario;
  Map<String, String> permisos = {};
  String version = '';

  /// servidor es la direccion del MiRed al que se le habla.
  ///
  /// Por omision el de este mismo equipo, que es el que levanta el propio
  /// programa. Se puede cambiar para ver el de otro sitio —la Raspberry de la
  /// sucursal—, que es justo lo que se gana teniendo un programa.
  String servidor = servidorPorOmision;

  String get _base {
    if (baseDePrueba != null) return baseDePrueba!;
    if (_apiDefinida.isNotEmpty) return _apiDefinida;
    return servidor;
  }

  /// cargarServidor recupera la direccion guardada.
  Future<void> cargarServidor() async {
    final guardado = await SharedPreferences.getInstance();
    servidor = guardado.getString(_claveServidor) ?? servidorPorOmision;
  }

  /// guardarServidor apunta el programa a otro MiRed y cierra lo que hubiera
  /// abierto: la sesion es de un servidor, no del programa.
  Future<void> guardarServidor(String direccion) async {
    servidor = direccion.trim().replaceAll(RegExp(r'/+$'), '');
    final guardado = await SharedPreferences.getInstance();
    await guardado.setString(_claveServidor, servidor);
    await _guardarToken(null);
    usuario = null;
    permisos = {};
  }

  Future<void> cargarToken() async {
    final guardado = await SharedPreferences.getInstance();
    _token = guardado.getString(_claveToken);
  }

  Future<void> _guardarToken(String? token) async {
    _token = token;
    final guardado = await SharedPreferences.getInstance();
    if (token == null) {
      await guardado.remove(_claveToken);
    } else {
      await guardado.setString(_claveToken, token);
    }
  }

  bool get hayToken => _token != null && _token!.isNotEmpty;

  Map<String, String> get _cabeceras => {
        'Content-Type': 'application/json; charset=utf-8',
        if (hayToken) 'Authorization': 'Bearer $_token',
      };

  // ------------------------------------------------------------ peticiones --

  Future<dynamic> obtener(String ruta) => _pedir('GET', ruta);
  Future<dynamic> enviar(String ruta, Object cuerpo) => _pedir('POST', ruta, cuerpo);
  Future<dynamic> modificar(String ruta, Object cuerpo) => _pedir('PATCH', ruta, cuerpo);
  Future<dynamic> reemplazar(String ruta, Object cuerpo) => _pedir('PUT', ruta, cuerpo);
  Future<dynamic> borrar(String ruta) => _pedir('DELETE', ruta);

  Future<dynamic> _pedir(String metodo, String ruta, [Object? cuerpo]) async {
    final destino = Uri.parse('$_base$ruta');
    late http.Response respuesta;

    try {
      final peticion = http.Request(metodo, destino)..headers.addAll(_cabeceras);
      if (cuerpo != null) peticion.body = jsonEncode(cuerpo);
      final flujo = await peticion.send().timeout(const Duration(seconds: 30));
      respuesta = await http.Response.fromStream(flujo);
    } catch (problema) {
      // Sin respuesta no hay bloque de detalles del servidor: se arma aqui uno
      // equivalente para que el usuario tenga algo que copiar igual.
      throw ErrorMiRed(
        modulo: 'Conexion',
        accion: '$metodo $ruta',
        causa: 'Red',
        endpoint: destino.toString(),
        mensaje: 'No se pudo contactar al servidor de MiRed.',
        errorCrudo: problema.toString(),
        estado: 'La operacion NO se realizo.',
      );
    }

    Map<String, dynamic> sobre;
    try {
      sobre = jsonDecode(respuesta.body) as Map<String, dynamic>;
    } catch (problema) {
      throw ErrorMiRed(
        modulo: 'Conexion',
        accion: '$metodo $ruta',
        causa: 'Red',
        endpoint: '$metodo $ruta → ${respuesta.statusCode}',
        mensaje: 'El servidor contesto algo que no se pudo interpretar.',
        errorCrudo: respuesta.body,
      );
    }

    if (sobre['ok'] == true) return sobre['datos'];

    final detalle = sobre['error'];
    if (detalle is Map<String, dynamic>) {
      throw ErrorMiRed.desdeJson(detalle);
    }
    throw ErrorMiRed(
      modulo: 'Conexion',
      accion: '$metodo $ruta',
      causa: 'Interno',
      mensaje: 'El servidor rechazo la peticion.',
      errorCrudo: respuesta.body,
    );
  }

  // --------------------------------------------------------------- sesion --

  /// estado es lo primero que se consulta y no necesita sesion: dice la version
  /// y si la instalacion todavia no tiene usuarios.
  Future<Map<String, dynamic>> estado() async =>
      await obtener('/api/estado') as Map<String, dynamic>;

  /// crearPrimerAdministrador da de alta al primer usuario de una instalacion
  /// recien hecha y deja la sesion abierta.
  ///
  /// El servidor solo lo permite mientras no exista ningun usuario.
  Future<void> crearPrimerAdministrador(
      String usuario, String nombre, String clave) async {
    final datos = await enviar('/api/primer-administrador', {
      'usuario': usuario,
      'nombre': nombre,
      'clave': clave,
    });
    final token = datos['token'] as String?;
    if (token != null) await _guardarToken(token);
    final creado = datos['usuario'] as Map<String, dynamic>?;
    if (creado != null) this.usuario = Usuario.desdeJson(creado);
  }

  Future<void> entrar(String usuario, String clave) async {
    final datos = await enviar('/api/sesion', {'usuario': usuario, 'clave': clave});
    await _guardarToken(datos['token'] as String);
    this.usuario = Usuario.desdeJson(datos['usuario'] as Map<String, dynamic>);
  }

  /// recuperarSesion reengancha una sesion guardada al abrir la pagina. Devuelve
  /// falso si ya no vale, para mandar a la pantalla de entrada sin tratarlo como
  /// error.
  Future<bool> recuperarSesion() async {
    await cargarServidor();
    await cargarToken();
    if (!hayToken) return false;
    try {
      final datos = await obtener('/api/sesion');
      usuario = Usuario.desdeJson(datos['usuario'] as Map<String, dynamic>);
      permisos = (datos['permisos'] as Map<String, dynamic>? ?? {})
          .map((clave, nivel) => MapEntry(clave, nivel as String));
      version = datos['version'] as String? ?? '';
      return true;
    } on ErrorMiRed catch (problema) {
      if (problema.esDeSesion) {
        await _guardarToken(null);
        return false;
      }
      rethrow;
    }
  }

  Future<void> salir() async {
    try {
      await borrar('/api/sesion');
    } catch (_) {
      // Que el servidor no conteste no debe impedir cerrar sesion aqui.
    }
    await _guardarToken(null);
    usuario = null;
    permisos = {};
  }

  // ---------------------------------------------------------------- redes --

  Future<List<Red>> listarRedes() async {
    final datos = await obtener('/api/redes') as List<dynamic>;
    return datos.map((fila) => Red.desdeJson(fila as Map<String, dynamic>)).toList();
  }

  Future<Red> crearRed(String nombre, String descripcion, List<String> subredes) async {
    final datos = await enviar('/api/redes', {
      'nombre': nombre,
      'descripcion': descripcion,
      'subredes': subredes,
    });
    return Red.desdeJson(datos as Map<String, dynamic>);
  }

  /// borrarRed quita la red. Por omision el archivo de datos se conserva, que
  /// es lo que permite recuperarla; con tambienLosDatos se borra de verdad.
  Future<void> borrarRed(String clave, {bool tambienLosDatos = false}) =>
      borrar('/api/redes/$clave${tambienLosDatos ? '?datos=si' : ''}');

  /// verRed vuelve a leer UNA red.
  ///
  /// Hace falta para que la pantalla de un sitio refresque sus propios
  /// contadores —las alertas abiertas, sobre todo— sin tener que salir al panel
  /// de inicio y volver a entrar.
  Future<Red> verRed(String clave) async {
    final datos = await obtener('/api/redes/$clave') as Map<String, dynamic>;
    return Red.desdeJson(datos['red'] as Map<String, dynamic>);
  }

  Future<List<Subred>> listarSubredes(String clave) async {
    final datos = await obtener('/api/redes/$clave/subredes') as List<dynamic>;
    return datos.map((fila) => Subred.desdeJson(fila as Map<String, dynamic>)).toList();
  }

  Future<Subred> crearSubred(String clave, String cidr, String nombre, int? vlan) async {
    final datos = await enviar('/api/redes/$clave/subredes', {
      'cidr': cidr,
      'nombre': nombre,
      'vlan': vlan,
    });
    return Subred.desdeJson(datos as Map<String, dynamic>);
  }

  // --------------------------------------------------- equipos y escaneos --

  Future<List<Equipo>> listarEquipos(String clave, {bool soloPresentes = false}) async {
    final ruta = '/api/redes/$clave/equipos${soloPresentes ? '?presentes=1' : ''}';
    final datos = await obtener(ruta) as List<dynamic>;
    return datos.map((fila) => Equipo.desdeJson(fila as Map<String, dynamic>)).toList();
  }

  Future<void> ponerAlias(String clave, int equipoId, String alias) =>
      modificar('/api/redes/$clave/equipos/$equipoId', {'alias': alias});

  /// De que esta hecha la red: el total y cuantos de cada tipo.
  ///
  /// Lo cuenta el servidor sobre la misma tabla que alimenta la lista y el mapa,
  /// no la interfaz sobre lo que tenga en pantalla: con un filtro puesto, contar
  /// aqui diria "3 equipos" en una red de treinta.
  Future<ComposicionDeRed> composicion(String clave) async {
    final datos = await obtener('/api/redes/$clave/composicion');
    return ComposicionDeRed.desdeJson(datos as Map<String, dynamic>);
  }

  // ------------------------------------------------- topologia declarada --

  /// La tercera fuente del mapa: lo que una persona declara porque tiene el
  /// cable delante. Trae tambien donde eso ya no cuadra con lo que reportan los
  /// equipos, para poder preguntar en vez de pisar en silencio.
  Future<TopologiaManual> topologiaManual(String clave) async {
    final datos = await obtener('/api/redes/$clave/topologia-manual');
    return TopologiaManual.desdeJson(datos as Map<String, dynamic>);
  }

  /// Da de alta un aparato que ningun barrido va a encontrar: el switch tonto,
  /// el modem que no habla SNMP hacia la LAN.
  Future<Equipo> crearEquipoManual(String clave, Map<String, dynamic> equipo) async {
    final datos = await enviar('/api/redes/$clave/equipos', equipo);
    return Equipo.desdeJson(datos as Map<String, dynamic>);
  }

  /// Guarda lo que una persona sabe del equipo. Sirve igual para lo descubierto
  /// y para lo declarado.
  Future<void> guardarFicha(String clave, int equipoId,
          {required String modelo,
          required String notas,
          required String conexion,
          String ubicacion = ''}) =>
      reemplazar('/api/redes/$clave/equipos/$equipoId', {
        'modelo': modelo,
        'notas': notas,
        'conexion': conexion,
        'ubicacion': ubicacion,
      });

  /// Solo borra los declarados a mano. Un equipo descubierto se marca ausente,
  /// nunca se borra: su historia es lo que despues permite avisar de que lleva
  /// dias sin aparecer.
  Future<void> borrarEquipoManual(String clave, int equipoId) =>
      borrar('/api/redes/$clave/equipos/$equipoId');

  // ---------------------------------------------------------- importacion --
  //
  // Son DOS pasos a proposito: la vista previa no escribe nada y dice renglon
  // por renglon lo que pasaria; aplicar escribe, y va todo en una transaccion.

  /// La plantilla para llenar y la guia de como se llena, las dos de la misma
  /// fuente: la definicion con la que el servidor lee el archivo.
  Future<PlantillaImportacion> plantillaDeImportacion(String clave) async {
    final datos = await obtener('/api/redes/$clave/importacion/plantilla');
    return PlantillaImportacion.desdeJson(datos as Map<String, dynamic>);
  }

  /// Que pasaria con este archivo. No toca la base.
  Future<PlanImportacion> vistaPreviaImportacion(
      String clave, String nombre, Uint8List archivo) async {
    final datos = await enviar('/api/redes/$clave/importacion/vista-previa', {
      'nombre': nombre,
      'contenido': base64Encode(archivo),
    });
    return PlanImportacion.desdeJson(datos as Map<String, dynamic>);
  }

  /// Aplica el archivo. [repetidos] es «actualizar» o «saltar».
  Future<ResumenImportacion> importar(String clave, String nombre, Uint8List archivo,
      {required String repetidos}) async {
    final datos = await enviar('/api/redes/$clave/importacion', {
      'nombre': nombre,
      'contenido': base64Encode(archivo),
      'repetidos': repetidos,
    });
    return ResumenImportacion.desdeJson(datos as Map<String, dynamic>);
  }

  Future<PuertoFisico> agregarPuerto(String clave, int equipoId,
      {required int numero, required String tipo, int? velocidadMbps, String notas = ''}) async {
    final datos = await enviar('/api/redes/$clave/equipos/$equipoId/puertos', {
      'numero': numero,
      'tipo': tipo,
      'velocidadMbps': velocidadMbps,
      'notas': notas,
    });
    return PuertoFisico.desdeJson(datos as Map<String, dynamic>);
  }

  Future<void> editarPuerto(String clave, int puertoId,
          {required int numero, required String tipo, int? velocidadMbps, String notas = ''}) =>
      reemplazar('/api/redes/$clave/puertos/$puertoId', {
        'numero': numero,
        'tipo': tipo,
        'velocidadMbps': velocidadMbps,
        'notas': notas,
      });

  Future<void> borrarPuerto(String clave, int puertoId) =>
      borrar('/api/redes/$clave/puertos/$puertoId');

  /// Conecta un puerto. El destino es OTRO puerto (switch con switch, switch con
  /// modem) o un equipo entero, cuando ese equipo tiene una sola salida de red y
  /// no tiene sentido inventarle un "puerto 1".
  Future<EnlaceFisico> conectar(String clave,
      {required int puertoOrigenId, int? puertoDestinoId, int? equipoDestinoId}) async {
    final datos = await enviar('/api/redes/$clave/enlaces', {
      'puertoOrigenId': puertoOrigenId,
      'puertoDestinoId': puertoDestinoId,
      'equipoDestinoId': equipoDestinoId,
    });
    return EnlaceFisico.desdeJson(datos as Map<String, dynamic>);
  }

  Future<void> desconectar(String clave, int enlaceId) =>
      borrar('/api/redes/$clave/enlaces/$enlaceId');

  Future<int> lanzarEscaneo(String clave, {bool soloPresencia = false}) async {
    final datos = await enviar('/api/redes/$clave/escaneos', {'soloPresencia': soloPresencia});
    return (datos as Map<String, dynamic>)['escaneoId'] as int;
  }

  /// Devuelve las ultimas corridas y si hay una en curso ahora mismo. El "en
  /// curso" lo dice el servidor y no se deduce de la lista: un escaneo puede
  /// estar corriendo aunque su renglon todavia no cambie.
  Future<({List<Escaneo> escaneos, bool enCurso})> listarEscaneos(String clave) async {
    final datos = await obtener('/api/redes/$clave/escaneos') as Map<String, dynamic>;
    final lista = (datos['escaneos'] as List<dynamic>? ?? [])
        .map((fila) => Escaneo.desdeJson(fila as Map<String, dynamic>))
        .toList();
    return (escaneos: lista, enCurso: datos['enCurso'] as bool? ?? false);
  }

  Future<List<EventoPresencia>> listarPresencia(String clave, int equipoId) async {
    final datos = await obtener('/api/redes/$clave/equipos/$equipoId/presencia') as List<dynamic>;
    return datos
        .map((fila) => EventoPresencia.desdeJson(fila as Map<String, dynamic>))
        .toList();
  }

  Future<Red> configurarAgenda(String clave,
      {required bool programado,
      required int presenciaCadaSegundos,
      required int profundoCadaMinutos}) async {
    final datos = await reemplazar('/api/redes/$clave/agenda', {
      'programado': programado,
      'presenciaCadaSegundos': presenciaCadaSegundos,
      'profundoCadaMinutos': profundoCadaMinutos,
    });
    return Red.desdeJson(datos as Map<String, dynamic>);
  }

  // ---------------------------------------------------------------- alertas --

  Future<({List<Alerta> alertas, int abiertas})> listarAlertas(String clave,
      {bool soloSinVer = false}) async {
    final datos = await obtener(
        '/api/redes/$clave/alertas${soloSinVer ? '?sinVer=1' : ''}') as Map<String, dynamic>;
    final lista = (datos['alertas'] as List<dynamic>? ?? [])
        .map((fila) => Alerta.desdeJson(fila as Map<String, dynamic>))
        .toList();
    return (alertas: lista, abiertas: datos['abiertas'] as int? ?? 0);
  }

  /// Con la lista vacia se marcan TODAS: es el boton de "ya las vi".
  Future<int> marcarAlertasVistas(String clave, {List<int> ids = const []}) async {
    final datos = await enviar('/api/redes/$clave/alertas/vistas', {'ids': ids});
    return (datos as Map<String, dynamic>)['abiertas'] as int? ?? 0;
  }

  Future<List<Regla>> listarReglas(String clave) async {
    final datos = await obtener('/api/redes/$clave/reglas') as List<dynamic>;
    return datos.map((fila) => Regla.desdeJson(fila as Map<String, dynamic>)).toList();
  }

  Future<void> guardarRegla(String clave, Regla regla) =>
      reemplazar('/api/redes/$clave/reglas/${regla.tipo}',
          {'activa': regla.activa, 'umbral': regla.umbral});

  Future<List<DestinoAlerta>> listarDestinos(String clave) async {
    final datos = await obtener('/api/redes/$clave/destinos') as List<dynamic>;
    return datos
        .map((fila) => DestinoAlerta.desdeJson(fila as Map<String, dynamic>))
        .toList();
  }

  Future<DestinoAlerta> crearDestino(String clave, Map<String, dynamic> destino) async {
    final datos = await enviar('/api/redes/$clave/destinos', destino);
    return DestinoAlerta.desdeJson(datos as Map<String, dynamic>);
  }

  Future<void> borrarDestino(String clave, int id) =>
      borrar('/api/redes/$clave/destinos/$id');

  // ------------------------------------------------ SNMP y mapa de puertos --

  Future<MapaPuertos> mapaDePuertos(String clave) async {
    final datos = await obtener('/api/redes/$clave/mapa-puertos');
    return MapaPuertos.desdeJson(datos as Map<String, dynamic>);
  }

  /// Genera el `.toml` de un equipo que nadie reconocio, ya relleno con lo que
  /// se vio. Es como se aporta al catalogo sin saber programar.
  Future<Map<String, dynamic>> proponerDefinicion(
      String clave, int equipoId, String nombre) async {
    final datos = await obtener(
        '/api/redes/$clave/equipos/$equipoId/propuesta?nombre=${Uri.encodeQueryComponent(nombre)}');
    return datos as Map<String, dynamic>;
  }

  /// Cuelga uno o VARIOS equipos de una antena, de una sola vez. El WiFi no
  /// tiene puertos: a una antena se le cuelga lo que sea sin inventarle uno.
  Future<Map<String, dynamic>> colgarPorWiFi(String clave,
      {required int antenaId, required List<int> equipos, String red = ''}) async {
    final datos = await enviar('/api/redes/$clave/inalambricos', {
      'antenaId': antenaId,
      'equipos': equipos,
      'red': red,
    });
    return datos as Map<String, dynamic>;
  }

  /// Quita un equipo de su antena.
  Future<void> descolgarDeWiFi(String clave, int enlaceId) async {
    await borrar('/api/redes/$clave/inalambricos/$enlaceId');
  }

  /// Guarda como se entra a un aparato. La clave vacia conserva la que hubiera:
  /// en un formulario donde se muestra oculta, en blanco significa "no la
  /// toques", no "quitala".
  Future<CredencialEquipo> guardarCredencial(String clave, int equipoId,
      Map<String, dynamic> credencial) async {
    final datos = await reemplazar('/api/redes/$clave/equipos/$equipoId/credencial', credencial);
    return CredencialEquipo.desdeJson(datos as Map<String, dynamic>);
  }

  /// Pide la clave en claro. Es la unica puerta por donde sale una clave, y el
  /// servidor deja anotado quien la pidio.
  Future<CredencialEquipo> verClave(String clave, int equipoId, {String tipo = 'web'}) async {
    final datos = await obtener('/api/redes/$clave/equipos/$equipoId/credencial/clave?tipo=$tipo');
    return CredencialEquipo.desdeJson(datos as Map<String, dynamic>);
  }

  /// Borra la credencial guardada de un equipo. Se llama distinto de
  /// [borrarCredencial], que es la de SNMP: son dos cosas diferentes y
  /// confundirlas seria borrar la que no es.
  Future<void> borrarCredencialDeEquipo(String clave, int credencialId) async {
    await borrar('/api/redes/$clave/credenciales/$credencialId');
  }

  /// Para los barridos automaticos de una red mientras alguien la edita.
  ///
  /// La pausa vence sola: si el programa se cierra de golpe, la red no se queda
  /// sin vigilancia para siempre por un aviso que nadie retiro.
  Future<void> pausarAgenda(String clave, {int minutos = 20}) async {
    await enviar('/api/redes/$clave/pausa', {'minutos': minutos});
  }

  /// Vuelve a dejar correr los barridos automaticos.
  Future<void> reanudarAgenda(String clave) async {
    await borrar('/api/redes/$clave/pausa');
  }

  /// Escucha que redes inalambricas se oyen desde el equipo donde corre MiRed.
  Future<Map<String, dynamic>> barrerAire(String clave) async {
    final datos = await obtener('/api/redes/$clave/aire');
    return datos as Map<String, dynamic>;
  }

  /// Que aparatos sabe reconocer esta instalacion, y que archivos no se
  /// pudieron cargar.
  Future<Map<String, dynamic>> catalogo() async {
    final datos = await obtener('/api/catalogo');
    return datos as Map<String, dynamic>;
  }

  /// Guarda una definicion en el catalogo de ESTA instalacion. El aparato queda
  /// reconocido desde el proximo escaneo, sin reiniciar nada.
  Future<Map<String, dynamic>> guardarDefinicion(String archivo, String contenido) async {
    final datos = await enviar('/api/catalogo/dispositivos', {
      'archivo': archivo,
      'contenido': contenido,
    });
    return datos as Map<String, dynamic>;
  }

  /// Trae las definiciones que publico la comunidad. No pisa las propias: se
  /// guardan en su propia carpeta.
  Future<Map<String, dynamic>> actualizarCatalogo() async {
    final datos = await enviar('/api/catalogo/actualizar', {});
    return datos as Map<String, dynamic>;
  }

  Future<Consumo> consumo(String clave) async {
    final datos = await obtener('/api/redes/$clave/consumo');
    return Consumo.desdeJson(datos as Map<String, dynamic>);
  }

  /// consumoPorAplicacion sale del paquete opcional mired-dpi. Si no esta
  /// instalado, la lista viene vacia y la pantalla lo explica.
  Future<List<ConsumoPorAplicacion>> consumoPorAplicacion(String clave) async {
    final datos = await obtener('/api/redes/$clave/aplicaciones') as Map<String, dynamic>;
    return ((datos['consumo'] as List<dynamic>?) ?? [])
        .map((fila) => ConsumoPorAplicacion.desdeJson(fila as Map<String, dynamic>))
        .toList();
  }

  /// historialVersiones dice que cambio en cada entrega. Lo abre el pie.
  Future<Map<String, dynamic>> historialVersiones() async =>
      await obtener('/api/versiones') as Map<String, dynamic>;

  // Las credenciales SNMP y las controladoras son DE CADA RED: cuelgan de ella
  // en la API igual que todo lo demas suyo. Estuvieron compartidas entre todas
  // hasta la Rev 44, y eso significaba probar la comunidad de un cliente contra
  // los switches de otro.

  Future<List<CredencialSNMP>> listarCredenciales(String clave) async {
    final datos = await obtener('/api/redes/$clave/credenciales-snmp') as List<dynamic>;
    return datos
        .map((fila) => CredencialSNMP.desdeJson(fila as Map<String, dynamic>))
        .toList();
  }

  Future<CredencialSNMP> crearCredencial(
      String clave, Map<String, dynamic> credencial) async {
    final datos = await enviar('/api/redes/$clave/credenciales-snmp', credencial);
    return CredencialSNMP.desdeJson(datos as Map<String, dynamic>);
  }

  Future<void> borrarCredencial(String clave, int id) =>
      borrar('/api/redes/$clave/credenciales-snmp/$id');

  // ------------------------------------------------------- controladoras --

  Future<List<Controladora>> listarControladoras(String clave) async {
    final datos = await obtener('/api/redes/$clave/controladoras') as List<dynamic>;
    return datos
        .map((fila) => Controladora.desdeJson(fila as Map<String, dynamic>))
        .toList();
  }

  Future<Controladora> crearControladora(
      String clave, Map<String, dynamic> controladora) async {
    final datos = await enviar('/api/redes/$clave/controladoras', controladora);
    return Controladora.desdeJson(datos as Map<String, dynamic>);
  }

  Future<void> borrarControladora(String clave, int id) =>
      borrar('/api/redes/$clave/controladoras/$id');

  Future<Map<String, dynamic>> estadoSonda() async =>
      await obtener('/api/sonda') as Map<String, dynamic>;

  // ------------------------------------------------------------- usuarios --

  Future<List<Usuario>> listarUsuarios() async {
    final datos = await obtener('/api/usuarios') as List<dynamic>;
    return datos.map((fila) => Usuario.desdeJson(fila as Map<String, dynamic>)).toList();
  }

  Future<Usuario> crearUsuario(
      String usuario, String nombre, String clave, String correo, bool superadmin) async {
    final datos = await enviar('/api/usuarios', {
      'usuario': usuario,
      'nombre': nombre,
      'clave': clave,
      'correo': correo,
      'superadmin': superadmin,
    });
    return Usuario.desdeJson(datos as Map<String, dynamic>);
  }

  Future<void> borrarUsuario(int id) => borrar('/api/usuarios/$id');

  Future<void> otorgarPermiso(int usuarioId, String red, String nivel) =>
      reemplazar('/api/usuarios/$usuarioId/permisos/$red', {'nivel': nivel});

  Future<void> quitarPermiso(int usuarioId, String red) =>
      borrar('/api/usuarios/$usuarioId/permisos/$red');

  // -------------------------------------------------------------- bitacora --

  /// reportarError manda el error al servidor para que quede en la bitacora.
  /// Es best-effort: si falla, no se reintenta y no se le dice nada al usuario,
  /// que ya esta viendo un error.
  Future<void> reportarError(ErrorMiRed problema, String trayectoria, String pila) async {
    try {
      await http
          .post(
            Uri.parse('$_base/api/log-error'),
            headers: {'Content-Type': 'application/json; charset=utf-8'},
            body: jsonEncode({
              'origen': 'interfaz',
              'id': problema.id,
              'momento': problema.momento,
              'modulo': problema.modulo,
              'accion': problema.accion,
              'causa': problema.causa,
              'ruta': problema.ruta,
              'tabla': problema.tabla,
              'endpoint': problema.endpoint,
              'version': problema.version,
              'build': problema.build,
              'so': problema.so,
              'mensaje': problema.mensaje,
              'error': problema.errorCrudo,
              'trayectoria': trayectoria,
              'pila': pila,
            }),
          )
          .timeout(const Duration(seconds: 3));
    } catch (_) {
      // A proposito en silencio.
    }
  }
}
