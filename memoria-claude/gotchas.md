---
name: gotchas
description: Reglas transversales de MiRed que cruzan modulos y evitan errores tontos
metadata: 
  node_type: memory
  type: project
  originSessionId: 20376d18-adf7-4315-bb9c-98a3aa84ec95
  modified: 2026-08-13T16:00:00.000Z
---

# Gotchas de MiRed

Reglas que valen en todo el proyecto. Una sola lectura evita la mayoria de los
errores tontos.

## Base de datos
- **`chmod` numerico NO limpia el bit setgid de una carpeta.** Hay que
  `chmod g-s` explicitamente. Mordio al armar el `.deb`: la carpeta del proyecto
  es setgid `www-data` y `dpkg-deb` rechaza el directorio de control por eso.
- **Las fechas se guardan como TEXT en ISO 8601 con zona horaria.** SQLite no
  tiene tipo fecha; el formato **es** el contrato. Usar `basedatos.Ahora()`.
- **Borrado suave con `estatus` -1/0/1** y los indices unicos **parciales**
  (`WHERE estatus >= 0`) para poder reutilizar el nombre de algo borrado.
- **`BuscarRed` no encuentra redes borradas** (`estatus >= 0` excluye -1). Para
  reactivar hay que buscar por nombre con `estatus = -1`, que es lo que hace
  `CrearRed`.
- **Migrar es idempotente y se llama al ABRIR**, no al instalar: una base
  restaurada de un respaldo viejo llega con esquema atrasado.
- **`Abrir` recibe contexto y lo respeta.** No se inventa su propio plazo: si el
  que llama trae uno, manda el suyo; si no, se le pone `EsperaAlAbrir` (30 s).
  El plazo es generoso a proposito —una Raspberry con tarjeta SD escribiendo un
  escaneo tarda— y el error distingue "tardo demasiado" de "no se pudo": son
  problemas distintos y se arreglan en sitios distintos.
- **`ON CONFLICT` contra un indice parcial exige repetir su condicion**:
  `ON CONFLICT (cidr) WHERE estatus >= 0 DO UPDATE …`.
- **Si tocas un `CHECK` → hay que REHACER la tabla → cuidado con lo que cuelga de
  ella.** SQLite no sabe cambiar un `CHECK`, y `DROP TABLE` con las llaves
  foraneas encendidas **dispara los `ON DELETE CASCADE` de los hijos**: soltar la
  tabla padre con el hijo colgando borra los datos del hijo sin decir nada. Orden
  obligado: crear las dos nuevas y copiar → soltar el HIJO → soltar el padre, que
  ya no tiene a quien arrastrar → renombrar (al renombrar el padre, SQLite
  reescribe sola la referencia del hijo). Y `PRAGMA defer_foreign_keys = ON` al
  principio, porque `foreign_keys` **no se puede apagar dentro de una
  transaccion** y las migraciones corren en una. Ejemplo hecho: red 0018, con una
  prueba que corre la migracion sobre las tablas de antes, con datos dentro, y
  comprueba que no se perdio ni un cable.
- **NUNCA decidir "esto es nuevo" o "esto ya no esta" comparando marcas de
  tiempo.** `Ahora()` tiene resolucion de SEGUNDOS y dos escaneos seguidos en una
  red chica caen en el mismo segundo. Ya mordio DOS veces (puertos que se cierran,
  puertos nuevos). Se compara contra **la lista de lo observado** o contra el
  **numero de escaneo**, que no mienten nunca. Ver [[historial-bugs]].

## Una base por red
- **El enrutado vive solo en `internal/basedatos`.** Ningun otro paquete abre un
  archivo `.db` por su cuenta.
- **`Red()` presta y hay que devolver**: `defer devolver()`. Mejor todavia,
  `ConRed()`, que no se puede olvidar. Sin devolver, el archivo queda abierto
  para siempre.
- **Las consultas que cruzan redes no son un `SELECT`.** El panel de inicio se
  arma con el resumen guardado en el catalogo, no abriendo treinta archivos.
- **La clave de la red no cambia nunca; el nombre si.** El archivo se llama por
  la clave (`matriz-a1b2.db`). Renombrar NO mueve el archivo.

