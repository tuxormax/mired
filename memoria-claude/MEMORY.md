# Memoria - MiRed (servicio propio de mapeo de redes)

Proyecto en `/home/tuxor/www/mired`. Descubre los equipos de una red, dibuja el
mapa y dice en que puerto de que switch esta cada aparato.
**Go (dos binarios) + Flutter web + SQLite, una base por red, entregado en `.deb`.**
**Codigo propio, NO un fork**: Scanopy solo se consulta, nunca se copia.
Arrancado el 2026-08-12.

## ⚠️ Antes de tocar — pre-flight obligatorio
1. Leer [mired](mired.md) — estado real, stack elegido y por que ya no es un fork.
2. Leer `PLAN.md` (raiz del repo) — las 10 fases y que entrega cada una.
3. Si el tema es base de datos, privilegios, catalogo `.toml` o empaquetado →
   [mired-arquitectura](mired-arquitectura.md).
4. Si el tema es que hace la herramienta (mapa, presencia, alertas, trafico) →
   [mired-capacidades](mired-capacidades.md).
5. Si hace falta ver **como lo resolvio otro**, y solo para leer →
   [mired-upstream-scanopy](mired-upstream-scanopy.md). **Copiar codigo de ahi
   esta prohibido**: heredaria la AGPL.

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

### 📦 Funcionalidades
- [mired-capacidades](mired-capacidades.md) — las 4 capacidades (mapa, presencia,
  alertas, ancho de banda) y como degradan en redes mixtas.

### 🔗 Contratos / interconexion
- N/A todavia. Se define en la fase 1: contrato API ↔ interfaz Flutter y el del
  socket servidor ↔ sonda.

### 🐛 Deteccion y correccion de bugs
- N/A todavia (sin codigo no hay bugs; al aparecer: `historial-bugs.md` + `bug-*.md`).

### 📚 Referencia
- [mired-upstream-scanopy](mired-upstream-scanopy.md) — Scanopy y el sector:
  donde mirar y que licencia tiene cada cual. **Solo lectura.**

### 👤 Como trabajar
- Reglas globales del usuario en `~/.claude/CLAUDE.md`. Aqui aplican tal cual:
  desde que el codigo es propio **ya no hay excepcion del ingles**.
