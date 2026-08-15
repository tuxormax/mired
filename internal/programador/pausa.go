package programador

import (
	"sync"
	"time"
)

// Poder decirle a MiRed: "ahora no".
//
// Un barrido trabaja de verdad —barre direcciones, toca puertos, le pregunta a
// cada aparato— y eso se nota en un equipo modesto. Mientras alguien esta
// EDITANDO el mapa no tiene ningun sentido pagar ese coste: se esta declarando
// cableado, no midiendo la red.
//
// Por eso la interfaz puede pedir una pausa mientras dura la edicion. Es una
// pausa de la AGENDA, no del servicio: un barrido pedido a mano sigue
// lanzandose, porque eso lo pidio una persona a proposito.
//
// **La pausa siempre lleva vencimiento.** Si el programa se cierra de golpe —o
// alguien se deja el mapa abierto y se va— la red no puede quedarse sin
// vigilancia para siempre por un aviso que nadie retiro.

// topeDePausa es lo maximo que se puede pausar de una vez. Quien siga editando
// pasado ese rato lo pedira otra vez: la interfaz renueva sola.
const topeDePausa = 30 * time.Minute

type pausas struct {
	candado sync.RWMutex
	hasta   map[string]time.Time
}

// PausarAgenda deja de disparar barridos automaticos de una red.
//
// Devuelve hasta cuando queda pausada, que puede ser antes de lo pedido: nadie
// pausa una red por medio dia sin decirlo.
func (s *Servicio) PausarAgenda(clave string, cuanto time.Duration) time.Time {
	if cuanto <= 0 || cuanto > topeDePausa {
		cuanto = topeDePausa
	}
	hasta := time.Now().Add(cuanto)

	s.pausadas.candado.Lock()
	defer s.pausadas.candado.Unlock()
	if s.pausadas.hasta == nil {
		s.pausadas.hasta = map[string]time.Time{}
	}
	s.pausadas.hasta[clave] = hasta
	return hasta
}

// ReanudarAgenda vuelve a dejar correr los barridos automaticos.
func (s *Servicio) ReanudarAgenda(clave string) {
	s.pausadas.candado.Lock()
	defer s.pausadas.candado.Unlock()
	delete(s.pausadas.hasta, clave)
}

// EnPausa dice si una red tiene la agenda detenida ahora mismo.
func (s *Servicio) EnPausa(clave string) bool {
	s.pausadas.candado.RLock()
	hasta, hay := s.pausadas.hasta[clave]
	s.pausadas.candado.RUnlock()
	if !hay {
		return false
	}
	if time.Now().After(hasta) {
		// Vencio: se limpia al pasar por aqui, sin necesidad de un vigilante
		// aparte para algo que se resuelve solo.
		s.ReanudarAgenda(clave)
		return false
	}
	return true
}
