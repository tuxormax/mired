#!/usr/bin/env bash
# Prueba de humo de MiRed: construye el paquete, lo desempaqueta en una carpeta
# aparte, levanta los dos servicios desde ahi y recorre el flujo completo.
#
#   ./herramientas/probar.sh
#   ./herramientas/probar.sh --subred 192.168.1.0/24
#
# Es la unica forma de comprobar que lo que se ENTREGA funciona. Correr los
# binarios desde el arbol de compilacion no prueba lo mismo: no valida que el
# paquete lleve la interfaz, ni el catalogo de dispositivos, ni la configuracion.
#
# No toca el sistema: no instala nada, no abre puertos conocidos y borra su
# carpeta de trabajo al terminar.
set -euo pipefail

RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CARPETA="$(mktemp -d /tmp/mired-prueba-XXXXXX)"
PUERTO=60099
PUERTO_FLUJOS=12099
SUBRED="192.168.1.0/29"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --subred) SUBRED="$2"; shift 2 ;;
        --puerto) PUERTO="$2"; shift 2 ;;
        *) echo "Opcion desconocida: $1" >&2; exit 1 ;;
    esac
done

fallos=0
paso() { printf '  \033[32m✓\033[0m %s\n' "$1"; }
falla() { printf '  \033[31m✗\033[0m %s\n' "$1"; fallos=$((fallos + 1)); }

limpiar() {
    pkill -f "$CARPETA/usr/bin" 2>/dev/null || true
    sleep 1
    rm -rf "$CARPETA"
}
trap limpiar EXIT

# --------------------------------------------------------------- construir --
echo "== Construyendo el paquete"
"$RAIZ/herramientas/construir.sh" --arquitectura amd64 >/dev/null
PAQUETE=$(ls -t "$RAIZ/instaladores/"mired_*_amd64.deb | head -1)
echo "   $PAQUETE"

dpkg-deb -x "$PAQUETE" "$CARPETA"

cat > "$CARPETA/mired.toml" <<FIN
[servidor]
escucha = "127.0.0.1:$PUERTO"
[datos]
ruta = "$CARPETA/datos"
[flujos]
escucha = "127.0.0.1:$PUERTO_FLUJOS"
[sonda]
socket = "$CARPETA/sonda.sock"
[catalogo]
# Las tres carpetas del catalogo, todas dentro de la carpeta de la prueba: lo
# que se guarde aqui NO puede acabar en /etc ni en /var/lib del equipo.
carpetas = ["$CARPETA/usr/share/mired/dispositivos", "$CARPETA/comunidad", "$CARPETA/propias"]
carpeta_propia = "$CARPETA/propias"
carpeta_comunidad = "$CARPETA/comunidad"
[registro]
nivel = "info"
FIN

echo "== Levantando los servicios desde el paquete"
"$CARPETA/usr/bin/mired-sonda" --configuracion "$CARPETA/mired.toml" > "$CARPETA/sonda.log" 2>&1 &
"$CARPETA/usr/bin/mired-servidor" --configuracion "$CARPETA/mired.toml" > "$CARPETA/servidor.log" 2>&1 &

for _ in $(seq 1 20); do
    if curl -s -o /dev/null "http://127.0.0.1:$PUERTO/api/estado"; then break; fi
    sleep 1
done

# El administrador que crea esta prueba. Lleva un operador porque el algoritmo
# TUXOR lo exige: sin el, no se puede calcular ningun hash.
ADMIN="+prueba"
ADMIN_CLAVE="*clavedeprueba#"

API="http://127.0.0.1:$PUERTO"
GALLETAS="$CARPETA/galletas.txt"
pedir() { curl -s -b "$GALLETAS" "$@"; }

# ------------------------------------------------------------------ probar --
echo "== Recorriendo el flujo completo"

curl -s "$API/api/estado" | grep -q '"ok":true' \
    && paso "el servicio responde" || falla "el servicio no responde"

# MiRed no tiene interfaz web: la raiz explica que es un programa, en vez de dar
# un 404 seco que haria pensar que algo se rompio.
curl -s "$API/" | grep -q "programa de escritorio" \
    && paso "la raiz explica que MiRed no se usa desde el navegador" \
    || falla "la raiz no explica nada"

