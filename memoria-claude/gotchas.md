---
name: gotchas
description: Reglas transversales de MiRed que cruzan modulos y evitan errores tontos
metadata: 
  node_type: memory
  type: project
  originSessionId: 20376d18-adf7-4315-bb9c-98a3aa84ec95
  modified: 2026-08-12T21:25:43.044Z
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
- **`ON CONFLICT` contra un indice parcial exige repetir su condicion**:
  `ON CONFLICT (cidr) WHERE estatus >= 0 DO UPDATE …`.

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

## Procesos
- **Un solo escritor**: `mired-servidor`. La sonda escanea y entrega por socket.
- **La sonda comprueba de verdad si puede abrir un socket crudo**, no supone por
  el usuario: con `AmbientCapabilities` se tienen permisos sin ser root.
- El `postinst` **nunca falla**: avisa y sigue.

## Go
- El punto de entrada de Flutter es `lib/principal.dart`, **no** `main.dart`: hay
  que compilar con `-t lib/principal.dart` (ya esta en `herramientas/construir.sh`).
- `internal/` es palabra reservada del compilador de Go y por eso es la unica
  carpeta en ingles. Adentro, todo en espanol.

**Ver tambien:** [[mired-arquitectura]], [[contrato-api]], [[mired]]
