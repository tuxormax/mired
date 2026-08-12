---
name: mired
description: MiRed, fork libre de Scanopy para mapear redes; estado, alcance y donde vive el plan
metadata:
  type: project
---

# MiRed — fork libre de Scanopy

Proyecto propio en `/home/tuxor/www/mired`, definido el **2026-08-12**. Nacio
como una herramienta mas del repo `linux`, pero el **2026-08-12** se saco a su
propio repositorio: no es una utilidad de escritorio empaquetada como `.deb`
suelto, es un servicio con su propio ciclo de vida.
Fork de [Scanopy](https://github.com/scanopy/scanopy) (AGPL-3.0) para descubrir
equipos, dibujar el mapa de la red y saber en que puerto de que switch esta
conectado cada aparato.

**Estado: en planeacion, sin una sola linea escrita.** El plan completo esta en
`PLAN.md` (versionado en el repo). Esta memoria guarda las decisiones y lo
que costo investigar, no repite el plan.

## Por que un fork y no usar Scanopy tal cual
La edicion Community es gratis y AGPL-3.0 pura, pero topa en **1 red y 1
asiento**, y el usuario necesita **muchas redes** (una por sitio o cliente). El
tope es una constante en codigo abierto, no un servidor de licencias: quitarlo es
legal y trivial. Lo caro es lo demas.

## Los cuatro cambios que definen el fork
1. **Sin topes** de redes ni usuarios.
2. **SQLite en vez de PostgreSQL**, para que quepa entero en un `.deb`.
3. **Una base de datos por red** (Matriz, Sucursal 1...), no una para todas.
4. **Catalogo de dispositivos en archivos `.toml`** que la comunidad amplia sin
   recompilar — es el diferenciador y la apuesta para que el fork atraiga gente.

Y ademas absorber lo mejor de tres herramientas mas: presencia en vivo estilo
WatchYourLAN, alertas de cambio estilo NetAlertX y consumo de ancho de banda
estilo ntopng. Detalle en [[mired-capacidades]].

## Tiempos acordados
- **4-5 meses** para la primera version completa. La inspeccion profunda de
  paquetes va aparte y se puede posponer.
- **Al terminar la fase 3 ya hay un `.deb` utilizable** (mapa + redes ilimitadas
  + base por red). Se saca y se usa en produccion propia mientras se construye el
  resto, en vez de esperar cinco meses a estrenar.

## Lo que falta decidir / hacer
- **Fase 0 pendiente**: instalar Scanopy Community con Docker contra una red real
  (de preferencia mixta) para inventariar que switches son administrables. Docker
  **no esta instalado** en el equipo del usuario.
- El usuario confirmo que **unos switches seran administrables y otros no**: el
  diseno contempla degradacion por capas, no es un si o no.

## Excepcion a la regla de nombrar en espanol (a proposito)
Los proyectos de tuxor nombran todo en espanol. MiRed **no puede por dentro**: son 159 000
lineas heredadas en ingles y traducir identificadores cerraria para siempre la
puerta a integrar los cambios de arriba. El espanol manda en empaquetado,
scripts, configuracion, formato del catalogo y documentacion.

**Ver tambien:** [[mired-arquitectura]], [[mired-capacidades]],
[[mired-upstream-scanopy]]
