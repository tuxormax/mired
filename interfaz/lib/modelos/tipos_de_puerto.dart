import 'package:flutter/material.dart';

/// La LISTA UNICA de lo que puede ser un puerto fisico.
///
/// **Este archivo viaja en paralelo con `TiposDePuerto` de
/// `internal/basedatos/topologiamanual.go` y con el CHECK de la migracion red
/// 0018.** Hay una prueba en Go que compara los dos y falla si se separan.
///
/// Un puerto no se llama "puerto 3": se llama **LAN 3**, o WAN 1, o DMZ 1, que
/// es como esta rotulado en el aparato. En un modem con LAN, WAN y DMZ, decir
/// "puerto 3" no dice por donde sale el cable, y el mapa esta justo para eso.
class TipoDePuerto {
  const TipoDePuerto({
    required this.clave,
    required this.nombre,
    required this.icono,
    required this.explicacion,
  });

  /// Lo que se guarda en la base. No cambia nunca.
  final String clave;

  /// Como se rotula en el aparato, y como se escribe en el mapa.
  final String nombre;

  final IconData icono;

  /// Que es, para quien no lo tenga claro al declararlo.
  final String explicacion;
}

const List<TipoDePuerto> tiposDePuerto = [
  TipoDePuerto(
    clave: 'lan',
    nombre: 'LAN',
    icono: Icons.settings_ethernet,
    explicacion: 'Hacia dentro de la red: lo normal',
  ),
  TipoDePuerto(
    clave: 'wan',
    nombre: 'WAN',
    icono: Icons.public,
    explicacion: 'Hacia el proveedor de internet',
  ),
  TipoDePuerto(
    clave: 'dmz',
    nombre: 'DMZ',
    icono: Icons.shield_outlined,
    explicacion: 'Zona aparte, fuera de la red de casa',
  ),
  TipoDePuerto(
    clave: 'sfp',
    nombre: 'SFP',
    icono: Icons.flare_outlined,
    explicacion: 'Ranura de fibra',
  ),
  TipoDePuerto(
    clave: 'consola',
    nombre: 'CONSOLA',
    icono: Icons.terminal,
    explicacion: 'Solo para administrar el aparato; no lleva red',
  ),
];

/// El tipo, buscado por su clave. Lo que no este en la lista NO se inventa: se
/// devuelve vacio y quien llame decide, en vez de dibujar un puerto que no es.
TipoDePuerto? tipoDePuertoPorClave(String clave) {
  for (final tipo in tiposDePuerto) {
    if (tipo.clave == clave) return tipo;
  }
  return null;
}

/// nombreDePuerto arma como se llama un puerto: `LAN 3`, `WAN 1`, `DMZ 1`.
///
/// Es UNA funcion y la usan todos —el mapa, la ficha del aparato, la lista de
/// puertos— para que el mismo puerto no se llame de dos maneras segun la
/// pantalla en que se mire.
String nombreDePuerto(String tipo, int numero) {
  final conocido = tipoDePuertoPorClave(tipo);
  return '${conocido?.nombre ?? tipo.toUpperCase()} $numero';
}

/// iconoDePuerto es el dibujito del tipo. Lo desconocido va con el generico: un
/// icono inventado sugeriria algo que no se sabe.
IconData iconoDePuerto(String tipo) =>
    tipoDePuertoPorClave(tipo)?.icono ?? Icons.settings_ethernet;
