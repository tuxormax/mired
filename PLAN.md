# MiRed — plan del proyecto

Servicio propio para descubrir los equipos de una red, dibujar su mapa y
documentar en qué puerto de qué switch está conectado cada aparato. Sin topes de
redes ni de usuarios, con SQLite y **una base de datos independiente por cada
red**, empaquetado en un `.deb` que se instala solo, y con un catálogo de
dispositivos que la comunidad puede ampliar sin recompilar nada.

**MiRed es código propio, no un fork.** El plan anterior partía de forkear
Scanopy; se descartó el 2026-08-12. Scanopy se conserva como **material de
consulta**: se lee para entender cómo resolvieron la topología de capa 2, el
reconocimiento de dispositivos o el manejo de LLDP, pero no se copia una sola
línea. Las ideas no llevan licencia; las líneas sí, y copiarlas obligaría a MiRed
a heredar la AGPL de otro.

---

## Estado del desarrollo

Actualizado el **2026-08-12**, al cerrar la primera jornada de trabajo.

| Fase | Estado | Nota |
|---|---|---|
| 1 — Cimientos | ✅ terminada | Servidor, sonda, base por red, auth, API, interfaz y `.deb` |
| 2 — Descubrimiento | ✅ terminada | ARP, ICMP y TCP; puertos, DNS inverso y fabricante por OUI |
| 3 — Presencia en vivo | ✅ terminada | Barrido rapido, historial de conexiones y agenda por red |
| 4 — SNMP y capa 2 | ⚠️ hecha, sin probar en equipo real | LLDP, CDP, enlaces entre switches dibujados y controladora UniFi. **Falta probarla contra equipo de verdad** |
| 5 — El mapa | ✅ terminada | Mapa visual y exportacion a PNG, SVG, PDF y CSV |
| 6 — Catalogo `.toml` | ✅ terminada | Formato, motor de reconocimiento, 15 definiciones semilla y "proponer definicion" |
| 7 — Alertas | ✅ terminada | Las 6 reglas detectan y los 4 destinos de aviso funcionan |
| 8 — Ancho de banda | ✅ terminada | Contadores SNMP por puerto y receptor de flujos: NetFlow v5, NetFlow v9, IPFIX y sFlow |
| 9 — Publicacion | ⚠️ parcial | Licencia decidida (AGPL-3.0) y documentacion lista (ES y EN). **Falta firmar y publicar los paquetes** |
| 10 — Inspeccion profunda | ✅ terminada | `mired-dpi` en el mismo paquete pero APAGADO: nombre del servidor por TLS, HTTP y DNS. Falta probarlo con un puerto espejo real |

**Probado de verdad:** el descubrimiento contra la red real de casa, los barridos
programados corriendo solos, el motor de alertas, el catalogo de dispositivos, el
receptor de flujos, y el `.deb` desempaquetado y corriendo desde su propio arbol.

**NO probado todavia:** instalar el `.deb` con `dpkg -i` en un equipo; SNMP y CDP
contra un switch administrable real; la controladora UniFi contra una de verdad
(solo contra un servidor de mentira que imita las dos generaciones); y la
inspeccion profunda contra un puerto espejo real. El switch administrable sigue
siendo el riesgo abierto mas grande del proyecto.

### Consumo de recursos medido (2026-08-12)

El plan lo pone como requisito, no como resultado, asi que se mide al cerrar cada
fase. Medido sobre el `.deb` de `amd64`, escaneando un `/24` completo:

| | En reposo | Pico durante un escaneo completo |
|---|---|---|
| `mired-servidor` | 12.2 MB | 14.7 MB |
| `mired-sonda` | 5.1 MB | 15.8 MB |

El escaneo completo de un `/24` (254 direcciones por 38 puertos cada una) tardo
**12 segundos**. Las bases de datos de una red con equipos descubiertos pesan
menos de 1 MB.

