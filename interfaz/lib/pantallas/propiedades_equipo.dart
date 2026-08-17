import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../modelos/categorias.dart';
import '../modelos/modelos.dart';
import '../servicios/api.dart';
import '../widgets/mensajes.dart';
import 'topologia_manual.dart';

/// Lo que se sabe de un aparato, y como se entra a el.
///
/// Vive aparte porque lo usan dos: la lista de equipos, donde la tarjeta se
/// despliega, y la ficha que abre el mapa. Si estuviera dentro de una de las
/// dos, la otra acabaria con una copia que se desincroniza.

/// PropiedadesDelEquipo dibuja todo lo que se sabe de un aparato.
class PropiedadesDelEquipo extends StatelessWidget {
  const PropiedadesDelEquipo({
    super.key,
    required this.clave,
    required this.equipo,
    required this.alCambiar,
  });

  final String clave;
  final Equipo equipo;
  final VoidCallback alCambiar;

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (equipo.puertos.isNotEmpty) ...[
          Wrap(
            spacing: 6,
            runSpacing: 6,
            children: [
              for (final puerto in equipo.puertos)
                Tooltip(
                  message: puerto.banner.isEmpty ? puerto.etiqueta : puerto.banner,
                  child: Chip(
                    label: Text(puerto.etiqueta),
                    visualDensity: VisualDensity.compact,
                  ),
                ),
            ],
          ),
          const SizedBox(height: 12),
        ],
        RenglonDeFicha(etiqueta: 'Reconocido como', valor: equipo.tipo),
        // La categoria es con la que cuenta el contador de la lista. Se muestra
        // para poder ver POR QUE un aparato cae en un cubo y no en otro.
        RenglonDeFicha(
            etiqueta: 'Cuenta como',
            valor: equipo.categoria.isEmpty
                ? 'Sin reconocer'
                : nombreDeCategoria(equipo.categoria)),
        RenglonDeFicha(etiqueta: 'Modelo', valor: equipo.modelo),
        // Donde ESTA, que no es de donde cuelga. En una instalacion con rosetas
        // es lo primero que se busca: quien va a desconectarlo necesita el sitio,
        // no el numero de puerto.
        RenglonDeFicha(etiqueta: 'Donde esta', valor: equipo.ubicacion),
        RenglonDeFicha(etiqueta: 'Nombre descubierto', valor: equipo.nombre),
        RenglonDeFicha(etiqueta: 'MAC', valor: equipo.mac),
        RenglonDeFicha(etiqueta: 'Fabricante', valor: equipo.fabricante),
        RenglonDeFicha(etiqueta: 'Subred', valor: equipo.subred),
        RenglonDeFicha(
            etiqueta: 'Como se conecta',
            valor: switch (equipo.conexion) {
              'cable' => 'Por cable',
              'wifi' => 'Por WiFi',
              _ => '',
            }),
        RenglonDeFicha(etiqueta: 'Notas', valor: equipo.notas),
        // De donde salio este renglon. Un equipo declarado no se midio, y el que
        // lo lea tiene que poder saberlo sin preguntar.
        RenglonDeFicha(
            etiqueta: 'De donde salio',
            valor: equipo.esManual
                ? 'Lo declaro una persona; ningun escaneo lo vio'
                : 'Lo encontro un barrido'),
        // Se dice como se vio: no es lo mismo "esta" que "algo contesto en esa
        // direccion", y ocultarlo seria mentir sobre la certeza.
        RenglonDeFicha(etiqueta: 'Certeza', valor: equipo.certeza),
        RenglonDeFicha(etiqueta: 'Visto por primera vez', valor: equipo.primeraVez),
        RenglonDeFicha(etiqueta: 'Visto por ultima vez', valor: equipo.ultimaVez),

