// Paquete escaneo descubre que equipos hay en una subred.
//
// Corre dentro de mired-sonda, que es el proceso con permisos de red cruda. NO
// toca la base de datos: devuelve lo que vio y el servidor decide que guardar.
//
// El descubrimiento va en capas, de la mas confiable a la menos:
//
//  1. ARP, si la subred esta conectada a una tarjeta de este equipo. Es el mejor
//     dato posible: da la MAC, no se puede filtrar con un firewall y no depende
//     de que el aparato quiera contestar.
//  2. ICMP, para lo que no esta en la misma subred. Da presencia pero no MAC, y
//     hay equipos que no contestan ping a proposito.
//  3. TCP, tocando puertos comunes. Encuentra lo que ignora el ping, pero es el
//     dato mas debil: solo dice que algo contesto en esa direccion.
//
// Cada equipo guarda con que metodo se vio, porque no es lo mismo "esta" que
// "algo contesto ahi".
package escaneo

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tuxormax/mired/internal/sonda"
)

// Valores por omision, pensados para no saturar una red chica ni una Raspberry.
const (
	esperaBarridoPorOmision = 3 * time.Second
	esperaPuertoPorOmision  = 700 * time.Millisecond
	// topeEquipos evita que un /8 mal capturado intente barrer 16 millones de
	// direcciones y deje el equipo inservible.
	topeEquipos = 65536
	// paralelismoPuertos es cuantas conexiones TCP se prueban a la vez en total.
	paralelismoPuertos = 128
)

// Barrer descubre los equipos de las subredes pedidas.
func Barrer(ctx context.Context, peticion sonda.PeticionEscaneo) (sonda.ResultadoEscaneo, error) {
	inicio := time.Now()
	resultado := sonda.ResultadoEscaneo{Equipos: []sonda.EquipoVisto{}}

	esperaBarrido := esperaBarridoPorOmision
	if peticion.EsperaBarridoMs > 0 {
		esperaBarrido = time.Duration(peticion.EsperaBarridoMs) * time.Millisecond
	}
	esperaPuerto := esperaPuertoPorOmision
	if peticion.EsperaPuertoMs > 0 {
		esperaPuerto = time.Duration(peticion.EsperaPuertoMs) * time.Millisecond
	}
	puertos := peticion.Puertos
	if len(puertos) == 0 {
		puertos = PuertosComunes()
	}

	// Se junta todo por IP: una misma direccion puede aparecer por ARP y por
	// ICMP, y debe salir una sola vez con el metodo mas confiable.
	vistos := map[string]*sonda.EquipoVisto{}

	for _, texto := range peticion.Subredes {
		prefijo, err := netip.ParsePrefix(strings.TrimSpace(texto))
		if err != nil {
			resultado.Advertencias = append(resultado.Advertencias,
				fmt.Sprintf("subred invalida, se omitio: %s", texto))
			continue
		}
		prefijo = prefijo.Masked()

		direcciones, err := direccionesDe(prefijo)
		if err != nil {
			resultado.Advertencias = append(resultado.Advertencias, err.Error())
			continue
		}

		tarjeta := interfazDe(prefijo)
		if tarjeta != nil && prefijo.Addr().Is4() {
			// Subred local: ARP es el mejor dato y el mas barato.
			encontrados, err := BarrerARP(ctx, tarjeta, direcciones, esperaBarrido)
			if err != nil {
				resultado.Advertencias = append(resultado.Advertencias,
					fmt.Sprintf("no se pudo barrer por ARP en %s: %v", prefijo, err))
			}
			for ip, mac := range encontrados {
				anotar(vistos, ip, prefijo.String(), "arp", mac)
			}
		}

		// ICMP cubre lo que no es local y, en la subred local, lo que tiene la
		// MAC en cache pero no contesto el ARP de este barrido.
		hayICMP := true
		encontrados, err := BarrerICMP(ctx, direcciones, esperaBarrido)
		if err != nil {
			hayICMP = false
			resultado.Advertencias = append(resultado.Advertencias,
				fmt.Sprintf("no se pudo barrer por ICMP en %s: %v", prefijo, err))
		}
		for _, ip := range encontrados {
			anotar(vistos, ip, prefijo.String(), "icmp", nil)
		}

		// Tercera capa: si las dos anteriores no pudieron trabajar y esta subred
		// quedo sin nada, se toca TCP. Es mas lento y menos confiable, pero es
		// lo unico que funciona sin permisos de red cruda, y una instalacion sin
		// esos permisos debe servir de inventario igual.
		if !hayICMP && sinEquiposDe(vistos, prefijo.String()) {
			resultado.Advertencias = append(resultado.Advertencias,
				fmt.Sprintf("en %s se descubrio tocando puertos TCP, que es menos confiable: "+
					"no da la MAC del equipo. Conceda CAP_NET_RAW a la sonda para usar ARP e ICMP.", prefijo))
			for _, ip := range BarrerTCP(ctx, direcciones, esperaPuerto) {
				anotar(vistos, ip, prefijo.String(), "tcp", nil)
			}
		}
	}

	if len(vistos) == 0 {
		resultado.DuracionMs = time.Since(inicio).Milliseconds()
		return resultado, nil
	}

	// Un barrido de presencia termina aqui: solo dice quien esta. Es el ritmo
	// rapido, separado del escaneo profundo, y es lo que lo hace liviano.
	if !peticion.SoloPresencia {
		revisarPuertos(ctx, vistos, puertos, esperaPuerto)
		resolverNombres(ctx, vistos)
	}

	for _, equipo := range vistos {
		if equipo.MAC != "" {
			equipo.Fabricante = Fabricante(equipo.MAC)
		}
		resultado.Equipos = append(resultado.Equipos, *equipo)
	}
	sort.Slice(resultado.Equipos, func(i, j int) bool {
		izquierda, _ := netip.ParseAddr(resultado.Equipos[i].IP)
		derecha, _ := netip.ParseAddr(resultado.Equipos[j].IP)
		return izquierda.Less(derecha)
	})

	resultado.DuracionMs = time.Since(inicio).Milliseconds()
	return resultado, nil
}

