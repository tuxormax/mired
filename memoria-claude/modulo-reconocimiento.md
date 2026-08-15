---
name: modulo-reconocimiento
description: "Como MiRed averigua QUE es cada aparato: fabricante por MAC, huellas activas y catalogo comunitario"
metadata:
  node_type: memory
  type: project
  modified: 2026-08-15T00:00:00.000Z
---

# Reconocer que es cada aparato

El escaneo dice **donde** hay algo. Este modulo dice **que** es. Son cosas
distintas y se rompio una vez por confundirlas: en la red de casa, el modem, la
television y la antena salian los tres como "Servidor web" porque lo unico que se
miraba era que tuvieran el puerto 80 abierto.

## Las cuatro fuentes, de mas firme a menos

| Fuente | Que da | Donde vive |
|---|---|---|
| **Prefijo de la MAC** | el fabricante | `internal/escaneo/fabricantes.go` + `catalogo/fabricantes.txt` |
| **Huella activa** | lo que el aparato dice DE SI MISMO | `internal/huella/` |
| **SNMP** | descripcion del sistema | `internal/snmp/` |
| **Puertos abiertos** | un sintoma, no una identidad | `internal/escaneo/puertos.go` |

## 1. El fabricante por la MAC

La lista **completa de la IEEE viaja en el paquete** como archivo de texto
(`/usr/share/mired/fabricantes.txt`, 52 977 prefijos), no dentro del binario: se
actualiza corriendo `herramientas/traer_fabricantes.py` y reemplazando el `.txt`.
La lista corta embebida en Go es el suelo para cuando el archivo falta.

**No todos los prefijos miden lo mismo.** La IEEE vende bloques de 24, 28 y 36
bits. La busqueda va del prefijo **mas largo al mas corto**: si el de 9 digitos
contesta, ese manda sobre el de 6, que solo diria el nombre del mayorista.

## 2. Las huellas activas — `internal/huella`

Se le pregunta al aparato y se guarda **con la fuente de donde salio**:

- **web**: titulo de la pagina, encabezado `Server`, el realm del candado.
- **certificado**: nombre comun y organizacion del TLS (ahi los UBNT dicen UBNT).
- **ssh**: el banner, que ya lo trae el escaneo de puertos.
- **mdns / ssdp / onvif**: lo que la red anuncia sola. Una sola pregunta al grupo
  por barrido trae el nombre y el modelo de televisiones, Chromecast, impresoras
  y camaras sin tocarle un puerto a nadie.
- **netbios**: el nombre de las maquinas Windows.
- **protocolos de fabricante**, en `internal/huella/fabricantes.go` y
  `marcas.go`:

| Marca | Puerto | Que da | Confianza |
|---|---|---|---|
| Ubiquiti | UDP 10001 | modelo, firmware, nombre, MAC del radio y **el SSID que emite** | documentado |
| MikroTik MNDP | UDP 5678 | identidad, RouterOS, placa | documentado |
| TP-Link Kasa | UDP 9999 | modelo, alias, firmware (focos y enchufes) | documentado |
| Hikvision SADP | UDP 37020 | modelo, serie, firmware | documentado |
| NETGEAR NSDP | UDP 63322 | modelo, nombre, MAC, firmware | documentado |
| Xiaomi miIO | UDP 54321 | confirma que es de casa inteligente + su identificador | documentado |
| **Tuya / Smart Life** | UDP 6666/6667 | **no contesta: avisa solo.** Hay que ESCUCHAR | documentado |
| TP-Link Omada | UDP 29810 | modelo del EAP | ingenieria inversa |
| D-Link DDP | UDP 62976 | modelo y nombre del switch o AP | ingenieria inversa |
| Dahua DHDiscover | UDP 37810 | tipo, serie y firmware | ingenieria inversa |
| Ruckus | UDP 12223 | modelo y nombre del AP | ingenieria inversa |
| **Fortinet** | — | no hay descubrimiento: se lee su **certificado** (trae el numero de serie, y de ahi el modelo) y el puerto **541** | seguro |
| Roku / TV Samsung / IPP | TCP 8060 / 8001 / 631 | ficha completa por HTTP | documentado |

