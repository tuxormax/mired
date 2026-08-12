# MiRed — plan del proyecto

Fork libre de [Scanopy](https://github.com/scanopy/scanopy) (AGPL-3.0) para
descubrir equipos en una red, dibujar su mapa y documentar en qué puerto de qué
switch está conectado cada aparato. Sin topes de redes ni de usuarios, con
SQLite en lugar de PostgreSQL —y una base de datos independiente por cada red—
para que quepa en un `.deb`, y con un catálogo de dispositivos que la comunidad
pueda ampliar sin recompilar nada.

---

## 1. Qué se hereda y qué se cambia

Scanopy resuelve ya la parte cara del problema: 159 000 líneas de Rust con
descubrimiento por ARP, SNMP, LLDP, CDP, mDNS y Docker, motor de topología de
capa 2 y 3, interfaz web en Svelte, exportación a PNG/SVG/PDF/Mermaid/CSV y un
demonio que se instala como servicio. Escribir eso desde cero no son meses, son
años. El fork parte de ahí.

| | Scanopy | MiRed |
|---|---|---|
| Licencia | AGPL-3.0 + licencia comercial | AGPL-3.0 (solo) |
| Redes | 1 en la edición Community | sin tope |
| Usuarios | 1 asiento | sin tope |
| Base de datos | PostgreSQL, una sola para todo | SQLite embebido, **una base por red** |
| Instalación | Docker Compose, 3 servicios | un `.deb`, dos servicios systemd |
| Catálogo de dispositivos | 257 archivos Rust, hay que recompilar | archivos `.toml` que se leen al arrancar |
| Presencia en vivo | solo escaneo programado | barrido rápido aparte, «encendidos ahora» |
| Alertas | en su hoja de ruta, y de pago | motor de reglas por red, gratis |
| Ancho de banda | no lo hace | contadores SNMP por puerto y flujos del router |
| Facturación, Stripe, planes, nube | sí | se elimina |

### Lo legal

`LICENSE.md` es la AGPL-3.0 sin cláusulas añadidas, y la interfaz está bajo la
misma licencia. Eso da derecho explícito a modificar, quitar los topes y
redistribuir. A cambio, tres obligaciones que el proyecto debe cumplir desde el
primer commit:

1. El código de MiRed se publica completo bajo AGPL-3.0.
2. La interfaz web incluye un enlace visible al código fuente (artículo 13 de la
   AGPL: quien usa el servicio por red tiene derecho a la fuente).
3. Se conservan los avisos de copyright originales y se declara el origen:
   «MiRed es un fork de Scanopy, no está afiliado ni respaldado por sus autores».

La marca no se hereda: nombre, logo y dominio son de ellos. De ahí que el rename
no sea cosmético sino un requisito legal.

---

## 2. Las cuatro decisiones que definen el proyecto

### 2.1 SQLite: dónde está el trabajo real

El acceso a datos no está disperso: vive casi entero en
`backend/src/server/shared/storage/` (unas 4 500 líneas). `generic.rs` arma el
SQL, `pg_value.rs` convierte los valores de Rust a tipos de PostgreSQL y
`filter/` traduce los filtros. Todo lo demás del backend habla con esa capa, no
con la base. Eso vuelve el cambio grande pero acotado.

Lo que hay que resolver:

- **Marcadores y dialecto**: `$1, $2` → `?`; `= ANY($1)` → `IN (?,?,…)`;
  `ILIKE` → `LIKE` (SQLite ya no distingue mayúsculas en ASCII);
  `DISTINCT ON` → subconsulta con `ROW_NUMBER()`.
- **Tipos que SQLite no tiene**: `UUID`, `INET`, `CIDR`, `MACADDR` y
  `TIMESTAMPTZ` pasan a `TEXT` con formato canónico, validado al escribir y
  parseado al leer. Son 3 375 usos de `Uuid` y 207 de `mac_address` en el
  código, pero todos cruzan la capa de almacenamiento: se tocan una vez, ahí.
- **JSONB**: SQLite trae JSON1 y cubre `json_extract`, `json_object` y
  `json_group_array`, que es lo que las consultas usan. La diferencia real es
  que JSONB indexa y SQLite no; se resuelve con columnas generadas más índice
  donde haga falta.
- **Triggers y `pgcrypto`**: hay dos triggers (borrado en cascada de etiquetas y
  reasignación de demonios) que en SQLite se reescriben con su propia sintaxis o
  se suben al código. `pgcrypto` solo se usa para hashear claves de API: se
  reemplaza por la función de hash que ya tiene el backend.
- **Concurrencia**: PostgreSQL aguanta escrituras en paralelo; SQLite serializa.
  Se activa modo WAL, `busy_timeout` y un único pool de escritura. Para el
  volumen de esta herramienta (un escaneo cada N minutos) sobra, pero hay que
  medirlo con muchas redes cargadas.

**Decisión clave sobre las migraciones.** Hay 116 migraciones (4 233 líneas de
SQL de PostgreSQL) que describen la historia del esquema desde octubre de 2025.
Traducirlas una por una es trabajo perdido y además haría imposible integrar los
cambios de arriba. La decisión es **aplanarlas**: se levanta el esquema final en
PostgreSQL, se vuelca su estructura, se traduce **una sola vez** a un esquema
base de SQLite (`0001_mired_base.sql`) y a partir de ahí MiRed lleva su propia
numeración. Cuando salga una versión nueva de Scanopy, solo se portan las
migraciones nuevas de esa versión — típicamente entre una y cinco, no 116.

### 2.2 Una base de datos por red

Al crear una red (Matriz, Sucursal 1, Bodega) se crea **su propio archivo
SQLite**, y ahí dentro vive todo lo de esa red: equipos, subredes, servicios,
puertos, interfaces, VLAN, grupos, topologías, históricos de escaneo y el
demonio que la escanea. Ni una tabla compartida con las demás redes.

**Lo que se gana**, y no es poco:

- **Se acaba la contención de escritura de SQLite**, que era el segundo riesgo
  del plan. El límite de «una escritura a la vez» es por archivo; con veinte
  redes escaneando en paralelo son veinte archivos escribiendo a la vez.
- **Respaldar un sitio es copiar un archivo.** Moverlo a otro servidor también.
- **Borrar una red es borrar el archivo**, sin barridos de tablas ni huérfanos.
- **El daño se contiene**: una base corrupta se lleva una red, no el sistema.
- **Escala de verdad a «muchas redes»**, que era el requisito.

**Cómo queda repartido.** No todo puede vivir en la base de la red: los usuarios
existen por encima de ellas. Quedan dos clases de archivo:

- **Catálogo** (`/var/lib/mired/mired.db`, uno solo): usuarios, contraseñas,
  sesiones, claves de API, permisos de qué usuario ve qué red, credenciales SNMP
  reutilizables, y el registro de redes (nombre, archivo, fecha, resumen del
  último escaneo).
- **Red** (`/var/lib/mired/redes/matriz-a1b2.db`, uno por red): todo lo
  operativo. Cada archivo lleva dentro su propia ficha de red, así que es
  **autodescriptivo**: se copia a otra instalación y se sabe qué es.

De las 29 tablas del esquema, la mayoría ya trae `network_id` y se van tal cual
al archivo de la red. Las tablas puente entre ambos mundos (etiquetas,
credenciales de equipo, permisos por red) son las que hay que decidir una por
una: o se vuelven propias de cada red, o guardan solo el identificador del lado
global sin llave foránea, validado en código.

**Dónde se enchufa, técnicamente.** El fondo del proyecto ayuda: hay un
`factory.rs` que construye un objeto de almacenamiento por cada tipo de entidad,
todos recibiendo hoy la misma conexión. Ese es el punto exacto donde se corta.
Cada entidad se marca como *global* o *de red*, y las de red resuelven su
conexión en el momento de la llamada según la red activa. La red activa la
propone el propio middleware de autenticación, que ya resuelve en cada petición
a qué redes tiene acceso quien pregunta: se guarda en el contexto de la tarea y
la capa de almacenamiento la lee. Ventaja de hacerlo así: **no hay que cambiar la
firma de las funciones** en 159 000 líneas de código.

Detalles que hay que resolver sí o sí, porque muerden:

- **Consultas que cruzan redes** («¿dónde está esta MAC en todos los sitios?»,
  el panel general): ya no son un `SELECT`. O se recorren los archivos uno por
  uno, o se usa `ATTACH` de SQLite para consultar varios en una conexión (con un
  tope prudente de archivos adjuntos a la vez). Para el panel general conviene
  además guardar en el catálogo un resumen por red (cuántos equipos, último
  escaneo, alertas) que se actualiza al terminar cada escaneo: así la pantalla
  de inicio no abre treinta archivos.
- **Archivos abiertos**: no se puede tener cien bases abiertas para siempre. Se
  mantiene un juego de conexiones de las redes usadas hace poco y las demás se
  cierran solas.
- **Migraciones**: ya no se aplican una vez sino a cada archivo. Al crear una red
  se levanta el esquema; al abrir una vieja o restaurada de respaldo, se
  actualiza antes de tocarla.
- **Nombre del archivo**: `matriz.db` es bonito pero se rompe con acentos,
  espacios, cambios de nombre y dos redes que se llamen igual. Se usa
  `nombre-corto-<id>.db` (`matriz-a1b2.db`): legible para respaldos y sin
  choques. El nombre bonito vive dentro del archivo y se puede cambiar cuando se
  quiera.

**El costo, dicho claro**: este es el cambio más invasivo del fork, más que
SQLite. Cambiar de motor es mecánico; romper la suposición «todos los datos
están en una sola base» es arquitectónico, y arriba van a seguir dando esa
suposición por buena para siempre. Cada función nueva de Scanopy que junte una
tabla global con una de red va a chocar aquí. Por eso el enrutador tiene que
quedar **concentrado en la capa de almacenamiento** y no repartido por los
módulos: es lo que decide si el fork se puede seguir sincronizando o no.

### 2.3 El catálogo de dispositivos como el diferenciador

Hoy cada uno de los 257 servicios que Scanopy reconoce es un archivo Rust que se
registra al compilar. Aportar un dispositivo nuevo exige saber Rust, clonar el
repo, compilar y esperar a que acepten el cambio. Ese es exactamente el cuello
de botella que impide que un catálogo crezca por comunidad.

Los patrones de detección, sin embargo, ya son declarativos por dentro: son un
árbol de `Puerto`, `TodosDe`, `AlgunoDe`, fabricante por MAC y consulta a un
endpoint. Es decir, son **datos disfrazados de código**. El plan los convierte en
datos de verdad:

- Un formato `.toml` por dispositivo: nombre, descripción, categoría, puertos,
  fabricante OUI, ruta HTTP a consultar, cadena SNMP esperada, icono.
- Se leen al arrancar desde `/usr/share/mired/dispositivos/` (los que trae el
  paquete) y `/etc/mired/dispositivos/` (los del usuario, que mandan sobre los
  anteriores). Agregar un dispositivo = copiar un archivo y reiniciar el
  servicio; sin compilar.
- Los 257 existentes se convierten con un script, no a mano.
- En la interfaz, todo equipo que quede sin identificar ofrece un botón
  **«proponer definición»** que genera el `.toml` ya relleno con lo que se vio
  (puertos abiertos, OUI, banners). El usuario revisa, le pone nombre y lo manda
  como aportación al repo.
- El repositorio valida cada aportación automáticamente: que el `.toml` sea
  válido, que no duplique otro y que el patrón no sea tan amplio que etiquete
  medio mundo.

### 2.4 Redes mixtas: unos switches administrables y otros no

Este es el escenario real y hay que diseñar para él desde el principio, no
parchearlo después. «Qué puerto está conectado a qué» solo se sabe si el switch
es administrable y responde SNMP, LLDP o CDP; con uno no administrable no hay a
quién preguntarle y ninguna herramienta del mundo puede inventarlo. Pero eso no
deja la red a oscuras: **degrada por capas**, y el trabajo de MiRed es que la
degradación sea honesta y visible en vez de un mapa con huecos sin explicar.

**Qué se sigue sabiendo sin un solo switch administrable:**

- Quién está en la red y quién se acaba de conectar (barrido ARP).
- Qué es cada aparato, qué servicios expone y qué puertos tiene abiertos.
- Topología de capa 3: subredes, salidas, rutas, qué red habla con cuál.
- Todas las alertas de cambio: equipo nuevo, equipo que desaparece, puerto
  abierto que antes no estaba.

**Qué se pierde:** el puerto exacto del switch, y el consumo por puerto.

**Lo que sí se puede inferir, y es más de lo que parece.** Si un switch
administrable ve **varias MAC colgando de una misma boca**, ahí hay un switch no
administrable o un punto de acceso, y todo lo que aparece por esa boca está
detrás de él. No da el puerto exacto, pero da el grupo: «estos nueve equipos
cuelgan del puerto 7, a través de un switch tonto». Para un plano de sitio eso
suele bastar. El código heredado ya trabaja en esa dirección —distingue enlaces
confirmados de inferidos y contempla explícitamente el aparato no administrable
al final de la línea—, así que es cuestión de aprovecharlo y presentarlo bien.

Además, el WiFi nunca tiene puerto de switch: ahí el punto de acceso es la hoja
del árbol. Si el punto de acceso es administrable (o hay una controladora tipo
UniFi, que el código heredado ya sabe consultar), se obtiene qué cliente está
asociado a qué antena, que es el equivalente inalámbrico del mapa de puertos.

**La consecuencia de diseño: perfil de capacidades por red.** Cada red descubre y
guarda qué se puede saber en ella, y la interfaz lo dice sin rodeos en vez de
mostrar pantallas vacías:

| | Mapa de puertos | Consumo por puerto | Presencia | Servicios | Alertas |
|---|---|---|---|---|---|
| Switch administrable con SNMP/LLDP | exacto | sí | sí | sí | sí |
| Controladora WiFi (UniFi y similares) | por antena | según equipo | sí | sí | sí |
| Switch no administrable colgado de uno administrable | grupo detrás del puerto | del grupo entero | sí | sí | sí |
| Todo no administrable | no | solo del router, si exporta flujos | sí | sí | sí |

Y para cada red se genera un **informe de qué falta para subir de nivel**: «en
Sucursal 2 no se ve el puerto porque el switch X no es administrable; con un
switch administrable en ese punto se cubrirían estos 9 equipos». Eso, para quien
administra varios sitios, es tanto una herramienta de diagnóstico como un
argumento para justificar una compra.

Con las redes mixtas confirmadas, **la fase 0 deja de ser una puerta de sí o no y
pasa a ser un inventario**: qué sitio tiene qué, para saber qué esperar de cada
uno y en qué orden conviene ir mejorando.

---

## 3. Lo mejor de cada herramienta en una sola

MiRed no se conforma con ser Scanopy sin topes: se queda con lo que cada
herramienta del sector hace mejor. Cuatro preguntas, cuatro capacidades.

| Pregunta | Quién la responde hoy | Qué hay ya en el código heredado |
|---|---|---|
| ¿Cómo está conectada mi red? | Scanopy | **Todo**: topología de capa 2 y 3, mapa visual, 257 servicios reconocidos. Es la base del fork. |
| ¿Qué dispositivo se conectó ahora? | NetAlertX | **La fontanería**: hay envío por webhook, correo y ntfy en el código, pero apagado para la edición gratuita. Falta el motor de reglas. |
| ¿Qué está encendido ya mismo? | WatchYourLAN | **El dato**: la última vez que se vio cada equipo ya se guarda (aparece en 286 puntos del código). Falta el barrido rápido y la pantalla. |
| ¿Quién consume mi ancho de banda? | ntopng | **Nada**. Cero referencias a contadores de tráfico. Es el único subsistema realmente nuevo. |

Dicho de otro modo: tres de las cuatro columnas son trabajo de terminar algo que
ya está a medias, y solo la cuarta se construye desde cero.

### 3.1 Presencia en vivo (lo de WatchYourLAN)

Un barrido ARP corto cada 30-60 segundos, **separado del escaneo profundo**, que
solo responde «está o no está». Es lo que hace a WatchYourLAN tan ligero: no
interroga servicios, solo pregunta quién contesta. Encima de eso, una pantalla de
encendidos ahora, con el histórico de presencia por equipo (a qué hora se conecta
el celular de quién, qué equipo lleva tres días apagado).

El escaneo profundo —puertos, servicios, SNMP, topología— sigue siendo programado
y espaciado, porque es el caro. Son dos ritmos distintos sobre los mismos datos, y
mezclarlos es justo lo que vuelve pesadas a otras herramientas.

### 3.2 Alertas de cambio (lo de NetAlertX)

El valor de NetAlertX no es escanear, es **avisar**. El motor de reglas dispara
sobre eventos que MiRed ya detecta o va a detectar:

- MAC nueva en la red (el aviso estrella: se conectó algo que no conocías).
- Equipo conocido que desaparece más de X tiempo.
- Equipo que cambia de IP, de nombre o de puerto del switch (se movió de lugar).
- Puerto nuevo abierto en un equipo conocido (esto sí es una señal de alarma).
- Servicio que deja de responder.
- Red entera sin reportar (el demonio se cayó).

Salidas: correo, webhook, ntfy y Telegram, que cubren lo que la gente usa de
verdad. Las tres primeras ya tienen código heredado; solo hay que encenderlas y
conectarles el motor. Cada regla se configura **por red**, que encaja con la base
por red: la sucursal avisa a quien lleva la sucursal.

### 3.3 Ancho de banda: la parte que hay que decir clara

**Escanear no mide tráfico.** Para saber quién consume hay que *ver* el tráfico, y
en una red con switches cada equipo solo ve lo suyo. Hay tres formas, de barata a
cara, y MiRed no debe elegir una sino ofrecerlas por capas:

1. **Contadores SNMP por puerto** (`ifHCInOctets`/`ifHCOutOctets` cada X
   segundos). El switch ya lleva la cuenta de los bytes que pasan por cada boca;
   solo hay que preguntársela. Y como MiRed ya sabe qué aparato cuelga de qué
   puerto, eso da **quién consume** sin capturar un solo paquete. Costo en
   recursos: casi nulo. Requiere switches administrables, así que en una red
   mixta cubre la parte administrable y en las bocas donde cuelga un switch
   tonto da el consumo **del grupo entero**, no de cada equipo.
   Da volumen, no aplicaciones.
2. **Flujos exportados por el router** (NetFlow, sFlow, IPFIX): quién habla con
   quién, cuánto y hacia dónde. El trabajo pesado lo hace el router y MiRed solo
   recibe y agrega. Costo medio. **Es la salida para los sitios sin un solo
   switch administrable**: no da el puerto, pero sí quién consume, porque el
   tráfico a internet pasa por el router sí o sí.
3. **Inspección profunda de paquetes** con nDPI (la biblioteca que usa ntopng por
   dentro): identifica la aplicación concreta —vídeo, respaldo en la nube,
   torrent, videollamada—. Exige un puerto espejo en el switch o estar en el
   camino del tráfico, y **es exactamente lo que pone a ntopng en «consumo
   alto»**.

**La decisión**: el punto 1 va en el núcleo, porque encaja con el mapa de puertos
que ya se está construyendo y sale casi gratis. El 2 va en el núcleo también, que
es solo escuchar y sumar. El 3 va en un **paquete aparte** (`mired-dpi`) que
solo instala quien quiera pagar el precio en recursos y pueda montar el espejo.
Así MiRed sigue corriendo en una Raspberry, que es donde la gente deja encendidas
estas cosas.

### 3.4 El consumo de recursos es un requisito, no un resultado

La tabla comparativa pone a WatchYourLAN en «muy bajo» y a ntopng en «alto». Es
una advertencia: MiRed va a hacer más cosas que las dos, y si no se cuida acaba
en «alto» sin haber ganado nada. Regla de diseño, en firme:

- Presencia y contadores SNMP tienen que correr cómodos en una Raspberry Pi.
- El escaneo profundo va programado y espaciado, nunca continuo.
- La inspección profunda es opcional y va en otro paquete.
- Se mide el consumo al cerrar cada fase, no al final del proyecto.

### 3.5 Licencias: se puede combinar, pero no es copiar y pegar

NetAlertX, WatchYourLAN y ntopng son GPL-3.0; nDPI es LGPL-3.0; Scanopy es
AGPL-3.0. GPL-3.0 y AGPL-3.0 son compatibles entre sí —ambas traen la cláusula
que permite combinarlas— y el resultado se distribuye como AGPL-3.0. Así que
legalmente no hay obstáculo.

En la práctica, sin embargo, **no hay código que copiar**: NetAlertX es Python y
PHP, WatchYourLAN es Go, ntopng es C++, y MiRed es Rust. Lo que se reutiliza son
las ideas, y las ideas no llevan licencia. La única dependencia directa real
sería nDPI, que al ser LGPL y una biblioteca en C se puede enlazar desde el
módulo opcional sin arrastrar obligaciones al resto.

---

## 4. Fases

### Fase 0 — Inventario de lo que hay (1 día)

Instalar Scanopy Community tal cual, con Docker, contra una red real —de
preferencia una de las mixtas, que es el caso difícil—. Comprobar qué descubre de
verdad: qué switches contestan SNMP o LLDP, si arma la tabla de puertos, si los
equipos salen identificados, y qué pasa en las bocas donde cuelga un switch no
administrable. Anotar marca y modelo de cada switch y de cada punto de acceso.

Ya no es una puerta de sí o no —se sabe que las redes son mixtas y el plan está
escrito para eso—, sino la medición que ordena el resto: **qué sitio da el mapa
de puertos completo, cuál lo da por grupos y cuál se queda en inventario más
alertas**. De ahí sale el tamaño real de la fase 6 y el contenido del informe de
capacidades de 2.4.

### Fase 1 — Fork, rename y limpieza (3-5 días)

- Repo `tuxormax/mired`, fork limpio, con el `git remote` de Scanopy como
  `upstream` para poder traer sus cambios después.
- Rename de marca: 2 509 apariciones de «scanopy» en 370 archivos. Se hace con
  script, pero hay que revisar a mano las variables de entorno (`SCANOPY_*` →
  `MIRED_*`), el nombre de la base, las rutas de configuración, los nombres de
  los binarios y el emisor de los tokens.
- Quitar los topes: `included_networks` e `included_seats` pasan de `1` a «sin
  límite» en la definición del plan, que es donde el código los declara.
- Arrancar la poda de lo que no aplica a un proyecto sin negocio detrás:
  facturación, Stripe, planes, correos de aviso de cuota, claves de licencia,
  telemetría. Es la mitad del valor de la fase: menos código es menos
  mantenimiento para siempre.
- Enlace al código fuente en el pie de la interfaz (obligación AGPL).

### Fase 2a — Puerto a SQLite (3-5 semanas, la fase crítica)

1. Levantar el esquema final en PostgreSQL y volcarlo.
2. Traducir a `0001_mired_base.sql` para SQLite.
3. Reescribir `pg_value.rs` como `sqlite_value.rs` (conversión de tipos).
4. Adaptar `generic.rs` y `filter/` al dialecto de SQLite.
5. Cambiar la dependencia `sqlx` de `postgres` a `sqlite` con la biblioteca
   embebida, para que el `.deb` no dependa de nada del sistema.
6. Hacer pasar la batería de pruebas del propio Scanopy (`storage/tests.rs` ya
   trae casi 1 000 líneas de pruebas del almacenamiento: es la red de seguridad
   del puerto, y por eso conviene no tocarla más de lo imprescindible).
7. Modo WAL y `busy_timeout`.

Se hace con **una sola base**, sin partir por redes todavía: primero que todo
funcione igual que antes pero sobre SQLite, con las pruebas en verde. Partir en
varios archivos antes de tener esa base firme sería depurar dos cambios a la vez.

Es la fase con más riesgo de que aparezcan sorpresas. Si algo va a descarrilar el
proyecto, va a ser aquí.

### Fase 2b — Una base por red (2-4 semanas)

Con SQLite ya funcionando, se parte:

1. Clasificar las 29 tablas en *globales* y *de red*, y resolver una por una las
   tablas puente (etiquetas, credenciales, permisos).
2. Separar el esquema en dos: el del catálogo y el de una red.
3. Enrutador de conexiones en `factory.rs`: cada entidad sabe si es global o de
   red; las de red resuelven su archivo según la red activa del contexto.
4. La red activa se fija en el middleware que ya resuelve permisos por red, y en
   los trabajos de fondo (escaneos) que ya saben sobre qué red operan.
5. Alta de red = crear archivo + aplicar esquema + registrar en el catálogo.
   Baja = archivar o borrar el archivo. Migración de esquema al abrir archivos
   viejos o restaurados.
6. Juego de conexiones con cierre de las que no se usan, y resumen por red en el
   catálogo para el panel general.
7. Recorrido por las pantallas que cruzan redes (búsqueda global, panel de
   inicio, informes) para que hagan recorrido o `ATTACH` en vez de un `SELECT`.
8. Prueba de carga de verdad: veinte redes escaneando a la vez, midiendo tiempos
   y archivos abiertos.

### Fase 3 — El `.deb` (1 semana)

Un solo paquete `mired` que deje el sistema listo:

- `/usr/bin/mired-servidor` y `/usr/bin/mired-daemon` (binarios Rust).
- `/usr/share/mired/web/` (interfaz Svelte ya compilada).
- `/usr/share/mired/dispositivos/` (catálogo base).
- `/var/lib/mired/mired.db` (catálogo, creado en el `postinst`) y
  `/var/lib/mired/redes/` (una base por red, creadas al vuelo). La carpeta de
  redes es lo único que hay que respaldar.
- `/etc/mired/mired.toml` (configuración).
- Dos unidades systemd: servidor y demonio. El demonio necesita privilegios de
  red para ARP y sondeos crudos; se le dan capacidades acotadas
  (`CAP_NET_RAW`, `CAP_NET_ADMIN`) en vez de correrlo como root entero.
- Usuario de sistema `mired` propio, no root, para el servidor.
- El `postinst` crea la base, habilita los servicios y no falla nunca aunque algo
  quede pendiente — misma regla que el driver de tickets.
- Compilar para `amd64` y `arm64` (que la Raspberry Pi es el equipo natural para
  dejar esto encendido todo el día).

### Fase 4 — Catálogo abierto de dispositivos (2 semanas)

Lo descrito en 2.3: formato `.toml`, carga en caliente desde disco, conversión
automática de los 257 existentes, botón «proponer definición» en la interfaz y
validación automática de las aportaciones en el repo.

### Fase 5 — Presencia en vivo y alertas (2-3 semanas)

Lo de 3.1 y 3.2: barrido rápido de presencia con su propio ritmo, pantalla de
«encendidos ahora», histórico de presencia, motor de reglas por red y salidas por
correo, webhook, ntfy y Telegram. Encender de paso las notificaciones que el
código heredado ya trae apagadas por ser de pago.

### Fase 6 — Ancho de banda sin capturar paquetes (2-3 semanas)

Sondeo de contadores SNMP por puerto y su gráfica por equipo, apoyada en el mapa
de puertos; receptor de flujos NetFlow/sFlow del router, que es lo que cubre los
sitios sin switches administrables; y los avisos que salen de ahí (un equipo que
se dispara, un enlace saturado). Todo dentro del presupuesto de recursos de 3.4.

Aquí entra también el **perfil de capacidades por red** de 2.4: detectar por
sitio qué se puede medir, marcar en el mapa lo confirmado frente a lo inferido, y
generar el informe de qué haría falta para subir de nivel en cada sitio.

### Fase 7 — Inspección profunda, opcional (2-3 semanas, se puede posponer)

Paquete aparte `mired-dpi` con nDPI, para quien tenga puerto espejo y quiera
saber qué aplicación consume, no solo cuánto. **No bloquea la primera versión**:
si el tiempo aprieta, esta es la que se corta.

### Fase 8 — Publicación y mantenimiento (continuo)

- Repositorio APT propio para que `apt update` traiga las actualizaciones, o
  publicación de los `.deb` en las releases de GitHub.
- Documentación de instalación en español y en inglés.
- Rutina de sincronización con Scanopy: revisar sus releases, traer los cambios
  útiles, portar sus migraciones nuevas al esquema de SQLite.

---

## 5. Costos y riesgos, sin adornos

**Esfuerzo total estimado: entre cuatro y cinco meses** de trabajo sostenido para
la primera versión instalable con presencia, alertas y ancho de banda por SNMP,
siendo las fases 2a y 2b más de la mitad. La inspección profunda (fase 7) va
aparte y se puede posponer sin bloquear nada.

Hay una versión útil antes: **al terminar la fase 3** ya existe un `.deb`
instalable con mapa de red, redes ilimitadas y base por red. Conviene sacarla y
usarla en producción propia mientras se construye el resto, en vez de esperar
cinco meses a estrenar.

Los riesgos reales, en orden:

1. **Divergencia con el proyecto original.** Es el riesgo que mata a los forks, y
   la base por red lo agrava: no es un cambio de motor sino un cambio de
   suposición, y arriba van a seguir escribiendo código que da por hecho que todo
   está en una sola base. Scanopy lleva unos 5 800 commits y desarrollo activo;
   cada versión suya que no se integre aumenta la distancia hasta que traer
   cambios se vuelve imposible. Mitigación: el enrutador concentrado en la capa
   de almacenamiento y en ningún otro lado, poda al mínimo indispensable, nada de
   reformatear ni reorganizar, y sincronizar con cada release en vez de cada seis
   meses.
2. **Las consultas que cruzan redes.** El panel general, la búsqueda global y los
   informes dejan de ser una consulta y pasan a ser un recorrido por archivos.
   Hay que encontrarlas todas en la fase 2b; la que se escape aparece después
   como una pantalla vacía o lentísima.
3. **Que los switches no sean administrables.** Cubierto por la fase 0.
4. **Querer las cuatro herramientas a la vez.** Juntar lo mejor de cuatro
   proyectos es la parte ilusionante y la que más fácil descarrila un fork de una
   persona. El orden de las fases está puesto a propósito: primero que exista un
   `.deb` que funcione (fase 3), después las capacidades nuevas de una en una, y
   la inspección profunda al final porque es la única que se puede cortar sin que
   duela.
5. **Un fork mantenido por una persona.** El catálogo abierto es precisamente la
   apuesta contra esto: es la parte donde alguien puede aportar sin saber Rust, y
   por lo tanto la que puede traer gente.

*(El riesgo de contención de escritura de SQLite, que era el segundo de la
versión anterior de este plan, lo elimina justamente la base por red.)*

## 6. Nota sobre las reglas del repo

Este repositorio nombra archivos, variables y funciones en español. MiRed no
puede cumplirlo por dentro: son 159 000 líneas heredadas en inglés y traducir
identificadores rompería para siempre la posibilidad de integrar los cambios de
arriba. La regla se aplica donde sí manda MiRed: el empaquetado, los scripts, la
configuración, el formato del catálogo y toda la documentación. El código
heredado se queda en inglés a propósito, y esta es la razón.
