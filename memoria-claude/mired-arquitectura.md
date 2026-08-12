---
name: mired-arquitectura
description: Decisiones de arquitectura de MiRed: SQLite, una base por red, catalogo .toml y empaquetado .deb
metadata:
  type: project
---

# MiRed — decisiones de arquitectura

Acordadas el **2026-08-12**. El desarrollo detallado esta en `PLAN.md`;
aqui queda **la decision y su porque**, que es lo que no se debe volver a
discutir.

## 1. SQLite en vez de PostgreSQL
**Por que:** para que el `.deb` se instale solo, sin pedir un servidor de base de
datos aparte. Portabilidad por encima de todo.

**Como:** el acceso a datos vive casi entero en `shared/storage/` (~4 500
lineas), asi que el cambio es grande pero acotado: se toca un lugar, no 159 000
lineas. Hay que resolver marcadores (`$1` → `?`), `= ANY()` → `IN`, tipos que
SQLite no tiene (UUID, INET, CIDR, MACADDR, TIMESTAMPTZ → TEXT canonico
validado), JSONB → JSON1, dos triggers y `pgcrypto` (se sustituye por el hash que
el backend ya trae).

**Decision sobre las 116 migraciones: se aplanan.** Se levanta el esquema final
en PostgreSQL, se vuelca, se traduce **una sola vez** a un esquema base de SQLite
y MiRed arranca su propia numeracion. Traducirlas una por una es trabajo perdido
y ademas imposibilitaria integrar los cambios de arriba (que luego serian 1-5
migraciones nuevas por version, no 116).

## 2. Una base de datos por red (la decision mas invasiva)
Al crear una red se crea **su propio archivo SQLite** con todo lo operativo de
esa red. Ni una tabla compartida con las demas.

**Por que:**
- El limite de "una escritura a la vez" de SQLite es **por archivo**: veinte
  redes escaneando en paralelo son veinte archivos escribiendo a la vez. Esto
  elimino el mayor riesgo tecnico que tenia el plan.
- Respaldar o mover un sitio = copiar un archivo. Borrar una red = borrarlo.
- Una base corrupta se lleva una red, no el sistema.

**Reparto:**
- **Catalogo** (`/var/lib/mired/mired.db`, uno): usuarios, sesiones, claves de
  API, permisos de quien ve que red, credenciales SNMP reutilizables y el
  registro de redes con un **resumen por red** (para que el panel de inicio no
  abra treinta archivos).
- **Red** (`/var/lib/mired/redes/matriz-a1b2.db`, una por red): todo lo
  operativo. Cada archivo lleva dentro su propia ficha de red → es
  **autodescriptivo** y se puede copiar a otra instalacion.

**Donde se enchufa:** en `storage/factory.rs`, que hoy construye un objeto de
almacenamiento por entidad con la misma conexion. Cada entidad se marca como
*global* o *de red*; las de red resuelven su archivo segun la **red activa del
contexto de la tarea**, que fija el middleware de autenticacion (ya resuelve por
peticion a que redes tiene acceso quien pregunta). Asi **no hay que cambiar la
firma de las funciones** en 159 000 lineas.

**Nombre del archivo:** `nombre-corto-<id>.db` (`matriz-a1b2.db`). `matriz.db` a
secas se rompe con acentos, espacios, cambios de nombre y dos redes que se llamen
igual. El nombre bonito vive dentro y se puede cambiar cuando sea.

**Lo que cuesta, y hay que vigilar siempre:**
- Las consultas que **cruzan redes** (panel general, busqueda global) dejan de
  ser un `SELECT` y pasan a ser recorrido de archivos o `ATTACH`.
- Las migraciones se aplican **a cada archivo**, incluso a uno viejo restaurado
  de un respaldo.
- No se pueden tener cien bases abiertas: juego de conexiones con cierre de las
  que no se usan.
- **El enrutador tiene que quedar concentrado en la capa de almacenamiento.** Si
  se reparte por los modulos, el fork deja de poder sincronizarse con arriba. Es
  el cambio que rompe la suposicion "todo esta en una sola base", y arriba van a
  seguir dandola por buena para siempre.

## 3. Catalogo de dispositivos en `.toml` (el diferenciador)
Hoy cada uno de los 257 dispositivos reconocidos es un archivo Rust: aportar uno
exige saber Rust y recompilar. Ese es el cuello de botella que impide que un
catalogo crezca por comunidad.

Se convierten a `.toml` leidos al arrancar desde `/usr/share/mired/dispositivos/`
(los del paquete) y `/etc/mired/dispositivos/` (los del usuario, que mandan). Los
257 se convierten con un script, no a mano. En la interfaz, todo equipo sin
identificar ofrece **"proponer definicion"**, que genera el `.toml` ya relleno con
lo que se vio. El repo valida cada aportacion automaticamente.

## 4. Empaquetado: un solo `.deb`
`/usr/bin/mired-servidor` y `/usr/bin/mired-daemon`, interfaz compilada en
`/usr/share/mired/web/`, catalogo en `/usr/share/mired/dispositivos/`, bases en
`/var/lib/mired/`, config en `/etc/mired/mired.toml`, dos unidades systemd.

- El servidor corre como usuario de sistema `mired`, **no root**.
- El demonio necesita red cruda: se le dan capacidades acotadas
  (`CAP_NET_RAW`, `CAP_NET_ADMIN`), no root entero. Mismo criterio que
  [[proyecto-patron-gui-root]].
- El `postinst` crea el catalogo, habilita servicios y **nunca falla**, igual que
  [[driver-tickets]].
- Compilar para `amd64` y `arm64`: la Raspberry Pi es el equipo natural para
  dejar esto encendido.

**Ver tambien:** [[mired]], [[mired-capacidades]], [[mired-upstream-scanopy]]