**Menos de 31 MB entre los dos procesos en el pico.** Cabe de sobra en una
Raspberry Pi, que era el requisito.

### Cobertura de pruebas

**98 pruebas en Go y 14 en Flutter**, sobre ~15 800 lineas de Go y ~6 400 de Dart.

En Go: almacenamiento, agenda, alertas, topologia, catalogo, la lectura de SNMP,
los cuatro formatos de flujos, la controladora UniFi contra un servidor de
mentira, y la identificacion de aplicaciones de la inspeccion profunda. Lo que
mas se cuida es lo que **falla sin dar error**: leer un registro con la plantilla
de otro router, colgar un vecino de la boca equivocada, o multiplicar mal la tasa
de muestreo de sFlow. Ninguna de esas tres revienta nada; todas dan cifras
plausibles y falsas.

En Flutter: 5 dibujan cada pantalla contra un servidor de mentira y 9 comprueban
la exportacion del mapa —que el PDF tenga bien la tabla de referencias cruzadas y
escape acentos y parentesis, que el SVG salga bien formado, que el PNG lleve su
firma, que el CSV entrecomille lo que debe y que un cable visto por LLDP y por
CDP se dibuje una sola vez—.

Mas la prueba de humo `herramientas/probar.sh`, con **23 comprobaciones**: cons-
truye los dos `.deb`, los desempaqueta y recorre el flujo completo. Es la unica
que comprueba lo que de verdad se entrega.

**Un aviso sobre las pruebas:** `go test ./...` falla de vez en cuando en el
paquete `basedatos` con *context deadline exceeded* al abrir una base. Es del
entorno, no del codigo: con 6 paquetes de pruebas corriendo en paralelo, abrir
una base con SQLite en Go puro puede pasarse de los 10 s de espera. **Con
`go test ./... -p 1` no ha fallado nunca.**

---

## 1. Decisiones de fondo

### 1.1 Go para el servicio

- **Un binario estático, cero dependencias.** El `.deb` no pide runtime, ni
  servidor de base de datos, ni bibliotecas del sistema. Se instala y corre.
- **Cruza a `arm64` con una variable de entorno.** La Raspberry Pi es el equipo
  natural para dejar esto encendido todo el día, y en Go compilar para ella no
  exige montar un toolchain cruzado.
- **Las bibliotecas de red existen en Go puro**: ARP crudo, sockets de bajo
  nivel, SNMP v1/v2c/v3, mDNS y el driver de SQLite sin cgo. Eso es lo que
  permite que el binario sea de verdad estático.
- **Referencia de consumo**: WatchYourLAN, la herramienta más liviana de la
  categoría, está escrita en Go. El presupuesto de recursos (§4) es alcanzable.

Rust habría dado algo menos de memoria a cambio de un lenguaje que hoy no se
maneja en casa y que habría que mantener durante años. No compensa.

### 1.2 Flutter para la interfaz: un programa de escritorio

**MiRed es un programa, no una página.** Se instala, aparece en el menú de
aplicaciones y se abre como cualquier otro. No hay interfaz web y no se compila
ninguna (decidido el 2026-08-13, después de haberla tenido).

- **El programa manda sobre los servicios**: los levanta al abrirse y los mata al
  cerrarse. No hay nada que habilitar ni que dejar corriendo.
- **Sus datos van a `~/.local/share/mired`**, no a `/var/lib`: los servicios
  corren como el usuario, y no tendría por qué poder escribir en una carpeta del
  sistema.
- El servidor **solo expone la API**. Sigue hablando HTTP porque es la forma más
  simple de que el programa y los servicios se entiendan — y porque así el
  programa puede ver también el MiRed de otro equipo, que es lo que sirve para
  administrar varios sitios desde uno.

