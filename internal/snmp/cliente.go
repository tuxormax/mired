// Paquete snmp le pregunta a los switches administrables lo que ningun barrido
// de red puede averiguar por su cuenta: que hay conectado en cada puerto.
//
// Corre dentro de mired-sonda. No toca la base de datos: devuelve lo que
// contesto el equipo y el servidor decide que guardar.
package snmp

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

// Credencial es como se le habla a un equipo por SNMP.
type Credencial struct {
	Nombre    string `json:"nombre"`
	Version   string `json:"version"` // v1, v2c o v3
	Comunidad string `json:"comunidad,omitempty"`

	Usuario                string `json:"usuario,omitempty"`
	AutenticacionProtocolo string `json:"autenticacionProtocolo,omitempty"`
	AutenticacionClave     string `json:"autenticacionClave,omitempty"`
	PrivacidadProtocolo    string `json:"privacidadProtocolo,omitempty"`
	PrivacidadClave        string `json:"privacidadClave,omitempty"`
}

// OID que se consultan. Se dejan con nombre porque un numero suelto en el codigo
// no le dice nada a nadie seis meses despues.
const (
	oidSysDescr    = "1.3.6.1.2.1.1.1.0"
	oidSysObjectID = "1.3.6.1.2.1.1.2.0"
	oidSysUpTime   = "1.3.6.1.2.1.1.3.0"
	oidSysContacto = "1.3.6.1.2.1.1.4.0"
	oidSysNombre   = "1.3.6.1.2.1.1.5.0"
	oidSysUbicado  = "1.3.6.1.2.1.1.6.0"
	// sysServices dice que capas atiende el equipo. El bit 2 (valor 2) es capa
	// 2: es la pista mas barata para distinguir un switch de una impresora.
	oidSysServices = "1.3.6.1.2.1.1.7.0"

	// Tabla de interfaces.
	oidIfDescr       = "1.3.6.1.2.1.2.2.1.2"
	oidIfTipo        = "1.3.6.1.2.1.2.2.1.3"
	oidIfMac         = "1.3.6.1.2.1.2.2.1.6"
	oidIfEstadoOper  = "1.3.6.1.2.1.2.2.1.8"
	oidIfNombre      = "1.3.6.1.2.1.31.1.1.1.1" // ifName, el nombre corto (Gi0/1)
	oidIfAlias       = "1.3.6.1.2.1.31.1.1.1.18"
	oidIfVelocidadHi = "1.3.6.1.2.1.31.1.1.1.15" // ifHighSpeed, en Mbps

	// Tabla de reenvio de MAC del puente (BRIDGE-MIB).
	oidFdbPuerto     = "1.3.6.1.2.1.17.4.3.1.2" // dot1dTpFdbPort
	oidPuertoAIfIndx = "1.3.6.1.2.1.17.1.4.1.2" // dot1dBasePortIfIndex
	// La version con VLAN (Q-BRIDGE-MIB), que es la que usan los switches
	// modernos cuando hay mas de una VLAN.
	oidFdbPuertoVLAN = "1.3.6.1.2.1.17.7.1.2.2.1.2" // dot1qTpFdbPort

	// Contadores de trafico. Los de 64 bits (ifHC*) son los buenos: los de 32
	// se desbordan en minutos en un puerto gigabit y dan cifras absurdas.
	oidIfEntradaHi = "1.3.6.1.2.1.31.1.1.1.6"  // ifHCInOctets
	oidIfSalidaHi  = "1.3.6.1.2.1.31.1.1.1.10" // ifHCOutOctets
	oidIfEntrada   = "1.3.6.1.2.1.2.2.1.10"    // ifInOctets, de 32 bits
	oidIfSalida    = "1.3.6.1.2.1.2.2.1.16"    // ifOutOctets, de 32 bits

	// LLDP: quien dice ser el vecino de cada puerto.
	oidLldpRemNombre = "1.0.8802.1.1.2.1.4.1.1.9"  // lldpRemSysName
	oidLldpRemPuerto = "1.0.8802.1.1.2.1.4.1.1.7"  // lldpRemPortId
	oidLldpRemDescr  = "1.0.8802.1.1.2.1.4.1.1.10" // lldpRemSysDesc
	oidLldpRemChasis = "1.0.8802.1.1.2.1.4.1.1.5"  // lldpRemChassisId

	// CDP: lo mismo que LLDP pero de Cisco, y anterior. Sigue siendo lo unico
	// que hablan muchos switches Cisco de los que hay instalados, que a menudo
	// traen LLDP apagado de fabrica.
	oidCdpVecinoNombre    = "1.3.6.1.4.1.9.9.23.1.2.1.1.6" // cdpCacheDeviceId
	oidCdpVecinoPuerto    = "1.3.6.1.4.1.9.9.23.1.2.1.1.7" // cdpCacheDevicePort
	oidCdpVecinoDescr     = "1.3.6.1.4.1.9.9.23.1.2.1.1.8" // cdpCachePlatform
	oidCdpVecinoDireccion = "1.3.6.1.4.1.9.9.23.1.2.1.1.4" // cdpCacheAddress
)

