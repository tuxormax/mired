---
name: ref-herramientas-debug
description: "Con que se diagnostica un problema de MiRed: donde estan los datos, los servicios y como se mide un tiron del equipo"
metadata:
  node_type: memory
  type: reference
  modified: 2026-08-15T00:00:00.000Z
---

# Con que se diagnostica

## Donde vive todo, corriendo como PROGRAMA

| Que | Donde |
|---|---|
| Base global (usuarios, redes, agenda) | `~/.local/share/mired/mired.db` |
| Una base por red | `~/.local/share/mired/redes/<clave>.db` |
| Socket de la sonda | `~/.local/share/mired/sonda.sock` |
| Llave del cifrado de claves | junto al archivo de configuracion, **no** con los datos |
| Servidor escuchando | `127.0.0.1:60072` (solo este equipo) |

Corriendo como SERVICIO, lo mismo pero en `/var/lib/mired` y `/etc/mired`.

## Lo primero que hay que mirar ante "va lento" o "se traba"

```
ps -eo pid,pcpu,pmem,rss,comm --sort=-pcpu | head          # quien gasta el equipo
ss -lntp | grep mired                                      # si el servidor escucha
./herramientas/medir.sh                                    # grabar un tiron
```

**`herramientas/medir.sh`** anota cada segundo quien consume CPU y memoria, y
marca los procesos atascados en disco o en el nucleo —que es lo que hace que se
trabe TODO el equipo y no solo un programa—. Un tiron de dos segundos no se
diagnostica mirando la pantalla: cuando uno abre el monitor, ya paso.

## Preguntas que se contestan leyendo la base, sin la interfaz

```sql
-- ¿Estan encendidos los barridos automaticos?
SELECT clave, programado, presencia_cada_segundos, profundo_cada_minutos FROM redes;
-- ¿Cuanto hay de verdad guardado?
SELECT COUNT(*) FROM equipos; SELECT COUNT(*) FROM huellas;
```

Se abren **en modo lectura** (`?mode=ro`): la base la escribe un solo proceso, y
meterse a escribir por detras es la forma segura de corromperla.

## Lo que ya se descarto una vez (2026-08-15)

Ante un "se traba toda la maquina al abrir la red", se midio y NO era:
- Los barridos: `programado = 0` y los dos servicios al **0% de CPU**, con 17 MB
  y 7 MB de memoria.
- Los datos: la base de la red pesaba **284 KB**.
- La memoria: 25 GB disponibles de 31 GB, sin rastro de OOM en `dmesg`.
- El lienzo del mapa: ~0.9 Mpx, que no ahoga a ninguna tarjeta.

Queda por descartar el camino grafico (Flutter sobre NVIDIA en X11), y para eso
hace falta una medicion **en el momento del tiron**.

**Ver tambien:** [[gotchas]] · [[historial-bugs]] · [[modulo-programa]]
