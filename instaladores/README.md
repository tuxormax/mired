# Instaladores

Aquí quedan los `.deb` que genera `herramientas/construir.sh`, y **sólo aquí**.

```
./herramientas/construir.sh                      # amd64
./herramientas/construir.sh --arquitectura todas # amd64 y arm64
```

Instalar:

```
sudo dpkg -i instaladores/mired_1.1-8_amd64.deb
```

Los archivos no se versionan —son binarios de 20 MB que se regeneran en un
minuto—, por eso esta carpeta aparece vacía en un clon recién hecho.

## Sobre el paquete de `arm64`

**Va sin el programa de escritorio.** Flutter no compila a `arm64` desde un
equipo `amd64`, y meter un binario de la arquitectura equivocada sería entregar
algo que no arranca; el constructor lo avisa por pantalla. Ese paquete sirve para
dejar una Raspberry corriendo como servicio y verla desde el programa de otro
equipo:

```
sudo systemctl enable --now mired-sonda mired-servidor
```

Para tener el programa nativo en la Raspberry hay que compilarlo en ella misma.
