// Paquete secreto cifra lo que no puede quedar en claro en la base.
//
// Hoy lo usa una sola cosa: las claves de los equipos que el usuario guarda en
// su ficha. Y esta escrito con una idea muy concreta de contra QUE protege,
// porque un cifrado del que no se sabe contra que sirve es peor que ninguno:
// da tranquilidad falsa.
//
// # Contra que protege
//
// **Contra el respaldo.** La carpeta de datos de MiRed es "lo unico que hay que
// respaldar", y esos respaldos acaban en discos externos, en la nube y en
// correos. La llave del cifrado NO vive con los datos: vive con la
// configuracion. Un respaldo de la base, copiado a cualquier lado, no trae con
// que descifrarse.
//
// # Contra que NO protege
//
// **Contra alguien sentado en la maquina con el usuario que corre MiRed.** Ese
// puede leer la llave y la base. No hay forma de evitarlo sin pedirle al usuario
// que teclee una clave maestra cada vez que abre el programa, y esa decision no
// se toma por el: si algun dia se quiere, se agrega encima de esto.
package secreto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ErrSinLlave la devuelve Descifrar cuando el texto esta cifrado con otra llave
// —o la llave se perdio—, para poder decirlo con palabras en vez de devolver
// basura.
var ErrSinLlave = errors.New("no se pudo descifrar: la llave no es la que cifro esto")

// marca distingue lo cifrado por MiRed de un texto cualquiera. Sin esto, una
// base migrada de una version vieja se leeria como si estuviera cifrada.
const marca = "mired1:"

// Caja guarda y saca secretos con una llave que vive en un archivo.
type Caja struct {
	rutaLlave string

	unaVez sync.Once
	sello  cipher.AEAD
	fallo  error
}

// NuevaCaja prepara el cifrado con la llave de esa ruta.
//
// La llave se crea sola la primera vez, con permisos 0600. La ruta debe estar en
// la carpeta de CONFIGURACION, nunca en la de datos: si viaja en el mismo
// respaldo que la base, el cifrado no protege de nada.
func NuevaCaja(rutaLlave string) *Caja {
	return &Caja{rutaLlave: rutaLlave}
}

func (c *Caja) preparar() {
	c.unaVez.Do(func() {
		llave, err := c.leerOCrearLlave()
		if err != nil {
			c.fallo = err
			return
		}
		bloque, err := aes.NewCipher(llave)
		if err != nil {
			c.fallo = err
			return
		}
		c.sello, c.fallo = cipher.NewGCM(bloque)
	})
}

func (c *Caja) leerOCrearLlave() ([]byte, error) {
	crudo, err := os.ReadFile(c.rutaLlave)
	if err == nil {
		llave, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(crudo)))
		if err != nil || len(llave) != 32 {
			return nil, fmt.Errorf("la llave de %s no sirve: %w", c.rutaLlave, err)
		}
		return llave, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("no se pudo leer la llave %s: %w", c.rutaLlave, err)
	}

	llave := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, llave); err != nil {
		return nil, fmt.Errorf("no se pudo generar la llave: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(c.rutaLlave), 0o700); err != nil {
		return nil, fmt.Errorf("no se pudo crear la carpeta de la llave: %w", err)
	}
	// 0600: solo el usuario que corre MiRed. Si esto queda legible para todos,
	// el cifrado no vale nada.
	if err := os.WriteFile(c.rutaLlave,
		[]byte(base64.StdEncoding.EncodeToString(llave)), 0o600); err != nil {
		return nil, fmt.Errorf("no se pudo escribir la llave: %w", err)
	}
	return llave, nil
}

// Cifrar guarda un secreto. Un texto vacio se queda vacio: no hay nada que
// esconder y guardar relleno solo confundiria a quien mire la base.
func (c *Caja) Cifrar(claro string) (string, error) {
	if claro == "" {
		return "", nil
	}
	c.preparar()
	if c.fallo != nil {
		return "", c.fallo
	}

	nonce := make([]byte, c.sello.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sellado := c.sello.Seal(nonce, nonce, []byte(claro), nil)
	return marca + base64.StdEncoding.EncodeToString(sellado), nil
}

// Descifrar saca un secreto guardado.
//
// Lo que no lleva la marca se devuelve tal cual: una base escrita antes de que
// existiera el cifrado tiene textos en claro, y romperse ahi seria dejar
// inservible lo que el usuario ya habia guardado.
func (c *Caja) Descifrar(guardado string) (string, error) {
	if guardado == "" {
		return "", nil
	}
	if !strings.HasPrefix(guardado, marca) {
		return guardado, nil
	}
	c.preparar()
	if c.fallo != nil {
		return "", c.fallo
	}

	sellado, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(guardado, marca))
	if err != nil {
		return "", ErrSinLlave
	}
	if len(sellado) < c.sello.NonceSize() {
		return "", ErrSinLlave
	}
	nonce, cuerpo := sellado[:c.sello.NonceSize()], sellado[c.sello.NonceSize():]

	claro, err := c.sello.Open(nil, nonce, cuerpo, nil)
	if err != nil {
		return "", ErrSinLlave
	}
	return string(claro), nil
}

// Listo dice si la caja puede trabajar. La interfaz lo usa para avisar en vez de
// dejar que cada intento de guardar falle por separado.
func (c *Caja) Listo() error {
	c.preparar()
	return c.fallo
}
