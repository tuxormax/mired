package catalogo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// El catalogo comunitario: lo que una persona aprende de su red, aprovechable
// por las demas.
//
// La idea es simple y es la razon de que las definiciones sean archivos y no
// codigo: quien tiene delante un aparato que MiRed no reconoce sabe lo que es.
// Si eso se queda en su equipo, cada instalacion vuelve a empezar de cero.
//
// El camino tiene tres pasos, y ninguno pide dar de alta una cuenta ni escribir
// Go:
//
//  1. **Guardar en el catalogo de aqui** — el aparato queda reconocido en esta
//     instalacion desde el proximo escaneo.
//  2. **Compartirlo** — se abre en el navegador un aporte ya escrito contra el
//     repositorio del proyecto. La persona solo revisa y publica.
//  3. **Traer lo de los demas** — se bajan las definiciones publicadas y se
//     guardan aparte, sin tocar ni las del paquete ni las propias.
//
// **Las tres carpetas mandan en orden**: paquete < comunidad < propias. Una
// definicion de la comunidad puede corregir a la del paquete, y la propia gana
// siempre: quien tiene el aparato delante sabe mas que nadie.

const (
	// CarpetaComunidad es donde se guarda lo que se baja. Va en /var/lib y no en
	// /etc porque no es configuracion de nadie: es una copia que se puede tirar
	// y volver a bajar.
	CarpetaComunidad = "/var/lib/mired/dispositivos-comunidad"

	// CarpetaPropia es donde se guarda lo que se declara aqui.
	CarpetaPropia = "/etc/mired/dispositivos"

	// Repositorio es de donde se bajan y a donde se aportan las definiciones.
	Repositorio = "tuxormax/mired"

	// rutaEnElRepo es la carpeta del repositorio donde viven las definiciones.
	rutaEnElRepo = "catalogo/dispositivos"
)

// nombreValido limita como se puede llamar un archivo de definicion. Sin esto,
// un nombre con ".." o con "/" escribiria donde no debe.
var nombreValido = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// Guardado dice que quedo escrito y donde.
type Guardado struct {
	Archivo string `json:"archivo"`
	Ruta    string `json:"ruta"`
}

// Guardar escribe una definicion en una carpeta, despues de comprobar que sirve.
//
// Se valida ANTES de escribir: un `.toml` roto no tumba el catalogo —se anota
// como problema y se sigue— pero deja al usuario creyendo que aporto algo que no
// reconoce nada. Es mejor decirselo en la cara.
func Guardar(carpeta, archivo, contenido string) (Guardado, error) {
	archivo = strings.ToLower(strings.TrimSpace(archivo))
	if !strings.HasSuffix(archivo, ".toml") {
		archivo += ".toml"
	}
	if !nombreValido.MatchString(archivo) {
		return Guardado{}, fmt.Errorf(
			"el nombre del archivo solo puede llevar letras, numeros, punto, guion y guion bajo: %q", archivo)
	}

	var definicion Definicion
	if err := toml.Unmarshal([]byte(contenido), &definicion); err != nil {
		return Guardado{}, fmt.Errorf("la definicion tiene un error de formato: %w", err)
	}
	if strings.TrimSpace(definicion.Nombre) == "" {
		return Guardado{}, fmt.Errorf("la definicion no dice como se llama el aparato")
	}
	if !tieneCondiciones(definicion.Coincidencias) {
		// Una definicion sin condiciones coincidiria con TODO y le pondria el
		// mismo nombre a media red. Es el error mas facil de cometer al escribir
		// la primera, y el mas caro de descubrir despues.
		return Guardado{}, fmt.Errorf(
			"la definicion no tiene ninguna condicion: reconoceria a todos los aparatos por igual")
	}

	if err := os.MkdirAll(carpeta, 0o755); err != nil {
		return Guardado{}, fmt.Errorf("no se pudo crear %s: %w", carpeta, err)
	}
	ruta := filepath.Join(carpeta, archivo)
	if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
		return Guardado{}, fmt.Errorf("no se pudo escribir %s: %w", ruta, err)
	}
	return Guardado{Archivo: archivo, Ruta: ruta}, nil
}

// tieneCondiciones dice si la definicion pide algo para coincidir.
func tieneCondiciones(c Coincidencias) bool {
	return len(c.Fabricantes) > 0 || len(c.PuertosTodos) > 0 || len(c.PuertosAlguno) > 0 ||
		len(c.BannerContiene) > 0 || len(c.NombreContiene) > 0 || len(c.SnmpContiene) > 0 ||
		len(c.HuellaContiene) > 0 || len(c.ModeloContiene) > 0 || len(c.PrefijosMac) > 0
}

// Traidas cuenta lo que dejo una actualizacion desde la comunidad.
type Traidas struct {
	Bajadas int      `json:"bajadas"`
	Nuevas  int      `json:"nuevas"`
	Fecha   string   `json:"fecha"`
	Fallos  []string `json:"fallos"`
}

// entradaGitHub es lo que contesta GitHub al listar una carpeta.
type entradaGitHub struct {
	Nombre   string `json:"name"`
	Tipo     string `json:"type"`
	Descarga string `json:"download_url"`
}

