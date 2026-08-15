import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mired_interfaz/modelos/modelos.dart';
import 'package:mired_interfaz/pantallas/mapa_plano.dart';

/// Pruebas del plano de lo que se declara a mano.
///
/// El caso es el de una casa de verdad: un modem con un switch colgado, y del
/// switch un grabador de video. Es donde se veian los dos errores que esto
/// corrige: el switch salia dos veces y el cable que sube al modem no ocupaba
/// ninguno de sus puertos.
void main() {
  Equipo equipo(int id, String nombre) => Equipo(
        id: id,
        ip: '',
        mac: '',
        fabricante: '',
        nombre: nombre,
        alias: '',
        tipo: '',
        subred: '192.168.1.0/24',
        metodo: '',
        presente: true,
        origen: 'manual',
        primeraVez: '',
        ultimaVez: '',
        puertos: const [],
      );

  /// El modem tiene un puerto (el 4) con el cable al switch; el switch tiene
  /// cinco, y por el 5 sube al modem.
  DatosMapa armarCasa({required bool cableEntrePuertos}) {
    return DatosMapa(
      mapa: const MapaPuertos(capacidad: 'ninguna', explicacion: '', puertos: [], enlaces: []),
      equipos: [equipo(1, 'MODEM TELMEX'), equipo(2, 'switch'), equipo(3, 'dvr')],
      topologia: TopologiaManual(
        puertos: const [
          PuertoFisico(id: 10, equipoId: 1, numero: 4, tipo: 'lan'),
          PuertoFisico(id: 20, equipoId: 2, numero: 1, tipo: 'lan'),
          PuertoFisico(id: 21, equipoId: 2, numero: 2, tipo: 'lan'),
          PuertoFisico(id: 22, equipoId: 2, numero: 3, tipo: 'lan'),
          PuertoFisico(id: 23, equipoId: 2, numero: 4, tipo: 'lan'),
          PuertoFisico(id: 24, equipoId: 2, numero: 5, tipo: 'lan'),
        ],
        enlaces: [
          // Del modem al switch. Con puerto de destino o sin el: las dos formas
          // tienen que dibujar lo mismo.
          EnlaceFisico(
            id: 100,
            puertoOrigenId: 10,
            equipoOrigenId: 1,
            numeroOrigen: 4,
            origenNombre: 'MODEM TELMEX',
            puertoDestinoId: cableEntrePuertos ? 24 : null,
            equipoDestinoId: cableEntrePuertos ? null : 2,
            numeroDestino: cableEntrePuertos ? 5 : 0,
            destinoNombre: 'switch',
            origenDato: 'manual',
          ),
          // Del switch al grabador, que no tiene puertos declarados.
          const EnlaceFisico(
            id: 101,
            puertoOrigenId: 20,
            equipoOrigenId: 2,
            numeroOrigen: 1,
            origenNombre: 'switch',
            equipoDestinoId: 3,
            destinoNombre: 'dvr',
            origenDato: 'manual',
          ),
        ],
      ),
    );
  }

  final colores = ColorScheme.fromSeed(seedColor: const Color(0xFF1565C0));

  test('cada aparato se dibuja una sola vez', () {
    final plano = armarPlano(armarCasa(cableEntrePuertos: true), colores);

    final cabeceras = plano.cajas.where((caja) => caja.titulo == 'switch').length;
    expect(cabeceras, 1,
        reason: 'el switch salia dos veces: como caja bajo el modem y como bloque suelto');
  });

  test('el cable que sube al modem ocupa un puerto del switch', () {
    final plano = armarPlano(armarCasa(cableEntrePuertos: true), colores);

    final libres = plano.cajas.where((caja) => caja.puertoLibre).toList();
    // Cinco puertos: uno sube al modem, otro lleva al grabador, quedan tres.
    expect(libres.length, 3,
        reason: 'un switch de 5 con el uplink puesto y un equipo colgado tiene 3 libres');
  });

  test('el puerto por donde sube el cable no se dibuja dos veces', () {
    // Con el aparato de la izquierda a la vista, esa conexion YA es la linea que
    // llega. Repetirla como una caja mas era ensuciar el dibujo con lo mismo.
    final plano = armarPlano(armarCasa(cableEntrePuertos: true), colores);

    expect(plano.cajas.where((caja) => caja.titulo.contains('MODEM')).length, 1,
        reason: 'el modem sale una vez: como cabecera, no tambien bajo el switch');
  });

  test('el mapa crece hacia la derecha, no hacia abajo', () {
    // Una red se lee como se cablea: la entrada a la izquierda y lo que cuelga,
    // a su derecha. Dibujado de arriba abajo, un switch de ocho puertos abria el
    // plano en abanico y las cajas se salian de la pantalla.
    final plano = armarPlano(armarCasa(cableEntrePuertos: true), colores);

    final modem = plano.cajas.firstWhere((caja) => caja.titulo == 'MODEM TELMEX');
    final conmutador = plano.cajas.firstWhere((caja) => caja.titulo == 'switch');
    final grabador = plano.cajas.firstWhere((caja) => caja.titulo == 'dvr');

    expect(conmutador.rectangulo.left, greaterThan(modem.rectangulo.left),
        reason: 'lo que cuelga del modem va a su derecha');
    expect(grabador.rectangulo.left, greaterThan(conmutador.rectangulo.left),
        reason: 'y lo que cuelga del switch, a la derecha del switch');

    // Los hermanos se apilan hacia abajo, en la misma columna.
    final libres = plano.cajas.where((caja) => caja.puertoLibre).toList();
    for (final libre in libres) {
      expect(libre.rectangulo.left, grabador.rectangulo.left,
          reason: 'los hermanos comparten columna');
    }
  });

  test('sin puerto de destino el switch tambien cuelga del modem', () {
    // Asi es como quedaron los cables declarados antes de que se pudiera elegir
    // el puerto del otro extremo. El dibujo no puede depender de eso.
    final plano = armarPlano(armarCasa(cableEntrePuertos: false), colores);

    expect(plano.cajas.where((caja) => caja.titulo == 'switch').length, 1);
    // Aqui los cinco puertos del switch siguen libres salvo el del grabador:
    // nadie dijo por cual sube, y MiRed no lo inventa.
    expect(plano.cajas.where((caja) => caja.puertoLibre).length, 4);
  });

  test('el grabador cuelga del switch y no queda suelto abajo', () {
    final datos = armarCasa(cableEntrePuertos: true);
    expect(datos.sinUbicar.map((equipo) => equipo.id), isNot(contains(3)));

    final plano = armarPlano(datos, colores);
    expect(plano.cajas.where((caja) => caja.titulo == 'dvr').length, 1);
  });

  test('el switch se ofrece como destino mientras le queden puertos', () {
    final datos = armarCasa(cableEntrePuertos: true);

    // Desde el puerto 4 del modem: el switch ya no deberia ofrecerse a si mismo
    // ni ofrecerse el propio modem.
    final desdeElModem = datos.conectablesDesde(10).map((equipo) => equipo.id).toList();
    expect(desdeElModem, contains(2), reason: 'al switch le quedan puertos libres');
    expect(desdeElModem, isNot(contains(1)), reason: 'un aparato no se conecta consigo mismo');
  });

  /// De la antena cuelgan varios equipos por el aire, sin puertos de por medio.
  DatosMapa armarCasaConWiFi() {
    return DatosMapa(
      mapa: const MapaPuertos(capacidad: 'ninguna', explicacion: '', puertos: [], enlaces: []),
      equipos: [
        equipo(2, 'switch'),
        equipo(5, 'AP ubiquiti'),
        equipo(6, 'telefono'),
        equipo(7, 'tv samsung'),
      ],
      topologia: TopologiaManual(
        puertos: const [
          PuertoFisico(id: 20, equipoId: 2, numero: 1, tipo: 'lan'),
          PuertoFisico(id: 21, equipoId: 2, numero: 2, tipo: 'lan'),
        ],
        enlaces: const [
          // El AP cuelga del switch por cable.
          EnlaceFisico(
            id: 100,
            puertoOrigenId: 20,
            equipoOrigenId: 2,
            numeroOrigen: 1,
            origenNombre: 'switch',
            equipoDestinoId: 5,
            destinoNombre: 'AP ubiquiti',
            origenDato: 'manual',
          ),
        ],
        inalambricos: const [
          EnlaceInalambrico(
              id: 1, equipoId: 6, antenaId: 5, red: 'CASA-5G', origenDato: 'manual'),
          EnlaceInalambrico(
              id: 2, equipoId: 7, antenaId: 5, red: 'CASA-5G', senalDbm: -62,
              origenDato: 'panel'),
        ],
      ),
    );
  }

  test('lo que entra por WiFi cuelga de su antena, no flota abajo', () {
    final datos = armarCasaConWiFi();

    // Ni el telefono ni la television siguen "sin ubicar".
    final sueltos = datos.sinUbicar.map((equipo) => equipo.id).toList();
    expect(sueltos, isNot(contains(6)));
    expect(sueltos, isNot(contains(7)));

    final plano = armarPlano(datos, colores);
    expect(plano.cajas.where((caja) => caja.titulo == 'telefono').length, 1);
    expect(plano.cajas.where((caja) => caja.titulo == 'tv samsung').length, 1);
  });

  test('la linea del WiFi se dibuja distinta de la del cable', () {
    // Son tres cosas distintas —medido, tecleado y por el aire— y el plano tiene
    // que dejar ver cual es cual sin leer la leyenda.
    final plano = armarPlano(armarCasaConWiFi(), colores);

    final porAire = plano.lineas.where((linea) => linea.inalambrica).toList();
    expect(porAire.length, 2, reason: 'los dos equipos por WiFi tienen su linea');
    for (final linea in porAire) {
      expect(linea.confirmada, isFalse);
      expect(linea.etiqueta, 'CASA-5G');
    }
  });

  test('de donde salio cada enlace se dice en la caja', () {
    // Lo declarado por una persona y lo que dijo el panel de la antena no se
    // presentan igual.
    final plano = armarPlano(armarCasaConWiFi(), colores);

    final telefono = plano.cajas.firstWhere((caja) => caja.titulo == 'telefono');
    expect(telefono.subtitulo, contains('declarado a mano'));

    final television = plano.cajas.firstWhere((caja) => caja.titulo == 'tv samsung');
    expect(television.subtitulo, contains('panel'));
    expect(television.subtitulo, contains('-62 dBm'));
  });

  test('el cable va en codo: sale horizontal, baja y entra horizontal', () {
    // En diagonal, un switch de varios puertos daba un abanico de rectas
    // cruzadas donde no se podia seguir ninguna con la vista.
    final plano = armarPlano(armarCasa(cableEntrePuertos: true), colores);

    final torcida = plano.lineas.firstWhere(
        (linea) => (linea.hasta.dy - linea.desde.dy).abs() > 1,
        orElse: () => throw StateError('ninguna linea cambia de altura'));

    final recorrido = recorridoDeEnlace(torcida);
    expect(recorrido.length, 5, reason: 'dos rectas, dos esquinas y la entrada');
    expect(recorrido.first.hasta.dy, torcida.desde.dy,
        reason: 'el primer tramo sale horizontal del aparato');
    expect(recorrido.last.hasta, torcida.hasta,
        reason: 'y el ultimo entra horizontal al de la derecha');
    expect(recorrido.where((tramo) => tramo.control != null).length, 2,
        reason: 'las dos esquinas van redondeadas');
  });

  test('a la misma altura el cable es una recta y ya', () {
    final plano = armarPlano(armarCasa(cableEntrePuertos: true), colores);

    final derecha = plano.lineas.firstWhere(
        (linea) => (linea.hasta.dy - linea.desde.dy).abs() < 1,
        orElse: () => throw StateError('ninguna linea va derecha'));

    expect(recorridoDeEnlace(derecha).length, 1);
  });

  test('dos hermanos no bajan por la misma vertical', () {
    // Con todos los cables por el mismo canal, diez enlaces de diez colores se
    // veian como una sola linea gorda.
    final plano = armarPlano(armarCasa(cableEntrePuertos: true), colores);

    final delSwitch = plano.lineas
        .where((linea) => linea.canalX != null && (linea.hasta.dy - linea.desde.dy).abs() > 1)
        .toList();
    final canales = delSwitch.map((linea) => linea.canalX).toSet();
    expect(canales.length, delSwitch.length,
        reason: 'cada cable que dobla baja por su propio canal');
  });

  test('la etiqueta del puerto va pegada al aparato al que entra el cable', () {
    // En mitad del cruce se encimaban unas con otras y con las cajas. Ahi
    // tampoco cabian: el hueco entre columnas se lo comia la propia bajada.
    final plano = armarPlano(armarCasa(cableEntrePuertos: true), colores);

    final cajas = {for (final caja in plano.cajas) caja.rectangulo.center: caja};

    for (final linea in plano.lineas) {
      final fin = finDeEtiqueta(linea);
      expect(fin.dx, lessThan(linea.hasta.dx),
          reason: 'la etiqueta termina antes de la caja, nunca encima');
      expect(fin.dx - anchoEtiquetaEnlace, greaterThan(linea.desde.dx),
          reason: 'y empieza despues del aparato del que sale');
      expect((fin.dy - linea.hasta.dy).abs(), lessThan(altoFila / 2),
          reason: 'va a la altura de lo que nombra');
    }
    expect(cajas, isNotEmpty);
  });

  test('dos etiquetas nunca caen en el mismo renglon', () {
    final plano = armarPlano(armarCasaConWiFi(), colores);

    final vistas = <String>{};
    for (final linea in plano.lineas) {
      final fin = finDeEtiqueta(linea);
      // Misma columna y misma altura seria una encima de la otra.
      final sitio = '${fin.dx.round()}:${fin.dy.round()}';
      expect(vistas.add(sitio), isTrue,
          reason: 'dos etiquetas en $sitio se taparian');
    }
  });

  test('una antena sin puertos declarados igual es cabecera de su bloque', () {
    // El AP no tiene ni un puerto declarado y aun asi le cuelgan dos equipos:
    // el WiFi no tiene puertos.
    final plano = armarPlano(armarCasaConWiFi(), colores);
    expect(plano.cajas.where((caja) => caja.titulo == 'AP ubiquiti').length, 1);
  });
}
