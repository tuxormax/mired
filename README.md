# MiRed

Descubre los equipos de una red, dibuja su mapa y documenta **en qué puerto de
qué switch está conectado cada aparato**.

Sin topes de redes ni de usuarios. Cada red vive en su propia base de datos
SQLite, así que respaldar un sitio es copiar un archivo. No necesita servidor de
base de datos ni Docker: son dos binarios y dos servicios, en un `.deb`.

> **Estado: en construcción (fases 1 a 8 de 10).** Ya descubre los equipos de la
> red, guarda quién está encendido en cada momento, escanea solo con la
> periodicidad que se le diga, le pregunta a los switches por SNMP en qué boca
> está cada aparato, dibuja el mapa y lo exporta, avisa de lo que cambia y mide
> el consumo. Falta probarlo contra un switch administrable de verdad y publicar
> los paquetes. El plan y el avance por fase están en [PLAN.md](PLAN.md).

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
- Descubrimiento en tres capas —ARP, ICMP y TCP—, que degradan con honestidad:
  cada equipo dice con qué método se vio, y el mapa distingue el puerto
  confirmado del grupo que cuelga de un switch no administrable.
- Dos procesos separados a propósito: el que está expuesto a la red no tiene
  privilegios, y el que los tiene no toca la base de datos.

## Construir desde el código

Hace falta Go 1.24 o superior y, para la interfaz, Flutter.

```
./herramientas/construir.sh                      # .deb para amd64
./herramientas/construir.sh --arquitectura todas # amd64 y arm64
go test ./...                                    # pruebas
./herramientas/probar.sh                         # prueba de humo del paquete
```

`probar.sh` construye el `.deb`, lo desempaqueta aparte, levanta los servicios
desde ahí y recorre el flujo completo: entrar, crear una red, escanear, revisar
alertas y consumo. Es la única forma de comprobar que lo que se **entrega**
funciona, y no toca el sistema.

Para trabajar sin instalar nada:

```
MIRED_DATOS=./datos MIRED_ESCUCHA=127.0.0.1:60072 go run ./programas/mired-servidor
MIRED_SOCKET_SONDA=/tmp/sonda.sock go run ./programas/mired-sonda
```

## Documentación

- [Instalación y primeros pasos](documentacion/instalacion.md)
- [Cómo aportar un dispositivo al catálogo](documentacion/aportar-dispositivos.md)
  — no hace falta programar
- [Installation guide (English)](documentacion/installation-en.md)

## Aportar un dispositivo al catálogo

Reconocer un aparato nuevo no exige programar: es un archivo `.toml` que se copia
a `/etc/mired/dispositivos/`. La interfaz incluso lo genera solo, ya relleno con
lo que vio del equipo. Esa es la puerta de entrada para quien quiera aportar sin
saber Go.

## Licencia

MiRed es software libre bajo la **[GNU Affero General Public License v3.0](LICENSE)**.

Se eligió la Affero y no la GPL corriente porque MiRed se usa desde el navegador:
con la GPL, quien monte MiRed como servicio de pago no está obligado a publicar
nada, porque nunca entrega el binario a nadie. La AGPL cierra ese hueco —quien lo
ofrezca por red tiene que ofrecer también el código, con sus cambios—, y por eso
el pie de la interfaz lleva el enlace al código, como pide la sección 13.

Copyright (C) 2026 tuxor.

## Sobre Scanopy

MiRed nació de la idea de forkear [Scanopy](https://github.com/scanopy/scanopy),
y se decidió no hacerlo: es código propio, escrito desde cero. Scanopy se
consulta como referencia de cómo se resuelven ciertos problemas, pero **no se
copió ni una línea de su código**, y MiRed no está afiliado ni respaldado por sus
autores.