## Todo dato dice de cuando es
Regla del proyecto (2026-08-13, la pidio el usuario):

- **Todo reporte que salga de MiRed dice de que momento son los datos.** No de
  cuando se exporto: **de cuando se midieron**. Un mapa exportado hoy con datos
  de hace tres semanas llevaria la fecha de hoy y pareceria al dia. Los cuatro
  formatos lo llevan, CSV incluido.
- **En pantalla igual**: la tarjeta de cada red, la cabecera de la red y la
  leyenda del mapa dicen "hace 5 minutos" o "hace 3 dias", no una fecha ISO que
  obligue a hacer la cuenta. Y en rojo cuando lleva mas de un dia.
- Vive en `interfaz/lib/servicios/frescura.dart`, en un solo sitio. Si se agrega
  otro reporte, **usa eso**; no se vuelve a escribir el calculo.
- El dato de "de cuando es el mapa" NO es la fecha del ultimo escaneo: es
  `MAX(ultima_vez)` de `conexiones_puerto`, que es cuando se le pregunto a los
  switches. Son cosas distintas y se sienten distintas al mirarlas.

## Categorias de equipo
- **`tipo` es para LEER, `categoria` es para CONTAR.** `tipo` guarda el nombre
  que puso el catalogo ("Impresora HP"); `categoria` es la clave del vocabulario
  cerrado. Agrupar por `tipo` saca cubos separados para "Impresora HP" e
  "Impresora de red". Ver [[ref-categorias]].
- **La lista de categorias vive en DOS archivos** (`internal/catalogo/
  categorias.go` y `interfaz/lib/modelos/categorias.dart`) y hay una prueba que
  los compara. Agregar una en un lado y no en el otro rompe el contador **sin dar
  error**: salen dos cubos para la misma cosa.
- **Nada de texto libre en la categoria.** Ni en el catalogo, ni en el
  formulario, ni en la API: el servidor la valida contra la lista aunque el
  desplegable ya la limite, porque un cliente viejo se salta la interfaz.

## Interfaz: no pedir lo que el programa ya sabe
- **SNMP se prueba SIEMPRE, aunque nadie haya cargado credenciales.** Se agrega
  la comunidad de fabrica `public` (v2c, solo lectura) a la lista en cada escaneo
  profundo. Antes, sin credenciales, la consulta se saltaba entera: MiRed nunca
  podia saber si habia switches administrables y le pedia al usuario que cargara
  una credencial para unos switches que en una red de casa **no existen**.
  Si nadie contesta, la capacidad queda en `no_disponible` — que es una
  RESPUESTA ("son switches simples"), no un "todavia no se sabe".
- **El formulario de nueva red NO pide una subred en notacion con barra.** Pedia
  `192.168.1.0/24` y el usuario —que es el dueño del proyecto— no supo que poner:
  es una cuenta que el programa puede hacer solo, porque la sonda ya le reporta
  las tarjetas del equipo. Ahora propone la red detectada, ya marcada.
- **Toda la traduccion de lo que se escribe vive en `servicios/subredes.dart`**,
  en un solo sitio. Acepta rango (`a - b`), direccion suelta y notacion con
  barra, y **siempre dice lo que entendio antes de crear nada**: aceptar varias
  formas sin enseñar el resultado seria magia.
- **Crear una red pide SOLO nombre y descripcion.** Nada de rangos: al pulsar
  "Escanear toda la red", MiRed adopta la red de este equipo, la guarda y lo
  dice. Antes se podia crear una red sin nada que escanear y el escaneo no
  encontraba nada, sin explicar por que.
- **"Este equipo no esta conectado a ninguna red" es un error propio**
  (`ErrEquipoSinRed`), distinto de "no hay nada configurado": uno se arregla en
  MiRed y el otro enchufando un cable. Darles el mismo mensaje manda a buscar
  donde no hay nada.
- Regla general que salio de ahi: **si el programa puede averiguar un dato, no se
  lo pregunta al usuario.**

## Interfaz: errores que no se ven
- **Los tres capturadores globales NO alcanzan a los errores de dibujo.** Para
  mostrar un modal hace falta una pantalla, y la pantalla es lo que fallo. Por
  eso hay un CUARTO candado, `ErrorWidget.builder = pantallaRota` en el arranque:
  sin el, un error al dibujar deja la ventana **en gris**, muda. Ya mordio.
  Ver [[historial-bugs]].
