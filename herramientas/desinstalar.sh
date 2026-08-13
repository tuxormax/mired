#!/usr/bin/env bash
# Quita MiRed de este equipo, sin dejar rastro.
#
#   sudo ./herramientas/desinstalar.sh          pregunta antes de borrar los datos
#   sudo ./herramientas/desinstalar.sh --todo   borra tambien los datos, sin preguntar
#
# Existe porque desinstalar el paquete NO basta: `dpkg --purge` deja a proposito
# las bases de datos de las redes, que es lo correcto en una actualizacion —
# nadie quiere perder el historico de su red por reinstalar— pero no es lo que
# se quiere cuando la intencion es empezar de cero.
#
# Lo que toca, en orden:
#   1. Detiene y deshabilita los servicios.
#   2. Purga el paquete (binarios, configuracion, unidades, entrada de menu).
#   3. Borra las bases de datos, si se confirma.
#   4. Borra el usuario de sistema.
#   5. Refresca el menu de aplicaciones.
set -euo pipefail

BORRAR_DATOS="preguntar"
[[ "${1:-}" == "--todo" ]] && BORRAR_DATOS="si"

if [[ "$(id -u)" -ne 0 ]]; then
    echo "Hay que correrlo como administrador:  sudo $0 ${1:-}" >&2
    exit 1
fi

# La carpeta del usuario que llamo a sudo, no la de root: los datos del programa
# viven en el HOME de quien lo usa, y bajo sudo $HOME es /root.
USUARIO_REAL="${SUDO_USER:-$USER}"
CASA_REAL="$(getent passwd "$USUARIO_REAL" | cut -d: -f6)"
DATOS_USUARIO="$CASA_REAL/.local/share/mired"
CONFIG_USUARIO="$CASA_REAL/.config/mired"

echo "== Deteniendo los servicios"
if command -v systemctl >/dev/null 2>&1; then
    for servicio in mired-dpi mired-servidor mired-sonda; do
        systemctl stop "$servicio" 2>/dev/null || true
        systemctl disable "$servicio" 2>/dev/null || true
    done
fi
# Y lo que haya lanzado el programa de escritorio, que no pasa por systemd.
pkill -f '/usr/bin/mired-(servidor|sonda|dpi)' 2>/dev/null || true

echo "== Quitando el paquete"
if dpkg -l mired >/dev/null 2>&1; then
    dpkg --purge mired 2>/dev/null || dpkg --purge --force-all mired
else
    echo "   (no estaba instalado)"
fi
# Por si quedo instalado el paquete separado de las versiones viejas.
dpkg -l mired-dpi >/dev/null 2>&1 && dpkg --purge mired-dpi 2>/dev/null || true

echo "== Buscando datos"
HAY_DATOS=0
for carpeta in /var/lib/mired "$DATOS_USUARIO"; do
    if [[ -d "$carpeta" ]]; then
        HAY_DATOS=1
        echo "   $carpeta  ($(du -sh "$carpeta" 2>/dev/null | cut -f1))"
    fi
done

if [[ $HAY_DATOS -eq 1 ]]; then
    if [[ "$BORRAR_DATOS" == "preguntar" ]]; then
        echo
        echo "   Ahi estan las redes descubiertas, su historico y sus alertas."
        echo "   BORRARLAS NO SE PUEDE DESHACER."
        read -r -p "   ¿Borrar los datos? [s/N] " respuesta
        [[ "$respuesta" =~ ^[sSyY]$ ]] && BORRAR_DATOS="si" || BORRAR_DATOS="no"
    fi

    if [[ "$BORRAR_DATOS" == "si" ]]; then
        rm -rf /var/lib/mired "$DATOS_USUARIO" "$CONFIG_USUARIO"
        echo "   Datos borrados."
    else
        echo "   Datos conservados."
    fi
else
    echo "   (no habia datos)"
fi

echo "== Quitando el usuario de sistema"
if getent passwd mired >/dev/null 2>&1; then
    # Solo se quita si ya no queda su carpeta: si el usuario decidio conservar
    # los datos, borrar su dueño los dejaria sin quien los pueda leer.
    if [[ -d /var/lib/mired ]]; then
        echo "   (se conserva: todavia es el dueño de /var/lib/mired)"
    else
        deluser --system mired 2>/dev/null || userdel mired 2>/dev/null || true
        echo "   Usuario mired quitado."
    fi
else
    echo "   (no existia)"
fi

echo "== Refrescando el menu de aplicaciones"
command -v update-desktop-database >/dev/null 2>&1 \
    && update-desktop-database -q /usr/share/applications 2>/dev/null || true
command -v gtk-update-icon-cache >/dev/null 2>&1 \
    && gtk-update-icon-cache -q -f /usr/share/icons/hicolor 2>/dev/null || true

echo
echo "== Comprobando que no quedo nada"
QUEDA=0
for rastro in /usr/bin/mired /usr/bin/mired-servidor /usr/bin/mired-sonda \
              /usr/bin/mired-dpi /etc/mired /usr/share/mired \
              /usr/share/applications/mired.desktop \
              /usr/share/icons/hicolor/scalable/apps/mired.svg \
              /lib/systemd/system/mired-servidor.service \
              /lib/systemd/system/mired-sonda.service \
              /lib/systemd/system/mired-dpi.service; do
    if [[ -e "$rastro" ]]; then
        echo "   QUEDA: $rastro"
        QUEDA=1
    fi
done
[[ -d /var/lib/mired ]] && { echo "   QUEDA (a proposito): /var/lib/mired"; }
[[ -d "$DATOS_USUARIO" ]] && { echo "   QUEDA (a proposito): $DATOS_USUARIO"; }

if [[ $QUEDA -eq 0 ]]; then
    echo "   Nada. El equipo quedo limpio."
fi
echo
