import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mired_interfaz/modelos/modelos.dart';
import 'package:mired_interfaz/pantallas/zz_plano_viejo.dart';

void main() {
  test('cuanto medía el lienzo viejo con la red de tuxor', () {
    Equipo eq(int id, String n, {String ip = ''}) => Equipo(
        id: id, ip: ip, mac: '', fabricante: '', nombre: n, alias: '', tipo: '',
        subred: '', metodo: '', presente: true, origen: 'manual',
        primeraVez: '', ultimaVez: '', puertos: const []);

    final datos = DatosMapaViejo(
      mapa: const MapaPuertos(capacidad: 'ninguna', explicacion: '', puertos: [], enlaces: []),
      equipos: [
        eq(1, 'MODEM TELMEX', ip: '192.168.1.254'), eq(2, 'switch'),
        eq(3, 'AP ubiquiti', ip: '192.168.1.86'), eq(4, 'pc tuxor'), eq(5, 'dvr'),
        eq(6, 'tv samsung'), eq(7, '192.168.1.69'), eq(8, '192.168.1.70'), eq(9, '192.168.1.77'),
      ],
      topologia: TopologiaManual(
        puertos: const [
          PuertoFisico(id: 10, equipoId: 1, numero: 1, tipo: 'lan'),
          PuertoFisico(id: 20, equipoId: 2, numero: 1, tipo: 'lan'),
          PuertoFisico(id: 21, equipoId: 2, numero: 2, tipo: 'lan'),
          PuertoFisico(id: 22, equipoId: 2, numero: 3, tipo: 'lan'),
          PuertoFisico(id: 23, equipoId: 2, numero: 4, tipo: 'lan'),
          PuertoFisico(id: 24, equipoId: 2, numero: 5, tipo: 'lan'),
        ],
        enlaces: const [
          EnlaceFisico(id: 1, puertoOrigenId: 10, equipoOrigenId: 1, numeroOrigen: 1,
              origenNombre: 'MODEM TELMEX', puertoDestinoId: 24, numeroDestino: 5,
              destinoNombre: 'switch', origenDato: 'manual'),
          EnlaceFisico(id: 2, puertoOrigenId: 20, equipoOrigenId: 2, numeroOrigen: 1,
              origenNombre: 'switch', equipoDestinoId: 5, destinoNombre: 'dvr', origenDato: 'manual'),
          EnlaceFisico(id: 3, puertoOrigenId: 21, equipoOrigenId: 2, numeroOrigen: 2,
              origenNombre: 'switch', equipoDestinoId: 3, destinoNombre: 'AP ubiquiti', origenDato: 'manual'),
          EnlaceFisico(id: 4, puertoOrigenId: 22, equipoOrigenId: 2, numeroOrigen: 3,
              origenNombre: 'switch', equipoDestinoId: 4, destinoNombre: 'pc tuxor', origenDato: 'manual'),
        ],
        inalambricos: const [
          EnlaceInalambrico(id: 1, equipoId: 6, antenaId: 3, equipoNombre: 'tv samsung', origenDato: 'manual'),
          EnlaceInalambrico(id: 2, equipoId: 7, antenaId: 3, equipoNombre: '.69', origenDato: 'manual'),
          EnlaceInalambrico(id: 3, equipoId: 8, antenaId: 3, equipoNombre: '.70', origenDato: 'manual'),
          EnlaceInalambrico(id: 4, equipoId: 9, antenaId: 3, equipoNombre: '.77', origenDato: 'manual'),
        ],
      ),
    );

    final plano = armarPlanoViejo(datos, ColorScheme.fromSeed(seedColor: const Color(0xFF1565C0)));
    final pixeles = plano.tamano.width * plano.tamano.height;
    print('LIENZO VIEJO: ${plano.tamano}  = ${(pixeles / 1e6).toStringAsFixed(1)} Mpx');
    print('a 3x de zoom: ${(pixeles * 9 / 1e6).toStringAsFixed(0)} Mpx '
        '= ${(pixeles * 9 * 4 / 1e9).toStringAsFixed(2)} GB de textura');
  });
}
