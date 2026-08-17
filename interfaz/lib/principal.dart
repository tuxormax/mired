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

/// verdeDeMiRed es el color del programa: el verde de terminal.
///
/// Se eligio a proposito y no es decoracion: MiRed es una herramienta de red que
/// se mira sobre fondo negro, y el verde de fosforo es el idioma de ese mundo
/// desde que existen las terminales. **Es la semilla del tema entero**, asi que
/// tocarlo aqui repinta la aplicacion completa; no hay colores sueltos
/// repartidos por las pantallas.
///
/// Un apunte al cambiarlo: `ColorScheme.fromSeed` no usa el color tal cual, saca
/// de el una paleta entera. Un verde demasiado brillante como semilla devuelve
/// pasteles lavados en el tema claro, asi que este esta un punto por debajo del
/// verde puro de pantalla.
const Color verdeDeMiRed = Color(0xFF00C853);

/// esquemaOscuroDeMiRed es el tema de verdad: verde de fosforo sobre negro.
///
/// **Los tonos van escritos y no salen de la semilla.** `ColorStheme.fromSeed`
/// arma una paleta correcta pero descafeinada: el verde acaba en menta pastel y
/// —lo peor— el tono terciario sale AZUL, porque Material lo calcula girando la
/// rueda de color. En una herramienta que quiere parecer una terminal, un
/// turquesa suelto en los chips rompe el conjunto entero.
///
/// Asi que se parte de la semilla —para que los treinta y tantos colores que
/// nadie nombra sigan siendo coherentes— y se fijan a mano los seis que se ven:
/// los verdes, los fondos y el rojo del error. **El rojo se queda rojo**: un
/// aviso de que algo se rompio no se pinta del color de la marca.
final ColorScheme esquemaOscuroDeMiRed = ColorScheme.fromSeed(
  seedColor: verdeDeMiRed,
  brightness: Brightness.dark,
).copyWith(
  primary: const Color(0xFF00E676),
  onPrimary: const Color(0xFF00150A),
  primaryContainer: const Color(0xFF00522E),
  onPrimaryContainer: const Color(0xFF7CFFB4),
  secondary: const Color(0xFF69F0AE),
  onSecondary: const Color(0xFF00150A),
  secondaryContainer: const Color(0xFF13351F),
  onSecondaryContainer: const Color(0xFFA8FFCC),
  // El terciario tambien verde, un punto mas frio, para que la paleta no se
  // salga nunca de la familia.
  tertiary: const Color(0xFF3DDC97),
  onTertiary: const Color(0xFF00150A),
  tertiaryContainer: const Color(0xFF11402C),
  onTertiaryContainer: const Color(0xFFB6FFDA),
  // Casi negro, no gris: el verde solo se ve verde de verdad sobre negro.
  surface: const Color(0xFF080B09),
  onSurface: const Color(0xFFD7E8DC),
  onSurfaceVariant: const Color(0xFF9BB3A3),
  surfaceContainerLowest: const Color(0xFF050705),
  surfaceContainerLow: const Color(0xFF0C110E),
  surfaceContainer: const Color(0xFF101711),
  surfaceContainerHigh: const Color(0xFF161F19),
  surfaceContainerHighest: const Color(0xFF1C2820),
  outline: const Color(0xFF3C5546),
  outlineVariant: const Color(0xFF243329),
);

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
          seedColor: verdeDeMiRed,
          brightness: Brightness.light,
        ),
      ),
      darkTheme: ThemeData(
        useMaterial3: true,
        colorScheme: esquemaOscuroDeMiRed,
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
    // La direccion guardada primero: si el programa apunta al MiRed de otro
    // equipo, aqui no se arranca ni se detiene nada.
    await Api.instancia.cargarServidor();
    await Servicios.instancia.arrancar(servidor: Api.instancia.servidor);
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
