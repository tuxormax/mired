# Inspección profunda — `mired-dpi`

Responde **en qué** se gasta el ancho de banda, no sólo **cuánto**: Netflix, un
respaldo, una actualización de Windows o una videollamada.

## Viene incluida, pero apagada

`mired-dpi` va dentro del mismo paquete que el resto de MiRed, así que no hay
nada que descargar aparte. Pero **el servicio no se enciende solo**.

Es a propósito, y por la misma razón por la que antes iba en un paquete
separado: es el **único proceso de MiRed que trabaja todo el tiempo**. Mirar
todos los paquetes cuesta mucho más que preguntarle a un switch cuántos bytes
pasó por un puerto, y ésa es exactamente la diferencia entre MiRed y las
herramientas que acaban en «consumo alto». Encenderlo sin que nadie lo pida
sería cobrarle ese coste a todo el mundo — y además no serviría de nada, porque
sin puerto espejo no hay nada que capturar.

Enciéndalo sólo si de verdad necesita el detalle por aplicación. Para «quién se
está llevando el internet» bastan los contadores SNMP o los flujos del router,
que funcionan desde el primer momento y cuestan casi cero.

## Qué necesita: un puerto espejo

Un switch le manda a cada puerto sólo lo suyo. Un equipo conectado normalmente ve
su propio tráfico y poco más, así que **sin un puerto espejo esto no mide casi
nada**.

Hay que decirle al switch que copie hacia el puerto donde está MiRed todo lo que
pasa por otra. Según la marca se llama **puerto espejo**, **SPAN** o **port
mirroring**, y está en el menú de administración del switch.

Si no puede configurarlo, no instale este paquete: no falla, simplemente mide muy
poco. La pantalla se lo dice, y el registro también.

## Encender

Ya está instalada. Dígale en qué tarjeta llega el puerto espejo, en
`/etc/mired/mired.toml`:

```
[dpi]
interfaz = "eth1"
```

Y enciéndala:

```
sudo systemctl enable --now mired-dpi
```

Si pone la interfaz **antes** de instalar MiRed —o si reinstala el paquete con la
interfaz ya puesta—, el instalador la enciende solo: encuentra la interfaz
configurada y entiende que usted la quiere.

Para comprobar que está capturando de verdad:

```
journalctl -u mired-dpi -f
```

Si el informe sale casi vacío después de un rato, el puerto espejo es el primer
sospechoso.

## Qué hace exactamente, y qué NO hace

**No descifra nada ni lo intenta.** No hay interceptación, no se instalan
certificados propios, no se toca el contenido de ninguna conexión.

Lo que lee son las tres cosas que viajan **en claro** aunque la conexión vaya
cifrada, y que dicen con quién se está hablando:

| De dónde | Qué da |
|---|---|
| Saludo de TLS (SNI) | El nombre del servidor. Va en claro porque el cliente tiene que decirlo para recibir el certificado correcto |
| Cabecera `Host` de HTTP | Lo poco que sigue sin cifrar |
| Consultas de DNS | El equipo pregunta el nombre **antes** de conectarse, y eso delata incluso lo que después va por QUIC |

Cuando nada de eso aparece, se usa el puerto como último recurso: dice el tipo de
servicio (SSH, escritorio remoto, VPN), no con quién se habla. La pantalla
distingue las dos cosas: un nombre sacado del saludo de TLS no vale lo mismo que
una suposición por número de puerto.

Y lo que no se identifica **sigue sumando bytes**, marcado como «sin
identificar». Tirarlo haría que el informe sumara menos que el consumo real y
nadie sabría por qué.

## Cómo se reparten el trabajo los procesos

Igual que la sonda de escaneo, y por la misma razón:

- **`mired-dpi`** es el que necesita permisos de red cruda (`CAP_NET_RAW` y
  `CAP_NET_ADMIN`, nunca root entero). A cambio **no toca ninguna base de datos**.
- **`mired-servidor`** le pregunta cada pocos minutos qué ha visto, y es el único
  que escribe.

## Privacidad, sin rodeos

Esto ve el tráfico de la red donde se instale. Antes de encenderlo en una oficina,
tenga en cuenta que:

- Queda registrado **con qué servidores habla cada equipo**, y por lo tanto qué
  sitios visita cada persona.
- No queda registrado el contenido: ni mensajes, ni contraseñas, ni archivos.
- El historial se conserva **14 días** y luego se borra solo.
- Todo se queda en el equipo donde está instalado MiRed. No se sube a ninguna
  nube ni se manda a nadie.

En muchos sitios esto exige avisar a la gente, y en algunos países exige su
consentimiento. Es su responsabilidad, no la de la herramienta.

## Apagar

```
sudo systemctl disable --now mired-dpi
```

El resto de MiRed sigue funcionando igual, y el equipo deja de hacer ese trabajo
por completo. El consumo por aplicación que ya se había guardado no se borra: es
historia de la red y vive en la base de cada red, junto a todo lo demás.