        // Lo que el aparato conto DE SI MISMO. Va con la fuente de cada dato: no
        // vale lo mismo un modelo firmado en un certificado que uno sacado del
        // titulo de una pagina, y juntarlo todo en un renglon "modelo" haria
        // pasar lo segundo por lo primero.
        if (equipo.huella.isNotEmpty) ...[
          const SizedBox(height: 12),
          Text('Lo que dice de si mismo',
              style: Theme.of(contexto)
                  .textTheme
                  .titleSmall
                  ?.copyWith(fontWeight: FontWeight.w600)),
          const SizedBox(height: 4),
          for (final dato in equipo.huella)
            Padding(
              padding: const EdgeInsets.only(bottom: 2),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(
                    width: 170,
                    child: Text(dato.comoSeLlamaLaClave,
                        style: TextStyle(color: colores.outline)),
                  ),
                  Expanded(child: Text(dato.valor)),
                  const SizedBox(width: 8),
                  Text('segun ${dato.comoSeLlamaLaFuente}',
                      style: Theme.of(contexto)
                          .textTheme
                          .bodySmall
                          ?.copyWith(color: colores.outline)),
                ],
              ),
            ),
        ],
      ],
    );
  }
}

/// RenglonDeFicha es una etiqueta y su valor. Lo vacio no se dibuja: una ficha
/// llena de renglones en blanco no dice nada y esconde lo que si hay.
class RenglonDeFicha extends StatelessWidget {
  const RenglonDeFicha({super.key, required this.etiqueta, required this.valor});

  final String etiqueta;
  final String valor;

  @override
  Widget build(BuildContext contexto) {
    if (valor.isEmpty) return const SizedBox.shrink();
    return Padding(
      padding: const EdgeInsets.only(bottom: 2),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 170,
            child: Text(etiqueta,
                style: TextStyle(color: Theme.of(contexto).colorScheme.outline)),
          ),
          Expanded(child: SelectableText(valor)),
        ],
      ),
    );
  }
}

/// CredencialesDelEquipo administra como se entra a un aparato.
///
/// **La clave no se muestra sola.** Se ve el usuario y la direccion del panel
/// —que es lo que se quiere de un vistazo— y la clave hay que pedirla: el
/// servidor la manda solo entonces y deja anotado quien la pidio. Tampoco sale
/// nunca en un mapa exportado.
class CredencialesDelEquipo extends StatefulWidget {
  const CredencialesDelEquipo({
    super.key,
    required this.clave,
    required this.equipo,
    required this.alCambiar,
  });

  final String clave;
  final Equipo equipo;
  final VoidCallback alCambiar;

  @override
  State<CredencialesDelEquipo> createState() => _CredencialesDelEquipoState();
}

class _CredencialesDelEquipoState extends State<CredencialesDelEquipo> {
  /// Las claves que se pidieron, por tipo. Se olvidan al cerrar la ficha: no se
  /// quedan en memoria mas de lo necesario.
  final Map<String, String> _vistas = {};
  bool _pidiendo = false;

  Future<void> _ver(CredencialEquipo credencial) async {
    setState(() => _pidiendo = true);
    try {
      final abierta = await Api.instancia
          .verClave(widget.clave, widget.equipo.id, tipo: credencial.tipo);
      if (mounted) setState(() => _vistas[credencial.tipo] = abierta.clave);
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    } finally {
      if (mounted) setState(() => _pidiendo = false);
    }
  }

  Future<void> _editar([CredencialEquipo? credencial]) async {
    final cambio = await showDialog<bool>(
      context: context,
      builder: (_) => DialogoCredencial(
          clave: widget.clave, equipo: widget.equipo, credencial: credencial),
    );
    if (cambio == true) widget.alCambiar();
  }

  Future<void> _borrar(CredencialEquipo credencial) async {
    final quitar = await showDialog<bool>(
      context: context,
      builder: (contextoModal) => AlertDialog(
        title: const Text('Borrar la credencial'),
        content: Text('Se pierde el usuario y la clave guardados para '
            '${credencial.comoSeLlamaElTipo.toLowerCase()} de este aparato. '
            'Esto no cambia nada en el equipo: solo se olvida aqui.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(contextoModal).pop(false),
              child: const Text('Cancelar')),
          FilledButton(
              onPressed: () => Navigator.of(contextoModal).pop(true),
              child: const Text('Borrar')),
        ],
      ),
    );
    if (quitar != true) return;

