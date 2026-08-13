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
