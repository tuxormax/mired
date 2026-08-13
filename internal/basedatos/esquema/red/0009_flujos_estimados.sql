-- sFlow no cuenta el trafico: lo muestrea.
--
-- El switch toma uno de cada N paquetes y quien recibe multiplica por N. La
-- cifra que sale de ahi sirve de sobra para saber quien se esta llevando el
-- ancho de banda, y NO sirve para facturarle a nadie.
--
-- Esa diferencia se guarda con el dato, no se deduce despues: una vez que las
-- dos clases de medicion estan en la misma tabla, sin esta columna ya no hay
-- forma de volver a separarlas.

ALTER TABLE trafico_flujos ADD COLUMN estimado INTEGER NOT NULL DEFAULT 0;
