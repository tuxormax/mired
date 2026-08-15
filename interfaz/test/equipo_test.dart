import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mired_interfaz/modelos/modelos.dart';
import 'package:mired_interfaz/pantallas/equipo.dart';

/// Pruebas de la ficha de un aparato.
///
/// Tocar un aparato abre ESTO, no un menu de acciones: en el mapa un clic es
/// para mirar. Los enlaces se administran dentro, en Conexiones, donde se ve por
/// que medio entra cada uno.
void main() {
  Equipo equipo({
    required int id,
    required String nombre,
    String ip = '',
    String categoria = '',
    String conexion = '',
    List<CredencialEquipo> credenciales = const [],
  }) =>
      Equipo(
        id: id,
        ip: ip,
        mac: '',
        fabricante: '',
        nombre: nombre,
        alias: '',
        tipo: '',
        subred: '192.168.1.0/24',
        metodo: '',
        presente: true,
        origen: 'manual',
        categoria: categoria,
        conexion: conexion,
        primeraVez: '',
        ultimaVez: '',
        puertos: const [],
        credenciales: credenciales,
      );

  Future<void> abrir(WidgetTester probador, Equipo aparato,
      {TopologiaManual topologia = const TopologiaManual(),
      List<Equipo> equipos = const []}) async {
    // Ventana alta: la ficha lleva varias secciones y con la de serie —600 de
    // alto— lo de mas abajo no se llega a construir y las pruebas fallarian por
    // el tamano de la ventana, no por lo que se quiere comprobar.
    probador.view.physicalSize = const Size(1200, 2000);
    probador.view.devicePixelRatio = 1;
    addTearDown(probador.view.resetPhysicalSize);
    addTearDown(probador.view.resetDevicePixelRatio);

    await probador.pumpWidget(MaterialApp(
      home: Scaffold(
        body: DialogoEquipo(
          clave: 'casa',
          equipo: aparato,
          topologia: topologia,
          equipos: equipos,
        ),
      ),
    ));
    await probador.pump();
  }

  testWidgets('la ficha abre en propiedades, no en un menu de acciones', (probador) async {
    await abrir(probador, equipo(id: 1, nombre: 'AP ubiquiti', ip: '192.168.1.86'));

    expect(find.text('Propiedades'), findsOneWidget);
    expect(find.text('Conexiones'), findsOneWidget);
    // Y nada que borre nada a un toque de distancia.
    expect(find.text('Quitar el cable'), findsNothing);
  });

  testWidgets('las conexiones se separan por medio: cable y WiFi', (probador) async {
    await abrir(probador, equipo(id: 1, nombre: 'AP ubiquiti', ip: '192.168.1.86'));

    await probador.tap(find.text('Conexiones'));
    await probador.pumpAndSettle();

    expect(find.text('Por cable'), findsOneWidget);
    expect(find.text('Por WiFi'), findsOneWidget);
  });

  testWidgets('lo que cuelga por WiFi se lista con su SSID y de donde se supo',
      (probador) async {
    const topologia = TopologiaManual(inalambricos: [
      EnlaceInalambrico(
          id: 1, equipoId: 9, antenaId: 1, equipoNombre: 'telefono',
          red: 'CASA-5G', origenDato: 'manual'),
    ]);

    await abrir(probador, equipo(id: 1, nombre: 'AP ubiquiti', ip: '192.168.1.86'),
        topologia: topologia, equipos: [equipo(id: 9, nombre: 'telefono')]);

    await probador.tap(find.text('Conexiones'));
    await probador.pumpAndSettle();

    expect(find.text('telefono'), findsOneWidget);
    expect(find.textContaining('CASA-5G'), findsOneWidget);
    // De donde salio el dato se dice siempre: lo declarado y lo medido no son
    // lo mismo.
    expect(find.textContaining('declarado a mano'), findsOneWidget);
  });

  testWidgets('un switch no administrable no ofrece credenciales', (probador) async {
    // No tiene direccion: no hay panel al que entrar, y ofrecer un formulario de
    // usuario y clave seria ofrecer algo que no existe.
    await abrir(probador,
        equipo(id: 2, nombre: 'switch del rack', categoria: 'switch_simple'));

    expect(find.text('Propiedades'), findsOneWidget);
    expect(find.text('Conexiones'), findsOneWidget);
    expect(find.text('Credenciales'), findsNothing);
  });

  testWidgets('un aparato con direccion si ofrece credenciales', (probador) async {
    await abrir(probador,
        equipo(id: 1, nombre: 'MODEM TELMEX', ip: '192.168.1.254', categoria: 'gateway'));

    expect(find.text('Credenciales'), findsOneWidget);

    await probador.tap(find.text('Credenciales'));
    await probador.pumpAndSettle();
    expect(find.text('Guardar una credencial'), findsOneWidget);
  });

  testWidgets('la clave guardada no se muestra sola: hay que pedirla', (probador) async {
    await abrir(
      probador,
      equipo(
        id: 1,
        nombre: 'MODEM TELMEX',
        ip: '192.168.1.254',
        credenciales: const [
          CredencialEquipo(
              id: 5, equipoId: 1, tipo: 'web', usuario: 'admin', tieneClave: true),
        ],
      ),
    );

    await probador.tap(find.text('Credenciales'));
    await probador.pumpAndSettle();

    expect(find.textContaining('usuario: admin'), findsOneWidget);
    expect(find.text('Ver clave'), findsOneWidget);
  });

  testWidgets('un aparato de punta muestra su LAN 1 y de que cuelga', (probador) async {
    // Antes, la ficha de un grabador colgado del switch decia "no tiene puertos
    // declarados" y nada mas: ni siquiera que estaba conectado. El dato existia
    // —el cable lo declaro el switch— pero solo se veia desde el otro lado.
    const topologia = TopologiaManual(
      puertos: [PuertoFisico(id: 20, equipoId: 2, numero: 1, tipo: 'lan')],
      enlaces: [
        EnlaceFisico(
          id: 100,
          puertoOrigenId: 20,
          equipoOrigenId: 2,
          numeroOrigen: 1,
          origenNombre: 'switch',
          equipoDestinoId: 3,
          destinoNombre: 'dvr',
          origenDato: 'manual',
        ),
      ],
    );

    await abrir(probador, equipo(id: 3, nombre: 'dvr'),
        topologia: topologia, equipos: [equipo(id: 2, nombre: 'switch')]);

    await probador.tap(find.text('Conexiones'));
    await probador.pumpAndSettle();

    expect(find.text('LAN 1 → switch'), findsOneWidget);
    expect(find.textContaining('desde su LAN 1'), findsOneWidget);
  });

  testWidgets('un aparato de punta sin cable dice que le falta declararlo', (probador) async {
    await abrir(probador, equipo(id: 3, nombre: 'pc tuxor'));

    await probador.tap(find.text('Conexiones'));
    await probador.pumpAndSettle();

    expect(find.text('LAN 1'), findsOneWidget);
    expect(find.text('sin cable declarado'), findsOneWidget);
  });

  testWidgets('a una laptop de solo WiFi no se le supone toma de red', (probador) async {
    // Muchas laptops y tabletas ya no traen conector de red. Dibujarles un
    // LAN 1 seria inventarles un agujero que no tienen.
    await abrir(probador,
        equipo(id: 4, nombre: 'laptop', categoria: 'computadora', conexion: 'wifi'));

    await probador.tap(find.text('Conexiones'));
    await probador.pumpAndSettle();

    expect(find.text('LAN 1'), findsNothing);
    expect(find.textContaining('entra por el aire'), findsOneWidget);
  });

  testWidgets('con un cable declarado si hay toma, diga lo que diga la ficha',
      (probador) async {
    // Alguien enchufo un cable: entonces hay donde enchufarlo. El dato duro
    // manda sobre el campo de la ficha.
    const topologia = TopologiaManual(
      puertos: [PuertoFisico(id: 20, equipoId: 2, numero: 3, tipo: 'lan')],
      enlaces: [
        EnlaceFisico(
          id: 100,
          puertoOrigenId: 20,
          equipoOrigenId: 2,
          numeroOrigen: 3,
          origenNombre: 'switch',
          equipoDestinoId: 4,
          destinoNombre: 'laptop',
          origenDato: 'manual',
        ),
      ],
    );

    await abrir(
        probador,
        equipo(id: 4, nombre: 'laptop', categoria: 'computadora', conexion: 'wifi'),
        topologia: topologia,
        equipos: [equipo(id: 2, nombre: 'switch')]);

    await probador.tap(find.text('Conexiones'));
    await probador.pumpAndSettle();

    expect(find.text('LAN 1 → switch'), findsOneWidget);
  });
}
