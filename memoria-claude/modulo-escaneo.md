---
name: modulo-escaneo
description: "Como MiRed descubre equipos (ARP, ICMP, TCP), la presencia en vivo y los barridos programados"
metadata: 
  node_type: memory
  type: project
  originSessionId: 20376d18-adf7-4315-bb9c-98a3aa84ec95
  modified: 2026-08-12T22:34:32.155Z
---

# Modulo: escaneo y presencia

**Que hace:** descubre que equipos hay en cada subred, con que nombre, que
puertos tienen abiertos y quien esta encendido ahora mismo.

**Donde vive:** `internal/escaneo/` (corre en la sonda),
`internal/programador/` (orquesta, corre en el servidor),
`internal/basedatos/equipos.go` y `agenda.go`.

## Descubrimiento en tres capas, de mas a menos confiable
1. **ARP** — solo si la subred esta conectada a una tarjeta de este equipo. Da la
   MAC, no se puede filtrar con firewall. Necesita `CAP_NET_RAW`.
2. **ICMP** — para lo que no es local. Da presencia, no MAC. Intenta primero el
   socket sin privilegios (`ping_group_range`) y cae al crudo.
3. **TCP** — toca 8 puertos muy comunes. **Es la unica capa que funciona sin
   permisos de red cruda**, y por eso existe: una instalacion sin capacidades
   sigue sirviendo de inventario en vez de mostrar pantalla vacia.

Cada equipo guarda **con que metodo se vio** (`equipos.metodo`) y la interfaz lo
muestra como "certeza". No es lo mismo "esta" que "algo contesto ahi".

## Dos ritmos, nunca mezclados
- **presencia**: barrido corto (15 s a 1 h), solo dice quien esta. NO resuelve
  nombres ni toca puertos. Es lo que lo hace liviano.
- **profundo**: puertos, DNS inverso, fabricante por OUI y, al final, SNMP.

## Identidad del equipo
`equipos.identidad` = **MAC** si la hay, `ip:<direccion>` si no. Con DHCP la IP
cambia sola: usarla como identidad duplicaria cada aparato en cada reconexion.
Al actualizar se usa `COALESCE(NULLIF(?, ''), columna)` para que un barrido
rapido **no borre** lo que averiguo uno profundo.

## Reglas que ya mordieron
- Los puertos que dejan de contestar se cierran **por lista de lo visto**, no
  comparando marcas de tiempo: dos escaneos en el mismo segundo tienen la misma
  marca (ver [[historial-bugs]]).
- Un escaneo por red a la vez, garantizado por el programador. Dos a la vez se
  pisan al marcar ausentes y los equipos parpadearian.
- La agenda se corre **antes** de lanzar el barrido, no despues: si no, un
  barrido mas lento que su intervalo dispararia el siguiente al terminar y la red
  quedaria escaneandose sin parar.
- Solo se anotan los **cambios** de presencia. Con barrido cada minuto, guardar
  siempre serian 1 440 renglones diarios por equipo para decir lo mismo.

**Ver tambien:** [[modulo-topologia]], [[contrato-api]], [[gotchas]]
