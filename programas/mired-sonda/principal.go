// mired-sonda es el proceso que toca la red.
//
// Es el unico binario de MiRed con permisos de red cruda (CAP_NET_RAW y
// CAP_NET_ADMIN), y a cambio NO toca ninguna base de datos: escanea y entrega lo
// que ve al servidor por un socket Unix local. Esa separacion es intencional: el
// proceso privilegiado es el mas pequeno y el mas facil de revisar.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/tuxormax/mired/internal/aire"
	"github.com/tuxormax/mired/internal/configuracion"
	"github.com/tuxormax/mired/internal/escaneo"
	"github.com/tuxormax/mired/internal/snmp"
	"github.com/tuxormax/mired/internal/sonda"
	"github.com/tuxormax/mired/internal/version"
)

func main() {
	archivoConfig := flag.String("configuracion", "", "ruta del archivo de configuracion")
	verVersion := flag.Bool("version", false, "muestra la version y termina")
	flag.Parse()

	if *verVersion {
		fmt.Println("mired-sonda", version.Completa())
		return
	}

	cfg, err := configuracion.Cargar(*archivoConfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error de configuracion:", err)
		os.Exit(1)
	}

	bitacora := armarBitacora(cfg.Registro.Nivel)
	if err := correr(cfg, bitacora); err != nil {
		bitacora.Error("la sonda se detuvo", "error", err)
		os.Exit(1)
	}
}

func correr(cfg configuracion.Configuracion, bitacora *slog.Logger) error {
	ctx, detener := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer detener()

	escucha, err := abrirSocket(cfg.Sonda.Socket)
	if err != nil {
		return err
	}
	defer escucha.Close()

	desde := time.Now().Format(time.RFC3339)
	bitacora.Info("sonda escuchando", "socket", cfg.Sonda.Socket, "version", version.Completa())
	if !puedeRedCruda() {
		bitacora.Warn("la sonda no tiene permisos de red cruda: el escaneo ARP no funcionara. " +
			"Faltan las capacidades CAP_NET_RAW y CAP_NET_ADMIN")
	}

	go func() {
		<-ctx.Done()
		escucha.Close()
	}()

	for {
		conexion, err := escucha.Accept()
		if err != nil {
			if ctx.Err() != nil {
				bitacora.Info("sonda detenida")
				return nil
			}
			bitacora.Error("no se pudo aceptar una conexion", "error", err)
			continue
		}
		go atender(conexion, desde, bitacora)
	}
}

// abrirSocket crea el socket Unix por donde entra el servidor.
//
// Los permisos son 0660 y no 0666: al socket solo llega quien este en el grupo
// mired. Un proceso privilegiado que acepte ordenes de cualquiera seria
// justamente el agujero que la separacion de privilegios quiere evitar.
func abrirSocket(ruta string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return nil, fmt.Errorf("no se pudo crear la carpeta del socket: %w", err)
	}
	// Un socket viejo de una caida anterior impide escuchar de nuevo.
	if err := os.Remove(ruta); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no se pudo limpiar el socket anterior: %w", err)
	}

	escucha, err := net.Listen("unix", ruta)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir el socket %s: %w", ruta, err)
	}
	if err := os.Chmod(ruta, 0o660); err != nil {
		escucha.Close()
		return nil, fmt.Errorf("no se pudieron ajustar los permisos del socket: %w", err)
	}
	return escucha, nil
}

func atender(conexion net.Conn, desde string, bitacora *slog.Logger) {
	defer conexion.Close()
	conexion.SetDeadline(time.Now().Add(30 * time.Second))

	var orden sonda.Orden
	if err := json.NewDecoder(conexion).Decode(&orden); err != nil {
		responder(conexion, sonda.Respuesta{Ok: false, Error: "no se entendio la orden: " + err.Error()})
		return
	}

	switch orden.Tipo {
	case sonda.OrdenEstado:
		estado, err := armarEstado(desde)
		if err != nil {
			responder(conexion, sonda.Respuesta{Ok: false, Error: err.Error()})
			return
		}
		datos, _ := json.Marshal(estado)
		responder(conexion, sonda.Respuesta{Ok: true, Datos: datos})

	case sonda.OrdenEscanear:
		var peticion sonda.PeticionEscaneo
		if err := json.Unmarshal(orden.Datos, &peticion); err != nil {
			responder(conexion, sonda.Respuesta{Ok: false, Error: "peticion de escaneo invalida: " + err.Error()})
			return
		}

		// Un barrido tarda mas que el plazo normal de la conexion, asi que se
		// amplia mientras dura.
		conexion.SetDeadline(time.Now().Add(30 * time.Minute))
		bitacora.Info("escaneo pedido", "subredes", peticion.Subredes, "soloPresencia", peticion.SoloPresencia)

		resultado, err := escaneo.Barrer(context.Background(), peticion)
		if err != nil {
			responder(conexion, sonda.Respuesta{Ok: false, Error: err.Error()})
			return
		}
		bitacora.Info("escaneo terminado",
			"equipos", len(resultado.Equipos), "ms", resultado.DuracionMs)

		datos, err := json.Marshal(resultado)
		if err != nil {
			responder(conexion, sonda.Respuesta{Ok: false, Error: "no se pudo armar el resultado: " + err.Error()})
			return
		}
		responder(conexion, sonda.Respuesta{Ok: true, Datos: datos})

	case sonda.OrdenSNMP:
		var peticion sonda.PeticionSNMP
		if err := json.Unmarshal(orden.Datos, &peticion); err != nil {
			responder(conexion, sonda.Respuesta{Ok: false, Error: "peticion SNMP invalida: " + err.Error()})
			return
		}

		conexion.SetDeadline(time.Now().Add(30 * time.Minute))
		resultado := consultarSNMP(peticion, bitacora)

		datos, err := json.Marshal(resultado)
		if err != nil {
			responder(conexion, sonda.Respuesta{Ok: false, Error: "no se pudo armar el resultado: " + err.Error()})
			return
		}
		responder(conexion, sonda.Respuesta{Ok: true, Datos: datos})

	case sonda.OrdenAire:
		// Barrer el aire tarda: la tarjeta tiene que recorrer los canales uno por
		// uno. Se amplia el plazo mientras dura, igual que en un escaneo.
		conexion.SetDeadline(time.Now().Add(2 * time.Minute))
		resultado := aire.Barrer(context.Background())
		bitacora.Info("aire barrido", "redes", len(resultado.Redes), "herramienta", resultado.Herramienta)

		datos, err := json.Marshal(resultado)
		if err != nil {
			responder(conexion, sonda.Respuesta{Ok: false, Error: "no se pudo armar el resultado: " + err.Error()})
			return
		}
		responder(conexion, sonda.Respuesta{Ok: true, Datos: datos})

	default:
		bitacora.Warn("orden desconocida", "tipo", orden.Tipo)
		responder(conexion, sonda.Respuesta{Ok: false, Error: "orden desconocida: " + orden.Tipo})
	}
}

