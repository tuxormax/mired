// Paquete sonda define el canal entre mired-servidor y mired-sonda.
//
// Los dos procesos estan separados a proposito: la sonda es la unica que
// necesita permisos de red cruda (ARP, sondeos, escucha de LLDP), y el servidor
// es el unico que escribe en las bases de datos. Hablan por un socket Unix
// local, nunca por la red.
//
// El protocolo es una linea JSON por mensaje: sencillo de depurar con nc y
// suficiente para el volumen de esta herramienta.
package sonda

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Tipos de orden que entiende la sonda.
const (
	// OrdenEstado pide la ficha de la sonda: version e interfaces de red que ve.
	OrdenEstado = "estado"
)

// Orden es lo que el servidor le pide a la sonda.
type Orden struct {
	Tipo  string          `json:"tipo"`
	Datos json.RawMessage `json:"datos,omitempty"`
}

// Respuesta es lo que la sonda contesta.
type Respuesta struct {
	Ok    bool            `json:"ok"`
	Datos json.RawMessage `json:"datos,omitempty"`
	Error string          `json:"error,omitempty"`
}

// Estado describe a la sonda: sirve para que la interfaz diga si el escaneo esta
// disponible y con que permisos, en vez de fallar cuando se pide un escaneo.
type Estado struct {
	Version     string     `json:"version"`
	Build       string     `json:"build"`
	Desde       string     `json:"desde"`
	RedCruda    bool       `json:"redCruda"`
	Interfaces  []Interfaz `json:"interfaces"`
	Advertencia string     `json:"advertencia,omitempty"`
}

// Interfaz es una tarjeta de red vista por la sonda.
type Interfaz struct {
	Nombre      string   `json:"nombre"`
	MAC         string   `json:"mac"`
	Direcciones []string `json:"direcciones"`
	Activa      bool     `json:"activa"`
}

// Preguntar manda una orden a la sonda y espera su respuesta.
func Preguntar(socket string, orden Orden, espera time.Duration) (Respuesta, error) {
	conexion, err := net.DialTimeout("unix", socket, espera)
	if err != nil {
		return Respuesta{}, fmt.Errorf("no se pudo hablar con la sonda en %s: %w", socket, err)
	}
	defer conexion.Close()
	conexion.SetDeadline(time.Now().Add(espera))

	if err := json.NewEncoder(conexion).Encode(orden); err != nil {
		return Respuesta{}, fmt.Errorf("no se pudo enviar la orden a la sonda: %w", err)
	}

	var respuesta Respuesta
	if err := json.NewDecoder(bufio.NewReader(conexion)).Decode(&respuesta); err != nil {
		return Respuesta{}, fmt.Errorf("la sonda no contesto bien: %w", err)
	}
	return respuesta, nil
}

// PedirEstado es el atajo para la orden mas comun.
func PedirEstado(socket string, espera time.Duration) (Estado, error) {
	respuesta, err := Preguntar(socket, Orden{Tipo: OrdenEstado}, espera)
	if err != nil {
		return Estado{}, err
	}
	if !respuesta.Ok {
		return Estado{}, fmt.Errorf("la sonda respondio con error: %s", respuesta.Error)
	}

	var estado Estado
	if err := json.Unmarshal(respuesta.Datos, &estado); err != nil {
		return Estado{}, fmt.Errorf("no se pudo interpretar el estado de la sonda: %w", err)
	}
	return estado, nil
}
