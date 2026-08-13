package autenticacion

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/bits"
	"strings"

	"golang.org/x/crypto/scrypt"
)

// TUXOR v2.0 — algoritmo de autenticacion de doble hash basado en operadores.
//
// Es el algoritmo de la casa (Bernardo Sanchez Gutierrez, GPL-3.0). Esta es la
// cuarta implementacion: ya existen en PHP, JavaScript y Python, y **las cuatro
// tienen que dar exactamente el mismo resultado**. Por eso el vector de prueba
// oficial esta en las pruebas de este paquete: si algun dia esto deja de
// coincidir con las otras, las claves guardadas por una no las verifica la otra.
//
// # Como funciona
//
//  1. PARSE   — se sacan los operadores del principio y del final de cada texto.
//  2. HASH    — SHA-256 de cada texto ya limpio.
//  3. SPLIT   — cada hash se parte en 4 bloques de 16 caracteres (64 bits).
//  4. OPERATE — a cada par de bloques se le aplica un operador, ciclicamente.
//  5. COMBINE — SHA-256 de todo lo anterior concatenado.
//
// # Modo seguro, que es el que se usa aqui
//
// El TUXOR a secas es rapido, y lo rapido es malo para guardar contrasenas:
// quien se lleve la base puede probar millones por segundo. Por eso se guarda
// `scrypt(tuxor, sal)`: el TUXOR pone la capa de confusion y scrypt la de
// resistencia, que es la que cuesta dinero romper.

// Operadores validos. El orden importa solo para leerlos; el algoritmo los toma
// como los encuentre.
const operadoresValidos = "+-*%^&|<>#"

const (
	tamanoBloque   = 16 // caracteres hexadecimales, o sea 64 bits
	cuantosBloques = 4
	// CostoTuxor es el exponente de scrypt (2^costo). 14 es lo que usan las
	// otras implementaciones de la casa; subirlo obligaria a rehacer las claves.
	CostoTuxor = 14
)

// desmenuzado es lo que queda de un texto tras separarle los operadores.
type desmenuzado struct {
	prefijo    []string
	sufijo     []string
	operadores []string
	limpio     string
}

// desmenuzar separa los operadores del principio y del final, y decide que parte
// del texto entra al hash.
//
// El modificador `@` es lo que controla eso, y tiene cuatro modos:
//
//	@@ al principio o al final → los operadores TAMBIEN entran al hash
//	@  al principio            → entran solo los del principio
//	@  al final                → entran solo los del final
//	sin @                      → no entra ninguno
func desmenuzar(texto string) desmenuzado {
	letras := []rune(texto)
	incluir := "ninguno"

	switch {
	case len(letras) >= 2 && letras[0] == '@' && letras[1] == '@':
		incluir = "todos"
		letras = letras[2:]
	case len(letras) >= 2 && letras[len(letras)-1] == '@' && letras[len(letras)-2] == '@':
		incluir = "todos"
		letras = letras[:len(letras)-2]
	case len(letras) >= 1 && letras[0] == '@':
		incluir = "prefijo"
		letras = letras[1:]
	case len(letras) >= 1 && letras[len(letras)-1] == '@':
		incluir = "sufijo"
		letras = letras[:len(letras)-1]
	}

	inicio, fin := 0, len(letras)-1
	var prefijo, sufijo []string

	for inicio <= fin && strings.ContainsRune(operadoresValidos, letras[inicio]) {
		prefijo = append(prefijo, string(letras[inicio]))
		inicio++
	}
	for fin >= inicio && strings.ContainsRune(operadoresValidos, letras[fin]) {
		sufijo = append(sufijo, string(letras[fin]))
		fin--
	}
	// El sufijo se recogio de derecha a izquierda: hay que darle la vuelta o el
	// orden de los operadores no coincidiria con las otras implementaciones.
	for i, j := 0, len(sufijo)-1; i < j; i, j = i+1, j-1 {
		sufijo[i], sufijo[j] = sufijo[j], sufijo[i]
	}

	medio := string(letras[inicio : fin+1])

	var limpio string
	switch incluir {
	case "todos":
		limpio = string(letras)
	case "prefijo":
		limpio = strings.Join(prefijo, "") + medio
	case "sufijo":
		limpio = medio + strings.Join(sufijo, "")
	default:
		limpio = medio
	}

	return desmenuzado{
		prefijo:    prefijo,
		sufijo:     sufijo,
		operadores: append(append([]string{}, prefijo...), sufijo...),
		limpio:     limpio,
	}
}

// OperadoresParaLeer devuelve los operadores separados, para poder ponerlos en
// un mensaje sin que se lean como un simbolo raro de diez caracteres.
func OperadoresParaLeer() string {
	return strings.Join(strings.Split(operadoresValidos, ""), " ")
}

// TuxorValido dice si un texto sirve como identidad o secreto de TUXOR.
//
// Se expone para que el formulario pueda avisar ANTES de enviar: el algoritmo
// exige que al menos uno de los dos textos traiga un operador, y descubrir eso
// despues de teclear una clave larga es de las cosas que hacen abandonar un alta.
func TuxorValido(texto string) bool {
	partes := desmenuzar(texto)
	return len(partes.operadores) > 0 && partes.limpio != ""
}

