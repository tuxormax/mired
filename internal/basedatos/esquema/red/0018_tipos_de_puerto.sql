-- Un puerto no es solo LAN o WAN.
--
-- 0011 dejo la lista en ('lan', 'wan') porque con eso alcanzaba para dibujar un
-- switch tonto colgado de un modem. En cuanto el mapa empezo a decir POR DONDE
-- entra y sale cada cable, la lista se quedo corta: un modem de casa tiene su
-- puerto DMZ, y un switch con fibra tiene sus SFP. Llamarles "LAN" a todos
-- seria escribir en el mapa algo que no esta en el aparato.
--
-- SQLite no sabe cambiar un CHECK: hay que rehacer la tabla. Y como
-- `enlaces_fisicos` apunta a esta con ON DELETE CASCADE, **soltar la tabla vieja
-- con el hijo todavia colgando se llevaria por delante todos los cables**. Por
-- eso se rehacen las dos y en este orden: se copian las dos nuevas, se suelta
-- primero el hijo —soltar un hijo no arrastra a nadie—, despues el padre, que
-- para entonces ya no tiene a quien arrastrar, y al final se renombran. Al
-- renombrar el padre, SQLite reescribe solo la referencia del hijo.
--
-- defer_foreign_keys aguanta las llaves foraneas hasta el commit: durante el
-- baile hay momentos en que no cuadran, y al final si.
PRAGMA defer_foreign_keys = ON;

CREATE TABLE puertos_fisicos_nuevo (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    equipo_id      INTEGER NOT NULL REFERENCES equipos (id) ON DELETE CASCADE,
    numero         INTEGER NOT NULL CHECK (numero BETWEEN 1 AND 512),
    -- La lista sigue cerrada: un tipo libre acabaria con "Lan", "LAN1" y "lan "
    -- conviviendo, y el mapa nombraria el mismo puerto de tres maneras.
    tipo           TEXT    NOT NULL CHECK (
                       tipo IN ('lan', 'wan', 'dmz', 'sfp', 'consola')
                   ),
    velocidad_mbps INTEGER CHECK (velocidad_mbps IS NULL OR velocidad_mbps > 0),
    notas          TEXT,
    creado_en      TEXT    NOT NULL
);

INSERT INTO puertos_fisicos_nuevo
    (id, equipo_id, numero, tipo, velocidad_mbps, notas, creado_en)
SELECT id, equipo_id, numero, tipo, velocidad_mbps, notas, creado_en
FROM puertos_fisicos;

CREATE TABLE enlaces_fisicos_nuevo (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    puerto_origen_id  INTEGER NOT NULL
                      REFERENCES puertos_fisicos_nuevo (id) ON DELETE CASCADE,
    puerto_destino_id INTEGER
                      REFERENCES puertos_fisicos_nuevo (id) ON DELETE CASCADE,
    equipo_destino_id INTEGER REFERENCES equipos (id) ON DELETE CASCADE,
    origen_dato       TEXT NOT NULL CHECK (
                          origen_dato IN ('manual', 'snmp', 'lldp', 'cdp', 'inferido')
                      ),
    notas             TEXT,
    creado_en         TEXT NOT NULL,
    CHECK (
        (puerto_destino_id IS NOT NULL AND equipo_destino_id IS NULL)
        OR
        (puerto_destino_id IS NULL AND equipo_destino_id IS NOT NULL)
    )
);

INSERT INTO enlaces_fisicos_nuevo
    (id, puerto_origen_id, puerto_destino_id, equipo_destino_id, origen_dato,
     notas, creado_en)
SELECT id, puerto_origen_id, puerto_destino_id, equipo_destino_id, origen_dato,
       notas, creado_en
FROM enlaces_fisicos;

DROP TABLE enlaces_fisicos;
DROP TABLE puertos_fisicos;

ALTER TABLE puertos_fisicos_nuevo RENAME TO puertos_fisicos;
ALTER TABLE enlaces_fisicos_nuevo RENAME TO enlaces_fisicos;

CREATE UNIQUE INDEX ux_puertos_fisicos ON puertos_fisicos (equipo_id, numero, tipo);
CREATE INDEX ix_puertos_fisicos_equipo ON puertos_fisicos (equipo_id);

CREATE UNIQUE INDEX ux_enlaces_fisicos_origen ON enlaces_fisicos (puerto_origen_id);
CREATE UNIQUE INDEX ux_enlaces_fisicos_destino
    ON enlaces_fisicos (puerto_destino_id)
    WHERE puerto_destino_id IS NOT NULL;
CREATE INDEX ix_enlaces_fisicos_destino ON enlaces_fisicos (puerto_destino_id);
CREATE INDEX ix_enlaces_fisicos_equipo ON enlaces_fisicos (equipo_destino_id);
