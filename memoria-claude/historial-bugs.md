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

## 2026-08-15 — Media red salia como "Servidor web"
**Problema:** el modem, la television y el AP aparecian los tres como "Servidor
web", y el contador los sumaba en "Servidores".
**Causa:** dos cosas encadenadas. (1) La lista de fabricantes embebida tenia
**99 prefijos** y ninguno de los de esta red, asi que nadie tenia fabricante.
(2) Sin fabricante, la unica definicion que coincidia era la mas debil de todas
—"tiene el 80 abierto"—, que ademas se llamaba "Servidor web" y contaba como
servidor.
**Solucion:** el paquete instala la lista completa de la IEEE (52 977 prefijos,
con los bloques de 28 y 36 bits, y gana el mas largo) y las definiciones que
describen un SINTOMA se marcan `generico = true`: solo contestan si ninguna otra
pudo. Ademas se agrego el reconocimiento por huella (modulo 16). Ver
[[modulo-reconocimiento]].

## 2026-08-15 — La PC donde corre MiRed no aparecia en su propio inventario
**Problema:** 192.168.1.100 —el equipo que corre MiRed— era el unico aparato de
la casa que no salia en la lista.
**Causa:** nadie se manda un ARP a su propia direccion, y la respuesta al ping
propio va por dentro sin salir a la red. Las dos capas de descubrimiento lo
saltaban por construccion.
**Solucion:** `anotarEsteEquipo` en `internal/escaneo`: la sonda lee sus propias
tarjetas y se anota con metodo `propio`, que es el dato mas firme de todos (por
encima de arp).

## 2026-08-15 — El switch salia dos veces en el mapa y su uplink no ocupaba puerto
**Problema:** un switch de 5 puertos colgado del modem se dibujaba dos veces
—una como caja bajo el puerto del modem y otra como bloque suelto con sus
puertos— y seguia ofreciendo 5 puertos libres cuando uno lo ocupa el cable que
sube al modem.
**Causa:** el plano dibujaba en DOS niveles fijos, sin arbol, y la interfaz solo
sabia conectar contra un EQUIPO (`equipoDestinoId`), nunca contra un puerto del
otro extremo, aunque la base lo soportaba desde 0011.
**Solucion:** `_ArbolDeclarado` en `mapa_plano.dart` (cada aparato se dibuja una
vez, colgando de quien lo alimenta) y `DialogoElegirPuerto` al conectar contra un
aparato con puertos declarados. Migracion 0014: un puerto tampoco puede recibir
dos cables por el lado del destino.

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

## 2026-08-14 — Rev 19: el lapiz que nadie encontro y la campanita clavada

Los dos salieron de la **primera sesion de uso real del modulo 15**, en una
laptop, y ninguno lo habria cazado una prueba: los dos dejan la pantalla
funcionando y sin un solo error.

### El boton de editar el cableado no se encontraba
**Que paso:** «no me aparece el lapiz». El boton estaba donde tenia que estar y
el `.deb` lo llevaba —se comprobo desempaquetando el paquete y buscando los
textos nuevos dentro del binario de la interfaz—, pero era un **icono suelto
entre otros dos** y ademas vivia **solo en la pantalla del mapa**.

**Por que:** un icono solo se reconoce cuando ya sabes que estas buscando. Y la
mano de quien quiere tocar el cableado va a la pestana **«Puertos»**, no al mapa:
el mapa se abre para MIRAR.

**Como se corrigio:** boton **«Editar el cableado» con su nombre** en la pestana
de Puertos, que abre el mapa ya en modo edicion (`PantallaMapa.editarAlAbrir`), y
el del mapa pasa de icono a boton con texto.

### La campanita de alertas se quedaba con el numero viejo
**Que paso:** se despachaban las alertas y el contador de la pantalla de la red
no bajaba. Habia que salir al panel de inicio y volver a entrar.

**Por que:** el servidor SI actualizaba la cuenta al momento. Lo que fallaba era
la pantalla: `_recargar()` refrescaba equipos, mapa y consumo, pero **no volvia a
leer la red**, y la campanita se pinta desde el objeto `Red` que llego por
parametro al abrir la pantalla.

**Como se corrigio:** `GET /api/redes/{clave}` desde la interfaz (`Api.verRed`) y
`_releerRed()` en cada recarga. Ademas, a peticion del usuario, el cartelito
«Nueva» de cada alerta —que solo repetia lo que ya decia la negrita del titulo—
pasa a ser un boton **«Marcar leida»** que despacha esa alerta sola, con la
cuenta de las que quedan sin ver siempre a la vista.

### Lo que enseño sobre las pruebas
Al escribir la prueba de la campanita salio una **prueba que pasaba por no cargar
nada**: `expect(find.text('2'), findsNothing)` se cumple igual con la pantalla
vacia. Se cambio para que el servidor de mentira conteste un numero **distinto y
reconocible** (7) y se busque ESE.

Y cambiar de pestana en una prueba resulto necesitar **dos esperas distintas**:
la del reloj falso para la animacion y la del reloj de verdad para la respuesta.
Las dos cosas quedaron en [[gotchas]].
