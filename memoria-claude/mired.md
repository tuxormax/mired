---
name: mired
description: "MiRed, servicio propio para mapear redes en Go + Flutter; estado, alcance y decisiones de fondo"
metadata: 
  node_type: memory
  type: project
  originSessionId: 20376d18-adf7-4315-bb9c-98a3aa84ec95
  modified: 2026-08-13T23:59:00.000Z
---

# MiRed — servicio propio de mapeo de redes

Proyecto en `/home/tuxor/www/mired`, definido el **2026-08-12**. Descubre los
equipos de una red, dibuja su mapa y dice en que puerto de que switch esta cada
aparato. El plan por fases esta en `PLAN.md` (versionado); esta memoria guarda
**las decisiones y su porque**, que es lo que no se debe volver a discutir.

## El giro del 2026-08-12: ya NO es un fork
El plan original era forkear [Scanopy](https://github.com/scanopy/scanopy)
(AGPL-3.0) y quitarle los topes. **El usuario lo descarto el mismo dia**: quiere
proyecto propio sin depender de nadie.

**Lo que eso cambio, y no es poco:**
- **Lo legal**: la AGPL viaja con el codigo, no con el historial de git. Copiar
  archivos —aunque sea sin historia— obligaria a MiRed a ser AGPL. Leyendo su
  codigo para entender como resolvieron algo y escribiendo el nuestro, MiRed es
  propio y su licencia se elige libre. **Scanopy = solo lectura de referencia.**
- **Lo practico**: desaparecieron las tres fases mas caras y riesgosas del plan
  viejo (rename de 2 509 apariciones, puerto a SQLite de codigo ajeno, romper su
  suposicion de "una sola base"): entre 5 y 9 semanas de pelear contra decisiones
  de otro.
- **Se acabo la excepcion del ingles**: siendo codigo propio, **todo va en
  espanol** como el resto de los proyectos de la casa.

## Stack elegido (2026-08-12)
El usuario pidio "go o rust con flutter para generar un .deb instalable" y
autorizo Python si hace falta. **Se eligio Go**, por tres razones concretas:
1. Binario estatico sin dependencias → el `.deb` no pide runtime ni servidor de BD.
2. Cruza a `arm64` (Raspberry Pi) con una variable de entorno; Rust exige montar
   toolchain cruzado.
3. ARP crudo, SNMP, mDNS y el driver de SQLite existen **en Go puro, sin cgo** —
   que es lo que permite que el binario sea de verdad estatico.
Ademas WatchYourLAN, la herramienta mas liviana de la categoria, es Go: el
presupuesto de recursos es alcanzable. Rust daria algo menos de memoria a cambio
de un lenguaje que hoy no se maneja en casa.

**MiRed es un PROGRAMA DE ESCRITORIO en Flutter** (decidido el 2026-08-13, tras
haber tenido interfaz web). Se instala el `.deb` y aparece en el menu. **No hay
interfaz web y no se compila ninguna**: el servidor solo expone la API.

**El programa manda sobre los servicios**: los levanta al abrirse y los mata al
cerrarse (`interfaz/lib/servicios/supervisor_generico.dart`). Datos en
`~/.local/share/mired`, no en `/var/lib`. Dos reglas que no se pueden olvidar:

1. **Solo mata lo que el arranco.** Si al abrir ya habia un MiRed vivo, se cuelga
   de el y al cerrar no lo toca: matar el servicio de otro dejaria sin
   vigilancia una red que si la tenia.
2. **Con el programa cerrado MiRed no vigila nada** — ni escaneos programados, ni
   alertas, ni flujos del router. Es el precio elegido a cambio de "se instala y
   ya". Para vigilancia continua (Raspberry en sucursal) el paquete trae las
   unidades de systemd **instaladas y apagadas**; las dos formas NO se mezclan.

La sonda recibe `CAP_NET_RAW` por `setcap` sobre el binario, no por systemd:
lanzada por un usuario normal, es la unica forma.

**Python solo auxiliar**: scripts, generadores del catalogo, laboratorio. Nunca
en el servicio, para no romper el binario unico.

## Herramental verificado en el equipo (2026-08-12)
Go 1.26.4 y Flutter 3.35.6 **ya instalados**. No hay Rust, ni Docker, ni
`sqlite3` de linea de comandos. Node es 18. PostgreSQL 16 esta (ya no hace falta).

## Versionado — se corrigio el 2026-08-13
Estaba mal y lo detecto el usuario. **`v1.X` NO es versionado semantico: X es el
NUMERO DE MODULOS.** Se habia subido a v1.1 "porque el cambio era grande", que es
el criterio de medio mundo pero no el de la casa. MiRed tenia **14 modulos**
(confirmados con el usuario), asi que era **v1.14**; con la topologia manual
(2026-08-14) son **15**, y por eso **v1.15**. La revision sigue su cuenta sin
reiniciarse al cambiar de version: va en **20**.

Lo que faltaba y ya esta:
- **Tablas `versionessistema` y `versiondb`** en el catalogo (migracion 0004),
  que son estandar de la casa y aqui no existian.
- **`internal/version/historial.toml`** es la fuente de la verdad, y **una sola**:
  de ahi salen la version y la revision con que compila `construir.sh` —ya no se
  teclean, que era como se podian reiniciar sin que nadie se enterara— y de ahi
  se siembran las dos tablas en cada arranque, de forma idempotente.
- **Pie clickable** que abre el historial, como en los demas proyectos.

Tres pruebas lo vigilan: que el binario y el historial digan la misma version,
que la revision **no tenga huecos ni repetidos**, y que sembrar mil veces no
duplique filas.

## Estado real (2026-08-17, v1.20 Rev 41)
**Las 10 fases cerradas**, salvo firmar los paquetes. Los **20 modulos**: los 15
del plan mas el **catalogo comunitario** (16), la **escucha del aire** (17), los
**enlaces inalambricos** (18), las **credenciales por equipo** (19) y la
**importacion de hojas** (20).

Lo hecho despues de cerrar el plan (Rev 21-41):
- **Reconocimiento de verdad** (Rev 21-27): lista IEEE completa —52 977
  prefijos, antes 99— y huellas activas: titulo y encabezados de su pagina,
  certificado TLS, banner SSH, mDNS, UPnP, ONVIF y los protocolos propios de
  Ubiquiti, MikroTik, TP-Link (Kasa y Omada), Hikvision, D-Link, NETGEAR, Dahua,
  Xiaomi, Ruckus, Tuya y Fortinet. Antes medio parque salia como «servidor web».
  Ver [[modulo-reconocimiento]].
- **Modulos 16-19** (Rev 24-31): aportar al catalogo desde el programa, saber
  que SSID emite cada antena, colgar equipos del WiFi de uno en uno o de varios a
  la vez, y guardar cifradas las credenciales del panel de cada aparato —las
  mismas con las que MiRed entra a leerle su tabla de conectados—.
  Ver [[modulo-inalambricos]] y [[modulo-aire]].
- **El mapa, redibujado** (Rev 32-37): crece hacia la derecha, cada aparato a la
  altura de su primer hijo, cables en codo con su propio canal y su color, y las
  etiquetas con las **dos puntas** del cable (`LAN 4 → LAN 1`). La ficha de un
  aparato abre en Propiedades y lleva Conexiones y Credenciales.
  Ver [[modulo-topologia-manual]].
- **Que no se coma el equipo** (Rev 34): mientras se edita el mapa la agenda de
  esa red se para, las cinco escuchas de red van a la vez y no en fila, y el
  barrido de puertos baja de 128 a 48 conexiones a la vez.
- **Importar una hoja de aparatos** (Rev 40-41, modulo 20): una instalacion
  documentada en una hoja de calculo se sube de una vez, en CSV, ODS o XLSX. De
  cada renglon salen el aparato, sus puertos, su cable y su credencial. La vista
  previa dice antes lo que se haria, renglon por renglon, sin escribir nada. De
  ahi salio tambien la columna **`ubicacion`**: donde ESTA el aparato, que no es
  de donde cuelga. Es una **pantalla completa** con la guia de llenado dentro
  —columnas, categorias, ejemplo y reglas—, y esa guia **la manda el servidor**
  desde la misma definicion con la que lee el archivo, para que no puedan
  separarse. Ver [[modulo-importacion]].
- **Lo exportado, usable** (Rev 39): se guarda **donde diga el usuario** —cuadro
  de guardar del escritorio, no la carpeta de descargas a la fuerza— y la hoja de
  calculo pasa a ser **dos tablas** («Aparatos» y «Conexiones») en **ODS, XLSX y
  CSV**. La tabla unica de antes metia en el mismo renglon un aparato, un puerto
  y un cable; sacaba cada cable dos veces y perdia entero lo que cuelga por el
  aire. Ver [[modulo-topologia]].

Lo que ya estaba:
1. Cimientos: servidor + sonda, una base SQLite por red, autenticacion con
   permisos por red, API con el estandar de errores de la casa, programa de
   escritorio y `.deb` para amd64 y arm64.
2. Descubrimiento de equipos por ARP, ICMP y TCP, con puertos, DNS inverso y
   fabricante por OUI.
3. Presencia en vivo, historial de conexiones y barridos programados por red.
4. SNMP v1/v2c/v3, tabla de MAC del switch, **LLDP y CDP**, enlaces entre
   switches dibujados en el mapa, **controladora WiFi UniFi** (el WiFi cuelga de
   su antena), mapa de puertos y perfil de capacidades.
5. Mapa visual en Flutter con exportacion a **PNG, SVG, PDF y hoja de calculo
   (ODS, XLSX y CSV)** (cerrada el 2026-08-13) y **edicion manual del cableado** (modulo 15, 2026-08-14): dar de
   alta lo que ningun escaneo ve, declarar puertos y conectarlas tocandolas, con
   modo edicion aparte de solo mirar. Ver [[modulo-topologia-manual]].
6. Catalogo abierto de dispositivos en `.toml`, con 15 definiciones semilla y el
   boton "proponer definicion" que genera el archivo ya relleno. Desde la Rev 20,
   una **lista unica de categorias** que comparten el catalogo y el alta manual,
   y el **contador de de que esta hecha la red** en la pestana de equipos.
   Ver [[ref-categorias]].
8. Ancho de banda: contadores SNMP por puerto (con tasa calculada entre dos
   lecturas) y receptor de flujos **NetFlow v5, NetFlow v9, IPFIX y sFlow** del
   router, que es lo que mide consumo donde no hay switches administrables. El
   formato se reconoce solo.
9. Publicacion: **AGPL-3.0**, repo publico, documentacion ES/EN y las **releases**
   publicadas desde el 2026-08-14, con el `.deb` de amd64.
   **Falta FIRMAR los paquetes**; por ahora las notas de la release solo llevan
   la huella SHA-256, que es lo unico con que comprobar el archivo.
   `arm64` no se publica: se construye a mano y va sin programa de escritorio,
   porque Flutter solo compila la interfaz para amd64.
10. **Inspeccion profunda** (`mired-dpi`): que aplicacion consume, sin descifrar
   nada. Va en el mismo `.deb` pero apagada. Ver [[modulo-inspeccion]].
7. Motor de alertas completo: las 6 reglas detectan (equipo nuevo, ausente,
   puerto nuevo, cambio de IP, cambio de puerto de switch y red que dejo de
   reportar) y avisa por ntfy, Telegram, correo y webhook.

**Probado de verdad:** el `dpkg -i` en el equipo del usuario (2026-08-13, funciono),
el descubrimiento contra la red de casa, los barridos programados, las alertas,
el catalogo, el receptor de flujos y el `.deb` desempaquetado corriendo aparte.

**Probado el 2026-08-13 en el equipo del usuario:** `dpkg -i`, el programa
abriendose desde el menu, **el supervisor levantando los dos servicios**, el
primer acceso creando el administrador, y el formulario de red proponiendo la red
detectada.

**NO probado:** que los servicios mueran al cerrar la ventana; un escaneo
completo desde el programa; SNMP y CDP contra un switch administrable real; la
controladora UniFi contra una de verdad; la inspeccion profunda contra un
puerto espejo; y **conectar puertos tocandolas con el raton en la ventana** (la
API entera y el dibujo del plano si estan cubiertos por pruebas). El switch administrable sigue
siendo el riesgo abierto mas grande.

## Cobertura (2026-08-17)
**231 pruebas en Go y 93 en Flutter**, sobre ~27 000 lineas de Go y ~14 900 de
Dart, en 15 paquetes. Mas `herramientas/probar.sh` con **74 comprobaciones**:
construye el `.deb`, lo desempaqueta y recorre el flujo completo. Es la unica que
prueba lo que de verdad se entrega.

Lo que mas se cuida es **lo que falla sin dar error**: leer un registro de flujos
con la plantilla de otro router, colgar un vecino del puerto equivocado,
multiplicar mal la tasa de muestreo de sFlow, o desviarse del vector oficial de
TUXOR. Ninguna de esas revienta nada; todas dan resultados plausibles y falsos.

## Herramientas del repo
- `herramientas/construir.sh` — compila y arma el `.deb`. **Los instaladores
  quedan en `instaladores/` y solo ahi** (regla del usuario, 2026-08-13: antes
  salian a `empaquetado/salida/` y no los encontraba).
- `herramientas/probar.sh` — la prueba de humo sobre el paquete.
- `herramientas/desinstalar.sh` — **quita MiRed sin dejar rastro**. Hace falta
  porque `dpkg --purge` conserva las bases a proposito, y para probar en limpio
  eso estorba.
- `herramientas/enviar_sflow.py` — datagrama de sFlow de mentira, para la prueba.

## Consumo de recursos medido (2026-08-12)
Sobre el `.deb` de amd64, escaneando un `/24` completo:
- **En reposo**: servidor 12.2 MB, sonda 5.1 MB.
- **Pico durante el escaneo**: servidor 14.7 MB, sonda 15.8 MB.
- Un `/24` completo (254 direcciones x 38 puertos) tarda **12 segundos**.
- Menos de 31 MB entre los dos. **Cabe de sobra en una Raspberry Pi**, que era el
  requisito del plan. Se vuelve a medir al cerrar cada fase.

## Tiempos
- **3-4 meses** hasta la fase 9 (publicable). La inspeccion profunda va aparte.
- **Al cerrar la fase 2 ya hay `.deb` instalable que inventaria la red**, y eso
  solo ya reemplaza varias herramientas. Se usa en produccion propia desde ahi.

## La licencia: AGPL-3.0 (decidido el 2026-08-13)
El codigo es propio, asi que no se heredaba ninguna. Se reviso que usa el usuario
en sus otros repos: **los publicos van con GPL-3.0** (`niveladordevolumen`,
`pcinfo`) y los de cliente con licencia propietaria. La regla de facto de la casa
es *herramienta que se publica -> GPL*.

**Se subio a AGPL-3.0 y no GPL-3.0** pensando en que MiRed se usaba desde el
navegador: con la GPL, quien lo monte como servicio de pago nunca entrega el
binario y por lo tanto no esta obligado a publicar sus cambios; la AGPL cierra
ese hueco.

**Ojo con esto ahora que MiRed es un programa de escritorio:** el articulo 13
—el que obliga a ofrecer el codigo a quien use el programa por red— casi no
llega a aplicar, porque ya nadie interactua por red con el. La licencia se queda
igual: sigue siendo la que mas protege si algun dia alguien lo ofrece como
servicio. El enlace al codigo en el pie del panel (`_PieVersion` en
`interfaz/lib/pantallas/redes.dart`) tampoco se quita: cuesta nada y es buena
practica.

## ▶ POR DONDE SEGUIR (al 2026-08-13, fin de la segunda jornada)

**El programa se instalo y se uso de verdad**, y de ahi salieron TODOS los fallos
de la jornada. Comprobado en el equipo del usuario:

- `dpkg -i` funciona y el programa aparece en el menu.
- **El supervisor levanta `mired-servidor` y `mired-sonda` al abrir.**
- El primer acceso crea el administrador y entra.
- Crear una red.

**Lo que NO se ha visto todavia:** un escaneo terminando con datos, que los
servicios se maten al cerrar la ventana, y todo lo que dependa de equipo real.

### ▶▶ LO SIGUIENTE, ya acordado con el usuario: EL MAPA DE CAPA 3

Es lo mas grande que quedo pendiente y esta decidido, solo falta escribirlo. El
mapa de hoy solo dibuja capa 2 —quien cuelga de que puerto—, que en una red sin
switches administrables **no dibuja nada**: todo cae en "sin ubicar". El plan
prometia capa 2 **y capa 3** desde la fase 5, y la capa 3 nunca se hizo.

Lo acordado, en tres partes:

1. **Detectar la puerta de enlace y dibujar desde ahi.** El modem arriba y todo lo
   descubierto colgando, **diciendo con todas sus letras** que el switch de en
   medio no puede precisar el puerto. MiRed hoy **no detecta el gateway**: no hay
   una sola linea que lo mire. Es la base; sin eso lo demas no tiene de donde
   colgar.
2. **Preguntarle a todo el que conteste** cuantos puertos tiene y que hay en cada
   una. **Ya esta hecho** (revision 17): se prueba la comunidad de fabrica en
   cada escaneo, sin que nadie configure nada.
3. **Que el usuario pueda declarar "aqui hay un switch"** y colgarle los equipos
   que el ve por el cable. Con **su regla del tamaño minimo**: N equipos + el
   cable de subida = al menos N+1 puertos, redondeado al siguiente tamaño de
   fabrica (5, 8, 16, 24, 48). Se dice **"al menos 8 puertos"**, nunca "es de 8":
   un switch con 5 equipos puede ser de 8 con tres libres o de 16 con once. Y lo
   declarado se marca **distinto de lo detectado**, para no confundir lo que se
   sabe con lo que alguien dijo.

**El limite que no se puede cruzar, y hay que repetirselo al usuario sin adornos:**
el puerto exacto solo lo sabe el switch. Uno simple no puede contarlo. No es un
limite de MiRed. Ver [[red-del-usuario]].

### Lo demas, en orden de valor:

1. **Terminar el primer escaneo** y ver el inventario, el mapa y las alertas con
   datos de verdad. Es donde se quedo el usuario, y desbloquea comprobar el mapa,
   la exportacion y las alertas de una sola vez. **De paso se sabra la direccion
   del AP Ubiquiti y si habla SNMP**, que es la unica esperanza de esa red.
2. **Comprobar que al cerrar el programa mueren los servicios.** Es la mitad del
   trato del supervisor y la unica parte sin verificar. Basta cerrar la ventana y
   mirar si quedan procesos.
3. **Cerrar la fase 9.** Falta subir los `.deb` a una release de GitHub. El
   usuario **ya eligio la opcion 1: publicar SIN firmar**, con las sumas SHA-256.
   No hay clave GPG en el equipo y crear una es decision suya, no de la sesion.
4. **Probar contra equipo real** — el riesgo abierto mas grande del proyecto:
   SNMP y CDP contra un switch administrable, la controladora UniFi contra una de
   verdad, y la inspeccion profunda contra un puerto espejo. Nada de eso se ha
   visto funcionar fuera de servidores de mentira.
5. **Medir el consumo de la inspeccion profunda.** El plan obliga a medir al
   cerrar cada fase y la 10 se cerro sin medir, porque sin puerto espejo no hay
   nada que capturar. Es justo el proceso donde mas importa: el unico que trabaja
   de continuo.
6. **El programa para arm64.** Hoy el `.deb` de arm64 sale sin el, porque Flutter
   no cruza desde amd64. Para una Raspberry hay que compilarlo en ella o montar
   compilacion cruzada.

**Lo que enseño la primera sesion de uso real, y vale para todo el proyecto:**
NINGUNO de los cuatro fallos del dia lo encontro una prueba. Los encontro alguien
usando el programa, con 144 pruebas en verde:

1. La pantalla se quedaba **en gris** al teclear el usuario (expresion regular).
2. Pedia la subred en **notacion con barra**, que el usuario no tenia por que
   conocer — y que el programa ya sabia.
3. Dejaba crear una red **sin nada que escanear**, y el barrido no encontraba
   nada sin decir por que.
4. Los reportes decian **cuando se exportaron**, no de cuando eran los datos.

Ninguno era un fallo de logica: los cuatro eran de **lo que el usuario ve y de lo
que se le pide**. Las pruebas comprobaban que el codigo hace lo que dice, no que
lo que dice tenga sentido para quien lo usa. **Abrir cada pantalla a mano antes
de darla por buena** no es opcional en este proyecto.

## Lo que falta decidir
- **Inventario de las redes reales** (marca/modelo de switches y puntos de acceso,
  cuales son administrables). Se necesita para probar SNMP contra equipo de
  verdad, que es el punto 3 de arriba.
- **Firmar los paquetes**: hace falta una clave GPG del usuario. Decidido dejarlo
  para cuando el proyecto tenga usuarios que la necesiten.

## Sin credenciales de fabrica (2026-08-13)
**Se quitaron las credenciales sembradas de todo el proyecto** el mismo dia que
el repo se hizo publico, por decision del usuario. En un proyecto publico, unas
credenciales iguales en todas las instalaciones son una puerta que cualquiera
puede buscar: basta rastrear el puerto 60072 y probarlas.

**Y se reescribio el historial de git** el 2026-08-13 con `git filter-repo`, para
que no queden ni en las revisiones viejas: borrarlas del arbol no las borra del
historial, y el repo ya era publico. Se hizo con el repo recien publicado y sin
forks, que es cuando sale barato. Por eso **no se vuelven a escribir aqui**: si
se anotaran en la memoria, volverian al repo por la puerta de atras.

**Como quedo:** una instalacion recien hecha no tiene usuarios. `/api/estado`
—que no pide sesion, y no puede pedirla— devuelve `sinEstrenar: true`, y la
interfaz pinta el formulario de crear administrador en vez del de entrar.
`POST /api/primer-administrador` es la unica ruta que crea un usuario sin sesion,
y **solo funciona mientras no haya ninguno**: esa comprobacion vive en
`autenticacion.CrearPrimerAdministrador`, no en la API.

**Las claves ahora usan TUXOR** (`internal/autenticacion/tuxor.go`), el algoritmo
de la casa, en modo seguro: `scrypt(tuxor(usuario, clave), sal)` con costo 14. Se
guarda como `tuxor$costo$sal$hash`. Dos cosas que no se pueden olvidar:

1. **El usuario forma parte del hash**, no solo la clave. `ComprobarClave` recibe
   los dos.
2. **TUXOR exige que el usuario o la clave lleven un operador** (`+ - * % ^ & | <
   > #`) al principio o al final. Es la regla que mas sorprende, y por eso el
   formulario la explica mientras se teclea en vez de rechazar al final.

La implementacion de Go es la **cuarta** (ya habia PHP, JavaScript y Python) y
tiene el vector oficial como prueba: si se desvia, las claves de una no las
verifica la otra. Ver [[gotchas]].

## Regla del usuario: nada sale a servicios externos por su cuenta (2026-08-13)
Lo exportado (mapas y demas archivos) **se guarda en el equipo y punto**: sin
Google Drive, sin subida a ninguna nube, sin envio automatico. El PNG, el SVG y
el PDF se arman **dentro del programa** —el PNG con Flutter, el SVG y el PDF
escritos a mano en `interfaz/lib/servicios/exportar_mapa.dart`, sin biblioteca de
terceros— y ni siquiera pasan por el servidor. **Donde queda el archivo lo elige
el usuario** en el cuadro de guardar del escritorio, desde el 2026-08-17; el
detalle, en [[modulo-topologia]].

**Aclarado el 2026-08-13: la regla es solo para archivos.** Los cuatro destinos
de aviso de la fase 7 (ntfy, Telegram, correo SMTP, webhook) **se quedan como
estan; el usuario dijo explicitamente que no se muevan**. Conviven con la regla
porque **ninguno se activa solo**: sin un destino dado de alta a mano en la
pantalla de alertas de la red, MiRed no manda nada a ningun lado.

**Ver tambien:** [[mired-arquitectura]], [[mired-capacidades]],
[[mired-upstream-scanopy]], [[modulo-programa]], [[red-del-usuario]]
