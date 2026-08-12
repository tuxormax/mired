package basedatos

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Enrutador es el unico punto del proyecto que decide a que archivo SQLite va
// cada consulta. Guarda el catalogo global y mantiene abiertas las bases de las
// redes que se estan usando, cerrando las que llevan rato quietas.
//
// No se pueden tener cien bases abiertas para siempre: cada una consume
// descriptores de archivo y memoria. Pero tampoco se puede cerrar una base que
// alguien esta usando, asi que se lleva la cuenta de prestamos vivos.
type Enrutador struct {
	// Catalogo es la base global. Siempre abierta: la usa cada peticion para
	// resolver quien entra y a que redes tiene acceso.
	Catalogo *Base

	carpetaRedes string
	maximo       int
	inactividad  time.Duration

	mu       sync.Mutex
	abiertas map[string]*redAbierta
	cerrado  bool
}

type redAbierta struct {
	base      *Base
	prestamos int       // cuantos la estan usando ahora mismo
	ultimoUso time.Time // para elegir cual cerrar cuando sobran
}

// NuevoEnrutador abre el catalogo, lo migra si hace falta y deja listo el juego
// de conexiones a las bases de red.
func NuevoEnrutador(ctx context.Context, archivoCatalogo, carpetaRedes string, maximo int, inactividad time.Duration) (*Enrutador, error) {
	catalogo, err := Abrir(archivoCatalogo)
	if err != nil {
		return nil, err
	}
	if err := catalogo.Migrar(ctx, EsquemaCatalogo); err != nil {
		catalogo.Close()
		return nil, fmt.Errorf("no se pudo preparar el catalogo: %w", err)
	}

	if maximo < 1 {
		maximo = 1
	}
	return &Enrutador{
		Catalogo:     catalogo,
		carpetaRedes: carpetaRedes,
		maximo:       maximo,
		inactividad:  inactividad,
		abiertas:     map[string]*redAbierta{},
	}, nil
}

// CarpetaRedes devuelve donde viven los archivos .db de las redes.
func (e *Enrutador) CarpetaRedes() string {
	return e.carpetaRedes
}

// ArchivoDeRed arma la ruta del archivo de una red a partir de su clave.
//
// El archivo se llama por la clave y no por el nombre: el nombre lleva acentos y
// espacios, se repite y cambia. La clave no cambia nunca.
func (e *Enrutador) ArchivoDeRed(clave string) string {
	return filepath.Join(e.carpetaRedes, clave+".db")
}

// Red presta la base de una red. Devuelve tambien la funcion que hay que llamar
// al terminar, sin excepcion:
//
//	base, devolver, err := enrutador.Red(ctx, "matriz-a1b2")
//	if err != nil { ... }
//	defer devolver()
//
// Mientras el prestamo este vivo la base no se cierra, aunque pase su tiempo de
// inactividad. Olvidar devolver() no corrompe nada, pero deja el archivo abierto
// para siempre.
func (e *Enrutador) Red(ctx context.Context, clave string) (*Base, func(), error) {
	e.mu.Lock()
	if e.cerrado {
		e.mu.Unlock()
		return nil, nil, fmt.Errorf("el enrutador ya se cerro")
	}
	if viva, hay := e.abiertas[clave]; hay {
		viva.prestamos++
		viva.ultimoUso = time.Now()
		e.mu.Unlock()
		return viva.base, func() { e.devolver(clave) }, nil
	}
	e.mu.Unlock()

	// Abrir y migrar puede tardar, asi que se hace fuera del candado. Si dos
	// peticiones abren la misma red a la vez, una de las dos sobra y se cierra
	// mas abajo; es raro y barato comparado con bloquear a todos.
	base, err := Abrir(e.ArchivoDeRed(clave))
	if err != nil {
		return nil, nil, err
	}
	// Una base restaurada de un respaldo llega con esquema atrasado: se actualiza
	// antes de dejar que nadie la toque.
	if err := base.Migrar(ctx, EsquemaRed); err != nil {
		base.Close()
		return nil, nil, fmt.Errorf("no se pudo preparar la red %s: %w", clave, err)
	}

	e.mu.Lock()
	if e.cerrado {
		e.mu.Unlock()
		base.Close()
		return nil, nil, fmt.Errorf("el enrutador ya se cerro")
	}
	if viva, hay := e.abiertas[clave]; hay {
		// Alguien se adelanto: se usa la suya y se descarta la nuestra.
		base.Close()
		viva.prestamos++
		viva.ultimoUso = time.Now()
		e.mu.Unlock()
		return viva.base, func() { e.devolver(clave) }, nil
	}
	viva := &redAbierta{base: base, prestamos: 1, ultimoUso: time.Now()}
	e.abiertas[clave] = viva
	e.recortarSinCandado()
	e.mu.Unlock()

	return base, func() { e.devolver(clave) }, nil
}

