---
name: mired-arquitectura
description: "Arquitectura de MiRed: dos binarios Go, Flutter web, una base SQLite por red, catalogo .toml y empaquetado .deb"
metadata: 
  node_type: memory
  type: project
  originSessionId: 20376d18-adf7-4315-bb9c-98a3aa84ec95
  modified: 2026-08-12T20:56:19.900Z
---

# MiRed — decisiones de arquitectura

Acordadas el **2026-08-12**. El desarrollo detallado esta en `PLAN.md`; aqui
queda **la decision y su porque**.

## 1. Dos procesos, no uno
- **`mired-servidor`**: usuario de sistema `mired`, **sin privilegios**. Sirve la
  API y la interfaz, y es el **unico que escribe en las bases**.
- **`mired-sonda`**: la unica que necesita red cruda (ARP, sondeos, escucha
  LLDP). Recibe `CAP_NET_RAW` y `CAP_NET_ADMIN`, **nunca root entero**, y **no
  toca la base**: escanea y entrega el resultado por socket Unix local.

**Por que:** separar privilegios al principio es barato; retroajustarlo con
veinte modulos escribiendo a la base es carisimo. Y como hay **un solo
escritor**, desaparecen de raiz los problemas de concurrencia de SQLite.

## 2. Una base de datos por red (la decision estructural)
Al crear una red se crea **su propio archivo SQLite** con todo lo operativo. Ni
una tabla compartida.

- **Catalogo** (`/var/lib/mired/mired.db`, uno): usuarios, sesiones, claves de
  API, permisos de quien ve que red, credenciales SNMP reutilizables y el
  registro de redes con **resumen por red** (para que el panel de inicio no abra
  treinta archivos).
- **Red** (`/var/lib/mired/redes/matriz-a1b2.db`, una por red): equipos,
  subredes, servicios, puertos, interfaces, VLAN, topologias, presencia, reglas
  de alerta y trafico. Cada archivo lleva dentro su ficha de red → es
  **autodescriptivo**: se copia a otra instalacion y se sabe que es.

**Por que:** respaldar un sitio = copiar un archivo; borrar una red = borrarlo;
una base corrupta se lleva una red, no el sistema; y el limite de "una escritura
a la vez" de SQLite es **por archivo**.

**Nombre del archivo:** `nombre-corto-<id>.db`. `matriz.db` a secas se rompe con
acentos, espacios, cambios de nombre y dos redes que se llamen igual. El nombre
bonito vive dentro y se puede cambiar cuando sea.

**Lo que cuesta, y hay que vigilar SIEMPRE:**
- Las consultas que **cruzan redes** (buscar una MAC en todos los sitios, panel
  general) no son un `SELECT`: son recorrido de archivos o `ATTACH`. Por eso el
  resumen por red vive en el catalogo.
- Las migraciones se aplican **a cada archivo**, incluso a uno viejo restaurado
  de un respaldo: al abrir una base se comprueba su version antes de tocarla.
- No se pueden tener cien bases abiertas: juego de conexiones que cierra las que
  llevan rato sin usarse.
- **El enrutado de conexiones vive solo en `internal/basedatos`.** Si se reparte
  por los modulos, cada funcion nueva tendra que acordarse de a que archivo
  escribe, y tarde o temprano una no se acordara.

## 3. Catalogo de dispositivos en `.toml` (el diferenciador)
Reconocer "esto es una impresora HP, esto una camara Hikvision" es lo que
convierte una lista de IP en un inventario. Ese reconocimiento **no se escribe en
codigo**: un `.toml` por dispositivo (nombre, categoria, icono, puertos, prefijo
MAC del fabricante, ruta HTTP y respuesta esperada, cadena SNMP).

Se cargan de `/usr/share/mired/dispositivos/` (los del paquete) y
`/etc/mired/dispositivos/` (los del usuario, que mandan). Agregar uno = copiar un
archivo y reiniciar. En la interfaz, todo equipo sin identificar ofrece
**"proponer definicion"**, que genera el `.toml` ya relleno con lo que se vio.

**Por que importa:** es la parte donde alguien puede aportar **sin saber Go**, y
por lo tanto la unica apuesta real contra que esto lo mantenga una sola persona.

## 4. Estructura del repo — todo en espanol
Unica excepcion: la carpeta `internal/`, cuyo nombre el compilador de Go
interpreta literalmente para impedir que otros proyectos importen esos paquetes.
Adentro, todo vuelve al espanol.

`programas/mired-servidor/`, `programas/mired-sonda/`, `internal/basedatos/`,
`internal/configuracion/`, `internal/autenticacion/`, `internal/api/`,
`internal/escaneo/`, `internal/snmp/`, `internal/topologia/`,
`internal/catalogo/`, `internal/alertas/`, `internal/trafico/`, `interfaz/`
(Flutter), `catalogo/dispositivos/`, `empaquetado/`, `herramientas/` (Python),
`documentacion/`.

## 5. Empaquetado: un solo `.deb`
Binarios en `/usr/bin/`, interfaz compilada en `/usr/share/mired/web/`, catalogo
en `/usr/share/mired/dispositivos/`, bases en `/var/lib/mired/`, config en
`/etc/mired/mired.toml`, dos unidades systemd.

- El servidor corre como usuario de sistema `mired`, **no root**.
- La sonda con capacidades acotadas, no root. Mismo criterio que
  [[proyecto-patron-gui-root]].
- El `postinst` crea el catalogo, habilita servicios y **nunca falla**, igual que
  [[driver-tickets]].
- Compilar para `amd64` y `arm64`: la Raspberry Pi es el equipo natural para
  dejar esto encendido.

**Ver tambien:** [[mired]], [[mired-capacidades]], [[mired-upstream-scanopy]]