[[ -x "$CARPETA/usr/share/mired/escritorio/mired_interfaz" ]] \
    && paso "el paquete trae el programa de escritorio" \
    || falla "el paquete no trae el programa de escritorio"

[[ -f "$CARPETA/usr/share/applications/mired.desktop" ]] \
    && paso "y su entrada en el menu de aplicaciones" \
    || falla "falta la entrada de menu"

# Una instalacion recien hecha NO trae usuario de fabrica: lo dice y pide crearlo.
curl -s "$API/api/estado" | grep -q '"sinEstrenar":true' \
    && paso "una instalacion nueva se declara sin estrenar" \
    || falla "no avisa que hay que crear el administrador"

# El usuario lleva un operador porque el algoritmo TUXOR lo exige.
curl -s -c "$GALLETAS" -X POST "$API/api/primer-administrador" \
     -d "{\"usuario\":\"$ADMIN\",\"nombre\":\"Prueba\",\"clave\":\"$ADMIN_CLAVE\"}" \
    | grep -q '"ok":true' \
    && paso "crea el administrador que eligio el usuario" \
    || falla "no se pudo crear el administrador"

# Y esa puerta se cierra en cuanto existe el primero: es lo que impide que
# alguien de fuera se cree un administrador en un MiRed ya en uso.
curl -s -X POST "$API/api/primer-administrador" \
     -d '{"usuario":"+intruso","nombre":"x","clave":"colado12"}' \
    | grep -q '"ok":false' \
    && paso "ya no deja crear un segundo administrador" \
    || falla "dejo crear un administrador con la instalacion ya estrenada"

curl -s "$API/api/estado" | grep -q '"sinEstrenar":false' \
    && paso "la instalacion ya no se declara sin estrenar" \
    || falla "sigue diciendo que esta sin estrenar"

curl -s -c "$GALLETAS" -X POST "$API/api/sesion" \
     -d "{\"usuario\":\"$ADMIN\",\"clave\":\"$ADMIN_CLAVE\"}" | grep -q '"ok":true' \
    && paso "entra el administrador recien creado" || falla "no se pudo entrar"

curl -s -X POST "$API/api/sesion" -d "{\"usuario\":\"$ADMIN\",\"clave\":\"mala1234\"}" \
    | grep -q '"ok":false' \
    && paso "rechaza una clave equivocada" || falla "acepto una clave equivocada"

# Un usuario sin ningun operador ni en el usuario ni en la clave: el algoritmo no
# lo admite, y el servidor tiene que decirlo aunque el formulario ya lo avise.
pedir -X POST "$API/api/usuarios" \
     -d '{"usuario":"soso","nombre":"Sin operador","clave":"clavesimple"}' \
    | grep -q '"ok":false' \
    && paso "rechaza un usuario sin operador de TUXOR" \
    || falla "acepto credenciales que TUXOR no puede calcular"

curl -s "$API/api/redes" | grep -q '"causa":"Sesion"' \
    && paso "sin sesion no deja ver nada" || falla "dejo ver datos sin sesion"

pedir "$API/api/catalogo" | grep -q '"definiciones"' \
    && paso "el catalogo de dispositivos carga" || falla "el catalogo no carga"

# Sin la lista de la IEEE nadie tiene fabricante, y sin fabricante el catalogo se
# queda adivinando por puertos abiertos: todo lo que tenga el 80 abierto acaba
# llamandose "servidor web".
[[ -s "$CARPETA/usr/share/mired/fabricantes.txt" ]] \
    && paso "el paquete trae la lista de fabricantes de la IEEE" \
    || falla "el paquete no trae usr/share/mired/fabricantes.txt"
grep -qi '^f492bf	' "$CARPETA/usr/share/mired/fabricantes.txt" 2>/dev/null \
    && paso "y una MAC conocida encuentra a su dueno" \
    || falla "la lista de fabricantes no reconoce una MAC que deberia estar"

# El catalogo comunitario: guardar una definicion propia tiene que dejarla
# reconociendo desde ese mismo momento, sin reiniciar el servicio.
DEFINICION='{"archivo":"prueba-de-humo.toml","contenido":"nombre = \"Aparato de prueba\"\ncategoria = \"otro\"\n[coincidencias]\nprefijos_mac = [\"aa:bb:cc\"]\n"}'
pedir -X POST "$API/api/catalogo/dispositivos" -d "$DEFINICION" \
    | grep -q '"archivo":"prueba-de-humo.toml"' \
    && paso "se puede guardar una definicion en el catalogo de aqui" \
    || falla "no se pudo guardar una definicion propia"

