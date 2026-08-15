package huella

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// Lo que se pregunta A TODA LA RED de una vez.
//
// mDNS, UPnP y WS-Discovery no se le preguntan a un aparato: se gritan a un
// grupo y contesta quien quiera. Una sola pregunta por subred trae el nombre y
// el modelo de las televisiones, las consolas, los Chromecast, las impresoras y
// las camaras, sin tocarle un puerto a nadie.
//
// Cada respuesta se guarda contra la IP de QUIEN CONTESTO, que es lo que permite
// pegar esto con el inventario.

// Direcciones de los grupos donde escucha cada protocolo.
var (
	grupoSSDP        = &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}
	grupoMDNS        = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}
	grupoWSDiscovery = &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 3702}
	grupoSADP        = &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 37020}
)

// PreguntarALaRed manda las preguntas de grupo y junta lo que conteste cada IP.
//
// La espera es lo que tarda todo junto: se pregunta una vez y se escucha, no se
// insiste. Un aparato que no contesto en ese rato aparecera en el proximo
// escaneo, y eso es preferible a alargar cada barrido.
func PreguntarALaRed(ctx context.Context, espera time.Duration) map[string][]Dato {
	if espera <= 0 {
		espera = 3 * time.Second
	}
	juntadas := map[string]*Huella{}

	preguntas := []struct {
		grupo *net.UDPAddr
		carga []byte
		leer  func(*Huella, string)
	}{
		{grupoSSDP, peticionSSDP(), leerSSDP},
		{grupoWSDiscovery, peticionWSDiscovery(), leerWSDiscovery},
		{grupoSADP, peticionSADP(), leerSADP},
		{grupoMDNS, peticionMDNS(), nil},
	}

	// Las cuatro preguntas y la escucha van A LA VEZ. Una tras otra eran cinco
	// esperas de tres segundos en fila —quince segundos de barrido en los que el
	// equipo solo aguarda—, y son independientes: cada una escucha en su propio
	// puerto.
	var candado sync.Mutex
	var grupo sync.WaitGroup

	anotar := func(ip string, hacer func(*Huella)) {
		candado.Lock()
		defer candado.Unlock()
		huella := juntadas[ip]
		if huella == nil {
			huella = &Huella{IP: ip}
			juntadas[ip] = huella
		}
		hacer(huella)
	}

	for _, pregunta := range preguntas {
		grupo.Add(1)
		go func(grupoDestino *net.UDPAddr, carga []byte, leer func(*Huella, string)) {
			defer grupo.Done()
			for ip, mensajes := range gritar(ctx, grupoDestino, carga, espera) {
				for _, mensaje := range mensajes {
					anotar(ip, func(huella *Huella) {
						if leer != nil {
							leer(huella, string(mensaje))
							return
						}
						leerMDNS(huella, mensaje)
					})
				}
			}
		}(pregunta.grupo, pregunta.carga, pregunta.leer)
	}

	// Y lo que no se pregunta: los aparatos que avisan solos.
	grupo.Add(1)
	go func() {
		defer grupo.Done()
		for ip, datos := range EscucharTuya(ctx, espera) {
			for _, dato := range datos {
				anotar(ip, func(huella *Huella) {
					huella.Agregar(dato.Fuente, dato.Clave, dato.Valor)
				})
			}
		}
	}()

	grupo.Wait()

	resultado := map[string][]Dato{}
	for ip, huella := range juntadas {
		huella.Ordenar()
		if len(huella.Datos) > 0 {
			resultado[ip] = huella.Datos
		}
	}
	return resultado
}

// gritar manda una pregunta al grupo y junta todo lo que llegue, por IP.
func gritar(ctx context.Context, grupo *net.UDPAddr, carga []byte, espera time.Duration) map[string][][]byte {
	conexion, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil
	}
	defer conexion.Close()

	if _, err := conexion.WriteToUDP(carga, grupo); err != nil {
		return nil
	}

	limite := time.Now().Add(espera)
	if plazo, hay := ctx.Deadline(); hay && plazo.Before(limite) {
		limite = plazo
	}
	if err := conexion.SetReadDeadline(limite); err != nil {
		return nil
	}

	respuestas := map[string][][]byte{}
	buzon := make([]byte, 8192)
	for {
		if ctx.Err() != nil {
			break
		}
		leidos, quien, err := conexion.ReadFromUDP(buzon)
		if err != nil {
			break
		}
		if leidos == 0 || quien == nil {
			continue
		}
		direccion, ok := netip.AddrFromSlice(quien.IP)
		if !ok {
			continue
		}
		clave := direccion.Unmap().String()
		copia := make([]byte, leidos)
		copy(copia, buzon[:leidos])
		respuestas[clave] = append(respuestas[clave], copia)
	}
	return respuestas
}

