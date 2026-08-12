# Memoria - MiRed (fork libre de Scanopy)

Proyecto propio en `/home/tuxor/www/mired`. Servicio para mapear redes: descubre
equipos, dibuja el mapa y dice en que puerto de que switch esta cada aparato.
**En planeacion desde 2026-08-12, sin una sola linea escrita.**

## ⚠️ Antes de tocar — pre-flight obligatorio
1. Leer [mired](mired.md) — estado real, alcance y que falta decidir.
2. Leer `PLAN.md` (raiz del repo) — el desarrollo por fases.
3. Si el tema es la base de datos, el catalogo `.toml` o el empaquetado →
   [mired-arquitectura](mired-arquitectura.md).
4. Si el tema es que hace la herramienta (mapa, presencia, alertas, trafico) →
   [mired-capacidades](mired-capacidades.md).
5. Si hay que tocar codigo heredado → [mired-upstream-scanopy](mired-upstream-scanopy.md)
   **antes de clonar o buscar nada**: ya tiene rutas, conteos y licencias.

## Indice
### 🏗️ Como esta construido
- [mired](mired.md) — que es, por que se forkea, alcance, tiempos, pendientes.
- [mired-arquitectura](mired-arquitectura.md) — SQLite, **una base por red**, donde
  se corta el codigo heredado, catalogo `.toml`, empaquetado `.deb`.
- [mired-upstream-scanopy](mired-upstream-scanopy.md) — radiografia del codigo
  ajeno: rutas exactas, conteos, licencias.

### 📦 Funcionalidades
- [mired-capacidades](mired-capacidades.md) — las 4 capacidades (mapa, presencia,
  alertas, ancho de banda) y como degradan en redes mixtas.

### 🔗 Contratos / interconexion
- N/A todavia (no hay codigo; el corte entre servicio, API y UI se define en la fase 1).

### 🐛 Deteccion y correccion de bugs
- N/A todavia (sin codigo no hay bugs; al aparecer: `historial-bugs.md` + `bug-*.md`).

### 👤 Como trabajar
- Reglas globales del usuario en `~/.claude/CLAUDE.md`. Excepcion propia de este
  proyecto: **el codigo va en ingles** (heredado), documentado en [mired](mired.md).
