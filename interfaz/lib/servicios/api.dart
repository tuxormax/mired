import 'dart:convert';

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

  /// En produccion la interfaz la sirve el mismo binario, asi que el servidor
  /// es el mismo origen. Para desarrollar con recarga en caliente se compila
  /// con --dart-define=MIRED_API=http://127.0.0.1:60072
  static const String _apiDefinida = String.fromEnvironment('MIRED_API');

  String? _token;
  Usuario? usuario;
  Map<String, String> permisos = {};
  String version = '';

  String get _base => _apiDefinida.isNotEmpty ? _apiDefinida : Uri.base.origin;

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

  Future<void> entrar(String usuario, String clave) async {
    final datos = await enviar('/api/sesion', {'usuario': usuario, 'clave': clave});
    await _guardarToken(datos['token'] as String);
    this.usuario = Usuario.desdeJson(datos['usuario'] as Map<String, dynamic>);
  }

  /// recuperarSesion reengancha una sesion guardada al abrir la pagina. Devuelve
  /// falso si ya no vale, para mandar a la pantalla de entrada sin tratarlo como
  /// error.
  Future<bool> recuperarSesion() async {
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

  Future<void> borrarRed(String clave) => borrar('/api/redes/$clave');

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

  Future<List<CredencialSNMP>> listarCredenciales() async {
    final datos = await obtener('/api/credenciales-snmp') as List<dynamic>;
    return datos
        .map((fila) => CredencialSNMP.desdeJson(fila as Map<String, dynamic>))
        .toList();
  }

  Future<CredencialSNMP> crearCredencial(Map<String, dynamic> credencial) async {
    final datos = await enviar('/api/credenciales-snmp', credencial);
    return CredencialSNMP.desdeJson(datos as Map<String, dynamic>);
  }

  Future<void> borrarCredencial(int id) => borrar('/api/credenciales-snmp/$id');

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
