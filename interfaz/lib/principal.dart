import 'dart:async';
import 'dart:ui';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

import 'modelos/modelos.dart';
import 'pantallas/entrar.dart';
import 'pantallas/primer_acceso.dart';
import 'pantallas/redes.dart';
import 'servicios/api.dart';
import 'servicios/supervisor.dart';
import 'widgets/mensajes.dart';
import 'widgets/pantalla_rota.dart';

/// navegador permite abrir el modal de error desde cualquier lado, incluso
/// desde un fallo que ocurrio fuera del arbol de pantallas.
final GlobalKey<NavigatorState> navegador = GlobalKey<NavigatorState>();

void main() {
  // Los tres candados de la casa. Sin ellos, un error no atrapado no llega a
  // ningun lado: el usuario ve la pantalla congelada y nadie se entera.
  runZonedGuarded<Future<void>>(() async {
    WidgetsFlutterBinding.ensureInitialized();

    FlutterError.onError = (detalles) {
      FlutterError.presentError(detalles);
      _reportarGlobal(detalles.exception, detalles.stack);
    };

    // El CUARTO candado, y el que faltaba: un error que ocurre mientras se
    // dibuja no se puede contar con un modal, porque para abrir un modal hace
    // falta una pantalla y la pantalla es lo que acaba de fallar. Sin esto,
    // Flutter pinta un hueco gris: sin mensaje, sin causa y sin nada que copiar.
    ErrorWidget.builder = pantallaRota;

    PlatformDispatcher.instance.onError = (problema, pila) {
      _reportarGlobal(problema, pila);
      return true;
    };

    runApp(const AplicacionMiRed());
  }, (problema, pila) => _reportarGlobal(problema, pila));
}

void _reportarGlobal(Object problema, StackTrace? pila) {
  final contexto = navegador.currentContext;
  if (contexto == null) {
    // Sin pantalla donde mostrarlo, al menos que quede en la bitacora.
    Api.instancia.reportarError(
      ErrorMiRed(
        modulo: 'Interfaz',
        accion: 'Error no atrapado',
        causa: 'Interno',
        mensaje: 'Error no atrapado en la interfaz.',
        errorCrudo: problema.toString(),
      ),
      '',
      pila?.toString() ?? '',
    );
    return;
  }
  mostrarProblema(contexto, problema, pila: pila?.toString());
}

class AplicacionMiRed extends StatelessWidget {
  const AplicacionMiRed({super.key});

  @override
  Widget build(BuildContext contexto) {
    return MaterialApp(
      title: 'MiRed',
      navigatorKey: navegador,
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        useMaterial3: true,
        colorScheme: ColorScheme.fromSeed(
          seedColor: const Color(0xFF7B2D8E),
          brightness: Brightness.light,
        ),
      ),
      darkTheme: ThemeData(
        useMaterial3: true,
        colorScheme: ColorScheme.fromSeed(
          seedColor: const Color(0xFF7B2D8E),
          brightness: Brightness.dark,
        ),
      ),
      home: const PantallaArranque(),
    );
  }
}

/// PantallaArranque decide a donde entrar: si hay una sesion guardada que
/// todavia vale, va directo a las redes.
class PantallaArranque extends StatefulWidget {
  const PantallaArranque({super.key});

  @override
  State<PantallaArranque> createState() => _PantallaArranqueState();
}

class _PantallaArranqueState extends State<PantallaArranque> with WidgetsBindingObserver {
  String _paso = 'Arrancando MiRed...';

  @override
  void initState() {
    super.initState();
    // Escuchar el cierre de la ventana es lo que permite matar los servicios:
    // sin esto quedarian corriendo despues de cerrar el programa, que es
    // justamente lo que no se quiere.
    WidgetsBinding.instance.addObserver(this);
    _decidir();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  /// didRequestAppExit es el aviso de que el usuario cerro la ventana.
  ///
  /// Se detienen los servicios ANTES de dejar salir al programa. Devolver
  /// `exit` sin esperar dejaria los procesos huerfanos: el programa se cierra y
  /// mired-servidor sigue con el puerto tomado.
  @override
  Future<AppExitResponse> didRequestAppExit() async {
    await Servicios.instancia.detener();
    return AppExitResponse.exit;
  }

  Future<void> _decidir() async {
    // Antes que nada, levantar lo que MiRed necesita para funcionar. En web esto
    // no hace nada: ahi la pagina la sirvio el propio servidor.
    setState(() => _paso = 'Arrancando los servicios de MiRed...');
    await Servicios.instancia.arrancar();
    if (!mounted) return;
    setState(() => _paso = 'Conectando...');

    var haySesion = false;
    var sinEstrenar = false;
    var operadores = '';
    try {
      haySesion = await Api.instancia.recuperarSesion();
      if (!haySesion) {
        // Una instalacion recien hecha no tiene usuarios: en vez del formulario
        // de entrar hay que pedir que se cree el administrador. Se consulta solo
        // cuando no hay sesion; con sesion la respuesta ya se sabe.
        final estado = await Api.instancia.estado();
        sinEstrenar = estado['sinEstrenar'] as bool? ?? false;
        operadores = estado['operadores'] as String? ?? '';
      }
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
    if (!mounted) return;

    Navigator.of(context).pushReplacement(MaterialPageRoute<void>(
      builder: (_) {
        if (haySesion) return const PantallaRedes();
        if (sinEstrenar) return PantallaPrimerAcceso(operadores: operadores);
        return const PantallaEntrar();
      },
    ));

    // Si algo quedo a medias se dice, pero DESPUES de dejar entrar: casi nada de
    // lo que puede fallar al arrancar impide trabajar, y bloquear el paso por un
    // aviso seria peor que el aviso.
    final aviso = Servicios.instancia.aviso;
    if (aviso != null && mounted) mensajeAviso(context, aviso);
  }

  @override
  Widget build(BuildContext contexto) => Scaffold(
        body: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.lan_outlined, size: 48,
                  color: Theme.of(contexto).colorScheme.primary),
              const SizedBox(height: 20),
              const CircularProgressIndicator(),
              const SizedBox(height: 20),
              Text(_paso, style: Theme.of(contexto).textTheme.bodySmall),
            ],
          ),
        ),
      );
}
