-- Consumo por APLICACION, no solo por equipo.
--
-- Lo llena el paquete opcional mired-dpi. La tabla existe siempre aunque no se
-- instale: crearla al vuelo obligaria a migrar una base en produccion el dia que
-- alguien decida instalarlo, que es justo el peor momento.
--
-- El nombre no sale de descifrar nada: sale del saludo de TLS, de la cabecera
-- Host de HTTP o de las consultas de DNS, que viajan en claro.

CREATE TABLE trafico_aplicaciones (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    equipo_id   INTEGER REFERENCES equipos (id) ON DELETE SET NULL,
    ip          TEXT    NOT NULL,
    aplicacion  TEXT    NOT NULL,
    -- De donde salio el nombre: tls, http, dns, puerto o desconocido. Se guarda
    -- porque no todos valen igual: un nombre sacado del saludo de TLS dice con
    -- quien se hablo, y uno sacado del puerto solo dice de que tipo de servicio
    -- se trata.
    como        TEXT    NOT NULL DEFAULT 'desconocido',
    momento     TEXT    NOT NULL,
    -- Bytes del periodo, no acumulados.
    bytes       INTEGER NOT NULL DEFAULT 0,
    paquetes    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX ix_trafico_aplicaciones_momento ON trafico_aplicaciones (momento);
CREATE INDEX ix_trafico_aplicaciones_ip ON trafico_aplicaciones (ip, momento);
