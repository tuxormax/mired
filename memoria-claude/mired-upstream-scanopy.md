---
name: mired-upstream-scanopy
description: Radiografia del codigo de Scanopy (donde esta cada cosa) y licencias de las herramientas que MiRed absorbe
metadata:
  type: reference
---

# Radiografia del codigo heredado (Scanopy)

Investigado el **2026-08-12** sobre un clon superficial de
`github.com/scanopy/scanopy`. **Esto es lo que costo trabajo averiguar**: sin
esta ficha hay que volver a clonar y volver a rastrear.

## Tamano y stack
- **Backend Rust**: 762 archivos, ~159 000 lineas. axum + sqlx 0.8 (feature
  `postgres`) + tokio. Binarios `server` y `daemon` en el mismo crate.
- **Interfaz**: SvelteKit + Tailwind, compilada a estatica (`adapter-static`).
- **Despliegue actual**: `docker-compose.yml` con tres servicios (daemon
  privilegiado en modo host, postgres 17, server en el puerto **60072**).
- Ya compilan binarios nativos para Linux/macOS/FreeBSD e instalador MSI de
  Windows, y traen unidad systemd de ejemplo. **El `.deb` no parte de cero.**

## Donde esta cada cosa (rutas reales)
| Que | Donde |
|---|---|
| **El tope de 1 red / 1 asiento** | `backend/src/server/billing/plans.rs` → `get_community_plan()`: `included_seats: Some(1)`, `included_networks: Some(1)`. El plan comercial pone `None` en los mismos campos. |
| Donde se aplica el tope | `backend/src/server/auth/middleware/features.rs` (~linea 223) |
| **Toda la capa de datos** | `backend/src/server/shared/storage/` (~4 500 lineas): `generic.rs` arma el SQL, `pg_value.rs` convierte tipos, `filter/` los filtros, `migration_runner.rs` las migraciones |
| **Donde se corta para la base por red** | `backend/src/server/shared/storage/factory.rs`: construye un objeto de almacenamiento por entidad, todos con la misma conexion |
| Migraciones | `backend/migrations/` — **116 archivos, 4 233 lineas** de SQL de PostgreSQL |
| Definiciones de dispositivos | `backend/src/server/services/definitions/` — **257 archivos `.rs`**, uno por servicio, registrados con `inventory::submit!` |
| Licencia por JWT (firma EdDSA) | `backend/src/server/license/key.rs` |

## Datos duros que mandan sobre el plan
- **No hay macros `sqlx::query!`** (0 usos): el SQL es de ejecucion, no
  comprobado al compilar. Un dolor menos al portar.
- **PostgreSQL esta metido en el esquema, no en los modulos**: en migraciones hay
  371 usos de JSONB, 297 de UUID, 128 TIMESTAMPTZ, 9 INET, 9 CIDR, 5 MACADDR, 24
  `gen_random_uuid()`, 2 triggers y la extension `pgcrypto` (solo para hashear
  claves de API).
- **29 tablas.** La mayoria ya lleva `network_id` (hosts, subnets, services,
  ports, interfaces, vlans, groups, topologies, snapshots, discovery, daemons,
  bindings, shares). Globales: users, organizations, tags, credentials,
  snmp_credentials, user_api_keys y las tablas de permisos por red.
- `storage/tests.rs` trae ~1 000 lineas de pruebas del almacenamiento: **es la
  red de seguridad del puerto a SQLite**, no tocarlas mas de lo imprescindible.
- Los patrones de deteccion ya son declarativos (`Pattern::Port`,
  `Pattern::AllOf`, OUI del fabricante, consulta a un endpoint): **son datos
  disfrazados de codigo**, por eso pasarlos a `.toml` es viable con un script.

## Que existe ya de las capacidades nuevas
- **Presencia**: el dato ya se guarda (`last_seen` aparece en 286 puntos).
- **Avisos**: hay fontaneria de webhook (97), correo/SMTP (77) y ntfy (8), pero
  **apagada porque es funcion de pago** (`webhooks: false` en el plan Community).
- **Ancho de banda**: **cero**. No hay contadores de trafico (`ifInOctets`,
  `bandwidth` → 0 usos). Es el unico subsistema realmente nuevo.
- **Capa 2 ya fuerte**: LLDP (856 usos), CDP (307), tablas de MAC del switch
  (205), controladora UniFi (descubre switches, antenas, `port_table`,
  `mac_table`, uplinks). `hosts/service/topology.rs` ya distingue enlaces
  confirmados de inferidos y contempla el aparato no administrable al final.

## Licencias (verificado)
| Proyecto | Licencia |
|---|---|
| Scanopy (backend y UI) | **AGPL-3.0 pura**, sin clausulas anadidas. La licencia comercial que venden es solo para quien no quiere cumplir AGPL |
| NetAlertX | GPL-3.0 (Python/PHP) |
| WatchYourLAN | GPL-3.0 (Go) |
| ntopng | GPL-3.0 (C++) |
| nDPI | **LGPL-3.0** (C, necesita libpcap) — la unica dependencia directa viable |

GPL-3.0 y AGPL-3.0 son compatibles entre si; el resultado se distribuye como
AGPL-3.0. Pero **no hay codigo que copiar** entre ellas (Python, Go, C++ contra
Rust): se reutilizan las ideas, y las ideas no llevan licencia.

**Obligaciones que MiRed debe cumplir desde el primer commit:** publicar su
codigo bajo AGPL-3.0, poner un **enlace visible a la fuente en la interfaz**
(articulo 13), conservar los avisos de copyright y declarar que es un fork no
afiliado. La marca no se hereda: nombre, logo y dominio son de ellos, por eso el
rename es requisito legal y no cosmetica (**2 509 apariciones de "scanopy" en 370
archivos**).

**Ver tambien:** [[mired]], [[mired-arquitectura]], [[mired-capacidades]]
