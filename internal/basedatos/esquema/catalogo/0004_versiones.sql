-- Historial de versiones: del sistema y de la base.
--
-- Son las dos tablas estandar de la casa. Aqui llegan sembradas desde
-- `versiones/historial.toml`, que viaja DENTRO del binario: en un `.deb` no hay
-- ningun SQL que alguien vaya a correr a mano, asi que el servidor las llena al
-- arrancar.
--
-- Sirven para lo que no puede contestar el numero de version a secas: **que
-- cambio, cuando y en que modulo**. Cuando alguien reporta un problema, lo
-- primero es saber que trae instalado y desde cuando.

CREATE TABLE versionessistema (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    version   TEXT    NOT NULL,               -- 1.14, donde 14 son los modulos
    revision  INTEGER NOT NULL,               -- sube en cada entrega, nunca se reinicia
    fecha     TEXT    NOT NULL,
    tipo      TEXT    NOT NULL CHECK (tipo IN ('feature', 'fix', 'ajuste', 'change')),
    modulo    TEXT    NOT NULL,
    notas     TEXT    NOT NULL
);
-- Version y revision juntas identifican una entrega: es lo que permite sembrar
-- el historial cada vez que arranca sin duplicar nada.
CREATE UNIQUE INDEX ux_versionessistema ON versionessistema (version, revision);

CREATE TABLE versiondb (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    version   TEXT    NOT NULL,
    revision  INTEGER NOT NULL,
    fecha     TEXT    NOT NULL,
    tipo      TEXT    NOT NULL CHECK (tipo IN ('feature', 'fix', 'ajuste', 'change')),
    modulo    TEXT    NOT NULL,
    notas     TEXT    NOT NULL
);
CREATE UNIQUE INDEX ux_versiondb ON versiondb (version, revision);
