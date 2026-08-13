# Instalar MiRed

MiRed son dos programas y un archivo de configuración. No necesita servidor de
base de datos, ni Docker, ni nada instalado aparte.

## 1. Instalar el paquete

Descargue el `.deb` que corresponda a su equipo y ejecute:

```
sudo dpkg -i mired_1.0-0_amd64.deb
```

Use `arm64` en vez de `amd64` si va a instalarlo en una Raspberry Pi.

Al terminar, la instalación le dice la dirección para entrar. Es el equipo donde
lo instaló, en el puerto **60072**:

```
http://192.168.1.10:60072
```

**Usuario `usuario-quitado`, clave `clave-quitada`.** Cámbiela en cuanto entre.

## 2. Darle permisos de red a la sonda

MiRed corre en dos partes: el servidor, que no tiene privilegios, y la sonda, que
es la única que toca la red. El paquete ya le concede a la sonda los dos permisos
que necesita (`CAP_NET_RAW` y `CAP_NET_ADMIN`) a través de systemd, sin darle
acceso de administrador al resto del sistema.

Si el panel de inicio le muestra un aviso amarillo diciendo que la sonda no tiene
permisos de red cruda, revise que el servicio esté corriendo:

```
systemctl status mired-sonda
journalctl -u mired-sonda -n 50
```

Sin esos permisos MiRed **sigue funcionando**: descubre los equipos tocando
puertos TCP en vez de usar ARP. Es más lento y no obtiene la dirección MAC, y la
propia interfaz se lo dice para que no crea que está viendo la red completa.

## 3. Crear su primera red

Una «red» en MiRed es un sitio: Matriz, Sucursal 1, Bodega. Cada una vive en su
propio archivo de base de datos, así que respaldar un sitio es copiar un archivo.

Al crearla le pide las **subredes a escanear**. Normalmente es una sola, la de su
oficina:

```
192.168.1.0/24
```

Si no sabe cuál es la suya, mire la dirección IP de su computadora y cambie el
último número por `0/24`.

## 4. Escanear

En la pantalla de la red, el botón del radar ofrece dos opciones:

- **Escaneo completo**: descubre equipos, resuelve sus nombres, revisa qué
  puertos tienen abiertos y le pregunta a los switches. Tarda minutos.
- **Solo presencia**: rápido, solo dice quién está encendido ahora.

Con el botón del reloj puede dejarlos programados: presencia cada minuto y
escaneo completo cada seis horas es una buena configuración de arranque.

## 5. El mapa de puertos (opcional, pero es lo bueno)

Para saber **en qué puerto de qué switch** está conectado cada aparato, MiRed
tiene que poder preguntárselo al switch. Hacen falta dos cosas:

1. Que el switch sea **administrable** y tenga **SNMP habilitado** (en el menú de
   su switch suele estar como «SNMP» o «Community»).
2. Que le cargue esa credencial a MiRed: en el panel de inicio, el ícono de la
   llave → **Nueva credencial**. Para casi todos los switches basta con
   «SNMP v2c» y la comunidad de lectura (por omisión suele ser `public`).

Después de eso, un escaneo completo llena la pestaña **Puertos**.

Si sus switches no son administrables, MiRed se lo dice claramente en esa
pantalla y le explica qué sigue funcionando sin ellos (que es casi todo:
inventario, presencia y alertas).

### El WiFi: dé de alta su controladora

Un punto de acceso **no tiene puertos, tiene antenas**, y quien sabe qué aparato
está colgado de cuál es la controladora, no el aparato. Sin darla de alta, en una
oficina moderna la mitad de los equipos —teléfonos, portátiles, cámaras— salen
como *«sin ubicar»* en el mapa.

En el panel de inicio, el ícono de la antena → **Nueva controladora**. Necesita
la misma dirección con la que usted entra desde el navegador
(`https://192.168.1.10:8443`), un usuario —basta uno de sólo lectura— y el
*sitio*, que en casi todas se llama `default`.

Deje **apagado** «exigir certificado válido»: casi toda controladora casera usa
un certificado que ella misma se firmó, y exigirlo dejaría la función inservible.

Después de eso, cada equipo por WiFi aparece en el mapa colgando de su antena, y
la boca lleva el nombre de la red a la que se conectó (`Oficina`, `Invitados`).
Si la controladora deja de contestar, la pantalla lo dice con el motivo: el WiFi
no desaparece del mapa en silencio.

