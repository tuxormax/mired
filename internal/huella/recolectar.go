package huella

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DeUnEquipo junta todo lo que se le puede preguntar a una sola direccion.
//
// Todo va en paralelo y con un plazo corto: son preguntas sueltas, y el aparato
// que no conteste no debe hacer esperar a los demas. Lo que no contesta no se
// inventa: simplemente no aparece.
func DeUnEquipo(ctx context.Context, ip string, puertosAbiertos []int, espera time.Duration) []Dato {
	if espera <= 0 {
		espera = 2 * time.Second
	}

	preguntas := []func() []Dato{
		func() []Dato { return DeLaPagina(ctx, ip, puertosAbiertos, espera) },
		func() []Dato { return DeUbiquiti(ctx, ip, espera) },
		func() []Dato { return DeMikrotik(ctx, ip, espera) },
		func() []Dato { return DeTPLink(ctx, ip, espera) },
		func() []Dato { return DeNetBIOS(ctx, ip, espera) },
		func() []Dato { return DeUnPanelConocido(ctx, ip, puertosAbiertos, espera) },
		func() []Dato { return DeOmada(ctx, ip, espera) },
		func() []Dato { return DeDLink(ctx, ip, espera) },
		func() []Dato { return DeNetgear(ctx, ip, espera) },
		func() []Dato { return DeDahua(ctx, ip, espera) },
		func() []Dato { return DeXiaomi(ctx, ip, espera) },
		func() []Dato { return DeRuckus(ctx, ip, espera) },
		func() []Dato { return DeFortinet(ctx, ip, puertosAbiertos, espera) },
	}

	var candado sync.Mutex
	var grupo sync.WaitGroup
	huella := Huella{IP: ip}

	for _, preguntar := range preguntas {
		grupo.Add(1)
		go func(hacer func() []Dato) {
			defer grupo.Done()
			datos := hacer()
			if len(datos) == 0 {
				return
			}
			candado.Lock()
			defer candado.Unlock()
			for _, dato := range datos {
				huella.Agregar(dato.Fuente, dato.Clave, dato.Valor)
			}
		}(preguntar)
	}
	grupo.Wait()

	huella.Ordenar()
	return huella.Datos
}

// DeUnPanelConocido pregunta a los aparatos que publican su ficha en una
// direccion web fija.
//
// No es adivinar: son puertos que solo usa ese fabricante, y la respuesta trae
// el modelo con todas sus letras.
func DeUnPanelConocido(ctx context.Context, ip string, puertosAbiertos []int, espera time.Duration) []Dato {
	var huella Huella

	// Roku: el puerto 8060 es suyo y contesta una ficha en XML.
	if tienePuerto(puertosAbiertos, 8060) {
		cuerpo := traerTexto(ctx, "http://"+net.JoinHostPort(ip, "8060")+"/query/device-info", espera)
		if cuerpo != "" {
			huella.Agregar(FuenteRoku, ClaveFabricante, "Roku")
			huella.Agregar(FuenteRoku, ClaveModelo, entreEtiquetas(cuerpo, "model-name"))
			huella.Agregar(FuenteRoku, ClaveNombre, entreEtiquetas(cuerpo, "user-device-name"))
			huella.Agregar(FuenteRoku, ClaveSerie, entreEtiquetas(cuerpo, "serial-number"))
			huella.Agregar(FuenteRoku, ClaveFirmware, entreEtiquetas(cuerpo, "software-version"))
		}
	}

	// Televisiones Samsung: el 8001 contesta un JSON con el modelo y el nombre
	// que el dueno le puso al aparato.
	if tienePuerto(puertosAbiertos, 8001) {
		cuerpo := traerTexto(ctx, "http://"+net.JoinHostPort(ip, "8001")+"/api/v2/", espera)
		if strings.Contains(cuerpo, "device") {
			var ficha struct {
				Device struct {
					Nombre     string `json:"name"`
					Modelo     string `json:"modelName"`
					Fabricante string `json:"type"`
					Version    string `json:"firmwareVersion"`
				} `json:"device"`
			}
			if err := json.Unmarshal([]byte(cuerpo), &ficha); err == nil {
				huella.Agregar(FuenteTelevisionLG, ClaveFabricante, "Samsung")
				huella.Agregar(FuenteTelevisionLG, ClaveModelo, ficha.Device.Modelo)
				huella.Agregar(FuenteTelevisionLG, ClaveNombre, ficha.Device.Nombre)
				huella.Agregar(FuenteTelevisionLG, ClaveFirmware, ficha.Device.Version)
			}
		}
	}

	// Impresoras: el 631 es IPP y casi todas sirven ahi una pagina con su
	// modelo en el titulo.
	if tienePuerto(puertosAbiertos, 631) {
		cuerpo := traerTexto(ctx, "http://"+net.JoinHostPort(ip, "631")+"/", espera)
		if partes := buscaTitulo.FindStringSubmatch(cuerpo); len(partes) > 1 {
			huella.Agregar(FuenteImpresora, ClaveModelo, sinEtiquetas(partes[1]))
		}
	}

	return huella.Datos
}

// traerTexto baja una pagina chica. Si no contesta, no pasa nada: ese aparato no
// era de los que hablan ese idioma.
func traerTexto(ctx context.Context, direccion string, espera time.Duration) string {
	cliente := &http.Client{Timeout: espera}
	peticion, err := http.NewRequestWithContext(ctx, http.MethodGet, direccion, nil)
	if err != nil {
		return ""
	}
	peticion.Header.Set("User-Agent", "MiRed (inventario de red)")

	respuesta, err := cliente.Do(peticion)
	if err != nil {
		return ""
	}
	defer respuesta.Body.Close()

	cuerpo, err := io.ReadAll(io.LimitReader(respuesta.Body, topePagina))
	if err != nil {
		return ""
	}
	return string(cuerpo)
}

// Resumen arma una linea corta para mostrar en la lista de equipos: lo que el
// aparato dice ser, en dos o tres palabras.
func Resumen(datos []Dato) string {
	huella := Huella{Datos: datos}
	for _, clave := range []string{ClaveModelo, ClaveTitulo, ClaveNombre, ClaveServidor} {
		if valor := huella.Valor(clave); valor != "" {
			return valor
		}
	}
	return ""
}
