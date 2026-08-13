/// Traduce lo que una persona escribe a lo que el escaneo entiende.
///
/// # Por que existe
///
/// Antes el formulario pedia una "subred" en notacion de administrador de redes
/// —`192.168.1.0/24`— y quien no la conoce no tiene forma de adivinarla. Es
/// pedirle al usuario que haga una cuenta que el programa puede hacer solo.
///
/// Ahora **lo normal es no escribir nada**: el programa propone la red que
/// detecto. Y para quien quiera acotar, este archivo acepta las tres formas en
/// que la gente lo dice de verdad:
///
///     192.168.1.0/24                    la notacion de siempre, para quien la sepa
///     192.168.1.100                     una direccion suelta: se toma SU red
///     192.168.1.10 - 192.168.1.50       de aqui hasta aca
///
/// Todo acaba convertido a la notacion con barra, porque es la que usa el resto
/// del sistema. **La traduccion se hace aqui y en ningun otro sitio**: si cada
/// pantalla interpretara el texto a su manera, dos formularios acabarian
/// entendiendo cosas distintas.
library;

/// Interpretacion es lo que se entendio de un texto, listo para mostrarselo al
/// usuario antes de que le de a Crear.
class Interpretacion {
  const Interpretacion({
    required this.subredes,
    required this.explicacion,
    required this.cuantasDirecciones,
  });

  /// Lo que se le manda al servidor, siempre en notacion con barra.
  final List<String> subredes;

  /// Lo que se le dice al usuario: "Se escanearan 254 direcciones, de la
  /// 192.168.1.1 a la 192.168.1.254". **Sin esto la flexibilidad seria magia**:
  /// el usuario escribe algo, el programa entiende otra cosa y nadie se entera
  /// hasta que el escaneo sale raro.
  final String explicacion;

  final int cuantasDirecciones;

  bool get seEntendio => subredes.isNotEmpty;

  static const noSeEntendio = Interpretacion(
    subredes: [],
    explicacion: '',
    cuantasDirecciones: 0,
  );
}

/// interpretar traduce lo que se escribio. Devuelve `noSeEntendio` si no hay
/// forma de sacar nada en claro.
Interpretacion interpretar(String texto) {
  final limpio = texto.trim();
  if (limpio.isEmpty) return Interpretacion.noSeEntendio;

  // "de aqui hasta aca". Se acepta el guion con o sin espacios, y tambien la
  // palabra, porque es como lo escribe la gente.
  final separado = limpio.split(RegExp(r'\s+(?:a|hasta)\s+|\s*-\s*'));
  if (separado.length == 2) {
    return _deRango(separado[0], separado[1]);
  }

  // "192.168.1.0/24", la notacion de siempre.
  if (limpio.contains('/')) return _deNotacionConBarra(limpio);

  // Una direccion suelta: se entiende que quiere SU red.
  final direccion = _leerDireccion(limpio);
  if (direccion == null) return Interpretacion.noSeEntendio;
  return _deNotacionConBarra('${_conUltimoEnCero(direccion)}/24');
}

/// deLaRedDetectada arma la propuesta a partir de lo que reporto la sonda.
///
/// Recibe direcciones como las da el sistema (`192.168.1.100/24`) y devuelve la
/// red a la que pertenecen. Es lo que permite que el formulario venga relleno y
/// que el usuario no tenga que saber que es una mascara.
Interpretacion? deLaRedDetectada(String direccionConMascara) {
  final partes = direccionConMascara.split('/');
  if (partes.length != 2) return null;

  final direccion = _leerDireccion(partes[0]);
  final bits = int.tryParse(partes[1]);
  if (direccion == null || bits == null) return null;

  // Una red mas grande que /16 son mas de 65 000 direcciones: eso no es "mi
  // red", es media internet, y barrerla tardaria horas. No se propone.
  if (bits < 16 || bits > 30) return null;

  final resultado = _deNotacionConBarra('${_enCero(direccion, bits)}/$bits');
  return resultado.seEntendio ? resultado : null;
}

// ------------------------------------------------------------------ dentro --

