package version

import "testing"

// El historial es la fuente de la version: si esta mal, los binarios salen mal
// numerados y nadie se entera hasta que hay que rastrear un reporte.
func TestElHistorialQueViajaDentroDelBinarioSeLee(t *testing.T) {
	historial, err := LeerHistorial()
	if err != nil {
		t.Fatalf("no se pudo leer el historial: %v", err)
	}
	if len(historial.Sistema) == 0 || len(historial.Base) == 0 {
		t.Fatal("el historial deberia traer las dos listas")
	}

	// La ultima entrada es la que manda: de ahi salen la version y la revision.
	ultima := historial.Ultima()
	if ultima.Version != Numero[1:] {
		t.Fatalf("el binario dice %s y el historial %s: se compilo mal",
			Numero, "v"+ultima.Version)
	}
}

func TestLaRevisionNuncaSeReinicia(t *testing.T) {
	// Es la regla de la casa, y la unica forma de que se cumpla es que nadie la
	// teclee: sale de aqui. Esta prueba la vigila.
	historial, err := LeerHistorial()
	if err != nil {
		t.Fatalf("no se pudo leer el historial: %v", err)
	}

	vistas := map[int]bool{}
	mayor := 0
	for _, entrada := range historial.Sistema {
		if vistas[entrada.Revision] {
			t.Fatalf("la revision %d esta dos veces", entrada.Revision)
		}
		vistas[entrada.Revision] = true
		if entrada.Revision > mayor {
			mayor = entrada.Revision
		}
	}

	// Sin huecos: de la 1 a la mayor tienen que estar todas.
	for numero := 1; numero <= mayor; numero++ {
		if !vistas[numero] {
			t.Errorf("falta la revision %d: la cuenta no puede tener huecos", numero)
		}
	}
}

func TestCadaEntradaDiceDeQueTipoEs(t *testing.T) {
	historial, err := LeerHistorial()
	if err != nil {
		t.Fatalf("no se pudo leer el historial: %v", err)
	}

	validos := map[string]bool{"feature": true, "fix": true, "ajuste": true, "change": true}
	for _, lista := range [][]Entrada{historial.Sistema, historial.Base} {
		for _, entrada := range lista {
			// La base lo comprueba con un CHECK; aqui se cacha antes de llegar.
			if !validos[entrada.Tipo] {
				t.Errorf("tipo invalido %q en la revision %d", entrada.Tipo, entrada.Revision)
			}
			if entrada.Modulo == "" || entrada.Notas == "" || entrada.Fecha == "" {
				t.Errorf("la revision %d esta incompleta: %+v", entrada.Revision, entrada)
			}
		}
	}
}
