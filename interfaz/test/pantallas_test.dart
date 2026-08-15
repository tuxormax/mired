import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mired_interfaz/modelos/modelos.dart';
import 'package:mired_interfaz/pantallas/alertas.dart';
import 'package:mired_interfaz/pantallas/mapa.dart';
import 'package:mired_interfaz/pantallas/red.dart';
import 'package:mired_interfaz/pantallas/redes.dart';
import 'package:mired_interfaz/servicios/api.dart';

/// Estas pruebas levantan un servidor de mentira y dibujan cada pantalla contra
/// el.
///
/// No comprueban como se ve —eso hay que mirarlo— sino que **se dibuje sin
/// reventar**, que es el fallo que de verdad ocurre: un desbordamiento de
/// tamano o un dato nulo dejan la pantalla en rojo, y sin una prueba asi eso se
/// descubre en produccion.
/// _ConRed devuelve un cliente HTTP de verdad.
///
/// El framework de pruebas de Flutter bloquea la red a proposito, para que nadie
/// escriba una prueba que dependa de internet. Aqui se destapa porque el
/// servidor esta en esta misma maquina y es de mentira: no hay red de por medio.
class _ConRed extends HttpOverrides {
  @override
  HttpClient createHttpClient(SecurityContext? contexto) =>
      super.createHttpClient(contexto);
}

