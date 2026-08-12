// Paquete programador corre los barridos de red: los que pide una persona desde
// la interfaz y los que van solos por agenda.
//
// Los dos caminos pasan por aqui a proposito. Si el boton de la interfaz y el
// reloj del sistema lanzaran barridos por su cuenta, nada impediria que corran
// dos a la vez sobre la misma red, y al marcar los equipos ausentes se pisarian:
// el resultado seria una red donde los equipos parpadean entre presente y
// ausente sin razon.
package programador

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/sonda"
)

// Errores que la capa de arriba distingue para responder con el mensaje justo.
var (
	// ErrYaEnCurso es que esa red ya se esta escaneando.
	ErrYaEnCurso = errors.New("ya hay un escaneo en curso en esta red")
	// ErrSinSubredes es que no hay nada que barrer.
	ErrSinSubredes = errors.New("la red no tiene subredes marcadas para escanear")
)

// esperaEscaneo es lo que se le da a la sonda para terminar un barrido antes de
// darlo por perdido. Un /24 con puertos tarda minutos, no segundos.
const esperaEscaneo = 20 * time.Minute

// Servicio lanza y vigila los barridos.
type Servicio struct {
	Datos       *basedatos.Enrutador
	SocketSonda string
	Bitacora    *slog.Logger

	mu      sync.Mutex
	enCurso map[string]bool
}

// Nuevo arma el servicio.
func Nuevo(datos *basedatos.Enrutador, socketSonda string, bitacora *slog.Logger) *Servicio {
	return &Servicio{
		Datos:       datos,
		SocketSonda: socketSonda,
		Bitacora:    bitacora,
		enCurso:     map[string]bool{},
	}
}

// EnCurso dice si esa red se esta escaneando ahora mismo.
func (s *Servicio) EnCurso(clave string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enCurso[clave]
}

func (s *Servicio) apartar(clave string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.enCurso[clave] {
		return false
	}
	s.enCurso[clave] = true
	return true
}

func (s *Servicio) liberar(clave string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.enCurso, clave)
}

// Lanzar arranca un barrido y devuelve el identificador del escaneo. No espera a
// que termine: un barrido tarda minutos y dejar la peticion HTTP abierta todo
// ese rato serviria para que cualquier corte la mate sin que nadie sepa si quedo.
func (s *Servicio) Lanzar(ctx context.Context, clave string, soloPresencia bool) (int64, []string, error) {
	subredes, err := s.subredesDe(ctx, clave)
	if err != nil {
		return 0, nil, err
	}
	if len(subredes) == 0 {
		return 0, nil, ErrSinSubredes
	}

	if !s.apartar(clave) {
		return 0, nil, ErrYaEnCurso
	}

	tipo := basedatos.TipoProfundo
	if soloPresencia {
		tipo = basedatos.TipoPresencia
	}

	var escaneoID int64
	err = s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
		var err error
		escaneoID, err = base.IniciarEscaneo(ctx, tipo)
		return err
	})
	if err != nil {
		s.liberar(clave)
		return 0, nil, err
	}

	go s.correr(clave, escaneoID, subredes, soloPresencia)
	return escaneoID, subredes, nil
}

func (s *Servicio) subredesDe(ctx context.Context, clave string) ([]string, error) {
	var subredes []string
	err := s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
		lista, err := base.ListarSubredes(ctx)
		if err != nil {
			return err
		}
		for _, subred := range lista {
			if subred.Escanear {
				subredes = append(subredes, subred.CIDR)
			}
		}
		return nil
	})
	return subredes, err
}

