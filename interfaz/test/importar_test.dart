import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mired_interfaz/modelos/modelos.dart';
import 'package:mired_interfaz/pantallas/importar.dart';
import 'package:mired_interfaz/servicios/api.dart';

/// Pruebas de la importacion de una hoja de aparatos.
///
/// Lo que se lee y se valida vive en el servidor —ahi estan sus pruebas—; aqui
/// se comprueba que la interfaz entienda el plan que llega y que la pantalla lo
/// dibuje sin reventar.
void main() {
  test('el plan que llega del servidor se entiende entero', () {
    final plan = PlanImportacion.desdeJson(const {
      'crear': 2,
      'actualizar': 1,
      'rechazar': 1,
      'cables': 2,
      'conClave': 1,
      'ignoradas': ['RESPONSABLE'],
      'reconocidas': ['NOMBRE', 'QUE_ES'],
      'renglones': [
        {
          'renglon': 4,
          'nombre': 'switch site',
          'accion': 'crear',
          'tipo': 'Switch no administrable',
          'ubicacion': 'site',
        },
        {
          'renglon': 5,
          'nombre': 'D01',
          'accion': 'crear',
          'tipo': 'PC o laptop',
          'cuelgaDe': 'switch site',
          'puertoTipo': 'lan',
          'puertoNumero': 7,
          'ubicacion': 'admon',
        },
        {
          'renglon': 6,
          'nombre': 'D02',
          'accion': 'actualizar',
          'tipo': 'PC o laptop',
        },
        {
          'renglon': 7,
          'nombre': 'D03',
          'accion': 'rechazar',
          'motivo': 'CUELGA_DE dice «switch fantasma» y no hay ninguno asi',
        },
      ],
    });

    expect(plan.renglones.length, 4);
    expect(plan.hayAlgoQueImportar, isTrue);

    // El puerto llega partido en tipo y numero, como se guarda, y se junta como
    // esta rotulado en el aparato: si la pantalla dijera «7» y el mapa «LAN 7»,
    // serian dos nombres para el mismo puerto.
    expect(plan.renglones[1].puerto, 'LAN 7');
    expect(plan.renglones[0].puerto, isEmpty);

    expect(plan.renglones[2].seActualiza, isTrue);
    expect(plan.renglones[3].seRechaza, isTrue);
    expect(plan.renglones[3].motivo, contains('switch fantasma'));
    expect(plan.ignoradas, ['RESPONSABLE']);
  });

  test('un plan donde no se salva ningun renglon lo dice', () {
    final plan = PlanImportacion.desdeJson(const {
      'crear': 0,
      'actualizar': 0,
      'rechazar': 3,
      'renglones': <Map<String, dynamic>>[],
    });
    // Sin esto, la pantalla dejaria pulsar «Importar» para no hacer nada y el
    // aviso de «listo» se leeria como si hubiera funcionado.
    expect(plan.hayAlgoQueImportar, isFalse);
  });

  test('el resumen se cuenta en una frase, no en numeros sueltos', () {
    const resumen = ResumenImportacion(
        creados: 7, cables: 6, puertos: 28, credenciales: 1, recableados: 2);
    final frase = resumen.enPalabras;

    expect(frase, contains('7 aparatos nuevos'));
    expect(frase, contains('6 cables'));
    expect(frase, contains('28 puertos declarados'));
    // Recablear es el unico caso en que importar PISA un dato que ya estaba, asi
    // que tiene que salir dicho.
    expect(frase, contains('2 puertos que ya tenian otro cable'));
  });

  test('el singular y el plural no se calculan pegando una s', () {
    expect(const ResumenImportacion(creados: 1, cables: 1).enPalabras,
        '1 aparato nuevo, 1 cable.');
    expect(const ResumenImportacion().enPalabras, 'No hubo nada que cambiar.');
  });

  group('la pantalla completa', () {
    late HttpServer servidor;

    // La guia la manda el servidor: sale de la misma definicion con la que lee
    // el archivo. Aqui se le da una de mentira para comprobar que la pantalla la
    // dibuja entera en vez de traer su propia copia escrita a mano.
    setUpAll(() async {
      HttpOverrides.global = _ConRed();
      servidor = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      Api.instancia.baseDePrueba = 'http://127.0.0.1:${servidor.port}';

      servidor.listen((peticion) async {
        peticion.response.headers.contentType = ContentType.json;
        peticion.response.write(jsonEncode({
          'ok': true,
          'datos': {
            'nombre': 'mired-plantilla-aparatos.csv',
            'contenido': 'NOMBRE,QUE_ES\nD01,pc\n',
            'guia': {
              'columnas': [
                {
                  'clave': 'NOMBRE',
                  'obligatoria': true,
                  'ayuda': 'Como se llama. No se puede repetir en la red',
                  'ejemplo': 'D01',
                  'sinonimos': ['NOMBRE', 'NODO', 'EQUIPO'],
                },
                {
                  'clave': 'UBICACION',
                  'obligatoria': false,
                  'ayuda': 'Donde esta fisicamente',
                  'ejemplo': 'cons 5',
                  'sinonimos': ['UBICACION', 'SITIO'],
                },
              ],
              'categorias': [
                {'clave': 'switch_simple', 'comoSeLee': 'Switch no administrable',
                 'apodos': ['switch', 'switch tonto']},
                {'clave': 'computadora', 'comoSeLee': 'PC o laptop', 'apodos': ['pc']},
              ],
              'ejemplo': [
                {'NOMBRE': 'switch site', 'UBICACION': 'site'},
                {'NOMBRE': 'D01', 'UBICACION': 'admon'},
              ],
            },
          },
        }));
        await peticion.response.close();
      });
    });

    tearDownAll(() async {
      await servidor.close(force: true);
      Api.instancia.baseDePrueba = null;
      HttpOverrides.global = null;
    });

    testWidgets('trae las instrucciones, el campo del archivo y la guia',
        (probador) async {
      // Una ventana alta: es una pantalla de escritorio y la prueba tiene que
      // ver las tres partes a la vez. Con los 600 pixeles de omision, lo de
      // abajo ni siquiera se dibuja y la prueba no probaria nada.
      probador.view.physicalSize = const Size(1400, 3600);
      probador.view.devicePixelRatio = 1;
      addTearDown(probador.view.reset);

      await probador.runAsync(() async {
        await probador.pumpWidget(const MaterialApp(home: PantallaImportar(red: _casa)));
        await Future<void>.delayed(const Duration(milliseconds: 300));
      });
      await probador.pump();

      // 1. Las instrucciones.
      expect(find.text('Como se hace'), findsOneWidget);
      expect(find.text('Descargue la plantilla'), findsOneWidget);
      expect(find.textContaining('Nada se guarda hasta que usted lo diga'),
          findsOneWidget);

      // 2. El campo para subir el archivo.
      expect(find.text('El archivo'), findsOneWidget);
      expect(find.text('Se aceptan CSV, ODS y XLSX'), findsOneWidget);
      expect(find.text('Elegir un archivo'), findsOneWidget);
      expect(find.text('Descargar la plantilla'), findsOneWidget);

      // 3. La guia, dibujada con lo que mando el servidor.
      expect(find.text('Guia para llenar la hoja'), findsOneWidget);
      expect(find.text('NOMBRE'), findsWidgets);
      expect(find.text('Como se llama. No se puede repetir en la red'), findsOneWidget);
      // Los sinonimos se dicen: es lo que evita reescribir una hoja ajena.
      expect(find.textContaining('NODO'), findsWidgets);
      // La lista cerrada de QUE_ES sale del catalogo, no de una copia.
      expect(find.text('switch_simple'), findsOneWidget);
      expect(find.text('PC o laptop'), findsOneWidget);
    });

    testWidgets('no se desborda en una ventana chica', (probador) async {
      // El fallo que de verdad pasa en una pantalla con tablas: un Row que no
      // cabe deja la franja amarilla y negra. Se prueba estrecha a proposito.
      probador.view.physicalSize = const Size(900, 700);
      probador.view.devicePixelRatio = 1;
      addTearDown(probador.view.reset);

      await probador.runAsync(() async {
        await probador.pumpWidget(const MaterialApp(home: PantallaImportar(red: _casa)));
        await Future<void>.delayed(const Duration(milliseconds: 300));
      });
      await probador.pump();

      expect(tomaDeExcepcion(), isNull);
      expect(find.text('Como se hace'), findsOneWidget);
    });

    testWidgets('la guia no repite el nombre de la columna como sinonimo',
        (probador) async {
      // «NOMBRE se puede llamar NOMBRE» no ayuda a nadie.
      const columna = ColumnaPlantilla(
          clave: 'NOMBRE', sinonimos: ['NOMBRE', 'NODO', 'EQUIPO']);
      expect(columna.otrosNombres, ['NODO', 'EQUIPO']);
    });
  });
}

const _casa = Red(
  id: 1, clave: 'casa', nombre: 'Casa', descripcion: '', equipos: 0,
  equiposPresentes: 0, alertasAbiertas: 0, programado: false,
  presenciaCadaSegundos: 60, profundoCadaMinutos: 360,
);

/// tomaDeExcepcion devuelve lo que reviento al dibujar, si reviento algo. Un
/// desbordamiento de tamano llega por aqui.
Object? tomaDeExcepcion() => TestWidgetsFlutterBinding.instance.takeException();

/// El framework de pruebas bloquea la red a proposito. Aqui se destapa porque el
/// servidor esta en esta misma maquina y es de mentira.
class _ConRed extends HttpOverrides {
  @override
  HttpClient createHttpClient(SecurityContext? contexto) =>
      super.createHttpClient(contexto);
}
