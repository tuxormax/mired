# Installing MiRed

MiRed is two programs and one configuration file. It needs no database server,
no Docker, nothing else installed.

> MiRed's interface and documentation are written in Spanish. This page exists so
> that people outside Spanish-speaking countries can install it and decide
> whether it is useful to them.

## 1. Install the package

```
sudo dpkg -i mired_1.0-0_amd64.deb
```

Use `arm64` instead of `amd64` for a Raspberry Pi.

When it finishes, open the address it prints — the machine where you installed
it, on port **60072**:

```
http://192.168.1.10:60072
```

**User `usuario-quitado`, password `clave-quitada`.** Change it as soon as you log in.

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
2. That credential loaded into MiRed: key icon on the home panel → *New
   credential*. SNMP v2c and the read community (often `public`) is enough for
   most switches.

If your switches are unmanaged, MiRed says so explicitly on that screen and
explains what still works without them — which is nearly everything: inventory,
presence and alerts.

### Saving the map

The download icon on the map screen. Four formats:

| Format | What for |
|---|---|
| **PNG** | An image, to drop into a document or send over chat |
| **SVG** | Vector: open it in Inkscape to move boxes around or annotate it |
| **PDF** | To print and pin up, or to email |
| **CSV** | For a spreadsheet, one row per port |

The file **lands on your machine and nowhere else**. MiRed uploads it to no
cloud and sends it to no one: the browser builds it and it drops into your
downloads folder like any other download. If you want somebody to have it, you
attach it yourself.

The three drawing formats carry the site name and the date across the top —
an undated network diagram is worthless a week later, because you can no longer
tell which of the three is the current one.

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
