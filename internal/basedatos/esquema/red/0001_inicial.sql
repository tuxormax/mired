-- Esquema inicial de UNA RED: el archivo redes/<clave>.db.
--
-- Todo lo operativo de un sitio vive aqui y nada se comparte con las demas
-- redes. Respaldar el sitio es copiar este archivo; borrarlo es borrar la red.
--
-- El archivo es AUTODESCRIPTIVO: la tabla ficha lleva dentro quien es esta red,
-- para que al copiarlo a otra instalacion se sepa que es sin consultar el
-- catalogo.

CREATE TABLE ficha (
    id          INTEGER PRIMARY KEY CHECK (id = 1),  -- fila unica, a proposito
    clave       TEXT NOT NULL,
    nombre      TEXT NOT NULL,
    descripcion TEXT,
    creada      TEXT NOT NULL,
    modificada  TEXT,
    -- Perfil de capacidades: que se puede llegar a saber en esta red. Lo
    -- descubre el escaneo y lo lee la interfaz para decir sin rodeos por que
    -- una pantalla esta vacia, en vez de mostrarla vacia y ya.
    capacidad_mapa_puertos TEXT NOT NULL DEFAULT 'desconocida'
        CHECK (capacidad_mapa_puertos IN ('desconocida', 'exacta', 'por_grupo', 'no_disponible')),
    capacidad_trafico      TEXT NOT NULL DEFAULT 'desconocida'
        CHECK (capacidad_trafico IN ('desconocida', 'por_puerto', 'por_flujos', 'no_disponible')),
    capacidades_al TEXT
);

-- Que se escanea en esta red. Una red puede tener varias subredes (192.168.1.0/24
-- y 10.0.0.0/24 en el mismo sitio).
CREATE TABLE subredes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    cidr        TEXT    NOT NULL,                -- 192.168.1.0/24
    nombre      TEXT,
    descripcion TEXT,
    vlan        INTEGER,
    escanear    INTEGER NOT NULL DEFAULT 1 CHECK (escanear IN (0, 1)),
    estatus     INTEGER NOT NULL DEFAULT 1 CHECK (estatus IN (-1, 0, 1)),
    creada      TEXT    NOT NULL,
    modificada  TEXT
);
CREATE UNIQUE INDEX ux_subredes_cidr ON subredes (cidr) WHERE estatus >= 0;

-- Cada corrida del escaneo, para poder comparar contra la anterior y saber que
-- cambio. Las alertas de la fase 7 salen justamente de esa comparacion.
CREATE TABLE escaneos (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    tipo       TEXT    NOT NULL CHECK (tipo IN ('presencia', 'profundo', 'snmp', 'trafico')),
    iniciado   TEXT    NOT NULL,
    terminado  TEXT,
    estado     TEXT    NOT NULL DEFAULT 'en_curso'
        CHECK (estado IN ('en_curso', 'terminado', 'fallido', 'cancelado')),
    equipos_vistos INTEGER NOT NULL DEFAULT 0,
    equipos_nuevos INTEGER NOT NULL DEFAULT 0,
    detalle    TEXT
);
CREATE INDEX ix_escaneos_iniciado ON escaneos (iniciado);
