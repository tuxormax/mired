package escaneo

import (
	"context"
	"net/netip"
	"sync"
	"time"
)

// puertosDeSondeo son los que se tocan para averiguar si hay algo en una
// direccion cuando no se puede preguntar por ARP ni por ICMP.
//
// Son pocos y muy comunes a proposito: la idea no es saber que servicios tiene
// el aparato —eso viene despues— sino solo si contesta alguien. Cada puerto de
// mas multiplica el tiempo por el numero de direcciones de la subred.
var puertosDeSondeo = []int{80, 443, 22, 445, 139, 3389, 8080, 62078}

// BarrerTCP descubre equipos tocando puertos comunes.
//
// Es la tercera capa del descubrimiento y la menos confiable: no da la MAC y un
// firewall en medio puede contestar por un equipo que no existe. Pero es la
// unica que funciona sin permisos de red cruda, y ahi esta su valor: una
// instalacion a la que le faltan capacidades sigue sirviendo de inventario en
// vez de mostrar una pantalla vacia.
func BarrerTCP(ctx context.Context, direcciones []netip.Addr, espera time.Duration) []netip.Addr {
	if espera <= 0 {
		espera = esperaPuertoPorOmision
	}

	permisos := make(chan struct{}, paralelismoPuertos)
	var candado sync.Mutex
	var grupo sync.WaitGroup
	contestaron := map[netip.Addr]bool{}

	for _, direccion := range direcciones {
		for _, puerto := range puertosDeSondeo {
			grupo.Add(1)
			go func(destino netip.Addr, numero int) {
				defer grupo.Done()
				permisos <- struct{}{}
				defer func() { <-permisos }()

				if ctx.Err() != nil {
					return
				}
				// Si ya se sabe que este contesta, no hace falta seguir
				// tocandole puertos: con uno basta para decir que esta.
				candado.Lock()
				yaEsta := contestaron[destino]
				candado.Unlock()
				if yaEsta {
					return
				}

				abierto, _ := probarTCP(ctx, destino.String(), numero, espera)
				if !abierto {
					return
				}
				candado.Lock()
				contestaron[destino] = true
				candado.Unlock()
			}(direccion, puerto)
		}
	}
	grupo.Wait()

	respuesta := make([]netip.Addr, 0, len(contestaron))
	for direccion := range contestaron {
		respuesta = append(respuesta, direccion)
	}
	return respuesta
}
