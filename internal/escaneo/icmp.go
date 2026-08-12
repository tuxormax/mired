package escaneo

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// BarrerICMP hace ping a todas las direcciones y devuelve las que contestaron.
//
// Se abre un solo socket para toda la subred, no uno por direccion: con 254
// direcciones eso serian 254 sockets, y con un /16 el equipo se queda sin
// descriptores de archivo.
func BarrerICMP(ctx context.Context, direcciones []netip.Addr, espera time.Duration) ([]netip.Addr, error) {
	conexion, err := abrirICMP()
	if err != nil {
		return nil, err
	}
	defer conexion.Close()

	buscadas := map[string]netip.Addr{}
	for _, direccion := range direcciones {
		if direccion.Is4() {
			buscadas[direccion.String()] = direccion
		}
	}
	if len(buscadas) == 0 {
		return nil, nil
	}

	limite := time.Now().Add(espera)
	var candado sync.Mutex
	contestaron := map[netip.Addr]bool{}
	listo := make(chan struct{})

	go func() {
		defer close(listo)
		buzon := make([]byte, 1500)
		for {
			if err := conexion.SetReadDeadline(limite); err != nil {
				return
			}
			leidos, quien, err := conexion.ReadFrom(buzon)
			if err != nil {
				return
			}
			mensaje, err := icmp.ParseMessage(ipv4.ICMPTypeEchoReply.Protocol(), buzon[:leidos])
			if err != nil || mensaje.Type != ipv4.ICMPTypeEchoReply {
				continue
			}

			texto := direccionDe(quien)
			candado.Lock()
			if direccion, hay := buscadas[texto]; hay {
				contestaron[direccion] = true
			}
			candado.Unlock()
		}
	}()

	identificador := os.Getpid() & 0xffff
	secuencia := 0
	for _, direccion := range direcciones {
		if ctx.Err() != nil {
			break
		}
		if !direccion.Is4() {
			continue
		}
		secuencia++

		peticion := icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &icmp.Echo{
				ID:   identificador,
				Seq:  secuencia,
				Data: []byte("mired"),
			},
		}
		crudo, err := peticion.Marshal(nil)
		if err != nil {
			continue
		}
		if _, err := conexion.WriteTo(crudo, &net.UDPAddr{IP: net.IP(direccion.AsSlice())}); err != nil {
			if errors.Is(err, net.ErrClosed) {
				break
			}
			continue
		}
		time.Sleep(time.Millisecond)
	}

	select {
	case <-listo:
	case <-ctx.Done():
	case <-time.After(time.Until(limite) + time.Second):
	}

	candado.Lock()
	defer candado.Unlock()
	respuesta := make([]netip.Addr, 0, len(contestaron))
	for direccion := range contestaron {
		respuesta = append(respuesta, direccion)
	}
	return respuesta, nil
}

// abrirICMP intenta primero el socket sin privilegios y despues el crudo.
//
// Linux permite hacer ping sin ser root si el grupo del proceso esta dentro de
// net.ipv4.ping_group_range. Intentarlo primero significa que en muchos equipos
// el barrido funciona aunque la sonda pierda sus capacidades.
func abrirICMP() (*icmp.PacketConn, error) {
	if conexion, err := icmp.ListenPacket("udp4", "0.0.0.0"); err == nil {
		return conexion, nil
	}
	conexion, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir ICMP (¿faltan permisos de red cruda?): %w", err)
	}
	return conexion, nil
}

// direccionDe saca la IP de quien contesto, sirva el socket de un tipo o del
// otro.
func direccionDe(quien net.Addr) string {
	switch valor := quien.(type) {
	case *net.UDPAddr:
		return valor.IP.String()
	case *net.IPAddr:
		return valor.IP.String()
	default:
		return quien.String()
	}
}
