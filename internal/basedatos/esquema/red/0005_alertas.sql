-- Alertas de cambio: lo que convierte un inventario en una herramienta que avisa.
--
-- El valor de esto no es escanear, es enterarse: que se conecto algo que no
-- conocias, que un equipo lleva dias apagado, que a un servidor le aparecio un
-- puerto abierto que antes no tenia.
--
-- Todo va por red a proposito: la sucursal le avisa a quien lleva la sucursal.

-- Que se vigila en esta red y con que umbral.
CREATE TABLE reglas_alerta (
    tipo       TEXT PRIMARY KEY CHECK (tipo IN (
                   'equipo_nuevo', 'equipo_ausente', 'puerto_nuevo',
                   'cambio_ip', 'cambio_puerto_switch', 'red_sin_reportar')),
    activa     INTEGER NOT NULL DEFAULT 1 CHECK (activa IN (0, 1)),
    -- umbral se interpreta segun la regla: minutos de ausencia, por ejemplo.
    umbral     INTEGER NOT NULL DEFAULT 0,
    modificada TEXT
);

-- Las reglas nacen encendidas menos la de ausencia, que sin umbral pensado
-- avisaria de cada celular que se va de la casa.
INSERT INTO reglas_alerta (tipo, activa, umbral) VALUES
    ('equipo_nuevo',         1, 0),
    ('equipo_ausente',       1, 1440),   -- avisa tras 24 horas sin aparecer
    ('puerto_nuevo',         1, 0),
    ('cambio_ip',            1, 0),
    ('cambio_puerto_switch', 1, 0),
    ('red_sin_reportar',     1, 120);    -- 2 horas sin un escaneo terminado

-- A donde se avisa. Cada red tiene sus destinos.
CREATE TABLE destinos_alerta (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    nombre   TEXT    NOT NULL,
    tipo     TEXT    NOT NULL CHECK (tipo IN ('webhook', 'ntfy', 'telegram', 'correo')),
    -- destino es la URL, el tema de ntfy, el chat de Telegram o el correo.
    destino  TEXT    NOT NULL,
    -- extra guarda lo que cada tipo necesita de mas (token, servidor SMTP...).
    extra    TEXT,
    activo   INTEGER NOT NULL DEFAULT 1 CHECK (activo IN (0, 1)),
    estatus  INTEGER NOT NULL DEFAULT 1 CHECK (estatus IN (-1, 0, 1)),
    creado   TEXT    NOT NULL,
    ultimo_envio TEXT,
    ultimo_error TEXT
);
CREATE UNIQUE INDEX ux_destinos_alerta_nombre ON destinos_alerta (nombre) WHERE estatus >= 0;

CREATE TABLE alertas (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    tipo      TEXT    NOT NULL,
    momento   TEXT    NOT NULL,
    equipo_id INTEGER REFERENCES equipos (id) ON DELETE SET NULL,
    titulo    TEXT    NOT NULL,
    detalle   TEXT,
    -- vista la marca una persona; enviada la marca el propio motor.
    vista     INTEGER NOT NULL DEFAULT 0 CHECK (vista IN (0, 1)),
    enviada   INTEGER NOT NULL DEFAULT 0 CHECK (enviada IN (0, 1)),
    -- huella evita repetir la misma alerta en cada escaneo.
    huella    TEXT    NOT NULL
);
CREATE UNIQUE INDEX ux_alertas_huella ON alertas (huella);
CREATE INDEX ix_alertas_momento ON alertas (momento);
CREATE INDEX ix_alertas_vista ON alertas (vista);
