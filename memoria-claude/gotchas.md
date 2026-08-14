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
  servidor. En el mapa, una boca con 9 MAC decia "1 equipos" cuando la respuesta
  traia un renglon resumido. La cuenta buena es `cuantosEnBoca`.

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
- **LLDP y CDP ponen la boca local en un lugar distinto del indice**: LLDP en la
  posicion 1 (`tiempo.puerto.vecino`) y CDP en la 0 (`ifIndex.vecino`).
  Confundirlos cuelga cada vecino de la boca equivocada: el mapa sale plausible
  y falso.
- **sFlow ESTIMA, no cuenta.** Se multiplica por la tasa de muestreo. Esa
  diferencia viaja en la columna `estimado` hasta la pantalla; una vez juntas en
  la misma tabla, ya no hay forma de volver a separarlas.
- **Nunca contar renglones para saber cuantos hay**: usar la cuenta que manda el
  servidor. En el mapa, una boca con 9 MAC decia "1 equipos".
- **Un equipo con `origen = 'manual'` NUNCA entra en un barrido.** Todo lo que
  recorra `equipos` para marcar ausencias, calcular presencia o disparar alertas
  tiene que filtrarlo: un switch no administrable no tiene direccion y no va a
  contestar jamas. Ya esta hecho en `marcarAusentes`; el proximo sitio que
  compare "lo visto" contra "lo que hay" tiene que acordarse.
- **Hay TRES origenes de un enlace, no dos**: medido (SNMP/LLDP/CDP), deducido
  (varias MAC en una boca) y **tecleado** (`origen_dato = 'manual'`). Todo lo que
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

**Ver tambien:** [[mired-arquitectura]], [[contrato-api]], [[mired]],
[[modulo-inspeccion]]
