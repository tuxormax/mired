---
name: mired
description: "MiRed, servicio propio para mapear redes en Go + Flutter; estado, alcance y decisiones de fondo"
metadata: 
  node_type: memory
  type: project
  originSessionId: 20376d18-adf7-4315-bb9c-98a3aa84ec95
  modified: 2026-08-12T21:26:00.738Z
---

# MiRed — servicio propio de mapeo de redes

Proyecto en `/home/tuxor/www/mired`, definido el **2026-08-12**. Descubre los
equipos de una red, dibuja su mapa y dice en que puerto de que switch esta cada
aparato. El plan por fases esta en `PLAN.md` (versionado); esta memoria guarda
**las decisiones y su porque**, que es lo que no se debe volver a discutir.

## El giro del 2026-08-12: ya NO es un fork
El plan original era forkear [Scanopy](https://github.com/scanopy/scanopy)
(AGPL-3.0) y quitarle los topes. **El usuario lo descarto el mismo dia**: quiere
proyecto propio sin depender de nadie.

**Lo que eso cambio, y no es poco:**
- **Lo legal**: la AGPL viaja con el codigo, no con el historial de git. Copiar
  archivos —aunque sea sin historia— obligaria a MiRed a ser AGPL. Leyendo su
  codigo para entender como resolvieron algo y escribiendo el nuestro, MiRed es
  propio y su licencia se elige libre. **Scanopy = solo lectura de referencia.**
- **Lo practico**: desaparecieron las tres fases mas caras y riesgosas del plan
  viejo (rename de 2 509 apariciones, puerto a SQLite de codigo ajeno, romper su
  suposicion de "una sola base"): entre 5 y 9 semanas de pelear contra decisiones
  de otro.
- **Se acabo la excepcion del ingles**: siendo codigo propio, **todo va en
  espanol** como el resto de los proyectos de la casa.

## Stack elegido (2026-08-12)
El usuario pidio "go o rust con flutter para generar un .deb instalable" y
autorizo Python si hace falta. **Se eligio Go**, por tres razones concretas:
1. Binario estatico sin dependencias → el `.deb` no pide runtime ni servidor de BD.
2. Cruza a `arm64` (Raspberry Pi) con una variable de entorno; Rust exige montar
   toolchain cruzado.
3. ARP crudo, SNMP, mDNS y el driver de SQLite existen **en Go puro, sin cgo** —
   que es lo que permite que el binario sea de verdad estatico.
Ademas WatchYourLAN, la herramienta mas liviana de la categoria, es Go: el
presupuesto de recursos es alcanzable. Rust daria algo menos de memoria a cambio
de un lenguaje que hoy no se maneja en casa.

**Flutter compilado a web**, servido por el propio binario: se entra desde
cualquier navegador de la red, es el entorno que ya se maneja en casa, y **el
mismo codigo da despues la app de escritorio y Android** sin reescribir nada.

**Python solo auxiliar**: scripts, generadores del catalogo, laboratorio. Nunca
en el servicio, para no romper el binario unico.

## Herramental verificado en el equipo (2026-08-12)
Go 1.26.4 y Flutter 3.35.6 **ya instalados**. No hay Rust, ni Docker, ni
`sqlite3` de linea de comandos. Node es 18. PostgreSQL 16 esta (ya no hace falta).

## Estado real (2026-08-12)
**Fases 1 a 4 cerradas** en un solo dia de trabajo:
1. Cimientos: servidor + sonda, una base SQLite por red, autenticacion con
   permisos por red, API con el estandar de errores de la casa, interfaz Flutter
   web y `.deb` para amd64 y arm64.
2. Descubrimiento de equipos por ARP, ICMP y TCP, con puertos, DNS inverso y
   fabricante por OUI.
3. Presencia en vivo, historial de conexiones y barridos programados por red.
4. SNMP v1/v2c/v3, tabla de MAC del switch, LLDP, mapa de puertos y perfil de
   capacidades.

Pruebas en verde (Go y Flutter). **Falta**: probar el `.deb` instalado de verdad,
probar SNMP contra un switch real, y las fases 5 a 10.

## Tiempos
- **3-4 meses** hasta la fase 9 (publicable). La inspeccion profunda va aparte.
- **Al cerrar la fase 2 ya hay `.deb` instalable que inventaria la red**, y eso
  solo ya reemplaza varias herramientas. Se usa en produccion propia desde ahi.

## Lo que falta decidir
- **La licencia**: ya no se hereda ninguna. Se decide antes de publicar (fase 9).
  AGPL protege contra versiones cerradas de terceros; MIT maximiza adopcion.
- **Inventario de las redes reales** (marca/modelo de switches y puntos de acceso,
  cuales son administrables). No bloquea hasta la fase 4, pero se necesita antes
  de llegar ahi para probar SNMP contra equipo de verdad.

**Ver tambien:** [[mired-arquitectura]], [[mired-capacidades]],
[[mired-upstream-scanopy]]
