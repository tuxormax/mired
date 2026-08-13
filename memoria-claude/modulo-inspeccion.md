---
name: modulo-inspeccion
description: "Fase 10 de MiRed: el paquete opcional mired-dpi, que dice QUE aplicacion consume"
metadata:
  node_type: memory
  type: project
  modified: 2026-08-13T18:00:00.000Z
---

# Modulo: inspeccion profunda (`mired-dpi`)

**Que hace:** responde **en que** se gasta el ancho de banda, no solo cuanto.
Netflix, un respaldo, una actualizacion, una videollamada.

**Clientes:** paquete Debian **aparte** (`mired-dpi`), que depende de `mired`.
Pesa 1.2 MB. Quien no lo instale no paga nada por el: el servidor pregunta en su
socket, nadie contesta, y se calla.

**Donde vive:** `internal/dpi/` (identificacion y captura),
`programas/mired-dpi/` (el binario), `internal/programador/inspeccion.go` (el
servidor recogiendo), `internal/basedatos/aplicaciones.go` (persistencia).

## Las tres decisiones que sostienen el resto

1. **Nada de nDPI ni libpcap.** Habrian obligado a compilar con cgo, y ahi se
   acaban los binarios estaticos y el `.deb` sin dependencias. La captura es
   AF_PACKET en Go puro con `mdlayher/packet`, que **ya era dependencia**
   (la usa `mdlayher/arp` del escaneo).
2. **No se descifra nada.** Se leen las tres cosas que viajan en claro aunque la
   conexion vaya cifrada: el **SNI del saludo de TLS**, la cabecera **Host** de
   HTTP y las **consultas de DNS**. Ultimo recurso: el puerto, que dice el tipo
   de servicio y no con quien se habla — y por eso se marca distinto.
   Las consultas de DNS valen mas de lo que parece: delatan incluso lo que
   despues va por QUIC, que no tiene saludo legible.
3. **Mismo reparto de papeles que la sonda.** `mired-dpi` tiene `CAP_NET_RAW` y
   `CAP_NET_ADMIN`, escucha en `/run/mired/dpi.sock` y **no toca la base**. El
   servidor le pregunta cada 5 minutos y escribe el. Repetir el patron en vez de
   inventar otro: quien entienda como habla la sonda ya entiende esto.

## Reglas de honestidad del dato
- **Lo que no se identifica SIGUE sumando bytes**, como "Sin identificar". Tirarlo
  haria que el informe sumara menos que el consumo real y nadie sabria por que.
- **De cada nombre se conserva de donde salio** (`tls`, `http`, `dns`, `puerto`,
  `desconocido`) y la pantalla lo dice. Un nombre del saludo de TLS no vale lo
  mismo que una suposicion por numero de puerto.
- El dominio se recorta con `DominioPrincipal`: sin eso, un solo servicio aparece
  como cincuenta destinos (`video-edge-a1b2.ftl.iad03.hls.ttvnw.net`). Los
  dominios de dos niveles (`com.mx`, `co.uk`) llevan una etiqueta mas o todo el
  pais quedaria agrupado bajo "com.mx".

## Sin puerto espejo no mide casi nada
Un switch manda a cada boca solo lo suyo. **Hace falta un puerto espejo (SPAN)**
copiando hacia la tarjeta configurada en `[dpi] interfaz`. Sin el no falla: mide
poco. Por eso `Estado` lleva la cuenta de **paquetes vistos** y avisa cuando son
menos de 100: es el unico dato que dice si el espejo esta bien configurado.

El modo promiscuo es imprescindible: sin el, la tarjeta descarta **en hardware**
todo lo que no vaya a su propia MAC, o sea justo todo lo que copia el espejo.

## Privacidad — esta escrito en la documentacion a proposito
Queda registrado con que servidores habla cada equipo, y por lo tanto que sitios
visita cada persona. NO queda el contenido. Se conserva **14 dias** y se poda
solo (escribe mucho mas que los otros medidores). Todo se queda en el equipo.
En muchos sitios esto exige avisar a la gente. Ver
`documentacion/inspeccion-profunda.md`.

## Especificaciones
| Cosa | Valor |
|---|---|
| Socket | `/run/mired/dpi.sock`, permisos 0660 (grupo `mired`) |
| Configuracion | `[dpi] socket`, `interfaz`, `cada_minutos` en `mired.toml` |
| Tabla | `trafico_aplicaciones` (migracion `red/0010`) |
| API | `GET /api/redes/{clave}/aplicaciones?horas=24` |
| Ordenes | `estado`, `cortar` |
| Poda | 14 dias |

**Ver tambien:** [[modulo-trafico]], [[mired-arquitectura]], [[mired-capacidades]],
[[contrato-api]], [[gotchas]]
