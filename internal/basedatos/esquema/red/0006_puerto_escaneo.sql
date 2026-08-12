-- De que escaneo salio cada puerto la primera vez.
--
-- Sin esto, "¿este puerto es nuevo?" se respondia comparando marcas de tiempo, y
-- las marcas tienen resolucion de SEGUNDOS: dos escaneos seguidos en una red
-- chica caen en el mismo segundo y todos los puertos parecian nuevos. El numero
-- de escaneo no miente nunca.
ALTER TABLE puertos ADD COLUMN escaneo_creado INTEGER;
