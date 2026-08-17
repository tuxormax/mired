---
name: modulo-accesos
description: "La lista unica de llaves de una red: SNMP, controladora WiFi y la clave del panel de cada aparato, en una sola pantalla"
metadata:
  node_type: memory
  type: project
---

# Accesos y contrasenas de una red (Rev 48)

## Que es
**Una** pantalla con todas las llaves de una red, y cada renglon dice **de que
aparato es**. Se llega por el menu de la red → «Accesos y contrasenas», y
tambien desde la ficha de un aparato, que es la misma lista por otra puerta.

## Por que existe
Habia **tres sitios distintos para lo mismo**, y eso lo dijo el usuario mirando
la pantalla: «¿no seria mejor un solo modulo y especificar para que dispositivo
es? asi ya parece el listado de credenciales que se tienen para esa red».

| Antes | Ahora |
|---|---|
| Menu de la red → Credenciales SNMP | un renglon, «Todos los aparatos» |
| Menu de la red → Controladoras WiFi | un renglon con su nombre |
| Ficha del aparato → Conexiones y credenciales | un renglon por aparato |

## Lo que NO se toco
**Por dentro se siguen guardando en tres tablas.** Tienen formas distintas de
verdad —la controladora lleva direccion, sitio y si se exige certificado; la de
SNMP v3 lleva dos protocolos y dos claves— y unificarlas seria migrar tres
tablas con contrasenas cifradas dentro para arreglar algo que era **de
presentacion**. Lo unificado es lo que se ve y lo que se piensa. Si algun dia se
juntan las tablas, nadie lo notara.

## La fila rara, y por que es rara
La de SNMP **no es de un aparato**, a proposito: se prueba contra todos y se usa
la que conteste, porque cuando se captura todavia no se sabe cuales son switches
administrables —eso es justo lo que MiRed va a averiguar con ella—. Sale como
**«Todos los aparatos»** con la etiqueta «el que conteste»: sin decirlo, el
renglon se lee como si le faltara el nombre.

En el alta, elegir «Todos» deja **solo** SNMP como forma de entrar; elegir un
aparato concreto deja las otras (su pagina, SSH, consola, app del fabricante, o
«es la controladora del WiFi»).

## El orden del alta
«¿De que aparato es?» → «¿Como se entra?» → los campos que toquen. Ese orden es
el punto: antes habia que saber **de antemano** en cual de las tres pantallas
estaba lo que se queria guardar, y eso solo lo sabe quien ya conoce el programa
por dentro.

## Donde vive
| Que | Donde |
|---|---|
| La lista y el alta en dos pasos | `interfaz/lib/pantallas/accesos.dart` |
| El dialogo de SNMP (con «Probar ahora») | `interfaz/lib/pantallas/credenciales.dart` |
| El dialogo de la controladora | `interfaz/lib/pantallas/controladoras.dart` |
| El dialogo de la clave de un aparato | `interfaz/lib/pantallas/topologia_manual.dart` |

Los tres dialogos son publicos y **no cambiaron**: la pantalla nueva los abre
tal cual. Fue lo que permitio unificar sin tocar nada de lo que ya funcionaba.

**Ver tambien:** [[modulo-inalambricos]], [[modulo-topologia]], [[contrato-api]],
[[gotchas]]
