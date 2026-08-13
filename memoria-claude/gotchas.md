---
name: gotchas
description: Reglas transversales de MiRed que cruzan modulos y evitan errores tontos
metadata: 
  node_type: memory
  type: project
  originSessionId: 20376d18-adf7-4315-bb9c-98a3aa84ec95
  modified: 2026-08-13T16:00:00.000Z
---

# Gotchas de MiRed

Reglas que valen en todo el proyecto. Una sola lectura evita la mayoria de los
errores tontos.

## Base de datos
- **`chmod` numerico NO limpia el bit setgid de una carpeta.** Hay que
  `chmod g-s` explicitamente. Mordio al armar el `.deb`: la carpeta del proyecto
  es setgid `www-data` y `dpkg-deb` rechaza el directorio de control por eso.
- **Las fechas se guardan como TEXT en ISO 8601 con zona horaria.** SQLite no
  tiene tipo fecha; el formato **es** el contrato. Usar `basedatos.Ahora()`.
- **Borrado suave con `estatus` -1/0/1** y los indices unicos **parciales**
  (`WHERE estatus >= 0`) para poder reutilizar el nombre de algo borrado.
- **`BuscarRed` no encuentra redes borradas** (`estatus >= 0` excluye -1). Para
  reactivar hay que buscar por nombre con `estatus = -1`, que es lo que hace
  `CrearRed`.
- **Migrar es idempotente y se llama al ABRIR**, no al instalar: una base
  restaurada de un respaldo viejo llega con esquema atrasado.
- **`Abrir` recibe contexto y lo respeta.** No se inventa su propio plazo: si el
  que llama trae uno, manda el suyo; si no, se le pone `EsperaAlAbrir` (30 s).
  El plazo es generoso a proposito —una Raspberry con tarjeta SD escribiendo un
  escaneo tarda— y el error distingue "tardo demasiado" de "no se pudo": son
  problemas distintos y se arreglan en sitios distintos.
- **`ON CONFLICT` contra un indice parcial exige repetir su condicion**:
  `ON CONFLICT (cidr) WHERE estatus >= 0 DO UPDATE …`.
- **NUNCA decidir "esto es nuevo" o "esto ya no esta" comparando marcas de
  tiempo.** `Ahora()` tiene resolucion de SEGUNDOS y dos escaneos seguidos en una
  red chica caen en el mismo segundo. Ya mordio DOS veces (puertos que se cierran,
  puertos nuevos). Se compara contra **la lista de lo observado** o contra el
  **numero de escaneo**, que no mienten nunca. Ver [[historial-bugs]].

## Una base por red
- **El enrutado vive solo en `internal/basedatos`.** Ningun otro paquete abre un
  archivo `.db` por su cuenta.
- **`Red()` presta y hay que devolver**: `defer devolver()`. Mejor todavia,
  `ConRed()`, que no se puede olvidar. Sin devolver, el archivo queda abierto
  para siempre.
- **Las consultas que cruzan redes no son un `SELECT`.** El panel de inicio se
  arma con el resumen guardado en el catalogo, no abriendo treinta archivos.
- **La clave de la red no cambia nunca; el nombre si.** El archivo se llama por
  la clave (`matriz-a1b2.db`). Renombrar NO mueve el archivo.

## Formularios (regla de la casa)
- **El formulario restringe exactamente lo que acepta la columna**, en Flutter y
  en el servidor. Hoy: usuario 60, nombre 120, clave >= 8, VLAN 1-4094, CIDR con
  formato. Si se cambia de un lado, se cambia del otro **el mismo dia**.
- Un campo de solo despliegue **nunca** lleva el `name` de su columna.

## Interfaz Flutter
- **`setState` con cuerpo de flecha y una asignacion revienta** si lo asignado es
  un Future: la expresion lo devuelve y hay una asercion contra eso. Usar
  siempre llaves: `setState(() { _x = algo(); });`. Solo falla en compilacion de
  **depuracion**, asi que probar en el navegador (release) NO lo detecta.
- Las pruebas de pantalla necesitan **dos destrabes**: `HttpOverrides.global`
  (el framework bloquea la red a proposito) y `probador.runAsync` (la respuesta
  del servidor llega por el reloj de verdad, no por el falso de la prueba).