// De donde salio un vecino.
const (
	OrigenLLDP = "lldp"
	OrigenCDP  = "cdp"
)

// Ficha es lo que se averiguo de un equipo por SNMP.
type Ficha struct {
	IP          string     `json:"ip"`
	Nombre      string     `json:"nombre"`
	Descripcion string     `json:"descripcion"`
	Contacto    string     `json:"contacto"`
	Ubicacion   string     `json:"ubicacion"`
	ObjectID    string     `json:"objectId"`
	EncendidoMs int64      `json:"encendidoMs"`
	EsSwitch    bool       `json:"esSwitch"`
	Credencial  string     `json:"credencial"`
	Interfaces  []Interfaz `json:"interfaces,omitempty"`
	// MacsPorPuerto son las MAC que el switch ve colgando de cada puerto. Es el
	// dato que responde "¿que hay conectado en el puerto 7?".
	MacsPorPuerto map[string][]string `json:"macsPorPuerto,omitempty"`
	// Contadores son los bytes acumulados de cada puerto, por indice de interfaz.
	Contadores map[int]Contador `json:"contadores,omitempty"`
	// Vecinos son los equipos que se anunciaron por LLDP en cada puerto.
	Vecinos []Vecino `json:"vecinos,omitempty"`
}

// Interfaz es un puerto del equipo.
type Interfaz struct {
	Indice        int    `json:"indice"`
	Nombre        string `json:"nombre"`
	Descripcion   string `json:"descripcion"`
	Alias         string `json:"alias"`
	MAC           string `json:"mac"`
	Tipo          int    `json:"tipo"`
	Activa        bool   `json:"activa"`
	VelocidadMbps int    `json:"velocidadMbps"`
}

// Contador son los bytes que han pasado por un puerto desde que el switch
// encendio. El dato util es la resta entre dos lecturas.
type Contador struct {
	Entrada uint64 `json:"entrada"`
	Salida  uint64 `json:"salida"`
	// SesentaYCuatro dice si vino del contador de 64 bits. Con los de 32 hay que
	// desconfiar de cualquier resta grande: se desbordan solos.
	SesentaYCuatro bool `json:"sesentaYCuatro"`
}

// Vecino es un equipo que se anuncio en un puerto, por LLDP o por CDP.
type Vecino struct {
	InterfazLocal string `json:"interfazLocal"`
	Nombre        string `json:"nombre"`
	Descripcion   string `json:"descripcion"`
	PuertoRemoto  string `json:"puertoRemoto"`
	ChasisID      string `json:"chasisId"`
	// Origen es "lldp" o "cdp". Se conserva porque los dos protocolos pueden ver
	// al mismo vecino y decir cosas distintas de el: guardar cual lo dijo permite
	// creerle al que corresponda en vez de que uno pise al otro.
	Origen string `json:"origen"`
	// DireccionIP es la que el vecino anuncia de si mismo. CDP la manda; LLDP la
	// deja en una tabla aparte que casi nadie llena, asi que ahi suele ir vacia.
	DireccionIP string `json:"direccionIp,omitempty"`
}

