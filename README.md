# MiRed

Descubre los equipos de una red, dibuja su mapa y documenta **en qué puerto de
qué switch está conectado cada aparato**.

Sin topes de redes ni de usuarios. Cada red vive en su propia base de datos
SQLite, así que respaldar un sitio es copiar un archivo. No necesita servidor de
base de datos ni Docker: son dos binarios y dos servicios, en un `.deb`.

> **Estado: en construcción (fase 1 de 10).** Ya funcionan los cimientos —
> servidor, base por red, autenticación, API y empaquetado—. El descubrimiento
> de equipos llega en la fase 2. El plan completo está en [PLAN.md](PLAN.md).

## Instalar

```
sudo dpkg -i mired_1.0-0_amd64.deb
```

Al terminar, entre desde el navegador a `http://<el-equipo>:60072` con el
usuario `usuario-quitado` y la clave `clave-quitada`. **Cámbiela al entrar.**

Hay paquete para `amd64` y para `arm64`, que es lo que necesita una Raspberry Pi.

## Dónde queda todo

| Ruta | Qué es |
|---|---|
| `/usr/bin/mired-servidor` | Interfaz y API. Corre sin privilegios |
| `/usr/bin/mired-sonda` | Escaneo. Único con permisos de red cruda, acotados |
| `/etc/mired/mired.toml` | Configuración |
| `/etc/mired/dispositivos/` | Definiciones de dispositivos propias |
| `/usr/share/mired/dispositivos/` | Catálogo de dispositivos del paquete |
| `/var/lib/mired/mired.db` | Catálogo: usuarios, permisos y registro de redes |
| `/var/lib/mired/redes/` | **Una base por red. Es lo único que hay que respaldar** |

## Cómo está hecho

- **Go** para el servicio: dos binarios estáticos, sin dependencias, que cruzan a
  `arm64` sin toolchain cruzado.
- **Flutter** para la interfaz, compilada a web y servida por el propio binario.
- **SQLite** para los datos, con una base independiente por cada red.
- Dos procesos separados a propósito: el que está expuesto a la red no tiene
  privilegios, y el que los tiene no toca la base de datos.

## Construir desde el código

Hace falta Go 1.24 o superior y, para la interfaz, Flutter.

```
./herramientas/construir.sh                      # .deb para amd64
./herramientas/construir.sh --arquitectura todas # amd64 y arm64
go test ./...                                    # pruebas
```

Para trabajar sin instalar nada:

```
MIRED_DATOS=./datos MIRED_ESCUCHA=127.0.0.1:60072 go run ./programas/mired-servidor
MIRED_SOCKET_SONDA=/tmp/sonda.sock go run ./programas/mired-sonda
```

## Aportar un dispositivo al catálogo

Desde la fase 6, reconocer un aparato nuevo no exige programar: es un archivo
`.toml` que se copia a `/etc/mired/dispositivos/`. Esa es la puerta de entrada
para quien quiera aportar sin saber Go.

## Sobre Scanopy

MiRed nació de la idea de forkear [Scanopy](https://github.com/scanopy/scanopy),
y se decidió no hacerlo: es código propio, escrito desde cero. Scanopy se
consulta como referencia de cómo se resuelven ciertos problemas, pero **no se
copió ni una línea de su código**, y MiRed no está afiliado ni respaldado por sus
autores.
