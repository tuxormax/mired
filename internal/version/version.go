// Paquete version guarda que version de MiRed es esta. Los valores se fijan al
// compilar con -ldflags, y quedan tambien en cada reporte de error para saber en
// que build ocurrio.
package version

import "fmt"

var (
	// Numero es la version del sistema: v1.X, donde **X es el numero de MODULOS**.
	//
	// No es versionado semantico: X no sube porque un cambio sea grande, sube
	// cuando hay un modulo mas. Los 19 de hoy: redes, subredes, equipos, escaneo
	// y agenda, presencia, mapa de puertos, mapa visual, alertas, consumo,
	// inspeccion profunda, usuarios, credenciales SNMP, controladoras WiFi,
	// catalogo de dispositivos, topologia manual, catalogo comunitario, escucha
	// del aire, enlaces inalambricos y credenciales por equipo.
	//
	// Este valor y el de abajo son solo el respaldo para `go run`: al compilar de
	// verdad los fija construir.sh leyendolos del historial.
	Numero = "v1.20"
	// Revision sube en cada entrega y NUNCA se reinicia, tampoco al cambiar de
	// version. Sale del historial, no de un argumento que alguien teclea.
	Revision = "47"
	// Build es el hash corto del commit con el que se compilo el binario.
	Build = "desarrollo"
)

// Completa arma la linea que se muestra en el pie de la interfaz y en los
// reportes de error.
func Completa() string {
	return fmt.Sprintf("MiRed %s Rev %s (%s)", Numero, Revision, Build)
}