pedir -X POST "$API/api/catalogo/dispositivos" \
     -d '{"archivo":"sin-condiciones.toml","contenido":"nombre = \"Todo\"\n"}' \
    | grep -q '"ok":false' \
    && paso "y se rechaza la que reconoceria a todos los aparatos por igual" \
    || falla "acepto una definicion sin ninguna condicion"

pedir "$API/api/catalogo" | grep -q 'Aparato de prueba' \
    && paso "la definicion recien guardada ya esta en el catalogo" \
    || falla "el catalogo no se recargo despues de guardar"

[[ -f "$CARPETA/propias/prueba-de-humo.toml" ]] \
    && paso "y quedo escrita en la carpeta que dice la configuracion" \
    || falla "la definicion no se escribio donde debia"

# El historial de versiones viaja dentro del binario y se siembra al arrancar.
# Que este vacio significaria que el pie de la interfaz no muestra nada.
pedir "$API/api/versiones" | grep -q '"versionessistema"\|"sistema"' \
    && paso "el historial de versiones se sembro" \
    || falla "el historial de versiones esta vacio"

pedir "$API/api/versiones" | grep -q '"revision":20' \
    && paso "y trae la revision de esta entrega" \
    || falla "el historial no llega hasta la revision de hoy"

pedir "$API/api/sonda" | grep -q '"viva":true' \
    && paso "el servidor habla con la sonda" || falla "el servidor no ve la sonda"

CLAVE=$(pedir -X POST "$API/api/redes" \
        -d "{\"nombre\":\"Prueba\",\"subredes\":[\"$SUBRED\"]}" \
        | sed -n 's/.*"clave":"\([^"]*\)".*/\1/p')
[[ -n "$CLAVE" ]] && paso "crea una red ($CLAVE)" || falla "no se pudo crear la red"

[[ -f "$CARPETA/datos/redes/$CLAVE.db" ]] \
    && paso "la red tiene su propio archivo de base de datos" \
    || falla "no se creo el archivo de la red"

pedir -X POST "$API/api/redes" -d '{"nombre":"Prueba"}' | grep -q '"ok":false' \
    && paso "no deja repetir el nombre de una red" || falla "permitio repetir el nombre"

pedir -X POST "$API/api/redes/$CLAVE/subredes" -d '{"cidr":"no-es-una-subred"}' \
    | grep -q '"ok":false' \
    && paso "rechaza una subred mal capturada" || falla "acepto una subred invalida"

echo "== Escaneando $SUBRED (puede tardar)"
pedir -X POST "$API/api/redes/$CLAVE/escaneos" -d '{}' >/dev/null
for _ in $(seq 1 60); do
    grep -q "escaneo terminado" "$CARPETA/servidor.log" && break
    sleep 2
done

grep -q "escaneo terminado" "$CARPETA/servidor.log" \
    && paso "el escaneo termina" || falla "el escaneo no termino"

pedir "$API/api/redes/$CLAVE/equipos" | grep -q '"ok":true' \
    && paso "el inventario responde" || falla "el inventario fallo"

pedir "$API/api/redes/$CLAVE/mapa-puertos" | grep -q '"capacidad"' \
    && paso "el mapa de puertos dice que se puede saber en esta red" \
    || falla "el mapa de puertos no responde"

pedir "$API/api/redes/$CLAVE/alertas" | grep -q '"abiertas"' \
    && paso "las alertas responden" || falla "las alertas fallaron"

# El aire: en un equipo sin tarjeta WiFi tiene que EXPLICARLO, no devolver una
# lista vacia que se leeria como "aqui no hay redes inalambricas".
AIRE=$(pedir "$API/api/redes/$CLAVE/aire")
echo "$AIRE" | grep -q '"redes"' \
    && paso "el barrido del aire responde" || falla "el barrido del aire no responde"
echo "$AIRE" | grep -qE '"redes":\[\{|"explicacion":"[^"]' \
    && paso "y si no puede oir nada, dice por que" \
    || falla "el aire devolvio una lista vacia sin explicar por que"

