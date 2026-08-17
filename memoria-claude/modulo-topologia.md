---
name: modulo-topologia
description: "SNMP, mapa de puertos y el perfil de capacidades por red de MiRed"
metadata: 
  node_type: memory
  type: project
  originSessionId: 20376d18-adf7-4315-bb9c-98a3aa84ec95
  modified: 2026-08-13T16:00:00.000Z
---

# Modulo: SNMP y mapa de puertos

**Que hace:** le pregunta a los switches administrables que hay conectado en cada
puerto. Es lo unico que ningun barrido puede averiguar solo.

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
- Puerto con **UNA** MAC → enlace **confirmado**: ahi cuelga ese aparato.
- Puerto con **VARIAS** MAC → **grupo**: atras hay un switch no administrable o un
  punto de acceso. Se guarda `cuantos_en_puerto` y `confirmado = 0`.
  **Inventarle un puerto a cada uno seria mentir**, y un plano con datos
  inventados es peor que no tenerlo.

## Perfil de capacidades por red (`ficha.capacidad_mapa_puertos`)
`exacta` (hay puertos confirmados) · `por_grupo` (solo grupos) · `no_disponible`
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
  anteriores de ese switch, o un equipo que se cambio de puerto apareceria en dos.

## El mapa y su exportacion (interfaz)
El calculo de posiciones y el pintor viven en
`interfaz/lib/pantallas/mapa_plano.dart`, **aparte de la pantalla**, porque los
comparten el dibujo y la exportacion: si cada uno calculara lo suyo, el archivo
guardado y la pantalla podrian discrepar.

`interfaz/lib/servicios/exportar_mapa.dart` genera los formatos. El PNG lo dibuja
Flutter con el mismo pintor; el **SVG y el PDF se escriben a mano**, sin
biblioteca de terceros —son cajas, lineas y texto, la parte facil de los dos
formatos—. El PDF usa las fuentes base 14 (Helvetica), asi que no incrusta
fuentes y pesa unos 4 KB.

### La hoja de calculo son DOS tablas (cambiado el 2026-08-17)
No una. Cada tabla tiene **un solo sujeto**, y esa es toda la idea:

| Hoja | Un renglon es | Columnas |
|---|---|---|
| **Aparatos** | un aparato | Aparato · Que es · IP · MAC · Ultimo barrido · Conexion · Cuelga de · Puerto · Que tan seguro · Como se supo |
| **Conexiones** | una conexion | De · Por · A · Entra por · Estado · Velocidad · Como se supo |

Reglas que no se pueden romper sin volver al problema de origen:
- **Cada cable UNA vez.** Se apunta el `id` del cable al emitirlo. Recorrer
  puertos lo sacaba dos veces —las dos puntas son puertos— y nadie podia saber
  que «dvr → switch» y «switch → dvr» eran el mismo.
- **Los puertos se llaman como en el mapa**: `LAN 3`, `WAN 1`, via
  `nombreDePuerto`. Nunca «puerto 3».
- **Lo inalambrico va en Conexiones**, aunque la hoja se llame asi: el WiFi no
  tiene puertos y una tabla que recorriera puertos lo perderia entero.
- **Los puertos libres y lo sin ubicar salen**, diciendolo en su columna. Una
  hoja que calla lo que no supo se lee como si estuviera completa.
- **Las dos tablas salen del mismo `ArbolDeclarado` que dibuja el plano**, y en
  el orden en que el mapa se lee. Por eso la clase es publica.

Cuidado con la raiz: un aparato sin padre en el arbol **solo cuelga de un cable
que le APUNTE**. Un cable que sale de el lo cuelga a el del otro, no al reves;
tomarlo por bueno ponia al modem colgando del switch que alimenta.

`interfaz/lib/servicios/hoja_calculo.dart` las entrega en tres formatos —**ODS y
XLSX con una pestana por tabla**, CSV con las dos seguidas— y
`interfaz/lib/servicios/zip.dart` arma el ZIP a mano (metodo «guardado», sin
comprimir, con su CRC-32), porque un ODS y un XLSX son un ZIP con XML dentro. El
CSV sale con marca de codificacion o Excel rompe los acentos. Caso de prueba
fijo: `interfaz/test/red_de_casa.dart`.

Todo se arma **dentro del programa** y se guarda en el equipo: no sube a ninguna
nube ni pasa por el servidor. Ver [[mired]].

### Donde se guarda lo exportado (cambiado el 2026-08-17)
Lo elige **el usuario**, en el cuadro de guardar del escritorio. Antes se
escribia derecho en la carpeta de descargas y solo se avisaba de la ruta:
funcionaba, pero obligaba a ir a buscar el archivo y moverlo a mano al sitio
donde de verdad iba (la carpeta del cliente, del sitio).

`interfaz/lib/servicios/descarga_generica.dart` abre el cuadro con
**`file_selector`**, el plugin oficial del equipo de Flutter (BSD, sin
contagio de licencia). Es la **unica dependencia de terceros de la exportacion**
—el SVG y el PDF se siguen escribiendo a mano— y se admitio porque un cuadro de
guardar nativo no se escribe a mano: es el dialogo GTK del sistema, con los
marcadores del usuario y la confirmacion de sobrescritura incluidos. Viaja en el
`.deb` como `usr/share/mired/escritorio/lib/libfile_selector_linux_plugin.so`.

Tres detalles que ya estan resueltos y conviene no volver a romper:
- **Cancelar devuelve cadena vacia, no excepcion.** Cerrar el cuadro sin guardar
  no es un error y no debe acabar en el modal de problema.
- **La extension se repone** si el usuario la borra al teclear el nombre: GTK no
  la pone sola y el archivo quedaria sin abrir con nada.
- **El titulo del cuadro sale en ingles** («Save File»): lo lleva escrito el
  plugin y la API no deja cambiarlo. Lo unico de ese cuadro que decide MiRed es
  la etiqueta del filtro, que si va en espanol.

## Pendiente
- **Nunca se ha probado contra un switch real.** Toda la logica esta cubierta por
  pruebas con datos armados a mano, pero SNMP en la vida real es un desorden
  (cada fabricante contesta distinto). Es el mayor riesgo abierto del proyecto.
- Falta CDP (solo hay LLDP) y la controladora WiFi tipo UniFi.

**Ver tambien:** [[modulo-escaneo]], [[modulo-topologia-manual]], [[mired-capacidades]], [[contrato-api]]
