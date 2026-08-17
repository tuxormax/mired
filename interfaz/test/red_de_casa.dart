import 'package:mired_interfaz/modelos/modelos.dart';
import 'package:mired_interfaz/pantallas/mapa_plano.dart';

/// La red real donde se prueba MiRed, con la forma exacta que tiene en el mapa.
///
/// No es un caso inventado: es la casa del usuario, y por eso vale como prueba.
/// Trae de una vez las cuatro cosas que los datos armados a mano no traian y que
/// rompieron la primera version de la hoja de calculo:
///  - una **raiz** (el modem) de la que cuelga todo, y que no cuelga de nadie;
///  - un aparato **de punta sin puertos declarados** (la PC);
///  - cuatro equipos colgados **por el aire** de una antena, sin puerto ninguno;
///  - un **puerto libre** y un aparato **sin ubicar**.
///
/// Nada de esto contesta SNMP: la red entera esta declarada a mano.
DatosMapa laCasa() {
  const equipos = [
    Equipo(
        id: 1, ip: '192.168.1.254', mac: 'a0:95:7f:bf:5c:60', fabricante: '',
        nombre: '', alias: 'MODEM TELMEX', tipo: '', subred: '192.168.1.0/24',
        metodo: 'manual', presente: true, primeraVez: '', ultimaVez: '2026-08-17',
        puertos: [], origen: 'manual', categoria: 'gateway'),
    Equipo(
        id: 2, ip: '', mac: '', fabricante: '', nombre: '', alias: 'switch 5ptos',
        tipo: '', subred: '', metodo: 'manual', presente: false, primeraVez: '',
        ultimaVez: '', puertos: [], origen: 'manual', categoria: 'switch_simple'),
    Equipo(
        id: 3, ip: '192.168.1.74', mac: '', fabricante: '', nombre: '',
        alias: 'dvr', tipo: '', subred: '192.168.1.0/24', metodo: 'arp',
        presente: true, primeraVez: '', ultimaVez: '2026-08-17', puertos: [],
        origen: 'manual', categoria: 'camara'),
    Equipo(
        id: 4, ip: '192.168.1.86', mac: '', fabricante: 'Ubiquiti', nombre: '',
        alias: 'AP ubiquiti', tipo: '', subred: '192.168.1.0/24', metodo: 'arp',
        presente: true, primeraVez: '', ultimaVez: '2026-08-17', puertos: [],
        origen: 'manual', categoria: 'punto_de_acceso'),
    Equipo(
        id: 5, ip: '192.168.1.100', mac: 'b4:2e:99:3b:e3:76', fabricante: '',
        nombre: '', alias: 'pc tuxor', tipo: '', subred: '192.168.1.0/24',
        metodo: 'arp', presente: true, primeraVez: '', ultimaVez: '2026-08-17',
        puertos: [], origen: 'manual', categoria: 'computadora'),
    Equipo(
        id: 6, ip: '192.168.1.68', mac: '', fabricante: 'Samsung', nombre: '',
        alias: 'tv samsung', tipo: '', subred: '192.168.1.0/24', metodo: 'manual',
        presente: false, primeraVez: '', ultimaVez: '', puertos: [],
        origen: 'manual', categoria: 'multimedia', conexion: 'wifi'),
    Equipo(
        id: 7, ip: '192.168.1.69', mac: '', fabricante: '', nombre: '', alias: '',
        tipo: '', subred: '192.168.1.0/24', metodo: 'arp', presente: true,
        primeraVez: '', ultimaVez: '2026-08-17', puertos: [], origen: 'manual',
        conexion: 'wifi'),
    Equipo(
        id: 8, ip: '192.168.1.70', mac: '', fabricante: '', nombre: '', alias: '',
        tipo: '', subred: '192.168.1.0/24', metodo: 'arp', presente: true,
        primeraVez: '', ultimaVez: '2026-08-17', puertos: [], origen: 'manual',
        conexion: 'wifi'),
    Equipo(
        id: 9, ip: '192.168.1.77', mac: '', fabricante: '', nombre: '', alias: '',
        tipo: '', subred: '192.168.1.0/24', metodo: 'arp', presente: true,
        primeraVez: '', ultimaVez: '2026-08-17', puertos: [], origen: 'manual',
        conexion: 'wifi'),
    Equipo(
        id: 10, ip: '192.168.1.71', mac: '08:65:f0:0b:d4:ba', fabricante: '',
        nombre: '', alias: '', tipo: '', subred: '192.168.1.0/24', metodo: 'arp',
        presente: true, primeraVez: '', ultimaVez: '2026-08-17', puertos: []),
  ];

  return const DatosMapa(
    mapa: MapaPuertos(
        capacidad: 'ninguna', explicacion: 'Nadie contesta SNMP', puertos: []),
    equipos: equipos,
    topologia: TopologiaManual(
      puertos: [
        PuertoFisico(id: 100, equipoId: 1, numero: 1, tipo: 'lan'),
        PuertoFisico(id: 201, equipoId: 2, numero: 1, tipo: 'lan'),
        PuertoFisico(id: 202, equipoId: 2, numero: 2, tipo: 'lan'),
        PuertoFisico(id: 203, equipoId: 2, numero: 3, tipo: 'lan'),
        PuertoFisico(id: 204, equipoId: 2, numero: 4, tipo: 'lan'),
        PuertoFisico(id: 205, equipoId: 2, numero: 5, tipo: 'lan'),
        PuertoFisico(id: 300, equipoId: 3, numero: 1, tipo: 'lan'),
        PuertoFisico(id: 400, equipoId: 4, numero: 1, tipo: 'lan'),
      ],
      enlaces: [
        EnlaceFisico(
            id: 1, puertoOrigenId: 100, equipoOrigenId: 1, numeroOrigen: 1,
            origenNombre: 'MODEM TELMEX', puertoDestinoId: 205,
            equipoDestinoId: 2, numeroDestino: 5, destinoNombre: 'switch 5ptos',
            origenDato: 'manual'),
        EnlaceFisico(
            id: 2, puertoOrigenId: 201, equipoOrigenId: 2, numeroOrigen: 1,
            origenNombre: 'switch 5ptos', puertoDestinoId: 300,
            equipoDestinoId: 3, numeroDestino: 1, destinoNombre: 'dvr',
            origenDato: 'manual'),
        EnlaceFisico(
            id: 3, puertoOrigenId: 202, equipoOrigenId: 2, numeroOrigen: 2,
            origenNombre: 'switch 5ptos', puertoDestinoId: 400,
            equipoDestinoId: 4, numeroDestino: 1, destinoNombre: 'AP ubiquiti',
            origenDato: 'manual'),
        EnlaceFisico(
            id: 4, puertoOrigenId: 203, equipoOrigenId: 2, numeroOrigen: 3,
            origenNombre: 'switch 5ptos', equipoDestinoId: 5,
            destinoNombre: 'pc tuxor', origenDato: 'manual'),
      ],
      inalambricos: [
        EnlaceInalambrico(id: 1, equipoId: 6, antenaId: 4, red: 'casa'),
        EnlaceInalambrico(id: 2, equipoId: 7, antenaId: 4, red: 'casa'),
        EnlaceInalambrico(id: 3, equipoId: 8, antenaId: 4, red: 'casa'),
        EnlaceInalambrico(id: 4, equipoId: 9, antenaId: 4, red: 'casa'),
      ],
    ),
  );
}

