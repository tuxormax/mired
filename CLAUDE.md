# MiRed — mapeo de redes (fork libre de Scanopy)

Servicio propio para descubrir equipos de una red, dibujar el mapa y saber en que
puerto de que switch esta conectado cada aparato. Fork de
[Scanopy](https://github.com/scanopy/scanopy) (AGPL-3.0).

**Estado: en planeacion, sin codigo.** El plan por fases esta en `PLAN.md`; las
decisiones y la radiografia del codigo heredado, en `memoria-claude/`.

## Que lo diferencia del upstream
1. Sin topes de redes ni de usuarios (Community topa en 1 y 1).
2. SQLite en vez de PostgreSQL, para que quepa entero en un `.deb`.
3. **Una base de datos por red** (Matriz, Sucursal 1...), no una para todas.
4. Catalogo de dispositivos en archivos `.toml` que la comunidad amplia sin
   recompilar.

## Licencia
AGPL-3.0 heredada — obligatoria, no opcional. Todo lo que se publique va con esa
licencia y con el aviso de origen.

## Reglas de codigo
- **El codigo va en ingles**, a proposito y como unica excepcion a la regla del
  usuario de nombrar en espanol: son 159 000 lineas heredadas y traducir
  identificadores cerraria la puerta a integrar los cambios de arriba.
- **En espanol si**: empaquetado, scripts, configuracion, formato del catalogo
  `.toml`, documentacion y textos de interfaz.
- Cambios quirurgicos sobre el codigo heredado: mientras menos diverja el fork,
  mas barato es traer mejoras del upstream.

## Memoria
Indice en `memoria-claude/MEMORY.md` (misma carpeta que la memoria de Claude, via
symlink, por eso se versiona con el repo).

## Historia
Nacio como una carpeta del repo `tuxormax/linux` y se saco a repo propio el
2026-08-12: no es una utilidad de escritorio, es un servicio con su propio ciclo
de vida.
