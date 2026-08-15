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
        primeraVez: '',
        ultimaVez: '',
        puertos: const [],
        credenciales: credenciales,
      );

  Future<void> abrir(WidgetTester probador, Equipo aparato,
      {TopologiaManual topologia = const TopologiaManual(),
      List<Equipo> equipos = const []}) async {
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
}
