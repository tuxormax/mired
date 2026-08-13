import 'package:flutter_test/flutter_test.dart';
import 'package:mired_interfaz/servicios/supervisor.dart';

/// Pruebas del supervisor, sobre las dos decisiones que de verdad importan:
/// **a que servidor toca** y **cuando hay que reemplazarlo**.
///
/// Existen por un fallo real (2026-08-13): tras actualizar el paquete sin cerrar
/// el programa, el `mired-servidor` viejo seguia vivo con su codigo en memoria y
/// se quedaba con el puerto. El programa nuevo se colgaba de el, y el resultado
/// era una interfaz nueva hablando con un motor viejo: el pie mostraba una
/// version que ya no estaba instalada y la mitad de las cosas nuevas no
/// funcionaban, sin que nada lo explicara.
void main() {
  group('a que servidor se le puede tocar', () {
    test('el de este equipo, si', () {
      for (final direccion in [
        'http://localhost:60072',
        'http://127.0.0.1:60072',
        'https://localhost:60072',
      ]) {
        expect(Servicios.esDeEsteEquipoParaPruebas(direccion), isTrue,
            reason: direccion);
      }
    });

    test('el de otro equipo, NO', () {
      // Es lo que impide apagarle el servicio a alguien mas. Si el programa
      // apunta a la Raspberry de una sucursal, ese MiRed puede estar vigilando
      // una red que no es la nuestra.
      for (final direccion in [
        'http://192.168.1.50:60072',
        'http://sucursal.local:60072',
        'no es una direccion',
      ]) {
        expect(Servicios.esDeEsteEquipoParaPruebas(direccion), isFalse,
            reason: direccion);
      }
    });
  });

  group('comparar versiones', () {
    test('se entiende lo que dice el binario y lo que dice la API', () {
      // Vienen en dos formatos distintos y tienen que acabar igual, o la
      // comparacion diria siempre que no coinciden y reiniciaria en cada
      // arranque.
      expect(Servicios.versionParaPruebas('mired-servidor MiRed v1.14 Rev 15 (4ea57e5)'),
          'v1.14 Rev 15');
      expect(Servicios.versionParaPruebas('v1.14 Rev 15'), 'v1.14 Rev 15');
      expect(Servicios.versionParaPruebas('1.14 Rev 15'), 'v1.14 Rev 15');
    });

    test('el hash del build NO cuenta', () {
      // Dos compilaciones de la misma entrega son la misma version. Si contara,
      // recompilar sin cambiar nada reiniciaria el servidor en cada arranque.
      final unaCompilacion =
          Servicios.versionParaPruebas('MiRed v1.14 Rev 15 (aaaaaaa)');
      final otraCompilacion =
          Servicios.versionParaPruebas('MiRed v1.14 Rev 15 (bbbbbbb)');
      expect(unaCompilacion, otraCompilacion);
    });

    test('versiones distintas se notan', () {
      // Es el caso que mordio: Rev 8 corriendo con Rev 15 instalada.
      expect(Servicios.versionParaPruebas('MiRed v1.1 Rev 8 (07f2887)'),
          isNot(Servicios.versionParaPruebas('MiRed v1.14 Rev 15 (4ea57e5)')));
    });

    test('lo que no se entiende no se inventa', () {
      // Ante la duda, null: mejor no reiniciar nada que reiniciar por haber
      // leido mal una cadena.
      for (final texto in ['', 'MiRed', 'algo raro', 'v1.14']) {
        expect(Servicios.versionParaPruebas(texto), isNull, reason: texto);
      }
    });
  });
}
