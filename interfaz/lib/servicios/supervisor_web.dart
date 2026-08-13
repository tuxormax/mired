/// Version web: no hay nada que supervisar.
///
/// Cuando MiRed se abre desde el navegador, la pagina la sirvio el propio
/// `mired-servidor`. Que esta interfaz este corriendo YA demuestra que el
/// servidor esta vivo, asi que arrancarlo seria absurdo — y matarlo al cerrar la
/// pestana, un desastre: dejaria sin servicio a todos los demas navegadores
/// conectados.
class Servicios {
  Servicios._();
  static final Servicios instancia = Servicios._();

  String? get aviso => null;
  bool get losArranqueYo => false;

  Future<void> arrancar() async {}
  Future<void> detener() async {}
}
