package programador

import (
	"context"
	"strings"
	"time"

	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/lectores"
)

// Preguntarle a cada aparato, con su credencial, quien tiene colgando.
//
// Es el tercer camino para saber quien cuelga de quien, y el ultimo:
//
//  1. La controladora WiFi, si la hay, lo sabe todo y no necesita nada de aqui.
//  2. La tabla de asociados por SNMP no pide credenciales de administrador.
//  3. Esto: entrar al panel con el usuario y la clave que el dueno guardo en la
//     ficha del aparato.
//
// **Solo se entra donde hay credencial guardada.** Sin ella no se intenta nada.
//
// # Lo inalambrico cuelga; lo de cable, NO
//
// Un panel lista las dos cosas juntas, pero no valen lo mismo:
//
//   - Un cliente asociado a un radio cuelga de ESE radio. No hay intermediario
//     posible: el aire no tiene switches en medio.
//   - Lo que el modem ve "por cable" es TODA su LAN, incluido lo que esta detras
//     del switch. Colgarlo del modem dibujaria una red plana donde hay un arbol,
//     y seria mentir con la cara mas seria del mundo.
//
// Por eso lo de cable no arma topologia. Se aprovecha para otra cosa: el nombre
// que el aparato le da a cada equipo —el que el propio equipo pidio al
// conectarse— se guarda como huella, y eso pone nombre a la mitad de las filas
// que hoy dicen solo una IP.

// preguntarALosPaneles recorre los equipos con credencial y lee su tabla.
func (s *Servicio) preguntarALosPaneles(ctx context.Context, clave string) {
	if s.Secretos == nil {
		return
	}

	err := s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
		equipos, err := base.ListarEquipos(ctx, true)
		if err != nil {
			return err
		}

		for _, equipo := range equipos {
			credencial, err := base.CredencialDeEquipo(ctx, s.Secretos, equipo.ID, "web", true)
			if err != nil || credencial.Clave == "" {
				// Sin credencial guardada no se toca el aparato. No es un fallo:
				// es que nadie dijo como entrar.
				continue
			}

			donde := credencial.Direccion
			if donde == "" {
				donde = equipo.IP
			}
			if donde == "" {
				continue
			}

			resultado, err := lectores.Leer(ctx, lectores.Destino{
				Direccion: donde,
				Usuario:   credencial.Usuario,
				Clave:     credencial.Clave,
				Pistas:    pistasDe(equipo),
				Espera:    12 * time.Second,
			})
			if err != nil {
				nivel := s.Bitacora.Debug
				if lectores.EsCredencialRechazada(err) {
					// Que la credencial guardada ya no sirva SI merece decirse:
					// se corrige cambiandola, no esperando al proximo escaneo.
					nivel = s.Bitacora.Warn
				}
				nivel("no se pudo leer el panel del aparato",
					"red", clave, "equipo", equipo.IP, "error", err)
				continue
			}

			colgados, nombres := s.guardarLoLeido(ctx, base, equipo.ID, resultado)
			s.Bitacora.Info("se leyo el panel del aparato",
				"red", clave, "equipo", equipo.IP, "lector", resultado.Lector,
				"colgados", colgados, "nombres", nombres)
		}
		return nil
	})
	if err != nil {
		s.Bitacora.Warn("no se pudo preguntar a los paneles", "red", clave, "error", err)
	}
}

// guardarLoLeido reparte lo que dijo el panel: lo inalambrico arma topologia y
// lo de cable solo aporta nombres.
func (s *Servicio) guardarLoLeido(ctx context.Context, base *basedatos.Base,
	aparatoID int64, resultado lectores.Resultado) (colgados, nombres int) {
	var inalambricos []basedatos.EnlaceInalambrico

	for _, conectado := range resultado.Conectados {
		equipoID, err := base.EquipoPorMAC(ctx, conectado.MAC)
		if err != nil {
			// Un conectado que el inventario todavia no conoce aparecera en el
			// proximo barrido. Crearlo aqui seria inventar un aparato del que
			// solo se sabe que alguien lo menciono.
			continue
		}
		if equipoID == aparatoID {
			continue
		}

		// El nombre que el aparato le da a cada equipo es el que el propio
		// equipo pidio al conectarse: sirve igual para el de cable y para el
		// inalambrico.
		if conectado.Nombre != "" {
			if err := base.GuardarHuellaDeEquipo(ctx, equipoID, []basedatos.DatoHuella{{
				Fuente: "panel", Clave: "nombre", Valor: conectado.Nombre,
			}}); err == nil {
				nombres++
			}
		}

		if conectado.PorCable {
			continue
		}
		enlace := basedatos.EnlaceInalambrico{EquipoID: equipoID, Red: conectado.Red}
		if conectado.SenalDbm != 0 {
			senal := conectado.SenalDbm
			enlace.SenalDbm = &senal
		}
		inalambricos = append(inalambricos, enlace)
	}

	if len(inalambricos) > 0 {
		guardados, err := base.GuardarAsociadosDeAntena(ctx, aparatoID,
			basedatos.OrigenPanel, inalambricos)
		if err == nil {
			colgados = guardados
		}
	}
	return colgados, nombres
}

// pistasDe junta lo que se sabe del aparato para elegir con que lector empezar.
func pistasDe(equipo basedatos.Equipo) string {
	partes := []string{equipo.Fabricante, equipo.Tipo, equipo.Modelo, equipo.Categoria}
	for _, dato := range equipo.Huella {
		partes = append(partes, dato.Valor)
	}
	return strings.ToLower(strings.Join(partes, " "))
}