func responder(conexion net.Conn, respuesta sonda.Respuesta) {
	json.NewEncoder(conexion).Encode(respuesta)
}

// armarEstado describe que ve la sonda: sus interfaces y si puede usar red
// cruda. La interfaz lo muestra para explicar por que un escaneo no esta
// disponible, en vez de dejar una pantalla vacia.
func armarEstado(desde string) (sonda.Estado, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return sonda.Estado{}, fmt.Errorf("no se pudieron leer las interfaces de red: %w", err)
	}

	estado := sonda.Estado{
		Version:  version.Numero + " Rev " + version.Revision,
		Build:    version.Build,
		Desde:    desde,
		RedCruda: puedeRedCruda(),
	}
	if !estado.RedCruda {
		estado.Advertencia = "La sonda no tiene permisos de red cruda (CAP_NET_RAW); " +
			"el barrido ARP no funcionara hasta concederlos."
	}

	for _, tarjeta := range interfaces {
		if tarjeta.Flags&net.FlagLoopback != 0 {
			continue
		}
		vista := sonda.Interfaz{
			Nombre: tarjeta.Name,
			MAC:    tarjeta.HardwareAddr.String(),
			Activa: tarjeta.Flags&net.FlagUp != 0,
		}
		direcciones, err := tarjeta.Addrs()
		if err == nil {
			for _, direccion := range direcciones {
				vista.Direcciones = append(vista.Direcciones, direccion.String())
			}
		}
		estado.Interfaces = append(estado.Interfaces, vista)
	}
	return estado, nil
}

// puedeRedCruda comprueba de verdad si se puede abrir un socket crudo, en vez de
// suponerlo por el usuario: correr como root no es la unica forma de tenerlo, y
// tener las capacidades sin ser root es justamente lo que hace el paquete .deb.
func puedeRedCruda() bool {
	descriptor, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, 0)
	if err != nil {
		return false
	}
	syscall.Close(descriptor)
	return true
}

func armarBitacora(nivel string) *slog.Logger {
	var severidad slog.Level
	switch nivel {
	case "debug":
		severidad = slog.LevelDebug
	case "aviso":
		severidad = slog.LevelWarn
	case "error":
		severidad = slog.LevelError
	default:
		severidad = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: severidad}))
}

// consultarSNMP interroga en paralelo a los equipos indicados.
//
// El paralelismo importa mas aqui que en cualquier otro lado: en una red normal
// la enorme mayoria de los equipos NO habla SNMP, y cada uno de esos cuesta la
// espera completa. En serie, un /24 serian varios minutos de puro esperar.
func consultarSNMP(peticion sonda.PeticionSNMP, bitacora *slog.Logger) sonda.ResultadoSNMP {
	inicio := time.Now()
	resultado := sonda.ResultadoSNMP{Fichas: []snmp.Ficha{}, Consultados: len(peticion.Destinos)}

	if len(peticion.Credenciales) == 0 {
		resultado.Advertencias = append(resultado.Advertencias,
			"no hay credenciales SNMP configuradas: sin ellas no se puede preguntar a los switches")
		return resultado
	}

	espera := 2 * time.Second
	if peticion.EsperaMs > 0 {
		espera = time.Duration(peticion.EsperaMs) * time.Millisecond
	}

	permisos := make(chan struct{}, 16)
	var candado sync.Mutex
	var grupo sync.WaitGroup

	for _, destino := range peticion.Destinos {
		grupo.Add(1)
		go func(ip string) {
			defer grupo.Done()
			permisos <- struct{}{}
			defer func() { <-permisos }()

			ficha, err := snmp.Consultar(ip, peticion.Credenciales, espera)
			if err != nil {
				// Que un equipo no conteste SNMP es lo normal, no un error:
				// solo los administrables hablan. No se registra para no llenar
				// la bitacora de ruido.
				return
			}
			candado.Lock()
			resultado.Fichas = append(resultado.Fichas, ficha)
			candado.Unlock()
		}(destino)
	}
	grupo.Wait()

	resultado.DuracionMs = time.Since(inicio).Milliseconds()
	bitacora.Info("consulta SNMP terminada",
		"consultados", resultado.Consultados, "contestaron", len(resultado.Fichas),
		"ms", resultado.DuracionMs)
	return resultado
}
