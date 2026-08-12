---
name: contrato-api
description: "Contrato entre el servidor Go, la interfaz Flutter y la sonda: sobre de respuesta, rutas y errores"
metadata: 
  node_type: memory
  type: project
  originSessionId: 20376d18-adf7-4315-bb9c-98a3aa84ec95
  modified: 2026-08-12T21:25:25.832Z
---

# Contrato de datos de MiRed

Definido en la **fase 1 (2026-08-12)**. Tres capas hablan entre si: interfaz
Flutter ↔ API HTTP ↔ servidor, y servidor ↔ sonda por socket Unix.

## 1. El sobre — TODA respuesta de la API
```
{ "ok": true,  "datos": ... }
{ "ok": false, "error": { …bloque de detalles… } }
```
La interfaz **nunca** revisa codigos HTTP a mano: `Api._pedir` convierte `ok:false`
en una excepcion `ErrorMiRed` con el bloque completo. Codigo en
`interfaz/lib/servicios/api.dart`.

## 2. Autenticacion — dos vias, la misma sesion
- **Cookie** `mired_sesion` (la pone el servidor al entrar).
- **`Authorization: Bearer <token>`** — es la que usa la interfaz, guardada en
  `shared_preferences`. Se eligio Bearer sobre cookie porque **funciona igual en
  desarrollo con recarga en caliente** (origen distinto) y en produccion.

La sesion **cuenta desde el ultimo uso**, no desde que se entro: cada peticion
valida renueva el vencimiento.

## 3. Rutas
| Metodo y ruta | Quien puede | Que hace |
|---|---|---|
| `GET /api/estado` | cualquiera | version del servicio (comprobar que esta vivo) |
| `POST /api/sesion` | cualquiera | entrar; devuelve token + usuario |
| `GET /api/sesion` | con sesion | quien soy + permisos por red + version |
| `DELETE /api/sesion` | con sesion | salir |
| `GET /api/sonda` | con sesion | si la sonda vive y si tiene red cruda |
| `GET/POST /api/redes` | sesion / superadmin | listar y crear redes |
| `GET/PATCH/DELETE /api/redes/{clave}` | permiso sobre esa red | ver, renombrar, borrar |
| `GET/POST /api/redes/{clave}/subredes` | lectura / escritura | subredes de la red |
| `GET/POST /api/usuarios`, `DELETE /api/usuarios/{id}` | superadmin | usuarios |
| `PUT/DELETE /api/usuarios/{id}/permisos/{red}` | superadmin | permisos por red |
| `POST /api/log-error` | **sin sesion a proposito** | bitacora de errores de la interfaz |

`/api/log-error` no exige sesion porque **el error que mas importa es el que
ocurre al entrar**; exigir sesion dejaria fuera justo esos.

## 4. La red activa
`conRed()` (en `internal/api/api.go`) comprueba permiso, comprueba que la red
existe y **fija la red activa en el contexto de la peticion**. De ahi en adelante
la capa de datos resuelve sola a que archivo `.db` va. **Ninguna funcion recibe
la red por parametro.** Si nadie la fija, no hay red activa — y eso es a
proposito: mejor que falle a que escriba en el archivo equivocado.

## 5. Bloque de errores
Campos: `id` (ERR-XXXXXXXX), `momento`, `modulo`, `accion`, `causa`, `ruta`,
`tabla`, `endpoint`, `red`, `usuario`, `version`, `build`, `so`, `estado`,
`mensaje` (lenguaje llano), `error` (crudo del sistema).

**Que se guarda y que no:** los errores de causa `Validacion`, `Permiso` y
`Sesion` se responden pero **NO** se guardan en `log_errores` — son uso normal y
ensuciarian la bitacora. Todo lo demas si se guarda, con `context.WithoutCancel`
para que quede anotado aunque el usuario cancele.

**Que muestra la interfaz:** `esDeCaptura` (Validacion/Permiso) → aviso corto
abajo. Lo demas → modal bloqueante que exige copiar antes de cerrar.

## 6. Contrato servidor ↔ sonda
Socket Unix `/run/mired/sonda.sock`, permisos 0660, una linea JSON por mensaje:
`{tipo, datos}` → `{ok, datos, error}`. Unica orden hoy: `estado`.
**La sonda nunca escribe en la base**: entrega y el servidor guarda.

**Ver tambien:** [[mired-arquitectura]], [[gotchas]], [[mired]]
