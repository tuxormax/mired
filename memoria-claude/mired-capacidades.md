---
name: mired-capacidades
description: Las cuatro capacidades que MiRed junta (mapa, presencia, alertas, ancho de banda) y como degrada en redes mixtas
metadata:
  type: project
---

# MiRed — las cuatro capacidades y las redes mixtas

Decidido el **2026-08-12** a partir de una comparativa que trajo el usuario entre
Scanopy, NetAlertX, WatchYourLAN y ntopng: **lo mejor de cada una en una sola
herramienta**.

## Cuatro preguntas, cuatro capacidades
| Pregunta | De quien se toma | Que hay ya en el codigo heredado |
|---|---|---|
| Como esta conectada mi red | Scanopy | Todo. Es la base del fork |
| Que dispositivo se conecto ahora | NetAlertX | La fontaneria de aviso (webhook, correo, ntfy) existe pero **apagada por ser de pago**. Falta el motor de reglas |
| Que esta encendido ya mismo | WatchYourLAN | El dato ya se guarda (`last_seen`). Falta el barrido rapido y la pantalla |
| Quien consume mi ancho de banda | ntopng | **Nada**. Unico subsistema nuevo de verdad |

Tres de las cuatro son terminar algo a medias; solo la cuarta se construye desde
cero. Detalle de los conteos en [[mired-upstream-scanopy]].

## Presencia: dos ritmos, no uno
Barrido ARP corto cada 30-60 segundos que **solo responde "esta o no esta"**,
separado del escaneo profundo (puertos, servicios, SNMP, topologia), que sigue
programado y espaciado. Ese es el truco por el que WatchYourLAN es tan ligero, y
mezclar los dos ritmos es justo lo que vuelve pesadas a otras herramientas.

## Alertas: el valor no es escanear, es avisar
Motor de reglas **configurable por red** (encaja con la base por red: la sucursal
avisa a quien lleva la sucursal): MAC nueva, equipo conocido que desaparece,
equipo que cambia de IP o de puerto de switch, puerto nuevo abierto en un equipo
conocido, servicio que deja de responder, red entera que deja de reportar.
Salidas: correo, webhook, ntfy y Telegram.

## Ancho de banda: escanear NO mide trafico
Para saber quien consume hay que **ver** el trafico, y con switches cada equipo
solo ve lo suyo. Tres capas, de barata a cara:

1. **Contadores SNMP por puerto** (`ifHCInOctets`/`ifHCOutOctets`). El switch ya
   lleva la cuenta; como MiRed sabe que aparato cuelga de cada boca, eso da
   **quien consume sin capturar un solo paquete** y a costo casi nulo. Da
   volumen, no aplicaciones. → **nucleo**
2. **Flujos del router** (NetFlow/sFlow/IPFIX). El router hace el trabajo pesado.
   **Es la salida para sitios sin ningun switch administrable**: no da el puerto,
   pero el trafico a internet pasa por el router si o si. → **nucleo**
3. **Inspeccion profunda con nDPI** (la biblioteca que ntopng usa dentro):
   identifica la aplicacion concreta. Exige puerto espejo o estar en el camino
   del trafico, y **es exactamente lo que pone a ntopng en "consumo alto"**. →
   **paquete aparte `mired-dpi`**, opcional, se puede posponer sin bloquear nada.

## El consumo de recursos es requisito, no resultado
MiRed va a hacer mas que WatchYourLAN y que ntopng juntas; si no se cuida acaba
en "consumo alto" sin haber ganado nada. En firme: presencia y contadores SNMP
tienen que correr comodos en una Raspberry Pi; el escaneo profundo va programado,
nunca continuo; la DPI va en otro paquete; **se mide el consumo al cerrar cada
fase**, no al final.

## Redes mixtas (administrables y no administrables a la vez)
El usuario confirmo que unos switches lo seran y otros no. **Se disena para eso
desde el principio**: no es un si o no, degrada por capas.

**Sin un solo switch administrable se sigue sabiendo:** quien esta en la red y
quien se acaba de conectar, que es cada aparato, que servicios y puertos expone,
la topologia de capa 3, y **todas las alertas**. Se pierde: el puerto exacto y el
consumo por puerto.

**Lo que si se infiere:** si un switch administrable ve **varias MAC en una misma
boca**, ahi cuelga un switch no administrable o un punto de acceso, y todo lo que
aparece por esa boca esta detras. Da el grupo ("estos nueve equipos cuelgan del
puerto 7"), no el puerto exacto — y para un plano de sitio suele bastar. El WiFi
nunca tiene puerto de switch: ahi la hoja del arbol es el punto de acceso, y con
controladora (UniFi y similares, ya soportada arriba) se sabe que cliente esta en
que antena.

**Consecuencia de diseno: perfil de capacidades por red.** Cada red descubre y
guarda que se puede saber en ella, y la interfaz **lo dice sin rodeos en vez de
mostrar pantallas vacias**. Mas un informe por sitio de que haria falta para
subir de nivel ("en Sucursal 2 no se ve el puerto porque el switch X no es
administrable; con uno administrable ahi se cubririan estos 9 equipos"), que para
quien administra varios sitios sirve igual de diagnostico que de argumento de
compra.

**Ver tambien:** [[mired]], [[mired-arquitectura]], [[mired-upstream-scanopy]]
