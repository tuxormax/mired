-- Agenda de escaneos de cada red.
--
-- Vive en el catalogo y no en el archivo de la red a proposito: es como ESTA
-- instalacion reparte su trabajo, no parte de la identidad de la red. Si el
-- archivo de una red se copia a otro servidor, el nuevo decide su propio ritmo.
--
-- Dos ritmos distintos sobre los mismos datos:
--   presencia: barrido corto y frecuente, solo dice quien esta.
--   profundo:  puertos, nombres y servicios; caro, espaciado.
-- Mezclarlos es justo lo que vuelve pesadas a otras herramientas.

ALTER TABLE redes ADD COLUMN programado INTEGER NOT NULL DEFAULT 0 CHECK (programado IN (0, 1));
ALTER TABLE redes ADD COLUMN presencia_cada_segundos INTEGER NOT NULL DEFAULT 60;
ALTER TABLE redes ADD COLUMN profundo_cada_minutos INTEGER NOT NULL DEFAULT 360;
ALTER TABLE redes ADD COLUMN proxima_presencia TEXT;
ALTER TABLE redes ADD COLUMN proximo_profundo TEXT;