**Lo que cuesta esa simplicidad, dicho claro:** con el programa cerrado, MiRed no
vigila nada — ni escaneos programados, ni alertas de equipo desconocido, ni
recepción de los flujos del router. Para un equipo que tiene que vigilar de
continuo sin nadie delante (una Raspberry en una sucursal), el paquete trae las
unidades de systemd **instaladas pero apagadas**; encenderlas es una orden. Las
dos formas no se mezclan: el servicio guarda en `/var/lib/mired` y el programa en
la carpeta de cada usuario.

Razones de fondo del stack:

- Es el entorno que ya se maneja en casa (MiAuto, la app de SCOLELTE, el POS).
- **El mismo código da después la app de Android** sin reescribir la interfaz:
  se cambia el objetivo de compilación y apunta a la misma API. Eso es gratis
  aquí y sería un proyecto entero en cualquier otro stack.
- El mapa de red es un grafo dibujado sobre lienzo, que es justo lo que Flutter
  hace bien y lo que en HTML obligaría a meter una biblioteca de terceros.

### 1.3 Dos procesos, no uno

- **`mired-servidor`**: corre como usuario de sistema `mired`, **sin privilegios**.
  Sirve la interfaz y la API, y **es el único que escribe en las bases de datos**.
- **`mired-sonda`**: es la que necesita red cruda (ARP, sondeos, escucha de LLDP).
  Recibe capacidades acotadas (`CAP_NET_RAW`, `CAP_NET_ADMIN`), **nunca root
  entero**, y **no toca la base de datos**: escanea y entrega el resultado al
  servidor por un socket Unix local.

Separar privilegios desde el primer día es barato; retroajustarlo después de
tener veinte módulos escribiendo a la base es carísimo. Y como solo hay un
escritor, desaparecen de raíz los problemas de concurrencia de SQLite.

### 1.4 Una base de datos por red

Al crear una red (Matriz, Sucursal 1, Bodega) se crea **su propio archivo
SQLite** con todo lo operativo de esa red. Ni una tabla compartida con las demás.

- **Catálogo** (`/var/lib/mired/mired.db`, uno solo): usuarios, sesiones, claves
  de API, permisos de qué usuario ve qué red, credenciales SNMP reutilizables y
  el registro de redes con un **resumen por red**, para que la pantalla de inicio
  no tenga que abrir treinta archivos.
- **Red** (`/var/lib/mired/redes/matriz-a1b2.db`, una por red): equipos, subredes,
  servicios, puertos, interfaces, VLAN, topologías, histórico de presencia,
  reglas de alerta y contadores de tráfico. Cada archivo lleva dentro su propia
  ficha de red, así que es **autodescriptivo**: se copia a otra instalación y se
  sabe qué es.

Lo que se gana: respaldar un sitio es copiar un archivo; borrar una red es
borrarlo; una base corrupta se lleva una red y no el sistema; y el límite de «una
escritura a la vez» de SQLite, que es por archivo, deja de importar.

Lo que hay que vigilar, y no se puede olvidar en ningún momento:

- Las consultas que **cruzan redes** (buscar una MAC en todos los sitios, el
  panel general) no son un `SELECT`: son recorrido de archivos o `ATTACH`. Por
  eso el resumen por red vive en el catálogo.
- Las migraciones se aplican **a cada archivo**, incluso a uno viejo restaurado
  de un respaldo. Al abrir una base se comprueba su versión antes de tocarla.
- No se pueden tener cien bases abiertas: juego de conexiones que cierra las que
  llevan rato sin usarse.
- **El enrutado de conexiones vive en un solo lugar** (`internal/basedatos`) y en
  ningún otro. Si se reparte por los módulos, cada función nueva tendrá que
  acordarse de a qué archivo escribe, y tarde o temprano una no se acordará.

Como el nombre bonito de la red cambia y puede repetirse, el archivo se llama
`nombre-corto-<id>.db`; el nombre real vive dentro.

### 1.5 El catálogo de dispositivos, en `.toml` desde el primer día

Reconocer «esto es una impresora HP, esto es una cámara Hikvision, esto es un
Chromecast» es lo que convierte una lista de IP en un inventario. Ese
reconocimiento **no se escribe en código**: se escribe en archivos `.toml` que se
leen al arrancar.

