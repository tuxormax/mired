# Memoria - MiRed (servicio propio de mapeo de redes)

Proyecto en `/home/tuxor/www/mired`. Descubre los equipos de una red, dibuja el
mapa y dice en que puerto de que switch esta cada aparato.
En **v1.20 Rev 42** (2026-08-17).
**PROGRAMA DE ESCRITORIO en Flutter + tres binarios en Go + SQLite, una base por
red, entregado en un `.deb`.** No hay interfaz web: el programa arranca sus
servicios al abrirse y los mata al cerrarse.
**Codigo propio, NO un fork**: Scanopy solo se consulta, nunca se copia.
Arrancado el 2026-08-12.

## ⚠️ Antes de tocar — pre-flight obligatorio
0. Leer [gotchas](gotchas.md) — **siempre**, antes de tocar cualquier codigo.
1. Leer [mired](mired.md) — estado real, stack elegido y por que ya no es un fork.
   Si el tema es como se usa MiRed o su ciclo de vida → [modulo-programa](modulo-programa.md).
2. Leer `PLAN.md` (raiz del repo) — las 10 fases y que entrega cada una.
3. Si el tema es base de datos, privilegios, catalogo `.toml` o empaquetado →
   [mired-arquitectura](mired-arquitectura.md).
4. Si el tema es que hace la herramienta (mapa, presencia, alertas, trafico) →
   [mired-capacidades](mired-capacidades.md).
   Si es del mapa, del cableado o de dar de alta algo que no sale en el escaneo →
   [modulo-topologia-manual](modulo-topologia-manual.md).
   Si es de subir una hoja de aparatos, de la plantilla o de `ubicacion` →
   [modulo-importacion](modulo-importacion.md).
   Si es del paquete opcional de inspeccion profunda →
   [modulo-inspeccion](modulo-inspeccion.md).
5. Si hace falta ver **como lo resolvio otro**, y solo para leer →
   [mired-upstream-scanopy](mired-upstream-scanopy.md). **Copiar codigo de ahi
   esta prohibido**: heredaria la AGPL.

## Antes de dar algo por terminado
Correr **`./herramientas/probar.sh`**: construye el `.deb`, lo desempaqueta
aparte, levanta los servicios desde ahi y recorre el flujo completo (74
comprobaciones). Correr los binarios del arbol de compilacion NO prueba lo mismo:
no valida que el paquete lleve el programa, el catalogo ni la configuracion.

Para probar una instalacion **en limpio**: `sudo ./herramientas/desinstalar.sh`.
`dpkg --purge` a secas conserva las bases a proposito y deja el equipo sucio.

## Reglas que no se negocian
- **Todo se nombra en espanol.** Unica excepcion: la carpeta `internal/`, que la
  impone el compilador de Go.
- **Un solo escritor de la base**: `mired-servidor`. La sonda escanea y entrega
  por socket, nunca escribe.
- **El enrutado de conexiones vive solo en `internal/basedatos`.**
- **Se mide el consumo de recursos al cerrar cada fase**, no al final.

## Indice
### 🏗️ Como esta construido
- [mired](mired.md) — que es, stack, tiempos, herramental verificado, pendientes.
- [mired-arquitectura](mired-arquitectura.md) — dos binarios, una base por red,
  catalogo `.toml`, estructura del repo, `.deb`.

### 📦 Funcionalidades por modulo
- [mired-capacidades](mired-capacidades.md) — las 4 capacidades (mapa, presencia,
  alertas, ancho de banda) y como degradan en redes mixtas.
- [modulo-escaneo](modulo-escaneo.md) — descubrimiento en tres capas, los dos
  ritmos de barrido, identidad del equipo y la agenda.
- [modulo-topologia](modulo-topologia.md) — SNMP, mapa de puertos, confirmado
  contra grupo, y el perfil de capacidades por red.
- [modulo-topologia-manual](modulo-topologia-manual.md) — el modulo 15: declarar
  a mano equipos, puertos y cables que ningun escaneo ve, sin que se confundan con
  lo medido. **Leer antes de tocar el mapa o el escaneo de presencia.**
- [modulo-importacion](modulo-importacion.md) — el modulo 20: subir una hoja de
  aparatos (CSV, ODS o XLSX) y meterla de una vez, con vista previa antes de
  escribir nada. **Leer antes de tocar el alta de equipos, la plantilla o la
  columna `ubicacion`.**
- [modulo-alertas](modulo-alertas.md) — que se vigila, la huella que evita
  repetir avisos, y los cuatro destinos de aviso.
- [modulo-trafico](modulo-trafico.md) — ancho de banda por contadores del switch
  y por flujos del router; por que la resta es el dato.
- [modulo-inspeccion](modulo-inspeccion.md) — la inspeccion profunda opcional
  (`mired-dpi`): que aplicacion consume, sin descifrar nada. Fase 10.
- [modulo-reconocimiento](modulo-reconocimiento.md) — **como se averigua QUE es
  cada aparato**: fabricante por MAC (lista IEEE completa), huellas activas (su
  pagina, su certificado, mDNS, UPnP y los protocolos de cada marca), reglas
  genericas degradadas y el catalogo comunitario. **Leer antes de tocar el
  catalogo o el escaneo profundo.**
- [modulo-aire](modulo-aire.md) — el modulo 17: escuchar el aire para saber que
  SSID emite cada antena y con que MAC de radio. Hace falta tarjeta WiFi.
- [modulo-inalambricos](modulo-inalambricos.md) — los modulos 18 y 19: **quien
  cuelga de que antena por el aire** (el WiFi no tiene puertos: uno o varios de
  una vez) y **las credenciales guardadas en cada equipo**, cifradas, que son
  las mismas con las que MiRed entra a su panel a preguntar. **Leer antes de
  tocar el mapa, las credenciales o los lectores de panel.**
- [modulo-programa](modulo-programa.md) — MiRed como programa de escritorio:
  como arranca y mata sus servicios, y por que ya no hay web.

### 🔗 Contratos / interconexion
- [contrato-api](contrato-api.md) — sobre `{ok, datos, error}`, rutas, sesion por
  Bearer, red activa en el contexto y el socket servidor ↔ sonda.

### 🐛 Deteccion y correccion de bugs
- [gotchas](gotchas.md) — **leer siempre antes de tocar codigo**: reglas
  transversales (fechas, borrado suave, prestamo de bases, formularios).
- [ref-herramientas-debug](ref-herramientas-debug.md) — con que se diagnostica:
  donde vive cada base, como se mira quien esta gastando el equipo y la
  grabadora `herramientas/medir.sh` para los tirones que duran dos segundos.
- [historial-bugs](historial-bugs.md) — cronica de cada problema: que paso, por
  que y como se corrigio.

### 📚 Referencia
- [ref-categorias](ref-categorias.md) — **la lista unica de categorias de
  equipo**: quien la usa, por que vive dos veces (Go y Dart) y la prueba que
  impide que se separen. **Leer antes de tocar el catalogo, el alta manual o el
  contador.**
- [red-del-usuario](red-del-usuario.md) — la red real donde se prueba: que hay,
  que contesta SNMP (nada) y que NO se puede saber ahi. **Leer antes de prometer
  cualquier cosa del mapa de puertos.**
- [mired-upstream-scanopy](mired-upstream-scanopy.md) — Scanopy y el sector:
  donde mirar y que licencia tiene cada cual. **Solo lectura.**

### 👤 Como trabajar
- Reglas globales del usuario en `~/.claude/CLAUDE.md`. Aqui aplican tal cual:
  desde que el codigo es propio **ya no hay excepcion del ingles**.
