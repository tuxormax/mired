-- Las credenciales de cada equipo, donde se buscan: en el propio equipo.
--
-- El caso de uso es el de siempre en una red que uno administra: hay que entrar
-- al panel del modem, del switch o de la antena, y la clave esta en un papel, en
-- una nota del telefono o en la cabeza de alguien que hoy no vino. Guardarla
-- junto al aparato en el mapa es tener el inventario y la llave en el mismo
-- lugar, que es justo lo que hace falta.
--
-- **La clave se guarda cifrada** (ver internal/secreto). No es teatro: la carpeta
-- de datos es lo unico que hay que respaldar, y esos respaldos se copian a
-- discos, a la nube y a correos. La llave del cifrado NO vive con los datos,
-- vive con la configuracion, asi que un respaldo robado no trae con que
-- descifrarse. Contra alguien sentado en la maquina con el usuario del servicio
-- esto no protege, y no se va a decir que si.
--
-- Lo que la interfaz muestra sin pedir nada es el USUARIO y la direccion del
-- panel; la clave solo viaja cuando alguien la pide expresamente, y eso queda
-- anotado en la bitacora de actividad. **Nunca sale en un mapa exportado**: un
-- PNG que se manda por correo con las claves de la red adentro seria un
-- desastre, y por eso la exportacion no las lee.
CREATE TABLE credenciales_equipo (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    equipo_id  INTEGER NOT NULL REFERENCES equipos (id) ON DELETE CASCADE,
    -- Para que sirve: el panel web, la consola SSH, la app del fabricante.
    tipo       TEXT    NOT NULL DEFAULT 'web'
               CHECK (tipo IN ('web', 'ssh', 'consola', 'app', 'otro')),
    usuario    TEXT,
    -- Cifrada. Nunca se guarda en claro, ni siquiera "mientras tanto".
    clave      TEXT,
    -- Donde se entra: https://192.168.1.1:8443, ssh://..., lo que sea.
    direccion  TEXT,
    notas      TEXT,
    creada     TEXT    NOT NULL,
    modificada TEXT
);
-- Un equipo puede tener varias: la del panel y la de la consola no son la misma.
CREATE UNIQUE INDEX ux_credenciales_equipo ON credenciales_equipo (equipo_id, tipo);
CREATE INDEX ix_credenciales_equipo ON credenciales_equipo (equipo_id);
