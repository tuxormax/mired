-- Donde esta fisicamente el aparato.
--
-- Hasta ahora un aparato se sabia por su nombre, su IP y su MAC —los tres del
-- mundo logico— y de donde COLGABA, que es su sitio en el cableado. Pero no
-- DONDE ESTA: en que consultorio, en que oficina, en que rack.
--
-- Sale de un caso real: la hoja de una clinica con 23 rosetas donde cada nodo se
-- llamaba D01, D02, D03... y la unica forma de saber cual era cual era su
-- columna UBICACION («farmacia», «cons 5», «reloj checador»). Sin esta columna
-- ese dato solo cabia en las notas, revuelto con «se poncho de nuevo», y no se
-- podia agrupar ni filtrar por sitio, que es justo para lo que sirve.
--
-- Va en `equipos` y no en `equipos_snmp`: aquella la escribe el propio aparato
-- cuando contesta SNMP (su sysLocation), y **esta la escribe una persona**. Un
-- switch tonto no contesta nada y tambien esta en algun sitio.
ALTER TABLE equipos ADD COLUMN ubicacion TEXT;

-- Se busca por sitio —«que hay en el consultorio 4»— asi que lleva indice.
CREATE INDEX ix_equipos_ubicacion ON equipos (ubicacion) WHERE ubicacion IS NOT NULL;