- Un archivo por dispositivo: nombre, categoría, icono, puertos característicos,
  prefijo MAC del fabricante, ruta HTTP a consultar y qué se espera de vuelta,
  cadena SNMP esperada.
- Se cargan desde `/usr/share/mired/dispositivos/` (los que trae el paquete) y
  `/etc/mired/dispositivos/` (los del usuario, que mandan sobre los anteriores).
  Agregar un dispositivo es copiar un archivo y reiniciar el servicio.
- En la interfaz, todo equipo que quede sin identificar ofrece **«proponer
  definición»**, que genera el `.toml` ya relleno con lo que se vio: puertos
  abiertos, fabricante, banners. El usuario le pone nombre y lo manda al repo.
- El repositorio valida cada aportación automáticamente: que el `.toml` sea
  válido, que no duplique otro y que el patrón no sea tan amplio que etiquete
  medio mundo.

Esta es la parte donde alguien puede aportar sin saber Go, y por lo tanto la que
puede traer gente a un proyecto que de otro modo mantiene una sola persona.

### 1.6 Redes mixtas: se diseña para ellas, no se parchean después

«Qué puerto está conectado a qué» solo se sabe si el switch es administrable y
contesta SNMP, LLDP o CDP. Con uno no administrable no hay a quién preguntarle, y
ninguna herramienta del mundo puede inventarlo. Pero la red no queda a oscuras:
**degrada por capas**, y el trabajo de MiRed es que la degradación sea visible y
explicada en vez de un mapa con huecos.

**Sin un solo switch administrable se sigue sabiendo:** quién está en la red y
quién se acaba de conectar, qué es cada aparato, qué servicios y puertos expone,
la topología de capa 3 (subredes, salidas, rutas) y **todas las alertas de
cambio**. Se pierde el puerto exacto y el consumo por puerto.

**Lo que sí se infiere:** si un switch administrable ve **varias MAC colgando de
una misma boca**, ahí hay un switch no administrable o un punto de acceso, y todo
lo que aparece por esa boca está detrás de él. No da el puerto exacto, da el
grupo: «estos nueve equipos cuelgan del puerto 7 a través de un switch tonto».
Para un plano de sitio eso suele bastar. Y el WiFi nunca tiene puerto de switch:
ahí la hoja del árbol es el punto de acceso.

| Escenario | Mapa de puertos | Consumo por puerto | Presencia | Servicios | Alertas |
|---|---|---|---|---|---|
| Switch administrable con SNMP/LLDP | exacto | sí | sí | sí | sí |
| Controladora WiFi (UniFi y similares) | por antena | según equipo | sí | sí | sí |
| Switch tonto colgado de uno administrable | grupo tras el puerto | del grupo entero | sí | sí | sí |
| Todo no administrable | no | solo del router, si exporta flujos | sí | sí | sí |

De ahí sale una obligación de diseño: cada red guarda **su perfil de
capacidades** —qué se puede saber en ella— y la interfaz lo dice sin rodeos. Más
un informe por sitio de qué haría falta para subir de nivel: «en Sucursal 2 no se
ve el puerto porque el switch X no es administrable; con uno administrable ahí se
cubrirían estos 9 equipos». Para quien administra varios sitios eso sirve igual
de diagnóstico que de argumento de compra.

### 1.7 Las cuatro capacidades

MiRed responde cuatro preguntas que hoy exigen cuatro herramientas distintas:

| Pregunta | Hoy la responde | En MiRed |
|---|---|---|
| ¿Cómo está conectada mi red? | Scanopy | Fases 4 y 5 |
| ¿Qué está encendido ya mismo? | WatchYourLAN | Fase 3 |
| ¿Qué dispositivo se acaba de conectar? | NetAlertX | Fase 7 |
| ¿Quién consume mi ancho de banda? | ntopng | Fase 8 |

