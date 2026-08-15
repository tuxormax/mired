---
name: modulo-aire
description: "El modulo 17: escuchar el aire para saber que SSID emite cada antena y con que MAC de radio"
metadata:
  node_type: memory
  type: project
  modified: 2026-08-15T00:00:00.000Z
---

# El aire: SSID y BSSID de las antenas

Es la **unica medicion de MiRed que no pasa por un cable**. Contesta lo que
ningun barrido de IP puede contestar:

- que SSID emite cada antena, y con que **BSSID — que es la MAC del radio**, que
  NO es la misma que la del cable;
- en que canal y en que banda, para ver dos antenas peleando por el mismo;
- con que fuerza llega cada una;
- y que redes hay alrededor que no son de uno.

## Hace falta tarjeta WiFi, y se dice

Una PC de escritorio conectada por cable **no oye nada**. Eso no es una falla: es
que no tiene con que oir. Se devuelve una **explicacion**, no una lista vacia,
porque una lista vacia se lee como "aqui no hay redes inalambricas".

Es justo el caso de la maquina donde se desarrolla (ver [[red-del-usuario]]):
solo tiene `enp4s0`.

## Como se barre — `internal/aire`

De menos a mas privilegio:

1. **`nmcli`** (NetworkManager). Ya tiene un barrido reciente y **no necesita
   permisos**: en un escritorio funciona como usuario normal. Su formato terse
   escapa los dos puntos que van dentro de un valor, y por eso la MAC llega como
   `AA\:BB\:CC\:...` — hay que deshacerlo (`partirTerse`).
2. **`iw dev X scan`**, que si necesita CAP_NET_ADMIN pero funciona en un equipo
   sin escritorio. Si no hay permiso se cae a `scan dump`, que devuelve lo ultimo
   que la tarjeta oyo: dato mas viejo, pero mucho mejor que nada.

No se implementa nl80211 por netlink: serian cientos de lineas para repetir lo
que ya hace una herramienta que viene en todas las distribuciones.

`nmcli` no expone el dBm crudo, solo un 0-100. La conversion a dBm es
**aproximada y esta dicho asi en el codigo**: presentar el porcentaje como una
medicion en dBm seria inventar precision que no hay.

## De que aparato es cada antena

`deQuienEsLaAntena` (en `internal/api/aire.go`) cruza el BSSID con el inventario
de la red activa, y **distingue dos cosas que no valen lo mismo**:

- **medido**: la MAC del radio ES la del equipo. Es un hecho.
- **deducido por cercania de MAC**: mismo prefijo de fabricante y a menos de
  cuatro numeros de distancia. Casi todos los fabricantes numeran asi sus
  interfaces, pero **es una deduccion** y se devuelve marcada para que la
  interfaz no la presente como medida.

## No se barre solo

Recorrer los canales **corta unos segundos el WiFi del equipo que barre**.
Hacerlo al abrir la pestana seria cortarle la conexion a alguien por dibujar una
lista: hay un boton, y hasta que no se pulsa no se toca nada.

Tampoco se guarda en la base: es una foto del momento. El aire cambia con quien
pasa por el pasillo, y guardar cada barrido como inventario llenaria la base de
ruido.

## Lo que este modulo NO hace
- No se conecta a ninguna red ni prueba claves.
- No dice quien esta conectado a cada SSID: eso solo lo sabe la antena, y para
  eso esta el modulo de controladoras (UniFi) o el protocolo propio del
  fabricante (ver [[modulo-reconocimiento]]).

**Ver tambien:** [[modulo-reconocimiento]] · [[red-del-usuario]] ·
[[mired-capacidades]] · [[contrato-api]]
