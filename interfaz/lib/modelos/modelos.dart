/// Los datos que viajan entre la API y la interfaz.
///
/// Los nombres de los campos son los mismos que en Go y en la base: si algo se
/// llama distinto en cada capa, tarde o temprano alguien mapea mal un campo.
library;

class Usuario {
  final int id;
  final String usuario;
  final String nombre;
  final String correo;
  final bool superadmin;
  final String? ultimoAcceso;

  const Usuario({
    required this.id,
    required this.usuario,
    required this.nombre,
    required this.correo,
    required this.superadmin,
    this.ultimoAcceso,
  });

  factory Usuario.desdeJson(Map<String, dynamic> json) => Usuario(
        id: json['id'] as int,
        usuario: json['usuario'] as String? ?? '',
        nombre: json['nombre'] as String? ?? '',
        correo: json['correo'] as String? ?? '',
        superadmin: json['superadmin'] as bool? ?? false,
        ultimoAcceso: json['ultimoAcceso'] as String?,
      );
}

class Red {
  final int id;
  final String clave;
  final String nombre;
  final String descripcion;
  final int equipos;
  final int equiposPresentes;
  final String? ultimoEscaneo;
  final int alertasAbiertas;

  const Red({
    required this.id,
    required this.clave,
    required this.nombre,
    required this.descripcion,
    required this.equipos,
    required this.equiposPresentes,
    this.ultimoEscaneo,
    required this.alertasAbiertas,
  });

  factory Red.desdeJson(Map<String, dynamic> json) => Red(
        id: json['id'] as int,
        clave: json['clave'] as String,
        nombre: json['nombre'] as String,
        descripcion: json['descripcion'] as String? ?? '',
        equipos: json['equipos'] as int? ?? 0,
        equiposPresentes: json['equiposPresentes'] as int? ?? 0,
        ultimoEscaneo: json['ultimoEscaneo'] as String?,
        alertasAbiertas: json['alertasAbiertas'] as int? ?? 0,
      );
}

class Subred {
  final int id;
  final String cidr;
  final String nombre;
  final String descripcion;
  final int? vlan;
  final bool escanear;

  const Subred({
    required this.id,
    required this.cidr,
    required this.nombre,
    required this.descripcion,
    this.vlan,
    required this.escanear,
  });

  factory Subred.desdeJson(Map<String, dynamic> json) => Subred(
        id: json['id'] as int,
        cidr: json['cidr'] as String,
        nombre: json['nombre'] as String? ?? '',
        descripcion: json['descripcion'] as String? ?? '',
        vlan: json['vlan'] as int?,
        escanear: json['escanear'] as bool? ?? true,
      );
}

/// Lo que la API devuelve cuando algo sale mal, con el bloque de detalles
/// completo del estandar de la casa. Es lo que se muestra en el modal y lo que
/// el usuario copia.
class ErrorMiRed implements Exception {
  final String id;
  final String momento;
  final String modulo;
  final String accion;
  final String causa;
  final String ruta;
  final String tabla;
  final String endpoint;
  final String red;
  final String usuario;
  final String version;
  final String build;
  final String so;
  final String estado;
  final String mensaje;
  final String errorCrudo;

  const ErrorMiRed({
    this.id = '',
    this.momento = '',
    this.modulo = '',
    this.accion = '',
    this.causa = '',
    this.ruta = '',
    this.tabla = '',
    this.endpoint = '',
    this.red = '',
    this.usuario = '',
    this.version = '',
    this.build = '',
    this.so = '',
    this.estado = '',
    this.mensaje = '',
    this.errorCrudo = '',
  });

  factory ErrorMiRed.desdeJson(Map<String, dynamic> json) => ErrorMiRed(
        id: json['id'] as String? ?? '',
        momento: json['momento'] as String? ?? '',
        modulo: json['modulo'] as String? ?? '',
        accion: json['accion'] as String? ?? '',
        causa: json['causa'] as String? ?? '',
        ruta: json['ruta'] as String? ?? '',
        tabla: json['tabla'] as String? ?? '',
        endpoint: json['endpoint'] as String? ?? '',
        red: json['red'] as String? ?? '',
        usuario: json['usuario'] as String? ?? '',
        version: json['version'] as String? ?? '',
        build: json['build'] as String? ?? '',
        so: json['so'] as String? ?? '',
        estado: json['estado'] as String? ?? '',
        mensaje: json['mensaje'] as String? ?? '',
        errorCrudo: json['error'] as String? ?? '',
      );

  /// esDeSesion distingue "no estas identificado" de un error de verdad: eso no
  /// se muestra como falla, se manda a la pantalla de entrada.
  bool get esDeSesion => causa == 'Sesion';

  /// esDeCaptura son los errores que corrige el usuario cambiando lo que
  /// escribio. No llevan modal de error: llevan un aviso corto.
  bool get esDeCaptura => causa == 'Validacion' || causa == 'Permiso';

  @override
  String toString() => mensaje.isNotEmpty ? mensaje : errorCrudo;
}