- **`RegExp.escape` de Dart no escapa el guion.** Meter una lista de signos en
  una clase de caracteres `[...]` puede convertir el `-` en un rango invalido y
  lanzar `FormatException`. En una clase de caracteres, el guion va **al final**
  o escapado a mano.
- **Nada de expresiones regulares en un `build()`** ni en algo que corra por cada
  tecla: si revientan ahi, se llevan la pantalla entera.

## El programa de escritorio
- **Un servidor viejo vivo se lleva el puerto y el programa se cuelga de el.**
  Al actualizar el paquete sin cerrar el programa, el `mired-servidor` anterior
  sigue con su codigo en memoria (su binario aparece como `(deleted)`), y el
  programa nuevo lo adopta: interfaz nueva contra motor viejo, con el pie
  mostrando una version que ya no esta instalada. Mordio el 2026-08-13. Se cierra
  por los dos lados y hacen falta LOS DOS:
    1. El `prerm` para los procesos al **actualizar** (no pasan por systemd: son
       hijos del programa, `systemctl stop` no los ve).
    2. El programa **compara su version con la del servidor vivo** y, si no
       coinciden y es local, lo detiene y levanta el suyo.
- **Al comparar versiones se descarta el hash del build**: dos compilaciones de
  la misma entrega son la misma version, y compararlo reiniciaria el servidor
  cada vez que alguien recompila sin cambiar nada.
- **Solo se toca lo de este equipo.** Si el programa apunta al MiRed de otro
  sitio, no se arranca ni se mata nada alli. Y una direccion mal escrita NO
  cuenta como local: `Uri.tryParse` acepta casi cualquier texto.
- **MiRed NO tiene interfaz web.** Si alguien vuelve a servir paginas desde el
  servidor, esta deshaciendo la decision del 2026-08-13.
- **El programa solo mata los servicios que EL arranco.** Ver [[modulo-programa]].
- **Flutter no cruza a arm64**: el `.deb` de arm64 sale sin programa, y el
  constructor lo avisa. No es un fallo, es un limite.
- **Lo que compila para el escritorio va con `dart:io`**; ya no hay
  exportaciones condicionales por objetivo, porque ya no hay objetivo web.

## Autenticacion
- **No hay usuario ni clave de fabrica.** El primero lo crea quien entra. Si
  alguien vuelve a sembrar uno, se abre la puerta que se cerro el 2026-08-13.
- **`ComprobarClave` recibe el USUARIO ademas de la clave**: en TUXOR el usuario
  forma parte del calculo. Cambiar el nombre de un usuario invalida su clave.
- **TUXOR exige un operador** (`+ - * % ^ & | < > #`) al principio o al final del
  usuario o de la clave. Sin eso no hay hash posible. El formulario lo avisa
  mientras se teclea Y el servidor lo comprueba: un navegador viejo o una
  peticion hecha por fuera se saltan el formulario.
- **El vector oficial de TUXOR es una prueba, no un adorno**
  (`+tuxor` / `*algorithm#` → `663b623d...`). Esta implementacion es la cuarta;
  si se desvia, una clave guardada por MiRed no la verifica ninguna otra
  herramienta de la casa.

## Formularios (regla de la casa)
- **El formulario restringe exactamente lo que acepta la columna**, en Flutter y
  en el servidor. Hoy: usuario 60, nombre 120, clave >= 8, VLAN 1-4094, CIDR con
  formato. Si se cambia de un lado, se cambia del otro **el mismo dia**.
- Un campo de solo despliegue **nunca** lleva el `name` de su columna.

## Interfaz Flutter
- **`setState` con cuerpo de flecha y una asignacion revienta** si lo asignado es
  un Future: la expresion lo devuelve y hay una asercion contra eso. Usar
  siempre llaves: `setState(() { _x = algo(); });`. Solo falla en compilacion de
  **depuracion**, asi que probar en el navegador (release) NO lo detecta.
