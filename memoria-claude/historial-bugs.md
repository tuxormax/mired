---
name: historial-bugs
description: "Bitacora cronologica de problemas de MiRed: que paso, por que y como se corrigio"
metadata: 
  node_type: memory
  type: project
  originSessionId: 20376d18-adf7-4315-bb9c-98a3aa84ec95
  modified: 2026-08-12T22:35:06.517Z
---

# Historial de bugs de MiRed

Bitacora cronologica. El **tripwire** de cada uno vive en [[gotchas]] o en el
`bug-*.md` que corresponda; aqui queda la cronica.

## 2026-08-12 — `dpkg-deb` rechazaba el paquete por permisos 2775
**Problema:** al armar el `.deb`, `dpkg-deb` abortaba con "el directorio de
control tiene permisos erroneos".
**Causa:** la carpeta del proyecto es setgid `www-data` y el bit se hereda a todo
lo que se crea dentro. `chmod 0755` **no limpia el setgid de una carpeta** (lo
bajo de 2775 a 2755 y ahi se quedo).
**Solucion:** `find -type d -exec chmod g-s` explicito antes del `chmod 0755`, en
`herramientas/construir.sh`.

## 2026-08-12 — Un puerto cerrado seguia marcandose como abierto
**Problema:** una prueba con dos escaneos seguidos mostraba los dos puertos
abiertos cuando el segundo escaneo solo vio uno.
**Causa:** el cierre de puertos comparaba `ultima_vez < momento`, y `Ahora()`
tiene resolucion de **segundos**: dos escaneos dentro del mismo segundo llevan la
misma marca y la comparacion no cierra nada. En produccion pasaria en cualquier
red chica, donde un barrido tarda menos de un segundo.
**Solucion:** cerrar por **lista de lo que se vio**
(`NOT IN (VALUES (numero, protocolo), ...)`) en vez de por marca de tiempo.
**Aprendizaje general:** no usar comparaciones de tiempo para decidir "esto ya no
esta"; usar la lista de lo observado, que siempre dice la verdad.

## 2026-08-12 — Una red sin switches se quedaba en "desconocida" para siempre
**Problema:** tras un escaneo completo en una red sin equipos SNMP, la pantalla
de puertos seguia diciendo "todavia no se ha consultado", que suena a que falta
correr algo.
**Causa:** el perfil de capacidades solo se recalculaba cuando **algun** equipo
contestaba SNMP.
**Solucion:** recalcularlo tambien cuando no contesta nadie, para que quede
`no_disponible` y la interfaz explique que se pierde y que haria falta.
**Aprendizaje general:** "no encontre nada" **es** un resultado y hay que
guardarlo; si no, la interfaz no puede distinguir entre "no se ha buscado" y "se
busco y no hay".

## 2026-08-12 — Todos los puertos parecian nuevos y disparaban alertas
**Problema:** al probar el motor de alertas, un equipo conocido genero un aviso
de "puerto nuevo" por CADA puerto que tenia, incluidos los de siempre.
**Causa:** la MISMA de dos bugs atras. "¿Es nuevo este puerto?" se respondia con
`primera_vez = ultima_vez`, y como las dos marcas se escriben en el mismo
segundo, todos los puertos parecian recien aparecidos.
**Solucion:** columna `puertos.escaneo_creado` (migracion 0006 de red): guarda en
que escaneo se vio por primera vez. Un puerto es nuevo si `escaneo_creado` es el
escaneo actual. El numero de escaneo no depende del reloj.
**Aprendizaje general:** **tercera vez que el reloj enganya.** Cualquier pregunta
del tipo "¿esto cambio?" se responde con identificadores de corrida o con listas
de lo observado, nunca con marcas de tiempo. Anotado como regla dura en
[[gotchas]].

## 2026-08-12 — Cuatro pantallas reventaban en compilacion de depuracion
**Problema:** al escribir las primeras pruebas de pantalla, cuatro fallaron con
*"setState() callback argument returned a Future"*. En una compilacion de
**depuracion** eso deja la pantalla en rojo.
**Causa:** `setState(() => _redes = Api.instancia.listarRedes())`. Con cuerpo de
flecha, la expresion **devuelve** el Future de la asignacion, y `setState` tiene
una asercion contra eso. En compilacion de **release** la asercion no corre, asi
que la pantalla se veia bien: por eso no se habia notado probandola en el
navegador.
**Solucion:** cuerpo entre llaves en `redes.dart`, `usuarios.dart`,
`credenciales.dart` y `mapa.dart`.
**Aprendizaje general:** **probar en release no basta.** Las aserciones de Flutter
solo corren en depuracion y son justo las que atrapan este tipo de error. Por eso
`interfaz/test/pantallas_test.dart` dibuja todas las pantallas contra un servidor
de mentira: es la unica prueba que las habria encontrado.

**Ver tambien:** [[gotchas]], [[modulo-escaneo]], [[modulo-topologia]],
[[modulo-alertas]]

## 2026-08-13 — La suite de pruebas fallaba sola cuando el equipo estaba ocupado

**Que pasaba.** `go test ./...` fallaba de vez en cuando en el paquete
`basedatos`, siempre igual: *"no se pudo conectar a .../mired.db: context
deadline exceeded"*, en pruebas distintas cada vez. Con `-p 1` no fallaba nunca,
y corriendo el paquete solo tampoco. Parecia cosa de la maquina.

**Por que.** Cada prueba crea dos bases —catalogo y red— y les corre dos docenas
de migraciones, sobre `t.TempDir()`, o sea sobre disco, con WAL y
`synchronous=NORMAL`. Son cientos de sincronizaciones que **no prueban nada**:
esas pruebas comprueban SQL, no que el disco aguante un corte de corriente. Con
el equipo ocupado —bastaba una compilacion de Flutter en paralelo— abrir una base
pasaba de 12 ms a mas de 10 s, y ahi reventaba el plazo que `Abrir` se ponia.

