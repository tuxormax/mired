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

Los fabricantes se confirmaron el **2026-08-15** contra el registro de la IEEE:

| Direccion | MAC | Fabricante | Que es |
|---|---|---|---|
| 192.168.1.254 | a0:95:7f | SERNET (Suzhou) | **modem Telmex** |
| 192.168.1.100 | — | — | **esta PC** (la que corre MiRed) |
| 192.168.1.86 | f4:92:bf | **Ubiquiti** | **el AP** ← era el que salia como "Equipo Linux o Unix" |
| 192.168.1.74 | 3c:e3:6b | Dahua | DVR / grabador |
| 192.168.1.65 | a0:d7:f3 | Samsung | probablemente la television |
| 192.168.1.71 | 08:65:f0 | JM Zengge | foco o enchufe WiFi |
| 192.168.1.77 | e8:5a:8b | Xiaomi | telefono o aparato de casa |
| .69 y .70 | MAC aleatoria | — | telefonos con MAC de privacidad |

```
Modem Telmex  192.168.1.254   ← puerta de enlace
      │ un solo cable a un puerto LAN
   Switch simple  (no administrable, declarado a mano)
      ├── esta PC        192.168.1.100
      ├── AP Ubiquiti    192.168.1.86
      ├── DVR Dahua      192.168.1.74
      └── lo demas
```

**Aqui NO se puede oir el aire**: esta PC solo tiene tarjeta de cable
(`enp4s0`). El modulo 17 avisa de eso con todas sus letras en vez de devolver
una lista vacia. Para ver SSID y BSSID hace falta correr MiRed en una laptop o
en una Raspberry con WiFi, o poner un adaptador USB.

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
  espejo de puertos. PoE+ en las 8 puertos.
- `TL-SG108PE` — linea **Easy Smart**: tiene web y espejo de puertos, pero **no
  SNMP**; se administra solo con la utilidad propietaria de TP-Link.
- Linea **Smart / JetStream** (`TL-SG2008`, `TL-SG2210P`, `T1500G` y arriba) —
  esos si traen SNMP v1/v2c/v3.

Un switch con **espejo de puertos** es ademas lo que necesita `mired-dpi`. Un
Smart con SNMP da las dos cosas: mapa de puertos e inspeccion profunda.

**Ver tambien:** [[modulo-topologia]], [[mired-capacidades]], [[mired]]