void main() {
  late HttpServer servidor;

  setUpAll(() async {
    HttpOverrides.global = _ConRed();
    servidor = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    Api.instancia.baseDePrueba = 'http://127.0.0.1:${servidor.port}';
    Api.instancia.usuario = const Usuario(
      id: 1, usuario: '+prueba', nombre: 'Administrador de prueba',
      correo: '', superadmin: true,
    );

    servidor.listen((peticion) async {
      final ruta = peticion.uri.path;
      final datos = _respuestaDe(ruta);
      peticion.response.headers.contentType = ContentType.json;
      peticion.response.write(jsonEncode({'ok': true, 'datos': datos}));
      await peticion.response.close();
    });
  });

  tearDownAll(() async {
    await servidor.close(force: true);
    Api.instancia.baseDePrueba = null;
    HttpOverrides.global = null;
  });

  testWidgets('el panel de redes se dibuja con sus tarjetas', (probador) async {
    await dibujar(probador, const PantallaRedes());

    expect(find.text('Matriz'), findsOneWidget);
    expect(find.text('12'), findsWidgets); // equipos
    expect(tomaDeErrores(), isNull);
  });

  testWidgets('la pantalla de una red dibuja sus cinco pestanas', (probador) async {
    await dibujar(probador, PantallaRed(red: _red));

    expect(find.text('Equipos'), findsOneWidget);
    expect(find.text('Puertos'), findsOneWidget);
    expect(find.text('Consumo'), findsOneWidget);
    expect(find.text('WiFi'), findsOneWidget);
    expect(find.text('Que se revisa'), findsOneWidget);
    expect(tomaDeErrores(), isNull);
  });

  testWidgets('el aire no se barre solo: hay que pedirlo', (probador) async {
    // Recorrer los canales corta el WiFi de este equipo unos segundos. Hacerlo
    // sin que nadie lo pida seria cortarle la conexion a alguien por dibujar
    // una lista.
    await dibujar(probador, PantallaRed(red: _red));
    await probador.tap(find.text('WiFi'));
    await probador.pumpAndSettle();

    expect(find.text('Escuchar el aire'), findsOneWidget);
    expect(find.textContaining('redes ·'), findsNothing);
    expect(tomaDeErrores(), isNull);
  });

  testWidgets('la lista de equipos muestra lo que reconocio el catalogo', (probador) async {
    await dibujar(probador, PantallaRed(red: _red));

    expect(find.text('Impresora de contabilidad'), findsOneWidget);
    expect(find.textContaining('Impresora HP'), findsWidgets);
  });

  testWidgets('la pantalla de alertas se dibuja', (probador) async {
    await dibujar(probador, PantallaAlertas(red: _red));

    expect(find.textContaining('Se conecto un equipo desconocido'), findsOneWidget);
    expect(tomaDeErrores(), isNull);
  });

  testWidgets('el mapa se dibuja sin reventar', (probador) async {
    // El mapa es lo que mas facil se rompe: dibuja sobre lienzo y calcula
    // posiciones a mano.
    await dibujar(probador, PantallaMapa(red: _red));

    expect(find.textContaining('Linea llena'), findsOneWidget);
    // La tercera fuente del mapa se anuncia en la leyenda: sin eso, lo tecleado
    // se leeria igual que lo medido.
    expect(find.textContaining('declarado a mano'), findsOneWidget);
    expect(tomaDeErrores(), isNull);
  });

  testWidgets('la lista de equipos dice de que esta hecha la red', (probador) async {
    await dibujar(probador, PantallaRed(red: _red));

    // El total y cuantos estan prendidos: un equipo apagado sigue siendo parte
    // de la red, asi que el total no baila con cada barrido.
    expect(find.textContaining('4 aparatos'), findsOneWidget);
    expect(find.textContaining('3 prendidos'), findsOneWidget);

    // Y el desglose, con el plural bien dicho: "switches", no "switchs".
    expect(find.text('2 PC y laptops'), findsOneWidget);
    expect(find.text('1 Switch no administrable'), findsOneWidget);
    expect(find.text('1 Impresora'), findsOneWidget);

    // Lo declarado a mano cuenta, y se dice que se declaro: ningun escaneo lo
    // vio, y esa diferencia no se puede perder.
    expect(find.text('1 a mano'), findsOneWidget);
    expect(tomaDeErrores(), isNull);
  });

  testWidgets('la campanita de la red se relee, no se queda con la foto vieja',
      (probador) async {
    // La pantalla se abre con la red que le pasaron —2 alertas abiertas— y el
    // servidor contesta 7 al releerla. Que se vea el 7 es lo unico que demuestra
    // que la pantalla volvio a preguntar.
    //
    // Antes el numero se quedaba clavado en el que traia al abrirse: se
    // despachaban las alertas y la campanita seguia mostrando las viejas hasta
    // salir al panel de inicio y volver a entrar.
    await dibujar(probador, PantallaRed(red: _red));
    await dejarLlegarLoPedido(probador);

    expect(find.text('7'), findsOneWidget);
    expect(find.text('2'), findsNothing);
    expect(tomaDeErrores(), isNull);
  });

  testWidgets('el cableado se edita desde la pestana de Puertos', (probador) async {
    // Aqui es donde va la mano de quien quiere tocar el cableado. Estaba solo
    // como un lapiz suelto en la pantalla del mapa y no lo encontro nadie.
    await dibujar(probador, PantallaRed(red: _red));
    await cambiarDePestana(probador, 'Puertos');
    expect(find.text('Editar el cableado'), findsOneWidget);
    expect(tomaDeErrores(), isNull);
  });

  testWidgets('el mapa puede abrirse ya en modo edicion', (probador) async {
    // Quien pulso "Editar el cableado" ya dijo a que viene: pedirle que lo diga
    // otra vez al llegar seria hacerle buscar un boton mas.
    await dibujar(probador, PantallaMapa(red: _red, editarAlAbrir: true));

    expect(find.textContaining('Modo edicion'), findsOneWidget);
    expect(find.text('Terminar'), findsOneWidget);
    expect(tomaDeErrores(), isNull);
  });

  testWidgets('cada alerta se puede despachar por separado', (probador) async {
    // El cartelito "Nueva" solo repetia lo que ya decia la negrita del titulo, y
    // no dejaba hacer nada: para quitar una habia que despacharlas todas.
    await dibujar(probador, PantallaAlertas(red: _red));

    expect(find.text('Nueva'), findsNothing);
    expect(find.text('Marcar leida'), findsOneWidget);
    // Y cuantas quedan sin ver, a la vista, con la cuenta que manda el servidor.
    expect(find.text('2 sin ver'), findsOneWidget);
    expect(tomaDeErrores(), isNull);
  });

  testWidgets('el modo edicion no esta encendido de entrada', (probador) async {
    // Un clic de navegacion no puede reescribir la topologia por accidente: el
    // mapa es justo lo que se consulta cuando algo no funciona.
    await dibujar(probador, PantallaMapa(red: _red));

    expect(find.textContaining('Modo edicion'), findsNothing);
    expect(find.text('Agregar aparato'), findsNothing);

    // El boton va CON SU NOMBRE, no como un lapiz suelto: un icono solo se
    // reconoce cuando ya sabes que estas buscando, y la primera persona que uso
    // el programa no lo encontro.
    await probador.tap(find.text('Editar el cableado'));
    await probador.pump();

    expect(find.textContaining('Modo edicion'), findsOneWidget);
    expect(find.text('Agregar aparato'), findsOneWidget);
    expect(tomaDeErrores(), isNull);
  });
}

