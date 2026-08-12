-- Consumo medido con los flujos que exporta el router (NetFlow).
--
-- Es la salida para los sitios SIN switches administrables: no da el puerto,
-- pero si da quien consume, porque el trafico a internet pasa por el router si o
-- si. El router hace el trabajo pesado y MiRed solo recibe y suma.

CREATE TABLE trafico_flujos (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    equipo_id   INTEGER REFERENCES equipos (id) ON DELETE SET NULL,
    ip          TEXT    NOT NULL,
    momento     TEXT    NOT NULL,
    -- Bytes del periodo, no acumulados: el router ya manda deltas.
    bytes_sube  INTEGER NOT NULL DEFAULT 0,
    bytes_baja  INTEGER NOT NULL DEFAULT 0,
    conversaciones INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX ix_trafico_flujos_ip ON trafico_flujos (ip, momento);
CREATE INDEX ix_trafico_flujos_momento ON trafico_flujos (momento);
