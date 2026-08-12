// Paquete avisos manda las alertas hacia afuera: webhook, ntfy, Telegram o
// correo.
//
// Todo el envio es best-effort y con plazo corto. Que un destino no conteste no
// puede frenar un escaneo ni perder la alerta: la alerta ya quedo guardada en la
// base, y el envio se reintenta en la corrida siguiente.
package avisos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"
)

// espera es el plazo de cada envio. Corto a proposito.
const espera = 10 * time.Second

// Alerta es lo que se avisa.
type Alerta struct {
	Tipo    string
	Momento string
	Titulo  string
	Detalle string
	Red     string
}

// Destino es a donde se avisa.
type Destino struct {
	Nombre  string
	Tipo    string
	Destino string
	Extra   string
}

// AjustesCorreo es lo que hace falta para mandar un correo, guardado como JSON
// en el campo extra del destino.
type AjustesCorreo struct {
	Servidor string `json:"servidor"`
	Puerto   int    `json:"puerto"`
	Usuario  string `json:"usuario"`
	Clave    string `json:"clave"`
	De       string `json:"de"`
}

// Enviar manda una alerta a un destino.
func Enviar(ctx context.Context, destino Destino, alerta Alerta) error {
	switch destino.Tipo {
	case "webhook":
		return enviarWebhook(ctx, destino, alerta)
	case "ntfy":
		return enviarNtfy(ctx, destino, alerta)
	case "telegram":
		return enviarTelegram(ctx, destino, alerta)
	case "correo":
		return enviarCorreo(destino, alerta)
	default:
		return fmt.Errorf("tipo de destino desconocido: %s", destino.Tipo)
	}
}

// enviarWebhook manda el JSON crudo, para que quien lo reciba haga lo que
// quiera con el.
func enviarWebhook(ctx context.Context, destino Destino, alerta Alerta) error {
	cuerpo, err := json.Marshal(map[string]any{
		"tipo":    alerta.Tipo,
		"momento": alerta.Momento,
		"red":     alerta.Red,
		"titulo":  alerta.Titulo,
		"detalle": alerta.Detalle,
		"origen":  "MiRed",
	})
	if err != nil {
		return err
	}

	peticion, err := http.NewRequestWithContext(ctx, http.MethodPost, destino.Destino,
		bytes.NewReader(cuerpo))
	if err != nil {
		return fmt.Errorf("webhook mal configurado: %w", err)
	}
	peticion.Header.Set("Content-Type", "application/json; charset=utf-8")
	return soltar(peticion)
}

// enviarNtfy usa el servicio de notificaciones al telefono. El destino es la URL
// del tema completo (https://ntfy.sh/mi-red-casa).
func enviarNtfy(ctx context.Context, destino Destino, alerta Alerta) error {
	peticion, err := http.NewRequestWithContext(ctx, http.MethodPost, destino.Destino,
		strings.NewReader(alerta.Detalle))
	if err != nil {
		return fmt.Errorf("ntfy mal configurado: %w", err)
	}
	peticion.Header.Set("Title", tituloConRed(alerta))
	peticion.Header.Set("Tags", etiquetaDe(alerta.Tipo))
	if alerta.Tipo == "equipo_nuevo" || alerta.Tipo == "puerto_nuevo" {
		peticion.Header.Set("Priority", "high")
	}
	if destino.Extra != "" {
		peticion.Header.Set("Authorization", "Bearer "+destino.Extra)
	}
	return soltar(peticion)
}

// enviarTelegram habla con la API del bot. El destino es el identificador del
// chat y el extra es el token del bot.
func enviarTelegram(ctx context.Context, destino Destino, alerta Alerta) error {
	if destino.Extra == "" {
		return fmt.Errorf("falta el token del bot de Telegram")
	}

	texto := tituloConRed(alerta)
	if alerta.Detalle != "" {
		texto += "\n" + alerta.Detalle
	}

	valores := url.Values{}
	valores.Set("chat_id", destino.Destino)
	valores.Set("text", texto)

	direccion := "https://api.telegram.org/bot" + destino.Extra + "/sendMessage"
	peticion, err := http.NewRequestWithContext(ctx, http.MethodPost, direccion,
		strings.NewReader(valores.Encode()))
	if err != nil {
		return fmt.Errorf("telegram mal configurado: %w", err)
	}
	peticion.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return soltar(peticion)
}

func enviarCorreo(destino Destino, alerta Alerta) error {
	var ajustes AjustesCorreo
	if err := json.Unmarshal([]byte(destino.Extra), &ajustes); err != nil {
		return fmt.Errorf("los ajustes de correo no se pudieron leer: %w", err)
	}
	if ajustes.Servidor == "" || ajustes.De == "" {
		return fmt.Errorf("faltan el servidor o el remitente en los ajustes de correo")
	}
	if ajustes.Puerto == 0 {
		ajustes.Puerto = 587
	}

	mensaje := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n"+
		"Content-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n\r\n-- \nMiRed\n",
		ajustes.De, destino.Destino, tituloConRed(alerta), alerta.Detalle)

	direccion := fmt.Sprintf("%s:%d", ajustes.Servidor, ajustes.Puerto)
	var autenticacion smtp.Auth
	if ajustes.Usuario != "" {
		autenticacion = smtp.PlainAuth("", ajustes.Usuario, ajustes.Clave, ajustes.Servidor)
	}

	if err := smtp.SendMail(direccion, autenticacion, ajustes.De,
		[]string{destino.Destino}, []byte(mensaje)); err != nil {
		return fmt.Errorf("no se pudo mandar el correo: %w", err)
	}
	return nil
}

func soltar(peticion *http.Request) error {
	cliente := &http.Client{Timeout: espera}
	respuesta, err := cliente.Do(peticion)
	if err != nil {
		return fmt.Errorf("no se pudo avisar: %w", err)
	}
	defer respuesta.Body.Close()

	if respuesta.StatusCode >= 300 {
		return fmt.Errorf("el destino contesto %d", respuesta.StatusCode)
	}
	return nil
}

func tituloConRed(alerta Alerta) string {
	if alerta.Red == "" {
		return alerta.Titulo
	}
	return "[" + alerta.Red + "] " + alerta.Titulo
}

// etiquetaDe elige el icono que ntfy muestra en el telefono.
func etiquetaDe(tipo string) string {
	switch tipo {
	case "equipo_nuevo":
		return "warning"
	case "equipo_ausente":
		return "zzz"
	case "puerto_nuevo":
		return "rotating_light"
	case "cambio_ip":
		return "arrows_counterclockwise"
	default:
		return "satellite"
	}
}
