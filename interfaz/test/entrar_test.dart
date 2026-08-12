import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mired_interfaz/pantallas/entrar.dart';

void main() {
  testWidgets('la pantalla de entrada se dibuja con sus dos campos', (probador) async {
    await probador.pumpWidget(const MaterialApp(home: PantallaEntrar()));

    expect(find.text('MiRed'), findsOneWidget);
    expect(find.text('Usuario'), findsOneWidget);
    expect(find.text('Clave'), findsOneWidget);
    expect(find.widgetWithText(FilledButton, 'Entrar'), findsOneWidget);
  });

  testWidgets('no deja entrar con los campos vacios', (probador) async {
    await probador.pumpWidget(const MaterialApp(home: PantallaEntrar()));

    await probador.tap(find.widgetWithText(FilledButton, 'Entrar'));
    await probador.pump();

    // Si la validacion no frenara aqui, la peticion saldria al servidor con los
    // campos vacios y el rechazo llegaria despues, que es peor experiencia.
    expect(find.text('Capture su usuario'), findsOneWidget);
    expect(find.text('Capture su clave'), findsOneWidget);
  });
}
