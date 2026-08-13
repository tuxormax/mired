#!/usr/bin/env python3
"""Manda un datagrama sFlow v5 de mentira al receptor de MiRed.

Lo usa la prueba de humo. Va en su propio archivo y no incrustado en probar.sh
porque el datagrama de sFlow son bloques encadenados —muestra dentro de
datagrama, registro dentro de muestra— y armarlo dentro de un heredoc se lee
fatal.

Lo que se comprueba con esto no es que "no reviente": es que el receptor sepa
distinguir sFlow de NetFlow v5. Los dos dicen "version 5"; NetFlow la pone en
dos bytes y sFlow en cuatro.
"""

import socket
import struct
import sys


def palabra(numero):
    return struct.pack("!I", numero)


def trama_ethernet(origen, destino):
    """Una trama Ethernet + IPv4 minima pero valida."""
    return (
        b"\x00\x11\x22\x33\x44\x55"          # MAC destino
        + b"\x66\x77\x88\x99\xAA\xBB"        # MAC origen
        + b"\x08\x00"                        # tipo: IPv4
        + b"\x45"                            # version 4, cabecera de 20 bytes
        + b"\x00" * 11
        + socket.inet_aton(origen)
        + socket.inet_aton(destino)
    )


def datagrama(origen, destino, tasa, tamano_de_trama):
    trama = trama_ethernet(origen, destino)

    # Registro de paquete crudo: protocolo, tamano real, recortado, cabecera.
    registro = palabra(1) + palabra(tamano_de_trama) + palabra(0)
    registro += palabra(len(trama)) + trama
    registro += b"\x00" * ((4 - len(registro) % 4) % 4)  # el formato alinea a 4

    # Muestra de flujo: secuencia, fuente, tasa, total, descartados, interfaces,
    # y sus registros.
    muestra = (
        palabra(1) + palabra(0) + palabra(tasa) + palabra(0) + palabra(0)
        + palabra(5) + palabra(7)
        + palabra(1) + palabra(1) + palabra(len(registro)) + registro
    )

    # Datagrama: version, tipo de direccion del agente, agente, subagente,
    # secuencia, tiempo encendido, y sus muestras.
    return (
        palabra(5) + palabra(1) + socket.inet_aton("10.0.0.1") + palabra(0)
        + palabra(1) + palabra(1000)
        + palabra(1) + palabra(1) + palabra(len(muestra)) + muestra
    )


def main():
    if len(sys.argv) < 2:
        print("uso: enviar_sflow.py <puerto>", file=sys.stderr)
        return 1

    datos = datagrama("192.168.1.5", "8.8.8.8", tasa=1000, tamano_de_trama=1500)
    enchufe = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    enchufe.sendto(datos, ("127.0.0.1", int(sys.argv[1])))
    return 0


if __name__ == "__main__":
    sys.exit(main())
