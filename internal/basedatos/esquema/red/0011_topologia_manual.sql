-- Lo que el usuario sabe y ninguna herramienta puede averiguar.
--
-- MiRed ya distinguia el enlace CONFIRMADO (se lo dijo el switch por SNMP, LLDP
-- o CDP) del INFERIDO (varias MAC en la misma boca, o sea algo no administrable
-- atras). Falta la tercera fuente: lo que una persona DECLARA porque tiene el
-- cable delante. Sin esto, la red domestica tipica —modem del ISP, switch tonto,
-- PC, antena, DVR— se queda sin mapa de puertos aunque su dueno sepa
-- perfectamente como esta cableada.
--
-- No es cosa solo de switches tontos: **cualquier equipo puede necesitar declarar
-- sus bocas a mano**, incluido un modem administrable que no habla SNMP hacia la
-- LAN, que es el caso mas comun en Mexico (Telmex y casi todo ISP). Por eso no
-- hay una tabla aparte de "equipos manuales": el origen del dato es una columna,
-- no un mundo paralelo.

-- ------------------------------------------------------------ los equipos --

-- modelo y notas son universales: sirven igual para lo descubierto y para lo
-- declarado. (notas ya existia desde 0002.)
ALTER TABLE equipos ADD COLUMN modelo TEXT;

-- origen dice si el equipo lo encontro un barrido o lo tecleo una persona. Un
-- switch tonto NUNCA va a aparecer en un escaneo —no tiene direccion— y sin esta
-- columna no habria forma de distinguir "no contesto" de "no existe".
ALTER TABLE equipos ADD COLUMN origen TEXT NOT NULL DEFAULT 'descubierto'
    CHECK (origen IN ('descubierto', 'manual'));

-- conexion solo aplica a equipos terminales (PC, laptop, celular): por cable o
-- por WiFi. En un switch o un router no significa nada y se queda en NULL.
ALTER TABLE equipos ADD COLUMN conexion TEXT
    CHECK (conexion IS NULL OR conexion IN ('cable', 'wifi'));

-- ------------------------------------------------------------ las bocas --

-- Las bocas FISICAS de un equipo, tal como se ven por fuera.
--
-- Va aparte de `interfaces` a proposito: `interfaces` son las que el equipo
-- ANUNCIA por SNMP, con su indice interno, y solo existen si el equipo habla.
-- Estas son las que alguien conto mirando el aparato. Mezclarlas obligaria a
-- inventar un indice SNMP para un switch que no tiene ni direccion IP.
CREATE TABLE puertos_fisicos (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    equipo_id      INTEGER NOT NULL REFERENCES equipos (id) ON DELETE CASCADE,
    numero         INTEGER NOT NULL CHECK (numero BETWEEN 1 AND 512),
    tipo           TEXT    NOT NULL CHECK (tipo IN ('lan', 'wan')),
    -- NULL cuando no se sabe: mejor vacio que un 100 inventado.
    velocidad_mbps INTEGER CHECK (velocidad_mbps IS NULL OR velocidad_mbps > 0),
    notas          TEXT,
    creado_en      TEXT    NOT NULL
);
CREATE UNIQUE INDEX ux_puertos_fisicos ON puertos_fisicos (equipo_id, numero, tipo);
CREATE INDEX ix_puertos_fisicos_equipo ON puertos_fisicos (equipo_id);

-- ------------------------------------------------------------ los cables --

-- Un cable entre dos bocas, venga de donde venga el dato.
--
-- Se llama `enlaces_fisicos` y no `enlaces` porque `enlaces` ya existe desde
-- 0004 y es otra cosa: ahi se guarda lo que un switch ANUNCIA de su vecino
-- (nombre, chasis, puerto remoto como texto), que es la materia prima de LLDP y
-- CDP. Esta tabla es el cable ya resuelto, con las dos puntas apuntando a filas
-- de la base.
--
-- `origen_dato` es lo que hace que esto no sea "la tabla de lo manual": el dia
-- que la fase 4 resuelva un tramo por SNMP, escribe aqui con su propio origen y
-- el mapa no tiene dos sistemas que reconciliar a mano.
CREATE TABLE enlaces_fisicos (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    puerto_origen_id  INTEGER NOT NULL REFERENCES puertos_fisicos (id) ON DELETE CASCADE,
    -- Boca contra boca: switch tonto con switch tonto, o switch con modem.
    puerto_destino_id INTEGER REFERENCES puertos_fisicos (id) ON DELETE CASCADE,
    -- El destino no tiene bocas declaradas —una laptop, un DVR con un solo
    -- cable—. Obligar a inventarle un "puerto 1" a un aparato con una sola
    -- salida seria ensuciar la base para cuadrar un modelo.
    equipo_destino_id INTEGER REFERENCES equipos (id) ON DELETE CASCADE,
    origen_dato       TEXT NOT NULL CHECK (
                          origen_dato IN ('manual', 'snmp', 'lldp', 'cdp', 'inferido')
                      ),
    notas             TEXT,
    creado_en         TEXT NOT NULL,
    -- Una punta y solo una: o boca, o equipo.
    CHECK (
        (puerto_destino_id IS NOT NULL AND equipo_destino_id IS NULL)
        OR
        (puerto_destino_id IS NULL AND equipo_destino_id IS NOT NULL)
    )
);
-- Una boca lleva UN cable. Si se declara otro, se reemplaza el anterior en vez
-- de dejar dos verdades incompatibles colgando de la misma boca.
CREATE UNIQUE INDEX ux_enlaces_fisicos_origen ON enlaces_fisicos (puerto_origen_id);
CREATE INDEX ix_enlaces_fisicos_destino ON enlaces_fisicos (puerto_destino_id);
CREATE INDEX ix_enlaces_fisicos_equipo ON enlaces_fisicos (equipo_destino_id);
