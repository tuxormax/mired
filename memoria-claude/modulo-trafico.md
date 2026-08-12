---
name: modulo-trafico
description: Como MiRed mide el ancho de banda: contadores del switch y flujos del router
metadata:
  type: project
---

# Modulo: ancho de banda

**Que hace:** responde "quien se esta comiendo el internet" **sin capturar un
solo paquete** y sin ponerse en el camino del trafico.

**Donde vive:** `internal/snmp/` (lectura de contadores), `internal/flujos/`
(receptor NetFlow), `internal/basedatos/topologia.go` (calculo y consulta),
`internal/programador/flujos.go` (reparto por red).

## Dos formas, para dos clases de red
| | Contadores SNMP | Flujos del router |
|---|---|---|
| Necesita | switch administrable | router que exporte NetFlow |
| Da | consumo **por boca**, cruzado con quien cuelga de ella | consumo **por equipo** |
| No da | trafico que no pasa por ese switch | en que puerto esta el equipo |
| Costo | casi nulo | casi nulo |

**Las dos se muestran juntas** en la pestana de Consumo, separadas por origen: en
un sitio sin switches administrables la segunda es lo unico que hay.

## Contadores SNMP: la resta es el dato
`ifHCInOctets`/`ifHCOutOctets` son **acumulados desde que el switch encendio**. El
dato util es la resta entre dos lecturas dividida por el tiempo. Por eso:
- **La primera muestra nunca tiene tasa.** Con una sola lectura no hay nada que
  restar, e inventar un numero ahi seria mostrar algo que nadie midio.
- **Una resta negativa se descarta**: significa que el contador se reinicio (el
  switch se reinicio, o el de 32 bits dio la vuelta). Tratarla como positiva
  pintaria un pico enorme que nunca ocurrio.
- Se prefieren los contadores de **64 bits**: los de 32 se desbordan en minutos
  en un puerto gigabit.
- Las muestras se podan a los 90 dias: 48 bocas cada 6 horas son 7 000 renglones
  al mes por switch, y esto tiene que correr en una Raspberry.

## Flujos: NetFlow v5, un puerto para todas las redes
El router no sabe nada de MiRed: manda lo que ve. **A que red pertenece cada
direccion se resuelve aqui**, con el mapa de subredes de todas las redes.
- Un flujo entre dos equipos locales suma a los dos: para el que manda es subida
  y para el que recibe es bajada.
- El trafico de afuera a afuera **no se cuenta**: inflaria el consumo con trafico
  que no es de nadie del sitio.
- Los flujos son del periodo, **no acumulados**: aqui no hay que restar. El corte
  limpia la cuenta cada minuto.
- **El mapa se recarga al crear una red o una subred**, no solo cada cinco
  minutos: sin eso, quien acaba de configurar su router pensaria que no funciona.
- Formato fijo v5 (MikroTik, pfSense y casi todo router de oficina). v9, IPFIX y
  sFlow usan plantillas y quedan pendientes.

**Ver tambien:** [[modulo-topologia]], [[modulo-escaneo]], [[gotchas]]