- Las pruebas de pantalla necesitan **dos destrabes**: `HttpOverrides.global`
  (el framework bloquea la red a proposito) y `probador.runAsync` (la respuesta
  del servidor llega por el reloj de verdad, no por el falso de la prueba).
- **`Picture.toImage()` en una prueba se cuelga hasta el timeout de 10 minutos si
  no va dentro de `probador.runAsync`.** No falla: se queda esperando, porque la
  conversion la hace el motor en otro hilo y el reloj falso nunca le da turno.
  Mordio al probar la exportacion del mapa a PNG.
- **Lo que solo existe en web (bajar archivos, abrir enlaces) va en un archivo
  aparte con exportacion condicional** (`servicios/descarga.dart`
  → `descarga_web.dart` si `dart.library.js_interop`, si no `descarga_generica.dart`).
  Importar `package:web` directo desde una pantalla rompe `flutter test`, que
  corre en la maquina virtual de Dart, no en un navegador.
- **No contar renglones para saber cuantos hay**: usar la cuenta que manda el
  servidor. En el mapa, un puerto con 9 MAC decia "1 equipos" cuando la respuesta
  traia un renglon resumido. La cuenta buena es `cuantosEnPuerto`.

## Leer paquetes y protocolos binarios
- **NetFlow y sFlow dicen los dos "version 5".** NetFlow la pone en 2 bytes y
  sFlow en 4: un datagrama de sFlow leido como NetFlow da version 0, y ese cero
  es la unica senal para distinguirlos. Es exacto, no heuristico.
- **NetFlow v9 e IPFIX necesitan la plantilla ANTES que los datos**, y la
  plantilla es **de cada exportador**: dos routers pueden numerar la 256 para
  cosas distintas. Mezclarlas no da error, da cifras equivocadas. Que lleguen
  datos antes que su plantilla es lo NORMAL al arrancar: se tiran callados.
- **En IPFIX, un campo de fabricante trae 4 bytes mas** en la plantilla. Sin
  saltarlos, todo lo que sigue se lee corrido.
- **Saltar las etiquetas de VLAN.** Un puerto espejo y un switch con VLAN mandan
  la trama con su etiqueta: sin saltarla, el tipo de protocolo se lee 4 bytes
  antes y el trafico de esa red desaparece entero, sin un solo error.
- **LLDP y CDP ponen el puerto local en un lugar distinto del indice**: LLDP en la
  posicion 1 (`tiempo.puerto.vecino`) y CDP en la 0 (`ifIndex.vecino`).
  Confundirlos cuelga cada vecino del puerto equivocado: el mapa sale plausible
  y falso.
- **sFlow ESTIMA, no cuenta.** Se multiplica por la tasa de muestreo. Esa
  diferencia viaja en la columna `estimado` hasta la pantalla; una vez juntas en
  la misma tabla, ya no hay forma de volver a separarlas.
- **Nunca contar renglones para saber cuantos hay**: usar la cuenta que manda el
  servidor. En el mapa, un puerto con 9 MAC decia "1 equipos".
- **Un equipo con `origen = 'manual'` NUNCA entra en un barrido.** Todo lo que
  recorra `equipos` para marcar ausencias, calcular presencia o disparar alertas
  tiene que filtrarlo: un switch no administrable no tiene direccion y no va a
  contestar jamas. Ya esta hecho en `marcarAusentes`; el proximo sitio que
  compare "lo visto" contra "lo que hay" tiene que acordarse.
- **Hay TRES origenes de un enlace, no dos**: medido (SNMP/LLDP/CDP), deducido
  (varias MAC en un puerto) y **tecleado** (`origen_dato = 'manual'`). Todo lo que
  dibuje, exporte o resuma el mapa tiene que dejar ver cual es cual — pantalla,
  PNG, SVG, PDF y CSV. Presentarlos igual hace pasar una declaracion por una
  medicion. Ver [[modulo-topologia-manual]].
- **`enlaces` y `enlaces_fisicos` NO son la misma tabla.** `enlaces` (migracion
  0004) guarda lo que un switch **anuncia** de su vecino por LLDP/CDP: nombre,
  chasis y puerto remoto como texto suelto. `enlaces_fisicos` (0011) guarda el
  cable **ya resuelto**, con las dos puntas apuntando a filas de la base.
  Confundirlas al escribir una consulta da un mapa plausible y falso.

