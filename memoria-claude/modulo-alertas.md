---
name: modulo-alertas
description: Motor de alertas de MiRed: que se vigila, como se evita repetir avisos y a donde se manda
metadata:
  type: project
---

# Modulo: alertas

**Que hace:** avisa cuando la red cambia. Es lo que separa un inventario de una
herramienta util: el valor no es escanear, es enterarse.

**Donde vive:** `internal/basedatos/alertas.go` (deteccion y persistencia),
`internal/avisos/` (envio), `internal/programador/` (cuando corre),
`internal/api/alertas.go`, `interfaz/lib/pantallas/alertas.dart`.

## Que se vigila (tabla `reglas_alerta`, por red)
| Regla | Umbral | Nace |
|---|---|---|
| `equipo_nuevo` | — | encendida |
| `equipo_ausente` | minutos sin aparecer (1440 = 24 h) | encendida |
| `puerto_nuevo` | — | encendida |
| `cambio_ip` | — | encendida |
| `cambio_puerto_switch` | — | encendida |
| `red_sin_reportar` | minutos (120) | encendida |

## Las tres decisiones que sostienen el modulo
1. **Huella unica por hecho**, no por momento: `tipo|equipo|detalle`. Sin esto se
   repetiria el mismo aviso en cada corrida hasta que nadie les haga caso, que es
   como mueren los sistemas de alertas.
2. **Se detecta sobre lo YA GUARDADO**, no volviendo a mirar la red: el motor no
   debe poder decir algo distinto de lo que quedo escrito.
3. **Un puerto nuevo solo alerta en un equipo CONOCIDO.** En uno recien
   descubierto la noticia era el equipo, no sus puertos.
4. **El cambio de puerto solo se detecta entre puertos CONFIRMADOS**: en un puerto
   compartido nunca se supo cual equipo estaba donde, asi que decir que se movio
   seria inventar.
5. **"Red sin reportar" es la unica alerta que no nace de un escaneo** — nace de
   que no hubo ninguno. La revisa el programador cada cinco minutos, y una red
   que nunca se escaneo NO avisa: seria ruido el dia que se crea el sitio.

## Envio
Destinos por red (tabla `destinos_alerta`): `ntfy`, `telegram`, `correo` (SMTP) y
`webhook`. Todo best-effort con 10 s de plazo. **Una alerta se marca enviada
aunque un destino falle**: reintentar por culpa de uno significaria avisar tres
veces a los que si funcionan. El error del destino queda en `ultimo_error` y se
ve en la interfaz.

Los secretos (token de Telegram, clave SMTP) **no vuelven al navegador**: la API
los sustituye por "(guardado)". Mismo criterio que las comunidades SNMP.

## Contador del panel
`redes.alertas_abiertas` en el **catalogo** se actualiza al generar alertas y al
marcarlas vistas. Si solo se actualizara al escanear, el panel seguiria mostrando
alertas ya atendidas hasta el barrido siguiente.

**Ver tambien:** [[modulo-escaneo]], [[modulo-topologia-manual]], [[gotchas]], [[historial-bugs]], [[contrato-api]]
