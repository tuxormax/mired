---
name: red-del-usuario
description: "La red real de tuxor: que hay, que contesta y que NO se puede saber en ella"
metadata:
  node_type: memory
  type: reference
  modified: 2026-08-13T23:59:00.000Z
---

# La red donde se esta probando MiRed

Medido el **2026-08-13**, no supuesto. Sirve para no volver a averiguarlo y, sobre
todo, para no prometer cosas que en esta red no se pueden dar.

## Que hay

```
Modem (Telmex/Megacable)  192.168.1.254   ← puerta de enlace
      │ un solo cable a una boca LAN
   Switch simple  (no administrable)
      ├── esta PC        192.168.1.100
      ├── AP Ubiquiti    (direccion por confirmar)
      ├── DVR
      └── otra PC        (apagada)
```

## Que contesta SNMP: NADA

Probado con las comunidades de fabrica `public`, `private`, `cablemodem` y
`admin`, en v1 y v2c, contra `192.168.1.254`, `192.168.1.1`, `192.168.100.1` (la
direccion tipica de gestion de un modem de cable) y `.2` a `.5`:

**Ninguno contesto.** El modem del proveedor no habla SNMP.

**Falta por probar el AP Ubiquiti**, que es la unica esperanza real de esta red:
esos equipos suelen traer SNMP. No se probo porque no se sabe su direccion — se
sabra al escanear.

## Lo que esto significa, y hay que decirlo sin rodeos
- **El puerto exacto de cada aparato NO se puede saber aqui.** Ese dato vive solo
  dentro del switch, y uno simple no tiene forma de contarselo a nadie. No es un
  limite de MiRed: no lo puede hacer ningun programa.
- Lo demas si: inventario, presencia, puertos abiertos de cada equipo, alertas y
  consumo por el router.

## Si algun dia se compra un switch
La regla para cualquier marca: **buscar "SNMP" en la hoja de especificaciones.**
No fiarse de las palabras "smart", "gestionable" o "administrable" del nombre.

Verificado el 2026-08-13:
- `TL-SG1008PE` — TP-Link lo cataloga como **no administrable**. Sin SNMP y sin
  espejo de puertos. PoE+ en las 8 bocas.
- `TL-SG108PE` — linea **Easy Smart**: tiene web y espejo de puertos, pero **no
  SNMP**; se administra solo con la utilidad propietaria de TP-Link.
- Linea **Smart / JetStream** (`TL-SG2008`, `TL-SG2210P`, `T1500G` y arriba) —
  esos si traen SNMP v1/v2c/v3.

Un switch con **espejo de puertos** es ademas lo que necesita `mired-dpi`. Un
Smart con SNMP da las dos cosas: mapa de puertos e inspeccion profunda.

**Ver tambien:** [[modulo-topologia]], [[mired-capacidades]], [[mired]]
