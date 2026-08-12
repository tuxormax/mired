-- Equipos descubiertos en la red, con sus puertos y servicios.
--
-- La identidad de un equipo es su MAC, no su IP: la IP cambia sola con DHCP y
-- tratarla como identidad duplica cada aparato cada vez que se reconecta. Cuando
-- no hay MAC (equipo en otra subred, detras de un router), se usa la IP como
-- identidad y se anota que fue asi, para no mentir sobre la certeza del dato.

CREATE TABLE equipos (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    -- identidad es la MAC normalizada (aa:bb:cc:dd:ee:ff) o "ip:1.2.3.4".
    identidad   TEXT    NOT NULL,
    ip          TEXT    NOT NULL,
    mac         TEXT,
    fabricante  TEXT,
    -- nombre es el que se descubrio (DNS inverso, mDNS, NetBIOS).
    nombre      TEXT,
    -- alias es el que le puso una persona. Manda sobre el descubierto.
    alias       TEXT,
    tipo        TEXT,                             -- lo llena el catalogo (fase 6)
    subred      TEXT,
    -- como se vio la ultima vez: arp, icmp o tcp. Dice cuanta certeza hay.
    metodo      TEXT,
    presente    INTEGER NOT NULL DEFAULT 1 CHECK (presente IN (0, 1)),
    primera_vez TEXT    NOT NULL,
    ultima_vez  TEXT    NOT NULL,
    notas       TEXT,
    estatus     INTEGER NOT NULL DEFAULT 1 CHECK (estatus IN (-1, 0, 1)),
    modificado  TEXT
);
CREATE UNIQUE INDEX ux_equipos_identidad ON equipos (identidad) WHERE estatus >= 0;
CREATE INDEX ix_equipos_ip ON equipos (ip);
CREATE INDEX ix_equipos_presente ON equipos (presente);
CREATE INDEX ix_equipos_ultima_vez ON equipos (ultima_vez);

CREATE TABLE puertos (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    equipo_id   INTEGER NOT NULL REFERENCES equipos (id) ON DELETE CASCADE,
    numero      INTEGER NOT NULL CHECK (numero BETWEEN 1 AND 65535),
    protocolo   TEXT    NOT NULL DEFAULT 'tcp' CHECK (protocolo IN ('tcp', 'udp')),
    servicio    TEXT,
    banner      TEXT,
    abierto     INTEGER NOT NULL DEFAULT 1 CHECK (abierto IN (0, 1)),
    primera_vez TEXT    NOT NULL,
    ultima_vez  TEXT    NOT NULL
);
CREATE UNIQUE INDEX ux_puertos_equipo ON puertos (equipo_id, numero, protocolo);
CREATE INDEX ix_puertos_abierto ON puertos (abierto);

-- Cada escaneo deja constancia de que equipos vio, para poder comparar contra el
-- anterior. De esa comparacion salen las alertas de la fase 7.
CREATE TABLE escaneo_equipos (
    escaneo_id INTEGER NOT NULL REFERENCES escaneos (id) ON DELETE CASCADE,
    equipo_id  INTEGER NOT NULL REFERENCES equipos (id) ON DELETE CASCADE,
    ip         TEXT    NOT NULL,
    nuevo      INTEGER NOT NULL DEFAULT 0 CHECK (nuevo IN (0, 1)),
    PRIMARY KEY (escaneo_id, equipo_id)
);