**Como se reprodujo.** Lanzando `flutter build web` y `go test ./...` a la vez:
5 fallos y 118 s en el paquete. Sin la compilacion al lado, ni uno.

**Que se corrigio.** Dos cosas, en dos sitios distintos a proposito:

1. **En las pruebas** (`carpetaDePrueba`): las bases se crean en `/dev/shm`, que
   es memoria. Con caida a `t.TempDir()` si no hay. **De 118 s con 5 fallos a
   0.6 s sin ninguno, bajo la misma carga.**
2. **En produccion** (`Abrir`): ya no se inventa su propio plazo — recibe
   contexto y lo respeta, con `EsperaAlAbrir` de 30 s cuando el que llama no trae
   uno. Y el error separa "tardo demasiado, el equipo puede estar saturado" de
   "no se pudo conectar". No era solo cosa de las pruebas: **una Raspberry con
   tarjeta SD escribiendo un escaneo es el caso normal**, y con el plazo viejo el
   sintoma habria sido una red desapareciendo de la interfaz sin explicacion.

**Tripwire.** Si una prueba de base de datos falla con *deadline exceeded*, lo
primero que hay que mirar es que mas estaba corriendo en el equipo, no el SQL.

## 2026-08-13 — La pantalla del primer acceso se ponia gris al teclear el usuario

**Que pasaba.** Al instalar el programa y escribir el usuario en la pantalla de
primer acceso, la ventana se quedaba **en gris**. Sin mensaje, sin modal, sin
nada que copiar. El usuario no llego ni a escribir la clave.

**Por que.** La comprobacion de la regla de TUXOR —que el usuario o la clave
lleven uno de los signos `+ - * % ^ & | < > #`— armaba una expresion regular
metiendo esa lista en una clase de caracteres. **`RegExp.escape` de Dart NO
escapa el guion**, asi que quedaba `[\+-\*%...]` y el `-` se leia como un RANGO
de `+` (0x2B) a `*` (0x2A): al reves, invalido, y la expresion lanza
`FormatException`.

Dos cosas lo hacian traicionero:

1. **Solo reventaba al teclear uno de esos signos**, que es exactamente lo que la
   pantalla pide hacer. Con cualquier otra letra la funcion salia antes de llegar
   a la expresion.
2. **Nadie lo vio venir porque el error ocurria al DIBUJAR.** Los tres
   capturadores globales de la casa no alcanzan ahi: para mostrar un modal hace
   falta una pantalla, y la pantalla era justo lo que fallaba. Flutter, en
   compilacion de entrega, pinta un hueco gris.

**Que se corrigio.** Dos cosas, y la segunda importa mas que la primera:

1. `_tieneOperador` ya **no usa ninguna expresion regular**: recorta con
   operaciones de texto, que no pueden fallar. Con pruebas que teclean **los diez
   signos uno por uno**, que es lo que faltaba.
2. **Se agrego el CUARTO candado**: `ErrorWidget.builder = pantallaRota`
   (`interfaz/lib/widgets/pantalla_rota.dart`). Un error al dibujar ya no deja
   gris: muestra que paso, donde, el stack y un boton de copiar. No se apoya en
   el tema ni en `Material`, porque es lo ultimo que queda en pie cuando ya fallo
   algo.

**Tripwires.**
- Si tocas una expresion regular con una clase de caracteres → **el guion va al
  final o escapado a mano**; `RegExp.escape` no lo hace por ti.
- Si armas una expresion regular en un `build()` o en algo que corre por tecla →
  no lo hagas. Reventar ahi deja la pantalla en gris.
- Si alguien quita `ErrorWidget.builder` del arranque, vuelven los grises mudos.

## 2026-08-13 — Interfaz nueva hablando con un motor viejo

**Que pasaba.** Tras instalar la Rev 15, el pie del programa seguia mostrando
**Rev 8** y el flujo nuevo de crear redes no aparecia: seguia pidiendo la subred
que ya se habia quitado.

**Por que.** El `mired-servidor` que estaba corriendo llevaba vivo desde hacia
hora y media —su binario aparecia ya como `(deleted)`, porque dpkg lo habia
reemplazado— y **seguia sirviendo el codigo viejo desde memoria**. El programa,
al abrirse, vio que algo contestaba en el puerto 60072 y se colgo de el, que es
justo lo que le dice su regla de "solo mata lo que el arranco".

A esa regla le faltaba la otra mitad: **nunca comprobaba que el servidor al que
se colgaba fuera de su misma version**. Y el paquete, al actualizarse, tampoco
paraba lo que estaba corriendo.

**Que se corrigio.** Los dos lados, porque ninguno basta solo:

1. **`prerm` para los procesos al actualizar.** No pasan por systemd —son hijos
   del programa— asi que `systemctl stop` no los ve: hay que pararlos por su ruta.
2. **El programa compara versiones.** Le pregunta al binario instalado
   (`--version`) y al servidor vivo (`/api/estado`); si no coinciden y el
   servidor es de este equipo, lo detiene y levanta el suyo, avisando de lo que
   hizo. Si el servidor es de otro equipo **no lo toca**: podria estar vigilando
   una red que no es la nuestra.

El hash del build se descarta al comparar: dos compilaciones de la misma entrega
son la misma version, y contarlo reiniciaria el servidor en cada arranque.

**Tripwire.** Si el pie muestra una version que no coincide con la instalada,
mira `readlink /proc/<pid>/exe` del servidor: si dice `(deleted)`, esta corriendo
un binario que ya no existe.
