import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mired_interfaz/pantallas/primer_acceso.dart';
import 'package:mired_interfaz/widgets/pantalla_rota.dart';

/// Pruebas de la pantalla del primer acceso y de la red de seguridad.
///
/// Estas existen por un fallo real (2026-08-13): al teclear el `+` del usuario,
/// la pantalla reventaba y el programa se quedaba **en gris**. La causa era una
/// expresion regular armada con la lista de signos de TUXOR, donde el guion
/// quedaba entre `+` y `*` y se leia como un rango al reves.
///
/// Lo que lo hacia tan traicionero: solo fallaba al escribir uno de esos signos,
/// que es exactamente lo que la pantalla pide hacer. Y no se veia ningun
/// mensaje, porque un error al dibujar no puede abrir un modal.
void main() {
  group('escribir en el primer acceso no puede reventar', () {
    testWidgets('teclear los signos de TUXOR uno por uno', (probador) async {
      await probador.pumpWidget(const MaterialApp(
        home: PantallaPrimerAcceso(operadores: '+ - * % ^ & | < > #'),
      ));

      // Se teclea CADA signo, que es justo lo que reventaba: la comprobacion
      // solo se llegaba a ejecutar cuando el texto empezaba o terminaba con uno.
      for (final signo in '+-*%^&|<>#'.split('')) {
        await probador.enterText(find.byType(TextFormField).first, '${signo}admin');
        await probador.pump();
        expect(probador.takeException(), isNull,
            reason: 'teclear "$signo" no deberia reventar la pantalla');
      }
    });

    testWidgets('teclear el signo al final y con el modificador @',
        (probador) async {
      await probador.pumpWidget(const MaterialApp(
        home: PantallaPrimerAcceso(operadores: '+ - * % ^ & | < > #'),
      ));

      for (final texto in ['admin+', '@@+admin+', '@+admin', 'admin+@', '+', '@', '']) {
        await probador.enterText(find.byType(TextFormField).first, texto);
        await probador.pump();
        expect(probador.takeException(), isNull,
            reason: 'teclear "$texto" no deberia reventar la pantalla');
      }
    });

    testWidgets('el aviso cambia cuando ya se cumple la regla', (probador) async {
      await probador.pumpWidget(const MaterialApp(
        home: PantallaPrimerAcceso(operadores: '+ - * % ^ & | < > #'),
      ));

      // Sin ningun signo, la pantalla explica la regla y no deja continuar.
      await probador.enterText(find.byType(TextFormField).first, 'admin');
      await probador.pump();
      expect(find.textContaining('deben empezar o terminar'), findsOneWidget);

      // Con el signo puesto, lo dice y habilita el boton.
      await probador.enterText(find.byType(TextFormField).first, '+admin');
      await probador.pump();
      expect(find.textContaining('ya lleva uno de los signos'), findsOneWidget);
      expect(probador.takeException(), isNull);
    });
  });

  group('la red de seguridad de las pantallas rotas', () {
    testWidgets('un error al dibujar muestra el error, no un hueco gris',
        (probador) async {
      // Se pone la red de seguridad igual que en el arranque del programa, y se
      // devuelve a su sitio DENTRO de la prueba: el marco comprueba al terminar
      // que nadie la haya dejado cambiada, y addTearDown corre demasiado tarde.
      final anterior = ErrorWidget.builder;
      ErrorWidget.builder = pantallaRota;
      try {
        await probador.pumpWidget(MaterialApp(
          home: Builder(builder: (_) => throw StateError('algo se rompio al dibujar')),
        ));
        probador.takeException(); // el error es el objeto de la prueba

        // Lo que el usuario tiene que ver: que paso, y algo que pueda copiar.
        expect(find.textContaining('no se pudo dibujar'), findsOneWidget);
        expect(find.textContaining('copie los detalles'), findsOneWidget);
        expect(find.textContaining('algo se rompio al dibujar'), findsOneWidget);
        expect(find.textContaining('Copiar'), findsOneWidget);
      } finally {
        ErrorWidget.builder = anterior;
      }
    });

    testWidgets('el recuadro no depende de que haya tema ni Material',
        (probador) async {
      // Es lo ultimo que queda en pie cuando ya fallo algo: si se apoyara en el
      // tema de la aplicacion, fallaria justo cuando hace falta.
      await probador.pumpWidget(pantallaRota(FlutterErrorDetails(
        exception: StateError('sin MaterialApp alrededor'),
        library: 'prueba',
      )));

      expect(probador.takeException(), isNull);
      expect(find.textContaining('sin MaterialApp alrededor'), findsOneWidget);
    });
  });
}
