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

  // Agenda de barridos automaticos.
  final bool programado;
  final int presenciaCadaSegundos;
  final int profundoCadaMinutos;

  const Red({
    required this.id,
    required this.clave,
    required this.nombre,
    required this.descripcion,
    required this.equipos,
    required this.equiposPresentes,
    this.ultimoEscaneo,
    required this.alertasAbiertas,
    required this.programado,
    required this.presenciaCadaSegundos,
    required this.profundoCadaMinutos,
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
        programado: json['programado'] as bool? ?? false,
        presenciaCadaSegundos: json['presenciaCadaSegundos'] as int? ?? 60,
        profundoCadaMinutos: json['profundoCadaMinutos'] as int? ?? 360,
      );
}

/// Una conexion o desconexion de un equipo.
class EventoPresencia {
  final String momento;
  final bool presente;
  final String ip;

  const EventoPresencia({
    required this.momento,
    required this.presente,
    required this.ip,
  });

  factory EventoPresencia.desdeJson(Map<String, dynamic> json) => EventoPresencia(
        momento: json['momento'] as String? ?? '',
        presente: json['presente'] as bool? ?? false,
        ip: json['ip'] as String? ?? '',
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

class Equipo {
  final int id;
  final String ip;
  final String mac;
  final String fabricante;
  final String nombre;
  final String alias;
  final String tipo;
  final String subred;
  final String metodo;
  final bool presente;
  final String primeraVez;
  final String ultimaVez;
  final List<PuertoEquipo> puertos;

  const Equipo({
    required this.id,
    required this.ip,
    required this.mac,
    required this.fabricante,
    required this.nombre,
    required this.alias,
    required this.tipo,
    required this.subred,
    required this.metodo,
    required this.presente,
    required this.primeraVez,
    required this.ultimaVez,
    required this.puertos,
  });

  factory Equipo.desdeJson(Map<String, dynamic> json) => Equipo(
        id: json['id'] as int,
        ip: json['ip'] as String,
        mac: json['mac'] as String? ?? '',
        fabricante: json['fabricante'] as String? ?? '',
        nombre: json['nombre'] as String? ?? '',
        alias: json['alias'] as String? ?? '',
        tipo: json['tipo'] as String? ?? '',
        subred: json['subred'] as String? ?? '',
        metodo: json['metodo'] as String? ?? '',
        presente: json['presente'] as bool? ?? false,
        primeraVez: json['primeraVez'] as String? ?? '',
        ultimaVez: json['ultimaVez'] as String? ?? '',
        puertos: ((json['puertos'] as List<dynamic>?) ?? [])
            .map((fila) => PuertoEquipo.desdeJson(fila as Map<String, dynamic>))
            .toList(),
      );

  /// El nombre que conviene mostrar: manda el que puso una persona sobre el
  /// descubierto, y si no hay ninguno, la IP.
  String get comoSeLlama {
    if (alias.isNotEmpty) return alias;
    if (nombre.isNotEmpty) return nombre;
    return ip;
  }

  /// Que tan seguro es que este equipo exista tal como se ve.
  String get certeza {
    switch (metodo) {
      case 'arp':
        return 'Confirmado por ARP';
      case 'icmp':
        return 'Contesto ping';
      case 'tcp':
        return 'Solo contesto un puerto TCP';
      default:
        return '';
    }
  }
}

class PuertoEquipo {
  final int numero;
  final String protocolo;
  final String servicio;
  final String banner;

  const PuertoEquipo({
    required this.numero,
    required this.protocolo,
    required this.servicio,
    required this.banner,
  });

  factory PuertoEquipo.desdeJson(Map<String, dynamic> json) => PuertoEquipo(
        numero: json['numero'] as int,
        protocolo: json['protocolo'] as String? ?? 'tcp',
        servicio: json['servicio'] as String? ?? '',
        banner: json['banner'] as String? ?? '',
      );

  String get etiqueta => servicio.isEmpty ? '$numero' : '$numero $servicio';
}

class Escaneo {
  final int id;
  final String tipo;
  final String iniciado;
  final String? terminado;
  final String estado;
  final int vistos;
  final int nuevos;
  final String? detalle;

  const Escaneo({
    required this.id,
    required this.tipo,
    required this.iniciado,
    this.terminado,
    required this.estado,
    required this.vistos,
    required this.nuevos,
    this.detalle,
  });

  factory Escaneo.desdeJson(Map<String, dynamic> json) => Escaneo(
        id: json['id'] as int,
        tipo: json['tipo'] as String? ?? '',
        iniciado: json['iniciado'] as String? ?? '',
        terminado: json['terminado'] as String?,
        estado: json['estado'] as String? ?? '',
        vistos: json['vistos'] as int? ?? 0,
        nuevos: json['nuevos'] as int? ?? 0,
        detalle: json['detalle'] as String?,
      );

  bool get enCurso => estado == 'en_curso';
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

/// Un renglon del mapa de puertos: que hay conectado en una boca de un switch.
class PuertoDeSwitch {
  final int switchId;
  final String switchNombre;
  final String switchIp;
  final int indice;
  final String puerto;
  final String alias;
  final bool activa;
  final int velocidadMbps;
  final String mac;
  final int? equipoId;
  final String equipoNombre;
  final String equipoIp;
  final bool confirmado;
  final int cuantosEnBoca;

  const PuertoDeSwitch({
    required this.switchId,
    required this.switchNombre,
    required this.switchIp,
    required this.indice,
    required this.puerto,
    required this.alias,
    required this.activa,
    required this.velocidadMbps,
    required this.mac,
    this.equipoId,
    required this.equipoNombre,
    required this.equipoIp,
    required this.confirmado,
    required this.cuantosEnBoca,
  });

  factory PuertoDeSwitch.desdeJson(Map<String, dynamic> json) => PuertoDeSwitch(
        switchId: json['switchId'] as int,
        switchNombre: json['switchNombre'] as String? ?? '',
        switchIp: json['switchIp'] as String? ?? '',
        indice: json['indice'] as int? ?? 0,
        puerto: json['puerto'] as String? ?? '',
        alias: json['alias'] as String? ?? '',
        activa: json['activa'] as bool? ?? false,
        velocidadMbps: json['velocidadMbps'] as int? ?? 0,
        mac: json['mac'] as String? ?? '',
        equipoId: json['equipoId'] as int?,
        equipoNombre: json['equipoNombre'] as String? ?? '',
        equipoIp: json['equipoIp'] as String? ?? '',
        confirmado: json['confirmado'] as bool? ?? false,
        cuantosEnBoca: json['cuantosEnBoca'] as int? ?? 1,
      );

  String get quienEs =>
      equipoNombre.isNotEmpty ? equipoNombre : (equipoIp.isNotEmpty ? equipoIp : mac);
}

/// El mapa de puertos de una red, con que tan seguro es en este sitio.
class MapaPuertos {
  final String capacidad;
  final String explicacion;
  final List<PuertoDeSwitch> puertos;

  const MapaPuertos({
    required this.capacidad,
    required this.explicacion,
    required this.puertos,
  });

  factory MapaPuertos.desdeJson(Map<String, dynamic> json) => MapaPuertos(
        capacidad: json['capacidad'] as String? ?? 'desconocida',
        explicacion: json['explicacion'] as String? ?? '',
        puertos: ((json['puertos'] as List<dynamic>?) ?? [])
            .map((fila) => PuertoDeSwitch.desdeJson(fila as Map<String, dynamic>))
            .toList(),
      );

  bool get hayMapa => puertos.isNotEmpty;
}

/// Una credencial para hablarle a los switches por SNMP.
class CredencialSNMP {
  final int id;
  final String nombre;
  final String version;
  final String usuario;
  final String creada;

  const CredencialSNMP({
    required this.id,
    required this.nombre,
    required this.version,
    required this.usuario,
    required this.creada,
  });

  factory CredencialSNMP.desdeJson(Map<String, dynamic> json) => CredencialSNMP(
        id: json['id'] as int,
        nombre: json['nombre'] as String? ?? '',
        version: json['version'] as String? ?? '',
        usuario: json['usuario'] as String? ?? '',
        creada: json['creada'] as String? ?? '',
      );
}

/// Un aviso generado por el motor de alertas.
class Alerta {
  final int id;
  final String tipo;
  final String momento;
  final int? equipoId;
  final String titulo;
  final String detalle;
  final bool vista;
  final bool enviada;

  const Alerta({
    required this.id,
    required this.tipo,
    required this.momento,
    this.equipoId,
    required this.titulo,
    required this.detalle,
    required this.vista,
    required this.enviada,
  });

  factory Alerta.desdeJson(Map<String, dynamic> json) => Alerta(
        id: json['id'] as int,
        tipo: json['tipo'] as String? ?? '',
        momento: json['momento'] as String? ?? '',
        equipoId: json['equipoId'] as int?,
        titulo: json['titulo'] as String? ?? '',
        detalle: json['detalle'] as String? ?? '',
        vista: json['vista'] as bool? ?? false,
        enviada: json['enviada'] as bool? ?? false,
      );
}

/// Que se vigila en una red y con que umbral.
class Regla {
  final String tipo;
  final bool activa;
  final int umbral;

  const Regla({required this.tipo, required this.activa, required this.umbral});

  factory Regla.desdeJson(Map<String, dynamic> json) => Regla(
        tipo: json['tipo'] as String? ?? '',
        activa: json['activa'] as bool? ?? false,
        umbral: json['umbral'] as int? ?? 0,
      );

  /// Como se le dice a esta regla en lenguaje de persona.
  String get comoSeLlama {
    switch (tipo) {
      case 'equipo_nuevo':
        return 'Se conecto un equipo desconocido';
      case 'equipo_ausente':
        return 'Un equipo conocido lleva tiempo sin aparecer';
      case 'puerto_nuevo':
        return 'A un equipo conocido le abrieron un puerto';
      case 'cambio_ip':
        return 'Un equipo cambio de direccion';
      case 'cambio_puerto_switch':
        return 'Un equipo se cambio de puerto del switch';
      case 'red_sin_reportar':
        return 'La red entera dejo de reportar';
      default:
        return tipo;
    }
  }

  bool get usaUmbral => tipo == 'equipo_ausente' || tipo == 'red_sin_reportar';
}

/// A donde se avisa.
class DestinoAlerta {
  final int id;
  final String nombre;
  final String tipo;
  final String destino;
  final bool activo;
  final String ultimoEnvio;
  final String ultimoError;

  const DestinoAlerta({
    required this.id,
    required this.nombre,
    required this.tipo,
    required this.destino,
    required this.activo,
    required this.ultimoEnvio,
    required this.ultimoError,
  });

  factory DestinoAlerta.desdeJson(Map<String, dynamic> json) => DestinoAlerta(
        id: json['id'] as int,
        nombre: json['nombre'] as String? ?? '',
        tipo: json['tipo'] as String? ?? '',
        destino: json['destino'] as String? ?? '',
        activo: json['activo'] as bool? ?? true,
        ultimoEnvio: json['ultimoEnvio'] as String? ?? '',
        ultimoError: json['ultimoError'] as String? ?? '',
      );
}

/// Cuanto gasta una boca de switch y quien cuelga de ella.
class ConsumoDePuerto {
  final int switchId;
  final String switchNombre;
  final int indice;
  final String puerto;
  final String equipoNombre;
  final String equipoIp;
  final bool confirmado;
  final int cuantosEnBoca;
  final int bpsEntrada;
  final int bpsSalida;
  final String momento;

  const ConsumoDePuerto({
    required this.switchId,
    required this.switchNombre,
    required this.indice,
    required this.puerto,
    required this.equipoNombre,
    required this.equipoIp,
    required this.confirmado,
    required this.cuantosEnBoca,
    required this.bpsEntrada,
    required this.bpsSalida,
    required this.momento,
  });

  factory ConsumoDePuerto.desdeJson(Map<String, dynamic> json) => ConsumoDePuerto(
        switchId: json['switchId'] as int,
        switchNombre: json['switchNombre'] as String? ?? '',
        indice: json['indice'] as int? ?? 0,
        puerto: json['puerto'] as String? ?? '',
        equipoNombre: json['equipoNombre'] as String? ?? '',
        equipoIp: json['equipoIp'] as String? ?? '',
        confirmado: json['confirmado'] as bool? ?? false,
        cuantosEnBoca: json['cuantosEnBoca'] as int? ?? 0,
        bpsEntrada: json['bpsEntrada'] as int? ?? 0,
        bpsSalida: json['bpsSalida'] as int? ?? 0,
        momento: json['momento'] as String? ?? '',
      );

  int get total => bpsEntrada + bpsSalida;

  String get quienEs {
    if (equipoNombre.isNotEmpty) return equipoNombre;
    if (cuantosEnBoca > 1) return '$cuantosEnBoca equipos en esta boca';
    return puerto;
  }

  /// Los bits por segundo en algo que una persona lea de un vistazo.
  static String enPalabras(int bps) {
    if (bps >= 1000000000) return '${(bps / 1000000000).toStringAsFixed(2)} Gbps';
    if (bps >= 1000000) return '${(bps / 1000000).toStringAsFixed(1)} Mbps';
    if (bps >= 1000) return '${(bps / 1000).toStringAsFixed(0)} kbps';
    return '$bps bps';
  }
}

/// El consumo de una red, con que tan fiable es en este sitio.
class Consumo {
  final String explicacion;

  /// Lo medido con los contadores del switch: dice en que boca esta cada quien.
  final List<ConsumoDePuerto> puertos;

  /// Lo medido con los flujos del router: no dice la boca, pero funciona sin
  /// switches administrables.
  final List<ConsumoDePuerto> porFlujos;

  const Consumo({
    required this.explicacion,
    required this.puertos,
    required this.porFlujos,
  });

  factory Consumo.desdeJson(Map<String, dynamic> json) => Consumo(
        explicacion: json['explicacion'] as String? ?? '',
        puertos: ((json['consumo'] as List<dynamic>?) ?? [])
            .map((fila) => ConsumoDePuerto.desdeJson(fila as Map<String, dynamic>))
            .toList(),
        porFlujos: ((json['porFlujos'] as List<dynamic>?) ?? [])
            .map((fila) => ConsumoDePuerto.desdeJson(fila as Map<String, dynamic>))
            .toList(),
      );

  bool get hayAlgo => puertos.isNotEmpty || porFlujos.isNotEmpty;
}