## Procesos
- **Un solo escritor**: `mired-servidor`. La sonda escanea y entrega por socket,
  y `mired-dpi` (opcional) hace lo mismo con lo que captura.
- **La sonda comprueba de verdad si puede abrir un socket crudo**, no supone por
  el usuario: con `AmbientCapabilities` se tienen permisos sin ser root.
- El `postinst` **nunca falla**: avisa y sigue.

## Go
- El punto de entrada de Flutter es `lib/principal.dart`, **no** `main.dart`: hay
  que compilar con `-t lib/principal.dart` (ya esta en `herramientas/construir.sh`).
- `internal/` es palabra reservada del compilador de Go y por eso es la unica
  carpeta en ingles. Adentro, todo en espanol.

## Interfaz: contadores y botones
- **Una pantalla que recibe un objeto por parametro se queda con esa foto.**
  `PantallaRed` mostraba el contador de alertas de `_red`, que llegaba del panel
  de inicio y nunca se releia: se despachaban las alertas y la campanita seguia
  con el numero viejo hasta salir y volver a entrar. Cualquier contador que se
  pinte desde un objeto recibido tiene que **volver a pedirlo** al recargar.
- **Un icono suelto no se encuentra.** El boton de editar el cableado era un
  lapiz entre otros dos iconos y la primera persona que uso el programa no lo
  vio. Un icono solo se reconoce cuando ya sabes que estas buscando: si la accion
  es nueva, va **con su nombre** y **donde va la mano**, no donde encaja mejor en
  el codigo.

## Pruebas
- **Las bases de prueba van en `/dev/shm`, no en `t.TempDir()`** (helper
  `carpetaDePrueba`). Cada prueba crea dos bases con dos docenas de migraciones;
  sobre disco son cientos de sincronizaciones que no prueban nada. Con el equipo
  ocupado —una compilacion de Flutter al lado basta— eso hacia que abrir una base
  tardara mas de 10 s y **la suite fallara con "context deadline exceeded" por
  algo que no tenia que ver con lo que se estaba probando**. Reproducido y
  corregido el 2026-08-13: de 118 s con 5 fallos a 0.6 s sin ninguno.
  Ver [[historial-bugs]].
- Las pruebas de exportacion del mapa a PNG necesitan `probador.runAsync` o se
  cuelgan 10 minutos (ver seccion de Flutter).
- **Cambiar de pestana en una prueba necesita DOS esperas distintas**, en este
  orden: `pump` en el reloj falso para que termine la animacion, y luego el
  reloj **de verdad** (`runAsync`) para que llegue la respuesta del servidor de
  mentira. Con solo la primera se ve la pestana vacia con su rueda girando y la
  prueba falla sin que haya nada roto; con solo la segunda no se ha cambiado de
  pestana todavia. Helper: `cambiarDePestana` en `pantallas_test.dart`.
- **`pumpAndSettle` no sirve en estas pantallas**: espera a que NO quede ninguna
  animacion, y siempre queda alguna rueda de carga girando, asi que se agota el
  tiempo sin que falle nada de verdad.
- **Cuidado con la prueba que pasa porque no cargo nada.** `expect(find.text('2'),
  findsNothing)` se cumple igual si la pantalla esta vacia. Para comprobar que
  algo se releyo, que el servidor de mentira conteste un valor **distinto y
  reconocible** y buscar ESE.

**Ver tambien:** [[mired-arquitectura]], [[contrato-api]], [[mired]],
[[modulo-inspeccion]]

## El vocabulario: es PUERTO, no boca

Lo que se llamaba "boca" se llama **puerto** en todo el sistema desde la
v1.15-21. Donde el conector fisico y el puerto TCP puedan confundirse se dice
**puerto fisico** y **puerto TCP**, con todas sus letras.

- La migracion **0004 no se toco**: una migracion ya aplicada se queda como se
  aplico. El cambio de nombre de la columna lo hace la **0013**.
- En `internal/version/historial.toml` las entradas viejas siguen diciendo
  "bocas": son el registro de lo que se entrego ese dia y no se reescribe.

## Un puerto se llama LAN 3, no "puerto 3"

