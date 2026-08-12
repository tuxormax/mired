package programador

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/flujos"
)

// RecibirFlujos escucha los flujos que exporta el router y los reparte entre las
// redes segun la direccion de cada equipo.
//
// Se escucha UN solo puerto para todas las redes porque el router no sabe nada
// de MiRed: manda lo que ve, y a que red pertenece cada direccion lo resuelve
// aqui el mapa de subredes.
func (s *Servicio) RecibirFlujos(ctx context.Context, direccion string) error {
	mapa := &mapaDeSubredes{datos: s.Datos, bitacora: s.Bitacora}
	if err := mapa.recargar(ctx); err != nil {
		s.Bitacora.Warn("no se pudo cargar el mapa de subredes para los flujos", "error", err)
	}
	s.mu.Lock()
	s.mapaFlujos = mapa
	s.mu.Unlock()

	receptor := &flujos.Receptor{
		Periodo: time.Minute,
		EsLocal: func(direccion netip.Addr) bool { return mapa.redDe(direccion) != "" },
		AlCortar: func(consumos []flujos.Consumo) {
			s.guardarFlujos(ctx, mapa, consumos)
		},
	}

	// El mapa se recarga cada tanto: si se agrega una red o una subred, los
	// flujos de esas direcciones deben empezar a contarse sin reiniciar nada.
	go func() {
		reloj := time.NewTicker(5 * time.Minute)
		defer reloj.Stop()
		for {
			select {
			case <-ctx.Done():
				receptor.Cerrar()
				return
			case <-reloj.C:
				if err := mapa.recargar(ctx); err != nil {
					s.Bitacora.Warn("no se pudo recargar el mapa de subredes", "error", err)
				}
			}
		}
	}()

	s.Bitacora.Info("escuchando flujos del router", "direccion", direccion)
	return receptor.Escuchar(direccion)
}

// RecargarSubredes vuelve a leer que rangos pertenecen a que red.
//
// La llama la API al crear una red o una subred: sin esto, los flujos de esas
// direcciones se tirarian hasta la siguiente recarga automatica, y quien acaba
// de configurar su router pensaria que no funciona.
func (s *Servicio) RecargarSubredes(ctx context.Context) {
	s.mu.Lock()
	mapa := s.mapaFlujos
	s.mu.Unlock()

	if mapa == nil {
		return
	}
	if err := mapa.recargar(ctx); err != nil {
		s.Bitacora.Warn("no se pudo recargar el mapa de subredes", "error", err)
	}
}

func (s *Servicio) guardarFlujos(ctx context.Context, mapa *mapaDeSubredes, consumos []flujos.Consumo) {
	// Se agrupan por red antes de escribir: cada red es un archivo distinto, y
	// abrirlo una vez por direccion seria abrirlo cientos de veces por minuto.
	porRed := map[string][]basedatos.ConsumoPorFlujo{}
	for _, consumo := range consumos {
		direccion, err := netip.ParseAddr(consumo.IP)
		if err != nil {
			continue
		}
		clave := mapa.redDe(direccion)
		if clave == "" {
			continue
		}
		porRed[clave] = append(porRed[clave], basedatos.ConsumoPorFlujo{
			IP:             consumo.IP,
			BytesSube:      consumo.BytesSube,
			BytesBaja:      consumo.BytesBaja,
			Conversaciones: consumo.Conversaciones,
		})
	}

	for clave, lista := range porRed {
		err := s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
			if err := base.GuardarFlujos(ctx, lista); err != nil {
				return err
			}
			return base.PodarFlujos(ctx, 30)
		})
		if err != nil {
			s.Bitacora.Error("no se pudieron guardar los flujos", "red", clave, "error", err)
			continue
		}
		s.Bitacora.Debug("flujos guardados", "red", clave, "direcciones", len(lista))
	}
}

// mapaDeSubredes resuelve a que red pertenece una direccion.
type mapaDeSubredes struct {
	datos    *basedatos.Enrutador
	bitacora interface {
		Warn(mensaje string, args ...any)
	}

	mu     sync.RWMutex
	rangos []rangoDeRed
}

type rangoDeRed struct {
	prefijo netip.Prefix
	clave   string
}

func (m *mapaDeSubredes) recargar(ctx context.Context) error {
	porRed, err := m.datos.SubredesDeTodasLasRedes(ctx)
	if err != nil {
		return fmt.Errorf("no se pudieron leer las subredes: %w", err)
	}

	var rangos []rangoDeRed
	for clave, subredes := range porRed {
		for _, cidr := range subredes {
			prefijo, err := netip.ParsePrefix(cidr)
			if err != nil {
				continue
			}
			rangos = append(rangos, rangoDeRed{prefijo: prefijo.Masked(), clave: clave})
		}
	}

	m.mu.Lock()
	m.rangos = rangos
	m.mu.Unlock()
	return nil
}

// redDe dice de que red es una direccion, o vacio si no es de ninguna.
func (m *mapaDeSubredes) redDe(direccion netip.Addr) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, rango := range m.rangos {
		if rango.prefijo.Contains(direccion) {
			return rango.clave
		}
	}
	return ""
}
