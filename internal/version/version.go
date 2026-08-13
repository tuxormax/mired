// Paquete version guarda que version de MiRed es esta. Los valores se fijan al
// compilar con -ldflags, y quedan tambien en cada reporte de error para saber en
// que build ocurrio.
package version

import "fmt"

var (
	// Numero es la version del sistema: v1.X, donde X sube con cada modulo.
	//
	// v1.1 (2026-08-13): MiRed dejo de ser un servicio con interfaz web y paso a
	// ser un programa de escritorio que arranca y detiene sus propios servicios.
	// Ese cambio se lleva un numero de version porque cambia como se usa, donde
	// viven los datos y que corre cuando nadie mira.
	Numero = "v1.1"
	// Revision sube en cada despliegue y nunca se reinicia.
	Revision = "0"
	// Build es el hash corto del commit con el que se compilo el binario.
	Build = "desarrollo"
)

// Completa arma la linea que se muestra en el pie de la interfaz y en los
// reportes de error.
func Completa() string {
	return fmt.Sprintf("MiRed %s Rev %s (%s)", Numero, Revision, Build)
}