// ConRed presta la base, ejecuta la funcion y la devuelve pase lo que pase. Es
// la forma preferida: no se puede olvidar devolverla.
func (e *Enrutador) ConRed(ctx context.Context, clave string, hacer func(*Base) error) error {
	base, devolver, err := e.Red(ctx, clave)
	if err != nil {
		return err
	}
	defer devolver()
	return hacer(base)
}

func (e *Enrutador) devolver(clave string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if viva, hay := e.abiertas[clave]; hay && viva.prestamos > 0 {
		viva.prestamos--
		viva.ultimoUso = time.Now()
	}
}

// Olvidar cierra la base de una red y la saca del juego de conexiones. Se usa al
// borrar o archivar una red: mientras el archivo siga abierto no se puede mover
// ni borrar limpiamente.
func (e *Enrutador) Olvidar(clave string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if viva, hay := e.abiertas[clave]; hay {
		delete(e.abiertas, clave)
		viva.base.Close()
	}
}

// recortarSinCandado cierra las bases sobrantes cuando se pasa del tope. Cierra
// primero la que lleva mas tiempo sin usarse y nunca una prestada. Quien llama
// ya tiene el candado.
func (e *Enrutador) recortarSinCandado() {
	if len(e.abiertas) <= e.maximo {
		return
	}

	type candidata struct {
		clave string
		viva  *redAbierta
	}
	var libres []candidata
	for clave, viva := range e.abiertas {
		if viva.prestamos == 0 {
			libres = append(libres, candidata{clave, viva})
		}
	}
	sort.Slice(libres, func(i, j int) bool {
		return libres[i].viva.ultimoUso.Before(libres[j].viva.ultimoUso)
	})

	sobran := len(e.abiertas) - e.maximo
	for i := 0; i < sobran && i < len(libres); i++ {
		delete(e.abiertas, libres[i].clave)
		libres[i].viva.base.Close()
	}
}

// CerrarInactivas cierra las bases que llevan mas del tiempo de inactividad sin
// usarse. El servidor la llama cada tanto.
func (e *Enrutador) CerrarInactivas() {
	e.mu.Lock()
	defer e.mu.Unlock()

	limite := time.Now().Add(-e.inactividad)
	for clave, viva := range e.abiertas {
		if viva.prestamos == 0 && viva.ultimoUso.Before(limite) {
			delete(e.abiertas, clave)
			viva.base.Close()
		}
	}
}

// Vigilar cierra bases inactivas cada tanto hasta que se cancele el contexto.
func (e *Enrutador) Vigilar(ctx context.Context) {
	intervalo := e.inactividad / 2
	if intervalo < time.Minute {
		intervalo = time.Minute
	}
	reloj := time.NewTicker(intervalo)
	defer reloj.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reloj.C:
			e.CerrarInactivas()
		}
	}
}

// Cerrar cierra el catalogo y todas las bases de red abiertas.
func (e *Enrutador) Cerrar() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.cerrado = true
	for clave, viva := range e.abiertas {
		viva.base.Close()
		delete(e.abiertas, clave)
	}
	return e.Catalogo.Close()
}
