package escaneo

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/mdlayher/arp"
)

// BarrerARP pregunta por cada direccion de la subred y junta las respuestas.
//
// Se hace en dos partes a proposito: un lector que recoge respuestas mientras un
// emisor sigue preguntando. La alternativa —preguntar y esperar respuesta una
// por una— tarda el tiempo de espera por cada equipo que no contesta, que en un
// /24 con la mitad vacia son minutos en vez de segundos.
func BarrerARP(ctx context.Context, tarjeta *net.Interface, direcciones []netip.Addr, espera time.Duration) (map[netip.Addr]net.HardwareAddr, error) {
	encontrados := map[netip.Addr]net.HardwareAddr{}

	cliente, err := arp.Dial(tarjeta)
	if err != nil {
		// El caso normal es no tener CAP_NET_RAW. Se dice claro, porque el
		// sintoma —"no encontro nada"— no ayuda a nadie.
		return encontrados, fmt.Errorf("no se pudo abrir ARP en %s (¿faltan permisos de red cruda?): %w",
			tarjeta.Name, err)
	}
	defer cliente.Close()

	buscadas := map[netip.Addr]bool{}
	for _, direccion := range direcciones {
		buscadas[direccion] = true
	}

	limite := time.Now().Add(espera)
	listo := make(chan map[netip.Addr]net.HardwareAddr, 1)

	go func() {
		respuestas := map[netip.Addr]net.HardwareAddr{}
		for {
			if err := cliente.SetReadDeadline(limite); err != nil {
				break
			}
			paquete, _, err := cliente.Read()
			if err != nil {
				// Al vencer el plazo se sale: es la forma normal de terminar.
				break
			}
			if paquete.Operation != arp.OperationReply {
				continue
			}
			if buscadas[paquete.SenderIP] {
				respuestas[paquete.SenderIP] = paquete.SenderHardwareAddr
			}
		}
		listo <- respuestas
	}()

	// Se pregunta con una pausa minima entre paquetes: sin ella, una subred
	// grande manda miles de tramas de golpe y switches modestos las tiran.
	for _, direccion := range direcciones {
		if ctx.Err() != nil {
			break
		}
		if err := cliente.Request(direccion); err != nil && !errors.Is(err, net.ErrClosed) {
			continue
		}
		time.Sleep(time.Millisecond)
	}

	select {
	case respuestas := <-listo:
		return respuestas, nil
	case <-ctx.Done():
		return encontrados, ctx.Err()
	case <-time.After(time.Until(limite) + time.Second):
		return encontrados, nil
	}
}
