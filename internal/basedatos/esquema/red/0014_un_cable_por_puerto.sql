-- Un puerto lleva UN cable, se mire por donde se mire.
--
-- 0011 dejo esa regla a medias: el indice unico cubria solo el puerto de ORIGEN.
-- Un mismo puerto podia recibir dos cables por el lado del destino, y entonces
-- el mapa dibujaba dos verdades incompatibles colgando del mismo lugar.
--
-- La regla vale mas ahora que la interfaz pregunta a que puerto del otro aparato
-- entra el cable: un switch de cinco puertos conectado al modem tiene cuatro
-- libres, y esa cuenta solo es cierta si el puerto del uplink no puede aparecer
-- ocupado dos veces.

-- Primero se limpia lo que ya estuviera repetido: se conserva el cable mas
-- reciente, que es el que refleja el ultimo movimiento de alguien con el cable
-- en la mano.
DELETE FROM enlaces_fisicos
WHERE puerto_destino_id IS NOT NULL
  AND id NOT IN (
      SELECT MAX(id) FROM enlaces_fisicos
      WHERE puerto_destino_id IS NOT NULL
      GROUP BY puerto_destino_id
  );

CREATE UNIQUE INDEX ux_enlaces_fisicos_destino
    ON enlaces_fisicos (puerto_destino_id)
    WHERE puerto_destino_id IS NOT NULL;