// correr le pide el barrido a la sonda y guarda lo que traiga.
func (s *Servicio) correr(clave string, escaneoID int64, subredes []string, soloPresencia bool) {
	defer s.liberar(clave)

	// Contexto propio: la peticion que lo lanzo ya termino hace rato, y el
	// programador no tiene ninguna.
	ctx, cancelar := context.WithTimeout(context.Background(), esperaEscaneo+time.Minute)
	defer cancelar()

	resultado, err := sonda.PedirEscaneo(s.SocketSonda, sonda.PeticionEscaneo{
		Subredes:      subredes,
		SoloPresencia: soloPresencia,
	}, esperaEscaneo)
	if err != nil {
		s.Bitacora.Error("el escaneo fallo", "red", clave, "escaneo", escaneoID, "error", err)
		s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
			return base.FallarEscaneo(ctx, escaneoID, err.Error())
		})
		return
	}
	for _, advertencia := range resultado.Advertencias {
		s.Bitacora.Warn("aviso del escaneo", "red", clave, "aviso", advertencia)
	}

	equipos := convertir(resultado.Equipos)

	err = s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
		resumen, err := base.GuardarDescubrimiento(ctx, escaneoID, !soloPresencia, equipos)
		if err != nil {
			return err
		}
		s.Bitacora.Info("escaneo terminado", "red", clave, "vistos", resumen.Vistos,
			"nuevos", resumen.Nuevos, "ausentes", resumen.Ausentes, "ms", resultado.DuracionMs)

		// El resumen del catalogo se actualiza al terminar cada barrido: es lo
		// que permite que el panel de inicio no abra el archivo de cada red.
		total, presentes, ultimo, err := base.ResumenDeRed(ctx)
		if err != nil {
			return err
		}
		return s.Datos.ActualizarResumen(ctx, clave, total, presentes, ultimo)
	})
	if err != nil {
		s.Bitacora.Error("no se pudo guardar el escaneo", "red", clave, "error", err)
		s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
			return base.FallarEscaneo(ctx, escaneoID, err.Error())
		})
	}
}

func convertir(vistos []sonda.EquipoVisto) []basedatos.EquipoDescubierto {
	equipos := make([]basedatos.EquipoDescubierto, 0, len(vistos))
	for _, visto := range vistos {
		puertos := make([]basedatos.PuertoDescubierto, 0, len(visto.Puertos))
		for _, puerto := range visto.Puertos {
			puertos = append(puertos, basedatos.PuertoDescubierto{
				Numero:    puerto.Numero,
				Protocolo: puerto.Protocolo,
				Servicio:  puerto.Servicio,
				Banner:    puerto.Banner,
			})
		}
		equipos = append(equipos, basedatos.EquipoDescubierto{
			IP:         visto.IP,
			MAC:        visto.MAC,
			Nombre:     visto.Nombre,
			Fabricante: visto.Fabricante,
			Metodo:     visto.Metodo,
			Subred:     visto.Subred,
			Puertos:    puertos,
		})
	}
	return equipos
}

// Vigilar corre la agenda hasta que se cancele el contexto.
//
// Revisa cada pocos segundos, no cada segundo: la agenda mas apretada que se
// puede configurar es de quince segundos, y preguntar mas seguido que eso solo
// gasta.
func (s *Servicio) Vigilar(ctx context.Context) {
	reloj := time.NewTicker(10 * time.Second)
	defer reloj.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reloj.C:
			s.revisarAgenda(ctx)
		}
	}
}

func (s *Servicio) revisarAgenda(ctx context.Context) {
	tareas, err := s.Datos.TareasPendientes(ctx)
	if err != nil {
		s.Bitacora.Warn("no se pudo revisar la agenda", "error", err)
		return
	}

	for _, tarea := range tareas {
		// La agenda se corre ANTES de lanzar el barrido, no despues. Si se
		// corriera al terminar, un barrido que tarda mas que su intervalo
		// dispararia el siguiente apenas acabe, y la red quedaria escaneandose
		// sin parar.
		if err := s.Datos.ProgramarSiguiente(ctx, tarea.Clave, tarea.Tipo); err != nil {
			s.Bitacora.Warn("no se pudo recorrer la agenda", "red", tarea.Clave, "error", err)
			continue
		}

		_, _, err := s.Lanzar(ctx, tarea.Clave, tarea.Tipo == basedatos.TipoPresencia)
		switch {
		case errors.Is(err, ErrYaEnCurso):
			// El barrido anterior todavia corre: se salta este turno sin ruido.
			// Pasa cuando el intervalo quedo mas corto que lo que tarda la red.
			s.Bitacora.Debug("se salto un barrido programado porque el anterior sigue",
				"red", tarea.Clave)
		case errors.Is(err, ErrSinSubredes):
			s.Bitacora.Warn("red programada sin subredes que escanear", "red", tarea.Clave)
		case err != nil:
			s.Bitacora.Error("no se pudo lanzar el barrido programado",
				"red", tarea.Clave, "error", fmt.Sprintf("%v", err))
		default:
			s.Bitacora.Debug("barrido programado lanzado", "red", tarea.Clave, "tipo", tarea.Tipo)
		}
	}
}
