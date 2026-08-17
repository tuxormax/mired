import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mired_interfaz/modelos/modelos.dart';
import 'package:mired_interfaz/pantallas/accesos.dart';
import 'package:mired_interfaz/servicios/api.dart';

/// Pruebas de la lista unica de accesos.
///
/// Antes eran TRES sitios para lo mismo: las credenciales SNMP en su pantalla,
/// las controladoras WiFi en otra, y la clave del panel de cada aparato dentro
/// de su ficha. En la cabeza de quien las usa son una sola cosa —las
/// contrasenas de los aparatos de esta red— y aqui se comprueba que asi se vean.

class _ConRed extends HttpOverrides {
  @override
  HttpClient createHttpClient(SecurityContext? c) => super.createHttpClient(c);
}

const _casa = Red(
    id: 1, clave: 'casa', nombre: 'casa', descripcion: '', equipos: 3,
    equiposPresentes: 3, alertasAbiertas: 0, programado: false,
    presenciaCadaSegundos: 60, profundoCadaMinutos: 360);

void main() {
  late HttpServer servidor;

  setUpAll(() async {
    HttpOverrides.global = _ConRed();
    servidor = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    Api.instancia.baseDePrueba = 'http://127.0.0.1:${servidor.port}';
    Api.instancia.usuario = const Usuario(
        id: 1, usuario: '+p', nombre: 'Admin', correo: '', superadmin: true);

    servidor.listen((peticion) async {
      final ruta = peticion.uri.path;
      dynamic datos = <dynamic>[];
      if (ruta.endsWith('/credenciales-snmp')) {
        datos = [
          {'id': 1, 'nombre': 'Contrasena de lectura', 'version': 'v2c', 'creada': ''},
        ];
      } else if (ruta.endsWith('/controladoras')) {
        datos = [
          {'id': 5, 'nombre': 'UniFi de la oficina', 'tipo': 'unifi',
           'url': 'https://192.168.1.10:8443', 'usuario': 'lectura',
           'sitio': 'default', 'verificarTls': false, 'creada': ''},
        ];
      } else if (ruta.endsWith('/equipos')) {
        datos = [
          {
            'id': 7, 'ip': '192.168.1.2', 'mac': '', 'fabricante': '', 'nombre': '',
            'alias': 'switch site', 'tipo': 'Switch', 'subred': '', 'metodo': 'manual',
            'presente': true, 'primeraVez': '', 'ultimaVez': '', 'puertos': [],
            'credenciales': [
              {'id': 30, 'equipoId': 7, 'tipo': 'web', 'usuario': 'admin',
               'tieneClave': true, 'direccion': 'http://192.168.1.2', 'creada': ''},
            ],
          },
          {
            'id': 8, 'ip': '192.168.1.74', 'mac': '', 'fabricante': '', 'nombre': '',
            'alias': 'dvr', 'tipo': '', 'subred': '', 'metodo': 'arp',
            'presente': true, 'primeraVez': '', 'ultimaVez': '', 'puertos': [],
            'credenciales': [],
          },
        ];
      }
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

  Future<void> abrir(WidgetTester probador) async {
    probador.view.physicalSize = const Size(1280, 900);
    probador.view.devicePixelRatio = 1;
    addTearDown(probador.view.reset);
    await probador.runAsync(() async {
      await probador.pumpWidget(const MaterialApp(home: PantallaAccesos(red: _casa)));
      await Future<void>.delayed(const Duration(milliseconds: 400));
    });
    await probador.pump();
    await probador.pump();
  }

  testWidgets('las tres clases de llave salen en UNA lista', (probador) async {
    await abrir(probador);

    // La de SNMP, la de la controladora y la del panel de un aparato: juntas.
    expect(find.text('Todos los aparatos'), findsOneWidget);
    expect(find.text('UniFi de la oficina'), findsOneWidget);
    expect(find.text('switch site'), findsOneWidget);

    // Y cada una dice COMO se entra.
    expect(find.textContaining('SNMP v2c'), findsOneWidget);
    expect(find.textContaining('Controladora WiFi'), findsOneWidget);
    expect(find.textContaining('Panel web'), findsOneWidget);
  });

  testWidgets('la de SNMP se marca: es la unica que no es de un aparato',
      (probador) async {
    await abrir(probador);

    // Sin decirlo, «Todos los aparatos» se lee como si le faltara el nombre.
    expect(find.text('el que conteste'), findsOneWidget);
    expect(find.textContaining('Para encontrar los switches'), findsOneWidget);
  });

  testWidgets('el alta pregunta primero de que aparato es', (probador) async {
    await abrir(probador);
    await probador.tap(find.text('Nuevo acceso'));
    await probador.pumpAndSettle();

    // El orden importa: es lo que una persona piensa primero. Antes habia que
    // saber de antemano en cual de las tres pantallas estaba lo que se queria.
    expect(find.text('¿De que aparato es?'), findsOneWidget);
    expect(find.text('¿Como se entra?'), findsOneWidget);
    expect(find.textContaining('Todos — para encontrar los switches'), findsOneWidget);
  });

  testWidgets('con un aparato concreto, las formas de entrar son otras',
      (probador) async {
    await abrir(probador);
    await probador.tap(find.text('Nuevo acceso'));
    await probador.pumpAndSettle();

    // Con «todos» solo cabe SNMP: es la unica que se prueba a ciegas.
    expect(find.textContaining('(SNMP)'), findsOneWidget);

    await probador.tap(find.text('¿De que aparato es?'));
    await probador.pumpAndSettle();
    await probador.tap(find.text('dvr').last);
    await probador.pumpAndSettle();

    // Con un aparato concreto, lo que se ofrece es como entrar A EL.
    expect(find.text('Su pagina de administracion'), findsOneWidget);
    expect(find.textContaining('se guarda como la llave de ese aparato'.toLowerCase()),
        findsNothing);
    expect(find.textContaining('aparece tambien en su ficha'), findsOneWidget);
  });
}