El **tipo mas el numero**, como esta rotulado en el aparato: `LAN 3`, `WAN 1`,
`DMZ 1`, `SFP 1`, `CONSOLA 1`. El nombre lo arma **una sola funcion**
(`nombreDePuerto`, en `interfaz/lib/modelos/tipos_de_puerto.dart`) y la usan
todas las pantallas: si cada una lo armara, el mismo puerto se llamaria de dos
maneras.

- La lista de tipos vive **tres veces** —CHECK de la tabla (red 0018),
  `TiposDePuerto` en Go y el archivo Dart— y hay dos pruebas en Go que la vigilan.
  Misma regla que [[ref-categorias]].
- **A un aparato de punta se le supone su LAN 1** (PC, TV, grabador): se conecta
  por un cable y entra por su unica toma, asi que el mapa dice las dos puntas.
  **Menos si va por el aire**: muchas laptops y tabletas ya ni traen conector de
  red. Y **menos si tiene puertos declarados**: ahi hay datos contados mirando el
  aparato y mandan esos. Ver [[modulo-topologia-manual]].

## Una definicion del catalogo suma sus condiciones (Y), no las alterna (O)

Poner `fabricantes` **y** `huella_contiene` en el mismo archivo exige las dos
cosas. Para reconocer por una **o** por la otra hacen falta **dos archivos**
(ver `ubiquiti.toml` y `ubiquiti-por-huella.toml`). Es el error mas facil de
cometer al ampliar el catalogo, y se nota como "dejo de reconocer lo que antes
reconocia".

## Lo que se ESCRIBE del catalogo no va donde se LEE

Corriendo como programa de escritorio, MiRed lee `/usr/share/mired/dispositivos`
pero **no puede escribir** en `/etc` ni en `/var/lib`. Por eso las carpetas de
escritura son configurables aparte (`carpeta_propia`, `carpeta_comunidad`) y el
programa las apunta a la carpeta del usuario al arrancar los servicios. Si se
agrega algo que escriba catalogo, tiene que usar esas dos, nunca una ruta fija.

## El WiFi no tiene puertos, y por eso va en otra tabla

Un enlace inalambrico NO se guarda en `enlaces_fisicos`: ahi cada punta necesita
un puerto, y una antena no los tiene. Vive en `enlaces_inalambricos`, donde de
una antena cuelgan **uno o varios** equipos y cada equipo cuelga de **una sola**
antena.

Si algun dia hay que colgar algo sin saber por que puerto entra, **no se
reutiliza esa tabla**: se llamaria "inalambrico" a un cable, y este proyecto no
miente en los nombres.

## Lo que el modem ve por cable NO sirve para colgar nada

El panel de un modem lista toda su LAN, incluido lo que esta **detras del
switch**. Colgar de el a todos los que reporta dibujaria una red plana donde en
realidad hay un arbol.

Solo lo INALAMBRICO arma topologia: un cliente asociado a un radio cuelga de ese
radio y no hay intermediario posible. Lo de cable se aprovecha unicamente para
poner nombre a los equipos.

## Una clave nunca viaja sin que alguien la pida

- No sale en `/equipos`, ni en la topologia, ni en el mapa exportado.
- Sale solo por `GET .../credencial/clave`, que exige escritura y **anota en la
  bitacora quien la pidio**.
- Se guarda cifrada, y **la llave vive con la configuracion, no con los datos**:
  si algun dia se mueve a la carpeta de datos, el cifrado deja de proteger el
  respaldo, que es lo unico de lo que protege.
- Guardar con la clave en blanco **conserva** la que habia. En un formulario
  donde se muestra oculta, en blanco significa "no la toques".

## El mapa crece hacia la DERECHA

Cada nivel es una columna y los hermanos van en lista hacia abajo. Dibujado de
arriba abajo, un switch de ocho puertos abria el plano en abanico: las lineas se
cruzaban y las cajas se salian de la pantalla.

Dos consecuencias al tocar `mapa_plano.dart`:
- `medir()` mide **alto**, no ancho. Lo que compite por sitio son las filas.
- El puerto por donde un aparato **sube** al de la izquierda no se dibuja como
  caja: esa conexion ES la linea que llega. Sigue contando como ocupado.

