---
name: modulo-programa
description: "MiRed como programa de escritorio: como arranca y mata sus servicios, y donde viven los datos"
metadata:
  node_type: memory
  type: project
  modified: 2026-08-13T21:00:00.000Z
---

# Modulo: MiRed como programa de escritorio

**Que hace:** MiRed **es un programa**, no un servicio con pagina web. Se instala
el `.deb`, aparece en el menu de aplicaciones y se abre como cualquier otro.

**Donde vive:** `interfaz/` (todo el Flutter),
`interfaz/lib/servicios/supervisor_generico.dart` (el que arranca y mata),
`empaquetado/escritorio/` (icono y entrada de menu),
`herramientas/construir.sh` (lo compila y empaqueta).

## El giro del 2026-08-13
Nacio como servicio con interfaz web servida por el propio binario. El usuario
instalo el `.deb` y **no lo encontro por ningun lado**: lo busco en el menu de
aplicaciones, como cualquiera. De ahi salieron dos decisiones, en este orden:

1. Compilar la interfaz **tambien** a programa de escritorio.
2. **Quitar la web del todo**: "no quiero nada web, todo es un programa de
   escritorio".

Ya no se compila interfaz web ni se empaqueta. El servidor solo expone la API.

## Como funciona
- Al **abrir** el programa se levantan `mired-servidor` y `mired-sonda` como
  procesos hijos, y se espera a que el servidor conteste antes de entrar.
- Al **cerrar** la ventana se matan, con `SIGTERM` y no `SIGKILL`: los dos
  cierran sus bases al recibirlo, y una base SQLite cortada de golpe deja su
  archivo de bitacora suelto. Se engancha con `didRequestAppExit`.
- Los datos van a **`~/.local/share/mired`**, no a `/var/lib`: los servicios
  corren como el usuario y no tendria por que poder escribir en el sistema.
- El servidor escucha en `127.0.0.1:60072` cuando lo lanza el programa: como
  programa, MiRed no tiene por que quedar expuesto a la red sin pedirlo.

## Las reglas que no se pueden romper
1. **Solo se mata lo que se arranco.** Si al abrir ya hay un MiRed vivo —dejado
   como servicio, o huerfano de una sesion anterior— el programa se cuelga de el
   y al cerrar NO lo toca. Matar el servicio de otro dejaria sin vigilancia una
   red que si la tenia.
2. **Con el programa cerrado MiRed no vigila nada.** Ni escaneos programados, ni
   alertas de equipo desconocido, ni flujos del router. Es el precio elegido a
   cambio de "se instala y ya", y el usuario lo eligio sabiendolo.
3. **La sonda recibe `CAP_NET_RAW` por `setcap` sobre el binario**, no por
   systemd: lanzada por un usuario normal, es la unica forma. Lo hace el
   `postinst`. Sin eso el descubrimiento cae a sondeo de puertos: mas lento y sin
   direcciones MAC.
4. **El paquete de arm64 NO lleva el programa**: Flutter no cruza a arm64 desde
   un equipo amd64. El constructor lo AVISA por pantalla; meter un binario de la
   arquitectura equivocada seria entregar algo que no arranca.

## Vigilancia continua: la otra forma
Para un equipo que tiene que vigilar sin nadie delante (una Raspberry en una
sucursal), el paquete trae las unidades de systemd **instaladas y apagadas**, con
la explicacion en su cabecera. Se encienden a mano:

    sudo systemctl enable --now mired-sonda mired-servidor

**Las dos formas NO se mezclan**: el servicio guarda en `/var/lib/mired` y el
programa en la carpeta de cada usuario. Son dos instalaciones distintas.

## Conectarse a otro equipo
El programa puede apuntar a otro MiRed (`DialogoServidor`). Prueba la direccion
**antes** de guardarla: una mal escrita cerraria la sesion y dejaria al programa
sin poder entrar ni volver atras facil. Cambiar de servidor cierra la sesion a
proposito — una sesion es de un servidor, no del programa.

**Ver tambien:** [[mired]], [[mired-arquitectura]], [[contrato-api]], [[gotchas]]