// Consultar le pregunta todo a un equipo con las credenciales dadas.
//
// Se prueban las credenciales en orden y se usa la primera que conteste: en una
// red real cada switch suele tener la suya, y obligar a decir cual va con cual
// seria pedirle al usuario el trabajo que la herramienta puede hacer sola.
func Consultar(ip string, credenciales []Credencial, espera time.Duration) (Ficha, error) {
	if len(credenciales) == 0 {
		return Ficha{}, fmt.Errorf("no hay credenciales SNMP configuradas")
	}

	var ultimoError error
	for _, credencial := range credenciales {
		ficha, err := consultarCon(ip, credencial, espera)
		if err == nil {
			return ficha, nil
		}
		ultimoError = err
	}
	return Ficha{}, ultimoError
}

func consultarCon(ip string, credencial Credencial, espera time.Duration) (Ficha, error) {
	conexion, err := abrir(ip, credencial, espera)
	if err != nil {
		return Ficha{}, err
	}
	defer conexion.Conn.Close()

	ficha := Ficha{IP: ip, Credencial: credencial.Nombre}

	// Primero lo barato: si el equipo no contesta ni su descripcion, no hay SNMP
	// y no vale la pena seguir preguntandole tablas enteras.
	respuesta, err := conexion.Get([]string{
		oidSysDescr, oidSysNombre, oidSysObjectID, oidSysUpTime,
		oidSysContacto, oidSysUbicado, oidSysServices,
	})
	if err != nil {
		return Ficha{}, fmt.Errorf("%s no contesto SNMP: %w", ip, err)
	}

	for _, dato := range respuesta.Variables {
		switch {
		case strings.HasPrefix(dato.Name, "."+oidSysDescr), dato.Name == oidSysDescr:
			ficha.Descripcion = comoTexto(dato)
		case strings.HasSuffix(dato.Name, oidSysNombre):
			ficha.Nombre = comoTexto(dato)
		case strings.HasSuffix(dato.Name, oidSysObjectID):
			ficha.ObjectID = comoTexto(dato)
		case strings.HasSuffix(dato.Name, oidSysUpTime):
			ficha.EncendidoMs = comoEntero(dato) * 10 // llega en centesimas
		case strings.HasSuffix(dato.Name, oidSysContacto):
			ficha.Contacto = comoTexto(dato)
		case strings.HasSuffix(dato.Name, oidSysUbicado):
			ficha.Ubicacion = comoTexto(dato)
		case strings.HasSuffix(dato.Name, oidSysServices):
			// El bit de capa 2 encendido: es puente, o sea switch.
			ficha.EsSwitch = comoEntero(dato)&0x02 != 0
		}
	}

	ficha.Interfaces = leerInterfaces(conexion)
	ficha.Contadores = leerContadores(conexion)
	ficha.MacsPorPuerto = leerTablaMac(conexion)
	ficha.Vecinos = append(leerVecinos(conexion, ficha.Interfaces),
		leerVecinosCDP(conexion, ficha.Interfaces)...)

	// Un equipo que reporta una tabla de MAC es un switch aunque sysServices no
	// lo diga: hay fabricantes que ese campo lo llenan mal o no lo llenan.
	if len(ficha.MacsPorPuerto) > 0 {
		ficha.EsSwitch = true
	}
	return ficha, nil
}

func abrir(ip string, credencial Credencial, espera time.Duration) (*gosnmp.GoSNMP, error) {
	conexion := &gosnmp.GoSNMP{
		Target:  ip,
		Port:    161,
		Timeout: espera,
		Retries: 1,
		MaxOids: gosnmp.MaxOids,
	}

	switch credencial.Version {
	case "v1":
		conexion.Version = gosnmp.Version1
		conexion.Community = credencial.Comunidad
	case "v3":
		conexion.Version = gosnmp.Version3
		conexion.SecurityModel = gosnmp.UserSecurityModel
		conexion.MsgFlags = nivelSeguridad(credencial)
		conexion.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 credencial.Usuario,
			AuthenticationProtocol:   protocoloAutenticacion(credencial.AutenticacionProtocolo),
			AuthenticationPassphrase: credencial.AutenticacionClave,
			PrivacyProtocol:          protocoloPrivacidad(credencial.PrivacidadProtocolo),
			PrivacyPassphrase:        credencial.PrivacidadClave,
		}
	default: // v2c
		conexion.Version = gosnmp.Version2c
		conexion.Community = credencial.Comunidad
	}

	if err := conexion.Connect(); err != nil {
		return nil, fmt.Errorf("no se pudo abrir SNMP con %s: %w", ip, err)
	}
	return conexion, nil
}

