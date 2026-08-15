---
name: modulo-topologia-manual
description: El modulo 15 de MiRed: declarar a mano equipos, puertos y cables que ningun escaneo puede ver, sin que se confundan con lo medido.
metadata:
  type: project
---

# Modulo 15 — Topologia manual (v1.15 Rev 20, 2026-08-14)

## Que hace

La **tercera fuente del mapa**, junto al enlace confirmado por SNMP/LLDP/CDP y al
inferido por conteo de MAC: lo que una persona **declara** porque tiene el cable
delante.

Sin esto, la red mas comun que existe —modem del ISP, switch tonto, PC, antena y
grabador de camaras— se queda sin mapa de puertos aunque su dueno sepa
perfectamente como esta cableada. Ver [red-del-usuario](red-del-usuario.md): esa
es exactamente la red donde se prueba MiRed.

**No es cosa solo de switches tontos.** Cualquier equipo puede necesitar declarar
sus puertos: el caso mas comun de todos es un **modem administrable sin SNMP hacia
la LAN** (Telmex y casi todo ISP). Por eso NO hay tabla de "equipos manuales": el
origen del dato es una columna, no un mundo paralelo.

## Clientes

Solo el programa de escritorio (no hay web — ver [modulo-programa](modulo-programa.md)).
Dos entradas a lo mismo:
1. **Ficha del equipo** (pestana Equipos): botones «Ficha» y «Puertos», y «Borrar»
   solo en lo declarado.
2. **Boton «Editar el cableado» en la pestana Puertos** → abre el mapa YA en
   modo edicion. Es la entrada principal.
3. **Modo edicion del mapa**: boton «Editar el cableado» en la barra. Toca un
   puerto libre → menu; toca un equipo → sus puertos; toca un cable declarado →
   quitarlo.

> **El boton va CON SU NOMBRE, y en la pestana de Puertos.** Empezo siendo un
> lapiz suelto entre otros dos iconos, solo en la pantalla del mapa, y la primera
> persona que uso el programa no lo encontro (Rev 19). Un icono solo se reconoce
> cuando ya sabes que estas buscando; y la mano de quien quiere tocar el cableado
> va a «Puertos», no al mapa.

## Reglas de negocio

- **Interaccion clic-clic, NO arrastre libre.** Se descarto arrastrar equipos
  sobre el lienzo: exige deteccion de colisiones en el pintor para un beneficio
  marginal sobre tocar-y-elegir.
- **El modo edicion es explicito.** Sin el, un clic de navegacion reescribiria la
  topologia por accidente, y el mapa es justo lo que se consulta cuando algo no
  funciona: el peor momento para cambiarlo sin querer.
- **Un puerto lleva UN cable.** Reconectar reemplaza; guardar dos serian dos
  verdades incompatibles colgando del mismo sitio.
- **Un equipo no se conecta consigo mismo** (ni puerto con puerto del mismo aparato).
- **Solo se borra lo declarado.** Lo descubierto se marca ausente y conserva su
  historia, que es de donde salen las alertas.
- **Un enlace declarado NO dispara la alerta de cambio de topologia**: lo movio el
  usuario a proposito, y avisarle de su propio cambio entrena a ignorar avisos.
  Decidido el 2026-08-14. Ver [modulo-alertas](modulo-alertas.md).
- **Al elegir «conectar uno existente» solo se ofrecen los equipos SIN ubicar.**
  Ofrecer todos dejaria mover por accidente uno que un switch ya reporto, y eso
  no se corrige tecleando: se corrige moviendo el cable.

## Especificaciones

### Migracion `red/0011_topologia_manual.sql`

