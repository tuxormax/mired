import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mired_interfaz/modelos/modelos.dart';
import 'package:mired_interfaz/pantallas/credenciales.dart';
import 'package:mired_interfaz/servicios/api.dart';

/// Pruebas de la pantalla de credenciales SNMP.
///
/// MiRed es para quien sabe poco de redes **y** para quien sabe mucho. Esta
/// pantalla es donde eso se pone a prueba: SNMP, «community» y v3 son jerga
/// pura. Lo que se vigila aqui es que lo simple este a la vista, que lo tecnico
/// siga estando —plegado, no borrado— y que se pueda PROBAR antes de guardar,
/// que es lo unico que convierte esto en algo usable sin saber de que va.

class _ConRed extends HttpOverrides {
  @override
  HttpClient createHttpClient(SecurityContext? c) => super.createHttpClient(c);
}

void main() {
  late HttpServer servidor;
  var loQueSeMando = <String, dynamic>{};

  setUpAll(() async {
    HttpOverrides.global = _ConRed();
    servidor = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    Api.instancia.baseDePrueba = 'http://127.0.0.1:${servidor.port}';
    Api.instancia.usuario = const Usuario(
        id: 1, usuario: '+p', nombre: 'Admin', correo: '', superadmin: true);

    servidor.listen((peticion) async {
      dynamic datos = <dynamic>[];
      if (peticion.uri.path.endsWith('/probar')) {
        loQueSeMando = jsonDecode(await utf8.decoder.bind(peticion).join())
            as Map<String, dynamic>;
        datos = {
          'consultados': 10,
          'contestaron': 2,
          'switches': [
            {'ip': '192.168.1.2', 'nombre': 'sw-principal', 'esSwitch': true, 'puertos': 24},
          ],
          'explicacion': 'Contestaron 2 aparatos.',
        };
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

  Future<void> abrirElDialogo(WidgetTester probador) async {
    probador.view.physicalSize = const Size(1280, 900);
    probador.view.devicePixelRatio = 1;
    addTearDown(probador.view.reset);

    // El dialogo se abre desde la lista de accesos, pero se prueba solo: lo que
    // se vigila aqui es COMO pregunta, no quien lo abre.
    await probador.runAsync(() async {
      await probador.pumpWidget(const MaterialApp(
          home: Scaffold(body: DialogoCredencialSNMP(clave: 'casa'))));
      await Future<void>.delayed(const Duration(milliseconds: 300));
    });
    await probador.pumpAndSettle();
  }

  testWidgets('lo simple a la vista: un solo campo y en cristiano',
      (probador) async {
    await abrirElDialogo(probador);

    // Nada de «comunidad» ni de «SNMP v2c» en la cara: un campo, con el nombre
    // de lo que es.
    expect(find.text('Contrasena de lectura del switch'), findsOneWidget);
    expect(find.textContaining('casi siempre es: public'), findsOneWidget);
    // Y donde buscarla, que es la pregunta siguiente de cualquiera.
    expect(find.textContaining('pagina del switch'), findsOneWidget);

    // La version de SNMP NO estorba de entrada: esta plegada.
    expect(find.text('Version de SNMP'), findsNothing);
    expect(find.textContaining('Opciones tecnicas'), findsOneWidget);
  });

  testWidgets('explica QUE es la community, no solo como se llama',
      (probador) async {
    // Saber el nombre de algo no es entenderlo: el usuario pregunto «¿que es eso
    // de community?» en cuanto vio la pantalla. La respuesta va aqui, donde se
    // hace la pregunta, y no en un manual aparte.
    await abrirElDialogo(probador);

    expect(find.textContaining('¿Que es esto de la «community»?'), findsOneWidget);
    // Plegado: quien ya lo sabe no tiene por que leerlo cada vez.
    expect(find.textContaining('contrasena de lectura del switch, y nada mas'),
        findsNothing);

    await probador.tap(find.textContaining('¿Que es esto'));
    await probador.pumpAndSettle();

    expect(find.textContaining('contrasena de lectura del switch, y nada mas'),
        findsOneWidget);
    // Las dos cosas que de verdad importan: que la de escritura NO se pone, y
    // que en v1/v2c la clave viaja en claro.
    expect(find.textContaining('No le ponga la de escritura'), findsOneWidget);
    expect(find.textContaining('viaja en claro'), findsOneWidget);
  });

  testWidgets('lo tecnico sigue ahi para quien lo busca', (probador) async {
    await abrirElDialogo(probador);
    await probador.tap(find.textContaining('Opciones tecnicas'));
    await probador.pumpAndSettle();

    // Quien lleva anos en redes abre esto y encuentra lo de siempre.
    expect(find.text('Version de SNMP'), findsOneWidget);
    // Y el nombre real del campo esta escrito, no escondido: es como lo llama el
    // manual del switch, que es donde va a tener que buscarlo.
    expect(find.textContaining('aparece como «community»'), findsOneWidget);
  });

  testWidgets('se puede probar antes de guardar, y dice cuantos contestaron',
      (probador) async {
    await abrirElDialogo(probador);
    await probador.enterText(find.byType(TextFormField).first, 'public');

    // El toque va DENTRO de runAsync: la peticion es de red de verdad —contra
    // el servidor de mentira de al lado— y en el reloj falso de las pruebas no
    // avanzaria nunca.
    await probador.runAsync(() async {
      await probador.tap(find.text('Probar ahora'));
      await Future<void>.delayed(const Duration(milliseconds: 600));
    });
    await probador.pump();
    await probador.pump();

    // Lo que contesto el servidor, a la vista y en una frase.
    expect(find.text('Contestaron 2 aparatos.'), findsOneWidget);
    // Con los switches por su nombre: saber CUALES contestaron es lo que
    // distingue «funciono» de «funciono con el que yo creia».
    expect(find.textContaining('sw-principal'), findsOneWidget);

    // Probar NO guarda: el dialogo sigue abierto esperando.
    expect(find.text('Guardar'), findsOneWidget);

    // Y se probo con lo escrito, no con otra cosa.
    expect(loQueSeMando['comunidad'], 'public');
    expect(loQueSeMando['version'], 'v2c');
  });

  testWidgets('sin nombre puesto, se le pone uno solo', (probador) async {
    // Pedir un nombre para algo de lo que casi siempre hay UNO es pedir trabajo
    // por nada.
    await abrirElDialogo(probador);
    await probador.enterText(find.byType(TextFormField).first, 'public');
    await probador.runAsync(() async {
      await probador.tap(find.text('Probar ahora'));
      await Future<void>.delayed(const Duration(milliseconds: 600));
    });
    await probador.pump();

    expect(loQueSeMando['nombre'], 'Contrasena de lectura');
  });
}