// anotar junta lo que se ve de una misma IP por varios caminos. ARP manda sobre
// ICMP y ICMP sobre TCP: se conserva siempre el dato mas confiable.
func anotar(vistos map[string]*sonda.EquipoVisto, ip netip.Addr, subred, metodo string, mac net.HardwareAddr) {
	clave := ip.String()
	equipo, hay := vistos[clave]
	if !hay {
		equipo = &sonda.EquipoVisto{IP: clave, Subred: subred, Metodo: metodo}
		vistos[clave] = equipo
	}
	if mac != nil && equipo.MAC == "" {
		equipo.MAC = strings.ToLower(mac.String())
	}
	if confianza(metodo) > confianza(equipo.Metodo) {
		equipo.Metodo = metodo
	}
}

// sinEquiposDe dice si esta subred no aporto ningun equipo todavia.
func sinEquiposDe(vistos map[string]*sonda.EquipoVisto, subred string) bool {
	for _, equipo := range vistos {
		if equipo.Subred == subred {
			return false
		}
	}
	return true
}

func confianza(metodo string) int {
	switch metodo {
	case "arp":
		return 3
	case "icmp":
		return 2
	case "tcp":
		return 1
	default:
		return 0
	}
}

// revisarPuertos prueba los puertos comunes en cada equipo encontrado, con un
// tope global de conexiones para no ahogar la red ni el equipo que escanea.
func revisarPuertos(ctx context.Context, vistos map[string]*sonda.EquipoVisto, puertos []int, espera time.Duration) {
	permisos := make(chan struct{}, paralelismoPuertos)
	var candado sync.Mutex
	var espera2 sync.WaitGroup

	for _, equipo := range vistos {
		for _, puerto := range puertos {
			espera2.Add(1)
			go func(destino *sonda.EquipoVisto, numero int) {
				defer espera2.Done()
				permisos <- struct{}{}
				defer func() { <-permisos }()

				if ctx.Err() != nil {
					return
				}
				abierto, banner := probarTCP(ctx, destino.IP, numero, espera)
				if !abierto {
					return
				}
				candado.Lock()
				destino.Puertos = append(destino.Puertos, sonda.PuertoVisto{
					Numero:    numero,
					Protocolo: "tcp",
					Servicio:  NombreServicio(numero),
					Banner:    banner,
				})
				candado.Unlock()
			}(equipo, puerto)
		}
	}
	espera2.Wait()

	for _, equipo := range vistos {
		sort.Slice(equipo.Puertos, func(i, j int) bool {
			return equipo.Puertos[i].Numero < equipo.Puertos[j].Numero
		})
	}
}

