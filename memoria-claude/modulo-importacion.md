---
name: modulo-importacion
description: "El modulo 20: subir una hoja de aparatos (CSV, ODS o XLSX) y meterla en la red de una vez, con vista previa"
metadata:
  node_type: memory
  type: project
---

# Importacion de hojas de aparatos (modulo 20, v1.20)

## Que hace
Sube una hoja de calculo con los aparatos de una instalacion y de cada renglon
saca **el aparato, sus puertos, el cable que lo cuelga de su switch y la
credencial de su panel**. Existe porque una instalacion documentada en una hoja
—lo normal en cualquier sitio cableado por alguien— no se captura formulario por
formulario.

**El caso que lo motivo** (2026-08-17): la hoja de una clinica con 23 rosetas,
columnas `PUERTO | NODO | UBICACION | OBSERVACIONES`, nodos llamados D01…D20 y un
renglon suelto al final cuyo puerto decia «modem 2» porque no colgaba del switch.

## Como se usa
Pantalla de la red → menu **⋮ → Importar aparatos de una hoja**. Se descarga la
plantilla, se llena y se sube. **Dos pasos y el primero no escribe nada.**

## Es una PANTALLA, no un cuadro de dialogo (Rev 41)
Y la razon es una sola: **la guia va dentro**. Quien llena la hoja la tiene
delante mientras la llena, en vez de ir a buscar un manual aparte. Cuatro
bloques, en este orden:

1. **Para que sirve** — dos frases.
2. **Como se hace** — los tres pasos numerados.
3. **El archivo** — la tarjeta con «Descargar la plantilla» y «Elegir un
   archivo», los formatos que se aceptan y, al elegirlo, su nombre con una X
   para quitarlo. Debajo caen la vista previa y el resumen.
4. **Guia para llenar la hoja** — las siete reglas, la tabla de columnas, la
   lista cerrada de QUE_ES y un ejemplo lleno.

**La guia NO esta escrita en la pantalla.** La manda el servidor en
`GET …/importacion/plantilla` (campo `guia`), sacada de la misma `Columnas` con
la que se lee el archivo y se arma el CSV descargable. Escribirla aparte serian
dos verdades —la que el servidor acepta y la que la pantalla dice que acepta— y
la segunda se descubre equivocada cuando alguien ya llenó la hoja. Las
categorias salen de `internal/catalogo`, la misma lista unica de siempre
([[ref-categorias]]).

Si la guia no carga, la pantalla **sigue sirviendo para subir el archivo**: se
queda sin la tabla y ya. Un fallo al pedir la ayuda no puede impedir el trabajo.

## La plantilla — 14 columnas
`NOMBRE` y `QUE_ES` obligatorias; el resto, opcionales.

| Columna | Que es |
|---|---|
| NOMBRE | como se llama. Unico en la red |
| QUE_ES | **lista cerrada** del catalogo. Nunca texto libre |
| PUERTOS | cuantos tiene (switches y modems) |
| CUELGA_DE | el **nombre** del padre |
| PUERTO | el puerto DE ESE padre: `7`, `LAN 7`, `WAN 1` |
| UBICACION | donde ESTA |
| IP · MAC · MODELO · NOTAS | lo demas del aparato |
| ACCESO · USUARIO · CLAVE · DIRECCION | como se entra a su panel |

**`CUELGA_DE` en cada renglon es la pieza clave.** Resuelve de una vez el switch
que no aparecia en la hoja original y el renglon raro del «modem 2»: deja de ser
un caso especial, es otro padre y ya. El switch tambien lleva su renglon, y **el
orden del archivo da igual**: se crean todos los aparatos y despues se tiran los
cables.

## Reglas que no se negocian
- **La vista previa NO escribe.** Dice renglon por renglon lo que se haria, con
  el **numero de renglon del archivo** para poder corregirlo. Importar 23
  aparatos y descubrir despues que tres estaban mal es borrarlos a mano.
- **Aplicar va en UNA transaccion.** O entra el archivo entero o no entra nada:
  media hoja capturada no se sabe por donde iba.
- **Una celda vacia no borra.** Significa «no lo se», no «borralo».
- **Categoria de lista cerrada.** Lo que no se reconoce se rechaza **con la lista
  delante**; inventar una categoria rompe el contador de la red en silencio.
- **La clave se cifra al guardar** con la caja del proyecto. El ARCHIVO la lleva
  en claro y la pantalla lo avisa: conviene borrarlo despues.
- **Recablear es lo unico que pisa un dato.** Si el puerto ya llevaba un cable a
  otro sitio manda el archivo, y se cuenta aparte en el resumen para decirlo.
- **Las columnas que MiRed no entiende se dicen**, no se callan.

## Que se traga sin protestar
- El **titulo** del sitio arriba y renglones en blanco antes de los encabezados.
- **Encabezados ajenos**: `NODO`, `OBSERVACIONES`, `CONECTADO_A`, `BOCA`, `SITIO`…
  Reconocerlos es lo que evita reescribir una hoja que lleva anos en uso.
- El **punto y coma** de un Excel en espanol y la **marca de codificacion**.
- Los **acentos** y las mayusculas: los nombres se comparan normalizados, asi que
  «Switch Site» y «switch site» son el mismo aparato.