func nivelSeguridad(credencial Credencial) gosnmp.SnmpV3MsgFlags {
	switch {
	case credencial.PrivacidadClave != "" && credencial.AutenticacionClave != "":
		return gosnmp.AuthPriv
	case credencial.AutenticacionClave != "":
		return gosnmp.AuthNoPriv
	default:
		return gosnmp.NoAuthNoPriv
	}
}

func protocoloAutenticacion(nombre string) gosnmp.SnmpV3AuthProtocol {
	switch strings.ToUpper(nombre) {
	case "MD5":
		return gosnmp.MD5
	case "SHA":
		return gosnmp.SHA
	case "SHA224":
		return gosnmp.SHA224
	case "SHA256":
		return gosnmp.SHA256
	case "SHA384":
		return gosnmp.SHA384
	case "SHA512":
		return gosnmp.SHA512
	default:
		return gosnmp.NoAuth
	}
}

func protocoloPrivacidad(nombre string) gosnmp.SnmpV3PrivProtocol {
	switch strings.ToUpper(nombre) {
	case "DES":
		return gosnmp.DES
	case "AES":
		return gosnmp.AES
	case "AES192":
		return gosnmp.AES192
	case "AES256":
		return gosnmp.AES256
	default:
		return gosnmp.NoPriv
	}
}

// leerInterfaces arma la tabla de puertos del equipo.
func leerInterfaces(conexion *gosnmp.GoSNMP) []Interfaz {
	porIndice := map[int]*Interfaz{}

	recorrer(conexion, oidIfDescr, func(indice string, dato gosnmp.SnmpPDU) {
		if numero, err := strconv.Atoi(indice); err == nil {
			interfaz(porIndice, numero).Descripcion = comoTexto(dato)
		}
	})
	recorrer(conexion, oidIfNombre, func(indice string, dato gosnmp.SnmpPDU) {
		if numero, err := strconv.Atoi(indice); err == nil {
			interfaz(porIndice, numero).Nombre = comoTexto(dato)
		}
	})
	recorrer(conexion, oidIfAlias, func(indice string, dato gosnmp.SnmpPDU) {
		if numero, err := strconv.Atoi(indice); err == nil {
			interfaz(porIndice, numero).Alias = comoTexto(dato)
		}
	})
	recorrer(conexion, oidIfTipo, func(indice string, dato gosnmp.SnmpPDU) {
		if numero, err := strconv.Atoi(indice); err == nil {
			interfaz(porIndice, numero).Tipo = int(comoEntero(dato))
		}
	})
	recorrer(conexion, oidIfMac, func(indice string, dato gosnmp.SnmpPDU) {
		if numero, err := strconv.Atoi(indice); err == nil {
			interfaz(porIndice, numero).MAC = comoMAC(dato)
		}
	})
	recorrer(conexion, oidIfEstadoOper, func(indice string, dato gosnmp.SnmpPDU) {
		if numero, err := strconv.Atoi(indice); err == nil {
			interfaz(porIndice, numero).Activa = comoEntero(dato) == 1
		}
	})
	recorrer(conexion, oidIfVelocidadHi, func(indice string, dato gosnmp.SnmpPDU) {
		if numero, err := strconv.Atoi(indice); err == nil {
			interfaz(porIndice, numero).VelocidadMbps = int(comoEntero(dato))
		}
	})

	lista := make([]Interfaz, 0, len(porIndice))
	for _, puerto := range porIndice {
		if puerto.Nombre == "" {
			puerto.Nombre = puerto.Descripcion
		}
		lista = append(lista, *puerto)
	}
	return lista
}

