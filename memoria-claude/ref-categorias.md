---
name: ref-categorias
description: La lista unica de categorias de equipo de MiRed, quien la usa y por que vive dos veces.
metadata:
  type: reference
---

# La lista unica de categorias (v1.15 Rev 20, 2026-08-14)

## Que es

**El vocabulario cerrado de lo que puede componer una red.** Una sola lista, que
usan tres sitios a la vez:

1. **El catalogo `.toml`** — el campo `categoria` de cada definicion tiene que
   ser una clave de aqui.
2. **El formulario de «agregar un aparato a mano»** — su desplegable sale de
   esta lista, **nunca texto libre**.
3. **El contador de la pantalla de equipos** — agrupa por esto.

## Por que existe

Antes `equipos.tipo` guardaba el **nombre** que le ponia el catalogo («Impresora
HP»), que sirve para leerlo y **no para contar**: agrupando por ahi salen cubos
separados para «Impresora HP» y «Impresora de red».

Y habia dos vocabularios en paralelo sin que nadie lo hubiera decidido: el alta
manual decia `ap`, `pc`, `nvr`; el catalogo decia `punto_de_acceso`,
`computadora`, `camara`. El contador habria mostrado «AP: 1» y «Punto de acceso:
2» como si fueran cosas distintas — exactamente la divergencia silenciosa que la
regla de concordancia del usuario existe para evitar.

## Donde vive

| Lado | Archivo |
|---|---|
| Go | `internal/catalogo/categorias.go` |
| Dart | `interfaz/lib/modelos/categorias.dart` |

**Vive DOS veces y no hay forma de que viva una**: son dos lenguajes y dos
binarios. Lo unico que queda es vigilar que no se separen, y de eso se encarga
`TestLasDosListasDeCategoriasDicenLoMismo`, que **lee el archivo Dart desde Go**
y compara las claves. Si alguien agrega una categoria en un lado y no en el otro,
la suite falla.

Otra prueba, `TestElCatalogoQueSeEntregaUsaCategoriasDeLaLista`, comprueba que
las 15 definiciones semilla usen claves validas. **Ya cazo una** al escribirla:
`switch-administrable.toml` decia `categoria = "switch"`.

## Las categorias

| Clave | Se lee | La detecta un escaneo |
|---|---|---|
| `gateway` | Modem o gateway | si |
| `router` | Router | si |
| `switch_administrable` | Switch administrable | **si, y lo dice el propio aparato por SNMP** |
| `switch_simple` | Switch no administrable | **NO — solo a mano** |
| `punto_de_acceso` | Punto de acceso WiFi | si |
| `computadora` | PC o laptop | si |
| `telefono` | Telefono o celular | si |
| `impresora` | Impresora | si |
| `camara` | Camara o grabador | si |
| `servidor` | Servidor | si |
| `almacenamiento` | Almacenamiento en red | si |
| `multimedia` | TV o multimedia | si |
| `otro` | Otro aparato | si |
| `sin_reconocer` | Sin reconocer | calculada, **no se puede elegir** |

## Reglas que no se negocian

- **`switch_simple` es la razon de ser del modulo 15.** No tiene direccion, no
  contesta a nada y ningun barrido lo va a ver jamas. Es la unica categoria con
  `Automatica = false`, y esa marca se muestra en el desplegable.
- **`sin_reconocer` no se esconde ni se reparte.** Un inventario que calla lo que
  no supo clasificar se lee como si estuviera completo, y ahi es justo donde hay
  que mirar. Tampoco se puede elegir: es un resultado, no una opcion.
- **El catalogo NUNCA pisa lo declarado a mano** (`PonerTipos` filtra
  `origen <> 'manual'`). El catalogo deduce por puertos abiertos; quien lo
  declaro lo tenia delante.
- **SNMP pisa al catalogo** para `switch_administrable`: un dato que da el propio
  aparato manda sobre uno deducido. Se aplica despues, en
  `MarcarSwitchesAdministrables`.
- **El plural no se calcula pegando una «s»**: «switch» da «switches» y «punto de
  acceso» da «puntos de acceso». Por eso cada categoria lleva su plural escrito.

## El contador

`GET /api/redes/{clave}/composicion` devuelve `{total, presentes, declarados,
categorias[]}`.

- **Lo cuenta el servidor sobre toda la red**, no la pantalla sobre lo que tenga
  a la vista: con un filtro escrito, contar en la interfaz diria «3 equipos» en
  una red de treinta.
- **Cuenta toda la red y dice aparte cuantos estan prendidos.** Un equipo apagado
  sigue siendo parte de la red; contar solo los prendidos haria bailar el numero
  con cada barrido.
- **Cuadra con el mapa por construccion, no por sincronizarse**: los dos leen
  `equipos`. Un switch declarado a mano cuenta desde el momento en que se
  declara. Lo fija `TestElContadorCuentaTambienLoDeclaradoAMano`.
- `declarados` se dice aparte: la cuenta es igual de real, pero **no viene de una
  medicion**.

## Al agregar una categoria nueva

1. Agregarla en **los dos** archivos, con la misma clave.
2. Correr `go test ./internal/catalogo/` — la prueba de sincronia falla si falta
   en uno.
3. Si algun `.toml` del catalogo va a usarla, comprobar que la escribe igual.

## La otra lista espejo: los tipos de puerto

Misma regla, distinto sitio y **tres** copias en vez de dos: el `CHECK` de
`puertos_fisicos` (red 0018), `TiposDePuerto` en
`internal/basedatos/topologiamanual.go` y
`interfaz/lib/modelos/tipos_de_puerto.dart`. Las vigila
`go test ./internal/basedatos/ -run TiposDePuerto`, que ademas mete un puerto de
cada tipo en la tabla de verdad: una lista que el servidor da por buena y la
tabla rechaza seria un error 500 en vez de un mensaje de negocio.

**Ver tambien:** [modulo-topologia-manual](modulo-topologia-manual.md) ·
[modulo-escaneo](modulo-escaneo.md) · [contrato-api](contrato-api.md) ·
[gotchas](gotchas.md)