// escucharDifusion se queda oyendo un puerto sin mandar nada.
//
// Hay aparatos que no se dejan preguntar: **avisan solos**, a gritos, cada pocos
// segundos. Los Tuya son el caso claro. Para esos no sirve el patron de
// preguntar y esperar: hay que estar escuchando cuando hablen.
//
// Si el puerto ya lo tiene alguien —otro programa de casa inteligente en el
// mismo equipo—, no se insiste ni se rompe nada: simplemente no se oye por ahi.
func escucharDifusion(ctx context.Context, puerto int, espera time.Duration) map[string][][]byte {
	conexion, err := net.ListenUDP("udp4", &net.UDPAddr{Port: puerto})
	if err != nil {
		return nil
	}
	defer conexion.Close()

	limite := time.Now().Add(espera)
	if plazo, hay := ctx.Deadline(); hay && plazo.Before(limite) {
		limite = plazo
	}
	if err := conexion.SetReadDeadline(limite); err != nil {
		return nil
	}

	oidos := map[string][][]byte{}
	buzon := make([]byte, 8192)
	for {
		if ctx.Err() != nil {
			break
		}
		leidos, quien, err := conexion.ReadFromUDP(buzon)
		if err != nil {
			break
		}
		if leidos == 0 || quien == nil {
			continue
		}
		direccion, ok := netip.AddrFromSlice(quien.IP)
		if !ok {
			continue
		}
		copia := make([]byte, leidos)
		copy(copia, buzon[:leidos])
		clave := direccion.Unmap().String()
		oidos[clave] = append(oidos[clave], copia)
	}
	return oidos
}

// ------------------------------------------------------------------ SSDP --

func peticionSSDP() []byte {
	return []byte("M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 2\r\n" +
		"ST: ssdp:all\r\n" +
		"USER-AGENT: MiRed/1.15 UPnP/1.1\r\n\r\n")
}

// leerSSDP se queda con lo que el aparato pone en su respuesta de UPnP.
//
// El encabezado SERVER trae el sistema y muchas veces el modelo entero; en los
// modems del proveedor y en las televisiones es donde esta el dato bueno.
func leerSSDP(huella *Huella, mensaje string) {
	for _, linea := range strings.Split(mensaje, "\n") {
		partes := strings.SplitN(strings.TrimSpace(linea), ":", 2)
		if len(partes) != 2 {
			continue
		}
		nombre := strings.ToUpper(strings.TrimSpace(partes[0]))
		valor := strings.TrimSpace(partes[1])

		switch nombre {
		case "SERVER":
			huella.Agregar(FuenteSSDP, ClaveServidor, valor)
		case "ST", "NT":
			// El tipo de servicio dice de que familia es: un MediaRenderer es
			// una television, un InternetGatewayDevice es el modem.
			huella.Agregar(FuenteSSDP, ClaveServicio, valor)
		case "USN":
			huella.Agregar(FuenteSSDP, ClaveSerie, entreDosPuntos(valor))
		}
	}
}

func entreDosPuntos(texto string) string {
	if inicio := strings.Index(texto, "uuid:"); inicio >= 0 {
		resto := texto[inicio+5:]
		if corte := strings.Index(resto, ":"); corte > 0 {
			return resto[:corte]
		}
		return resto
	}
	return ""
}

// --------------------------------------------------- WS-Discovery (ONVIF) --

func peticionWSDiscovery() []byte {
	return []byte(`<?xml version="1.0" encoding="utf-8"?>` +
		`<e:Envelope xmlns:e="http://www.w3.org/2003/05/soap-envelope" ` +
		`xmlns:w="http://schemas.xmlsoap.org/ws/2004/08/addressing" ` +
		`xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery" ` +
		`xmlns:dn="http://www.onvif.org/ver10/network/wsdl">` +
		`<e:Header>` +
		`<w:MessageID>uuid:11111111-2222-3333-4444-555555555555</w:MessageID>` +
		`<w:To e:mustUnderstand="true">urn:schemas-xmlsoap-org:ws:2005:04:discovery</w:To>` +
		`<w:Action e:mustUnderstand="true">http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</w:Action>` +
		`</e:Header>` +
		`<e:Body><d:Probe><d:Types>dn:NetworkVideoTransmitter</d:Types></d:Probe></e:Body>` +
		`</e:Envelope>`)
}

