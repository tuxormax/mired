# Memoria - MiRed (servicio propio de mapeo de redes)

Proyecto en `/home/tuxor/www/mired`. Descubre los equipos de una red, dibuja el
mapa y dice en que puerto de que switch esta cada aparato.
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
   Si es del paquete opcional de inspeccion profunda →
   [modulo-inspeccion](modulo-inspeccion.md).
5. Si hace falta ver **como lo resolvio otro**, y solo para leer →
   [mired-upstream-scanopy](mired-upstream-scanopy.md). **Copiar codigo de ahi
   esta prohibido**: heredaria la AGPL.

## Antes de dar algo por terminado
Correr **`./herramientas/probar.sh`**: construye el `.deb`, lo desempaqueta
aparte, levanta los servicios desde ahi y recorre el flujo completo (32
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
- [modulo-alertas](modulo-alertas.md) — que se vigila, la huella que evita
  repetir avisos, y los cuatro destinos de aviso.
- [modulo-trafico](modulo-trafico.md) — ancho de banda por contadores del switch
  y por flujos del router; por que la resta es el dato.
- [modulo-inspeccion](modulo-inspeccion.md) — la inspeccion profunda opcional
  (`mired-dpi`): que aplicacion consume, sin descifrar nada. Fase 10.
- [modulo-programa](modulo-programa.md) — MiRed como programa de escritorio:
  como arranca y mata sus servicios, y por que ya no hay web.

### 🔗 Contratos / interconexion
- [contrato-api](contrato-api.md) — sobre `{ok, datos, error}`, rutas, sesion por
  Bearer, red activa en el contexto y el socket servidor ↔ sonda.

### 🐛 Deteccion y correccion de bugs
- [gotchas](gotchas.md) — **leer siempre antes de tocar codigo**: reglas
  transversales (fechas, borrado suave, prestamo de bases, formularios).
- [historial-bugs](historial-bugs.md) — cronica de cada problema: que paso, por
  que y como se corrigio.

### 📚 Referencia
- [red-del-usuario](red-del-usuario.md) — la red real donde se prueba: que hay,
  que contesta SNMP (nada) y que NO se puede saber ahi. **Leer antes de prometer
  cualquier cosa del mapa de puertos.**
- [mired-upstream-scanopy](mired-upstream-scanopy.md) — Scanopy y el sector:
  donde mirar y que licencia tiene cada cual. **Solo lectura.**

### 👤 Como trabajar
- Reglas globales del usuario en `~/.claude/CLAUDE.md`. Aqui aplican tal cual:
  desde que el codigo es propio **ya no hay excepcion del ingles**.
