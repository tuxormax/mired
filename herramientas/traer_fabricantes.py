#!/usr/bin/env python3
"""Arma la lista de fabricantes de MiRed desde el registro de la IEEE.

    ./herramientas/traer_fabricantes.py

Escribe `catalogo/fabricantes.txt`, que el `.deb` instala en
/usr/share/mired/fabricantes.txt. Cada renglon es un prefijo de MAC y su dueno:

    f492bf	Ubiquiti Networks Inc.

Se bajan los tres registros, porque no todos los prefijos miden lo mismo:

  * **MA-L** (24 bits, 6 dígitos) — el clasico: una empresa dueña de 16 millones
    de direcciones. Cubre a los fabricantes grandes.
  * **MA-M** (28 bits, 7 dígitos) y **MA-S** (36 bits, 9 dígitos) — bloques
    chicos que la IEEE vende a empresas pequeñas. Sin estos, medio catalogo de
    aparatos baratos sale como "IEEE Registration Authority", que no dice nada.

Por eso la busqueda va del prefijo mas largo al mas corto: el que mide 9 gana
sobre el de 6, que es justo lo que la IEEE quiere decir al partir un bloque.

El archivo se versiona con el repo a proposito: construir el paquete no puede
depender de que la maquina que compila tenga internet.
"""

import csv
import io
import sys
import urllib.request
from datetime import date
from pathlib import Path

RAIZ = Path(__file__).resolve().parent.parent
DESTINO = RAIZ / "catalogo" / "fabricantes.txt"

REGISTROS = [
    ("MA-L", "https://standards-oui.ieee.org/oui/oui.csv"),
    ("MA-M", "https://standards-oui.ieee.org/oui28/mam.csv"),
    ("MA-S", "https://standards-oui.ieee.org/oui36/oui36.csv"),
]

# Nombres que no identifican a nadie: son el dueño del bloque grande, no del
# aparato. Guardarlos seria peor que no tener el prefijo, porque taparian la
# respuesta buena que si trae el registro de 28 o 36 bits.
INUTILES = {
    "ieee registration authority",
    "private",
}


def limpiar(nombre: str) -> str:
    return " ".join(nombre.replace("\n", " ").replace("\t", " ").split()).strip('"')


def bajar(url: str) -> str:
    # La IEEE contesta 418 a los clientes que no se presentan.
    peticion = urllib.request.Request(url, headers={"User-Agent": "MiRed/1.15 (herramientas/traer_fabricantes.py)"})
    with urllib.request.urlopen(peticion, timeout=120) as respuesta:
        return respuesta.read().decode("utf-8", errors="replace")


def main() -> int:
    entradas: dict[str, str] = {}
    cuenta: dict[str, int] = {}

    for registro, url in REGISTROS:
        try:
            crudo = bajar(url)
        except Exception as error:  # noqa: BLE001
            print(f"no se pudo bajar {registro}: {error}", file=sys.stderr)
            return 1

        lector = csv.DictReader(io.StringIO(crudo))
        cuenta[registro] = 0
        for fila in lector:
            prefijo = (fila.get("Assignment") or "").strip().lower()
            nombre = limpiar(fila.get("Organization Name") or "")
            if not prefijo or not nombre:
                continue
            if nombre.lower() in INUTILES:
                continue
            entradas[prefijo] = nombre
            cuenta[registro] += 1

    if not entradas:
        print("la IEEE no devolvio nada", file=sys.stderr)
        return 1

    DESTINO.parent.mkdir(parents=True, exist_ok=True)
    with DESTINO.open("w", encoding="utf-8") as salida:
        salida.write("# Fabricantes por prefijo de MAC, del registro de la IEEE.\n")
        salida.write(f"# Bajado el {date.today().isoformat()} con herramientas/traer_fabricantes.py\n")
        salida.write("# " + "  ".join(f"{reg}: {num}" for reg, num in cuenta.items()) + "\n")
        salida.write("#\n")
        salida.write("# prefijo<TAB>fabricante. Gana el prefijo mas largo que coincida.\n")
        for prefijo in sorted(entradas):
            salida.write(f"{prefijo}\t{entradas[prefijo]}\n")

    print(f"{len(entradas)} prefijos en {DESTINO}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