**Escanear no mide tráfico.** Para saber quién consume hay que *ver* el tráfico, y
en una red con switches cada equipo solo ve lo suyo. Hay tres formas, de barata a
cara, y MiRed ofrece las dos primeras en el núcleo:

1. **Contadores SNMP por puerto** (`ifHCInOctets`/`ifHCOutOctets`): el switch ya
   lleva la cuenta de los bytes de cada boca, solo hay que preguntársela. Como
   MiRed ya sabe qué aparato cuelga de cada boca, eso da **quién consume sin
   capturar un solo paquete**, a costo casi nulo. Da volumen, no aplicaciones.
2. **Flujos exportados por el router** (NetFlow, sFlow, IPFIX): quién habla con
   quién y cuánto. El trabajo pesado lo hace el router. **Es la salida para los
   sitios sin ningún switch administrable**, porque el tráfico a internet pasa
   por el router sí o sí.
3. **Inspección profunda de paquetes**: identifica la aplicación concreta —vídeo,
   respaldo, torrent, videollamada—. Exige puerto espejo y es exactamente lo que
   pone a ntopng en «consumo alto». Es **opcional** y se puede posponer sin
   bloquear nada. **Hecho** (fase 10): va en el mismo `.deb` pero con el servicio
   **apagado** hasta que se configure la interfaz del puerto espejo, así que
   quien no lo use no paga nada por él.

---

## 2. Cómo está organizado el repositorio

Todo se nombra en español, como el resto de los proyectos de la casa. La única
excepción es la carpeta `internal/`, cuyo nombre el compilador de Go interpreta
literalmente para impedir que otros proyectos importen esos paquetes; adentro,
todo vuelve a estar en español.

| Carpeta | Qué contiene |
|---|---|
| `programas/mired-servidor/` | Binario del servidor: API, interfaz, único escritor de las bases |
| `programas/mired-sonda/` | Binario de la sonda: escaneo con privilegios acotados |
| `programas/mired-dpi/` | Binario de la inspección profunda, opcional y apagado por omisión |
| `internal/basedatos/` | Catálogo, base por red, migraciones y el enrutado de conexiones |
| `internal/configuracion/` | Lectura de `/etc/mired/mired.toml` y valores por omisión |
| `internal/autenticacion/` | Usuarios, sesiones, claves de API y permisos por red |
| `internal/api/` | Rutas HTTP y su contrato con la interfaz |
| `internal/escaneo/` | ARP, ICMP, puertos TCP, mDNS, resolución de nombres |
| `internal/snmp/` | SNMP v1/v2c/v3, tabla de MAC del switch, interfaces, VLAN |
| `internal/topologia/` | Armado del mapa de capa 2 y capa 3, confirmado contra inferido |
| `internal/catalogo/` | Motor de reconocimiento y carga de los `.toml` |
| `internal/alertas/` | Motor de reglas y salidas (correo, webhook, ntfy, Telegram) |
| `internal/trafico/` | Contadores SNMP y receptor de flujos del router |
| `internal/controladora/` | Controladoras WiFi (UniFi): qué cuelga de qué antena |
| `internal/dpi/` | Inspección profunda: captura e identificación de aplicación |
| `interfaz/` | Proyecto Flutter |
| `catalogo/dispositivos/` | Los `.toml` de dispositivos que trae el paquete |
| `empaquetado/` | Todo lo del `.deb`: control, `postinst`, unidades systemd |
| `herramientas/` | Scripts auxiliares en Python (generadores, laboratorio, validación) |
| `documentacion/` | Manual de instalación y de uso, en español y en inglés |

---

## 3. Fases

Cada fase cierra con algo que se puede instalar y usar. El orden está puesto para
que haya producto útil en la primera semana y siga creciendo, no para que todo
llegue junto al final.

### Fase 1 — Cimientos (1-2 semanas)

Esqueleto del proyecto en Go, base de datos y una interfaz que ya entra.