// resolverNombres busca el nombre de cada equipo por DNS inverso. Es barato y
// resuelve la mayoria de los equipos de una red con servidor propio.
func resolverNombres(ctx context.Context, vistos map[string]*sonda.EquipoVisto) {
	var espera sync.WaitGroup
	permisos := make(chan struct{}, 32)
	resolutor := net.Resolver{}

	for _, equipo := range vistos {
		espera.Add(1)
		go func(destino *sonda.EquipoVisto) {
			defer espera.Done()
			permisos <- struct{}{}
			defer func() { <-permisos }()

			consulta, cancelar := context.WithTimeout(ctx, 2*time.Second)
			defer cancelar()

			nombres, err := resolutor.LookupAddr(consulta, destino.IP)
			if err != nil || len(nombres) == 0 {
				return
			}
			destino.Nombre = strings.TrimSuffix(nombres[0], ".")
		}(equipo)
	}
	espera.Wait()
}

// direccionesDe enumera las direcciones utilizables de una subred, sin la de red
// ni la de difusion.
func direccionesDe(prefijo netip.Prefix) ([]netip.Addr, error) {
	if !prefijo.Addr().Is4() {
		return nil, fmt.Errorf("por ahora solo se barren subredes IPv4: %s", prefijo)
	}

	bits := prefijo.Bits()
	cuantas := 1 << uint(32-bits)
	if cuantas > topeEquipos {
		return nil, fmt.Errorf("la subred %s es demasiado grande para barrerla (%d direcciones); "+
			"conviene partirla en subredes mas chicas", prefijo, cuantas)
	}

	var direcciones []netip.Addr
	actual := prefijo.Addr()
	for i := 0; i < cuantas; i++ {
		// En una subred normal, la primera es la de red y la ultima la de
		// difusion: ninguna es un equipo. En /31 y /32 no aplica.
		esUtil := bits >= 31 || (i != 0 && i != cuantas-1)
		if esUtil {
			direcciones = append(direcciones, actual)
		}
		actual = actual.Next()
	}
	return direcciones, nil
}

// interfazDe devuelve la tarjeta de red conectada a esa subred, o nil si la
// subred no es local.
func interfazDe(prefijo netip.Prefix) *net.Interface {
	tarjetas, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for i := range tarjetas {
		if tarjetas[i].Flags&net.FlagUp == 0 || tarjetas[i].Flags&net.FlagLoopback != 0 {
			continue
		}
		direcciones, err := tarjetas[i].Addrs()
		if err != nil {
			continue
		}
		for _, direccion := range direcciones {
			red, ok := direccion.(*net.IPNet)
			if !ok {
				continue
			}
			propia, err := netip.ParsePrefix(red.String())
			if err != nil {
				continue
			}
			// Basta con que la subred pedida quede DENTRO de la de la tarjeta:
			// escanear un rango chico de la propia red (por ejemplo un /28
			// dentro del /24) sigue siendo local, y ahi ARP funciona.
			if propia.Contains(prefijo.Addr()) && prefijo.Bits() >= propia.Bits() {
				return &tarjetas[i]
			}
		}
	}
	return nil
}
