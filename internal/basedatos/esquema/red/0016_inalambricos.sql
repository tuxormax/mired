-- Quien cuelga de que antena, por el aire.
--
-- Hasta aqui el mapa solo sabia de cables: un equipo colgaba de un puerto
-- fisico, y punto. Los telefonos, la television y todo lo que entra por WiFi
-- quedaban flotando abajo, en la zona de "no cuelgan de ningun sitio conocido",
-- aunque cualquiera que mire la casa sepa perfectamente de que antena cuelgan.
--
-- Va en tabla APARTE de enlaces_fisicos a proposito. Un enlace inalambrico no
-- tiene puerto: meterlo ahi obligaria a inventarle a cada antena un puerto por
-- cada telefono que se conecte, y esos puertos no existen. Lo que sí comparte es
-- la idea que sostiene todo el modulo: **el dato dice de donde salio**.
--
--   manual        lo declaro una persona que sabe de que antena cuelga
--   snmp          lo dijo la propia antena, en su tabla de asociados
--   panel         se leyo del panel del aparato, entrando con su credencial
--   controladora  lo dijo la controladora WiFi (UniFi)
--
-- **Aqui solo entra lo INALAMBRICO, y hay una razon de fondo.** El panel de un
-- modem lista tambien lo que tiene por cable, pero eso NO sirve para colgar
-- nada: el modem ve en su LAN todo lo que hay detras del switch, asi que
-- colgarlo de el dibujaria una red plana donde en realidad hay un arbol. Lo
-- inalambrico es distinto: un cliente asociado a un radio cuelga de ESE radio,
-- sin intermediarios posibles.
--
-- Un cliente esta asociado a UNA antena a la vez: eso no es una regla de MiRed,
-- es como funciona el WiFi. Por eso el indice unico es por equipo: reconectar
-- reemplaza, en vez de dejar el mismo telefono colgando de dos antenas.
CREATE TABLE enlaces_inalambricos (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    -- El cliente: el telefono, la television, la laptop.
    equipo_id   INTEGER NOT NULL REFERENCES equipos (id) ON DELETE CASCADE,
    -- La antena de la que cuelga.
    antena_id   INTEGER NOT NULL REFERENCES equipos (id) ON DELETE CASCADE,
    -- El SSID, cuando se sabe. Vacio si nadie lo dijo: inventarlo seria peor.
    red         TEXT,
    -- La senal en dBm, si se midio. NULL cuando se declaro a mano: una persona
    -- sabe de que antena cuelga su telefono, no con cuantos dBm llega.
    senal_dbm   INTEGER,
    origen_dato TEXT NOT NULL CHECK (
                    origen_dato IN ('manual', 'snmp', 'panel', 'controladora')
                ),
    notas       TEXT,
    ultima_vez  TEXT NOT NULL,
    creado_en   TEXT NOT NULL,
    -- Una antena no se cuelga de si misma.
    CHECK (equipo_id <> antena_id)
);
CREATE UNIQUE INDEX ux_enlaces_inalambricos ON enlaces_inalambricos (equipo_id);
CREATE INDEX ix_enlaces_inalambricos_antena ON enlaces_inalambricos (antena_id);