- Módulo Go, estructura de carpetas, licencia y `README`.
- Capa de base de datos: catálogo, base por red, motor de migraciones propio
  (numeración desde `0001`) y el enrutado de conexiones concentrado en un lugar.
- Alta y baja de redes: crear archivo, aplicar esquema, registrar en el catálogo.
- Autenticación: usuarios, sesiones, permisos por red y **creación del
  administrador en el primer acceso**. MiRed no trae ningún usuario ni clave de
  fábrica: unas credenciales iguales en todas las instalaciones son, en un
  proyecto público, una puerta que cualquiera puede buscar. Las claves se guardan
  con el algoritmo **TUXOR** de la casa sobre scrypt.
- API REST con el contrato base y el manejo de errores de la casa.
- Programa de escritorio en Flutter: entra con usuario y contraseña y lista las
  redes.
- `.deb` mínimo que instala, crea el usuario de sistema, levanta los dos
  servicios y no falla nunca en el `postinst`.

**Entregable: se instala, se entra y se pueden crear redes.**

### Fase 2 — Descubrimiento de equipos (2-3 semanas)

- Barrido ARP de la subred y sondeo ICMP para lo que no contesta ARP.
- Escaneo de puertos TCP con concurrencia acotada y tiempos de espera cuerdos.
- Identificación: DNS inverso, mDNS, NetBIOS y fabricante por prefijo MAC.
- Modelo de equipo, subred, puerto y servicio en la base de la red.
- Escaneos programados por red, con su histórico.
- Pantalla de equipos con búsqueda, filtros y ficha de cada aparato.

**Entregable: ya sirve como inventario de red, que es el 80 % del uso diario.**

### Fase 3 — Presencia en vivo (1 semana)

- Barrido rápido cada 30-60 segundos que **solo responde «está o no está»**,
  con su propio ritmo, separado del escaneo profundo. Ese es el truco por el que
  WatchYourLAN es tan liviano, y mezclar los dos ritmos es justo lo que vuelve
  pesadas a otras herramientas.
- Pantalla de «encendidos ahora» e histórico de presencia por equipo.

### Fase 4 — SNMP y topología de capa 2 (3-4 semanas)

Es la fase que da lo que ninguna herramienta gratuita da bien, y la más técnica.

- Cliente SNMP v1, v2c y v3, con credenciales guardadas en el catálogo.
- Tabla de reenvío de MAC del switch, tabla de interfaces, VLAN y estado de cada
  puerto.
- LLDP y CDP para los enlaces entre switches. Los dos, porque **CDP viene
  encendido de fabrica en los Cisco y LLDP no**: cada uno cubre la mitad del
  parque instalado. Un mismo cable visto por los dos se guarda dos veces y se
  dibuja una, marcado como confirmado por ambos.
- **Controladora WiFi.** Un punto de acceso no tiene puertos, tiene antenas, y
  quien sabe quien cuelga de cual es la controladora. Lo que contesta se guarda
  por el MISMO camino que SNMP —el punto de acceso es, para el mapa, un switch
  cuyas bocas son redes WiFi—, y asi hereda gratis el mapa, la exportacion y la
  alerta de "se movio de lugar" en vez de ser un segundo mapa que mantener
  aparte. Corre en el servidor y no en la sonda: es HTTPS, no necesita
  privilegios.
- Armado del árbol: qué aparato cuelga de qué boca, distinguiendo **enlace
  confirmado** de **inferido**, y detectando las bocas con varias MAC como
  «grupo detrás de un switch no administrable».
- Perfil de capacidades por red y el informe de qué falta para subir de nivel.

### Fase 5 — El mapa (2 semanas)

- Grafo interactivo en Flutter: capa 2 (quién cuelga de qué puerto) y capa 3
  (subredes y rutas), con lo confirmado y lo inferido distinguidos a la vista.
- Agrupar, filtrar, buscar y fijar posiciones.
- Exportar a PNG, SVG, PDF y CSV.

