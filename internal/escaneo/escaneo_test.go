package escaneo

import (
	"net"
	"net/netip"
	"testing"

	"github.com/tuxormax/mired/internal/sonda"
)

// direccionPropia busca una direccion IPv4 de este equipo para armar la prueba
// con datos reales, no inventados.
func direccionPropia(t *testing.T) netip.Addr {
	t.Helper()
	tarjetas, err := net.Interfaces()
	if err != nil {
		t.Skipf("no se pudieron leer las tarjetas: %v", err)
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
			if !ok || red.IP.To4() == nil {
				continue
			}
			propia, ok := netip.AddrFromSlice(red.IP.To4())
			if ok {
				return propia
			}
		}
	}
	t.Skip("este equipo no tiene ninguna direccion IPv4 de red")
	return netip.Addr{}
}

func TestElEquipoDondeCorreMiRedSeVeASiMismo(t *testing.T) {
	// El bug: la PC del usuario era el unico aparato de la casa que no salia en
	// el inventario. Nadie se manda un ARP a si mismo, y su ping se contesta por
	// dentro sin salir a la red, asi que las dos capas de descubrimiento la
	// saltaban.
	propia := direccionPropia(t)
	prefijo := netip.PrefixFrom(propia, 24).Masked()

	vistos := map[string]*sonda.EquipoVisto{}
	anotarEsteEquipo(vistos, prefijo)

	equipo, hay := vistos[propia.String()]
	if !hay {
		t.Fatalf("%s es de este equipo y no aparecio en el barrido de %s", propia, prefijo)
	}
	if equipo.Metodo != "propio" {
		t.Errorf("se anoto con el metodo %q y debia ser propio", equipo.Metodo)
	}
	if equipo.MAC == "" {
		t.Error("de la tarjeta propia se conoce la MAC: no puede quedar vacia")
	}
	if equipo.Nombre == "" {
		t.Error("de este equipo se conoce el nombre: no puede quedar vacio")
	}
}

func TestFueraDelRangoNoSeAnotaNada(t *testing.T) {
	// Escanear la red del vecino no puede hacer aparecer a este equipo dentro
	// de ella.
	vistos := map[string]*sonda.EquipoVisto{}
	anotarEsteEquipo(vistos, netip.MustParsePrefix("203.0.113.0/24"))

	if len(vistos) != 0 {
		t.Fatalf("se anotaron %d equipos en una subred que no es de aqui", len(vistos))
	}
}

func TestSerEsteEquipoEsElDatoMasFirme(t *testing.T) {
	// Si el mismo aparato se vio por TCP —el mas debil— y ademas resulta ser
	// este equipo, la certeza que se guarda es la segunda.
	vistos := map[string]*sonda.EquipoVisto{}
	prefijo := netip.MustParsePrefix("192.168.1.0/24")
	direccion := netip.MustParseAddr("192.168.1.100")

	anotar(vistos, direccion, prefijo.String(), "tcp", nil)
	anotar(vistos, direccion, prefijo.String(), "propio", nil)

	if vistos[direccion.String()].Metodo != "propio" {
		t.Fatalf("gano %q sobre propio", vistos[direccion.String()].Metodo)
	}

	// Y al reves: haberse visto despues por un camino peor no degrada el dato.
	anotar(vistos, direccion, prefijo.String(), "icmp", nil)
	if vistos[direccion.String()].Metodo != "propio" {
		t.Fatalf("icmp piso a propio: quedo %q", vistos[direccion.String()].Metodo)
	}
}