| Donde | Que |
|---|---|
| `equipos` | + `modelo TEXT`, `origen TEXT NOT NULL DEFAULT 'descubierto'` (`descubierto`\|`manual`), `conexion TEXT` (`cable`\|`wifi`\|NULL). `notas` ya existia desde 0002. |
| `puertos_fisicos` | `equipo_id`, `numero` (1-512), `tipo` (`lan`\|`wan`), `velocidad_mbps` NULL, `notas`, `creado_en`. UNIQUE (equipo_id, numero, tipo). |
| `enlaces_fisicos` | `puerto_origen_id`, y **o** `puerto_destino_id` **o** `equipo_destino_id` (CHECK: una y solo una), `origen_dato` (`manual`\|`snmp`\|`lldp`\|`cdp`\|`inferido`), `notas`, `creado_en`. UNIQUE en `puerto_origen_id`. |

### Restricciones que el formulario impone igual que la columna

| Campo | UI |
|---|---|
| nombre | `maxLength: 120` |
| modelo | `maxLength: 120` |
| notas | `maxLength: 2000` |
| numero de puerto | validado 1-512 antes de enviar; el `+` del contador se apaga en 512 |
| tipo de puerto | dropdown LAN/WAN, nunca texto libre |
| que es el aparato | dropdown de la **lista unica de categorias**, nunca texto libre. Ver [ref-categorias](ref-categorias.md) |
| velocidad | dropdown 10/100/1000/2500/10000 + «No se sabe» (guarda NULL) |
| conexion | dropdown cable/wifi/vacio |

El servidor valida **lo mismo** y contesta con mensaje de negocio, no con un 500.

## Interconexion

- **El escaneo salta lo declarado.** `marcarAusentes` filtra `origen <> 'manual'`.
  Sin esa linea, el switch tonto se marcaria ausente en cada barrido, generaria un
  evento de presencia cada vez y la alerta de «lleva dias sin aparecer» avisaria
  de un aparato atornillado a la pared. Ver [modulo-escaneo](modulo-escaneo.md).
- **Reconciliacion con SNMP.** `Base.Contradicciones()` compara cada cable
  `manual` contra `conexiones_puerto` (puertos confirmados) y contra `enlaces`
  (LLDP/CDP). Cuando no cuadran se muestran **las dos versiones** en un aviso rojo
  arriba del mapa; **no se pisa ninguna**. Prioridad al reconciliar:
  `snmp`/`lldp`/`cdp` > `manual` > `inferido`, pero la decision la toma quien
  mira. Ver [modulo-topologia](modulo-topologia.md).
- **Exportacion honesta.** Lo declarado va punteado en pantalla, SVG y PDF (guion
  largo en el cable, borde punteado en la caja), y el CSV lleva columna
  `origen_del_dato`. Tres trazos para tres cosas: continuo = medido, punteado
  corto = deducido, punteado largo = tecleado.

### Como se llama un puerto (Rev 36)

**`LAN 3`, `WAN 1`, `DMZ 1`** — el tipo y el numero, como esta rotulado en el
aparato. Nunca «puerto 3» a secas: en un modem con LAN, WAN y DMZ eso no dice
por donde sale el cable.

- La lista de tipos vive **tres veces**: el CHECK de la tabla (red 0018), la
  constante `TiposDePuerto` de Go y `interfaz/lib/modelos/tipos_de_puerto.dart`.
  Dos pruebas en Go las vigilan: una compara Go contra Dart, la otra mete un
  puerto de cada tipo en la tabla de verdad. Es la misma regla de
  [ref-categorias](ref-categorias.md).
- **El nombre lo arma una sola funcion** (`nombreDePuerto`) y la usan el mapa, la
  ficha y las listas: si cada pantalla lo armara, el mismo puerto se llamaria de
  dos maneras.
- La etiqueta del cable en el mapa lleva **las dos puntas**: `LAN 4 → LAN 5`.

### La toma unica de un aparato de punta (Rev 37)

Un aparato que **no declaro ningun puerto** es de punta —PC, TV, grabador,
impresora— y se conecta por UN cable: esa toma se llama **LAN 1** en todas las
pantallas (`puertoUnicoDeUnEquipoFinal`). No es suponer: es como se conecta un
aparato asi. En cuanto alguien le declara puertos, se deja de suponer.

Dos limites, los dos con prueba:

- **A un aparato que va por el aire NO se le supone toma.** Muchas laptops y
  tabletas ya no traen conector de red, y dibujarles un LAN 1 seria inventarles
  un agujero que no tienen. Se mira si cuelga de una antena o si su ficha dice
  `conexion = wifi`. **Un cable declarado manda sobre eso**: si alguien dijo que
  hay cable, hay donde enchufarlo.
- **A un aparato CON puertos declarados no se le senala ninguno al azar.** Ahi
  hay datos contados mirando el aparato y esos mandan: el mapa dice solo la punta
  que sabe.

En la ficha, la pestana **Conexiones** de un equipo de punta muestra ese LAN 1
con de que cuelga. Antes decia «no tiene puertos declarados» y nada mas: el dato
existia —el cable lo declaro el switch— pero solo se veia desde el otro lado.
- **Rehacer `puertos_fisicos` es peligroso**: `enlaces_fisicos` cuelga de ella
  con `ON DELETE CASCADE`, y soltar la tabla vieja con el hijo colgando borra
  todos los cables sin avisar. Orden obligado: copiar las dos nuevas → soltar el
  hijo → soltar el padre → renombrar. La prueba
  `TestLaMigracionDeTiposDePuertoNoSeLlevaLosCables` lo cuida.

### Como se dibuja un cable (Rev 35-36)

El plano crece hacia la **derecha**: una columna por nivel, los hermanos en lista
hacia abajo. **Cada aparato va a la misma altura que su PRIMER hijo** —no
centrado en su franja, que lo dejaba flotando en medio de un hueco— y cada rama
reserva su franja entera, con todo lo que le cuelga: por eso dos ramas no se
enciman nunca y no hace falta correr nada a un lado. Hay una prueba que compara
todas las cajas contra todas.

Y el cable **va en codo, nunca en diagonal**:

1. Sale horizontal del aparato.
2. Baja por **su propio canal** (`canalDeEnlace`), separado del de sus hermanos:
   con todos por la misma vertical, diez enlaces de diez colores se ven como una
   sola linea gorda.
3. Dobla con la esquina redondeada (`radioDeCodo`) y entra horizontal al de la
   derecha.

La **etiqueta del puerto** va pegada a la caja a la que ENTRA el cable, alineada
por la derecha y a su altura (`finDeEtiqueta`). En el centro del enlace —donde
estaba— caian todas encima unas de otras y de las cajas, porque el centro es
justo donde se juntan las bajadas.

**El recorrido lo calcula una sola funcion, `recorridoDeEnlace`**, y la usan los
tres que dibujan: pantalla, SVG y PDF. Si cada uno lo calculara por su cuenta, el
archivo exportado dejaria de ser el mapa que se vio. El PDF no tiene curva
cuadratica: convierte cada esquina a cubica con la regla de dos tercios.

→ [contrato-api](contrato-api.md) · → [gotchas](gotchas.md)

## Donde vive el codigo

| Capa | Archivo |
|---|---|
| Esquema | `internal/basedatos/esquema/red/0011_topologia_manual.sql` |
| Datos | `internal/basedatos/topologiamanual.go` (+ `topologiamanual_test.go`) |
| API | `internal/api/topologiamanual.go`, rutas en `internal/api/api.go` |
| Modelos | `interfaz/lib/modelos/modelos.dart` (`PuertoFisico`, `EnlaceFisico`, `Contradiccion`, `TopologiaManual`) |
| Formularios | `interfaz/lib/pantallas/topologia_manual.dart` (los usan las DOS pantallas) |
| Mapa | `interfaz/lib/pantallas/mapa.dart` (modo edicion) y `mapa_plano.dart` (plano) |

**Ver tambien:** [ref-categorias](ref-categorias.md) ·
[modulo-topologia](modulo-topologia.md) ·
[modulo-escaneo](modulo-escaneo.md) · [modulo-alertas](modulo-alertas.md) ·
[contrato-api](contrato-api.md) · [red-del-usuario](red-del-usuario.md) ·
[gotchas](gotchas.md) · [mired-arquitectura](mired-arquitectura.md)