**El archivo exportado se queda en el equipo.** No hay subida a Google Drive, ni
envío por correo, ni integración con ningún servicio de nadie: el navegador arma
el archivo y lo baja a la carpeta de descargas. El PNG lo dibuja Flutter; el SVG
y el PDF se escriben a mano en la propia interfaz, sin biblioteca de terceros,
así que ni siquiera pasan por el servidor. Quien quiera mandarlo a alguien lo
adjunta él, como cualquier otro archivo suyo.

### Fase 6 — Catálogo abierto de dispositivos (1-2 semanas)

Lo descrito en 1.5: formato `.toml`, motor de reconocimiento, catálogo semilla
con lo más común, botón «proponer definición» en la interfaz y validación
automática de las aportaciones en el repositorio.

### Fase 7 — Alertas (2 semanas)

Motor de reglas configurable **por red**, que encaja con la base por red: la
sucursal avisa a quien lleva la sucursal.

- MAC nueva en la red (el aviso estrella: se conectó algo que no conocías).
- Equipo conocido que desaparece más de X tiempo.
- Equipo que cambia de IP, de nombre o de puerto del switch.
- Puerto nuevo abierto en un equipo conocido.
- Servicio que deja de responder.
- Red entera sin reportar, porque la sonda se cayó.

Salidas: correo, webhook, ntfy y Telegram.

### Fase 8 — Ancho de banda (2-3 semanas)

- Sondeo de contadores SNMP por puerto y su gráfica por equipo, apoyada en el
  mapa de puertos de la fase 4.
- Receptor de flujos NetFlow v5, NetFlow v9, IPFIX y sFlow, que es lo que cubre
  los sitios sin switches administrables. El formato se reconoce solo, sin
  configurarlo.
- Alertas que salen de ahí: un equipo que se dispara, un enlace saturado.

**Los tres primeros formatos cuentan el tráfico; sFlow lo estima**, porque
muestrea uno de cada N paquetes y multiplica. Esa diferencia viaja con el dato
—columna `estimado` en la base— hasta la pantalla, donde la cifra sale con una
tilde delante. Presentar una estimación como si fuera una medición es la misma
mentira silenciosa que el mapa evita cuando distingue el puerto confirmado del
grupo inferido.

### Fase 9 — Empaquetado y publicación (1 semana)

- `.deb` para `amd64` y `arm64`, firmado.
- Publicación en las releases de GitHub y, si crece, repositorio APT propio.
- Manual de instalación y de uso en español e inglés.
- Guía de cómo aportar un dispositivo al catálogo, que es la puerta de entrada
  para quien no programa.

### Fase 10 — Inspección profunda, opcional (2-3 semanas)

`mired-dpi`, para quien tenga puerto espejo y quiera saber qué aplicación
consume, no solo cuánto. **No bloquea nada**: si el tiempo aprieta, esta es la
que se corta.

**Cómo quedó.** Sin nDPI y sin libpcap: la captura es AF_PACKET en Go puro, y la
identificación **no descifra nada**. Se leen las tres cosas que viajan en claro
aunque la conexión vaya cifrada —el nombre del servidor del saludo de TLS, la
cabecera `Host` de HTTP y las consultas de DNS—, que es justo lo que dice con
quién se está hablando. Meter nDPI habría obligado a compilar con cgo, y ahí se
acaban los binarios estáticos y el `.deb` que se instala sin dependencias.

Dos decisiones que sostienen el resto:

- **Se entrega en el mismo `.deb` pero con el servicio apagado** (decidido el
  2026-08-13, tras haberlo llevado un rato en un paquete aparte). Lo que costaba
  caro nunca fue el binario de 1.3 MB: era el **proceso capturando**, que es el
  único de MiRed que trabaja de continuo. Ese coste se controla mejor donde de
  verdad está —en el arranque del servicio— que partiendo la entrega en dos. El
  `postinst` lo enciende solo si encuentra una interfaz configurada, que es la
  señal de que alguien lo quiere.