## Mientras se edita el mapa, la agenda de esa red se para

Editar es DECLARAR cableado, no medir. La interfaz pide la pausa al entrar en
modo edicion y la renueva cada diez minutos; **siempre lleva vencimiento** (tope
de 30 minutos) para que un programa que se cierra de golpe no deje una red sin
vigilancia para siempre.

Un barrido pedido A MANO se lanza igual: eso lo pidio una persona a proposito.

Al escribir pruebas de pantalla que dibujen el mapa, pasar
`pausarAgendaAlEditar: false`: si no, la peticion queda en vuelo y la prueba
falla por un reloj pendiente que no es ningun error.

## `ubicacion` es DONDE ESTA, no de donde cuelga

Son dos preguntas distintas y las dos hacen falta: un aparato puede colgar del
puerto 7 del switch y estar en el consultorio 4. Lo primero es el cableado
(`enlaces_fisicos`), lo segundo es el sitio (`equipos.ubicacion`), y quien va a
desconectarlo necesita el segundo.

No confundir con `equipos_snmp.ubicacion`, que es el `sysLocation` que **el
aparato dice de si mismo**. La de `equipos` **la escribe una persona**, y un
switch tonto no contesta nada y tambien esta en algun sitio.

Si se toca, se toca en los CUATRO sitios a la vez —alta a mano, ficha,
propiedades y hoja exportada—: es la regla de concordancia de la casa.

## Al importar, una celda vacia NO borra

Significa «no lo se», no «borralo». Quien deja MODELO en blanco no esta pidiendo
que se borre lo que alguien tecleo hace un mes. Es la misma regla que ya seguia la
clave de una credencial. Ver [[modulo-importacion]].

## Leer un ODS o un XLSX tiene dos trampas de formato

Las dos corren las columnas y las dos son silenciosas:

- **ODS**: LibreOffice comprime las celdas vacias seguidas con
  `number-columns-repeated="4"`. Ignorarlo desplaza todo lo que viene detras del
  primer hueco.
- **XLSX**: Excel **no escribe** las celdas vacias, y los textos viven en una
  tabla compartida (`sharedStrings.xml`). Hay que colocar cada celda por la letra
  de su referencia (`C2`) y resolver el indice del texto.

Escribirlos es mas facil que leerlos, y el proyecto hace las dos cosas:
`interfaz/lib/servicios/hoja_calculo.dart` los escribe,
`internal/importacion/lector.go` los lee.

## El color NO se escribe en las pantallas

MiRed es verde de terminal sobre negro, y ese verde vive en **un solo sitio**:
`verdeDeMiRed` y `esquemaOscuroDeMiRed`, en `interfaz/lib/principal.dart`. Se
toca ahi y se repinta la aplicacion entera.

En una pantalla se usa `Theme.of(contexto).colorScheme.…`, **nunca** un
`Colors.green` ni un `Colors.lightGreenAccent`: un verde de la paleta de Material
al lado del verde del programa no se lee como otro color, se lee como un error de
pintura. Ya paso con el icono de la agenda y con los puntos de «presente».

Dos apuntes del tema oscuro:
- **Los tonos van escritos, no salen de la semilla.** `ColorScheme.fromSeed`
  saca un terciario **azul** girando la rueda de color, y un turquesa suelto
  rompe el conjunto. Se parte de la semilla y se fijan a mano los que se ven.
- **El rojo se queda rojo.** Es el unico color que no es verde en toda la
  aplicacion: un aviso de que algo se rompio no se pinta del color de la marca.

## Arriba del panel de redes solo va lo de TODO el programa

La barra de la pantalla de redes es para lo que no pertenece a ninguna red:
usuarios, catalogo de dispositivos, actualizar y la cuenta. Lo que se usa
trabajando sobre UNA red —las credenciales SNMP con las que se le pregunta a sus
switches, su controladora WiFi— se llega desde la pantalla de esa red.

Ojo con el matiz: **esas dos cosas se guardan una sola vez y las comparten todas
las redes** (viven en el catalogo, no en la base de cada red, para no repetirlas
sitio por sitio). Por eso sus pantallas lo dicen en el titulo. Se llega a ellas
desde una red porque es donde se necesitan, no porque sean de esa red.
