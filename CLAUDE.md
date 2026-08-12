# MiRed — mapeo de redes

Servicio propio para descubrir los equipos de una red, dibujar el mapa y saber en
que puerto de que switch esta conectado cada aparato. **Codigo propio, no un
fork**: Scanopy se consulta como referencia, nunca se copia.

El plan por fases esta en `PLAN.md`; las decisiones, en `memoria-claude/`.

## Stack
- **Servicio**: Go — dos binarios estaticos, `mired-servidor` (API + interfaz,
  usuario sin privilegios, unico escritor de las bases) y `mired-sonda`
  (escaneo, con `CAP_NET_RAW`/`CAP_NET_ADMIN`, no toca la base).
- **Interfaz**: Flutter compilado a web, servido por el propio servidor.
- **Datos**: SQLite. Un catalogo global + **una base por red**.
- **Auxiliar**: Python para scripts, generadores y laboratorio. Nunca en el servicio.
- **Entrega**: un `.deb` para `amd64` y `arm64`.

## Reglas de codigo
- **Todo en espanol**: paquetes, funciones, variables, archivos, tablas, columnas
  y documentacion. Unica excepcion: la carpeta `internal/`, cuyo nombre lo impone
  el compilador de Go; adentro todo vuelve al espanol.
- **El enrutado de conexiones vive solo en `internal/basedatos`.** Ningun otro
  paquete decide a que archivo escribe.
- **La sonda nunca escribe en la base**: escanea y entrega al servidor por socket
  Unix. Un solo escritor.
- Toda base se abre en modo WAL y con `busy_timeout`.
- Antes de tocar una base de red se comprueba su version de esquema y se migra.

## Memoria
Indice en `memoria-claude/MEMORY.md` (misma carpeta que la memoria de Claude, via
symlink, por eso se versiona con el repo).

## Historia
Nacio como carpeta del repo `tuxormax/linux` y se saco a repo propio el
2026-08-12. Ese mismo dia se descarto el plan de forkear Scanopy: se construye
desde cero en Go + Flutter.
