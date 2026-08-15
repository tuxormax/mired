-- Lo que cada aparato dice de SI MISMO.
--
-- Hasta aqui, de un equipo se sabia donde esta (IP y MAC), quien lo fabrico (por
-- el prefijo de la MAC) y que puertos tiene abiertos. Con eso no se puede
-- distinguir un modem de una television: los dos contestan en el 80.
--
-- Esta tabla guarda lo que el propio aparato publica cuando se le pregunta bien:
-- el titulo de su pagina, el nombre de su certificado, lo que anuncia por
-- mDNS/Bonjour o por UPnP, y lo que contesta al protocolo de su fabricante.
--
-- **La fuente se guarda con el dato** y no se mezcla en un solo campo "modelo".
-- No vale lo mismo un modelo firmado en un certificado que uno sacado del titulo
-- de una pagina que cualquiera pudo cambiar, y la interfaz tiene que poder decir
-- de donde salio cada cosa en vez de presentarlo todo como igual de firme.
CREATE TABLE huellas (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    equipo_id  INTEGER NOT NULL REFERENCES equipos (id) ON DELETE CASCADE,
    -- web, certificado, ssh, mdns, ssdp, netbios, onvif, ubiquiti, mikrotik,
    -- tplink, hikvision, dahua, roku, impresora, television.
    fuente     TEXT    NOT NULL,
    -- nombre, modelo, fabricante, firmware, serie, titulo, servidor, realm,
    -- mac, servicio, red.
    clave      TEXT    NOT NULL,
    valor      TEXT    NOT NULL,
    -- Cuando se supo por ultima vez. Un aparato al que le cambian el firmware
    -- contesta otra cosa, y hay que poder ver que este dato es de ayer.
    ultima_vez TEXT    NOT NULL
);

-- El mismo dato de la misma fuente no se repite: se actualiza la fecha.
CREATE UNIQUE INDEX ux_huellas ON huellas (equipo_id, fuente, clave, valor);
CREATE INDEX ix_huellas_equipo ON huellas (equipo_id);