// Actualizar baja las definiciones publicadas y las deja en la carpeta de la
// comunidad.
//
// No toca las del paquete ni las propias: se guardan aparte a proposito, para
// que actualizar nunca pise una correccion que alguien hizo aqui.
func Actualizar(ctx context.Context, carpeta string) (Traidas, error) {
	if carpeta == "" {
		carpeta = CarpetaComunidad
	}
	resultado := Traidas{Fallos: []string{}}

	cliente := &http.Client{Timeout: 30 * time.Second}
	listado := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", Repositorio, rutaEnElRepo)

	peticion, err := http.NewRequestWithContext(ctx, http.MethodGet, listado, nil)
	if err != nil {
		return resultado, err
	}
	peticion.Header.Set("Accept", "application/vnd.github+json")
	peticion.Header.Set("User-Agent", "MiRed")

	respuesta, err := cliente.Do(peticion)
	if err != nil {
		return resultado, fmt.Errorf("no se pudo hablar con GitHub: %w", err)
	}
	defer respuesta.Body.Close()

	if respuesta.StatusCode != http.StatusOK {
		return resultado, fmt.Errorf("GitHub contesto %s al listar las definiciones", respuesta.Status)
	}

	var entradas []entradaGitHub
	if err := json.NewDecoder(io.LimitReader(respuesta.Body, 1<<20)).Decode(&entradas); err != nil {
		return resultado, fmt.Errorf("no se entendio la respuesta de GitHub: %w", err)
	}

	if err := os.MkdirAll(carpeta, 0o755); err != nil {
		return resultado, fmt.Errorf("no se pudo crear %s: %w", carpeta, err)
	}

	for _, entrada := range entradas {
		if entrada.Tipo != "file" || !strings.HasSuffix(entrada.Nombre, ".toml") {
			continue
		}
		if !nombreValido.MatchString(strings.ToLower(entrada.Nombre)) {
			resultado.Fallos = append(resultado.Fallos, "nombre raro, se omitio: "+entrada.Nombre)
			continue
		}

		contenido, err := bajar(ctx, cliente, entrada.Descarga)
		if err != nil {
			resultado.Fallos = append(resultado.Fallos, entrada.Nombre+": "+err.Error())
			continue
		}
		// Se valida antes de guardar: una definicion rota bajada de internet no
		// tiene por que ensuciar el catalogo de nadie.
		var definicion Definicion
		if err := toml.Unmarshal(contenido, &definicion); err != nil || definicion.Nombre == "" {
			resultado.Fallos = append(resultado.Fallos, entrada.Nombre+": no se entendio el archivo")
			continue
		}

		ruta := filepath.Join(carpeta, strings.ToLower(entrada.Nombre))
		_, yaEstaba := os.Stat(ruta)
		if err := os.WriteFile(ruta, contenido, 0o644); err != nil {
			resultado.Fallos = append(resultado.Fallos, entrada.Nombre+": "+err.Error())
			continue
		}
		resultado.Bajadas++
		if yaEstaba != nil {
			resultado.Nuevas++
		}
	}

	sort.Strings(resultado.Fallos)
	return resultado, nil
}

func bajar(ctx context.Context, cliente *http.Client, direccion string) ([]byte, error) {
	if direccion == "" {
		return nil, fmt.Errorf("la definicion no trae de donde bajarse")
	}
	peticion, err := http.NewRequestWithContext(ctx, http.MethodGet, direccion, nil)
	if err != nil {
		return nil, err
	}
	peticion.Header.Set("User-Agent", "MiRed")

	respuesta, err := cliente.Do(peticion)
	if err != nil {
		return nil, err
	}
	defer respuesta.Body.Close()
	if respuesta.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("contesto %s", respuesta.Status)
	}
	return io.ReadAll(io.LimitReader(respuesta.Body, 256<<10))
}

// URLParaAportar arma la direccion de un aporte ya escrito contra el repositorio.
//
// Se hace asi, y no mandando la definicion desde el servicio, por una razon de
// fondo: aportar es un acto de una PERSONA, con su cuenta y su nombre. Un
// servicio que publicara por su cuenta necesitaria guardar una credencial del
// proyecto en cada instalacion, y cualquiera podria llenar el catalogo de basura
// sin que se sepa quien fue.
func URLParaAportar(nombreDelAparato, archivo, contenido string) string {
	titulo := "Definicion nueva: " + strings.TrimSpace(nombreDelAparato)

	cuerpo := strings.Builder{}
	cuerpo.WriteString("Esta definicion la propuso MiRed a partir de un aparato real.\n\n")
	cuerpo.WriteString("Archivo sugerido: `" + archivo + "`\n\n")
	cuerpo.WriteString("```toml\n")
	cuerpo.WriteString(contenido)
	if !strings.HasSuffix(contenido, "\n") {
		cuerpo.WriteString("\n")
	}
	cuerpo.WriteString("```\n")

	valores := url.Values{}
	valores.Set("title", titulo)
	valores.Set("body", cuerpo.String())
	valores.Set("labels", "catalogo")

	return fmt.Sprintf("https://github.com/%s/issues/new?%s", Repositorio, valores.Encode())
}
