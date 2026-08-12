/// Trayectoria guarda las ultimas cosas que hizo el usuario.
///
/// Sin esto, un reporte de error dice que fallo pero no como se llego ahi, y
/// reproducir el problema se vuelve adivinanza.
class Trayectoria {
  Trayectoria._();
  static final Trayectoria instancia = Trayectoria._();

  static const _maximo = 10;
  final List<String> _pasos = [];

  void anotar(String paso) {
    final ahora = DateTime.now();
    final hora = '${_dos(ahora.hour)}:${_dos(ahora.minute)}:${_dos(ahora.second)}';
    _pasos.add('  $hora  $paso');
    if (_pasos.length > _maximo) _pasos.removeAt(0);
  }

  String get texto => _pasos.isEmpty ? '  (sin pasos registrados)' : _pasos.join('\n');

  static String _dos(int numero) => numero.toString().padLeft(2, '0');
}
