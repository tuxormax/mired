#!/usr/bin/env bash
# Grabadora para cuando el equipo se traba.
#
#   ./herramientas/medir.sh              # graba hasta que se pulse Ctrl-C
#   ./herramientas/medir.sh --segundos 120
#
# Un tiron que dura dos segundos no se puede diagnosticar mirando la pantalla:
# cuando uno abre el monitor del sistema ya paso. Esto anota, cada segundo, quien
# esta gastando el equipo, y deja un archivo que se puede leer despues con calma.
#
# No toca nada: solo mira.
set -euo pipefail

SEGUNDOS=0
SALIDA="${TMPDIR:-/tmp}/mired-medicion-$(date +%Y%m%d-%H%M%S).txt"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --segundos) SEGUNDOS="$2"; shift 2 ;;
        --salida) SALIDA="$2"; shift 2 ;;
        *) echo "Opcion desconocida: $1" >&2; exit 1 ;;
    esac
done

echo "Grabando en $SALIDA"
echo "Reproduzca el problema. Para terminar: Ctrl-C"
echo ""

{
    echo "# Medicion de MiRed"
    echo "# equipo: $(uname -srm)   $(nproc) nucleos"
    echo "# memoria: $(free -h | awk 'NR==2 {print $2" total, "$7" disponible"}')"
    echo "# empezo: $(date -Is)"
    echo ""
} > "$SALIDA"

vuelta=0
while true; do
    vuelta=$((vuelta + 1))

    {
        echo "=== $(date +%H:%M:%S)  carga:$(cut -d' ' -f1-3 /proc/loadavg)  " \
             "libre:$(free -m | awk 'NR==2 {print $7}')MB"

        # Los cinco que mas CPU estan gastando ahora mismo. Es lo primero que se
        # mira: un tiron de todo el equipo casi siempre tiene un solo culpable.
        ps -eo pcpu,pmem,rss,comm --sort=-pcpu --no-headers | head -5 |
            awk '{printf "    cpu %5s%%  mem %5s%%  %6d MB  %s\n", $1, $2, $3/1024, $4}'

        # Y lo de MiRed, siempre, aunque no este entre los cinco: para poder
        # decir con datos que no era MiRed cuando no lo sea.
        ps -eo pcpu,pmem,rss,comm --no-headers |
            grep -E "mired" |
            awk '{printf "    MIRED    cpu %5s%%  mem %5s%%  %6d MB  %s\n", $1, $2, $3/1024, $4}' || true

        # Procesos atascados esperando al disco o al nucleo: es lo que hace que
        # se trabe TODO el equipo y no solo un programa.
        atascados=$(ps -eo stat,comm --no-headers | awk '$1 ~ /D/ {print "    ATASCADO en disco/nucleo: " $2}')
        [[ -n "$atascados" ]] && echo "$atascados"
    } >> "$SALIDA" 2>/dev/null

    if [[ "$SEGUNDOS" -gt 0 && "$vuelta" -ge "$SEGUNDOS" ]]; then
        break
    fi
    sleep 1
done

echo ""
echo "Listo: $SALIDA"
