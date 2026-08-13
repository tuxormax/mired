library;

/// Dice cuando fue la ultima vez y **que tan viejo es eso**.
///
/// # Por que existe
///
/// La fecha del ultimo escaneo se guardaba desde el principio, pero se mostraba
/// tal cual: `2026-08-13T15:12:04-06:00`. Eso obliga a quien lo lee a hacer la
/// cuenta de cabeza, y sobre todo **no responde la pregunta que de verdad
/// importa**: ¿lo que estoy viendo sigue siendo cierto?
///
/// Un inventario de red envejece. Lo de hace diez minutos es la realidad; lo de
/// hace tres semanas es historia, y quien lo mire tiene que saberlo sin tener
/// que fijarse en una fecha.

/// Frescura es que tan al dia esta un dato.
enum Frescura {
  /// Nunca se ha escaneado.
  nunca,

  /// Recien hecho: menos de una hora.
  alDia,

  /// De hoy: sirve para trabajar.
  reciente,

  /// Mas de un dia: puede haber cambiado algo.
  viejo,

  /// Mas de una semana: casi seguro ya no es cierto.
  muyViejo,
}

/// Cuando junta el texto para una persona y que tan viejo es.
class Cuando {
  const Cuando(this.texto, this.frescura);

  final String texto;
  final Frescura frescura;
}

/// haceCuanto traduce una fecha ISO a algo que se lee de un vistazo.
///
/// Devuelve tambien la frescura, para que la pantalla pueda avisar sin que nadie
/// tenga que interpretar la fecha.
Cuando haceCuanto(String? fechaISO) {
  if (fechaISO == null || fechaISO.trim().isEmpty) {
    return const Cuando('Sin escanear todavia', Frescura.nunca);
  }

  final momento = DateTime.tryParse(fechaISO);
  if (momento == null) {
    // Una fecha que no se puede leer no se disimula: decir "hace un momento"
    // seria inventar. Se muestra tal cual y que se vea que algo no cuadra.
    return Cuando(fechaISO, Frescura.viejo);
  }

  final transcurrido = DateTime.now().difference(momento);

  // Una fecha en el futuro significa que los relojes no coinciden —tipico al
  // conectarse al MiRed de otro equipo—. No se trata como recien hecho.
  if (transcurrido.isNegative) {
    return const Cuando('Hace un momento', Frescura.alDia);
  }

  final minutos = transcurrido.inMinutes;
  final horas = transcurrido.inHours;
  final dias = transcurrido.inDays;

  late final String texto;
  if (minutos < 1) {
    texto = 'Hace un momento';
  } else if (minutos < 60) {
    texto = 'Hace $minutos ${minutos == 1 ? 'minuto' : 'minutos'}';
  } else if (horas < 24) {
    texto = 'Hace $horas ${horas == 1 ? 'hora' : 'horas'}';
  } else if (dias < 30) {
    texto = 'Hace $dias ${dias == 1 ? 'dia' : 'dias'}';
  } else {
    final meses = dias ~/ 30;
    texto = 'Hace $meses ${meses == 1 ? 'mes' : 'meses'}';
  }

  late final Frescura frescura;
  if (horas < 1) {
    frescura = Frescura.alDia;
  } else if (horas < 24) {
    frescura = Frescura.reciente;
  } else if (dias < 7) {
    frescura = Frescura.viejo;
  } else {
    frescura = Frescura.muyViejo;
  }

  return Cuando(texto, frescura);
}

/// aviso dice, cuando hace falta, por que conviene volver a escanear.
///
/// Devuelve vacio si el dato esta al dia: un aviso permanente deja de leerse.
String aviso(Frescura frescura) {
  switch (frescura) {
    case Frescura.nunca:
      return 'Escanee para descubrir los equipos de esta red.';
    case Frescura.viejo:
      return 'Lo que ve puede haber cambiado. Conviene escanear de nuevo.';
    case Frescura.muyViejo:
      return 'Esta informacion tiene mas de una semana: casi seguro ya no es '
          'la de ahora.';
    case Frescura.alDia:
    case Frescura.reciente:
      return '';
  }
}
