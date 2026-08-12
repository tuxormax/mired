# Aportar un dispositivo al catálogo

Esta es la parte de MiRed donde **no hace falta programar**. Reconocer que un
aparato es una impresora, una cámara o un Chromecast no se escribe en código: se
escribe en un archivo de texto que cualquiera puede hacer.

## La forma fácil: que MiRed lo escriba por usted

1. Entre a la red y busque en la lista un equipo que aparezca **sin reconocer**.
2. Abra su ficha y pulse **«Proponer definición»**.
3. MiRed genera el archivo ya relleno con lo que vio de ese aparato: su
   fabricante, los puertos que tiene abiertos y lo que contestaron sus servicios.
4. Póngale nombre, revise que no sobre nada, y cópielo.

Ese archivo ya sirve. Guárdelo en `/etc/mired/dispositivos/` y reinicie el
servicio para usarlo solo en su instalación:

```
sudo systemctl restart mired-servidor
```

Y si lo manda al repositorio, lo tendrá todo el mundo.

## Cómo es el archivo

```toml
nombre = "Impresora HP"
categoria = "impresora"
icono = "print"
descripcion = "Impresora de red HP con puerto de impresión crudo."
aporta = "su nombre, si quiere aparecer"

[coincidencias]
fabricantes = ["HP", "Hewlett"]
puertos_todos = [9100]
```

### Las condiciones

Se pueden combinar. **Todas las que ponga tienen que cumplirse**; dentro de cada
una basta con que coincida un elemento.

| Condición | Qué hace |
|---|---|
| `prefijos_mac` | Los primeros tres bytes de la MAC (`["b8:27:eb"]`). Es la señal más fuerte: no depende de que el aparato conteste nada |
| `fabricantes` | Texto dentro del fabricante deducido de la MAC |
| `puertos_todos` | Exige que **todos** esos puertos estén abiertos |
| `puertos_alguno` | Basta con que **uno** lo esté |
| `banner_contiene` | Texto en lo que contestaron los servicios del aparato |
| `nombre_contiene` | Texto en el nombre descubierto del equipo |
| `snmp_contiene` | Texto en lo que el aparato dice de sí mismo por SNMP |

### Qué gana cuando dos definiciones coinciden

Gana **la más específica**, no la primera ni la que se llame distinto. Si una dice
«es HP» y otra dice «es HP y tiene el 9100 abierto», la segunda describe mejor al
aparato y es la que se usa.

Eso significa que puede escribir definiciones amplias sin miedo: no van a tapar a
las buenas.

### Lo que no funciona

- **Una definición sin ninguna condición** coincidiría con todo. MiRed la ignora
  a propósito: eso no es un dispositivo, es un archivo a medio escribir.
- **Condiciones demasiado amplias** (solo `puertos_alguno = [80]`) etiquetan medio
  mundo como lo mismo. Si su aparato solo se distingue por el 80, mejor agregue
  el fabricante o el prefijo de MAC.

## Dónde se guardan

| Carpeta | Qué es |
|---|---|
| `/usr/share/mired/dispositivos/` | Las que trae el paquete. No las edite: una actualización las pisa |
| `/etc/mired/dispositivos/` | Las suyas. **Mandan sobre las anteriores** |

Un archivo suyo con el mismo nombre que uno del paquete lo reemplaza por
completo. Así puede corregir una definición que le funciona mal sin esperar a que
nadie le acepte el cambio.

## Mandar el aporte

Abra una propuesta en <https://github.com/tuxormax/mired> con su archivo `.toml`.
Se revisa que sea válido, que no duplique otro y que la condición no sea tan
amplia que etiquete medio mundo.

Ponga su nombre en `aporta` si quiere aparecer como autor.