func interfaz(porIndice map[int]*Interfaz, numero int) *Interfaz {
	if existente, hay := porIndice[numero]; hay {
		return existente
	}
	nueva := &Interfaz{Indice: numero}
	porIndice[numero] = nueva
	return nueva
}

// leerTablaMac averigua que MAC ve el switch en cada puerto.
//
// Es EL dato de la fase: sin el no hay mapa de puertos. Se intenta primero la
// tabla con VLAN, que es la que llenan los switches modernos, y se cae a la
// clasica si esa viene vacia.
func leerTablaMac(conexion *gosnmp.GoSNMP) map[string][]string {
	// dot1dTpFdbPort da un "puerto de puente" que NO es el indice de interfaz:
	// hay que traducirlo con dot1dBasePortIfIndex.
	puenteAInterfaz := map[int]int{}
	recorrer(conexion, oidPuertoAIfIndx, func(indice string, dato gosnmp.SnmpPDU) {
		if puerto, err := strconv.Atoi(indice); err == nil {
			puenteAInterfaz[puerto] = int(comoEntero(dato))
		}
	})

	porPuerto := map[string][]string{}

	juntar := func(sufijo string, dato gosnmp.SnmpPDU) {
		mac := macDesdeOID(sufijo)
		if mac == "" {
			return
		}
		puerto := int(comoEntero(dato))
		if puerto == 0 {
			return
		}
		if traducido, hay := puenteAInterfaz[puerto]; hay {
			puerto = traducido
		}
		clave := strconv.Itoa(puerto)
		porPuerto[clave] = append(porPuerto[clave], mac)
	}

	recorrer(conexion, oidFdbPuertoVLAN, juntar)
	if len(porPuerto) == 0 {
		recorrer(conexion, oidFdbPuerto, juntar)
	}
	return porPuerto
}

// leerContadores lee los bytes acumulados de cada puerto.
//
// Se intentan primero los de 64 bits. Los de 32 se desbordan cada pocos minutos
// en un puerto gigabit, asi que una resta hecha sobre ellos puede dar cualquier
// cosa; por eso se anota de cual vinieron.
func leerContadores(conexion *gosnmp.GoSNMP) map[int]Contador {
	contadores := map[int]Contador{}

	juntar := func(raiz string, sesentaYCuatro bool, entrada bool) bool {
		encontro := false
		recorrer(conexion, raiz, func(indice string, dato gosnmp.SnmpPDU) {
			numero, err := strconv.Atoi(indice)
			if err != nil {
				return
			}
			encontro = true
			contador := contadores[numero]
			contador.SesentaYCuatro = sesentaYCuatro
			if entrada {
				contador.Entrada = comoSinSigno(dato)
			} else {
				contador.Salida = comoSinSigno(dato)
			}
			contadores[numero] = contador
		})
		return encontro
	}

	if !juntar(oidIfEntradaHi, true, true) {
		juntar(oidIfEntrada, false, true)
	}
	if !juntar(oidIfSalidaHi, true, false) {
		juntar(oidIfSalida, false, false)
	}
	return contadores
}

func comoSinSigno(dato gosnmp.SnmpPDU) uint64 {
	switch valor := dato.Value.(type) {
	case uint64:
		return valor
	case uint:
		return uint64(valor)
	case uint32:
		return uint64(valor)
	case int64:
		if valor < 0 {
			return 0
		}
		return uint64(valor)
	case int:
		if valor < 0 {
			return 0
		}
		return uint64(valor)
	default:
		return 0
	}
}