# ---------------------------------------------------- topologia declarada --
#
# Lo que ningun escaneo puede ver: un switch no administrable no tiene
# direccion, no contesta a nada y no existe para ningun barrido.

pedir "$API/api/redes/$CLAVE/topologia-manual" | grep -q '"contradicciones"' \
    && paso "la topologia declarada a mano responde" \
    || falla "la topologia declarada a mano no responde"

# De que esta hecha la red: el total y cuantos de cada tipo.
pedir "$API/api/redes/$CLAVE/composicion" | grep -q '"categorias"' \
    && paso "la composicion de la red responde" \
    || falla "la composicion de la red no responde"

# La categoria sale de una lista cerrada. Texto libre aqui significa que el
# contador saca dos cubos para la misma cosa, y nadie se entera.
pedir -X POST "$API/api/redes/$CLAVE/equipos" \
     -d '{"nombre":"Invento","categoria":"ap","puertos":0}' | grep -q '"ok":false' \
    && paso "rechaza una categoria que no esta en la lista" \
    || falla "acepto una categoria inventada"

MANUAL=$(pedir -X POST "$API/api/redes/$CLAVE/equipos" \
         -d '{"nombre":"Switch del rack","categoria":"switch_simple","tipo":"Switch no administrable","modelo":"TP-Link SG108","puertos":8}' \
         | sed -n 's/.*"id":\([0-9]*\).*/\1/p')
[[ -n "$MANUAL" ]] \
    && paso "da de alta un switch que ningun escaneo veria (id $MANUAL)" \
    || falla "no se pudo dar de alta un equipo a mano"

pedir -X POST "$API/api/redes/$CLAVE/equipos" -d '{"nombre":"Switch del rack"}' \
    | grep -q '"ok":false' \
    && paso "no deja repetir el nombre de un equipo" \
    || falla "permitio repetir el nombre de un equipo"

pedir "$API/api/redes/$CLAVE/topologia-manual" | grep -q '"numero":8' \
    && paso "y le quedaron declaradas sus ocho puertos" \
    || falla "los puertos del switch no se declararon"

# El contador y el mapa leen la MISMA tabla: un switch declarado a mano cuenta
# desde el momento en que se declara, sin nada que sincronizar.
pedir "$API/api/redes/$CLAVE/composicion" | grep -q '"switch_simple"' \
    && paso "el switch declarado a mano ya cuenta en la composicion" \
    || falla "el contador no vio el switch declarado a mano"

pedir "$API/api/redes/$CLAVE/composicion" | grep -q '"declarados":1' \
    && paso "y se dice que es declarado, no medido" \
    || falla "el contador no distingue lo declarado de lo medido"

# Y ahora se vuelve a escanear CON el equipo declarado ya dado de alta: no tiene
# direccion y ningun barrido lo va a ver nunca, asi que si se marcara ausente
# generaria un evento de presencia y una alerta en cada corrida.
pedir -X POST "$API/api/redes/$CLAVE/escaneos" -d '{"soloPresencia":true}' >/dev/null
for _ in $(seq 1 30); do
    sleep 2
    pedir "$API/api/redes/$CLAVE/escaneos" | grep -q '"enCurso":false' && break
done

pedir "$API/api/redes/$CLAVE/equipos" \
    | grep -q "\"id\":$MANUAL,[^}]*\"presente\":true" \
    && paso "el equipo declarado sigue presente tras un escaneo posterior" \
    || falla "el escaneo marco ausente un equipo que nunca va a contestar"

PUERTO=$(pedir "$API/api/redes/$CLAVE/topologia-manual" \
       | sed -n 's/.*"puertos":\[{"id":\([0-9]*\).*/\1/p')
pedir -X POST "$API/api/redes/$CLAVE/enlaces" \
     -d "{\"puertoOrigenId\":$PUERTO,\"equipoDestinoId\":$MANUAL}" \
    | grep -q '"ok":false' \
    && paso "no deja conectar un equipo consigo mismo" \
    || falla "permitio conectar un equipo consigo mismo"

pedir -X PUT "$API/api/redes/$CLAVE/equipos/$MANUAL" \
     -d '{"modelo":"TP-Link SG108","notas":"En el rack de arriba","conexion":"paloma"}' \
    | grep -q '"ok":false' \
    && paso "rechaza una forma de conexion que no existe" \
    || falla "acepto una conexion que la columna no admite"