    try {
      await Api.instancia.borrarCredencialDeEquipo(widget.clave, credencial.id);
      widget.alCambiar();
    } catch (problema, pila) {
      if (mounted) await mostrarProblema(context, problema, pila: pila.toString());
    }
  }

  @override
  Widget build(BuildContext contexto) {
    final colores = Theme.of(contexto).colorScheme;

    return ListView(
      padding: const EdgeInsets.symmetric(vertical: 12),
      children: [
        Text(
          'Como se entra a este aparato, guardado con el aparato para no buscarlo en '
          'un papel. La clave se guarda cifrada y no se muestra sola: hay que pedirla, '
          'y el servidor anota quien la pidio. Nunca sale en un mapa exportado.',
          style: Theme.of(contexto).textTheme.bodySmall,
        ),
        const SizedBox(height: 12),

        if (widget.equipo.credenciales.isEmpty)
          Text('Sin credenciales guardadas.',
              style: Theme.of(contexto).textTheme.bodyMedium?.copyWith(color: colores.outline)),

        for (final credencial in widget.equipo.credenciales)
          Card(
            margin: const EdgeInsets.only(bottom: 8),
            child: ListTile(
              leading: Icon(switch (credencial.tipo) {
                'ssh' => Icons.terminal,
                'consola' => Icons.cable,
                'app' => Icons.phone_android,
                _ => Icons.public,
              }),
              title: Text(credencial.comoSeLlamaElTipo),
              subtitle: SelectableText([
                if (credencial.usuario.isNotEmpty) 'usuario: ${credencial.usuario}',
                if (_vistas[credencial.tipo] != null) 'clave: ${_vistas[credencial.tipo]}',
                if (credencial.direccion.isNotEmpty) credencial.direccion,
                if (credencial.notas.isNotEmpty) credencial.notas,
              ].join('\n')),
              isThreeLine: credencial.notas.isNotEmpty,
              trailing: Wrap(
                spacing: 4,
                children: [
                  if (credencial.tieneClave && _vistas[credencial.tipo] == null)
                    TextButton.icon(
                      icon: const Icon(Icons.visibility_outlined, size: 16),
                      label: const Text('Ver clave'),
                      onPressed: _pidiendo ? null : () => _ver(credencial),
                    ),
                  if (_vistas[credencial.tipo] != null)
                    IconButton(
                      tooltip: 'Copiar la clave',
                      icon: const Icon(Icons.copy, size: 18),
                      onPressed: () async {
                        await Clipboard.setData(
                            ClipboardData(text: _vistas[credencial.tipo]!));
                        if (contexto.mounted) mensajeAviso(contexto, 'Clave copiada.');
                      },
                    ),
                  IconButton(
                    tooltip: 'Corregir',
                    icon: const Icon(Icons.edit_outlined, size: 18),
                    onPressed: () => _editar(credencial),
                  ),
                  IconButton(
                    tooltip: 'Borrar',
                    icon: const Icon(Icons.delete_outline, size: 18),
                    onPressed: () => _borrar(credencial),
                  ),
                ],
              ),
            ),
          ),

        const SizedBox(height: 8),
        Align(
          alignment: Alignment.centerLeft,
          child: FilledButton.icon(
            icon: const Icon(Icons.add),
            label: const Text('Guardar una credencial'),
            onPressed: () => _editar(),
          ),
        ),

        const SizedBox(height: 16),
        Text(
          'MiRed usa esta misma credencial para entrar al panel del aparato y leer su '
          'tabla de conectados: asi se sabe quien cuelga de el sin declararlo a mano.',
          style: Theme.of(contexto).textTheme.bodySmall?.copyWith(color: colores.outline),
        ),
      ],
    );
  }
}