// leerVecinos junta lo que los switches se anuncian entre si por LLDP. Es lo que
// permite dibujar los enlaces entre switches y no solo las hojas.
func leerVecinos(conexion *gosnmp.GoSNMP, interfaces []Interfaz) []Vecino {
	porIndice := map[int]string{}
	for _, puerto := range interfaces {
		porIndice[puerto.Indice] = puerto.Nombre
	}

	vecinos := map[string]*Vecino{}
	tomar := func(sufijo string) *Vecino {
		puerto, ok := puertoLocal(sufijo, 1, porIndice)
		if !ok {
			return nil
		}
		if existente, hay := vecinos[sufijo]; hay {
			return existente
		}
		nuevo := &Vecino{InterfazLocal: puerto}
		vecinos[sufijo] = nuevo
		return nuevo
	}

	recorrer(conexion, oidLldpRemNombre, func(sufijo string, dato gosnmp.SnmpPDU) {
		if vecino := tomar(sufijo); vecino != nil {
			vecino.Nombre = comoTexto(dato)
		}
	})
	recorrer(conexion, oidLldpRemPuerto, func(sufijo string, dato gosnmp.SnmpPDU) {
		if vecino := tomar(sufijo); vecino != nil {
			vecino.PuertoRemoto = comoTexto(dato)
		}
	})
	recorrer(conexion, oidLldpRemDescr, func(sufijo string, dato gosnmp.SnmpPDU) {
		if vecino := tomar(sufijo); vecino != nil {
			vecino.Descripcion = comoTexto(dato)
		}
	})
	recorrer(conexion, oidLldpRemChasis, func(sufijo string, dato gosnmp.SnmpPDU) {
		if vecino := tomar(sufijo); vecino != nil {
			vecino.ChasisID = comoMAC(dato)
		}
	})

	lista := make([]Vecino, 0, len(vecinos))
	for _, vecino := range vecinos {
		if vecino.Nombre != "" || vecino.PuertoRemoto != "" {
			vecino.Origen = OrigenLLDP
			lista = append(lista, *vecino)
		}
	}
	return lista
}

// leerVecinosCDP hace lo mismo que leerVecinos pero con el protocolo de Cisco.
//
// Vale la pena preguntar por los dos: **CDP viene encendido de fabrica en los
// Cisco y LLDP no**, asi que en una red con equipo Cisco viejo esta tabla es la
// unica que trae los enlaces entre switches. Y al reves: un switch de otra marca
// no sabe nada de CDP. Preguntar los dos cuesta dos recorridos y cubre las dos
// mitades del parque instalado.
//
// La diferencia que importa al leerlo: en CDP el indice de la fila es
// ifIndexLocal.numeroDeVecino, o sea el **primer** numero es el puerto local. En
// LLDP es el del medio. Confundirlos cuelga cada vecino del puerto equivocado.
func leerVecinosCDP(conexion *gosnmp.GoSNMP, interfaces []Interfaz) []Vecino {
	porIndice := map[int]string{}
	for _, puerto := range interfaces {
		porIndice[puerto.Indice] = puerto.Nombre
	}

	vecinos := map[string]*Vecino{}
	tomar := func(sufijo string) *Vecino {
		puerto, ok := puertoLocal(sufijo, 0, porIndice)
		if !ok {
			return nil
		}
		if existente, hay := vecinos[sufijo]; hay {
			return existente
		}
		nuevo := &Vecino{InterfazLocal: puerto, Origen: OrigenCDP}
		vecinos[sufijo] = nuevo
		return nuevo
	}

	recorrer(conexion, oidCdpVecinoNombre, func(sufijo string, dato gosnmp.SnmpPDU) {
		if vecino := tomar(sufijo); vecino != nil {
			vecino.Nombre = comoTexto(dato)
		}
	})
	recorrer(conexion, oidCdpVecinoPuerto, func(sufijo string, dato gosnmp.SnmpPDU) {
		if vecino := tomar(sufijo); vecino != nil {
			vecino.PuertoRemoto = comoTexto(dato)
		}
	})
	recorrer(conexion, oidCdpVecinoDescr, func(sufijo string, dato gosnmp.SnmpPDU) {
		if vecino := tomar(sufijo); vecino != nil {
			vecino.Descripcion = comoTexto(dato)
		}
	})
	recorrer(conexion, oidCdpVecinoDireccion, func(sufijo string, dato gosnmp.SnmpPDU) {
		if vecino := tomar(sufijo); vecino != nil {
			vecino.DireccionIP = comoIP(dato)
		}
	})

	lista := make([]Vecino, 0, len(vecinos))
	for _, vecino := range vecinos {
		if vecino.Nombre != "" || vecino.PuertoRemoto != "" {
			lista = append(lista, *vecino)
		}
	}
	return lista
}

