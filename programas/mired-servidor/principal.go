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
	"github.com/tuxormax/mired/internal/catalogo"
	"github.com/tuxormax/mired/internal/configuracion"
	"github.com/tuxormax/mired/internal/programador"
	"github.com/tuxormax/mired/internal/secreto"
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

	// El historial de versiones viaja dentro del binario y se siembra en cada
	// arranque. Es idempotente: reinstalar no duplica nada.
	if historial, err := version.LeerHistorial(); err != nil {
		bitacora.Warn("no se pudo leer el historial de versiones", "error", err)
	} else if err := datos.SembrarVersiones(ctx,
		aEntradas(historial.Sistema), aEntradas(historial.Base)); err != nil {
		bitacora.Warn("no se pudo sembrar el historial de versiones", "error", err)
	}

	// Las credenciales SNMP y las controladoras estuvieron compartidas entre
	// todas las redes hasta la Rev 44. Al arrancar con la version nueva se
	// reparten a cada red y se vacia el catalogo; despues de eso esto no vuelve a
	// hacer nada, porque no queda nada que mudar.
	if credenciales, controladoras, err := datos.MudarCredencialesALasRedes(ctx); err != nil {
		bitacora.Warn("no se pudo mudar lo compartido a cada red", "error", err)
	} else if credenciales > 0 || controladoras > 0 {
		bitacora.Info("las credenciales y controladoras ahora son de cada red",
			"credenciales", credenciales, "controladoras", controladoras)
	}

	autenticador := autenticacion.Nuevo(datos, cfg.Servidor.DuracionSesion.Duration)
	// No se siembra ningun usuario: el primero lo crea quien entre, con el
	// usuario y la clave que el elija. Unas credenciales iguales en todas las
	// instalaciones serian, en un proyecto publico, una puerta que cualquiera
	// puede buscar.
	sinEstrenar, err := autenticador.SinEstrenar(ctx)
	if err != nil {
		return err
	}
	if sinEstrenar {
		bitacora.Info("instalacion sin estrenar: al entrar por primera vez se pedira " +
			"crear el administrador")
	}

	// El catalogo de dispositivos se carga al arrancar. Que falte o tenga
	// archivos rotos NO impide arrancar: solo deja equipos sin reconocer, y eso
	// se dice en la bitacora en vez de tumbar el servicio.
	dispositivos, err := catalogo.Cargar(cfg.Catalogo.Carpetas)
	if err != nil {
		bitacora.Warn("no se pudo cargar el catalogo de dispositivos", "error", err)
	}
	if dispositivos != nil {
		bitacora.Info("catalogo de dispositivos cargado",
			"definiciones", len(dispositivos.Definiciones()))
		for _, problema := range dispositivos.Problemas() {
			bitacora.Warn("definicion de dispositivo con problemas", "detalle", problema)
		}
	}

	agenda := programador.Nuevo(datos, cfg.Sonda.Socket, bitacora)
	agenda.Catalogo = dispositivos

	// La caja de secretos cifra las claves de los equipos. La llave se crea sola
	// la primera vez, con permisos 0600, AL LADO DE LA CONFIGURACION: si viviera
	// con los datos, viajaria en cada respaldo y cifrar no serviria de nada.
	secretos := secreto.NuevaCaja(cfg.ArchivoLlave())
	if err := secretos.Listo(); err != nil {
		bitacora.Warn("no se pudo preparar el cifrado de claves de equipo",
			"llave", cfg.ArchivoLlave(), "error", err)
	}

	agenda.Secretos = secretos

	servicio := &api.API{
		Datos:       datos,
		Auth:        autenticador,
		Bitacora:    bitacora,
		SocketSonda: cfg.Sonda.Socket,
		Programador: agenda,
		Catalogo:    dispositivos,
		Secretos:    secretos,
		// Las mismas carpetas de donde se cargo, para poder releerlas cuando se
		// guarde una definicion propia o se traigan las de la comunidad, sin
		// reiniciar el servicio.
		CarpetasCatalogo: cfg.Catalogo.Carpetas,
		CarpetaPropia:    cfg.Catalogo.Propia,
		CarpetaComunidad: cfg.Catalogo.Comunidad,
	}

	// Tareas de fondo: cerrar bases de red que ya nadie usa y limpiar sesiones
	// vencidas. Sin esto, con muchas redes el proceso acumula archivos abiertos
	// y la tabla de sesiones crece sola.
	go datos.Vigilar(ctx)
	go limpiarSesiones(ctx, datos, bitacora)
	// El programador corre los barridos automaticos de las redes que lo tengan
	// encendido: presencia frecuente y escaneo profundo espaciado.
	go agenda.Vigilar(ctx)
	// El receptor de flujos es lo que mide consumo en los sitios sin switches
	// administrables. Que no se pueda abrir el puerto NO tumba el servicio: se
	// avisa y todo lo demas sigue funcionando.
	if cfg.Flujos.Escucha != "" {
		go func() {
			if err := agenda.RecibirFlujos(ctx, cfg.Flujos.Escucha); err != nil {
				bitacora.Warn("no se pudieron recibir flujos del router",
					"direccion", cfg.Flujos.Escucha, "error", err)
			}
		}()
	}
	// La inspeccion profunda es OPCIONAL y viaja en otro paquete: lo normal es
	// que nadie conteste en ese socket. Por eso se recoge en silencio y no se
	// avisa de nada si no esta.
	if cfg.Dpi.Socket != "" {
		go agenda.RecogerInspeccion(ctx, cfg.Dpi.Socket,
			time.Duration(cfg.Dpi.CadaMinutos)*time.Minute)
	}

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

// aEntradas pasa el historial del paquete version al de la base. Son dos
// estructuras iguales a proposito: la capa de datos no tiene por que depender
// del formato del archivo que se compilo dentro.
func aEntradas(origen []version.Entrada) []basedatos.EntradaDeVersion {
	entradas := make([]basedatos.EntradaDeVersion, 0, len(origen))
	for _, entrada := range origen {
		entradas = append(entradas, basedatos.EntradaDeVersion{
			Version:  entrada.Version,
			Revision: entrada.Revision,
			Fecha:    entrada.Fecha,
			Tipo:     entrada.Tipo,
			Modulo:   entrada.Modulo,
			Notas:    entrada.Notas,
		})
	}
	return entradas
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
