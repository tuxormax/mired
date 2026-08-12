-- Trafico por puerto, medido con los contadores del propio switch.
--
-- Escanear NO mide trafico: para saber quien consume hay que ver el trafico, y
-- con switches cada equipo solo ve lo suyo. Pero el switch YA lleva la cuenta de
-- los bytes que pasan por cada boca; solo hay que preguntarsela. Como MiRed ya
-- sabe que aparato cuelga de cada boca, eso da "quien consume" sin capturar un
-- solo paquete y a costo casi nulo.

CREATE TABLE muestras_trafico (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    switch_id       INTEGER NOT NULL REFERENCES equipos (id) ON DELETE CASCADE,
    interfaz_indice INTEGER NOT NULL,
    momento         TEXT    NOT NULL,
    -- Los contadores son acumulados desde que el switch encendio, y se
    -- desbordan. El dato util es la RESTA entre dos muestras, no el numero.
    bytes_entrada   INTEGER NOT NULL,
    bytes_salida    INTEGER NOT NULL,
    -- bits por segundo ya calculados contra la muestra anterior. Se guardan
    -- porque recalcularlos en cada consulta obligaria a leer siempre la muestra
    -- de al lado, y una grafica pide cientos de puntos.
    bps_entrada     INTEGER,
    bps_salida      INTEGER
);
CREATE INDEX ix_muestras_trafico ON muestras_trafico (switch_id, interfaz_indice, momento);
CREATE INDEX ix_muestras_momento ON muestras_trafico (momento);
