-- Las credenciales SNMP y las controladoras WiFi pasan a ser DE CADA RED.
--
-- Vivian en el catalogo, compartidas por todas las redes, con este argumento:
-- «la misma comunidad sirve para varios sitios, y repetirla red por red es la
-- forma segura de que una quede desactualizada». Es cierto para quien administra
-- una sola instalacion con varias subredes; es falso —y peligroso— para quien
-- lleva las redes de varios clientes, que es justo lo que hace MiRed sin topes.
--
-- Lo que estaba mal de compartirlas:
--  * la comunidad SNMP de un cliente se probaba contra los equipos de otro, y un
--    intento fallido queda en la bitacora del switch ajeno;
--  * dar permiso de lectura sobre UNA red dejaba ver credenciales que abren los
--    switches de TODAS;
--  * borrar la credencial de un sitio la borraba para los demas.
--
-- Las tablas son las mismas que habia en el catalogo, tal cual: no se aprovecha
-- la mudanza para cambiar columnas, porque asi la copia de los datos viejos es
-- un INSERT directo y no hay nada que traducir. Lo que existiera en el catalogo
-- se copia A CADA RED al arrancar, una sola vez, y despues se vacia de alli.

CREATE TABLE credenciales_snmp (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    nombre      TEXT    NOT NULL,
    version     TEXT    NOT NULL CHECK (version IN ('v1', 'v2c', 'v3')),
    comunidad   TEXT,                            -- v1 y v2c
    usuario     TEXT,                            -- v3
    autenticacion_protocolo TEXT CHECK (autenticacion_protocolo IN ('MD5', 'SHA', 'SHA224', 'SHA256', 'SHA384', 'SHA512')),
    autenticacion_clave     TEXT,
    privacidad_protocolo    TEXT CHECK (privacidad_protocolo IN ('DES', 'AES', 'AES192', 'AES256')),
    privacidad_clave        TEXT,
    estatus     INTEGER NOT NULL DEFAULT 1 CHECK (estatus IN (-1, 0, 1)),
    creada      TEXT    NOT NULL,
    modificada  TEXT
);
CREATE UNIQUE INDEX ux_credenciales_snmp_nombre ON credenciales_snmp (nombre) WHERE estatus >= 0;

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
