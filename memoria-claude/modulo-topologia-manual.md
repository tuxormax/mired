---
name: modulo-topologia-manual
description: El modulo 15 de MiRed: declarar a mano equipos, bocas y cables que ningun escaneo puede ver, sin que se confundan con lo medido.
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
sus bocas: el caso mas comun de todos es un **modem administrable sin SNMP hacia
la LAN** (Telmex y casi todo ISP). Por eso NO hay tabla de "equipos manuales": el
origen del dato es una columna, no un mundo paralelo.

## Clientes

Solo el programa de escritorio (no hay web — ver [modulo-programa](modulo-programa.md)).
Dos entradas a lo mismo:
1. **Ficha del equipo** (pestana Equipos): botones «Ficha» y «Bocas», y «Borrar»
   solo en lo declarado.
2. **Boton «Editar el cableado» en la pestana Puertos** → abre el mapa YA en
   modo edicion. Es la entrada principal.
3. **Modo edicion del mapa**: boton «Editar el cableado» en la barra. Toca una
   boca libre → menu; toca un equipo → sus bocas; toca un cable declarado →
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
- **Una boca lleva UN cable.** Reconectar reemplaza; guardar dos serian dos
  verdades incompatibles colgando del mismo sitio.
- **Un equipo no se conecta consigo mismo** (ni boca con boca del mismo aparato).
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
| numero de boca | validado 1-512 antes de enviar; el `+` del contador se apaga en 512 |
| tipo de boca | dropdown LAN/WAN, nunca texto libre |
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
  `manual` contra `conexiones_puerto` (bocas confirmadas) y contra `enlaces`
  (LLDP/CDP). Cuando no cuadran se muestran **las dos versiones** en un aviso rojo
  arriba del mapa; **no se pisa ninguna**. Prioridad al reconciliar:
  `snmp`/`lldp`/`cdp` > `manual` > `inferido`, pero la decision la toma quien
  mira. Ver [modulo-topologia](modulo-topologia.md).
- **Exportacion honesta.** Lo declarado va punteado en pantalla, SVG y PDF (guion
  largo en el cable, borde punteado en la caja), y el CSV lleva columna
  `origen_del_dato`. Tres trazos para tres cosas: continuo = medido, punteado
  corto = deducido, punteado largo = tecleado.

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
