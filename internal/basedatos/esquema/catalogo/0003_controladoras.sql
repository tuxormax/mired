-- Controladoras WiFi (UniFi y compatibles).
--
-- Un punto de acceso no tiene puertos: tiene antenas, y quien sabe que aparato
-- esta colgado de cual es la controladora, no el aparato. Sin preguntarle a
-- ella, todo el WiFi de un sitio aparece como "sin ubicar" en el mapa, que es
-- justo la mitad de los equipos de una oficina moderna.
--
-- Vive en el catalogo y no en cada red por la misma razon que las credenciales
-- SNMP: una controladora suele atender varios sitios, y repetirla red por red es
-- la forma segura de que una quede desactualizada.

CREATE TABLE controladoras (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    nombre      TEXT    NOT NULL,
    tipo        TEXT    NOT NULL DEFAULT 'unifi' CHECK (tipo IN ('unifi')),
    url         TEXT    NOT NULL,               -- https://192.168.1.10:8443
    usuario     TEXT    NOT NULL,
    clave       TEXT    NOT NULL,
    -- El "sitio" de UniFi: una misma controladora atiende varios, y el de por
    -- omision se llama literalmente "default".
    sitio       TEXT    NOT NULL DEFAULT 'default',
    -- Casi toda controladora casera usa un certificado que ella misma se firmo.
    -- Exigir uno valido dejaria la funcion inservible en la mayoria de las
    -- instalaciones; se deja elegir, y el valor por omision es el que funciona.
    verificar_tls INTEGER NOT NULL DEFAULT 0 CHECK (verificar_tls IN (0, 1)),
    estatus     INTEGER NOT NULL DEFAULT 1 CHECK (estatus IN (-1, 0, 1)),
    creada      TEXT    NOT NULL,
    modificada  TEXT,
    -- Cuando contesto por ultima vez y que dijo. Que una controladora lleve dias
    -- sin contestar es un dato, no un silencio.
    ultimo_exito TEXT,
    ultimo_error TEXT
);
CREATE UNIQUE INDEX ux_controladoras_nombre ON controladoras (nombre) WHERE estatus >= 0;
