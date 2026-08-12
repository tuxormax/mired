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

	"github.com/tuxormax/mired/internal/snmp"
)

// Tipos de orden que entiende la sonda.
const (
	// OrdenEstado pide la ficha de la sonda: version e interfaces de red que ve.
	OrdenEstado = "estado"
	// OrdenEscanear le pide un barrido de una o varias subredes.
	OrdenEscanear = "escanear"
	// OrdenSNMP le pide interrogar por SNMP a una lista de equipos.
	OrdenSNMP = "snmp"
)

// PeticionSNMP es la lista de equipos a interrogar y con que credenciales.
type PeticionSNMP struct {
	Destinos     []string          `json:"destinos"`
	Credenciales []snmp.Credencial `json:"credenciales"`
	// EsperaMs es cuanto se espera a que cada equipo conteste. En una red con
	// muchos equipos que NO hablan SNMP, este numero manda sobre lo que tarda
	// todo: cada uno que no contesta cuesta una espera completa.
	EsperaMs int `json:"esperaMs,omitempty"`
}

// ResultadoSNMP son las fichas de los que si contestaron.
type ResultadoSNMP struct {
	Fichas       []snmp.Ficha `json:"fichas"`
	DuracionMs   int64        `json:"duracionMs"`
	Consultados  int          `json:"consultados"`
	Advertencias []string     `json:"advertencias,omitempty"`
}

// PedirSNMP le encarga a la sonda interrogar equipos por SNMP.
func PedirSNMP(socket string, peticion PeticionSNMP, espera time.Duration) (ResultadoSNMP, error) {
	crudo, err := json.Marshal(peticion)
	if err != nil {
		return ResultadoSNMP{}, fmt.Errorf("no se pudo armar la peticion SNMP: %w", err)
	}

	respuesta, err := Preguntar(socket, Orden{Tipo: OrdenSNMP, Datos: crudo}, espera)
	if err != nil {
		return ResultadoSNMP{}, err
	}
	if !respuesta.Ok {
		return ResultadoSNMP{}, fmt.Errorf("la sonda no pudo consultar SNMP: %s", respuesta.Error)
	}

	var resultado ResultadoSNMP
	if err := json.Unmarshal(respuesta.Datos, &resultado); err != nil {
		return ResultadoSNMP{}, fmt.Errorf("no se pudo interpretar el resultado SNMP: %w", err)
	}
	return resultado, nil
}

// PeticionEscaneo es lo que el servidor le pide barrer.
type PeticionEscaneo struct {
	Subredes []string `json:"subredes"`
	// Puertos a probar en cada equipo encontrado. Vacio usa la lista comun.
	Puertos []int `json:"puertos,omitempty"`
	// EsperaPuertoMs es cuanto se espera a que un puerto conteste.
	EsperaPuertoMs int `json:"esperaPuertoMs,omitempty"`
	// EsperaBarridoMs es cuanto se espera a que contesten los equipos.
	EsperaBarridoMs int `json:"esperaBarridoMs,omitempty"`
	// SoloPresencia hace un barrido rapido: quien esta, sin tocar puertos.
	SoloPresencia bool `json:"soloPresencia,omitempty"`
}

// ResultadoEscaneo es lo que la sonda encontro. La sonda NO guarda nada: se lo
// entrega al servidor, que es el unico que escribe en las bases.
type ResultadoEscaneo struct {
	Equipos      []EquipoVisto `json:"equipos"`
	DuracionMs   int64         `json:"duracionMs"`
	Advertencias []string      `json:"advertencias,omitempty"`
}

// EquipoVisto es un aparato que contesto.
type EquipoVisto struct {
	IP     string `json:"ip"`
	MAC    string `json:"mac,omitempty"`
	Nombre string `json:"nombre,omitempty"`
	// Fabricante sale del prefijo de la MAC.
	Fabricante string `json:"fabricante,omitempty"`
	// Metodo dice como se vio: arp, icmp o tcp. Un equipo visto por ARP es
	// seguro; uno visto solo por TCP pudo contestar desde otra parte.
	Metodo  string        `json:"metodo"`
	Subred  string        `json:"subred,omitempty"`
	Puertos []PuertoVisto `json:"puertos,omitempty"`
}

// PuertoVisto es un puerto abierto en un equipo.
type PuertoVisto struct {
	Numero    int    `json:"numero"`
	Protocolo string `json:"protocolo"`
	Servicio  string `json:"servicio,omitempty"`
	Banner    string `json:"banner,omitempty"`
}

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

// PedirEscaneo le encarga un barrido a la sonda.
//
// La espera se calcula generosa a proposito: un barrido de varias subredes con
// escaneo de puertos tarda, y cortar la conexion a la mitad significa perder
// todo el trabajo ya hecho.
func PedirEscaneo(socket string, peticion PeticionEscaneo, espera time.Duration) (ResultadoEscaneo, error) {
	crudo, err := json.Marshal(peticion)
	if err != nil {
		return ResultadoEscaneo{}, fmt.Errorf("no se pudo armar la peticion de escaneo: %w", err)
	}

	respuesta, err := Preguntar(socket, Orden{Tipo: OrdenEscanear, Datos: crudo}, espera)
	if err != nil {
		return ResultadoEscaneo{}, err
	}
	if !respuesta.Ok {
		return ResultadoEscaneo{}, fmt.Errorf("la sonda no pudo escanear: %s", respuesta.Error)
	}

	var resultado ResultadoEscaneo
	if err := json.Unmarshal(respuesta.Datos, &resultado); err != nil {
		return ResultadoEscaneo{}, fmt.Errorf("no se pudo interpretar el resultado del escaneo: %w", err)
	}
	return resultado, nil
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
