# MiRed

Descubre los equipos de una red, dibuja su mapa y documenta **en qué puerto de
qué switch está conectado cada aparato**.

Sin topes de redes ni de usuarios. Cada red vive en su propia base de datos
SQLite, así que respaldar un sitio es copiar un archivo. No necesita servidor de
base de datos ni Docker: se instala el `.deb` y aparece en el menú.

> **Estado: en construcción (fases 1 a 10, falta publicar).** Ya descubre los equipos de la
> red, guarda quién está encendido en cada momento, escanea solo con la
> periodicidad que se le diga, le pregunta a los switches por SNMP en qué boca
> está cada aparato, dibuja el mapa y lo exporta, avisa de lo que cambia y mide
> el consumo. Falta probarlo contra un switch administrable de verdad y publicar
> los paquetes. El plan y el avance por fase están en [PLAN.md](PLAN.md).

## Instalar

Baje el `.deb` de la [última release](https://github.com/tuxormax/mired/releases/latest)
y:

```
sudo dpkg -i mired_1.15-18_amd64.deb
sudo apt-get -f install     # sólo si faltara alguna dependencia
```

Las notas de cada release traen la huella SHA-256 del archivo. **Los paquetes
todavía no van firmados**: es lo que queda pendiente de la fase 9.

Quien compile desde el código encuentra el suyo en `instaladores/`.

Al terminar, **ábralo desde el menú de aplicaciones** buscando «MiRed», o
escribiendo `mired` en una terminal.

**Es un programa, no una página web.** Levanta lo que necesita al abrirse y lo
apaga al cerrarse: no hay servicios que habilitar ni nada que dejar corriendo.
Desde el mismo programa puede ver también el MiRed de otro equipo — la Raspberry
de una sucursal, por ejemplo.

**MiRed no trae ningún usuario ni clave de fábrica.** La primera vez que entre le
pedirá que cree el administrador con el usuario y la clave que usted elija. Las
claves se guardan con el algoritmo [TUXOR](https://github.com/tuxormax/tuxor)
sobre scrypt, que exige que el usuario o la clave empiecen o terminen con uno de
estos signos: `+ - * % ^ & | < > #`. La pantalla se lo dice mientras teclea.

Hay paquete para `amd64` y para `arm64`, que es lo que necesita una Raspberry Pi.

## Dónde queda todo

| Ruta | Qué es |
|---|---|
| `/usr/bin/mired-servidor` | Interfaz y API. Corre sin privilegios |
| `/usr/bin/mired-sonda` | Escaneo. Permisos de red cruda, acotados |
| `/usr/bin/mired-dpi` | Inspección profunda. **Viene apagada**; necesita puerto espejo |
| `/etc/mired/mired.toml` | Configuración |
| `/etc/mired/dispositivos/` | Definiciones de dispositivos propias |
| `/usr/share/mired/dispositivos/` | Catálogo de dispositivos del paquete |
| `~/.local/share/mired/` | **Sus datos, cuando lo usa como programa. Es lo que hay que respaldar** |
| `/var/lib/mired/` | Los datos cuando se deja corriendo como servicio |

## Cómo está hecho

- **Go** para el servicio: dos binarios estáticos, sin dependencias, que cruzan a
  `arm64` sin toolchain cruzado.
- **Flutter** para la interfaz: un programa de escritorio nativo, que arranca y
  detiene los servicios él mismo.
- **SQLite** para los datos, con una base independiente por cada red.
- Descubrimiento en tres capas —ARP, ICMP y TCP—, que degradan con honestidad:
  cada equipo dice con qué método se vio, y el mapa distingue el puerto
  confirmado del grupo que cuelga de un switch no administrable.
- **El cableado que ninguna herramienta puede ver, se declara a mano.** Un switch
  no administrable no tiene dirección y no existe para ningún escaneo; un módem
  de ISP casi nunca habla SNMP hacia la LAN. El mapa tiene un modo edición donde
  se dan de alta esos aparatos, se declaran sus bocas y se conectan tocándolas.
  Lo declarado se dibuja **punteado** —en pantalla y en los cuatro formatos
  exportados—, y si el equipo acaba contestando algo distinto se avisa con las
  dos versiones a la vista, sin pisar ninguna.
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

Los instaladores quedan en **`instaladores/`**, y sólo ahí.

`probar.sh` construye el `.deb`, lo desempaqueta aparte, levanta los servicios
desde ahí y recorre el flujo completo: entrar, crear una red, escanear, revisar
alertas y consumo. Es la única forma de comprobar que lo que se **entrega**
funciona, y no toca el sistema.

Para trabajar sin instalar nada:

```
MIRED_DATOS=./datos MIRED_ESCUCHA=127.0.0.1:60072 go run ./programas/mired-servidor
MIRED_SOCKET_SONDA=/tmp/sonda.sock go run ./programas/mired-sonda
```

## Quitarlo

```
sudo ./herramientas/desinstalar.sh
```

Desinstalar el paquete a secas **no basta**: `dpkg --purge` deja a propósito las
bases de datos de las redes, que es lo correcto al actualizar —nadie quiere
perder el histórico de su red por reinstalar— pero no es lo que se quiere para
empezar de cero. Este script para los servicios, purga el paquete, pregunta si
borra los datos, quita el usuario de sistema y comprueba que no quedó nada.

Con `--todo` no pregunta.

## Documentación

- [Instalación y primeros pasos](documentacion/instalacion.md)
- [Cómo aportar un dispositivo al catálogo](documentacion/aportar-dispositivos.md)
  — no hace falta programar
- [Inspección profunda: qué aplicación consume](documentacion/inspeccion-profunda.md)
  — viene incluida y apagada
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
