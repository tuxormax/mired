#!/usr/bin/env bash
# Construye MiRed: compila los binarios, arma la interfaz y genera el .deb.
#
#   ./herramientas/construir.sh                     amd64, version del repo
#   ./herramientas/construir.sh --arquitectura arm64
#   ./herramientas/construir.sh --arquitectura todas
#   ./herramientas/construir.sh --revision 12
#
# Los binarios salen estaticos (CGO_ENABLED=0), asi que el .deb no depende de
# ninguna biblioteca del sistema y el de arm64 se compila desde este mismo
# equipo sin toolchain cruzado. Esa es media razon por la que el proyecto es Go.
set -euo pipefail

RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SALIDA="$RAIZ/empaquetado/salida"

VERSION="v1.0"
REVISION="0"
ARQUITECTURAS=("amd64")

while [[ $# -gt 0 ]]; do
    case "$1" in
        --arquitectura)
            if [[ "$2" == "todas" ]]; then
                ARQUITECTURAS=("amd64" "arm64")
            else
                ARQUITECTURAS=("$2")
            fi
            shift 2
            ;;
        --version)  VERSION="$2"; shift 2 ;;
        --revision) REVISION="$2"; shift 2 ;;
        *) echo "Opcion desconocida: $1" >&2; exit 1 ;;
    esac
done

BUILD="$(git -C "$RAIZ" rev-parse --short HEAD 2>/dev/null || echo desconocido)"
# La version del paquete Debian no admite la v inicial ni mayusculas.
VERSION_DEB="${VERSION#v}-${REVISION}"

echo "== MiRed $VERSION Rev $REVISION ($BUILD)"

# ---------------------------------------------------------------- interfaz --
WEB=""
if [[ -f "$RAIZ/interfaz/pubspec.yaml" ]]; then
    if command -v flutter >/dev/null 2>&1; then
        echo "== Compilando la interfaz Flutter"
        (cd "$RAIZ/interfaz" && flutter build web --release -t lib/principal.dart)
        WEB="$RAIZ/interfaz/build/web"
    else
        echo "AVISO: Flutter no esta instalado; el paquete saldra sin interfaz."
    fi
else
    echo "AVISO: todavia no hay proyecto Flutter en interfaz/; el paquete saldra sin interfaz."
fi

# --------------------------------------------------------------- paquetes ---
mkdir -p "$SALIDA"

for ARQ in "${ARQUITECTURAS[@]}"; do
    echo "== Compilando binarios para $ARQ"
    ARBOL="$SALIDA/mired_${VERSION_DEB}_${ARQ}"
    rm -rf "$ARBOL"
    mkdir -p "$ARBOL/DEBIAN" \
             "$ARBOL/usr/bin" \
             "$ARBOL/usr/share/mired/dispositivos" \
             "$ARBOL/usr/share/doc/mired" \
             "$ARBOL/etc/mired/dispositivos" \
             "$ARBOL/lib/systemd/system" \
             "$ARBOL/var/lib/mired/redes"

    MARCAS="-s -w"
    MARCAS+=" -X github.com/tuxormax/mired/internal/version.Numero=$VERSION"
    MARCAS+=" -X github.com/tuxormax/mired/internal/version.Revision=$REVISION"
    MARCAS+=" -X github.com/tuxormax/mired/internal/version.Build=$BUILD"

    for PROGRAMA in mired-servidor mired-sonda; do
        CGO_ENABLED=0 GOOS=linux GOARCH="$ARQ" \
            go build -trimpath -ldflags "$MARCAS" \
            -o "$ARBOL/usr/bin/$PROGRAMA" "$RAIZ/programas/$PROGRAMA"
    done

    cp "$RAIZ/empaquetado/systemd/"*.service "$ARBOL/lib/systemd/system/"
    cp "$RAIZ/empaquetado/mired.toml" "$ARBOL/etc/mired/mired.toml"
    cp "$RAIZ/PLAN.md" "$ARBOL/usr/share/doc/mired/" 2>/dev/null || true
    cp "$RAIZ/documentacion/"*.md "$ARBOL/usr/share/doc/mired/" 2>/dev/null || true
    [[ -f "$RAIZ/LICENSE" ]] && cp "$RAIZ/LICENSE" "$ARBOL/usr/share/doc/mired/copyright"

    if [[ -d "$RAIZ/catalogo/dispositivos" ]]; then
        cp -r "$RAIZ/catalogo/dispositivos/." "$ARBOL/usr/share/mired/dispositivos/" 2>/dev/null || true
    fi
    if [[ -n "$WEB" ]]; then
        mkdir -p "$ARBOL/usr/share/mired/web"
        cp -r "$WEB/." "$ARBOL/usr/share/mired/web/"
    fi

    sed -e "s/^Version: VERSION/Version: $VERSION_DEB/" \
        -e "s/^Architecture: ARQUITECTURA/Architecture: $ARQ/" \
        "$RAIZ/empaquetado/debian/control" > "$ARBOL/DEBIAN/control"

    for GUION in postinst prerm postrm; do
        cp "$RAIZ/empaquetado/debian/$GUION" "$ARBOL/DEBIAN/$GUION"
        chmod 755 "$ARBOL/DEBIAN/$GUION"
    done

    # El archivo de configuracion es conffile: si el usuario lo edito, una
    # actualizacion no se lo pisa sin preguntar.
    echo "/etc/mired/mired.toml" > "$ARBOL/DEBIAN/conffiles"

    # El bit setgid se hereda de la carpeta del proyecto y dpkg-deb lo rechaza.
    # Hay que quitarlo explicitamente: chmod numerico no lo limpia en carpetas.
    find "$ARBOL" -type d -exec chmod g-s {} +
    find "$ARBOL" -type d -exec chmod 0755 {} +
    # Y los archivos van sin permiso de escritura para el grupo, que es lo que
    # espera un paquete del sistema.
    find "$ARBOL" -type f -exec chmod 0644 {} +
    chmod 0755 "$ARBOL/usr/bin/mired-servidor" "$ARBOL/usr/bin/mired-sonda"
    chmod 0755 "$ARBOL/DEBIAN/postinst" "$ARBOL/DEBIAN/prerm" "$ARBOL/DEBIAN/postrm"

    dpkg-deb --build --root-owner-group "$ARBOL" \
        "$SALIDA/mired_${VERSION_DEB}_${ARQ}.deb" >/dev/null
    rm -rf "$ARBOL"

    echo "   -> $SALIDA/mired_${VERSION_DEB}_${ARQ}.deb"
done

echo "== Listo"
