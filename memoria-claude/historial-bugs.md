---
name: historial-bugs
description: "Bitacora cronologica de problemas de MiRed: que paso, por que y como se corrigio"
metadata: 
  node_type: memory
  type: project
  originSessionId: 20376d18-adf7-4315-bb9c-98a3aa84ec95
  modified: 2026-08-12T22:35:06.517Z
---

# Historial de bugs de MiRed

Bitacora cronologica. El **tripwire** de cada uno vive en [[gotchas]] o en el
`bug-*.md` que corresponda; aqui queda la cronica.

## 2026-08-12 — `dpkg-deb` rechazaba el paquete por permisos 2775
**Problema:** al armar el `.deb`, `dpkg-deb` abortaba con "el directorio de
control tiene permisos erroneos".
**Causa:** la carpeta del proyecto es setgid `www-data` y el bit se hereda a todo
lo que se crea dentro. `chmod 0755` **no limpia el setgid de una carpeta** (lo
bajo de 2775 a 2755 y ahi se quedo).
**Solucion:** `find -type d -exec chmod g-s` explicito antes del `chmod 0755`, en
`herramientas/construir.sh`.

## 2026-08-12 — Un puerto cerrado seguia marcandose como abierto
**Problema:** una prueba con dos escaneos seguidos mostraba los dos puertos
abiertos cuando el segundo escaneo solo vio uno.
**Causa:** el cierre de puertos comparaba `ultima_vez < momento`, y `Ahora()`
tiene resolucion de **segundos**: dos escaneos dentro del mismo segundo llevan la
misma marca y la comparacion no cierra nada. En produccion pasaria en cualquier
red chica, donde un barrido tarda menos de un segundo.
**Solucion:** cerrar por **lista de lo que se vio**
(`NOT IN (VALUES (numero, protocolo), ...)`) en vez de por marca de tiempo.
**Aprendizaje general:** no usar comparaciones de tiempo para decidir "esto ya no
esta"; usar la lista de lo observado, que siempre dice la verdad.

## 2026-08-12 — Una red sin switches se quedaba en "desconocida" para siempre
**Problema:** tras un escaneo completo en una red sin equipos SNMP, la pantalla
de puertos seguia diciendo "todavia no se ha consultado", que suena a que falta
correr algo.
**Causa:** el perfil de capacidades solo se recalculaba cuando **algun** equipo
contestaba SNMP.
**Solucion:** recalcularlo tambien cuando no contesta nadie, para que quede
`no_disponible` y la interfaz explique que se pierde y que haria falta.
**Aprendizaje general:** "no encontre nada" **es** un resultado y hay que
guardarlo; si no, la interfaz no puede distinguir entre "no se ha buscado" y "se
busco y no hay".

**Ver tambien:** [[gotchas]], [[modulo-escaneo]], [[modulo-topologia]]