/// dibujar levanta la pantalla y espera a que llegue la respuesta del servidor.
///
/// pumpAndSettle solo por si mismo NO sirve aqui: adelanta el reloj falso, y la
/// respuesta del servidor llega por el reloj de verdad. runAsync es lo que deja
/// correr esa espera.
/// cambiarDePestana toca una pestana y espera a que su contenido llegue.
///
/// Son DOS esperas distintas y hacen falta las dos, en este orden:
///   1. `pump` en el reloj falso, para que termine la animacion de la pestana.
///   2. el reloj de verdad, para que llegue la respuesta del servidor.
/// Con solo la primera se ve la pestana vacia con su rueda girando; con solo la
/// segunda no se ha cambiado de pestana todavia.
Future<void> cambiarDePestana(WidgetTester probador, String nombre) async {
  await probador.tap(find.text(nombre));
  await probador.pump();
  await probador.pump(const Duration(seconds: 1));
  await dejarLlegarLoPedido(probador);
}

/// dejarLlegarLoPedido deja correr el reloj DE VERDAD un momento.
///
/// Hace falta cada vez que la prueba destapa una parte de la pantalla que pide
/// datos —cambiar de pestana, abrir una ficha—, y es el mismo motivo que explica
/// `dibujar`: la respuesta del servidor de mentira viaja por el reloj real,
/// mientras que `pump` solo adelanta el falso. Sin esto, el `FutureBuilder`
/// recien construido se queda esperando para siempre y la prueba ve una pestana
/// **vacia** en vez de la que se quiere comprobar.
Future<void> dejarLlegarLoPedido(WidgetTester probador) async {
  await probador.runAsync(() async {
    await Future<void>.delayed(const Duration(milliseconds: 200));
  });
  await probador.pump();
  await probador.pump(const Duration(seconds: 1));
}

Future<void> dibujar(WidgetTester probador, Widget pantalla) async {
  await probador.runAsync(() async {
    await probador.pumpWidget(MaterialApp(home: pantalla));
    await Future<void>.delayed(const Duration(milliseconds: 400));
  });
  await probador.pump();
  await probador.pump();
}

/// tomaDeErrores devuelve la excepcion que dejo el ultimo dibujo, si hubo.
Object? tomaDeErrores() {
  final detalles = TestWidgetsFlutterBinding.instance.takeException();
  return detalles;
}

const _red = Red(
  id: 1, clave: 'matriz-a1b2', nombre: 'Matriz', descripcion: 'Oficina central',
  equipos: 12, equiposPresentes: 9, ultimoEscaneo: '2026-08-12T10:00:00-06:00',
  alertasAbiertas: 2, programado: true, presenciaCadaSegundos: 60,
  profundoCadaMinutos: 360,
);

