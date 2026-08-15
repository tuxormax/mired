// mired-dpi mira los paquetes para decir QUE aplicacion consume, no solo
// cuanto.
//
// Es la fase 10 del plan y viaja en un paquete Debian **aparte**, a proposito:
// mirar todos los paquetes cuesta mucho mas que preguntarle a un switch cuantos
// bytes paso por un puerto, y esa es la diferencia entre MiRed y las herramientas
// que acaban en "consumo alto". Quien no lo instale no paga nada por ello.
//
// Reparte los papeles igual que mired-sonda: es el que necesita permisos de red
// cruda, asi que NO toca ninguna base de datos. Escucha en su socket Unix y el
// servidor le pregunta cada tanto que ha visto.
//
// **No descifra nada.** Lee el nombre del servidor del saludo de TLS, la
// cabecera Host de HTTP y las consultas de DNS, que viajan en claro. No hay
// intercepcion, ni certificados propios, ni se toca el contenido de nada.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tuxormax/mired/internal/configuracion"
	"github.com/tuxormax/mired/internal/dpi"
	"github.com/tuxormax/mired/internal/version"
)

func main() {
	archivoConfig := flag.String("configuracion", "", "ruta del archivo de configuracion")
	interfaz := flag.String("interfaz", "", "tarjeta donde llega el puerto espejo")
	verVersion := flag.Bool("version", false, "muestra la version y termina")
	flag.Parse()

	if *verVersion {
		fmt.Println("mired-dpi", version.Completa())
		return
	}

	cfg, err := configuracion.Cargar(*archivoConfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error de configuracion:", err)
		os.Exit(1)
	}
	if *interfaz != "" {
		cfg.Dpi.Interfaz = *interfaz
	}

	bitacora := armarBitacora(cfg.Registro.Nivel)
	if err := correr(cfg, bitacora); err != nil {
		bitacora.Error("mired-dpi se detuvo", "error", err)
		os.Exit(1)
	}
}

func correr(cfg configuracion.Configuracion, bitacora *slog.Logger) error {
	if cfg.Dpi.Interfaz == "" {
		// Sin interfaz no hay nada que mirar. Se dice y se sale con bien: es una
		// instalacion a medio configurar, no un fallo.
		bitacora.Warn("mired-dpi no tiene interfaz configurada y no va a capturar nada. " +
			"Ponga [dpi] interfaz = \"eth1\" en /etc/mired/mired.toml, apuntando a la tarjeta " +
			"donde llega el puerto espejo del switch")
		return nil
	}

	ctx, detener := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer detener()

	locales, err := redesLocales()
	if err != nil {
		bitacora.Warn("no se pudieron leer las redes locales; se contara todo", "error", err)
	}

	capturador := &dpi.Capturador{
		Interfaz: cfg.Dpi.Interfaz,
		EsLocal: func(direccion netip.Addr) bool {
			for _, prefijo := range locales {
				if prefijo.Contains(direccion) {
					return true
				}
			}
			return false
		},
	}

	escucha, err := abrirSocket(cfg.Dpi.Socket)
	if err != nil {
		return err
	}
	defer escucha.Close()

	desde := time.Now().Format(time.RFC3339)
	bitacora.Info("mired-dpi capturando",
		"interfaz", cfg.Dpi.Interfaz, "socket", cfg.Dpi.Socket, "version", version.Completa())

	// La captura corre en su propia rutina: el socket tiene que seguir
	// contestando aunque no pase un solo paquete por el cable.
	problemas := make(chan error, 1)
	go func() { problemas <- capturador.Escuchar(ctx) }()

	go func() {
		<-ctx.Done()
		escucha.Close()
	}()

	for {
		select {
		case err := <-problemas:
			if err != nil {
				return err
			}
			return nil
		default:
		}

		conexion, err := escucha.Accept()
		if err != nil {
			if ctx.Err() != nil {
				bitacora.Info("mired-dpi detenido")
				return nil
			}
			bitacora.Error("no se pudo aceptar una conexion", "error", err)
			continue
		}
		go atender(conexion, capturador, cfg.Dpi.Interfaz, desde, bitacora)
	}
}

// abrirSocket crea el socket Unix por donde entra el servidor.
//
// Permisos 0660, igual que el de la sonda: al socket solo llega quien este en el
// grupo mired. Un proceso privilegiado que acepte ordenes de cualquiera seria el
// agujero que la separacion de privilegios quiere evitar.
func abrirSocket(ruta string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return nil, fmt.Errorf("no se pudo crear la carpeta del socket: %w", err)
	}
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

func atender(conexion net.Conn, capturador *dpi.Capturador, interfaz, desde string, bitacora *slog.Logger) {
	defer conexion.Close()
	conexion.SetDeadline(time.Now().Add(30 * time.Second))

	var orden dpi.Orden
	if err := json.NewDecoder(conexion).Decode(&orden); err != nil {
		responder(conexion, dpi.Respuesta{Ok: false, Error: "no se entendio la orden: " + err.Error()})
		return
	}

	switch orden.Tipo {
	case dpi.OrdenEstado:
		vistos := capturador.Paquetes()
		estado := dpi.Estado{
			Version:  version.Numero + " Rev " + version.Revision,
			Build:    version.Build,
			Desde:    desde,
			Interfaz: interfaz,
			Paquetes: vistos,
		}
		// Es el aviso que de verdad hace falta: sin puerto espejo esto no falla,
		// simplemente ve casi nada, y sin decirlo el informe pareceria correcto.
		if vistos < 100 {
			estado.Advertencia = "Se han visto muy pocos paquetes. Casi seguro falta configurar " +
				"el puerto espejo (SPAN) del switch hacia la interfaz " + interfaz + "."
		}
		datos, _ := json.Marshal(estado)
		responder(conexion, dpi.Respuesta{Ok: true, Datos: datos})

	case dpi.OrdenCortar:
		corte := dpi.Corte{
			Consumos: capturador.Cortar(),
			Paquetes: capturador.Paquetes(),
		}
		datos, err := json.Marshal(corte)
		if err != nil {
			responder(conexion, dpi.Respuesta{Ok: false, Error: "no se pudo armar el corte: " + err.Error()})
			return
		}
		bitacora.Debug("corte entregado", "renglones", len(corte.Consumos))
		responder(conexion, dpi.Respuesta{Ok: true, Datos: datos})

	default:
		bitacora.Warn("orden desconocida", "tipo", orden.Tipo)
		responder(conexion, dpi.Respuesta{Ok: false, Error: "orden desconocida: " + orden.Tipo})
	}
}

func responder(conexion net.Conn, respuesta dpi.Respuesta) {
	_ = json.NewEncoder(conexion).Encode(respuesta)
}

// redesLocales lee que rangos tiene este equipo, para saber a quien apuntarle el
// consumo de cada conversacion.
//
// Se sacan de las propias tarjetas y no de la base de datos porque mired-dpi NO
// habla con la base: es el mismo principio que con la sonda.
func redesLocales() ([]netip.Prefix, error) {
	direcciones, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	var prefijos []netip.Prefix
	for _, direccion := range direcciones {
		red, ok := direccion.(*net.IPNet)
		if !ok {
			continue
		}
		prefijo, err := netip.ParsePrefix(red.String())
		if err != nil {
			continue
		}
		if prefijo.Addr().IsLoopback() {
			continue
		}
		prefijos = append(prefijos, prefijo.Masked())
	}
	return prefijos, nil
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
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: severidad}))
}
