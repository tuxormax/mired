# Installing MiRed

MiRed is two programs and one configuration file. It needs no database server,
no Docker, nothing else installed.

> MiRed's interface and documentation are written in Spanish. This page exists so
> that people outside Spanish-speaking countries can install it and decide
> whether it is useful to them.

## 1. Install the package

```
sudo dpkg -i mired_1.1-8_amd64.deb
```

Use `arm64` instead of `amd64` for a Raspberry Pi.

When it finishes, open the address it prints — the machine where you installed
it, on port **60072**:

```
http://192.168.1.10:60072
```

**MiRed ships with no default user or password.** The first time you open it you
get the "create administrator" form instead of the login form: you pick the
username and the password. Every other user is created from that one.

Shared default credentials would be, in a public project, a door anyone can go
looking for — scan the port, try them. Having you choose your own from the start
is the only way they don't end up unchanged.

One quirk of the algorithm the passwords are stored with
([TUXOR](https://github.com/tuxormax/tuxor)): **the username or the password must
start or end with one of these signs**

```
+  -  *  %  ^  &  |  <  >  #
```

For example `+admin` as the username, or `*mypassword#` as the password. The
screen tells you as you type and won't let you continue until it's satisfied.

Once the first administrator exists that screen is gone for good: nobody from
outside can create another.

## 2. Network privileges

MiRed runs as two processes: the server, which has no privileges, and the probe,
which is the only one that touches the network. The package grants the probe
exactly two capabilities (`CAP_NET_RAW` and `CAP_NET_ADMIN`) through systemd — it
never runs as root.

Without those capabilities MiRed **still works**: it discovers hosts by knocking
on TCP ports instead of using ARP. That is slower and yields no MAC address, and
the interface says so plainly rather than letting you believe you are seeing the
whole network.

## 3. Create your first network

A "network" in MiRed is a site: Headquarters, Branch 1, Warehouse. Each one lives
in its own database file, so backing up a site means copying one file.

When creating it you are asked for the **subnets to scan**, usually just one:

```
192.168.1.0/24
```

## 4. Scan

On the network screen, the radar button offers two options:

- **Full scan**: discovers hosts, resolves names, checks open ports and queries
  the switches over SNMP. Takes minutes.
- **Presence only**: fast, just tells you who is online right now.

The clock button schedules them. Presence every minute and a full scan every six
hours is a sensible starting point.

## 5. The port map

To learn **which switch port each device is plugged into**, MiRed has to ask the
switch. You need:

1. A **managed switch** with **SNMP enabled**.
2. That credential loaded into MiRed: on **that network's** screen, left menu →
   *Credenciales SNMP* → *Nueva credencial*. **Credentials belong to one network
   and are never shared**, so one client's community is never tried against
   another's switches. SNMP v2c and the read community (often `public`) is enough for
   most switches.

If your switches are unmanaged, MiRed says so explicitly on that screen and
explains what still works without them — which is nearly everything: inventory,
presence and alerts.

### Wi-Fi: add your controller

An access point **has no ports, it has radios**, and the one who knows which
device hangs off which is the controller, not the access point. Without adding
it, half the devices in a modern office — phones, laptops, cameras — show up as
*"unplaced"* on the map.

On **that network's** screen, left menu → *Controladoras WiFi* → *Nueva
controladora* (also per network, never shared). It needs the same address you
use in your own browser (`https://192.168.1.10:8443`), a user — read-only is
enough — and the *site*, which is called `default` on nearly every install.

Leave "require a valid certificate" **off**: almost every on-premise controller
uses a certificate it signed itself, and requiring one would make the feature
useless.

After that, each Wi-Fi device appears on the map under its access point, and the
port carries the name of the network it joined (`Office`, `Guest`). If the
controller stops answering, the screen says so and why — the Wi-Fi does not
quietly vanish from the map.

**UniFi (Ubiquiti)** is supported today, which is what most small sites run.

### Saving the map

The download icon on the map screen. Six formats:

| Format | What for |
|---|---|
| **PNG** | An image, to drop into a document or send over chat |
| **SVG** | Vector: open it in Inkscape to move boxes around or annotate it |
| **PDF** | To print and pin up, or to email |
| **ODS** | A LibreOffice spreadsheet, with two tabs |
| **XLSX** | The same for Excel |
| **CSV** | Plain text: both tables, one after the other |

The three spreadsheets carry **two tables, not one**:

- **Aparatos** (devices) — one row per device: what it is, its IP and MAC, what
  it hangs off, through which port, how certain that is and where it came from.
- **Conexiones** (links) — one row per link, **each cable exactly once**. Free
  ports are listed too, and so is anything connected over the air, which has no
  port at all.

Saving opens your desktop's save dialog and **you pick the folder and the
name**. The file **stays on your machine and nowhere else**: MiRed uploads it to
no cloud and sends it to no one. If you want somebody to have it, you attach it
yourself.

All six carry the site name and **the date the data was measured** across the
top — not the date it was exported. An undated network diagram is worthless a
week later, because you can no longer tell which of the three is the current
one.

### Uploading a sheet you already had

If the site is already documented in a spreadsheet — the usual case for anything
cabled by somebody — you do not have to type it in device by device. On the
network screen: **⋮ → Importar aparatos de una hoja** (import devices from a
sheet).

It is a full screen: **what it is for, the three steps, the file field and the
filling guide right there**, so you have it in front of you while you fill the
sheet. Download the template, fill it in and upload it. **CSV, ODS and XLSX** are
all accepted.

| Column | Required? | What goes in it |
|---|---|---|
| `NOMBRE` | **yes** | What it is called: `D01`, `serv1`, `switch site` |
| `QUE_ES` | **yes** | switch, modem, router, pc, camara, impresora, telefono, servidor, punto de acceso, tv, almacenamiento, otro |
| `PUERTOS` | no | Switches and modems only: how many ports it has |
| `CUELGA_DE` | no | The **name** of the device it hangs off |
| `PUERTO` | no | The port **on that device**: `7`, `LAN 7`, `WAN 1` |
| `UBICACION` | no | Where it physically is: `farmacia`, `cons 5` |
| `IP` · `MAC` · `MODELO` · `NOTAS` | no | Whenever you know them |
| `ACCESO` · `USUARIO` · `CLAVE` · `DIRECCION` | no | How to get into its panel. The password is stored **encrypted** |

Three things worth knowing:

- **The switch gets its own row too**, and everything else hangs off it by naming
  it in `CUELGA_DE`. Row order does not matter.
- **Nothing is written until you say so.** Picking the file shows you, with the
  row number from your own sheet, what will be created, what already exists and
  what cannot be imported and why.
- If your sheet comes from somewhere else, **you do not have to rewrite it**:
  headers like `NODO`, `OBSERVACIONES` or `CONECTADO_A` are recognised, and so
  are the semicolons a Spanish Excel writes and the accents it mangles.

If the file carries passwords, MiRed stores them encrypted but **the file holds
them in the clear**: delete it once you are done. The screen says so.

Uploading the same sheet again duplicates nothing: what already exists is either
updated or left alone, whichever you pick, and **an empty cell never erases data
that was already there**.

## 6. Bandwidth

Two ways, and they complement each other:

- **With managed switches**: MiRed reads each port's counters. Since it already
  knows which device hangs off each port, that tells you who is consuming. It
  needs at least two full scans, because usage is the difference between two
  readings.
- **Without them**: configure your router to export **NetFlow** to port `2055` of
  the machine running MiRed (MikroTik: `IP → Traffic Flow`; pfSense: the
  `softflowd` package). That gives per-device usage, without saying which port.

## Where everything lives

| Path | What it is |
|---|---|
| `/etc/mired/mired.toml` | Configuration |
| `/etc/mired/dispositivos/` | Your own device definitions |
| `/var/lib/mired/mired.db` | Users, permissions and the network registry |
| `/var/lib/mired/redes/` | **One database per network. This is the only thing you need to back up** |

Removing the package never deletes your data, not even on purge.
