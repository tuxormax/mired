package programador

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/sonda"
)

// adoptarLaRedDetectada le pone a un sitio sin configurar la red de este equipo.
//
// # Por que existe
//
// Un sitio recien creado y sin ningun rango no puede escanear nada: el barrido
// corre, no mira ni una direccion y no encuentra nada. Antes eso terminaba en un
// error —"la red no tiene subredes"— que le pedia al usuario que fuera a una
// pestaña llamada "Subredes" a escribir algo en una notacion que no tenia por
// que conocer. **El programa ya sabe en que red esta**, porque la sonda le
// reporta las tarjetas del equipo: preguntarlo era trabajo regalado.
//
// # Lo que NO hace
//
// **No se calla.** Adopta la red, la guarda y devuelve cuales adopto, para que
// la pantalla lo diga con todas sus letras. Escanear una red que el usuario no
// pidio y no decirlo seria peor que el error que vino a sustituir: en un sitio
// remoto —una sucursal creada desde la matriz— la red de ESTE equipo no es la
// del sitio, y quien lo vea tiene que poder corregirlo.
//
// Solo actua cuando el sitio no tiene NADA. En cuanto hay un rango configurado,
// esto no se vuelve a meter.
func (s *Servicio) adoptarLaRedDetectada(ctx context.Context, clave string) ([]string, error) {
	detectadas, err := s.redesDeEsteEquipo()
	if err != nil {
		return nil, err
	}
	// Sin ninguna tarjeta con direccion, este equipo no esta en ninguna red: no
	// hay nada que escanear y no es un problema de configuracion de MiRed.
	if len(detectadas) == 0 {
		return nil, ErrEquipoSinRed
	}

	var adoptadas []string
	err = s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
		for _, cidr := range detectadas {
			if _, err := base.CrearSubred(ctx, cidr, "Detectada automaticamente", "", nil); err != nil {
				// Que una no se pueda guardar no debe tirar las demas: con una
				// sola que entre, el escaneo ya sirve.
				s.Bitacora.Warn("no se pudo adoptar una red detectada",
					"red", clave, "cidr", cidr, "error", err)
				continue
			}
			adoptadas = append(adoptadas, cidr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(adoptadas) == 0 {
		return nil, fmt.Errorf("no se pudo guardar ninguna de las redes detectadas")
	}

	s.Bitacora.Info("el sitio no tenia nada que escanear: se adopto la red de este equipo",
		"red", clave, "adoptadas", adoptadas)
	return adoptadas, nil
}

// redesDeEsteEquipo pregunta a la sonda que tarjetas ve y devuelve sus redes.
func (s *Servicio) redesDeEsteEquipo() ([]string, error) {
	estado, err := sonda.PedirEstado(s.SocketSonda, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("no se pudo preguntar a la sonda que redes ve: %w", err)
	}

	vistas := map[string]bool{}
	var redes []string
	for _, interfaz := range estado.Interfaces {
		if !interfaz.Activa {
			continue
		}
		for _, direccion := range interfaz.Direcciones {
			cidr, sirve := redDe(direccion)
			if !sirve || vistas[cidr] {
				continue
			}
			vistas[cidr] = true
			redes = append(redes, cidr)
		}
	}
	return redes, nil
}

// redDe convierte la direccion de una tarjeta en la red a la que pertenece.
//
// Descarta lo que no tiene sentido barrer: la interfaz local, IPv6 —el barrido
// es IPv4 por ahora— y las redes tan grandes que tardarian horas. Una /16 son ya
// 65 000 direcciones; mas que eso no es "mi red".
func redDe(direccionConMascara string) (string, bool) {
	prefijo, err := netip.ParsePrefix(direccionConMascara)
	if err != nil {
		return "", false
	}
	if !prefijo.Addr().Is4() || prefijo.Addr().IsLoopback() {
		return "", false
	}
	if prefijo.Bits() < 16 || prefijo.Bits() > 30 {
		return "", false
	}
	return prefijo.Masked().String(), true
}
