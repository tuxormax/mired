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

	// Llave es donde se guarda la llave del cifrado de secretos. Vacio deja que
	// la calcule ArchivoLlave, al lado del archivo de configuracion.
	Llave string `toml:"llave_secretos"`

	// Archivo es de donde se leyo esta configuracion. No sale del .toml: lo
	// rellena Cargar, y sirve para saber donde poner la llave.
	Archivo string `toml:"-"`
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
	// Propia es donde se ESCRIBE lo que se declara en esta instalacion.
	//
	// Va aparte de la lista porque escribir y leer no son lo mismo: corriendo
	// como programa de escritorio, MiRed lee lo que instalo el paquete en
	// /usr/share pero no puede escribir ahi ni en /etc, y sus definiciones
	// tienen que caer en la carpeta del usuario.
	Propia string `toml:"carpeta_propia"`
	// Comunidad es donde se guarda lo que se baja del repositorio. Se guarda
	// aparte para que actualizar nunca pise una definicion escrita aqui.
	Comunidad string `toml:"carpeta_comunidad"`
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
			// En orden, de menos a mas mandona: lo que trae el paquete, lo que
			// se bajo de la comunidad, y lo que se escribio aqui. Quien tiene el
			// aparato delante sabe mas que el repositorio.
			Carpetas: []string{
				"/usr/share/mired/dispositivos",
				"/var/lib/mired/dispositivos-comunidad",
				"/etc/mired/dispositivos",
			},
			Propia:    "/etc/mired/dispositivos",
			Comunidad: "/var/lib/mired/dispositivos-comunidad",
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

	// De donde salio, para poder poner la llave de los secretos a su lado.
	cfg.Archivo = ruta

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
	texto("MIRED_DISPOSITIVOS_PROPIOS", &cfg.Catalogo.Propia)
	texto("MIRED_DISPOSITIVOS_COMUNIDAD", &cfg.Catalogo.Comunidad)
	texto("MIRED_NIVEL_REGISTRO", &cfg.Registro.Nivel)
	texto("MIRED_LLAVE_SECRETOS", &cfg.Llave)
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

// ArchivoLlave es donde vive la llave con la que se cifran las claves de los
// equipos.
//
// **Va con la CONFIGURACION, no con los datos.** Esa es toda la gracia: la
// carpeta de datos es lo unico que hay que respaldar, y esos respaldos se copian
// a discos, a la nube y a correos. Si la llave viajara ahi dentro, cifrar no
// protegeria de nada.
func (c Configuracion) ArchivoLlave() string {
	if c.Llave != "" {
		return c.Llave
	}
	if c.Archivo != "" {
		return filepath.Join(filepath.Dir(c.Archivo), "llave-secretos")
	}
	return "/etc/mired/llave-secretos"
}