pedir -X DELETE "$API/api/redes/$CLAVE/equipos/$MANUAL" | grep -q '"borrado":true' \
    && paso "borra el equipo declarado a mano" \
    || falla "no se pudo borrar el equipo declarado"

pedir "$API/api/redes/$CLAVE/consumo" | grep -q '"explicacion"' \
    && paso "el consumo responde y se explica" || falla "el consumo fallo"

# Un flujo NetFlow v5 armado a mano, para comprobar que el receptor escucha.
python3 - "$PUERTO_FLUJOS" <<'PY' && paso "el receptor de flujos acepta un datagrama" \
    || falla "el receptor de flujos no acepta datagramas"
import socket, struct, sys
cabecera = struct.pack('!HHIIIIBBH', 5, 1, 0, 0, 0, 0, 0, 0, 0)
registro = (socket.inet_aton('192.168.1.5') + socket.inet_aton('8.8.8.8') +
            b'\x00' * 4 + struct.pack('!HH', 0, 0) +
            struct.pack('!II', 10, 100000) + b'\x00' * 28)
socket.socket(socket.AF_INET, socket.SOCK_DGRAM).sendto(
    cabecera + registro, ('127.0.0.1', int(sys.argv[1])))
PY

# El mismo puerto recibe los cuatro formatos y los reconoce solo. sFlow es el que
# mas facil se confunde con NetFlow v5: los dos dicen "version 5", y solo se
# distinguen por si esa version viene en dos bytes o en cuatro.
python3 "$RAIZ/herramientas/enviar_sflow.py" "$PUERTO_FLUJOS" \
    && paso "el receptor entiende tambien sFlow" \
    || falla "el receptor no acepto un datagrama de sFlow"

pedir "$API/api/redes/$CLAVE/aplicaciones" | grep -q '"explicacion"' \
    && paso "el consumo por aplicacion responde" \
    || falla "el consumo por aplicacion no responde"

# La inspeccion profunda va en el MISMO paquete desde que se juntaron, asi que su
# binario y su unidad tienen que venir dentro.
[[ -x "$CARPETA/usr/bin/mired-dpi" ]] \
    && paso "el paquete trae el binario de inspeccion profunda" \
    || falla "falta el binario de inspeccion profunda"

[[ -f "$CARPETA/lib/systemd/system/mired-dpi.service" ]] \
    && paso "y su unidad de systemd" \
    || falla "falta la unidad de systemd de la inspeccion profunda"

# Pero SIN interfaz configurada no captura nada: lo dice y termina con bien, en
# vez de reventar o ponerse a girar en balde.
"$CARPETA/usr/bin/mired-dpi" --configuracion "$CARPETA/mired.toml" 2>&1 \
    | grep -q "interfaz configurada" \
    && paso "la inspeccion sin interfaz avisa en vez de arrancar" \
    || falla "mired-dpi deberia avisar cuando no tiene interfaz"

# Y el postinst la deja apagada mientras no haya interfaz: es lo que evita
# cobrarle el proceso continuo a todo el que instale MiRed.
grep -q "systemctl disable mired-dpi" "$CARPETA/../DEBIAN/postinst" 2>/dev/null \
    || grep -q "habilitar_inspeccion" "$RAIZ/empaquetado/debian/postinst" \
    && paso "el instalador deja la inspeccion apagada por omision" \
    || falla "el instalador deberia dejar la inspeccion apagada"

pedir -X DELETE "$API/api/redes/$CLAVE" | grep -q '"borrada":true' \
    && paso "borra la red" || falla "no se pudo borrar la red"

[[ -f "$CARPETA/datos/redes/$CLAVE.db" ]] \
    && paso "el archivo de la red se conserva tras borrarla" \
    || falla "se borro el archivo de datos de la red"

# ---------------------------------------------------------------- resultado --
echo
if [[ $fallos -eq 0 ]]; then
    echo "== Todo bien"
else
    echo "== $fallos comprobaciones fallaron"
    echo "   bitacora del servidor: $CARPETA/servidor.log (se borra al salir)"
    tail -20 "$CARPETA/servidor.log"
    exit 1
fi