// leerWSDiscovery lee la respuesta de una camara ONVIF.
//
// Casi toda camara IP vendida aqui habla ONVIF, y en sus "scopes" pone el
// fabricante, el modelo y a veces el nombre que le puso el instalador.
func leerWSDiscovery(huella *Huella, mensaje string) {
	if !strings.Contains(mensaje, "Scopes") && !strings.Contains(mensaje, "onvif") {
		return
	}
	huella.Agregar(FuenteONVIF, ClaveServicio, "camara ONVIF")

	for _, alcance := range strings.Fields(entreEtiquetas(mensaje, "Scopes")) {
		switch {
		case strings.Contains(alcance, "/name/"):
			huella.Agregar(FuenteONVIF, ClaveNombre, ultimoTramo(alcance))
		case strings.Contains(alcance, "/hardware/"):
			huella.Agregar(FuenteONVIF, ClaveModelo, ultimoTramo(alcance))
		case strings.Contains(alcance, "/manufacturer/"):
			huella.Agregar(FuenteONVIF, ClaveFabricante, ultimoTramo(alcance))
		}
	}
}

// -------------------------------------------------------- Hikvision (SADP) --

func peticionSADP() []byte {
	return []byte(`<?xml version="1.0" encoding="utf-8"?>` +
		`<Probe><Uuid>11111111-2222-3333-4444-555555555555</Uuid>` +
		`<Types>inquiry</Types></Probe>`)
}

// leerSADP lee lo que contestan las camaras y grabadores Hikvision al protocolo
// que usa su propia herramienta de instalacion.
func leerSADP(huella *Huella, mensaje string) {
	if !strings.Contains(mensaje, "DeviceType") && !strings.Contains(mensaje, "Types") {
		return
	}
	huella.Agregar(FuenteHikvision, ClaveModelo, entreEtiquetas(mensaje, "DeviceType"))
	huella.Agregar(FuenteHikvision, ClaveNombre, entreEtiquetas(mensaje, "DeviceDescription"))
	huella.Agregar(FuenteHikvision, ClaveSerie, entreEtiquetas(mensaje, "DeviceSN"))
	huella.Agregar(FuenteHikvision, ClaveFirmware, entreEtiquetas(mensaje, "DeviceSoftwareVersion"))
	huella.Agregar(FuenteHikvision, ClaveMAC, entreEtiquetas(mensaje, "MAC"))
	if len(huella.Datos) > 0 {
		huella.Agregar(FuenteHikvision, ClaveFabricante, "Hikvision")
	}
}

// entreEtiquetas saca el contenido de una etiqueta XML, sin importar con que
// prefijo de espacio de nombres venga: la misma camara contesta <Scopes> o
// <d:Scopes> segun la marca, y exigir uno de los dos seria dejar de leer a la
// mitad de los aparatos. Sirve para campos hoja, que es lo que se busca aqui.
func entreEtiquetas(texto, etiqueta string) string {
	minusculas := strings.ToLower(texto)
	buscada := strings.ToLower(etiqueta)

	desde := 0
	for desde < len(minusculas) {
		abre := strings.Index(minusculas[desde:], "<")
		if abre < 0 {
			return ""
		}
		abre += desde
		cierra := strings.Index(minusculas[abre:], ">")
		if cierra < 0 {
			return ""
		}
		cierra += abre

		nombre := minusculas[abre+1 : cierra]
		if corte := strings.IndexAny(nombre, " \t\r\n"); corte >= 0 {
			nombre = nombre[:corte]
		}
		if corte := strings.Index(nombre, ":"); corte >= 0 {
			nombre = nombre[corte+1:]
		}

		if nombre == buscada {
			fin := strings.Index(minusculas[cierra:], "</")
			if fin < 0 {
				return ""
			}
			return strings.TrimSpace(texto[cierra+1 : cierra+fin])
		}
		desde = cierra + 1
	}
	return ""
}

func ultimoTramo(texto string) string {
	if corte := strings.LastIndex(texto, "/"); corte >= 0 && corte+1 < len(texto) {
		texto = texto[corte+1:]
	}
	// Los scopes vienen con los espacios escapados.
	return strings.ReplaceAll(texto, "%20", " ")
}
