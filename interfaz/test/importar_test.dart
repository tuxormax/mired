import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mired_interfaz/modelos/modelos.dart';
import 'package:mired_interfaz/pantallas/importar.dart';

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

  testWidgets('la pantalla de importar se dibuja y explica como llenar la hoja',
      (probador) async {
    await probador.pumpWidget(const MaterialApp(
      home: PantallaImportar(
        red: Red(
          id: 1, clave: 'casa', nombre: 'Casa', descripcion: '', equipos: 0,
          equiposPresentes: 0, alertasAbiertas: 0, programado: false,
          presenciaCadaSegundos: 60, profundoCadaMinutos: 360,
        ),
      ),
    ));
    await probador.pump();

    expect(find.text('Descargar la plantilla'), findsOneWidget);
    expect(find.text('Elegir un archivo'), findsOneWidget);
    // Lo unico que de verdad cuesta entender de la plantilla tiene que estar a
    // la vista antes de elegir el archivo, no despues.
    expect(find.textContaining('El switch tambien lleva su renglon'), findsOneWidget);
    expect(find.textContaining('Nada se guarda hasta que usted lo diga'), findsOneWidget);
  });
}