Interpretacion _deNotacionConBarra(String texto) {
  final partes = texto.split('/');
  if (partes.length != 2) return Interpretacion.noSeEntendio;

  final direccion = _leerDireccion(partes[0]);
  final bits = int.tryParse(partes[1]);
  if (direccion == null || bits == null || bits < 0 || bits > 32) {
    return Interpretacion.noSeEntendio;
  }

  final numeroDeRed = _aNumero(direccion) & _mascara(bits);
  final red = _aTexto(numeroDeRed);
  final cuantas = 1 << (32 - bits);
  // La primera direccion es la de la red y la ultima la de difusion: ninguna de
  // las dos es un equipo.
  final utiles = cuantas <= 2 ? cuantas : cuantas - 2;

  final primera = cuantas <= 2 ? numeroDeRed : numeroDeRed + 1;
  final ultima = primera + utiles - 1;

  return Interpretacion(
    subredes: ['$red/$bits'],
    explicacion: _explicar(utiles, _aTexto(primera), _aTexto(ultima)),
    cuantasDirecciones: utiles,
  );
}

Interpretacion _deRango(String textoDesde, String textoHasta) {
  final desde = _leerDireccion(textoDesde.trim());
  final hasta = _leerDireccion(textoHasta.trim());
  if (desde == null || hasta == null) return Interpretacion.noSeEntendio;

  var inicio = _aNumero(desde);
  var fin = _aNumero(hasta);
  // Si se escribieron al reves, se entiende igual: nadie quiere un error por eso.
  if (inicio > fin) {
    final vuelta = inicio;
    inicio = fin;
    fin = vuelta;
  }

  final cuantas = fin - inicio + 1;
  if (cuantas > 65536) return Interpretacion.noSeEntendio;

  return Interpretacion(
    subredes: _enBloques(inicio, fin),
    explicacion: _explicar(cuantas, _aTexto(inicio), _aTexto(fin)),
    cuantasDirecciones: cuantas,
  );
}

/// _enBloques parte un rango en los bloques con barra mas grandes que quepan.
///
/// El resto del sistema —la base, el escaneo, las alertas— habla en notacion con
/// barra. Traducir aqui evita cambiar todo eso solo porque el formulario pregunte
/// de otra forma, que es justo lo contrario de un cambio quirurgico.
List<String> _enBloques(int inicio, int fin) {
  final bloques = <String>[];
  var actual = inicio;

  while (actual <= fin) {
    // El bloque mas grande que puede empezar aqui sin desalinearse...
    var bits = 32;
    while (bits > 0) {
      final tamano = 1 << (32 - (bits - 1));
      if (actual % tamano != 0) break;
      if (actual + tamano - 1 > fin) break;
      bits--;
    }
    bloques.add('${_aTexto(actual)}/$bits');
    actual += 1 << (32 - bits);
  }
  return bloques;
}

String _explicar(int cuantas, String primera, String ultima) {
  if (cuantas == 1) return 'Se revisara solo $primera.';
  return 'Se revisaran $cuantas direcciones, de la $primera a la $ultima.';
}

List<int>? _leerDireccion(String texto) {
  final partes = texto.split('.');
  if (partes.length != 4) return null;

  final numeros = <int>[];
  for (final parte in partes) {
    final numero = int.tryParse(parte);
    if (numero == null || numero < 0 || numero > 255) return null;
    numeros.add(numero);
  }
  return numeros;
}

String _conUltimoEnCero(List<int> direccion) =>
    '${direccion[0]}.${direccion[1]}.${direccion[2]}.0';

/// _enCero apaga los bits que no son de la red, que es lo que convierte una
/// direccion cualquiera en el nombre de su red.
String _enCero(List<int> direccion, int bits) =>
    _aTexto(_aNumero(direccion) & _mascara(bits));

int _mascara(int bits) =>
    bits == 0 ? 0 : (0xFFFFFFFF << (32 - bits)) & 0xFFFFFFFF;

int _aNumero(List<int> direccion) =>
    direccion[0] << 24 | direccion[1] << 16 | direccion[2] << 8 | direccion[3];

String _aTexto(int numero) => '${(numero >> 24) & 255}.${(numero >> 16) & 255}'
    '.${(numero >> 8) & 255}.${numero & 255}';
