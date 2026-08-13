package autenticacion

import (
	"strings"
	"testing"
)

// La prueba que manda sobre todas las demas de este archivo.
//
// TUXOR ya existe en PHP, JavaScript y Python. Esta es la cuarta implementacion,
// y **las cuatro tienen que dar el mismo valor byte por byte**: si esta se
// desvia, una clave guardada por MiRed no la verificaria ninguna otra
// herramienta de la casa, y al reves. El vector es el oficial de la
// especificacion.
func TestElVectorOficialDeTuxor(t *testing.T) {
	const (
		identidad = "+tuxor"
		secreto   = "*algorithm#"
		esperado  = "663b623d1f5f78b197cfe54fbdbb47dcb679c8842e0bb138d90e001aaa50fdb8"
	)

	obtenido, err := tuxorCrudo(identidad, secreto)
	if err != nil {
		t.Fatalf("no se pudo calcular: %v", err)
	}
	if obtenido != esperado {
		t.Fatalf("esta implementacion se desvio de las otras tres:\n  se obtuvo  %s\n  se esperaba %s",
			obtenido, esperado)
	}
}

func TestCadaOperadorHaceAlgoDistinto(t *testing.T) {
	// Si dos operadores dieran el mismo resultado, uno de los dos estaria mal
	// implementado y nadie lo notaria: las claves seguirian funcionando.
	vistos := map[string]string{}
	for _, operador := range strings.Split(operadoresValidos, "") {
		resultado, err := tuxorCrudo(operador+"usuario", "clave")
		if err != nil {
			t.Fatalf("operador %q: %v", operador, err)
		}
		if otro, repetido := vistos[resultado]; repetido {
			t.Fatalf("los operadores %q y %q dan lo mismo", operador, otro)
		}
		vistos[resultado] = operador
	}
}

func TestElModificadorArrobaCambiaElResultado(t *testing.T) {
	// El `@` decide si los operadores entran tambien al texto que se hashea. Los
	// cuatro modos tienen que dar cuatro resultados distintos.
	casos := map[string]string{
		"sin arroba":   "+usuario+",
		"solo prefijo": "@+usuario+",
		"solo sufijo":  "+usuario+@",
		"todos":        "@@+usuario+",
	}

	vistos := map[string]string{}
	for nombre, identidad := range casos {
		resultado, err := tuxorCrudo(identidad, "clave")
		if err != nil {
			t.Fatalf("%s: %v", nombre, err)
		}
		if otro, repetido := vistos[resultado]; repetido {
			t.Fatalf("%q y %q dan lo mismo y deberian diferir", nombre, otro)
		}
		vistos[resultado] = nombre
	}
}

func TestSinOperadoresNoHayTuxor(t *testing.T) {
	// Es la regla que mas sorprende del algoritmo, y por eso el formulario tiene
	// que avisarla antes de que alguien teclee una clave larga.
	if _, err := tuxorCrudo("usuario", "clave"); err == nil {
		t.Fatal("sin ningun operador no deberia poder calcularse")
	}
	// Basta con que UNO de los dos lo traiga.
	if _, err := tuxorCrudo("usuario", "+clave"); err != nil {
		t.Fatalf("con el operador en la clave deberia bastar: %v", err)
	}
	if _, err := tuxorCrudo("+usuario", "clave"); err != nil {
		t.Fatalf("con el operador en el usuario deberia bastar: %v", err)
	}
	// Pero un texto que sea SOLO operadores no deja nada que hashear.
	if _, err := tuxorCrudo("+++", "+clave"); err == nil {
		t.Fatal("un texto de puros operadores no deberia valer")
	}
}

func TestTuxorValidoAvisaLoMismoQueElCalculo(t *testing.T) {
	// El formulario usa TuxorValido y el servidor usa el calculo: si no
	// coincidieran, el formulario dejaria pasar algo que el servidor rechaza.
	casos := map[string]bool{
		"+usuario":    true,
		"usuario#":    true,
		"@@+usuario+": true,
		"usuario":     false,
		"+++":         false,
		"":            false,
		"@":           false,
	}
	for texto, esperado := range casos {
		if obtenido := TuxorValido(texto); obtenido != esperado {
			t.Errorf("TuxorValido(%q) = %v y se esperaba %v", texto, obtenido, esperado)
		}
	}
}

func TestLaClaveSeGuardaConSalYNoEnClaro(t *testing.T) {
	const usuario = "+admin"
	const clave = "*micla^ve"

	guardado, err := HashClave(usuario, clave)
	if err != nil {
		t.Fatalf("no se pudo proteger la clave: %v", err)
	}

	if strings.Contains(guardado, clave) {
		t.Fatal("la clave no puede aparecer en el hash")
	}
	if !strings.HasPrefix(guardado, "tuxor$") {
		t.Fatalf("el formato guardado deberia decir con que se hizo: %s", guardado)
	}

	// La sal hace que la misma clave nunca se guarde dos veces igual: sin eso,
	// dos usuarios con la misma clave se delatarian entre si.
	otro, err := HashClave(usuario, clave)
	if err != nil {
		t.Fatalf("no se pudo proteger la clave: %v", err)
	}
	if otro == guardado {
		t.Fatal("dos veces la misma clave deberian dar hashes distintos")
	}

	if !ComprobarClave(guardado, usuario, clave) {
		t.Fatal("la clave correcta deberia verificarse")
	}
	if ComprobarClave(guardado, usuario, "*otracla^ve") {
		t.Fatal("una clave equivocada no deberia verificarse")
	}
	// El usuario es parte del calculo: la MISMA clave con otro usuario no vale.
	if ComprobarClave(guardado, "+otro", clave) {
		t.Fatal("el usuario forma parte del hash y no deberia poder cambiarse")
	}
}

func TestUnHashEchoAManoNoSeCuela(t *testing.T) {
	// Un hash con formato raro tiene que fallar cerrado, nunca abierto.
	casos := []string{
		"",
		"tuxor",
		"tuxor$14$sal",                  // le falta el hash
		"bcrypt$14$sal$hash",            // otro algoritmo
		"tuxor$99$sal$hash",             // costo imposible
		"tuxor$abc$sal$hash",            // costo que no es numero
		"tuxor$14$sal$0000000000000000", // hash que no corresponde
	}
	for _, guardado := range casos {
		if ComprobarClave(guardado, "+admin", "*clave") {
			t.Errorf("no deberia aceptar %q", guardado)
		}
	}
}
