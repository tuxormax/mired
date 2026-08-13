package programador

import (
	"context"
	"net/netip"
	"time"

	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/dpi"
)

// RecogerInspeccion le pregunta cada tanto a mired-dpi que aplicaciones vio.
//
// **Es el unico subsistema que puede no estar.** mired-dpi viaja en un paquete
// Debian aparte, asi que lo normal es que no conteste nadie en ese socket. Eso
// no es un error y no se registra como tal: se intenta, se calla y se vuelve a
// intentar mas tarde, por si algun dia lo instalan.
//
// El reparto de papeles es el mismo de siempre: mired-dpi captura y no toca la
// base; el servidor pregunta y escribe.
func (s *Servicio) RecogerInspeccion(ctx context.Context, socket string, cada time.Duration) {
	if socket == "" {
		return
	}
	if cada <= 0 {
		cada = 5 * time.Minute
	}

	mapa := &mapaDeSubredes{datos: s.Datos, bitacora: s.Bitacora}
	if err := mapa.recargar(ctx); err != nil {
		s.Bitacora.Warn("no se pudo cargar el mapa de subredes para la inspeccion", "error", err)
	}

	// La primera vez se avisa si contesta, y solo la primera: repetirlo cada
	// cinco minutos llenaria la bitacora en la instalacion normal, que es la que
	// NO tiene mired-dpi.
	if estado, err := dpi.PedirEstado(socket, 5*time.Second); err == nil {
		s.Bitacora.Info("inspeccion profunda disponible",
			"interfaz", estado.Interfaz, "version", estado.Version)
		if estado.Advertencia != "" {
			s.Bitacora.Warn("aviso de la inspeccion profunda", "aviso", estado.Advertencia)
		}
	}

	reloj := time.NewTicker(cada)
	defer reloj.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reloj.C:
			if err := mapa.recargar(ctx); err != nil {
				s.Bitacora.Warn("no se pudo recargar el mapa de subredes", "error", err)
			}
			s.recogerUnCorte(ctx, socket, mapa)
		}
	}
}

func (s *Servicio) recogerUnCorte(ctx context.Context, socket string, mapa *mapaDeSubredes) {
	corte, err := dpi.PedirCorte(socket, 30*time.Second)
	if err != nil {
		// Lo normal es que mired-dpi no este instalado. No es un fallo.
		s.Bitacora.Debug("la inspeccion profunda no contesto", "error", err)
		return
	}
	if len(corte.Consumos) == 0 {
		return
	}

	// Se agrupan por red antes de escribir: cada red es un archivo distinto, y
	// abrirlo una vez por renglon seria abrirlo cientos de veces.
	porRed := map[string][]basedatos.ConsumoPorAplicacion{}
	for _, consumo := range corte.Consumos {
		direccion, err := netip.ParseAddr(consumo.IP)
		if err != nil {
			continue
		}
		clave := mapa.redDe(direccion)
		if clave == "" {
			continue
		}
		porRed[clave] = append(porRed[clave], basedatos.ConsumoPorAplicacion{
			IP:         consumo.IP,
			Aplicacion: consumo.Aplicacion,
			Como:       consumo.Como,
			Bytes:      int64(consumo.Bytes),
			Paquetes:   int64(consumo.Paquetes),
		})
	}

	for clave, lista := range porRed {
		err := s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
			if err := base.GuardarAplicaciones(ctx, lista); err != nil {
				return err
			}
			// mired-dpi escribe mucho mas que los otros medidores, asi que se
			// poda en cada corte y no de vez en cuando.
			return base.PodarAplicaciones(ctx, 14)
		})
		if err != nil {
			s.Bitacora.Error("no se pudo guardar el consumo por aplicacion",
				"red", clave, "error", err)
			continue
		}
		s.Bitacora.Debug("consumo por aplicacion guardado", "red", clave, "renglones", len(lista))
	}
}
