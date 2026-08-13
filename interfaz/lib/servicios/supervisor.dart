library;

/// supervisor arranca y detiene los servicios que MiRed necesita para funcionar.
///
/// **Es lo que convierte a MiRed en un programa.** Al abrir la ventana se
/// levantan `mired-servidor` y `mired-sonda`; al cerrarla se matan. Se instala y
/// ya: no hay nada que habilitar, ni que arrancar a mano, ni que dejar
/// corriendo.
///
/// El precio de esa simplicidad hay que tenerlo claro: **con el programa cerrado
/// MiRed no vigila nada.** No hay escaneos programados, no llegan alertas de
/// equipo desconocido, y los flujos que exporte el router se pierden. Quien
/// necesite vigilancia continua —una Raspberry en una sucursal, sin nadie que
/// abra un programa— tiene las unidades de systemd que trae el paquete,
/// instaladas y apagadas.
export 'supervisor_generico.dart';
