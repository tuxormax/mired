package programador

import (
	"context"
	"fmt"
	"time"

	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/controladora"
)

// consultarControladoras le pregunta a las controladoras WiFi que aparato esta
// colgado de que antena, y lo guarda como si fuera un mapa de puertos mas.
//
// **Por que se guarda por el mismo camino que SNMP:** un punto de acceso es, para
// el mapa, un switch cuyos puertos son redes WiFi en vez de puertos. Metiendo el
// dato por GuardarSNMP se hereda gratis todo lo que ya funciona —el mapa, la
// exportacion, la alerta de "se movio de lugar", el perfil de capacidades— en
// vez de escribir un segundo mapa que habria que mantener en paralelo.
//
// **Por que corre en el servidor y no en la sonda:** hablar con la controladora
// es HTTPS contra una direccion de la red. No hace falta ningun privilegio, y la
// sonda existe precisamente para lo que si lo necesita.
func (s *Servicio) consultarControladoras(ctx context.Context, clave string) {
	// Las controladoras son DE ESTA RED: se leen de su base, no de una lista
	// compartida. La de un cliente no tiene nada que hacer en la red de otro.
	var controladoras []basedatos.Controladora
	err := s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
		var err error
		controladoras, err = base.ListarControladoras(ctx)
		return err
	})
	if err != nil {
		s.Bitacora.Warn("no se pudieron leer las controladoras", "red", clave, "error", err)
		return
	}
	if len(controladoras) == 0 {
		return // Nadie configuro ninguna. No es un error.
	}

	var fichas []basedatos.FichaSNMP
	for _, guardada := range controladoras {
		lectura, err := controladora.Consultar(ctx, controladora.Acceso{
			Nombre:       guardada.Nombre,
			URL:          guardada.URL,
			Usuario:      guardada.Usuario,
			Clave:        guardada.Clave,
			Sitio:        guardada.Sitio,
			VerificarTLS: guardada.VerificarTLS,
		}, 15*time.Second)

		// El resultado se anota siempre, salga bien o mal: una controladora que
		// lleva dias sin contestar tiene que poder verse en la pantalla, no
		// desaparecer del mapa en silencio.
		anotar := s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
			return base.AnotarConsultaAControladora(ctx, guardada.ID, err)
		})
		if anotar != nil {
			s.Bitacora.Warn("no se pudo anotar el resultado de la controladora",
				"controladora", guardada.Nombre, "error", anotar)
		}
		if err != nil {
			s.Bitacora.Warn("la controladora no contesto",
				"controladora", guardada.Nombre, "error", err)
			continue
		}

		fichas = append(fichas, fichasDeControladora(lectura, guardada.Nombre)...)
	}
	if len(fichas) == 0 {
		return
	}

	err = s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
		movimientos, err := base.GuardarSNMP(ctx, fichas)
		if err != nil {
			return err
		}
		if len(movimientos) > 0 {
			if err := base.AlertasDeMovimiento(ctx, movimientos); err != nil {
				return err
			}
		}
		capacidad, err := base.CalcularCapacidades(ctx)
		if err != nil {
			return err
		}
		s.Bitacora.Info("controladoras consultadas", "red", clave,
			"aparatos", len(fichas), "mapa de puertos", capacidad)
		return nil
	})
	if err != nil {
		s.Bitacora.Error("no se pudo guardar lo que dijo la controladora",
			"red", clave, "error", err)
	}
}

// fichasDeControladora traduce lo que dijo la controladora a la misma forma que
// devuelve SNMP.
//
// Se descartan los aparatos sin IP: la ficha se cuelga del equipo descubierto
// por el barrido, y sin direccion no hay a quien colgarla. Un punto de acceso
// que la controladora administra pero que el barrido nunca vio es, casi siempre,
// uno de otra subred que esta red no escanea.
func fichasDeControladora(lectura controladora.Lectura, nombreControladora string) []basedatos.FichaSNMP {
	// Que cuelga de cada puerto de cada aparato.
	porAparato := map[string]map[string][]string{}
	for _, conectado := range lectura.Conectados {
		puertos, hay := porAparato[conectado.AparatoMAC]
		if !hay {
			puertos = map[string][]string{}
			porAparato[conectado.AparatoMAC] = puertos
		}
		clave := fmt.Sprintf("%d", conectado.Puerto)
		puertos[clave] = append(puertos[clave], conectado.MAC)
	}

	fichas := make([]basedatos.FichaSNMP, 0, len(lectura.Aparatos))
	for _, aparato := range lectura.Aparatos {
		if aparato.IP == "" {
			continue
		}

		puertos := aparato.Puertos
		if aparato.EsPuntoDeAcceso {
			// Un punto de acceso no tiene una lista de puertos que consultar: sus
			// puertos son las redes WiFi, y solo se sabe cuales estan en uso
			// mirando quien esta conectado.
			puertos = controladora.PuertosDeRedes(lectura.Conectados, aparato.MAC)
		}

		interfaces := make([]basedatos.InterfazSNMP, 0, len(puertos))
		for _, puerto := range puertos {
			interfaces = append(interfaces, basedatos.InterfazSNMP{
				Indice:        puerto.Indice,
				Nombre:        puerto.Nombre,
				Activa:        puerto.Activa,
				VelocidadMbps: puerto.VelocidadMbps,
			})
		}

		fichas = append(fichas, basedatos.FichaSNMP{
			IP:          aparato.IP,
			Nombre:      aparato.Nombre,
			Descripcion: aparato.Modelo,
			// Se marca como switch tambien a los puntos de acceso: para el mapa
			// lo son —tienen puertos de las que cuelgan equipos—, y llamarlos de
			// otra forma obligaria a duplicar toda la logica del dibujo.
			EsSwitch:      true,
			Credencial:    "controladora " + nombreControladora,
			Interfaces:    interfaces,
			MacsPorPuerto: porAparato[aparato.MAC],
		})
	}
	return fichas
}
