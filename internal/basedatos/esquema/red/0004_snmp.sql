-- Lo que solo se sabe preguntandole a un switch administrable.
--
-- Aqui vive la respuesta a "¿que esta conectado en el puerto 7?", que es lo que
-- ninguna herramienta puede averiguar barriendo la red: hay que preguntarselo al
-- switch, y solo contesta si es administrable.

-- Ficha SNMP de un equipo. Va aparte de equipos porque la mayoria de los
-- aparatos no habla SNMP y no tiene sentido dejarles diez columnas vacias.
CREATE TABLE equipos_snmp (
    equipo_id    INTEGER PRIMARY KEY REFERENCES equipos (id) ON DELETE CASCADE,
    nombre       TEXT,
    descripcion  TEXT,
    contacto     TEXT,
    ubicacion    TEXT,
    object_id    TEXT,
    encendido_ms INTEGER,
    es_switch    INTEGER NOT NULL DEFAULT 0 CHECK (es_switch IN (0, 1)),
    credencial   TEXT,
    consultado   TEXT NOT NULL
);

-- Las bocas de un equipo administrable.
CREATE TABLE interfaces (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    equipo_id      INTEGER NOT NULL REFERENCES equipos (id) ON DELETE CASCADE,
    indice         INTEGER NOT NULL,
    nombre         TEXT,
    descripcion    TEXT,
    alias          TEXT,
    mac            TEXT,
    tipo           INTEGER,
    activa         INTEGER NOT NULL DEFAULT 0 CHECK (activa IN (0, 1)),
    velocidad_mbps INTEGER,
    ultima_vez     TEXT NOT NULL
);
CREATE UNIQUE INDEX ux_interfaces_equipo ON interfaces (equipo_id, indice);

-- Que MAC ve cada switch en cada boca. Este es el mapa de puertos.
--
-- confirmado = 1 significa que en esa boca hay UNA sola MAC: ahi cuelga ese
-- aparato y nada mas. Con varias MAC en la misma boca, atras hay un switch no
-- administrable o un punto de acceso, y lo unico honesto es decir "estos cuelgan
-- del puerto 7, en grupo", no inventar un puerto para cada uno.
CREATE TABLE conexiones_puerto (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    switch_id       INTEGER NOT NULL REFERENCES equipos (id) ON DELETE CASCADE,
    interfaz_indice INTEGER NOT NULL,
    mac             TEXT    NOT NULL,
    equipo_id       INTEGER REFERENCES equipos (id) ON DELETE SET NULL,
    confirmado      INTEGER NOT NULL DEFAULT 0 CHECK (confirmado IN (0, 1)),
    cuantos_en_boca INTEGER NOT NULL DEFAULT 1,
    ultima_vez      TEXT    NOT NULL
);
CREATE UNIQUE INDEX ux_conexiones_puerto ON conexiones_puerto (switch_id, interfaz_indice, mac);
CREATE INDEX ix_conexiones_equipo ON conexiones_puerto (equipo_id);

-- Enlaces entre equipos administrables, anunciados por LLDP o CDP. Son los que
-- dibujan el esqueleto del mapa: switch con switch.
CREATE TABLE enlaces (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    equipo_id       INTEGER NOT NULL REFERENCES equipos (id) ON DELETE CASCADE,
    interfaz_local  TEXT    NOT NULL,
    vecino_nombre   TEXT,
    vecino_puerto   TEXT,
    vecino_chasis   TEXT,
    vecino_equipo_id INTEGER REFERENCES equipos (id) ON DELETE SET NULL,
    origen          TEXT    NOT NULL DEFAULT 'lldp' CHECK (origen IN ('lldp', 'cdp')),
    ultima_vez      TEXT    NOT NULL
);
CREATE UNIQUE INDEX ux_enlaces ON enlaces (equipo_id, interfaz_local, origen);
