/// Los datos que viajan entre la API y la interfaz.
///
/// Los nombres de los campos son los mismos que en Go y en la base: si algo se
/// llama distinto en cada capa, tarde o temprano alguien mapea mal un campo.
library;

import 'tipos_de_puerto.dart';

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

  /// Lo que escribe una persona. No sale de ningun barrido.
  final String modelo;
  final String notas;

  /// Donde ESTA el aparato: «farmacia», «cons 5», «rack del site».
  ///
  /// No es lo mismo que de donde cuelga. Un aparato puede colgar del puerto 7 del
  /// switch y estar en el consultorio 4: lo primero es el cableado y lo segundo
  /// es el sitio, y quien va a desconectarlo necesita el segundo.
  final String ubicacion;

  /// `descubierto` o `manual`. Un switch no administrable NUNCA va a salir en un
  /// escaneo —no tiene direccion—, asi que hay que poder distinguir "no
  /// contesto" de "no existe".
  final String origen;

  /// `cable` o `wifi`. Solo aplica a equipos terminales; vacio en un switch.
  final String conexion;

  /// La clave de la lista unica de categorias (ver `categorias.dart`).
  ///
  /// No es lo mismo que [tipo]: `tipo` es el nombre para LEER («Impresora HP») y
  /// `categoria` es para CONTAR («impresora»). Agrupando por `tipo` salen cubos
  /// separados para «Impresora HP» y «Impresora de red».
  final String categoria;

  /// Lo que el aparato conto DE SI MISMO: el titulo de su pagina, el nombre de
  /// su certificado, lo que anuncia por mDNS o UPnP y lo que contesta al
  /// protocolo de su fabricante. Cada dato trae de que fuente salio.
  final List<DatoHuella> huella;

  /// Como se entra a este aparato. **Sin la clave**: aqui viene el usuario y la
  /// direccion del panel, y la clave solo cuando alguien la pide.
  final List<CredencialEquipo> credenciales;

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
    this.modelo = '',
    this.ubicacion = '',
    this.notas = '',
    this.origen = 'descubierto',
    this.conexion = '',
    this.categoria = '',
    this.huella = const [],
    this.credenciales = const [],
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
        modelo: json['modelo'] as String? ?? '',
        ubicacion: json['ubicacion'] as String? ?? '',
        notas: json['notas'] as String? ?? '',
        origen: json['origen'] as String? ?? 'descubierto',
        conexion: json['conexion'] as String? ?? '',
        categoria: json['categoria'] as String? ?? '',
        huella: ((json['huella'] as List<dynamic>?) ?? [])
            .map((fila) => DatoHuella.desdeJson(fila as Map<String, dynamic>))
            .toList(),
        credenciales: ((json['credenciales'] as List<dynamic>?) ?? [])
            .map((fila) => CredencialEquipo.desdeJson(fila as Map<String, dynamic>))
            .toList(),
      );

  /// El nombre que conviene mostrar: manda el que puso una persona sobre el
  /// descubierto, y si no hay ninguno, la IP.
  String get comoSeLlama {
    if (alias.isNotEmpty) return alias;
    if (nombre.isNotEmpty) return nombre;
    if (ip.isNotEmpty) return ip;
    // Un equipo declarado a mano puede no tener direccion ninguna: un switch
    // tonto no la tiene. Sin esto se veria un renglon en blanco.
    return 'Equipo $id';
  }

  /// Lo declaro una persona, no lo encontro ningun barrido.
  bool get esManual => origen == 'manual';

  /// Si tiene sentido guardarle credenciales de administracion.
  ///
  /// **Sin direccion no hay panel al que entrar.** Un switch no administrable no
  /// tiene direccion —por eso hay que declararlo a mano— y ofrecerle un
  /// formulario de usuario y clave seria ofrecer algo que no existe. Lo mismo
  /// con cualquier aparato que solo se vio de pasada y no contesta en ningun
  /// lado.
  bool get puedeAdministrarse =>
      ip.isNotEmpty && categoria != 'switch_simple';

  /// El modelo que dijo el propio aparato, si alguna fuente lo dijo. Se prefiere
  /// al que teclee una persona solo para MOSTRARLO: el tecleado sigue mandando
  /// en la ficha, porque quien tiene el aparato delante sabe mas.
  String get modeloDicho {
    for (final dato in huella) {
      if (dato.clave == 'modelo') return dato.valor;
    }
    return '';
  }

  /// La red inalambrica que emite, cuando el aparato la anuncia.
  String get redQueEmite {
    for (final dato in huella) {
      if (dato.clave == 'red') return dato.valor;
    }
    return '';
  }

  /// Que tan seguro es que este equipo exista tal como se ve.
  String get certeza {
    switch (metodo) {
      case 'propio':
        return 'Este equipo, donde corre MiRed';
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

/// Un dato que el aparato conto de si mismo, y de donde se supo.
///
/// La fuente se guarda y se muestra a proposito: no vale lo mismo un modelo
/// firmado en un certificado que uno sacado del titulo de una pagina, y la
/// interfaz tiene que poder decirlo en vez de presentarlo todo igual de firme.
class DatoHuella {
  final String fuente;
  final String clave;
  final String valor;

  const DatoHuella({required this.fuente, required this.clave, required this.valor});

  factory DatoHuella.desdeJson(Map<String, dynamic> json) => DatoHuella(
        fuente: json['fuente'] as String? ?? '',
        clave: json['clave'] as String? ?? '',
        valor: json['valor'] as String? ?? '',
      );

  /// Como se lee la fuente en pantalla.
  String get comoSeLlamaLaFuente {
    switch (fuente) {
      case 'web':
        return 'su pagina web';
      case 'certificado':
        return 'su certificado';
      case 'ssh':
        return 'su SSH';
      case 'mdns':
        return 'lo que anuncia por Bonjour';
      case 'ssdp':
        return 'lo que anuncia por UPnP';
      case 'netbios':
        return 'su nombre de Windows';
      case 'onvif':
        return 'el protocolo de camaras ONVIF';
      case 'ubiquiti':
        return 'el protocolo de Ubiquiti';
      case 'mikrotik':
        return 'el protocolo de MikroTik';
      case 'tplink':
        return 'el protocolo de TP-Link';
      case 'hikvision':
        return 'el protocolo de Hikvision';
      case 'roku':
        return 'el protocolo de Roku';
      case 'television':
        return 'la ficha de la television';
      case 'impresora':
        return 'su panel de impresion';
      default:
        return fuente;
    }
  }

  /// Como se lee lo que se supo.
  String get comoSeLlamaLaClave {
    switch (clave) {
      case 'nombre':
        return 'Nombre';
      case 'modelo':
        return 'Modelo';
      case 'fabricante':
        return 'Fabricante';
      case 'firmware':
        return 'Version';
      case 'serie':
        return 'Numero de serie';
      case 'titulo':
        return 'Titulo de su pagina';
      case 'servidor':
        return 'Servidor web';
      case 'realm':
        return 'Pide clave para';
      case 'mac':
        return 'MAC que declara';
      case 'servicio':
        return 'Ofrece';
      case 'red':
        return 'Red WiFi que emite';
      default:
        return clave;
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

/// Un renglon del mapa de puertos: que hay conectado en un puerto de un switch.
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
  final int cuantosEnPuerto;

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
    required this.cuantosEnPuerto,
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
        cuantosEnPuerto: json['cuantosEnPuerto'] as int? ?? 1,
      );

  String get quienEs =>
      equipoNombre.isNotEmpty ? equipoNombre : (equipoIp.isNotEmpty ? equipoIp : mac);
}

/// Un cable de switch a switch, anunciado por LLDP o por CDP.
///
/// Es lo que convierte una lista de switches sueltos en un arbol: sin esto se
/// sabe que existen, pero no cual cuelga de cual.
class EnlaceEntreEquipos {
  final int equipoId;
  final String equipoNombre;
  final String interfazLocal;
  final String vecinoNombre;
  final String vecinoPuerto;
  final int? vecinoId;

  /// `lldp` o `cdp`. Se conserva para poder decir de donde salio el dato: los
  /// dos protocolos ven el mismo cable y lo describen distinto.
  final String origen;

  const EnlaceEntreEquipos({
    required this.equipoId,
    required this.equipoNombre,
    required this.interfazLocal,
    required this.vecinoNombre,
    required this.vecinoPuerto,
    this.vecinoId,
    required this.origen,
  });

  factory EnlaceEntreEquipos.desdeJson(Map<String, dynamic> json) => EnlaceEntreEquipos(
        equipoId: json['equipoId'] as int,
        equipoNombre: json['equipoNombre'] as String? ?? '',
        interfazLocal: json['interfazLocal'] as String? ?? '',
        vecinoNombre: json['vecinoNombre'] as String? ?? '',
        vecinoPuerto: json['vecinoPuerto'] as String? ?? '',
        vecinoId: json['vecinoId'] as int?,
        origen: json['origen'] as String? ?? 'lldp',
      );
}

/// El mapa de puertos de una red, con que tan seguro es en este sitio.
class MapaPuertos {
  final String capacidad;
  final String explicacion;
  final List<PuertoDeSwitch> puertos;
  final List<EnlaceEntreEquipos> enlaces;

  /// De cuando son estos datos, en ISO. **No es cuando se exporto**: un mapa se
  /// puede guardar hoy con datos de hace tres semanas, y sin esto el archivo
  /// llevaria la fecha de hoy y pareceria al dia.
  final String momento;

  const MapaPuertos({
    required this.capacidad,
    required this.explicacion,
    required this.puertos,
    this.enlaces = const [],
    this.momento = '',
  });

  factory MapaPuertos.desdeJson(Map<String, dynamic> json) => MapaPuertos(
        capacidad: json['capacidad'] as String? ?? 'desconocida',
        explicacion: json['explicacion'] as String? ?? '',
        puertos: ((json['puertos'] as List<dynamic>?) ?? [])
            .map((fila) => PuertoDeSwitch.desdeJson(fila as Map<String, dynamic>))
            .toList(),
        enlaces: ((json['enlaces'] as List<dynamic>?) ?? [])
            .map((fila) => EnlaceEntreEquipos.desdeJson(fila as Map<String, dynamic>))
            .toList(),
        momento: json['momento'] as String? ?? '',
      );

  bool get hayMapa => puertos.isNotEmpty;

  /// enlacesUnicos deja un renglon por cable, sin repetir el mismo visto por los
  /// dos protocolos, y dice cuales lo confirmaron.
  ///
  /// Un switch que habla LLDP y CDP anuncia cada cable dos veces. Dibujarlo dos
  /// veces no agrega informacion: lo que si agrega es **que los dos coincidan**,
  /// porque un cable confirmado por ambos protocolos es un dato mas firme que
  /// uno que solo vio uno.
  List<EnlaceDibujable> get enlacesUnicos {
    final porCable = <String, EnlaceDibujable>{};
    for (final enlace in enlaces) {
      final clave = '${enlace.equipoId}|${enlace.interfazLocal}';
      final existente = porCable[clave];
      if (existente == null) {
        porCable[clave] = EnlaceDibujable(enlace: enlace, origenes: {enlace.origen});
      } else {
        existente.origenes.add(enlace.origen);
        // Se prefiere el que si supo a que equipo apunta: entre un nombre suelto
        // y un enlace con destino conocido, el segundo dibuja algo.
        if (existente.enlace.vecinoId == null && enlace.vecinoId != null) {
          porCable[clave] = EnlaceDibujable(enlace: enlace, origenes: existente.origenes);
        }
      }
    }
    return porCable.values.toList();
  }
}

/// De que esta hecha una red: cuantos aparatos hay y cuantos de cada tipo.
///
/// Sale de la MISMA tabla que la lista de equipos y que el mapa, asi que los tres
/// no pueden discrepar. Un switch declarado a mano cuenta aqui en cuanto se
/// declara: no hay nada que sincronizar ni que verificar despues.
class ComposicionDeRed {
  final int total;
  final int presentes;

  /// Cuantos los puso una persona. Se dice aparte porque no los vio ningun
  /// escaneo: la cuenta es igual de real, pero no viene de una medicion.
  final int declarados;
  final List<CuentaPorCategoria> categorias;

  const ComposicionDeRed({
    this.total = 0,
    this.presentes = 0,
    this.declarados = 0,
    this.categorias = const [],
  });

  factory ComposicionDeRed.desdeJson(Map<String, dynamic> json) => ComposicionDeRed(
        total: json['total'] as int? ?? 0,
        presentes: json['presentes'] as int? ?? 0,
        declarados: json['declarados'] as int? ?? 0,
        categorias: ((json['categorias'] as List<dynamic>?) ?? [])
            .map((fila) => CuentaPorCategoria.desdeJson(fila as Map<String, dynamic>))
            .toList(),
      );
}

/// Cuantos aparatos de un tipo hay en la red.
class CuentaPorCategoria {
  final String categoria;
  final int cuantos;
  final int presentes;
  final int declarados;

  const CuentaPorCategoria({
    required this.categoria,
    required this.cuantos,
    this.presentes = 0,
    this.declarados = 0,
  });

  factory CuentaPorCategoria.desdeJson(Map<String, dynamic> json) => CuentaPorCategoria(
        categoria: json['categoria'] as String? ?? '',
        cuantos: json['cuantos'] as int? ?? 0,
        presentes: json['presentes'] as int? ?? 0,
        declarados: json['declarados'] as int? ?? 0,
      );
}

/// Un puerto fisico de un equipo, contada mirando el aparato.
///
/// No es lo mismo que las interfaces que anuncia un switch por SNMP: aquellas
/// solo existen si el equipo habla. Un switch de ocho puertos de cien pesos no
/// habla, no tiene direccion, y aun asi tiene ocho puertos.
class PuertoFisico {
  final int id;
  final int equipoId;
  final int numero;

  /// Uno de [tiposDePuerto]: `lan`, `wan`, `dmz`, `sfp` o `consola`.
  final String tipo;

  /// Nulo cuando no se sabe. Mejor vacio que un 100 inventado que despues
  /// alguien lea como medido.
  final int? velocidadMbps;
  final String notas;

  const PuertoFisico({
    required this.id,
    required this.equipoId,
    required this.numero,
    required this.tipo,
    this.velocidadMbps,
    this.notas = '',
  });

  factory PuertoFisico.desdeJson(Map<String, dynamic> json) => PuertoFisico(
        id: json['id'] as int,
        equipoId: json['equipoId'] as int,
        numero: json['numero'] as int,
        tipo: json['tipo'] as String? ?? 'lan',
        velocidadMbps: json['velocidadMbps'] as int?,
        notas: json['notas'] as String? ?? '',
      );

  /// Como se llama el puerto: `LAN 3`, `WAN 1`, `DMZ 1`.
  ///
  /// Nunca "puerto 3" a secas: en un modem con LAN, WAN y DMZ eso no dice por
  /// donde sale el cable, que es justo lo que se va a mirar en el mapa.
  String get etiqueta => nombreDePuerto(tipo, numero);
}

/// Un cable con las dos puntas ya resueltas, venga de donde venga el dato.
class EnlaceFisico {
  final int id;
  final int puertoOrigenId;
  final int equipoOrigenId;
  final int numeroOrigen;
  final String origenNombre;
  final int? puertoDestinoId;
  final int? equipoDestinoId;
  final int numeroDestino;
  final String destinoNombre;

  /// `manual`, `snmp`, `lldp`, `cdp` o `inferido`. Es lo que permite dibujar
  /// distinto lo tecleado de lo medido en vez de mezclarlo y presentarlo todo
  /// como verdad sin procedencia.
  final String origenDato;
  final String notas;

  const EnlaceFisico({
    required this.id,
    required this.puertoOrigenId,
    required this.equipoOrigenId,
    required this.numeroOrigen,
    required this.origenNombre,
    this.puertoDestinoId,
    this.equipoDestinoId,
    this.numeroDestino = 0,
    required this.destinoNombre,
    required this.origenDato,
    this.notas = '',
  });

  factory EnlaceFisico.desdeJson(Map<String, dynamic> json) => EnlaceFisico(
        id: json['id'] as int,
        puertoOrigenId: json['puertoOrigenId'] as int,
        equipoOrigenId: json['equipoOrigenId'] as int? ?? 0,
        numeroOrigen: json['numeroOrigen'] as int? ?? 0,
        origenNombre: json['origenNombre'] as String? ?? '',
        puertoDestinoId: json['puertoDestinoId'] as int?,
        equipoDestinoId: json['equipoDestinoId'] as int?,
        numeroDestino: json['numeroDestino'] as int? ?? 0,
        destinoNombre: json['destinoNombre'] as String? ?? '',
        origenDato: json['origenDato'] as String? ?? 'manual',
        notas: json['notas'] as String? ?? '',
      );

  bool get esManual => origenDato == 'manual';
}

/// Un tramo donde lo declarado a mano y lo que reporta el equipo no coinciden.
///
/// No se pisa ninguno de los dos: se muestran las dos versiones y se pregunta.
class Contradiccion {
  final int enlaceId;
  final int equipoId;
  final String equipoNombre;
  final int numero;
  final String declarado;
  final String medido;
  final String fuente;
  final String momento;

  const Contradiccion({
    required this.enlaceId,
    required this.equipoId,
    required this.equipoNombre,
    required this.numero,
    required this.declarado,
    required this.medido,
    required this.fuente,
    required this.momento,
  });

  factory Contradiccion.desdeJson(Map<String, dynamic> json) => Contradiccion(
        enlaceId: json['enlaceId'] as int,
        equipoId: json['equipoId'] as int,
        equipoNombre: json['equipoNombre'] as String? ?? '',
        numero: json['numero'] as int? ?? 0,
        declarado: json['declarado'] as String? ?? '',
        medido: json['medido'] as String? ?? '',
        fuente: json['fuente'] as String? ?? '',
        momento: json['momento'] as String? ?? '',
      );
}

/// Todo lo que se declaro a mano en una red, mas donde eso ya no cuadra con lo
/// que reportan los equipos.
/// Un equipo colgado de una antena por el aire.
///
/// El WiFi no tiene puertos: de una antena cuelgan uno o VARIOS equipos, sin
/// inventarle un puerto a cada uno. La regla va al reves: un equipo cuelga de
/// UNA antena a la vez, porque asi funciona el WiFi.
class EnlaceInalambrico {
  final int id;
  final int equipoId;
  final int antenaId;
  final String equipoNombre;
  final String antenaNombre;

  /// El SSID, cuando se sabe.
  final String red;

  /// Con cuanta fuerza llega, en dBm. Nulo cuando se declaro a mano: una
  /// persona sabe de que antena cuelga su telefono, no con cuantos dBm llega.
  final int? senalDbm;

  /// `manual`, `snmp`, `airos` o `controladora`. Es lo que permite dibujar
  /// distinto lo tecleado de lo medido.
  final String origenDato;
  final String notas;
  final String ultimaVez;

  const EnlaceInalambrico({
    required this.id,
    required this.equipoId,
    required this.antenaId,
    this.equipoNombre = '',
    this.antenaNombre = '',
    this.red = '',
    this.senalDbm,
    this.origenDato = 'manual',
    this.notas = '',
    this.ultimaVez = '',
  });

  factory EnlaceInalambrico.desdeJson(Map<String, dynamic> json) => EnlaceInalambrico(
        id: json['id'] as int,
        equipoId: json['equipoId'] as int,
        antenaId: json['antenaId'] as int,
        equipoNombre: json['equipoNombre'] as String? ?? '',
        antenaNombre: json['antenaNombre'] as String? ?? '',
        red: json['red'] as String? ?? '',
        senalDbm: json['senalDbm'] as int?,
        origenDato: json['origenDato'] as String? ?? 'manual',
        notas: json['notas'] as String? ?? '',
        ultimaVez: json['ultimaVez'] as String? ?? '',
      );

  bool get esManual => origenDato == 'manual';

  /// Como se dice de donde salio este enlace. Lo declarado y lo medido no se
  /// presentan igual: eso es lo que separa un inventario de una suposicion.
  String get comoSeSupo {
    switch (origenDato) {
      case 'manual':
        return 'declarado a mano';
      case 'snmp':
        return 'lo dijo la antena por SNMP';
      case 'airos':
        return 'lo dijo el panel de la antena';
      case 'controladora':
        return 'lo dijo la controladora';
      default:
        return origenDato;
    }
  }
}

/// Como se entra a un aparato.
///
/// La clave **no viaja** en los listados: aqui llega vacia y [tieneClave] dice
/// si hay una guardada. Solo se pide cuando alguien la quiere ver, y eso queda
/// anotado en la bitacora del servidor.
class CredencialEquipo {
  final int id;
  final int equipoId;
  final String tipo;
  final String usuario;
  final String clave;
  final bool tieneClave;
  final String direccion;
  final String notas;

  const CredencialEquipo({
    this.id = 0,
    required this.equipoId,
    this.tipo = 'web',
    this.usuario = '',
    this.clave = '',
    this.tieneClave = false,
    this.direccion = '',
    this.notas = '',
  });

  factory CredencialEquipo.desdeJson(Map<String, dynamic> json) => CredencialEquipo(
        id: json['id'] as int? ?? 0,
        equipoId: json['equipoId'] as int? ?? 0,
        tipo: json['tipo'] as String? ?? 'web',
        usuario: json['usuario'] as String? ?? '',
        clave: json['clave'] as String? ?? '',
        tieneClave: json['tieneClave'] as bool? ?? false,
        direccion: json['direccion'] as String? ?? '',
        notas: json['notas'] as String? ?? '',
      );

  /// Como se lee el tipo en pantalla.
  String get comoSeLlamaElTipo {
    switch (tipo) {
      case 'web':
        return 'Panel web';
      case 'ssh':
        return 'Consola SSH';
      case 'consola':
        return 'Consola';
      case 'app':
        return 'Aplicacion del fabricante';
      default:
        return 'Otro acceso';
    }
  }
}

class TopologiaManual {
  final List<PuertoFisico> puertos;
  final List<EnlaceFisico> enlaces;
  final List<Contradiccion> contradicciones;
  /// Lo que cuelga de una antena por el aire.
  final List<EnlaceInalambrico> inalambricos;

  /// De cuando es lo mas reciente que se declaro.
  final String momento;

  const TopologiaManual({
    this.puertos = const [],
    this.enlaces = const [],
    this.contradicciones = const [],
    this.inalambricos = const [],
    this.momento = '',
  });

  factory TopologiaManual.desdeJson(Map<String, dynamic> json) => TopologiaManual(
        puertos: ((json['puertos'] as List<dynamic>?) ?? [])
            .map((fila) => PuertoFisico.desdeJson(fila as Map<String, dynamic>))
            .toList(),
        enlaces: ((json['enlaces'] as List<dynamic>?) ?? [])
            .map((fila) => EnlaceFisico.desdeJson(fila as Map<String, dynamic>))
            .toList(),
        contradicciones: ((json['contradicciones'] as List<dynamic>?) ?? [])
            .map((fila) => Contradiccion.desdeJson(fila as Map<String, dynamic>))
            .toList(),
        inalambricos: ((json['inalambricos'] as List<dynamic>?) ?? [])
            .map((fila) => EnlaceInalambrico.desdeJson(fila as Map<String, dynamic>))
            .toList(),
        momento: json['momento'] as String? ?? '',
      );

  bool get hayAlgo =>
      puertos.isNotEmpty || enlaces.isNotEmpty || inalambricos.isNotEmpty;

  /// Lo que cuelga de una antena por el aire.
  List<EnlaceInalambrico> inalambricosDe(int antenaId) =>
      inalambricos.where((enlace) => enlace.antenaId == antenaId).toList();

  /// De que antena cuelga un equipo, si cuelga de alguna.
  EnlaceInalambrico? antenaDe(int equipoId) {
    for (final enlace in inalambricos) {
      if (enlace.equipoId == equipoId) return enlace;
    }
    return null;
  }

  /// Los puertos de un equipo, ordenadas como se ven en el aparato.
  List<PuertoFisico> puertosDe(int equipoId) =>
      puertos.where((puerto) => puerto.equipoId == equipoId).toList();

  /// Los puertos de un equipo que todavia no llevan cable.
  ///
  /// Es lo que hay que ofrecer al conectar: un switch de 5 puertos con el
  /// uplink puesto tiene 4 libres, no 5.
  List<PuertoFisico> puertosLibresDe(int equipoId) =>
      puertosDe(equipoId).where((puerto) => enlaceDe(puerto.id) == null).toList();

  /// Un puerto, por su id. Sirve para nombrar la OTRA punta de un cable: sin
  /// esto el mapa solo podia decir de donde sale, no a donde entra.
  PuertoFisico? puertoPorId(int? puertoId) {
    if (puertoId == null) return null;
    for (final puerto in puertos) {
      if (puerto.id == puertoId) return puerto;
    }
    return null;
  }

  /// Como se llama el puerto por el que un aparato se conecta a la red, cuando
  /// se puede saber sin suponer de mas.
  ///
  /// Un aparato que no declaro ningun puerto es de punta —PC, laptop, TV,
  /// grabador— y entra a la red por su unica toma: [puertoUnicoDeUnEquipoFinal].
  /// Uno que SI declaro puertos tiene varios y hay que mirar cable por cable:
  /// ahi se devuelve vacio en vez de senalar uno al azar.
  String? puertoUnicoDe(int equipoId) =>
      puertosDe(equipoId).isEmpty ? puertoUnicoDeUnEquipoFinal : null;

  /// El cable que llega a un aparato que no tiene puertos declarados.
  ///
  /// Es como cuelga de la red un equipo de punta: el cable lo declaro el del
  /// otro lado —el switch— apuntando al aparato entero, porque este no tiene
  /// puertos que senalar.
  EnlaceFisico? cableHaciaElEquipo(int equipoId) {
    for (final enlace in enlaces) {
      if (enlace.equipoDestinoId == equipoId) return enlace;
    }
    return null;
  }

  /// De que equipo es un puerto.
  int? equipoDelPuerto(int puertoId) {
    for (final puerto in puertos) {
      if (puerto.id == puertoId) return puerto.equipoId;
    }
    return null;
  }

  /// El cable que sale de un puerto, si hay alguno.
  EnlaceFisico? enlaceDe(int puertoId) {
    for (final enlace in enlaces) {
      if (enlace.puertoOrigenId == puertoId || enlace.puertoDestinoId == puertoId) {
        return enlace;
      }
    }
    return null;
  }
}

/// Un cable ya sin repetir, con la lista de protocolos que lo vieron.
class EnlaceDibujable {
  EnlaceDibujable({required this.enlace, required this.origenes});

  final EnlaceEntreEquipos enlace;
  final Set<String> origenes;

  /// Confirmado por los dos protocolos: el dato mas firme que hay de un cable.
  bool get porAmbos => origenes.length > 1;
}

/// Una controladora WiFi (UniFi y compatibles).
///
/// Un punto de acceso no tiene puertos: tiene antenas, y quien sabe que aparato
/// esta colgado de cual es la controladora, no el aparato.
class Controladora {
  final int id;
  final String nombre;
  final String tipo;
  final String url;
  final String usuario;
  final String sitio;
  final bool verificarTls;
  final String creada;

  /// Cuando contesto por ultima vez y que dijo. Que una controladora lleve dias
  /// sin contestar es un dato, no un silencio.
  final String ultimoExito;
  final String ultimoError;

  const Controladora({
    required this.id,
    required this.nombre,
    required this.tipo,
    required this.url,
    required this.usuario,
    required this.sitio,
    required this.verificarTls,
    required this.creada,
    this.ultimoExito = '',
    this.ultimoError = '',
  });

  factory Controladora.desdeJson(Map<String, dynamic> json) => Controladora(
        id: json['id'] as int,
        nombre: json['nombre'] as String? ?? '',
        tipo: json['tipo'] as String? ?? 'unifi',
        url: json['url'] as String? ?? '',
        usuario: json['usuario'] as String? ?? '',
        sitio: json['sitio'] as String? ?? 'default',
        verificarTls: json['verificarTls'] as bool? ?? false,
        creada: json['creada'] as String? ?? '',
        ultimoExito: json['ultimoExito'] as String? ?? '',
        ultimoError: json['ultimoError'] as String? ?? '',
      );
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

/// Cuanto gasta un puerto de switch y quien cuelga de ella.
class ConsumoDePuerto {
  final int switchId;
  final String switchNombre;
  final int indice;
  final String puerto;
  final String equipoNombre;
  final String equipoIp;
  final bool confirmado;
  final int cuantosEnPuerto;
  final int bpsEntrada;
  final int bpsSalida;
  final String momento;

  /// La cifra viene de un muestreo (sFlow) y no de una cuenta exacta.
  ///
  /// Se arrastra desde el receptor hasta aqui a proposito: presentar una
  /// estimacion como si fuera una medicion es la clase de mentira silenciosa
  /// que este proyecto evita en el mapa, y aqui vale igual.
  final bool estimado;

  const ConsumoDePuerto({
    required this.switchId,
    required this.switchNombre,
    required this.indice,
    required this.puerto,
    required this.equipoNombre,
    required this.equipoIp,
    required this.confirmado,
    required this.cuantosEnPuerto,
    required this.bpsEntrada,
    required this.bpsSalida,
    required this.momento,
    this.estimado = false,
  });

  factory ConsumoDePuerto.desdeJson(Map<String, dynamic> json) => ConsumoDePuerto(
        switchId: json['switchId'] as int,
        switchNombre: json['switchNombre'] as String? ?? '',
        indice: json['indice'] as int? ?? 0,
        puerto: json['puerto'] as String? ?? '',
        equipoNombre: json['equipoNombre'] as String? ?? '',
        equipoIp: json['equipoIp'] as String? ?? '',
        confirmado: json['confirmado'] as bool? ?? false,
        cuantosEnPuerto: json['cuantosEnPuerto'] as int? ?? 0,
        bpsEntrada: json['bpsEntrada'] as int? ?? 0,
        bpsSalida: json['bpsSalida'] as int? ?? 0,
        momento: json['momento'] as String? ?? '',
        estimado: json['estimado'] as bool? ?? false,
      );

  int get total => bpsEntrada + bpsSalida;

  String get quienEs {
    if (equipoNombre.isNotEmpty) return equipoNombre;
    if (cuantosEnPuerto > 1) return '$cuantosEnPuerto equipos en este puerto';
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

/// Lo que un equipo movio hablando con un destino concreto.
///
/// Lo llena el paquete OPCIONAL mired-dpi. Responde "en que se gasta el ancho de
/// banda", no solo "cuanto".
class ConsumoPorAplicacion {
  final String ip;
  final String equipo;
  final String aplicacion;

  /// De donde salio el nombre: `tls`, `http`, `dns`, `puerto` o `desconocido`.
  ///
  /// No todos valen igual y por eso se conserva: un nombre sacado del saludo de
  /// TLS dice con quien se hablo; uno sacado del puerto solo dice de que tipo de
  /// servicio se trata. Esconder la diferencia seria presentar una suposicion
  /// con la misma cara que un dato.
  final String como;

  final int bytes;
  final int paquetes;

  const ConsumoPorAplicacion({
    required this.ip,
    required this.equipo,
    required this.aplicacion,
    required this.como,
    required this.bytes,
    required this.paquetes,
  });

  factory ConsumoPorAplicacion.desdeJson(Map<String, dynamic> json) =>
      ConsumoPorAplicacion(
        ip: json['ip'] as String? ?? '',
        equipo: json['equipo'] as String? ?? '',
        aplicacion: json['aplicacion'] as String? ?? '',
        como: json['como'] as String? ?? 'desconocido',
        bytes: json['bytes'] as int? ?? 0,
        paquetes: json['paquetes'] as int? ?? 0,
      );

  /// De donde salio el nombre, dicho para una persona.
  String get procedencia {
    switch (como) {
      case 'tls':
        return 'nombre del servidor (TLS)';
      case 'http':
        return 'cabecera Host (HTTP)';
      case 'dns':
        return 'consulta de DNS';
      case 'puerto':
        return 'supuesto por el puerto';
      default:
        return 'no se pudo identificar';
    }
  }

  /// Los bytes en algo que una persona lea de un vistazo.
  String get enPalabras {
    const unidades = ['B', 'KB', 'MB', 'GB', 'TB'];
    var valor = bytes.toDouble();
    var unidad = 0;
    while (valor >= 1024 && unidad < unidades.length - 1) {
      valor /= 1024;
      unidad++;
    }
    return '${valor.toStringAsFixed(valor >= 100 || unidad == 0 ? 0 : 1)} ${unidades[unidad]}';
  }
}

/// El consumo de una red, con que tan fiable es en este sitio.
class Consumo {
  final String explicacion;

  /// Lo medido con los contadores del switch: dice en que puerto esta cada quien.
  final List<ConsumoDePuerto> puertos;

  /// Lo medido con los flujos del router: no dice el puerto, pero funciona sin
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

// ------------------------------------------------------------ importacion ---

/// Lo que se haria con un renglon de una hoja que se va a importar.
///
/// Viene del servidor **antes** de escribir nada: la pantalla lo ensena renglon
/// por renglon y quien importa decide. Importar a ciegas 23 aparatos y descubrir
/// despues que tres estaban mal significa borrarlos a mano de uno en uno.
class RenglonImportado {
  final int renglon;
  final String nombre;

  /// `crear`, `actualizar` o `rechazar`.
  final String accion;

  /// Por que se rechaza, en cristiano y diciendo que hacer.
  final String motivo;

  /// Se importa igual, pero conviene mirarlo.
  final String aviso;

  final String tipo;
  final String cuelgaDe;
  final String puerto;
  final String ubicacion;
  final String ip;

  const RenglonImportado({
    required this.renglon,
    required this.nombre,
    required this.accion,
    this.motivo = '',
    this.aviso = '',
    this.tipo = '',
    this.cuelgaDe = '',
    this.puerto = '',
    this.ubicacion = '',
    this.ip = '',
  });

  factory RenglonImportado.desdeJson(Map<String, dynamic> json) {
    // El puerto llega partido en tipo y numero, como se guarda; aqui se junta
    // como se lee en el aparato: «LAN 7».
    final numero = json['puertoNumero'] as int? ?? 0;
    final tipoPuerto = json['puertoTipo'] as String? ?? '';
    return RenglonImportado(
      renglon: json['renglon'] as int? ?? 0,
      nombre: json['nombre'] as String? ?? '',
      accion: json['accion'] as String? ?? 'rechazar',
      motivo: json['motivo'] as String? ?? '',
      aviso: json['aviso'] as String? ?? '',
      tipo: json['tipo'] as String? ?? '',
      cuelgaDe: json['cuelgaDe'] as String? ?? '',
      puerto: numero == 0 ? '' : nombreDePuerto(tipoPuerto, numero),
      ubicacion: json['ubicacion'] as String? ?? '',
      ip: json['ip'] as String? ?? '',
    );
  }

  bool get seRechaza => accion == 'rechazar';
  bool get seActualiza => accion == 'actualizar';
}

/// El plan entero: que se haria con el archivo, sin haber tocado nada.
class PlanImportacion {
  final List<RenglonImportado> renglones;

  /// Columnas del archivo que MiRed no entiende. Se dicen en vez de callarlas:
  /// quien capturo «RESPONSABLE» tiene que enterarse de que no se guardo.
  final List<String> ignoradas;
  final List<String> reconocidas;

  final int crear;
  final int actualizar;
  final int rechazar;
  final int cables;

  /// Cuantos renglones traen contrasena, para poder avisar de que el ARCHIVO la
  /// lleva en claro aunque en la base quede cifrada.
  final int conClave;

  const PlanImportacion({
    this.renglones = const [],
    this.ignoradas = const [],
    this.reconocidas = const [],
    this.crear = 0,
    this.actualizar = 0,
    this.rechazar = 0,
    this.cables = 0,
    this.conClave = 0,
  });

  factory PlanImportacion.desdeJson(Map<String, dynamic> json) => PlanImportacion(
        renglones: ((json['renglones'] as List<dynamic>?) ?? [])
            .map((fila) => RenglonImportado.desdeJson(fila as Map<String, dynamic>))
            .toList(),
        ignoradas: ((json['ignoradas'] as List<dynamic>?) ?? [])
            .map((valor) => valor as String)
            .toList(),
        reconocidas: ((json['reconocidas'] as List<dynamic>?) ?? [])
            .map((valor) => valor as String)
            .toList(),
        crear: json['crear'] as int? ?? 0,
        actualizar: json['actualizar'] as int? ?? 0,
        rechazar: json['rechazar'] as int? ?? 0,
        cables: json['cables'] as int? ?? 0,
        conClave: json['conClave'] as int? ?? 0,
      );

  bool get hayAlgoQueImportar => crear > 0 || actualizar > 0;
}

/// Lo que de verdad se hizo al importar.
class ResumenImportacion {
  final int creados;
  final int actualizados;
  final int saltados;
  final int rechazados;
  final int puertos;
  final int cables;
  final int credenciales;

  /// Puertos que ya llevaban un cable a otro sitio y se quedaron con el del
  /// archivo. Es el unico caso en que importar PISA un dato que ya estaba, asi
  /// que se dice aparte.
  final int recableados;

  const ResumenImportacion({
    this.creados = 0,
    this.actualizados = 0,
    this.saltados = 0,
    this.rechazados = 0,
    this.puertos = 0,
    this.cables = 0,
    this.credenciales = 0,
    this.recableados = 0,
  });

  factory ResumenImportacion.desdeJson(Map<String, dynamic> json) => ResumenImportacion(
        creados: json['creados'] as int? ?? 0,
        actualizados: json['actualizados'] as int? ?? 0,
        saltados: json['saltados'] as int? ?? 0,
        rechazados: json['rechazados'] as int? ?? 0,
        puertos: json['puertos'] as int? ?? 0,
        cables: json['cables'] as int? ?? 0,
        credenciales: json['credenciales'] as int? ?? 0,
        recableados: json['recableados'] as int? ?? 0,
      );

  /// Como se cuenta lo que paso, en una frase.
  String get enPalabras {
    final partes = <String>[
      if (creados > 0) '$creados ${creados == 1 ? 'aparato nuevo' : 'aparatos nuevos'}',
      if (actualizados > 0) '$actualizados actualizados',
      if (saltados > 0) '$saltados sin tocar',
      if (cables > 0) '$cables ${cables == 1 ? 'cable' : 'cables'}',
      if (puertos > 0) '$puertos puertos declarados',
      if (credenciales > 0) '$credenciales accesos guardados',
      if (recableados > 0) '$recableados puertos que ya tenian otro cable',
    ];
    if (partes.isEmpty) return 'No hubo nada que cambiar.';
    return '${partes.join(', ')}.';
  }
}

/// Una columna de la plantilla de importacion, tal como la define el servidor.
///
/// La guia de la pantalla se dibuja de esto y **no de una copia escrita a mano**:
/// el dia que se agregue una columna, la guia se entera sola. Dos listas
/// paralelas se separan siempre, y la de la pantalla se descubre equivocada
/// cuando alguien ya llenó la hoja.
class ColumnaPlantilla {
  final String clave;
  final bool obligatoria;
  final String ayuda;
  final String ejemplo;

  /// Los otros encabezados que tambien se aceptan para esta columna. Es lo que
  /// permite subir una hoja ajena sin renombrar nada.
  final List<String> sinonimos;

  const ColumnaPlantilla({
    required this.clave,
    this.obligatoria = false,
    this.ayuda = '',
    this.ejemplo = '',
    this.sinonimos = const [],
  });

  factory ColumnaPlantilla.desdeJson(Map<String, dynamic> json) => ColumnaPlantilla(
        clave: json['clave'] as String? ?? '',
        obligatoria: json['obligatoria'] as bool? ?? false,
        ayuda: json['ayuda'] as String? ?? '',
        ejemplo: json['ejemplo'] as String? ?? '',
        sinonimos: ((json['sinonimos'] as List<dynamic>?) ?? [])
            .map((valor) => valor as String)
            .toList(),
      );

  /// Los sinonimos que de verdad hay que decir: los que NO son el nombre de la
  /// columna. Repetir «NOMBRE se puede llamar NOMBRE» no ayuda a nadie.
  List<String> get otrosNombres =>
      sinonimos.where((nombre) => nombre != clave).toList();
}

/// Un valor que se puede escribir en la columna QUE_ES.
class CategoriaDeGuia {
  final String clave;
  final String comoSeLee;

  /// Otras formas de escribirlo que tambien se aceptan: «pc», «nvr», «ap».
  final List<String> apodos;

  const CategoriaDeGuia({
    required this.clave,
    required this.comoSeLee,
    this.apodos = const [],
  });

  factory CategoriaDeGuia.desdeJson(Map<String, dynamic> json) => CategoriaDeGuia(
        clave: json['clave'] as String? ?? '',
        comoSeLee: json['comoSeLee'] as String? ?? '',
        apodos: ((json['apodos'] as List<dynamic>?) ?? [])
            .map((valor) => valor as String)
            .toList(),
      );
}

/// La plantilla descargable y la guia para llenarla, las dos de la misma fuente.
class PlantillaImportacion {
  final String nombre;
  final String contenido;
  final List<ColumnaPlantilla> columnas;
  final List<CategoriaDeGuia> categorias;

  /// Renglones ya llenos: los MISMOS que trae el archivo descargable.
  final List<Map<String, String>> ejemplo;

  const PlantillaImportacion({
    this.nombre = 'mired-plantilla-aparatos.csv',
    this.contenido = '',
    this.columnas = const [],
    this.categorias = const [],
    this.ejemplo = const [],
  });

  factory PlantillaImportacion.desdeJson(Map<String, dynamic> json) {
    final guia = (json['guia'] as Map<String, dynamic>?) ?? const {};
    return PlantillaImportacion(
      nombre: json['nombre'] as String? ?? 'mired-plantilla-aparatos.csv',
      contenido: json['contenido'] as String? ?? '',
      columnas: ((guia['columnas'] as List<dynamic>?) ?? [])
          .map((fila) => ColumnaPlantilla.desdeJson(fila as Map<String, dynamic>))
          .toList(),
      categorias: ((guia['categorias'] as List<dynamic>?) ?? [])
          .map((fila) => CategoriaDeGuia.desdeJson(fila as Map<String, dynamic>))
          .toList(),
      ejemplo: ((guia['ejemplo'] as List<dynamic>?) ?? [])
          .map((fila) => (fila as Map<String, dynamic>)
              .map((clave, valor) => MapEntry(clave, valor as String)))
          .toList(),
    );
  }

  ColumnaPlantilla? porClave(String clave) {
    for (final columna in columnas) {
      if (columna.clave == clave) return columna;
    }
    return null;
  }
}

/// El resultado de probar una credencial SNMP contra los aparatos de la red.
///
/// Existe para que configurar SNMP no sea un acto de fe. Sin esto se guarda una
/// contrasena a ciegas y, si estaba mal, nadie se entera: el mapa se queda sin
/// puertos y no hay nada que diga por que.
class PruebaDeCredencial {
  /// Cuantos aparatos se consultaron y cuantos contestaron.
  final int consultados;
  final int contestaron;

  /// Los que contestaron, para poder decir cuales son por su nombre.
  final List<SwitchQueContesto> switches;

  /// El resultado contado como se lo diria una persona a otra. Lo arma el
  /// servidor: la frase depende de cuantos contestaron y de por que.
  final String explicacion;

  const PruebaDeCredencial({
    this.consultados = 0,
    this.contestaron = 0,
    this.switches = const [],
    this.explicacion = '',
  });

  factory PruebaDeCredencial.desdeJson(Map<String, dynamic> json) => PruebaDeCredencial(
        consultados: json['consultados'] as int? ?? 0,
        contestaron: json['contestaron'] as int? ?? 0,
        switches: ((json['switches'] as List<dynamic>?) ?? [])
            .map((fila) => SwitchQueContesto.desdeJson(fila as Map<String, dynamic>))
            .toList(),
        explicacion: json['explicacion'] as String? ?? '',
      );
}

/// Un aparato que contesto a la prueba.
class SwitchQueContesto {
  final String ip;
  final String nombre;
  final bool esSwitch;
  final int puertos;

  const SwitchQueContesto({
    required this.ip,
    this.nombre = '',
    this.esSwitch = false,
    this.puertos = 0,
  });

  factory SwitchQueContesto.desdeJson(Map<String, dynamic> json) => SwitchQueContesto(
        ip: json['ip'] as String? ?? '',
        nombre: json['nombre'] as String? ?? '',
        esSwitch: json['esSwitch'] as bool? ?? false,
        puertos: json['puertos'] as int? ?? 0,
      );
}
