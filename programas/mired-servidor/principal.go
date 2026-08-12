// mired-servidor sirve la interfaz y la API de MiRed.
//
// Corre sin privilegios y es el UNICO proceso que escribe en las bases de datos.
// Todo lo que necesita permisos de red lo hace mired-sonda, que le entrega el
// resultado por un socket Unix.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tuxormax/mired/internal/api"
	"github.com/tuxormax/mired/internal/autenticacion"
	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/configuracion"
	"github.com/tuxormax/mired/internal/version"
)

func main() {
	archivoConfig := flag.String("configuracion", "", "ruta del archivo de configuracion")
	verVersion := flag.Bool("version", false, "muestra la version y termina")
	flag.Parse()

	if *verVersion {
		fmt.Println("mired-servidor", version.Completa())
		return
	}

	cfg, err := configuracion.Cargar(*archivoConfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error de configuracion:", err)
		os.Exit(1)
	}

	bitacora := armarBitacora(cfg.Registro.Nivel)
	if err := correr(cfg, bitacora); err != nil {
		bitacora.Error("el servidor se detuvo", "error", err)
		os.Exit(1)
	}
}

func correr(cfg configuracion.Configuracion, bitacora *slog.Logger) error {
	ctx, detener := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer detener()

	// Abrir el catalogo tambien lo migra. Una instalacion nueva queda con el
	// esquema puesto sin que el postinst tenga que correr ningun SQL.
	datos, err := basedatos.NuevoEnrutador(ctx, cfg.ArchivoCatalogo(), cfg.CarpetaRedes(),
		cfg.Datos.RedesAbiertas, cfg.Datos.InactividadRed.Duration)
	if err != nil {
		return err
	}
	defer datos.Cerrar()

	autenticador := autenticacion.Nuevo(datos, cfg.Servidor.DuracionSesion.Duration)
	sembrado, err := autenticador.SembrarSuperadmin(ctx)
	if err != nil {
		return err
	}
	if sembrado {
		bitacora.Warn("se creo el superadministrador inicial. CAMBIE LA CLAVE al entrar",
			"usuario", autenticacion.UsuarioSuperadmin,
			"clave", autenticacion.ClaveSuperadmin)
	}

	servicio := &api.API{
		Datos:       datos,
		Auth:        autenticador,
		Bitacora:    bitacora,
		RutaWeb:     cfg.Servidor.RutaWeb,
		SocketSonda: cfg.Sonda.Socket,
	}

	// Tareas de fondo: cerrar bases de red que ya nadie usa y limpiar sesiones
	// vencidas. Sin esto, con muchas redes el proceso acumula archivos abiertos
	// y la tabla de sesiones crece sola.
	go datos.Vigilar(ctx)
	go limpiarSesiones(ctx, datos, bitacora)

	servidor := &http.Server{
		Addr:              cfg.Servidor.Escucha,
		Handler:           servicio.Rutas(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errores := make(chan error, 1)
	go func() {
		bitacora.Info("MiRed escuchando",
			"direccion", cfg.Servidor.Escucha,
			"version", version.Completa(),
			"datos", cfg.Datos.Ruta)
		if err := servidor.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errores <- err
		}
	}()

	select {
	case err := <-errores:
		return err
	case <-ctx.Done():
		bitacora.Info("cerrando MiRed")
	}

	// Se le da margen a las peticiones en curso para terminar antes de cortar.
	cierre, cancelar := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelar()
	return servidor.Shutdown(cierre)
}

func limpiarSesiones(ctx context.Context, datos *basedatos.Enrutador, bitacora *slog.Logger) {
	reloj := time.NewTicker(time.Hour)
	defer reloj.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reloj.C:
			if err := datos.LimpiarSesiones(ctx); err != nil {
				bitacora.Warn("no se pudieron limpiar las sesiones vencidas", "error", err)
			}
		}
	}
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