## Donde vive
| Que | Donde |
|---|---|
| Leer CSV/ODS/XLSX y planear | `internal/importacion/` (lector.go, plan.go, plantilla.go) |
| Escribir el plan | `internal/basedatos/importacion.go` |
| Las tres rutas | `internal/api/importacion.go` |
| La pantalla | `interfaz/lib/pantallas/importar.dart` |

Los tres formatos se leen **con la biblioteca estandar de Go y nada mas**:
`encoding/csv`, `archive/zip` y `encoding/xml`. Un ODS y un XLSX son un ZIP con
XML dentro. Es la misma decision que en la interfaz, donde esos dos formatos se
**escriben** a mano (ver [[modulo-topologia]]).

Cuidado con dos detalles del formato, que ya estan resueltos y no hay que volver
a romper:
- **ODS**: `number-columns-repeated` comprime las celdas vacias seguidas.
  Ignorarlo corre todas las columnas a partir del primer hueco.
- **XLSX**: los textos van en una tabla compartida (`sharedStrings.xml`) y **las
  celdas vacias no se escriben**; hay que colocar cada una por la letra de su
  referencia (`C2`) o el hueco corre lo que viene detras.

## Rutas
- `GET  /api/redes/{clave}/importacion/plantilla` — el CSV de ejemplo **y la
  guia** (`{nombre, contenido, guia:{columnas, categorias, ejemplo}}`).
- `POST /api/redes/{clave}/importacion/vista-previa` — el plan. No escribe.
- `POST /api/redes/{clave}/importacion` — aplica. `repetidos`: `actualizar` o
  `saltar`.

El archivo viaja **entero y en base64 en los dos pasos**, en vez de guardarse a
medias entre ellos: asi el servidor no recuerda nada entre peticiones y dos
personas pueden importar a la vez sin pisarse. Tope de 1 MB por peticion.

## La columna `ubicacion` (migracion red 0019)
Salio de aqui y **es de todo el proyecto, no de este modulo**: esta en el alta a
mano, en la ficha, en las propiedades del aparato y en la hoja exportada. Es
DONDE ESTA, que no es de donde cuelga: un aparato puede colgar del puerto 7 y
estar en el consultorio 4, y quien va a desconectarlo necesita lo segundo.

Va en `equipos` y no en `equipos_snmp`: aquella la escribe el propio aparato
cuando contesta SNMP (su `sysLocation`), esta la escribe una persona, y un switch
tonto no contesta nada y tambien esta en algun sitio.

## LO SIGUIENTE: cerrar el viaje de ida y vuelta (pedido el 2026-08-17)
**Es la tarea con la que arranca la proxima sesion.** El usuario lo pidio asi:
«si se exporta el mapa de la red ya sea en archivo .zip para importar en otra
instalacion o en un excel o csv, o ods, tambien va la contrasena».

Las contrasenas ya van (Rev 50). Lo que falta es que **lo exportado se pueda
volver a importar**, y hoy no se puede porque las dos hojas no hablan el mismo
idioma:

| La hoja que se EXPORTA | La plantilla que se IMPORTA |
|---|---|
| `Aparato` | `NOMBRE` |
| `Que es` | `QUE_ES` ✔ (sinonimo) |
| `Donde esta` | `UBICACION` |
| `Cuelga de` | `CUELGA_DE` ✔ |
| `Puerto` | `PUERTO` ✔ |
| `Acceso`, `Usuario`, `Clave`, `Direccion del panel` | `ACCESO`, `USUARIO`, `CLAVE`, `DIRECCION` |
| `Ultimo barrido`, `Que tan seguro`, `Como se supo` | *(no existen: se ignoran al importar)* |

Tres formas de cerrarlo, de menos a mas trabajo:

1. **Renombrar los encabezados de la exportacion** a los de la plantilla y
   agregar los que faltan (`PUERTOS`, `MODELO`, `NOTAS`). Casi gratis: la hoja
   pasa a ser reporte **y** plantilla a la vez, y el importador ya avisa de las
   columnas que no entiende. Lo unico feo es que el reporte pierde nombres
   bonitos («Aparato» pasa a «NOMBRE»).
2. **Agregar sinonimos al lector** (`APARATO`, `DONDE_ESTA`, `DIRECCION_DEL_PANEL`)
   y dejar la exportacion como esta. Tambien casi gratis y no afea el reporte;
   a cambio, la lista de sinonimos crece y hay que acordarse de tocarla cuando
   cambie una columna.
3. **El `.zip`** que pidio: un archivo con la hoja dentro —y quiza el PNG del
   mapa— para mudar una instalacion entera. La escritura del ZIP ya existe
   (`interfaz/lib/servicios/zip.dart`) y la lectura tambien (`archive/zip` en
   `internal/importacion/lector.go`, que hoy espera un ODS o un XLSX dentro);
   habria que aceptar «un zip con una hoja suelta dentro».

Recomendado: **2 + 3**. El reporte se queda legible, el viaje de ida y vuelta
cierra, y el `.zip` es lo que de verdad pidio para mudar instalaciones.

## Otros pendientes
- No se importan **enlaces inalambricos**: el WiFi no tiene puertos y la
  plantilla va por `CUELGA_DE` + `PUERTO`. Haria falta una forma de decir «cuelga
  de esta antena por el aire». Ver [[modulo-inalambricos]].

**Ver tambien:** [[modulo-topologia-manual]], [[modulo-topologia]],
[[ref-categorias]], [[contrato-api]], [[gotchas]]