// puertoLocal saca de que puerto del equipo consultado habla una fila de vecinos, y
// la devuelve con el nombre bonito del puerto si se conoce.
//
// **Los dos protocolos ponen ese numero en un lugar distinto del indice**, y es
// el error facil de cometer leyendo las MIB:
//   - LLDP indexa `tiempo.puertoLocal.numeroDeVecino` → posicion 1, la de en medio.
//   - CDP indexa `ifIndexLocal.numeroDeVecino` → posicion 0, la primera.
//
// Equivocarse no revienta nada: cuelga cada vecino del puerto equivocado, que es
// mucho peor porque el mapa sale plausible y falso.
func puertoLocal(sufijo string, posicion int, porIndice map[int]string) (string, bool) {
	partes := strings.Split(sufijo, ".")
	if posicion >= len(partes) {
		return "", false
	}
	numero, err := strconv.Atoi(partes[posicion])
	if err != nil {
		return "", false
	}
	if nombre, hay := porIndice[numero]; hay && nombre != "" {
		return nombre, true
	}
	return partes[posicion], true
}

// comoIP formatea la direccion que CDP manda como bytes crudos: cuatro para IPv4
// y dieciseis para IPv6.
func comoIP(dato gosnmp.SnmpPDU) string {
	crudo, ok := dato.Value.([]byte)
	if !ok {
		return ""
	}
	if len(crudo) != 4 && len(crudo) != 16 {
		return ""
	}
	direccion := net.IP(crudo)
	if direccion.IsUnspecified() {
		return ""
	}
	return direccion.String()
}

// recorrer camina una tabla y entrega a la funcion el sufijo del OID (el indice
// de la fila) y el valor.
func recorrer(conexion *gosnmp.GoSNMP, raiz string, hacer func(sufijo string, dato gosnmp.SnmpPDU)) {
	// BulkWalk va mucho mas rapido, pero no existe en SNMPv1.
	caminar := conexion.BulkWalk
	if conexion.Version == gosnmp.Version1 {
		caminar = conexion.Walk
	}

	_ = caminar(raiz, func(dato gosnmp.SnmpPDU) error {
		nombre := strings.TrimPrefix(dato.Name, ".")
		if !strings.HasPrefix(nombre, raiz+".") {
			return nil
		}
		hacer(strings.TrimPrefix(nombre, raiz+"."), dato)
		return nil
	})
}

func comoTexto(dato gosnmp.SnmpPDU) string {
	switch valor := dato.Value.(type) {
	case string:
		return strings.TrimSpace(valor)
	case []byte:
		return strings.TrimSpace(string(valor))
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", valor))
	}
}

func comoEntero(dato gosnmp.SnmpPDU) int64 {
	switch valor := dato.Value.(type) {
	case int:
		return int64(valor)
	case int64:
		return valor
	case uint:
		return int64(valor)
	case uint32:
		return int64(valor)
	case uint64:
		return int64(valor)
	default:
		return 0
	}
}

// comoMAC formatea una direccion fisica que llega como bytes crudos.
func comoMAC(dato gosnmp.SnmpPDU) string {
	crudo, ok := dato.Value.([]byte)
	if !ok || len(crudo) != 6 {
		return ""
	}
	return strings.ToLower(net.HardwareAddr(crudo).String())
}

// macDesdeOID arma la MAC que viene codificada en el indice de la fila.
//
// La tabla de reenvio no guarda la MAC como valor sino como parte del OID: seis
// numeros decimales al final. En la version con VLAN vienen siete (el primero es
// la VLAN) y se descarta el primero.
func macDesdeOID(sufijo string) string {
	partes := strings.Split(sufijo, ".")
	if len(partes) == 7 {
		partes = partes[1:]
	}
	if len(partes) != 6 {
		return ""
	}

	bytes := make([]byte, 6)
	for i, parte := range partes {
		numero, err := strconv.Atoi(parte)
		if err != nil || numero < 0 || numero > 255 {
			return ""
		}
		bytes[i] = byte(numero)
	}
	return strings.ToLower(net.HardwareAddr(bytes).String())
}
