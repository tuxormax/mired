-- Esquema inicial del CATALOGO: la base global de MiRed.
--
-- Aqui vive solo lo que existe por encima de las redes: quien entra, que puede
-- ver, y el registro de que redes hay. Todo lo operativo de una red (equipos,
-- puertos, topologia, alertas) vive en el archivo .db de esa red y NO aqui.
--
-- Fechas: SQLite no tiene tipo fecha. Se guardan como TEXT en ISO 8601 con zona
-- horaria (2026-08-12T19:42:18-06:00). Ese formato es el contrato del proyecto.
-- Borrado suave: estatus -1 borrado, 0 inactivo, 1 activo. Los indices unicos
-- son parciales (WHERE estatus >= 0) para poder reutilizar el nombre de algo
-- borrado.

CREATE TABLE usuarios (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    usuario      TEXT    NOT NULL,
    nombre       TEXT    NOT NULL,
    clave        TEXT    NOT NULL,              -- bcrypt
    correo       TEXT,
    superadmin   INTEGER NOT NULL DEFAULT 0 CHECK (superadmin IN (0, 1)),
    estatus      INTEGER NOT NULL DEFAULT 1 CHECK (estatus IN (-1, 0, 1)),
    creado       TEXT    NOT NULL,
    modificado   TEXT,
    ultimo_acceso TEXT
);
CREATE UNIQUE INDEX ux_usuarios_usuario ON usuarios (usuario) WHERE estatus >= 0;

CREATE TABLE sesiones (
    id          TEXT    PRIMARY KEY,            -- token opaco, 32 bytes en hexadecimal
    usuario_id  INTEGER NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    creada      TEXT    NOT NULL,
    ultimo_uso  TEXT    NOT NULL,
    expira      TEXT    NOT NULL,
    direccion   TEXT,
    agente      TEXT
);
CREATE INDEX ix_sesiones_usuario ON sesiones (usuario_id);
CREATE INDEX ix_sesiones_expira ON sesiones (expira);

-- Registro de redes. La columna archivo apunta al .db de esa red dentro de
-- redes/. La clave es la parte legible del nombre del archivo y NO cambia aunque
-- se renombre la red: el nombre bonito vive en la columna nombre y dentro del
-- propio archivo.
CREATE TABLE redes (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    clave         TEXT    NOT NULL,             -- matriz-a1b2
    nombre        TEXT    NOT NULL,             -- Matriz
    descripcion   TEXT,
    archivo       TEXT    NOT NULL,             -- matriz-a1b2.db
    estatus       INTEGER NOT NULL DEFAULT 1 CHECK (estatus IN (-1, 0, 1)),
    creada        TEXT    NOT NULL,
    modificada    TEXT,

    -- Resumen para el panel de inicio, actualizado al terminar cada escaneo.
    -- Existe para que la pantalla principal no tenga que abrir treinta archivos
    -- SQLite solo para decir cuantos equipos hay en cada sitio.
    equipos           INTEGER NOT NULL DEFAULT 0,
    equipos_presentes INTEGER NOT NULL DEFAULT 0,
    ultimo_escaneo    TEXT,
    alertas_abiertas  INTEGER NOT NULL DEFAULT 0,
    resumen_al        TEXT
);
CREATE UNIQUE INDEX ux_redes_clave ON redes (clave) WHERE estatus >= 0;
CREATE UNIQUE INDEX ux_redes_nombre ON redes (nombre) WHERE estatus >= 0;

-- Quien ve que red. El superadministrador no necesita fila aqui: ve todas.
CREATE TABLE permisos_red (
    usuario_id INTEGER NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    red_id     INTEGER NOT NULL REFERENCES redes (id) ON DELETE CASCADE,
    nivel      TEXT    NOT NULL CHECK (nivel IN ('lectura', 'escritura', 'administra')),
    otorgado   TEXT    NOT NULL,
    PRIMARY KEY (usuario_id, red_id)
);
CREATE INDEX ix_permisos_red ON permisos_red (red_id);

-- Credenciales SNMP reutilizables entre redes. Viven en el catalogo justamente
-- para no repetirlas sitio por sitio.
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

CREATE TABLE claves_api (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    usuario_id  INTEGER NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    nombre      TEXT    NOT NULL,
    huella      TEXT    NOT NULL,               -- hash de la clave; la clave nunca se guarda
    prefijo     TEXT    NOT NULL,               -- primeros caracteres, para reconocerla en la lista
    estatus     INTEGER NOT NULL DEFAULT 1 CHECK (estatus IN (-1, 0, 1)),
    creada      TEXT    NOT NULL,
    expira      TEXT,
    ultimo_uso  TEXT
);
CREATE UNIQUE INDEX ux_claves_api_huella ON claves_api (huella);
CREATE INDEX ix_claves_api_usuario ON claves_api (usuario_id);

CREATE TABLE log_actividad (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    momento    TEXT    NOT NULL,
    usuario_id INTEGER REFERENCES usuarios (id) ON DELETE SET NULL,
    red_id     INTEGER REFERENCES redes (id) ON DELETE SET NULL,
    modulo     TEXT    NOT NULL,
    accion     TEXT    NOT NULL,
    detalle    TEXT,
    direccion  TEXT
);
CREATE INDEX ix_log_actividad_momento ON log_actividad (momento);

-- Bitacora de errores con el formato de la casa: soporte ve todos los errores
-- sin depender de que el usuario los reporte.
CREATE TABLE log_errores (
    id          TEXT PRIMARY KEY,               -- ERR-9F3A2B1C
    momento     TEXT NOT NULL,
    modulo      TEXT NOT NULL,
    accion      TEXT NOT NULL,
    causa       TEXT NOT NULL,
    ruta        TEXT,
    tabla       TEXT,
    endpoint    TEXT,
    origen      TEXT,                            -- servidor, sonda o interfaz
    red_id      INTEGER,
    usuario_id  INTEGER,
    version     TEXT,
    build       TEXT,
    so          TEXT,
    trayectoria TEXT,                            -- JSON con las ultimas acciones
    peticion    TEXT,                            -- JSON del cuerpo enviado
    respuesta   TEXT,                            -- JSON del cuerpo recibido
    pila        TEXT,
    error       TEXT NOT NULL,                   -- mensaje crudo del sistema
    extra       TEXT
);
CREATE INDEX ix_log_errores_momento ON log_errores (momento);

-- Version del sistema instalada, para el pie de la interfaz y los reportes de
-- error. Una fila por despliegue.
CREATE TABLE versiones_sistema (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    version   TEXT NOT NULL,
    revision  INTEGER NOT NULL,
    instalada TEXT NOT NULL,
    notas     TEXT
);
