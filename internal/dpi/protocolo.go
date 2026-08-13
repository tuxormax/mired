package dpi

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// El canal entre mired-servidor y mired-dpi.
//
// Es el MISMO reparto de papeles que con la sonda, y por las mismas razones: el
// que necesita permisos de red cruda no toca la base de datos, y el que escribe
// en la base no tiene privilegios. mired-dpi escucha en su socket Unix y el
// servidor le pregunta cada tanto que ha visto.
//
// Repetir el patron en vez de inventar otro no es pereza: significa que quien
// entienda como habla la sonda ya entiende como habla esto.

// Ordenes que entiende mired-dpi.
const (
	// OrdenEstado pide la ficha: version, interfaz y cuantos paquetes ha visto.
	OrdenEstado = "estado"
	// OrdenCortar entrega lo acumulado y limpia la cuenta.
	OrdenCortar = "cortar"
)

// Orden es lo que el servidor le pide.
type Orden struct {
	Tipo string `json:"tipo"`
}

// Respuesta es lo que mired-dpi contesta.
type Respuesta struct {
	Ok    bool            `json:"ok"`
	Datos json.RawMessage `json:"datos,omitempty"`
	Error string          `json:"error,omitempty"`
}

// Estado describe al proceso de inspeccion.
type Estado struct {
	Version  string `json:"version"`
	Build    string `json:"build"`
	Desde    string `json:"desde"`
	Interfaz string `json:"interfaz"`
	// Paquetes es cuantos se han visto desde que arranco. Es el unico dato que
	// dice si el puerto espejo esta bien configurado: con un puñado por minuto,
	// la respuesta es que no.
	Paquetes    uint64 `json:"paquetes"`
	Advertencia string `json:"advertencia,omitempty"`
}

// Corte es lo acumulado desde la ultima vez que se pregunto.
type Corte struct {
	Consumos []Consumo `json:"consumos"`
	Paquetes uint64    `json:"paquetes"`
}

// Preguntar manda una orden y espera la respuesta.
func Preguntar(socket string, orden Orden, espera time.Duration) (Respuesta, error) {
	conexion, err := net.DialTimeout("unix", socket, espera)
	if err != nil {
		return Respuesta{}, fmt.Errorf("no se pudo hablar con mired-dpi en %s: %w", socket, err)
	}
	defer conexion.Close()
	conexion.SetDeadline(time.Now().Add(espera))

	if err := json.NewEncoder(conexion).Encode(orden); err != nil {
		return Respuesta{}, fmt.Errorf("no se pudo enviar la orden a mired-dpi: %w", err)
	}

	var respuesta Respuesta
	if err := json.NewDecoder(bufio.NewReader(conexion)).Decode(&respuesta); err != nil {
		return Respuesta{}, fmt.Errorf("mired-dpi no contesto bien: %w", err)
	}
	return respuesta, nil
}

// PedirCorte recoge lo acumulado.
func PedirCorte(socket string, espera time.Duration) (Corte, error) {
	respuesta, err := Preguntar(socket, Orden{Tipo: OrdenCortar}, espera)
	if err != nil {
		return Corte{}, err
	}
	if !respuesta.Ok {
		return Corte{}, fmt.Errorf("mired-dpi no pudo entregar el corte: %s", respuesta.Error)
	}

	var corte Corte
	if err := json.Unmarshal(respuesta.Datos, &corte); err != nil {
		return Corte{}, fmt.Errorf("no se pudo interpretar el corte: %w", err)
	}
	return corte, nil
}

// PedirEstado pregunta como esta el proceso de inspeccion.
func PedirEstado(socket string, espera time.Duration) (Estado, error) {
	respuesta, err := Preguntar(socket, Orden{Tipo: OrdenEstado}, espera)
	if err != nil {
		return Estado{}, err
	}
	if !respuesta.Ok {
		return Estado{}, fmt.Errorf("mired-dpi respondio con error: %s", respuesta.Error)
	}

	var estado Estado
	if err := json.Unmarshal(respuesta.Datos, &estado); err != nil {
		return Estado{}, fmt.Errorf("no se pudo interpretar el estado de mired-dpi: %w", err)
	}
	return estado, nil
}
