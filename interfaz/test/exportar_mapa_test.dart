import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:mired_interfaz/modelos/modelos.dart';
import 'package:mired_interfaz/pantallas/mapa_plano.dart';
import 'package:mired_interfaz/servicios/exportar_mapa.dart';
import 'package:mired_interfaz/servicios/hoja_calculo.dart';

import 'red_de_casa.dart';

/// Pruebas de la exportacion del mapa.
///
/// El SVG, el PDF, el ODS y el XLSX se escriben a mano, byte por byte. Un PDF
/// con un desplazamiento mal calculado en la tabla de referencias —o un ZIP con
/// un indice que no cuadra— abre en un lector tolerante y revienta en otro, asi
/// que aqui se comprueba la ESTRUCTURA del archivo, no solo que la funcion no
/// lance excepcion.
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
          equipoIp: '192.168.1.47', confirmado: true, cuantosEnPuerto: 1,
        ),
        PuertoDeSwitch(
          switchId: 3, switchNombre: 'sw-principal', switchIp: '192.168.1.1',
          indice: 7, puerto: 'Gi0/7', alias: '', activa: true,
          velocidadMbps: 100, mac: 'cc:cc:cc:00:00:20',
          equipoNombre: '', equipoIp: '', confirmado: false, cuantosEnPuerto: 4,
        ),
        PuertoDeSwitch(
          switchId: 8, switchNombre: 'sw-bodega', switchIp: '192.168.1.2',
          indice: 3, puerto: 'Fa0/3', alias: '', activa: true,
          velocidadMbps: 100, mac: 'dd:dd:dd:00:00:01',
          equipoNombre: '', equipoIp: '', confirmado: false, cuantosEnPuerto: 9,
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

  test('la hoja de calculo son DOS tablas, no una mezclada', () {
    final tablas = tablasDelMapa(datos);

    expect(tablas.map((tabla) => tabla.nombre), ['Aparatos', 'Conexiones']);
    // Cada tabla tiene un sujeto: un renglon es un aparato, o es una conexion.
    // Nunca las dos cosas segun la fila, que era el problema de la tabla unica.
    expect(tablas.first.encabezados.first, 'Aparato');
    expect(tablas.last.encabezados.first, 'De');
    for (final tabla in tablas) {
      for (final fila in tabla.filas) {
        expect(fila.length, tabla.encabezados.length,
            reason: 'un renglon con otro numero de columnas descuadra la hoja');
      }
    }
  });

  test('la tabla de aparatos dice de donde cuelga cada uno y con que certeza', () {
    final aparatos = tablasDelMapa(datos).first;
    String filaDe(String aparato) =>
        aparatos.filas.firstWhere((fila) => fila.first.contains(aparato)).join('|');

    expect(filaDe('Impresora de contabilidad'), contains('Gi0/5'));
    expect(filaDe('Impresora de contabilidad'), contains('Puerto exacto'));
    expect(filaDe('Impresora de contabilidad'), contains('SNMP'));

    // El equipo 9 no cuelga de ningun puerto: tiene que salir DICIENDOLO, no
    // desaparecer ni salir con las celdas en blanco, que se lee como si el
    // inventario estuviera completo.
    expect(filaDe('Camara del pasillo'), contains('Sin ubicar'));
  });

  test('el CSV lleva las dos tablas, con su titulo y de cuando son los datos', () {
    // Es la regla del proyecto: todo reporte que salga de MiRed dice de que
    // momento es. Un archivo suelto sin fecha, a la semana, ya no se sabe si
    // sirve — y el CSV es el que mas facil acaba en el correo de alguien.
    final renglones = const LineSplitter().convert(csvDelMapa(datos, encabezado));

    expect(renglones.first, contains('Mapa de Matriz'));
    expect(renglones.first, contains('Exportado el 2026-08-13'));
    // Con un renglon en blanco detras, para que la hoja de calculo siga
    // encontrando los encabezados como una fila.
    expect(renglones[1], isEmpty);

    // El CSV no tiene pestanas: las dos tablas van seguidas y cada una dice como
    // se llama, o quien la abre no sabe donde termina una y empieza la otra.
    expect(renglones.any((r) => r.startsWith('APARATOS')), isTrue);
    expect(renglones.any((r) => r.startsWith('CONEXIONES')), isTrue);
    expect(renglones.any((r) => r.startsWith('Aparato,Que es,Donde esta,IP')), isTrue);
    expect(renglones.any((r) => r.startsWith('De,Por,A,Entra por')), isTrue);

    // Un nombre con coma va entrecomillado o parte el renglon en dos columnas.
    expect(renglones.any((r) => r.contains('"Camara del pasillo, ala norte «ñ»"')),
        isTrue);
  });

  test('el CSV sale con la marca de codificacion, o Excel rompe los acentos', () {
    // Sin estos tres bytes, Excel en Windows abre el archivo con la codificacion
    // local del equipo y «Camara» sale como «CÃ¡mara».
    final bytes = csvEnBytes(csvDelMapa(datos, encabezado));
    expect(bytes.sublist(0, 3), [0xEF, 0xBB, 0xBF]);
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

    // El puerto con varias MAC dice cuantas conto el servidor, no cuantos
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
            confirmado: true, cuantosEnPuerto: 1,
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
    // Se cuenta por el trazo grueso y no por el numero de `<path>`: desde que
    // los cables van en codo, cada cable es tambien un `<path>`, y contarlos
    // todos ya no dice nada de este.
    expect(RegExp('stroke-width="3"').allMatches(svg).length, 1);
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

  // ------------------------------------------ la hoja contra la red de casa ---
  //
  // Los datos armados a mano de arriba no tienen raiz, ni WiFi, ni puertos
  // libres. La red de casa si, y es donde la primera version de la hoja fallo.

  group('la hoja de calculo de la red de casa', () {
    final casa = laCasa();

    /// Las columnas se buscan por su ENCABEZADO y no por su numero: agregar una
    /// columna a la hoja no puede romper una prueba que hablaba de otra cosa.
    String celda(Tabla tabla, List<String> fila, String encabezado) =>
        fila[tabla.encabezados.indexOf(encabezado)];

    test('cada cable sale UNA vez, no una por punta', () {
      final conexiones = tablasDelMapa(casa).last;

      // Cinco puertos declarados en el switch mas el del modem: seis renglones
      // de cable, no once. Antes cada cable salia dos veces —«dvr → switch» y
      // «switch → dvr»— y no habia forma de saber que eran el mismo.
      final cables = conexiones.filas
          .where((fila) => fila[4] == 'Ocupado')
          .map((fila) => '${fila[0]} ${fila[1]} ${fila[2]}')
          .toList();
      expect(cables.length, 4);
      expect(cables.toSet().length, cables.length);
      expect(cables.where((cable) => cable.contains('dvr')).length, 1);
    });

    test('el modem es la raiz: no cuelga del switch al que alimenta', () {
      final aparatos = tablasDelMapa(casa).first;
      final modem =
          aparatos.filas.firstWhere((fila) => fila.first == 'MODEM TELMEX');
      final interruptor =
          aparatos.filas.firstWhere((fila) => fila.first == 'switch 5ptos');

      // El cable modem→switch cuelga al switch DEL modem, no al reves. Leerlo al
      // reves ponia al modem colgando del switch que alimenta, justo lo
      // contrario de lo que dibuja el mapa.
      expect(celda(aparatos, modem, 'Cuelga de'), 'Raiz de la red');
      expect(celda(aparatos, interruptor, 'Cuelga de'), 'MODEM TELMEX');
      expect(celda(aparatos, interruptor, 'Puerto'), 'LAN 1');
    });

    test('lo que cuelga por el aire tambien sale, con su antena y su red', () {
      final tablas = tablasDelMapa(casa);
      final porElAire = tablas.first.filas
          .where((fila) => celda(tablas.first, fila, 'Conexion') == 'WiFi')
          .toList();

      // Cuatro equipos colgados de la antena. En la tabla vieja no salia ninguno
      // de los cuatro: como el WiFi no tiene puertos y la tabla iba por puertos,
      // desaparecian del archivo aunque el mapa los dibujara.
      expect(porElAire.length, 4);
      expect(
          porElAire.every((fila) => celda(tablas.first, fila, 'Cuelga de') == 'AP ubiquiti'),
          isTrue);
      expect(
          porElAire.every((fila) => celda(tablas.first, fila, 'Puerto') == 'WiFi «casa»'),
          isTrue);
      expect(tablas.last.filas.where((fila) => fila[4] == 'Por el aire').length, 4);
    });

    test('el puerto libre y lo sin ubicar se dicen, no se callan', () {
      final tablas = tablasDelMapa(casa);

      final libre = tablas.last.filas.where((fila) => fila[4] == 'Libre').toList();
      expect(libre.length, 1);
      expect(libre.single[1], 'LAN 4');

      final perdido = tablas.first.filas
          .firstWhere((fila) => celda(tablas.first, fila, 'Cuelga de') == 'Sin ubicar');
      expect(perdido.first, '192.168.1.71');
    });

    test('la hoja lleva las contrasenas de los aparatos', () {
      // Decision del usuario: «si se exporta el mapa de la red tambien va la
      // contrasena, no importa que se vea». Sin ellas el archivo no sirve para
      // mudar una instalacion a otro equipo ni para entregarsela a nadie.
      final tablas = tablasDelMapa(casa, credenciales: const [
        CredencialEquipo(
            id: 1, equipoId: 1, tipo: 'web', usuario: 'admin',
            clave: 'laClaveDelModem', direccion: 'http://192.168.1.254'),
      ]);
      final aparatos = tablas.first;
      final modem =
          aparatos.filas.firstWhere((fila) => fila.first == 'MODEM TELMEX');

      expect(aparatos.encabezados, contains('Clave'));
      expect(celda(aparatos, modem, 'Usuario'), 'admin');
      expect(celda(aparatos, modem, 'Clave'), 'laClaveDelModem');
      expect(celda(aparatos, modem, 'Direccion del panel'), 'http://192.168.1.254');

      // Y el que no tiene, sale vacio: no se inventa nada.
      final switchTonto =
          aparatos.filas.firstWhere((fila) => fila.first == 'switch 5ptos');
      expect(celda(aparatos, switchTonto, 'Clave'), isEmpty);
    });

    test('los puertos se llaman como en el mapa: LAN 3, no «puerto 3»', () {
      final conexiones = tablasDelMapa(casa).last;
      final aLaPc =
          conexiones.filas.firstWhere((fila) => fila[2] == 'pc tuxor');

      // El mapa rotula ese cable «LAN 3 → LAN 1». La hoja tiene que decir lo
      // mismo: si una pantalla llama al puerto de dos maneras, no hay dato.
      expect(aLaPc[1], 'LAN 3');
      expect(aLaPc[3], 'LAN 1');
    });
  });

  // ------------------------------------------------------ ODS, XLSX y su ZIP ---

  group('las hojas de calculo son paquetes bien armados', () {
    final casa = laCasa();
    final momento = DateTime.utc(2026, 8, 17, 9, 30);

    test('el ODS lleva el mimetype primero y una hoja por tabla', () {
      final ods = odsDelMapa(casa, encabezado, momento);
      final texto = latin1.decode(ods);

      _comprobarZip(ods, 4);
      // El mimetype tiene que ir el PRIMERO y sin comprimir, o el lector no
      // reconoce el paquete como una hoja de calculo.
      expect(texto.indexOf('mimetype'), 30);
      expect(texto.indexOf('application/vnd.oasis.opendocument.spreadsheet'),
          lessThan(100));
      expect(texto, contains('<table:table table:name="Aparatos">'));
      expect(texto, contains('<table:table table:name="Conexiones">'));
    });

    test('el XLSX declara sus dos hojas y las lleva dentro', () {
      final xlsx = xlsxDelMapa(casa, encabezado, momento);
      final texto = latin1.decode(xlsx);

      _comprobarZip(xlsx, 6);
      expect(texto, contains('<sheet name="Aparatos"'));
      expect(texto, contains('<sheet name="Conexiones"'));
      expect(texto, contains('xl/hojas/hoja1.xml'));
      expect(texto, contains('xl/hojas/hoja2.xml'));
      // Cada celda lleva su texto dentro, sin tabla compartida aparte.
      expect(texto, contains('<c r="A5" t="inlineStr">'));
    });

    test('el mismo mapa exportado dos veces da el mismo archivo', () {
      // La fecha entra como dato, no la pregunta el reloj: sin eso dos
      // exportaciones del mismo mapa darian archivos distintos y no habria forma
      // de comparar nada.
      expect(odsDelMapa(casa, encabezado, momento),
          odsDelMapa(casa, encabezado, momento));
    });
  });
}

/// _comprobarZip revisa que el indice del final del paquete cuadre.
///
/// Un ZIP se lee de atras hacia adelante: el bloque final dice cuantas entradas
/// hay y donde empieza el indice. Si ese numero esta mal, unos lectores abren el
/// archivo y otros lo dan por corrupto — el mismo peligro que la tabla de
/// referencias del PDF.
void _comprobarZip(Uint8List paquete, int cuantasEntradas) {
  expect(paquete.sublist(0, 4), [0x50, 0x4B, 0x03, 0x04], reason: 'no empieza como un ZIP');

  final fin = paquete.length - 22; // el bloque final, sin comentario
  expect(paquete.sublist(fin, fin + 4), [0x50, 0x4B, 0x05, 0x06]);

  int leer2(int donde) => paquete[donde] | (paquete[donde + 1] << 8);
  int leer4(int donde) =>
      paquete[donde] |
      (paquete[donde + 1] << 8) |
      (paquete[donde + 2] << 16) |
      (paquete[donde + 3] << 24);

  expect(leer2(fin + 10), cuantasEntradas);
  final inicioIndice = leer4(fin + 16);
  expect(leer4(fin + 12), paquete.length - 22 - inicioIndice,
      reason: 'el tamano del indice no cuadra con donde empieza');
  expect(paquete.sublist(inicioIndice, inicioIndice + 4), [0x50, 0x4B, 0x01, 0x02],
      reason: 'el indice no empieza donde el bloque final dice');
}
