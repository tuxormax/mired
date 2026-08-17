---
name: feedback-como-trabaja-tuxor
description: "Como revisa tuxor MiRed y que pide siempre: para quien es el programa, como decide y que corrige"
metadata:
  node_type: memory
  type: feedback
---

# Como trabaja tuxor con MiRed

## Revisa MIRANDO, y manda una captura
Casi todo lo que cambio el 2026-08-17 salio de abrir el programa y senalar una
pantalla, no de leer codigo ni de repasar el plan. Consecuencia directa para
trabajar con el:

**Compilar no es haber probado.** Antes de dar por buena una pantalla, hay que
verla. Lo que funciona: una prueba de widget que dibuja la pantalla contra un
servidor de mentira, la captura a PNG con `RepaintBoundary.toImage()` —dentro de
`runAsync`, o se cuelga— y leer la imagen. El texto sale en cajitas, pero el
color, el reparto del espacio, lo que se desborda y lo que se corta se ven
perfectamente. Ahi se cazaron: la tabla del ejemplo cortada, el icono verde
amarillento que peleaba con el tema y el menu de tres lineas por fila.

## Para quien es MiRed: **las dos personas a la vez**
Lo dijo tal cual: «para alguien que tiene muy poco conocimiento de redes» **y**
«para alguien que tiene mucha experiencia». No es simplificar: es que las dos
encuentren lo suyo en la misma pantalla. La receta completa esta en [[gotchas]]
(«MiRed es para quien sabe poco de redes Y para quien sabe mucho»).

Lo que pregunta cuando algo no esta claro es literal —«¿que es eso de
community?»— y esa pregunta **es la especificacion**: la respuesta va dentro de
la pantalla, plegada, no en un manual.

## Lo que pide una y otra vez
- **Todo el ancho de la pantalla.** Sin topes de lectura: en 1 280 los 1 280, en
  1 920 los 1 920, y el contenido que se acomode.
- **Una cosa en un solo sitio.** Si algo esta repartido en tres pantallas y en su
  cabeza es una sola cosa, se junta (paso con las llaves de la red).
- **Que se pueda probar.** Un boton que diga si funciono vale mas que tres
  parrafos de ayuda.
- **En espanol y en cristiano**, tambien en los titulos: «Preguntarle a los
  switches» antes que «Credenciales SNMP».

## Como se decide
**El decide el rumbo; yo propongo.** Cuando hay dos caminos con consecuencias
distintas —una migracion frente a un cambio de pantalla, una dependencia nueva
frente a escribirlo a mano— se le presentan con su costo y **se espera**. Ha
elegido varias veces la opcion que yo no recomendaba (el 100% del ancho, las
credenciales por red) y tenia razon las dos veces: conoce el uso real.

Cuando corrige una decision mia ya tomada, se cambia sin discutir y **se corrige
tambien la regla escrita en la memoria**, que si no queda contradiciendo al
codigo.

## El cierre de cada tanda
«Actualiza el proyecto» = memoria + revision en `historial.toml` + commit + push
+ **generar los instaladores**. Ojo con el orden: los paquetes llevan grabado el
hash del commit, asi que **se construyen DESPUES de commitear** o salen
apuntando al anterior. Paso una vez.

**Ver tambien:** [[gotchas]], [[modulo-accesos]], [[mired]]
