// Paquete configuracion lee los ajustes del servicio desde /etc/mired/mired.toml
// y deja valores por omision razonables para que MiRed arranque sin configurar
// nada. Cualquier valor se puede sobrescribir con variables de entorno MIRED_*,
// que es lo comodo para desarrollo y para contenedores.
package configuracion

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// RutaPorOmision es donde el paquete .deb deja el archivo de configuracion.
const RutaPorOmision = "/etc/mired/mired.toml"

// RutaDelUsuario es la configuracion propia de quien usa el programa.
//
// Existe porque MiRed tambien se usa como programa de escritorio, y ahi los
// servicios corren como el usuario: no puede editar /etc/mired/mired.toml sin
// ser administrador, y no tendria por que. Si este archivo existe, manda sobre
// el del sistema.
const RutaDelUsuario = ".config/mired/mired.toml"

// Configuracion son todos los ajustes del servicio.
type Configuracion struct {
	Servidor Servidor `toml:"servidor"`
	Datos    Datos    `toml:"datos"`
	Sonda    Sonda    `toml:"sonda"`
	Dpi      Dpi      `toml:"dpi"`
	Flujos   Flujos   `toml:"flujos"`
	Catalogo Catalogo `toml:"catalogo"`
	Registro Registro `toml:"registro"`
}

// Servidor ajusta como escucha y que sirve el binario mired-servidor.
type Servidor struct {
	// Escucha es la direccion y puerto del servicio, por ejemplo ":60072".
	Escucha string `toml:"escucha"`
	// DuracionSesion es cuanto vale una sesion desde el ultimo uso.
	DuracionSesion Duracion `toml:"duracion_sesion"`
}

// Datos ajusta donde viven las bases de datos.
type Datos struct {
	// Ruta es la carpeta base: adentro van mired.db y la carpeta redes/.
	Ruta string `toml:"ruta"`
	// RedesAbiertas es el tope de bases de red abiertas a la vez. Pasado ese
	// numero se cierran las que llevan mas tiempo sin usarse.
	RedesAbiertas int `toml:"redes_abiertas"`
	// InactividadRed es cuanto puede estar sin usarse una base de red antes de
	// cerrarse sola.
	InactividadRed Duracion `toml:"inactividad_red"`
}

// Sonda ajusta el canal entre el servidor y la sonda de escaneo.
type Sonda struct {
	// Socket es el socket Unix por donde la sonda entrega lo que descubre.
	Socket string `toml:"socket"`
}

// Dpi ajusta el canal con mired-dpi, el paquete opcional de inspeccion
// profunda.
//
// Va aparte del resto porque **es opcional**: si el paquete mired-dpi no esta
// instalado, aqui no sobra nada, simplemente nadie contesta en ese socket y el
// servidor no vuelve a preguntar.
type Dpi struct {
	// Socket es por donde mired-dpi entrega lo que vio. Vacio lo apaga.
	Socket string `toml:"socket"`
	// Interfaz es la tarjeta donde llega el puerto espejo. La usa mired-dpi.
	Interfaz string `toml:"interfaz"`
	// CadaMinutos es cada cuanto el servidor recoge lo acumulado.
	CadaMinutos int `toml:"cada_minutos"`
}

// Flujos ajusta el receptor de NetFlow que exporta el router.
type Flujos struct {
	// Escucha es donde se reciben los flujos. Vacio lo apaga.
	Escucha string `toml:"escucha"`
}

// Catalogo ajusta de donde se leen las definiciones de dispositivos.
type Catalogo struct {
	// Carpetas se leen en orden: las ultimas mandan sobre las primeras, para que
	// una definicion propia pueda corregir a una que trae el paquete.
	Carpetas []string `toml:"carpetas"`
}

// Registro ajusta el detalle de la bitacora.
type Registro struct {
	// Nivel es debug, info, aviso o error.
	Nivel string `toml:"nivel"`
}

// Duracion permite escribir "30m" o "24h" en el archivo .toml.
type Duracion struct {
	time.Duration
}

// UnmarshalText interpreta el texto del .toml como duracion de Go.
func (d *Duracion) UnmarshalText(texto []byte) error {
	valor, err := time.ParseDuration(string(texto))
	if err != nil {
		return fmt.Errorf("duracion invalida %q: %w", texto, err)
	}
	d.Duration = valor
	return nil
}

