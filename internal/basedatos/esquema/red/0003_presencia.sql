-- Historial de presencia: cuando se conecto y cuando se fue cada equipo.
--
-- Se guarda un renglon SOLO cuando cambia el estado, no en cada barrido. Con un
-- barrido por minuto, guardar siempre serian 1 440 renglones por equipo al dia
-- para decir lo mismo; guardando cambios son dos o tres.
--
-- De aqui salen las dos preguntas que la gente hace de verdad: "¿a que hora
-- llega el celular de fulano?" y "¿desde cuando esta apagado ese equipo?".

CREATE TABLE eventos_presencia (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    equipo_id INTEGER NOT NULL REFERENCES equipos (id) ON DELETE CASCADE,
    momento   TEXT    NOT NULL,
    presente  INTEGER NOT NULL CHECK (presente IN (0, 1)),
    ip        TEXT
);
CREATE INDEX ix_eventos_presencia_equipo ON eventos_presencia (equipo_id, momento);
CREATE INDEX ix_eventos_presencia_momento ON eventos_presencia (momento);
