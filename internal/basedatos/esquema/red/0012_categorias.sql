-- De que TIPO es cada equipo, en un vocabulario cerrado.
--
-- `tipo` ya existia, pero guarda el NOMBRE que le puso el catalogo —"Impresora
-- HP", "Camara Hikvision"—, que sirve para leerlo y no para contar: agrupando
-- por ahi salen cubos separados para "Impresora HP" y "Impresora de red".
--
-- `categoria` es la clave de la lista unica de MiRed (internal/catalogo/
-- categorias.go). Es lo que permite responder "cuantos switches hay", y lo que
-- hace que un aparato descubierto y uno declarado a mano cuenten juntos: los dos
-- escriben aqui el mismo vocabulario.

ALTER TABLE equipos ADD COLUMN categoria TEXT;
CREATE INDEX ix_equipos_categoria ON equipos (categoria) WHERE estatus = 1;

-- Lo que ya estaba declarado a mano se traduce al vocabulario nuevo.
--
-- El formulario viejo guardaba en `tipo` sus propias palabras —'ap', 'pc',
-- 'nvr'—, que NO son las del catalogo. Esa es justamente la divergencia que la
-- lista unica viene a cerrar; aqui se salda la que ya se habia creado, en vez de
-- dejar esos equipos contando aparte para siempre.
UPDATE equipos SET categoria = CASE LOWER(COALESCE(tipo, ''))
    WHEN 'switch' THEN 'switch_simple'
    WHEN 'router' THEN 'router'
    WHEN 'ap'     THEN 'punto_de_acceso'
    WHEN 'nvr'    THEN 'camara'
    WHEN 'pc'     THEN 'computadora'
    ELSE 'otro'
END
WHERE origen = 'manual' AND categoria IS NULL;

-- Lo descubierto se queda sin categoria a proposito: la pone el catalogo en el
-- proximo reconocimiento, que corre en cada escaneo. Adivinarla aqui desde el
-- nombre seria inventar un dato que en un minuto se puede saber de verdad.