**Los de ingenieria inversa se tratan distinto**: se manda el saludo que se sabe
que funciona y se lee **solo lo que se entiende** de la respuesta —las cadenas
legibles que se parecen a un modelo de esa marca—, sin fingir que se decodifica
el protocolo entero. Es poco, pero es cierto.

**Tuya rompe el patron de preguntar y esperar.** Esos aparatos no contestan a
nada; anuncian solos cada pocos segundos. Por eso hay `escucharDifusion` en
`aire.go`: se abre el puerto y se oye. El anuncio va tapado con una llave FIJA y
publica (la misma en todos los aparatos del mundo, y esta en la documentacion de
sus propias librerias): sirve para leer lo que el aparato grita, no para entrar
en el. Si el puerto ya lo tiene otro programa del equipo, no se insiste.

**Nada de esto entra a ningun lado**: son los mismos mensajes que usan las
aplicaciones de cada fabricante para encontrar sus equipos. No se prueban claves.

Se guarda en la tabla `huellas` (migracion red 0015), con `fuente`, `clave` y
`valor`. **La fuente va con el dato a proposito**: no vale lo mismo un modelo
firmado en un certificado que uno sacado del titulo de una pagina que cualquiera
pudo cambiar.

## 3. Lo generico no le gana a lo que identifica

Una definicion del catalogo puede marcarse `generico = true`. Eso significa que
describe un **sintoma** ("tiene el 80 abierto", "acepta SSH"), no un aparato.

**Una definicion generica solo contesta si ninguna otra pudo.** Sin esto, el
modem, la television y la antena salen los tres como "Servidor web", que es la
peor respuesta posible: parece un dato y no lo es.

Hoy son genericas `servidor-web.toml` (que ademas ya no cuenta como servidor:
categoria `otro`, nombre "Equipo con pagina web") y `servidor-ssh.toml`.

## 4. El catalogo comunitario — modulo 16

Tres carpetas, y **mandan en este orden**:

1. `/usr/share/mired/dispositivos` — lo que trae el paquete.
2. `/var/lib/mired/dispositivos-comunidad` — lo que se bajo del repositorio.
3. `/etc/mired/dispositivos` — lo que se escribio aqui. **Gana siempre**: quien
   tiene el aparato delante sabe mas que ningun repositorio.

Corriendo **como programa de escritorio** las dos ultimas se mueven a la carpeta
del usuario (`MIRED_DISPOSITIVOS_PROPIOS` y `MIRED_DISPOSITIVOS_COMUNIDAD`), que
las pone el propio programa al arrancar los servicios: en `/etc` no hay permiso.

- `POST /api/catalogo/dispositivos` guarda una definicion y **recarga el catalogo
  en caliente** (`Catalogo.Reemplazar`, con candado). Reemplaza el CONTENIDO y no
  el puntero: el servidor y el programador guardan el suyo por separado.
- `POST /api/catalogo/actualizar` baja las definiciones publicadas.
- Compartir NO lo hace el servicio: se abre en el navegador un aporte ya escrito
  (`URLParaAportar`). Aportar es un acto de una PERSONA con su cuenta; un
  servicio que publicara solo necesitaria una credencial del proyecto en cada
  instalacion y cualquiera podria llenar el catalogo de basura sin que se sepa
  quien fue.

## Antes de tocar esto
- Una definicion **sin condiciones coincide con TODO**. `Guardar` lo rechaza; si
  se agrega una condicion nueva, hay que sumarla a `tieneCondiciones`.
- Las condiciones dentro de una definicion se suman (**Y**). Para reconocer por
  fabricante **o** por huella hacen falta **dos archivos** (ver `ubiquiti.toml`
  y `ubiquiti-por-huella.toml`).

**Ver tambien:** [[modulo-escaneo]] · [[modulo-aire]] · [[ref-categorias]] ·
[[red-del-usuario]] · [[gotchas]]