/// _respuestaDe imita lo que contesta el servidor de verdad para cada ruta.
dynamic _respuestaDe(String ruta) {
  if (ruta.endsWith('/equipos')) {
    return [
      {
        'id': 1, 'ip': '192.168.1.47', 'mac': 'b8:27:eb:1a:2b:3c',
        'fabricante': 'HP', 'nombre': 'hp-lj', 'alias': 'Impresora de contabilidad',
        'tipo': 'Impresora HP', 'subred': '192.168.1.0/24', 'metodo': 'arp',
        'presente': true, 'primeraVez': '2026-08-01T09:00:00-06:00',
        'ultimaVez': '2026-08-12T10:00:00-06:00',
        'puertos': [
          {'numero': 9100, 'protocolo': 'tcp', 'servicio': 'Impresion cruda', 'banner': ''},
          {'numero': 80, 'protocolo': 'tcp', 'servicio': 'HTTP', 'banner': ''},
        ],
      },
      {
        'id': 2, 'ip': '192.168.1.99', 'mac': '', 'fabricante': '', 'nombre': '',
        'alias': '', 'tipo': '', 'subred': '192.168.1.0/24', 'metodo': 'tcp',
        'presente': false, 'primeraVez': '2026-08-10T09:00:00-06:00',
        'ultimaVez': '2026-08-11T09:00:00-06:00', 'puertos': [],
      },
      // El switch tonto: sin IP, sin MAC y sin fabricante, porque no contesta a
      // nada. Existe solo porque alguien lo tecleo.
      {
        'id': 4, 'ip': '', 'mac': '', 'fabricante': '', 'nombre': '',
        'alias': 'Switch del rack', 'tipo': 'switch', 'subred': '', 'metodo': 'manual',
        'presente': true, 'primeraVez': '2026-08-14T09:00:00-06:00',
        'ultimaVez': '2026-08-14T09:00:00-06:00', 'puertos': [],
        'modelo': 'TP-Link SG108', 'notas': '', 'origen': 'manual', 'conexion': '',
      },
    ];
  }
  if (ruta.endsWith('/subredes')) {
    return [
      {'id': 1, 'cidr': '192.168.1.0/24', 'nombre': 'Oficinas', 'descripcion': '',
       'vlan': 10, 'escanear': true},
    ];
  }
  if (ruta.endsWith('/mapa-puertos')) {
    return {
      'capacidad': 'exacta',
      'explicacion': 'En esta red se sabe el puerto exacto de cada equipo.',
      'puertos': [
        {'switchId': 3, 'switchNombre': 'sw-principal', 'switchIp': '192.168.1.1',
         'indice': 5, 'puerto': 'Gi0/5', 'alias': '', 'activa': true,
         'velocidadMbps': 1000, 'mac': 'b8:27:eb:1a:2b:3c', 'equipoId': 1,
         'equipoNombre': 'Impresora de contabilidad', 'equipoIp': '192.168.1.47',
         'confirmado': true, 'cuantosEnPuerto': 1},
        {'switchId': 3, 'switchNombre': 'sw-principal', 'switchIp': '192.168.1.1',
         'indice': 7, 'puerto': 'Gi0/7', 'alias': '', 'activa': true,
         'velocidadMbps': 100, 'mac': 'cc:cc:cc:00:00:20', 'equipoId': null,
         'equipoNombre': '', 'equipoIp': '', 'confirmado': false, 'cuantosEnPuerto': 4},
      ],
    };
  }
  // Lo declarado a mano: un switch tonto (id 4) de dos puertos, con el primero
  // conectada a un equipo ya descubierto y la segunda libre. Es el caso que
  // ningun escaneo puede ver y que hace falta que el mapa sepa dibujar.
  // De que esta hecha la red. Lo cuenta el SERVIDOR sobre toda la red, no la
  // pantalla sobre lo que tenga a la vista.
  if (ruta.endsWith('/composicion')) {
    return {
      'total': 4,
      'presentes': 3,
      'declarados': 1,
      'categorias': [
        {'categoria': 'computadora', 'cuantos': 2, 'presentes': 2, 'declarados': 0},
        {'categoria': 'switch_simple', 'cuantos': 1, 'presentes': 1, 'declarados': 1},
        {'categoria': 'impresora', 'cuantos': 1, 'presentes': 0, 'declarados': 0},
      ],
    };
  }
  if (ruta.endsWith('/topologia-manual')) {
    return {
      'puertos': [
        {'id': 11, 'equipoId': 4, 'numero': 1, 'tipo': 'lan', 'velocidadMbps': 1000,
         'notas': '', 'creadoEn': '2026-08-14T09:00:00-06:00'},
        {'id': 12, 'equipoId': 4, 'numero': 2, 'tipo': 'lan', 'velocidadMbps': null,
         'notas': '', 'creadoEn': '2026-08-14T09:00:00-06:00'},
      ],
      'enlaces': [
        {'id': 21, 'puertoOrigenId': 11, 'equipoOrigenId': 4, 'numeroOrigen': 1,
         'origenNombre': 'Switch del rack', 'puertoDestinoId': null,
         'equipoDestinoId': 1, 'numeroDestino': 0,
         'destinoNombre': 'Impresora de contabilidad', 'origenDato': 'manual',
         'notas': '', 'creadoEn': '2026-08-14T09:00:00-06:00'},
      ],
      'contradicciones': [],
      'momento': '2026-08-14T09:00:00-06:00',
    };
  }
  if (ruta.endsWith('/consumo')) {
    return {
      'explicacion': 'El consumo sale de los contadores de cada puerto del switch.',
      'consumo': [
        {'switchId': 3, 'switchNombre': 'sw-principal', 'indice': 5, 'puerto': 'Gi0/5',
         'equipoNombre': 'Impresora de contabilidad', 'equipoIp': '192.168.1.47',
         'confirmado': true, 'cuantosEnPuerto': 1, 'bpsEntrada': 8000000,
         'bpsSalida': 2000000, 'momento': '2026-08-12T10:00:00-06:00'},
      ],
      'porFlujos': [],
    };
  }
  if (ruta.endsWith('/alertas')) {
    return {
      'abiertas': 2,
      'alertas': [
        {'id': 1, 'tipo': 'equipo_nuevo', 'momento': '2026-08-12T10:00:00-06:00',
         'equipoId': 2, 'titulo': 'Se conecto un equipo desconocido: 192.168.1.99',
         'detalle': 'IP 192.168.1.99', 'vista': false, 'enviada': true},
      ],
    };
  }
  if (ruta.endsWith('/reglas')) {
    return [
      {'tipo': 'equipo_nuevo', 'activa': true, 'umbral': 0},
      {'tipo': 'equipo_ausente', 'activa': true, 'umbral': 1440},
    ];
  }
  if (ruta.endsWith('/destinos')) return [];
  if (ruta.endsWith('/escaneos')) return {'escaneos': [], 'enCurso': false};
  if (ruta == '/api/sonda') return {'viva': true, 'estado': {'advertencia': ''}};
  if (ruta == '/api/redes') {
    return [
      {'id': 1, 'clave': 'matriz-a1b2', 'nombre': 'Matriz', 'descripcion': 'Oficina central',
       'equipos': 12, 'equiposPresentes': 9, 'ultimoEscaneo': '2026-08-12T10:00:00-06:00',
       'alertasAbiertas': 2, 'programado': true, 'presenciaCadaSegundos': 60,
       'profundoCadaMinutos': 360},
    ];
  }
  // Releer UNA red. Es lo que deja que la campanita de la pantalla del sitio
  // baje sin tener que salir al panel de inicio y volver a entrar.
  if (ruta == '/api/redes/matriz-a1b2') {
    return {
      'red': {
        'id': 1, 'clave': 'matriz-a1b2', 'nombre': 'Matriz', 'descripcion': 'Oficina central',
        'equipos': 12, 'equiposPresentes': 9, 'ultimoEscaneo': '2026-08-12T10:00:00-06:00',
        'alertasAbiertas': 7, 'programado': true, 'presenciaCadaSegundos': 60,
        'profundoCadaMinutos': 360,
      },
      'nivel': 'escritura',
    };
  }
  return {};
}
