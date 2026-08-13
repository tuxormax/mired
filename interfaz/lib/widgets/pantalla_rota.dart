import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// pantallaRota es lo que se muestra cuando una pantalla revienta al dibujarse.
///
/// # Por que existe
///
/// Los tres capturadores globales de la casa no alcanzan a esto. Un error que
/// ocurre **mientras se dibuja** no se puede contar con un modal: para abrir un
/// modal hace falta una pantalla, y la pantalla es justo lo que acaba de
/// fallar. Flutter, en compilacion de entrega, lo resuelve pintando un hueco
/// **gris**: sin mensaje, sin causa y sin nada que copiar.
///
/// Eso paso de verdad (2026-08-13): una expresion regular mal armada reventaba
/// al teclear el usuario en el primer acceso, y lo unico que se veia era la
/// ventana en gris. Sin este recuadro, cualquier fallo al dibujar vuelve a ser
/// invisible.
///
/// # Reglas de este archivo
///
/// **Nada de aqui puede fallar.** Es lo ultimo que queda en pie cuando ya fallo
/// algo, asi que no se apoya en el tema, ni en `Material`, ni en ningun dato de
/// la aplicacion: se dibuja con lo minimo que Flutter garantiza siempre.
Widget pantallaRota(FlutterErrorDetails detalles) {
  final texto = _detallesEnTexto(detalles);

  return Directionality(
    textDirection: TextDirection.ltr,
    child: Material(
      color: const Color(0xFF3B0A0A),
      child: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              const Row(
                children: [
                  Icon(Icons.error_outline, color: Color(0xFFFFB4AB), size: 22),
                  SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Esta parte de MiRed no se pudo dibujar',
                      style: TextStyle(
                        color: Color(0xFFFFDAD6),
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 6),
              const Text(
                'Ha habido un error, copie los detalles y notifique a seismexico.',
                style: TextStyle(color: Color(0xFFFFDAD6), fontSize: 13),
              ),
              const SizedBox(height: 12),
              Flexible(
                child: SingleChildScrollView(
                  child: SelectableText(
                    texto,
                    style: const TextStyle(
                      color: Color(0xFFFFDAD6),
                      fontSize: 11,
                      fontFamily: 'monospace',
                    ),
                  ),
                ),
              ),
              const SizedBox(height: 12),
              _BotonCopiar(texto: texto),
            ],
          ),
        ),
      ),
    ),
  );
}

/// _BotonCopiar deja el error en el portapapeles y lo dice.
///
/// Es un boton hecho a mano y no un `FilledButton` porque este recuadro puede
/// acabar dibujado en cualquier punto del arbol, incluso donde no hay tema del
/// que sacar colores.
class _BotonCopiar extends StatefulWidget {
  const _BotonCopiar({required this.texto});

  final String texto;

  @override
  State<_BotonCopiar> createState() => _BotonCopiarState();
}

class _BotonCopiarState extends State<_BotonCopiar> {
  bool _copiado = false;

  @override
  Widget build(BuildContext contexto) {
    return GestureDetector(
      onTap: () async {
        await Clipboard.setData(ClipboardData(text: widget.texto));
        if (!mounted) return;
        setState(() => _copiado = true);
        // Vuelve a su estado por si hace falta copiarlo otra vez.
        Future<void>.delayed(const Duration(seconds: 2), () {
          if (mounted) setState(() => _copiado = false);
        });
      },
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
        decoration: BoxDecoration(
          color: const Color(0xFF93000A),
          borderRadius: BorderRadius.circular(8),
        ),
        child: Text(
          _copiado ? '✓ Copiado' : '📋 Copiar los detalles',
          style: const TextStyle(color: Color(0xFFFFDAD6), fontSize: 13),
        ),
      ),
    );
  }
}

/// _detallesEnTexto arma el bloque que el usuario copia.
///
/// Va envuelto en su propio try: si armar el texto fallara, este recuadro se
/// quedaria tan mudo como el gris que vino a sustituir.
String _detallesEnTexto(FlutterErrorDetails detalles) {
  try {
    final partes = <String>[
      'Error:     ${detalles.exceptionAsString()}',
      if (detalles.library != null) 'Donde:     ${detalles.library}',
      if (detalles.context != null) 'Dibujando: ${detalles.context}',
      '',
      '--- Stack ---',
      (detalles.stack?.toString() ?? 'sin stack'),
    ];
    return partes.join('\n');
  } catch (problema) {
    return 'No se pudieron leer los detalles del error: $problema';
  }
}
