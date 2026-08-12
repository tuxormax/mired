---
name: modulo-topologia
description: "SNMP, mapa de puertos y el perfil de capacidades por red de MiRed"
metadata: 
  node_type: memory
  type: project
  originSessionId: 20376d18-adf7-4315-bb9c-98a3aa84ec95
  modified: 2026-08-12T22:34:47.555Z
---

# Modulo: SNMP y mapa de puertos

**Que hace:** le pregunta a los switches administrables que hay conectado en cada
boca. Es lo unico que ningun barrido puede averiguar solo.

**Donde vive:** `internal/snmp/cliente.go` (corre en la sonda),
`internal/basedatos/topologia.go` (persistencia y armado),
`internal/basedatos/credenciales.go` (credenciales, en el catalogo).

## Como se consigue el dato
1. Terminado un escaneo **profundo**, el programador le pasa a la sonda las IP
   descubiertas y **todas** las credenciales SNMP del catalogo.
2. La sonda prueba las credenciales **en orden** contra cada IP y usa la primera
   que conteste. Asi nadie tiene que decir que credencial va con que switch.
3. Se consultan: sistema, tabla de interfaces, **tabla de reenvio de MAC** y
   LLDP.

## La decision honesta del proyecto
- Boca con **UNA** MAC → enlace **confirmado**: ahi cuelga ese aparato.
- Boca con **VARIAS** MAC → **grupo**: atras hay un switch no administrable o un
  punto de acceso. Se guarda `cuantos_en_boca` y `confirmado = 0`.
  **Inventarle un puerto a cada uno seria mentir**, y un plano con datos
  inventados es peor que no tenerlo.

## Perfil de capacidades por red (`ficha.capacidad_mapa_puertos`)
`exacta` (hay bocas confirmadas) · `por_grupo` (solo grupos) · `no_disponible`
(ningun switch contesto) · `desconocida` (todavia no se consulta).
**Se recalcula tambien cuando NADIE contesta**: que nadie conteste es una
respuesta, y la interfaz la explica en vez de quedarse en "desconocida".

## Detalles de SNMP que cuesta averiguar
- `dot1dTpFdbPort` da un **puerto de puente**, NO el indice de interfaz: hay que
  traducirlo con `dot1dBasePortIfIndex`.
- La MAC **no es el valor**: viene codificada en el indice del OID (6 numeros; 7
  en la variante con VLAN, donde el primero es la VLAN).
- Se intenta primero `dot1qTpFdbPort` (con VLAN) y se cae a la clasica.
- `BulkWalk` no existe en SNMPv1: ahi se usa `Walk`.
- La tabla de reenvio es una **foto**: al guardar se borran las conexiones
  anteriores de ese switch, o un equipo que se cambio de boca apareceria en dos.

## Pendiente
- **Nunca se ha probado contra un switch real.** Toda la logica esta cubierta por
  pruebas con datos armados a mano, pero SNMP en la vida real es un desorden
  (cada fabricante contesta distinto). Es el mayor riesgo abierto del proyecto.
- Falta CDP (solo hay LLDP) y la controladora WiFi tipo UniFi.

**Ver tambien:** [[modulo-escaneo]], [[mired-capacidades]], [[contrato-api]]
