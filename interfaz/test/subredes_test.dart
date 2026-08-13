import 'package:flutter_test/flutter_test.dart';
import 'package:mired_interfaz/servicios/subredes.dart';

/// Pruebas de la traduccion de lo que escribe una persona a lo que entiende el
/// escaneo.
///
/// Existen por un tropiezo real (2026-08-13): el formulario pedia una subred en
/// notacion de administrador de redes —`192.168.1.0/24`— y el usuario no tenia
/// forma de saber que poner. Aqui se comprueba que las formas en que la gente lo
/// dice de verdad se entiendan, y **que se entiendan bien**: interpretar mal un
/// rango no da un error, da un escaneo que mira donde no debe.
void main() {
  test('una direccion suelta se entiende como su red', () {
    // Es lo que hace cualquiera: mira la IP de su equipo y la escribe.
    final leido = interpretar('192.168.1.100');

    expect(leido.subredes, ['192.168.1.0/24']);
    expect(leido.cuantasDirecciones, 254);
    expect(leido.explicacion, contains('192.168.1.1 a la 192.168.1.254'));
  });

  test('la notacion con barra sigue funcionando para quien la sepa', () {
    expect(interpretar('192.168.1.0/24').subredes, ['192.168.1.0/24']);
    // Y se corrige sola si se escribio una direccion cualquiera de esa red.
    expect(interpretar('192.168.1.77/24').subredes, ['192.168.1.0/24']);
    expect(interpretar('10.0.0.0/16').cuantasDirecciones, 65534);
  });

  group('de aqui hasta aca', () {
    test('se entiende con guion, con "a" y con "hasta"', () {
      for (final texto in [
        '192.168.1.10-192.168.1.50',
        '192.168.1.10 - 192.168.1.50',
        '192.168.1.10 a 192.168.1.50',
        '192.168.1.10 hasta 192.168.1.50',
      ]) {
        final leido = interpretar(texto);
        expect(leido.cuantasDirecciones, 41, reason: 'con "$texto"');
        expect(leido.explicacion, contains('de la 192.168.1.10 a la 192.168.1.50'));
      }
    });

    test('escrito al reves se entiende igual', () {
      // Nadie quiere un error por haber puesto el mayor primero.
      final leido = interpretar('192.168.1.50 - 192.168.1.10');
      expect(leido.cuantasDirecciones, 41);
      expect(leido.explicacion, contains('de la 192.168.1.10 a la 192.168.1.50'));
    });

    test('el rango cubre exactamente lo pedido, ni una direccion mas', () {
      // Un rango se parte en bloques con barra porque es lo que entiende el
      // resto del sistema. Si esos bloques cubrieran de mas, el escaneo tocaria
      // equipos que nadie pidio tocar; si cubrieran de menos, faltarian.
      final leido = interpretar('192.168.1.10 - 192.168.1.50');

      var total = 0;
      for (final bloque in leido.subredes) {
        final bits = int.parse(bloque.split('/')[1]);
        total += 1 << (32 - bits);
      }
      expect(total, 41, reason: 'los bloques deben sumar el rango exacto');
    });

    test('una sola direccion', () {
      final leido = interpretar('192.168.1.7 - 192.168.1.7');
      expect(leido.cuantasDirecciones, 1);
      expect(leido.subredes, ['192.168.1.7/32']);
      expect(leido.explicacion, contains('solo 192.168.1.7'));
    });
  });

  test('lo que no se entiende se dice, no se adivina', () {
    // Adivinar aqui seria escanear una red que nadie pidio.
    for (final texto in [
      '',
      'la red de la oficina',
      '192.168.1',
      '192.168.1.999',
      '192.168.1.0/99',
      '999.999.999.999',
      '192.168.1.10 - ',
    ]) {
      expect(interpretar(texto).seEntendio, isFalse, reason: 'con "$texto"');
    }
  });

  group('la red que detecto la sonda', () {
    test('se convierte en la propuesta del formulario', () {
      final leido = deLaRedDetectada('192.168.1.100/24');

      expect(leido, isNotNull);
      expect(leido!.subredes, ['192.168.1.0/24']);
      expect(leido.cuantasDirecciones, 254);
    });

    test('una red enorme no se propone', () {
      // Un /8 son 16 millones de direcciones: eso no es "mi red", y barrerlo
      // tardaria dias. Mejor no proponerlo que proponer algo que no termina.
      expect(deLaRedDetectada('10.0.0.5/8'), isNull);
      expect(deLaRedDetectada('192.168.1.100/32'), isNull);
    });

    test('lo que no se puede leer no se propone', () {
      for (final texto in ['192.168.1.100', 'algo/24', '192.168.1.100/', '']) {
        expect(deLaRedDetectada(texto), isNull, reason: 'con "$texto"');
      }
    });
  });
}