De momento se entiende **UniFi (Ubiquiti)**, que es la que hay instalada en la
mayoría de los sitios chicos.

### Guardar el mapa

En la pantalla del mapa, el ícono de la flecha hacia abajo. Cuatro formatos:

| Formato | Para qué |
|---|---|
| **PNG** | Una imagen, para pegarla en un documento o mandarla por chat |
| **SVG** | Vectorial: se abre en Inkscape y se le mueven cajas o se le anota |
| **PDF** | Para imprimirlo y colgarlo, o mandarlo por correo |
| **CSV** | Para una hoja de cálculo, con una fila por puerto |

El archivo **se baja a su equipo y nada más**. MiRed no lo sube a ninguna nube ni
lo manda a ningún lado: lo arma el navegador y cae en su carpeta de descargas,
como cualquier otra descarga. Si quiere hacérselo llegar a alguien, lo adjunta
usted.

Los tres formatos de dibujo llevan arriba de qué sitio es el mapa y de qué día,
porque un plano de red sin fecha, a la semana, ya no se sabe si es el bueno.

## 6. Medir quién consume

Hay dos formas y no se estorban:

- **Con switches administrables**: MiRed lee los contadores de cada boca. Como ya
  sabe qué aparato cuelga de cada una, eso le dice quién consume. Necesita al
  menos dos escaneos completos, porque el consumo se calcula restando dos
  lecturas.
- **Sin ellos**: configure su router o su switch para que exporte flujos al
  puerto `2055` del equipo donde instaló MiRed. En MikroTik está en
  `IP → Traffic Flow`; en pfSense, con el paquete `softflowd`; en un switch HP o
  Dell, como `sFlow`. Eso le da el consumo por equipo, aunque no diga en qué
  puerto está.

  MiRed entiende los cuatro formatos que hay, y no hay que decirle cuál es: lo
  reconoce solo.

  | Formato | Quién lo exporta |
  |---|---|
  | **NetFlow v5** | MikroTik, pfSense y casi todo router de oficina |
  | **NetFlow v9** | Cisco y equipo de gama media |
  | **IPFIX** | El estándar de IETF, cada vez más común |
  | **sFlow** | Switches HP, Dell, Brocade y de centro de datos |

  Con una diferencia que MiRed le dice en la pantalla en vez de dejarlo suponer:
  **los tres primeros cuentan el tráfico y sFlow lo estima.** sFlow no mira todos
  los paquetes, mira uno de cada N y multiplica. Sirve de sobra para saber quién
  se está llevando el ancho de banda; no sirve para facturarle a nadie. Cuando la
  cifra viene de ahí, aparece con una tilde delante (`~1.2 Gb`) y el título dice
  *«estimado por muestreo»*.

## 7. Que le avise

En la pantalla de la red, el ícono de la campana. Ahí decide qué se vigila
(equipo desconocido que se conecta, puerto nuevo abierto, equipo que lleva días
apagado...) y a dónde avisar: al teléfono con ntfy, a Telegram, por correo o a un
webhook propio.

## Dónde queda todo

| Ruta | Qué es |
|---|---|
| `/etc/mired/mired.toml` | Configuración |
| `/etc/mired/dispositivos/` | Sus definiciones de dispositivos |
| `/var/lib/mired/mired.db` | Usuarios, permisos y registro de redes |
| `/var/lib/mired/redes/` | **Una base por red. Es lo único que hay que respaldar** |

Para respaldar todo:

```
sudo tar czf mired-respaldo.tar.gz /var/lib/mired
```

## Quitarlo

```
sudo apt remove mired
```

**Los datos no se borran**, ni siquiera al purgar: quedan en `/var/lib/mired`. Si
de verdad quiere borrarlos, hágalo a mano.

## Problemas comunes

**No abre la página.** Revise que el servicio esté vivo (`systemctl status
mired-servidor`) y que el firewall del equipo deje pasar el puerto 60072.

**Escanea y no encuentra nada.** Revise la subred que capturó: tiene que ser la
de su propia red. Y mire el aviso de la sonda en el panel de inicio.

**Un equipo aparece dos veces.** Ocurre cuando MiRed lo vio una vez sin poder
obtener su MAC (por TCP) y otra con ella. Conceda los permisos de red a la sonda
y desaparece.

**Todo lo demás.** Cuando algo falla, MiRed muestra una ventana con todos los
detalles y un botón para copiarlos. Copie eso y mándelo: ahí está lo que hace
falta para arreglarlo.
