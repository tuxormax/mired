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
      id: 1, usuario: 'usuario-quitado', nombre: 'Superadministrador',
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

  testWidgets('la pantalla de una red dibuja sus cuatro pestanas', (probador) async {
    await dibujar(probador, PantallaRed(red: _red));

    expect(find.text('Equipos'), findsOneWidget);
    expect(find.text('Puertos'), findsOneWidget);
    expect(find.text('Consumo'), findsOneWidget);
    expect(find.text('Subredes'), findsOneWidget);
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
    expect(tomaDeErrores(), isNull);
  });
}

/// dibujar levanta la pantalla y espera a que llegue la respuesta del servidor.
///
/// pumpAndSettle solo por si mismo NO sirve aqui: adelanta el reloj falso, y la
/// respuesta del servidor llega por el reloj de verdad. runAsync es lo que deja
/// correr esa espera.
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
         'confirmado': true, 'cuantosEnBoca': 1},
        {'switchId': 3, 'switchNombre': 'sw-principal', 'switchIp': '192.168.1.1',
         'indice': 7, 'puerto': 'Gi0/7', 'alias': '', 'activa': true,
         'velocidadMbps': 100, 'mac': 'cc:cc:cc:00:00:20', 'equipoId': null,
         'equipoNombre': '', 'equipoIp': '', 'confirmado': false, 'cuantosEnBoca': 4},
      ],
    };
  }
  if (ruta.endsWith('/consumo')) {
    return {
      'explicacion': 'El consumo sale de los contadores de cada boca del switch.',
      'consumo': [
        {'switchId': 3, 'switchNombre': 'sw-principal', 'indice': 5, 'puerto': 'Gi0/5',
         'equipoNombre': 'Impresora de contabilidad', 'equipoIp': '192.168.1.47',
         'confirmado': true, 'cuantosEnBoca': 1, 'bpsEntrada': 8000000,
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
  return {};
}