// MarshalText escribe la duracion en el formato que se lee de vuelta.
func (d Duracion) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// PorOmision devuelve la configuracion con la que MiRed arranca si no hay
// archivo. Coincide con las rutas que crea el paquete .deb.
func PorOmision() Configuracion {
	return Configuracion{
		Servidor: Servidor{
			Escucha:        ":60072",
			DuracionSesion: Duracion{12 * time.Hour},
		},
		Datos: Datos{
			Ruta:           "/var/lib/mired",
			RedesAbiertas:  8,
			InactividadRed: Duracion{15 * time.Minute},
		},
		Sonda: Sonda{
			Socket: "/run/mired/sonda.sock",
		},
		Dpi: Dpi{
			Socket:      "/run/mired/dpi.sock",
			CadaMinutos: 5,
		},
		Flujos: Flujos{
			Escucha: ":2055",
		},
		Catalogo: Catalogo{
			Carpetas: []string{
				"/usr/share/mired/dispositivos",
				"/etc/mired/dispositivos",
			},
		},
		Registro: Registro{
			Nivel: "info",
		},
	}
}

// Cargar lee el archivo indicado y aplica encima las variables de entorno. Si el
// archivo no existe no es un error: se usan los valores por omision, que es lo
// que permite arrancar recien instalado.
func Cargar(ruta string) (Configuracion, error) {
	cfg := PorOmision()

	if ruta == "" {
		// La del usuario primero: en el programa de escritorio es la unica que
		// puede editar quien lo usa.
		if casa, err := os.UserHomeDir(); err == nil {
			propia := filepath.Join(casa, RutaDelUsuario)
			if _, err := os.Stat(propia); err == nil {
				ruta = propia
			}
		}
	}
	if ruta == "" {
		ruta = RutaPorOmision
	}
	contenido, err := os.ReadFile(ruta)
	switch {
	case err == nil:
		if err := toml.Unmarshal(contenido, &cfg); err != nil {
			return cfg, fmt.Errorf("configuracion invalida en %s: %w", ruta, err)
		}
	case os.IsNotExist(err):
		// Sin archivo se usan los valores por omision, a proposito.
	default:
		return cfg, fmt.Errorf("no se pudo leer %s: %w", ruta, err)
	}

	aplicarEntorno(&cfg)

	if cfg.Datos.RedesAbiertas < 1 {
		cfg.Datos.RedesAbiertas = 1
	}
	if cfg.Datos.InactividadRed.Duration <= 0 {
		cfg.Datos.InactividadRed = Duracion{15 * time.Minute}
	}
	return cfg, nil
}

// aplicarEntorno deja que las variables MIRED_* manden sobre el archivo.
func aplicarEntorno(cfg *Configuracion) {
	texto := func(clave string, destino *string) {
		if valor, hay := os.LookupEnv(clave); hay && valor != "" {
			*destino = valor
		}
	}
	entero := func(clave string, destino *int) {
		if valor, hay := os.LookupEnv(clave); hay && valor != "" {
			if numero, err := strconv.Atoi(valor); err == nil {
				*destino = numero
			}
		}
	}
	duracion := func(clave string, destino *Duracion) {
		if valor, hay := os.LookupEnv(clave); hay && valor != "" {
			if lapso, err := time.ParseDuration(valor); err == nil {
				destino.Duration = lapso
			}
		}
	}

	texto("MIRED_ESCUCHA", &cfg.Servidor.Escucha)
	duracion("MIRED_DURACION_SESION", &cfg.Servidor.DuracionSesion)
	texto("MIRED_DATOS", &cfg.Datos.Ruta)
	entero("MIRED_REDES_ABIERTAS", &cfg.Datos.RedesAbiertas)
	duracion("MIRED_INACTIVIDAD_RED", &cfg.Datos.InactividadRed)
	texto("MIRED_SOCKET_SONDA", &cfg.Sonda.Socket)
	texto("MIRED_SOCKET_DPI", &cfg.Dpi.Socket)
	texto("MIRED_INTERFAZ_DPI", &cfg.Dpi.Interfaz)
	texto("MIRED_ESCUCHA_FLUJOS", &cfg.Flujos.Escucha)
	if valor, hay := os.LookupEnv("MIRED_DISPOSITIVOS"); hay && valor != "" {
		cfg.Catalogo.Carpetas = strings.Split(valor, ":")
	}
	texto("MIRED_NIVEL_REGISTRO", &cfg.Registro.Nivel)
}

// ArchivoCatalogo es la ruta de la base global: usuarios, permisos y el registro
// de redes.
func (c Configuracion) ArchivoCatalogo() string {
	return filepath.Join(c.Datos.Ruta, "mired.db")
}

// CarpetaRedes es donde vive un archivo .db por cada red.
func (c Configuracion) CarpetaRedes() string {
	return filepath.Join(c.Datos.Ruta, "redes")
}
