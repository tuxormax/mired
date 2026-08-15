package huella

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// puertosWeb son los que se prueban buscando una pagina, en orden de preferencia.
var puertosWeb = []int{80, 8080, 8000, 8081, 443, 8443, 10443}

// puertosSeguros dice cuales de esos hablan TLS.
var puertosSeguros = map[int]bool{443: true, 8443: true, 8843: true, 10443: true}

// topePagina es cuanto se lee de la pagina. El titulo esta siempre en las
// primeras lineas; bajarse el panel entero de un router seria maleducado y no
// aportaria nada.
const topePagina = 64 << 10

var (
	buscaTitulo = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	buscaRealm  = regexp.MustCompile(`(?i)realm="([^"]+)"`)
	// Muchos paneles no tienen <title> y se presentan en un <meta> o en el
	// nombre del formulario. Es el caso de varios modems del proveedor.
	buscaGenerador = regexp.MustCompile(`(?is)<meta[^>]+name=["']?(?:generator|application-name)["']?[^>]+content=["']([^"']+)["']`)
)

// DeLaPagina pregunta por la pagina del aparato y se queda con lo que dice de
// si mismo: el titulo, quien dice ser el servidor y el nombre del candado.
//
// Es la senal mas rentable que hay despues del fabricante: ahi el UniFi dice
// UniFi, el AirOS dice AirOS, el modem dice su modelo y la camara dice su marca.
func DeLaPagina(ctx context.Context, ip string, puertosAbiertos []int, espera time.Duration) []Dato {
	var huella Huella

	for _, puerto := range puertosWeb {
		if !tienePuerto(puertosAbiertos, puerto) {
			continue
		}
		seguro := puertosSeguros[puerto]
		if seguro {
			for _, dato := range delCertificado(ctx, ip, puerto, espera) {
				huella.Datos = append(huella.Datos, dato)
			}
		}
		leerPagina(ctx, &huella, ip, puerto, seguro, espera)

		// Con una pagina que conteste algo basta: las demas son el mismo panel
		// por otro puerto, y cada intento cuesta tiempo en toda la red.
		if huella.Valor(ClaveTitulo) != "" || huella.Valor(ClaveServidor) != "" {
			break
		}
	}
	return huella.Datos
}

func leerPagina(ctx context.Context, huella *Huella, ip string, puerto int, seguro bool, espera time.Duration) {
	esquema := "http"
	if seguro {
		esquema = "https"
	}
	direccion := esquema + "://" + net.JoinHostPort(ip, strconv.Itoa(puerto)) + "/"

	cliente := &http.Client{
		Timeout: espera,
		Transport: &http.Transport{
			// El certificado de un aparato de red SIEMPRE es invalido: se lo
			// firma el mismo y lleva el nombre de fabrica. Verificarlo aqui
			// seria negarse a leer justo lo que se viene a leer. No se manda
			// ninguna credencial por esta conexion.
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, // #nosec G402
			DisableKeepAlives:   true,
			TLSHandshakeTimeout: espera,
		},
		// Las redirecciones se siguen hasta dos veces: casi todo panel manda de
		// http a https o a /login, y ahi es donde esta el titulo bueno.
		CheckRedirect: func(peticion *http.Request, anteriores []*http.Request) error {
			if len(anteriores) >= 2 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	peticion, err := http.NewRequestWithContext(ctx, http.MethodGet, direccion, nil)
	if err != nil {
		return
	}
	peticion.Header.Set("User-Agent", "MiRed (inventario de red)")
	peticion.Header.Set("Accept", "text/html,*/*")

	respuesta, err := cliente.Do(peticion)
	if err != nil {
		return
	}
	defer respuesta.Body.Close()

	huella.Agregar(FuenteWeb, ClaveServidor, respuesta.Header.Get("Server"))
	if candado := respuesta.Header.Get("WWW-Authenticate"); candado != "" {
		if partes := buscaRealm.FindStringSubmatch(candado); len(partes) > 1 {
			huella.Agregar(FuenteWeb, ClaveRealm, partes[1])
		} else {
			huella.Agregar(FuenteWeb, ClaveRealm, candado)
		}
	}
	// Algunos fabricantes se anuncian en encabezados propios. Es un regalo:
	// dicen la marca sin que haya que interpretar nada.
	for _, nombre := range []string{"X-Frame-Options-Vendor", "X-Vendor", "X-Device-Model", "X-Model"} {
		huella.Agregar(FuenteWeb, ClaveModelo, respuesta.Header.Get(nombre))
	}

	cuerpo, err := io.ReadAll(io.LimitReader(respuesta.Body, topePagina))
	if err != nil {
		return
	}
	pagina := string(cuerpo)

	if partes := buscaTitulo.FindStringSubmatch(pagina); len(partes) > 1 {
		huella.Agregar(FuenteWeb, ClaveTitulo, sinEtiquetas(partes[1]))
	}
	if partes := buscaGenerador.FindStringSubmatch(pagina); len(partes) > 1 {
		huella.Agregar(FuenteWeb, ClaveModelo, partes[1])
	}
}

// delCertificado saca lo que el aparato firmo con su nombre.
//
// En un certificado de fabrica el nombre comun y la organizacion suelen traer la
// marca y hasta el modelo —"UBNT", "Hikvision", "TP-LINK"—, y eso no cambia
// aunque alguien le ponga clave al panel.
func delCertificado(ctx context.Context, ip string, puerto int, espera time.Duration) []Dato {
	var huella Huella

	marcador := &net.Dialer{Timeout: espera}
	conexion, err := tls.DialWithDialer(marcador, "tcp",
		net.JoinHostPort(ip, strconv.Itoa(puerto)),
		&tls.Config{InsecureSkipVerify: true}) // #nosec G402 — es un aparato, no un banco
	if err != nil {
		return nil
	}
	defer conexion.Close()

	if err := conexion.HandshakeContext(ctx); err != nil {
		return nil
	}
	certificados := conexion.ConnectionState().PeerCertificates
	if len(certificados) == 0 {
		return nil
	}

	certificado := certificados[0]
	huella.Agregar(FuenteCertificado, ClaveNombre, certificado.Subject.CommonName)
	if len(certificado.Subject.Organization) > 0 {
		huella.Agregar(FuenteCertificado, ClaveFabricante, certificado.Subject.Organization[0])
	}
	if len(certificado.Subject.OrganizationalUnit) > 0 {
		huella.Agregar(FuenteCertificado, ClaveModelo, certificado.Subject.OrganizationalUnit[0])
	}
	return huella.Datos
}

func tienePuerto(puertos []int, buscado int) bool {
	for _, puerto := range puertos {
		if puerto == buscado {
			return true
		}
	}
	return false
}

func sinEtiquetas(texto string) string {
	for {
		abre := strings.Index(texto, "<")
		if abre < 0 {
			break
		}
		cierra := strings.Index(texto[abre:], ">")
		if cierra < 0 {
			break
		}
		texto = texto[:abre] + texto[abre+cierra+1:]
	}
	return texto
}
