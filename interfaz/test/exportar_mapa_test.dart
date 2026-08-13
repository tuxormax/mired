import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:mired_interfaz/modelos/modelos.dart';
import 'package:mired_interfaz/pantallas/mapa_plano.dart';
import 'package:mired_interfaz/servicios/exportar_mapa.dart';

/// Pruebas de la exportacion del mapa.
///
/// El SVG y el PDF se escriben a mano, byte por byte. Un PDF con un
/// desplazamiento mal calculado en la tabla de referencias abre en un visor
/// tolerante y revienta en otro, asi que aqui se comprueba la estructura del
/// archivo, no solo que la funcion no lance excepcion.
void main() {
  final datos = DatosMapa(
    mapa: const MapaPuertos(
      capacidad: 'exacta',
      explicacion: 'En esta red se sabe el puerto exacto de cada equipo.',
      puertos: [
        PuertoDeSwitch(
          switchId: 3, switchNombre: 'sw-principal', switchIp: '192.168.1.1',
          indice: 5, puerto: 'Gi0/5', alias: '', activa: true,
          velocidadMbps: 1000, mac: 'b8:27:eb:1a:2b:3c', equipoId: 1,
          equipoNombre: 'Impresora de contabilidad, planta baja',
          equipoIp: '192.168.1.47', confirmado: true, cuantosEnBoca: 1,
        ),
        PuertoDeSwitch(
          switchId: 3, switchNombre: 'sw-principal', switchIp: '192.168.1.1',
          indice: 7, puerto: 'Gi0/7', alias: '', activa: true,
          velocidadMbps: 100, mac: 'cc:cc:cc:00:00:20',
          equipoNombre: '', equipoIp: '', confirmado: false, cuantosEnBoca: 4,
        ),
        PuertoDeSwitch(
          switchId: 8, switchNombre: 'sw-bodega', switchIp: '192.168.1.2',
          indice: 3, puerto: 'Fa0/3', alias: '', activa: true,
          velocidadMbps: 100, mac: 'dd:dd:dd:00:00:01',
          equipoNombre: '', equipoIp: '', confirmado: false, cuantosEnBoca: 9,
        ),
      ],
      // Dos switches unidos por un cable, visto por los dos protocolos: es el
      // caso en que el arco tiene que salir una sola vez y mas grueso.
      enlaces: [
        EnlaceEntreEquipos(
          equipoId: 3, equipoNombre: 'sw-principal', interfazLocal: 'Gi0/1',
          vecinoNombre: 'sw-bodega', vecinoPuerto: 'Gi0/24', vecinoId: 8,
          origen: 'lldp',
        ),
        EnlaceEntreEquipos(
          equipoId: 3, equipoNombre: 'sw-principal', interfazLocal: 'Gi0/1',
          vecinoNombre: 'sw-bodega.local', vecinoPuerto: 'GigabitEthernet0/24',
          vecinoId: 8, origen: 'cdp',
        ),
      ],
    ),
    equipos: const [
      Equipo(
        id: 1, ip: '192.168.1.47', mac: 'b8:27:eb:1a:2b:3c', fabricante: 'HP',
        nombre: 'hp-lj', alias: 'Impresora de contabilidad', tipo: 'Impresora HP',
        subred: '192.168.1.0/24', metodo: 'arp', presente: true,
        primeraVez: '', ultimaVez: '', puertos: [],
      ),
      // Uno con acento y con coma, que es lo que rompe el CSV y el PDF.
      Equipo(
        id: 9, ip: '192.168.1.99', mac: 'aa:bb:cc:dd:ee:ff', fabricante: '',
        nombre: '', alias: 'Camara del pasillo, ala norte «ñ»', tipo: '',
        subred: '192.168.1.0/24', metodo: 'tcp', presente: false,
        primeraVez: '', ultimaVez: '', puertos: [],
      ),
    ],
  );

  final plano = armarPlano(datos, coloresParaExportar);
  const encabezado = EncabezadoMapa(
    titulo: 'Mapa de Matriz',
    subtitulo: 'Exportado el 2026-08-13',
  );

  test('el CSV lleva encabezado, lo confirmado, el grupo y lo sin ubicar', () {
    final renglones = const LineSplitter().convert(csvDelMapa(datos));

    expect(renglones.first, startsWith('switch,ip_switch,puerto'));

    expect(renglones.any((r) => r.contains('Gi0/5') && r.contains('confirmado')), isTrue);
    expect(renglones.any((r) => r.contains('Gi0/7') && r.contains('grupo')), isTrue);
    // El equipo 9 no cuelga de ninguna boca: tiene que salir marcado, no
    // desaparecer.
    expect(renglones.any((r) => r.contains('sin ubicar')), isTrue);
    // Un nombre con coma va entrecomillado o parte el renglon en dos columnas.
    expect(renglones.any((r) => r.contains('"Camara del pasillo, ala norte «ñ»"')), isTrue);
  });

  test('el CSV dice de cuando son los datos, como los otros tres formatos', () {
    // Es la regla del proyecto: todo reporte que salga de MiRed dice de que
    // momento es. Un archivo suelto sin fecha, a la semana, ya no se sabe si
    // sirve — y el CSV es el que mas facil acaba en el correo de alguien.
    final renglones = const LineSplitter().convert(csvDelMapa(datos, encabezado));

    expect(renglones.first, contains('Mapa de Matriz'));
    expect(renglones.first, contains('Exportado el 2026-08-13'));
    // Con un renglon en blanco detras, para que la hoja de calculo siga
    // encontrando los encabezados como una fila.
    expect(renglones[1], isEmpty);
    expect(renglones[2], startsWith('switch,ip_switch,puerto'));
  });

  test('el SVG sale bien formado y con una caja por nodo del plano', () {
    final svg = svgDelPlano(plano, encabezado);

    expect(svg, startsWith('<?xml version="1.0" encoding="UTF-8"?>'));
    expect(svg, contains('<svg xmlns="http://www.w3.org/2000/svg"'));
    expect(svg.trimRight(), endsWith('</svg>'));
    expect(svg, contains('Mapa de Matriz'));

    // Una por caja del plano, mas la del fondo.
    final cajas = RegExp('<rect ').allMatches(svg).length;
    expect(cajas, plano.cajas.length + 1);

    // El grupo tras un switch tonto va punteado tambien en el archivo: es la
    // regla de dibujo del proyecto y tiene que sobrevivir a la exportacion.
    expect(svg, contains('stroke-dasharray="6 5"'));

    // La boca con varias MAC dice cuantas conto el servidor, no cuantos
    // renglones llegaron.
    expect(svg, contains('>4 equipos<'));

    // Cada etiqueta abierta se cierra.
    expect(RegExp('<text ').allMatches(svg).length,
        RegExp('</text>').allMatches(svg).length);
  });

  test('el PDF tiene una tabla de referencias que apunta a cada objeto', () {
    final pdf = pdfDelPlano(plano, encabezado);
    final texto = latin1.decode(pdf);

    expect(texto, startsWith('%PDF-1.4'));
    expect(texto.trimRight(), endsWith('%%EOF'));

    // startxref tiene que caer justo en la palabra xref.
    final inicioXref = int.parse(
        RegExp(r'startxref\n(\d+)\n').firstMatch(texto)!.group(1)!);
    expect(texto.substring(inicioXref, inicioXref + 4), 'xref');

    // Y cada renglon de la tabla, en el encabezado de su objeto. Esta es la
    // comprobacion que de verdad importa: un desplazamiento corrido deja el
    // archivo ilegible para la mitad de los visores.
    final renglones = RegExp(r'^(\d{10}) 00000 n $', multiLine: true)
        .allMatches(texto)
        .map((coincidencia) => int.parse(coincidencia.group(1)!))
        .toList();
    expect(renglones.length, 6);
    for (var i = 0; i < renglones.length; i++) {
      expect(texto.substring(renglones[i]), startsWith('${i + 1} 0 obj'));
    }

    // La longitud declarada del flujo de contenido tiene que ser la real.
    final largoDeclarado = int.parse(
        RegExp(r'<< /Length (\d+) >>\nstream\n').firstMatch(texto)!.group(1)!);
    final desde = texto.indexOf('stream\n') + 'stream\n'.length;
    expect(texto.indexOf('\nendstream', desde) - desde, largoDeclarado);
  });

  test('el PDF escapa parentesis, contrabarras y acentos', () {
    final conTrampas = DatosMapa(
      mapa: MapaPuertos(
        capacidad: 'exacta',
        explicacion: 'prueba',
        puertos: [
          PuertoDeSwitch(
            switchId: 1, switchNombre: r'sw (viejo) \ bodega', switchIp: '10.0.0.1',
            indice: 1, puerto: 'Fa0/1', alias: '', activa: true,
            velocidadMbps: 100, mac: '00:11:22:33:44:55', equipoId: 1,
            equipoNombre: 'Camara ñoña — patio', equipoIp: '10.0.0.9',
            confirmado: true, cuantosEnBoca: 1,
          ),
        ],
      ),
      equipos: const [],
    );

    final texto = latin1.decode(pdfDelPlano(
        armarPlano(conTrampas, coloresParaExportar), encabezado));

    // Los parentesis y la contrabarra van escapados; si no, cierran la cadena
    // antes de tiempo y el archivo queda roto.
    expect(texto, contains(r'\(viejo\)'));
    expect(texto, contains(r'\\'));
    // La ñ se escribe como su byte de WinAnsi en octal, no como UTF-8.
    expect(texto, contains(r'\361'));
    // Ningun byte por encima de 255: el flujo tiene que ser puro latin1.
    expect(() => latin1.encode(texto), returnsNormally);
  });

  test('el cable entre switches se dibuja una vez aunque lo vean los dos protocolos', () {
    // El switch anuncia el mismo cable por LLDP y por CDP. Dibujarlo dos veces
    // no agrega nada; lo que si agrega es que los dos coincidan, y eso se marca
    // con un trazo mas grueso.
    expect(datos.mapa.enlaces.length, 2);
    expect(plano.enlaces.length, 1);
    expect(plano.enlaces.first.porAmbos, isTrue);

    final svg = svgDelPlano(plano, encabezado);
    expect(RegExp('<path ').allMatches(svg).length, 1);
    expect(svg, contains('stroke-width="3"'));
    expect(svg, contains('Gi0/1 ↔ Gi0/24'));

    // Y en el PDF, una sola curva cubica. La flecha, que no existe en la fuente
    // del PDF, cae a un guion en vez de a un "?" que se leeria como dato dudoso.
    final pdf = latin1.decode(pdfDelPlano(plano, encabezado));
    expect(RegExp(r' c S').allMatches(pdf).length, 1);
    expect(pdf, contains('(Gi0/1 - Gi0/24)'));
    expect(pdf, isNot(contains('(Gi0/1 ? Gi0/24)')));
  });

  test('un cable a un switch que no esta en el mapa no se dibuja', () {
    // Un switch puede anunciar como vecino a algo que MiRed no ha descubierto.
    // Ahi no hay donde poner el otro extremo del arco: se calla, no se inventa.
    final huerfano = DatosMapa(
      mapa: MapaPuertos(
        capacidad: 'exacta',
        explicacion: 'prueba',
        puertos: datos.mapa.puertos,
        enlaces: const [
          EnlaceEntreEquipos(
            equipoId: 3, equipoNombre: 'sw-principal', interfazLocal: 'Gi0/2',
            vecinoNombre: 'sw-que-nadie-descubrio', vecinoPuerto: 'Gi0/1',
            origen: 'lldp',
          ),
        ],
      ),
      equipos: datos.equipos,
    );

    expect(armarPlano(huerfano, coloresParaExportar).enlaces, isEmpty);
  });

  testWidgets('el PNG sale con la firma de un PNG de verdad', (probador) async {
    // Va dentro de runAsync a la fuerza: convertir el dibujo a imagen lo hace el
    // motor en otro hilo, y el reloj falso de las pruebas nunca le llega el
    // turno. Sin runAsync esto no falla, se queda colgado diez minutos.
    await probador.runAsync(() async {
      final png = await pngDelPlano(plano, encabezado, escala: 1);

      expect(png.length, greaterThan(1000));
      expect(png.sublist(0, 8), [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A]);
    });
  });
}
