---
name: modulo-inalambricos
description: "Los modulos 18 y 19: quien cuelga de que antena por el aire, y las credenciales guardadas en cada equipo"
metadata:
  node_type: memory
  type: project
  modified: 2026-08-15T00:00:00.000Z
---

# Lo que cuelga por el aire, y como se entra a cada aparato

Dos modulos que nacieron juntos porque se necesitan: **la credencial que se
guarda para no buscarla en un papel es la misma con la que MiRed entra al
aparato a preguntarle quien tiene colgando.**

## El WiFi no tiene puertos

Esa frase es todo el diseno del modulo 18:

- De una antena cuelgan **uno o VARIOS** equipos, y no hay un puerto que
  declarar por cada uno. Por eso `enlaces_inalambricos` va en tabla APARTE de
  `enlaces_fisicos`: meterlo ahi obligaria a inventarle a cada antena un puerto
  por cada telefono que se conecte.
- Un equipo cuelga de **UNA** antena a la vez. Eso no es una regla de MiRed, es
  como funciona el WiFi, y por eso el indice unico es por equipo: reconectar
  reemplaza.
- En el mapa se dibuja con **puntos finos**, distinto de las tres lineas del
  cable (llena = confirmado, punteada = grupo inferido, guion largo = declarado).
- Una antena es cabecera de bloque **aunque no tenga ni un puerto declarado**.
  Si se tratara como hoja se dibujaria dos veces.

## Los cuatro caminos para saberlo, de mas firme a menos

| Origen | Quien lo dijo | Que hace falta |
|---|---|---|
| `controladora` | la controladora WiFi (UniFi) | tener controladora |
| `snmp` | la propia antena, en su tabla de asociados | SNMP encendido en la antena |
| `panel` | el panel del aparato, entrando con su credencial | credencial guardada |
| `manual` | una persona que sabe de que antena cuelga | nada |

Lo MEDIDO pisa lo declarado —la antena sabe quien esta asociado ahora mismo— y
lo declarado NO se borra porque un escaneo no vea nada.

## Los lectores de panel — `internal/lectores`

Un marco, no un lector suelto: cada marca publica su tabla a su manera.

| Lector | Que lee | Confianza |
|---|---|---|
| **AirOS** | `/sta.cgi`, `/api/stations`, `/status.cgi` de Ubiquiti | alta |
| **MikroTik** | API REST de RouterOS 7: registro inalambrico + arrendamientos DHCP | alta |
| **TR-064** | la tabla `Hosts` de los **modems del proveedor** (Telmex, Totalplay, izzi) | estandar, aunque algunos proveedores lo apagan |
| **Generico** | entra al panel y saca **las MAC que encuentre** | poco, pero cierto |

Se prueba primero el que reconoce la marca por las pistas (fabricante, modelo y
huella del equipo). El generico va siempre al final.

## La regla que evita dibujar una red falsa

**Lo inalambrico cuelga; lo de cable, NO.**

Un panel lista las dos cosas juntas, pero no valen lo mismo:

- Un cliente asociado a un radio cuelga de ESE radio: en el aire no hay switches
  en medio.
- Lo que el modem ve "por cable" es **toda su LAN**, incluido lo que esta detras
  del switch. Colgarlo del modem dibujaria una red plana donde hay un arbol.

Por eso lo de cable no arma topologia: se aprovecha solo para poner **nombre** a
los equipos (el que el propio equipo pidio al conectarse), como una huella mas.

## Como se administra todo esto en la interfaz

Tocar un aparato —en la lista o en el mapa— abre **su ficha**, nunca un menu de
acciones: en el mapa un clic es para MIRAR, y quitar un cable por accidente
mientras se consulta la red estaba a un toque de distancia.

La ficha (`interfaz/lib/pantallas/equipo.dart`) tiene pestanas:

1. **Propiedades** — lo que se sabe del aparato, incluida su huella. Lo dibuja
   `PropiedadesDelEquipo`, que comparten la ficha y la tarjeta de la lista: un
   solo sitio donde se decide como se ve un aparato.
2. **Conexiones** — que cuelga de el y **por que medio**. Por cable va puerto por
   puerto, porque un puerto lleva un cable; por WiFi va una lista sin numeros, a
   la que se le agregan uno o VARIOS equipos de una vez. Arriba del todo, de que
   cuelga EL, que es lo que orienta.
3. **Credenciales** — **solo si el aparato se puede administrar**. Un switch no
   administrable no tiene direccion, asi que no hay panel al que entrar:
   ofrecerle un formulario de usuario y clave seria ofrecer algo que no existe.
   La regla vive en `Equipo.puedeAdministrarse`.

En el mapa, tocar un **puerto libre** sigue ofreciendo conectar algo: es el
camino corto y no borra nada.

## Las credenciales — modulo 19

- Se guardan por equipo y por tipo (`web`, `ssh`, `consola`, `app`, `otro`).
- **La clave va cifrada** (`internal/secreto`), y **la llave vive con la
  configuracion, no con los datos**: un respaldo de la base copiado a cualquier
  lado no trae con que abrirse. Contra alguien sentado en la maquina con el
  usuario del servicio esto NO protege, y no se dice que si.
- La clave **no viaja en ningun listado**. Solo por `GET .../credencial/clave`,
  que exige escritura y **anota en la bitacora quien la pidio**.
- **Nunca sale en un mapa exportado.** La exportacion no lee esa tabla.
- Guardar con la clave en blanco **no la borra**: significa "no la toques".

## Antes de tocar esto
- Si se agrega un lector nuevo, va en `Registrados()` y su `Reconoce` tiene que
  ser especifico: un lector que reconozca de mas le habla en el idioma
  equivocado a media red.
- Un conectado cuya MAC no esta en el inventario **no se crea**: aparecera en el
  proximo barrido. Crearlo aqui seria inventar un aparato del que solo se sabe
  que alguien lo menciono.

**Ver tambien:** [[modulo-reconocimiento]] · [[modulo-topologia-manual]] ·
[[modulo-aire]] · [[contrato-api]] · [[gotchas]]