- **`Picture.toImage()` en una prueba se cuelga hasta el timeout de 10 minutos si
  no va dentro de `probador.runAsync`.** No falla: se queda esperando, porque la
  conversion la hace el motor en otro hilo y el reloj falso nunca le da turno.
  Mordio al probar la exportacion del mapa a PNG.
- **Lo que solo existe en web (bajar archivos, abrir enlaces) va en un archivo
  aparte con exportacion condicional** (`servicios/descarga.dart`
  → `descarga_web.dart` si `dart.library.js_interop`, si no `descarga_generica.dart`).
  Importar `package:web` directo desde una pantalla rompe `flutter test`, que
  corre en la maquina virtual de Dart, no en un navegador.
- **No contar renglones para saber cuantos hay**: usar la cuenta que manda el
  servidor. En el mapa, una boca con 9 MAC decia "1 equipos" cuando la respuesta
  traia un renglon resumido. La cuenta buena es `cuantosEnBoca`.

## Leer paquetes y protocolos binarios
- **NetFlow y sFlow dicen los dos "version 5".** NetFlow la pone en 2 bytes y
  sFlow en 4: un datagrama de sFlow leido como NetFlow da version 0, y ese cero
  es la unica senal para distinguirlos. Es exacto, no heuristico.
- **NetFlow v9 e IPFIX necesitan la plantilla ANTES que los datos**, y la
  plantilla es **de cada exportador**: dos routers pueden numerar la 256 para
  cosas distintas. Mezclarlas no da error, da cifras equivocadas. Que lleguen
  datos antes que su plantilla es lo NORMAL al arrancar: se tiran callados.
- **En IPFIX, un campo de fabricante trae 4 bytes mas** en la plantilla. Sin
  saltarlos, todo lo que sigue se lee corrido.
- **Saltar las etiquetas de VLAN.** Un puerto espejo y un switch con VLAN mandan
  la trama con su etiqueta: sin saltarla, el tipo de protocolo se lee 4 bytes
  antes y el trafico de esa red desaparece entero, sin un solo error.
- **LLDP y CDP ponen la boca local en un lugar distinto del indice**: LLDP en la
  posicion 1 (`tiempo.puerto.vecino`) y CDP en la 0 (`ifIndex.vecino`).
  Confundirlos cuelga cada vecino de la boca equivocada: el mapa sale plausible
  y falso.
- **sFlow ESTIMA, no cuenta.** Se multiplica por la tasa de muestreo. Esa
  diferencia viaja en la columna `estimado` hasta la pantalla; una vez juntas en
  la misma tabla, ya no hay forma de volver a separarlas.
- **Nunca contar renglones para saber cuantos hay**: usar la cuenta que manda el
  servidor. En el mapa, una boca con 9 MAC decia "1 equipos".

## Procesos
- **Un solo escritor**: `mired-servidor`. La sonda escanea y entrega por socket,
  y `mired-dpi` (opcional) hace lo mismo con lo que captura.
- **La sonda comprueba de verdad si puede abrir un socket crudo**, no supone por
  el usuario: con `AmbientCapabilities` se tienen permisos sin ser root.
- El `postinst` **nunca falla**: avisa y sigue.

## Go
- El punto de entrada de Flutter es `lib/principal.dart`, **no** `main.dart`: hay
  que compilar con `-t lib/principal.dart` (ya esta en `herramientas/construir.sh`).
- `internal/` es palabra reservada del compilador de Go y por eso es la unica
  carpeta en ingles. Adentro, todo en espanol.

## Pruebas
- **Las bases de prueba van en `/dev/shm`, no en `t.TempDir()`** (helper
  `carpetaDePrueba`). Cada prueba crea dos bases con dos docenas de migraciones;
  sobre disco son cientos de sincronizaciones que no prueban nada. Con el equipo
  ocupado —una compilacion de Flutter al lado basta— eso hacia que abrir una base
  tardara mas de 10 s y **la suite fallara con "context deadline exceeded" por
  algo que no tenia que ver con lo que se estaba probando**. Reproducido y
  corregido el 2026-08-13: de 118 s con 5 fallos a 0.6 s sin ninguno.
  Ver [[historial-bugs]].
- Las pruebas de exportacion del mapa a PNG necesitan `probador.runAsync` o se
  cuelgan 10 minutos (ver seccion de Flutter).

**Ver tambien:** [[mired-arquitectura]], [[contrato-api]], [[mired]],
[[modulo-inspeccion]]
