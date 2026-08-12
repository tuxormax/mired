package escaneo

import (
	"context"
	"net"
	"strconv"
	"strings"
	"time"
)

// puertosComunes es la lista que se prueba cuando nadie pide otra.
//
// Es corta a proposito. Probar los 65 535 puertos de cada equipo tarda horas,
// molesta a los equipos y no aporta: lo que identifica a un aparato son sus
// servicios visibles, y estos cubren practicamente todos los que aparecen en una
// red de oficina o casa.
var puertosComunes = []int{
	21,    // FTP
	22,    // SSH
	23,    // Telnet
	25,    // SMTP
	53,    // DNS
	80,    // HTTP
	110,   // POP3
	135,   // RPC de Windows
	139,   // NetBIOS
	143,   // IMAP
	161,   // SNMP (por TCP, raro pero lo hay)
	389,   // LDAP
	443,   // HTTPS
	445,   // Compartir archivos de Windows
	515,   // Impresion LPD
	554,   // RTSP (camaras)
	587,   // SMTP autenticado
	631,   // IPP (impresoras)
	1433,  // SQL Server
	1723,  // PPTP
	3000,  // Paneles y aplicaciones web
	3306,  // MySQL o MariaDB
	3389,  // Escritorio remoto
	5000,  // UPnP y paneles
	5432,  // PostgreSQL
	5900,  // VNC
	6379,  // Redis
	7000,  // Camaras y streaming
	8000,  // HTTP alterno
	8006,  // Proxmox
	8080,  // HTTP alterno
	8443,  // HTTPS alterno
	8888,  // HTTP alterno
	9000,  // Paneles
	9100,  // Impresion cruda (JetDirect)
	27017, // MongoDB
	32400, // Plex
	62078, // Sincronizacion de iPhone
}

// nombresServicio traduce el numero de puerto a algo que una persona entienda.
var nombresServicio = map[int]string{
	21: "FTP", 22: "SSH", 23: "Telnet", 25: "SMTP", 53: "DNS", 80: "HTTP",
	110: "POP3", 135: "RPC Windows", 139: "NetBIOS", 143: "IMAP", 161: "SNMP",
	389: "LDAP", 443: "HTTPS", 445: "Compartir archivos", 515: "Impresion LPD",
	554: "RTSP (camara)", 587: "SMTP", 631: "IPP (impresora)", 1433: "SQL Server",
	1723: "PPTP", 3000: "Panel web", 3306: "MySQL/MariaDB", 3389: "Escritorio remoto",
	5000: "UPnP / panel", 5432: "PostgreSQL", 5900: "VNC", 6379: "Redis",
	7000: "Streaming", 8000: "HTTP alterno", 8006: "Proxmox", 8080: "HTTP alterno",
	8443: "HTTPS alterno", 8888: "HTTP alterno", 9000: "Panel web",
	9100: "Impresion cruda", 27017: "MongoDB", 32400: "Plex", 62078: "iPhone",
}

// PuertosComunes devuelve la lista que se prueba por omision.
func PuertosComunes() []int {
	copia := make([]int, len(puertosComunes))
	copy(copia, puertosComunes)
	return copia
}

// NombreServicio traduce un puerto a su servicio conocido.
func NombreServicio(puerto int) string {
	if nombre, hay := nombresServicio[puerto]; hay {
		return nombre
	}
	return ""
}

// probarTCP intenta conectarse a un puerto. Devuelve si estaba abierto y, si el
// servicio se presenta solo, las primeras letras de lo que dijo.
func probarTCP(ctx context.Context, ip string, puerto int, espera time.Duration) (bool, string) {
	destino := net.JoinHostPort(ip, strconv.Itoa(puerto))

	marcador := net.Dialer{Timeout: espera}
	conexion, err := marcador.DialContext(ctx, "tcp", destino)
	if err != nil {
		return false, ""
	}
	defer conexion.Close()

	return true, leerBanner(conexion, espera)
}

// leerBanner escucha un momento por si el servicio se presenta solo (SSH, FTP y
// SMTP lo hacen). No se le manda nada: mandar datos a ciegas puede dejar en
// estados raros a equipos delicados, como las impresoras.
func leerBanner(conexion net.Conn, espera time.Duration) string {
	if err := conexion.SetReadDeadline(time.Now().Add(espera)); err != nil {
		return ""
	}

	buzon := make([]byte, 256)
	leidos, err := conexion.Read(buzon)
	if err != nil || leidos == 0 {
		return ""
	}

	texto := strings.TrimSpace(string(buzon[:leidos]))
	texto = strings.Map(func(letra rune) rune {
		if letra < 32 && letra != '\n' && letra != '\t' {
			return -1
		}
		return letra
	}, texto)

	if corte := strings.IndexAny(texto, "\r\n"); corte > 0 {
		texto = texto[:corte]
	}
	if len(texto) > 120 {
		texto = texto[:120]
	}
	return texto
}
