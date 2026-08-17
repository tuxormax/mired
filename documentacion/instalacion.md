# Instalar MiRed

MiRed son dos programas y un archivo de configuración. No necesita servidor de
base de datos, ni Docker, ni nada instalado aparte.

## 1. Instalar el paquete

Descargue el `.deb` que corresponda a su equipo y ejecute:

```
sudo dpkg -i mired_1.1-8_amd64.deb
```

Use `arm64` en vez de `amd64` si va a instalarlo en una Raspberry Pi.

Al terminar, la instalación le dice la dirección para entrar. Es el equipo donde
lo instaló, en el puerto **60072**:

```
http://192.168.1.10:60072
```

**MiRed no trae ningún usuario ni clave de fábrica.** La primera vez que entre,
en vez del formulario de acceso verá el de crear el administrador: elija usted el
usuario y la clave. Desde ese usuario se dan de alta los demás.

Unas credenciales iguales en todas las instalaciones serían, en un proyecto
público, una puerta que cualquiera puede buscar: bastaría rastrear el puerto y
probarlas. Que las elija usted desde el primer momento es la única forma de que
no se queden sin cambiar.

Un detalle del algoritmo con el que se guardan las claves
([TUXOR](https://github.com/tuxormax/tuxor)): **el usuario o la clave deben
empezar o terminar con uno de estos signos**

```
+  -  *  %  ^  &  |  <  >  #
```

Por ejemplo `+admin` como usuario, o `*miclave#` como clave. La pantalla se lo
va diciendo mientras teclea y no deja continuar hasta que se cumple.

En cuanto existe el primer administrador, esa pantalla desaparece para siempre:
nadie de fuera puede crearse otro.

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

Al crearla le pregunta **qué revisar**, y normalmente no hay que escribir nada:
MiRed detecta su red y la propone ya marcada —*«Toda mi red: se revisarán 254
direcciones, de la 192.168.1.1 a la 192.168.1.254»*—. Le da a **Crear** y listo.

Si quiere acotar, elija *«Sólo una parte, u otra red»* y escríbalo como le salga
natural:

```
192.168.1.10 - 192.168.1.50      de aquí hasta aquí
192.168.1.100                    una dirección: se toma su red entera
192.168.1.0/24                   la notación de siempre, para quien la conozca
```

Debajo le va diciendo lo que entendió —cuántas direcciones y cuáles— **antes** de
crear nada.

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
2. Que le cargue esa contraseña a MiRed: en la pantalla de **esa red**, menú de
   la izquierda → **Accesos y contraseñas** → *Nuevo acceso* → «Todos». Para casi todos los
   switches basta con «SNMP v2c» y la comunidad de lectura (por omisión suele ser
   `public`).

**Las credenciales son de cada red y no se comparten**: la comunidad de un
cliente no se prueba nunca contra los switches de otro. Si lleva varias
instalaciones, cada una tiene las suyas.

Después de eso, un escaneo completo llena la pestaña **Puertos**.

Si sus switches no son administrables, MiRed se lo dice claramente en esa
pantalla y le explica qué sigue funcionando sin ellos (que es casi todo:
inventario, presencia y alertas).

### El WiFi: dé de alta su controladora

Un punto de acceso **no tiene puertos, tiene antenas**, y quien sabe qué aparato
está colgado de cuál es la controladora, no el aparato. Sin darla de alta, en una
oficina moderna la mitad de los equipos —teléfonos, portátiles, cámaras— salen
como *«sin ubicar»* en el mapa.

En la pantalla de **esa red**, menú de la izquierda → **Accesos y contraseñas** →
*Nuevo acceso*, elija la controladora como aparato y «Es la controladora del
WiFi». Es de esa red y de ninguna otra. Necesita
la misma dirección con la que usted entra desde el navegador
(`https://192.168.1.10:8443`), un usuario —basta uno de sólo lectura— y el
*sitio*, que en casi todas se llama `default`.

Deje **apagado** «exigir certificado válido»: casi toda controladora casera usa
un certificado que ella misma se firmó, y exigirlo dejaría la función inservible.

Después de eso, cada equipo por WiFi aparece en el mapa colgando de su antena, y
el puerto lleva el nombre de la red a la que se conectó (`Oficina`, `Invitados`).
Si la controladora deja de contestar, la pantalla lo dice con el motivo: el WiFi
no desaparece del mapa en silencio.

De momento se entiende **UniFi (Ubiquiti)**, que es la que hay instalada en la
mayoría de los sitios chicos.

### Guardar el mapa

En la pantalla del mapa, el ícono de la flecha hacia abajo. Seis formatos:

| Formato | Para qué |
|---|---|
| **PNG** | Una imagen, para pegarla en un documento o mandarla por chat |
| **SVG** | Vectorial: se abre en Inkscape y se le mueven cajas o se le anota |
| **PDF** | Para imprimirlo y colgarlo, o mandarlo por correo |
| **ODS** | Hoja de cálculo de LibreOffice, con dos pestañas |
| **XLSX** | Lo mismo para Excel |
| **CSV** | Texto plano: las dos tablas, una tras otra |

Las tres hojas de cálculo llevan **dos tablas, no una**:

- **Aparatos** — un renglón por aparato: qué es, su IP y su MAC, de qué cuelga,
  por qué puerto, qué tan seguro es ese dato y de dónde salió.
- **Conexiones** — un renglón por conexión, **cada cable una sola vez**. Van
  también los puertos libres y lo que cuelga por el aire, que no tiene puerto.

Al guardar se abre el cuadro del escritorio y **usted elige la carpeta y el
nombre**. El archivo **se queda en su equipo y nada más**: MiRed no lo sube a
ninguna nube ni lo manda a ningún lado. Si quiere hacérselo llegar a alguien, lo
adjunta usted.

Los seis llevan arriba de qué sitio es el mapa y **de qué día son los datos** —no
de cuándo se exportó—, porque un plano de red sin fecha, a la semana, ya no se
sabe si es el bueno.

### Subir una hoja que ya tenía

Si la instalación ya está documentada en una hoja de cálculo —lo normal en un
sitio cableado por alguien— no hace falta capturarla aparato por aparato. En la
pantalla de la red, menú **⋮ → Importar aparatos de una hoja**.

Es una pantalla completa: **para qué sirve, los tres pasos, el campo del archivo
y la guía de llenado dentro**, para tenerla delante mientras se llena la hoja. Se
descarga la plantilla, se llena y se sube. Acepta **CSV, ODS y XLSX**.

| Columna | ¿Obligatoria? | Qué se escribe |
|---|---|---|
| `NOMBRE` | **sí** | Cómo se llama: `D01`, `serv1`, `switch site` |
| `QUE_ES` | **sí** | switch, modem, router, pc, camara, impresora, telefono, servidor, punto de acceso, tv, almacenamiento, otro |
| `PUERTOS` | no | Solo para switches y modems: cuántos tiene |
| `CUELGA_DE` | no | El **nombre** del aparato del que cuelga |
| `PUERTO` | no | El puerto **de ese** aparato: `7`, `LAN 7`, `WAN 1` |
| `UBICACION` | no | Dónde está: `farmacia`, `cons 5` |
| `IP` · `MAC` · `MODELO` · `NOTAS` | no | Cuando se sepan |
| `ACCESO` · `USUARIO` · `CLAVE` · `DIRECCION` | no | Cómo se entra a su panel. La clave se guarda **cifrada** |

Tres cosas que conviene saber:

- **El switch también lleva su renglón**, y los demás cuelgan de él poniéndolo en
  `CUELGA_DE`. El orden de los renglones da igual.
- **Nada se guarda hasta que usted lo diga.** Al elegir el archivo se muestra, con
  el número de renglón de su hoja, qué se va a crear, qué ya existía y qué no se
  puede importar y por qué.
- Si su hoja viene de otro sitio, **no hace falta reescribirla**: se reconocen
  encabezados como `NODO`, `OBSERVACIONES` o `CONECTADO_A`, el punto y coma que
  pone Excel en español y los acentos.

Si el archivo trae contraseñas, MiRed las guarda cifradas pero **el archivo las
lleva en claro**: conviene borrarlo después de importar. La pantalla lo avisa.

Volver a subir la misma hoja no duplica nada: lo que ya existe se actualiza o se
deja como está, según lo que usted elija, y **una celda vacía nunca borra un dato
que ya estaba**.

## 6. Medir quién consume

Hay dos formas y no se estorban:

- **Con switches administrables**: MiRed lee los contadores de cada puerto. Como ya
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