// tuxorCrudo calcula el TUXOR de una identidad y un secreto.
func tuxorCrudo(identidad, secreto string) (string, error) {
	id := desmenuzar(identidad)
	sec := desmenuzar(secreto)

	if len(id.operadores) == 0 && len(sec.operadores) == 0 {
		return "", fmt.Errorf("el usuario o la clave deben llevar al menos un operador (%s)",
			operadoresValidos)
	}
	if id.limpio == "" || sec.limpio == "" {
		return "", fmt.Errorf("el usuario y la clave deben tener al menos una letra ademas de los operadores")
	}

	operadores := append(append(append(append([]string{},
		id.prefijo...), id.sufijo...), sec.prefijo...), sec.sufijo...)

	hashI := sha256Hex(id.limpio)
	hashS := sha256Hex(sec.limpio)

	var resultados strings.Builder
	for n := 0; n < cuantosBloques; n++ {
		bloqueI := hashI[n*tamanoBloque : (n+1)*tamanoBloque]
		bloqueS := hashS[n*tamanoBloque : (n+1)*tamanoBloque]

		valorI, ok1 := new(big.Int).SetString(bloqueI, 16)
		valorS, ok2 := new(big.Int).SetString(bloqueS, 16)
		if !ok1 || !ok2 {
			return "", fmt.Errorf("no se pudo interpretar el bloque %d", n)
		}

		operado, err := aplicarOperador(operadores[n%len(operadores)],
			valorI.Uint64(), valorS.Uint64(), bloqueI, bloqueS)
		if err != nil {
			return "", err
		}
		resultados.WriteString(fmt.Sprintf("%016x", operado))
	}

	return sha256Hex(resultados.String()), nil
}

// aplicarOperador es el corazon del algoritmo: cada operador combina los dos
// bloques de otra forma, siempre en 64 bits.
func aplicarOperador(operador string, valorI, valorS uint64, hexI, hexS string) (uint64, error) {
	switch operador {
	case "+":
		return valorI + valorS, nil // el desbordamiento natural de uint64 ES el modulo
	case "-":
		return valorI - valorS, nil
	case "*":
		return valorI * valorS, nil
	case "%":
		if valorS == 0 {
			return 0, nil // I % 1 siempre es 0
		}
		return valorI % valorS, nil
	case "^":
		return valorI ^ valorS, nil
	case "&":
		return valorI & valorS, nil
	case "|":
		return valorI | valorS, nil
	case "<":
		return bits.RotateLeft64(valorI, int(valorS%64)), nil
	case ">":
		return bits.RotateLeft64(valorI, -int(valorS%64)), nil
	case "#":
		rehecho := sha256Hex(hexI + hexS)
		valor, ok := new(big.Int).SetString(rehecho[:tamanoBloque], 16)
		if !ok {
			return 0, fmt.Errorf("no se pudo rehashear el bloque")
		}
		return valor.Uint64(), nil
	default:
		return 0, fmt.Errorf("operador invalido: %s", operador)
	}
}

func sha256Hex(mensaje string) string {
	suma := sha256.Sum256([]byte(mensaje))
	return hex.EncodeToString(suma[:])
}

// HashClave protege una contrasena con TUXOR en modo seguro.
//
// Devuelve una sola cadena con todo lo que hace falta para verificarla despues:
// `tuxor$costo$sal$hash`. Guardar la sal y el costo junto al hash es lo que
// permite subir el costo mas adelante sin invalidar las claves viejas.
func HashClave(usuario, clave string) (string, error) {
	crudo, err := tuxorCrudo(usuario, clave)
	if err != nil {
		return "", err
	}

	sal := make([]byte, 16)
	if _, err := rand.Read(sal); err != nil {
		return "", fmt.Errorf("no se pudo generar la sal: %w", err)
	}
	salHex := hex.EncodeToString(sal)

	derivado, err := derivar(crudo, salHex, CostoTuxor)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("tuxor$%d$%s$%s", CostoTuxor, salHex, derivado), nil
}

// ComprobarClave verifica una contrasena contra su hash guardado.
func ComprobarClave(guardado, usuario, clave string) bool {
	partes := strings.Split(guardado, "$")
	if len(partes) != 4 || partes[0] != "tuxor" {
		return false
	}

	var costo int
	if _, err := fmt.Sscanf(partes[1], "%d", &costo); err != nil || costo < 1 || costo > 20 {
		return false
	}

	crudo, err := tuxorCrudo(usuario, clave)
	if err != nil {
		return false
	}
	derivado, err := derivar(crudo, partes[2], costo)
	if err != nil {
		return false
	}
	// Comparacion de tiempo constante: comparar cadenas con == delata la clave
	// por cuanto tarda en encontrar la primera letra distinta.
	return subtle.ConstantTimeCompare([]byte(derivado), []byte(partes[3])) == 1
}

// derivar aplica scrypt al TUXOR crudo. Es la capa que cuesta dinero romper.
func derivar(crudo, sal string, costo int) (string, error) {
	derivado, err := scrypt.Key([]byte(crudo+":"+sal), []byte(sal), 1<<costo, 8, 1, 64)
	if err != nil {
		return "", fmt.Errorf("no se pudo derivar la clave: %w", err)
	}
	return hex.EncodeToString(derivado), nil
}