- **Reparte los papeles igual que la sonda**: `mired-dpi` tiene `CAP_NET_RAW` y
  `CAP_NET_ADMIN` y **no toca la base de datos**; el servidor le pregunta cada
  pocos minutos por su socket Unix y escribe él. Repetir el patrón en vez de
  inventar otro significa que quien entienda cómo habla la sonda ya entiende esto.
- **Lo que no se identifica sigue sumando bytes**, marcado como «sin
  identificar». Tirarlo haría que el informe sumara menos que el consumo real y
  nadie sabría por qué. Y de cada nombre se conserva **de dónde salió**: uno del
  saludo de TLS no vale lo mismo que una suposición por número de puerto, y la
  pantalla lo dice.

Documentación propia, con la parte de privacidad sin rodeos, en
`documentacion/inspeccion-profunda.md`.

---

## 4. El consumo de recursos es un requisito, no un resultado

MiRed va a hacer más que WatchYourLAN y que ntopng juntas. Si no se cuida, acaba
en «consumo alto» sin haber ganado nada. En firme:

- Presencia y contadores SNMP tienen que correr cómodos en una Raspberry Pi.
- El escaneo profundo va programado y espaciado, **nunca continuo**.
- La inspección profunda es opcional y va en otro paquete.
- **Se mide el consumo al cerrar cada fase**, no al final del proyecto, y el
  número queda anotado. Una fase que empeore el consumo sin justificarlo se
  revisa antes de seguir.

---

## 5. Costos y riesgos, sin adornos

**Esfuerzo total estimado: entre tres y cuatro meses** de trabajo sostenido hasta
la fase 9. La inspección profunda va aparte.

Pero hay producto mucho antes: **al cerrar la fase 2 ya existe un `.deb`
instalable que descubre e inventaría la red**, y ese solo ya reemplaza a varias
herramientas. Conviene usarlo en producción propia desde ahí, en vez de esperar
meses para estrenar.

Los riesgos reales, en orden:

1. **La fase 4 es la difícil.** SNMP en la vida real es un desorden: cada
   fabricante contesta a su manera, hay MIB que no implementan, credenciales v3
   que fallan en silencio y switches que devuelven la tabla de MAC incompleta.
   Es donde se va a ir el tiempo que se salga de presupuesto. Mitigación: probar
   contra equipo real desde el primer día de la fase, no al final.
2. **Querer las cuatro capacidades a la vez.** Juntar lo mejor de cuatro
   herramientas es la parte ilusionante y la que más fácil descarrila un
   proyecto de una persona. El orden de las fases está puesto a propósito:
   primero inventario, luego mapa, luego lo demás de una en una.
3. **Que las consultas que cruzan redes se escapen.** Cada pantalla que junte
   datos de varias redes hay que resolverla a mano. La que se escape aparece
   después como una pantalla vacía o lentísima.
4. **Un proyecto mantenido por una persona.** El catálogo abierto de
   dispositivos es la apuesta contra esto: es la parte donde alguien puede
   aportar sin saber Go, y por lo tanto la que puede traer gente.

---

## 6. Lo que queda por decidir

- ~~**La licencia de MiRed.**~~ **Decidido el 2026-08-13: AGPL-3.0.** El código
  es propio, así que no se heredaba ninguna. Se eligió la Affero y no la GPL
  corriente porque MiRed se usa desde el navegador: con la GPL, quien lo monte
  como servicio de pago no entrega el binario a nadie y por lo tanto no está
  obligado a publicar nada. La AGPL cierra ese hueco. El pie de la interfaz lleva
  el enlace al código, como pide la sección 13.
- **Inventario de las redes reales**: qué switches y puntos de acceso hay en cada
  sitio, marca y modelo, y cuáles son administrables. No bloquea el desarrollo
  hasta la fase 4, pero conviene tenerlo antes de llegar ahí para probar contra
  equipo de verdad.
