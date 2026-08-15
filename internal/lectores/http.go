package lectores

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"time"
)

// Lo que comparten todos los lectores: una sesion HTTP contra el panel de un
// aparato de red.
//
// Dos cosas que se hacen a proposito y conviene tener escritas:
//
//   - **No se verifica el certificado.** El de un aparato de red es de fabrica,
//     autofirmado y con el nombre equivocado SIEMPRE. Verificarlo aqui seria
//     negarse a hablar con el aparato que uno mismo administra.
//   - **No se sigue a cualquier lado.** Solo se habla con la direccion del
//     aparato: un panel que redirija a internet no arrastra la credencial fuera
//     de la red.

// topeRespuesta es cuanto se lee de una pagina. Un panel entero cabe de sobra;
// bajarse un firmware por error, no.
const topeRespuesta = 4 << 20

// nuevaSesion prepara un cliente con galletas, que es como funcionan todos estos
// paneles: se entra una vez y la sesion viaja en la galleta.
func nuevaSesion(espera time.Duration) *http.Client {
	frasco, _ := cookiejar.New(nil)
	return &http.Client{
		Timeout: espera,
		Jar:     frasco,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // #nosec G402
			DisableKeepAlives: true,
		},
	}
}

// comoURL arma la direccion base del panel. Acepta lo que el usuario haya
// guardado: una IP suelta, un host con puerto o una URL completa.
func comoURL(direccion string, seguro bool) string {
	direccion = strings.TrimSpace(strings.TrimRight(direccion, "/"))
	if strings.HasPrefix(direccion, "http://") || strings.HasPrefix(direccion, "https://") {
		return direccion
	}
	if seguro {
		return "https://" + direccion
	}
	return "http://" + direccion
}

// pedir hace una peticion y devuelve el cuerpo.
func pedir(ctx context.Context, cliente *http.Client, metodo, direccion string,
	cuerpo io.Reader, cabeceras map[string]string, usuario, clave string) ([]byte, error) {
	peticion, err := http.NewRequestWithContext(ctx, metodo, direccion, cuerpo)
	if err != nil {
		return nil, err
	}
	peticion.Header.Set("User-Agent", "MiRed (inventario de red)")
	for nombre, valor := range cabeceras {
		peticion.Header.Set(nombre, valor)
	}
	if usuario != "" {
		peticion.SetBasicAuth(usuario, clave)
	}

	respuesta, err := cliente.Do(peticion)
	if err != nil {
		return nil, err
	}
	defer respuesta.Body.Close()

	leido, err := io.ReadAll(io.LimitReader(respuesta.Body, topeRespuesta))
	if err != nil {
		return nil, err
	}
	if respuesta.StatusCode >= 400 {
		return leido, &ErrorDePanel{Codigo: respuesta.StatusCode, Estado: respuesta.Status}
	}
	return leido, nil
}

// ErrorDePanel es lo que contesto el aparato cuando no dejo pasar.
//
// Se distingue de un fallo de red a proposito: un 401 significa que la
// credencial guardada ya no sirve, y eso se corrige cambiandola, no esperando.
type ErrorDePanel struct {
	Codigo int
	Estado string
}

func (e *ErrorDePanel) Error() string {
	if e.Codigo == 401 || e.Codigo == 403 {
		return "el aparato rechazo la credencial guardada (" + e.Estado + ")"
	}
	return "el panel del aparato contesto " + e.Estado
}

// EsCredencialRechazada dice si el fallo fue por la credencial.
func EsCredencialRechazada(err error) bool {
	var fallo *ErrorDePanel
	if ok := comoErrorDePanel(err, &fallo); ok {
		return fallo.Codigo == 401 || fallo.Codigo == 403
	}
	return false
}

func comoErrorDePanel(err error, destino **ErrorDePanel) bool {
	for err != nil {
		if fallo, ok := err.(*ErrorDePanel); ok {
			*destino = fallo
			return true
		}
		desenvolver, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = desenvolver.Unwrap()
	}
	return false
}

// buscaMAC encuentra direcciones fisicas separadas por dos puntos o por guion.
//
// Van como dos alternativas completas y no con una referencia al separador
// porque el motor de expresiones de Go no las tiene: es a proposito, para que
// una expresion no pueda tardar un tiempo absurdo. Aqui no hace falta.
var buscaMAC = regexp.MustCompile(
	`(?i)\b([0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}` +
		`|[0-9a-f]{2}-[0-9a-f]{2}-[0-9a-f]{2}-[0-9a-f]{2}-[0-9a-f]{2}-[0-9a-f]{2})\b`)

// macsDe saca todas las direcciones fisicas de un texto, en orden de aparicion.
//
// Es la base del lector generico: en el panel de casi cualquier aparato, la
// tabla de conectados es lo unico que tiene MAC dentro.
func macsDe(texto string) []string {
	var macs []string
	vistas := map[string]bool{}
	for _, coincidencia := range buscaMAC.FindAllStringSubmatch(texto, -1) {
		mac := NormalizarMAC(coincidencia[1])
		if mac == "" || vistas[mac] {
			continue
		}
		vistas[mac] = true
		macs = append(macs, mac)
	}
	return macs
}
