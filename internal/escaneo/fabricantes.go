package escaneo

import (
	"bufio"
	"os"
	"strings"
	"sync"
)

// El fabricante de un aparato se sabe por los primeros tres bytes de su MAC, que
// la IEEE asigna por empresa. Es lo primero que convierte una lista de IP en
// algo legible: "Hikvision" ya dice camara, "Brother" ya dice impresora.
//
// La lista completa de la IEEE son unas 35 000 entradas y 3 MB. Aqui va
// embebida una lista corta con lo que aparece de verdad en una red de oficina o
// casa, y se puede ampliar sin recompilar dejando el archivo oficial en
// /usr/share/mired/oui.txt (formato: prefijo de 6 caracteres y nombre).

const rutaOUI = "/usr/share/mired/oui.txt"

var fabricantesComunes = map[string]string{
	"000c29": "VMware", "005056": "VMware", "001c14": "VMware",
	"080027": "VirtualBox", "525400": "QEMU/KVM", "0242ac": "Docker",
	"b827eb": "Raspberry Pi", "dca632": "Raspberry Pi", "e45f01": "Raspberry Pi",
	"2ccf67": "Raspberry Pi", "d83add": "Raspberry Pi", "88a29e": "Raspberry Pi",
	"001a11": "Google", "3c5ab4": "Google", "f4f5d8": "Google", "a4778b": "Google",
	"6466b3": "Google", "1c53f9": "Amazon", "44650d": "Amazon", "fc65de": "Amazon",
	"68547e": "Amazon", "ac63be": "Amazon",
	"001132": "Synology", "0011d8": "ASUSTek", "1c872c": "ASUSTek", "2c56dc": "ASUSTek",
	"d850e6": "ASUSTek", "704d7b": "ASUSTek",
	"000e08": "Cisco", "00170f": "Cisco", "001a2f": "Cisco", "0025b4": "Cisco",
	"6400f1": "Cisco", "d0c789": "Cisco", "f8a5c5": "Cisco",
	"fcecda": "Ubiquiti", "788a20": "Ubiquiti", "b4fbe4": "Ubiquiti",
	"245a4c": "Ubiquiti", "687251": "Ubiquiti", "74acb9": "Ubiquiti",
	"e063da": "Ubiquiti", "802aa8": "Ubiquiti", "dc9fdb": "Ubiquiti",
	"001478": "TP-Link", "50c7bf": "TP-Link", "a42bb0": "TP-Link", "b0be76": "TP-Link",
	"6466b3f": "TP-Link", "c006c3": "TP-Link", "9c5322": "TP-Link",
	"000fb5": "NETGEAR", "20e52a": "NETGEAR", "a00460": "NETGEAR", "9c3dcf": "NETGEAR",
	"001cf0": "D-Link", "1cbdb9": "D-Link", "34089e": "D-Link",
	"000d88": "D-Link", "b8a386": "D-Link",
	"001d0f": "Mikrotik", "4c5e0c": "Mikrotik", "6c3b6b": "Mikrotik",
	"e48d8c": "Mikrotik", "dc2c6e": "Mikrotik", "748114": "Mikrotik",
	"0017c8": "Kyocera", "001bb1": "Kyocera",
	"0000aa": "Xerox", "9c934e": "Xerox",
	"001b78": "HP", "3464a9": "HP", "a0481c": "HP", "b05ada": "HP", "d48564": "HP",
	"e4e749": "HP", "308d99": "HP", "94574a": "HP",
	"0080a3": "Lantronix", "001321": "Advantech",
	"3c2af4": "Brother", "008077": "Brother", "0080920": "Brother",
	"00000e": "Fujitsu", "000b97": "Fujitsu",
	"3ca82a": "Hewlett Packard Enterprise", "94f128": "Hewlett Packard Enterprise",
	"c81f66": "Dell", "18dbf2": "Dell", "f8bc12": "Dell", "d4be d9": "Dell",
	"b8ca3a": "Dell", "246e96": "Dell", "e4434b": "Dell",
	"0019b9": "Dell", "005043": "Dell",
	"001e8f": "Canon", "00bb c1": "Canon", "88873d": "Canon",
	"0026ab": "Seiko Epson", "a4ee57": "Seiko Epson", "9c5c8e": "Seiko Epson",
	"44d9e7": "Ubiquiti", "0418d6": "Ubiquiti",
	"4c11bf": "Hikvision", "bc ad28": "Hikvision", "c0562d": "Hikvision",
	"28571a": "Hikvision", "44194b": "Hikvision", "58 03fb": "Hikvision",
	"3c ef8c": "Dahua", "4c11bf1": "Dahua", "e0509f": "Dahua", "90 02a9": "Dahua",
	"001217": "Cisco-Linksys", "0018f8": "Cisco-Linksys",
	"00040f": "AVM", "3810d5": "AVM",
	"7c2f80": "Gigaset", "001e2a": "NETGEAR",
	"f0272d": "Apple", "ac bc32": "Apple", "3c0754": "Apple", "40331a": "Apple",
	"a4b197": "Apple", "d0817a": "Apple", "f0dbf8": "Apple", "8866 5a": "Apple",
	"04d3cf": "Apple", "9c04eb": "Apple", "60facd": "Apple", "78 7e61": "Apple",
	"0050f2": "Microsoft", "7c1e52": "Microsoft", "0017fa": "Microsoft",
	"001e c2": "Apple", "b8f6b1": "Apple", "d4909c": "Apple",
	"00248c": "ASRock", "d05099": "ASRock",
	"001485": "Giga-Byte", "1c1b0d": "Giga-Byte", "b42e99": "Giga-Byte",
	"7085c2": "ASRock", "8c8caa": "ASUSTek",
	"5ccf7f": "Espressif (ESP8266/ESP32)", "24 0ac4": "Espressif (ESP8266/ESP32)",
	"a020a6": "Espressif (ESP8266/ESP32)", "3c 6105": "Espressif (ESP8266/ESP32)",
	"7cdfa1": "Espressif (ESP8266/ESP32)", "84 f3eb": "Espressif (ESP8266/ESP32)",
	"ecfabc": "Espressif (ESP8266/ESP32)", "b4e62d": "Espressif (ESP8266/ESP32)",
	"000ec6": "ASIX", "001c42": "Parallels",
	"c8 3a35": "Tenda", "5c f9dd": "Dell", "00 1f16": "Wistron",
	"3c 22fb": "Apple", "10 dd b1": "Apple", "f4 5c89": "Apple",
	"8c 3bad": "NETGEAR", "44 07 0b": "Google", "6c ad f8": "Amazon",
	"00 1a 79": "Sagemcom", "7c 03 4c": "Sagemcom", "00 1d 6a": "Broadcom",
	"48 8f 5a": "Routerboard", "00 90 4c": "Epigram/Broadcom",
	"70 4f 57": "Xiaomi", "64 09 80": "Xiaomi", "8c be be": "Xiaomi",
	"f8 a4 5f": "Xiaomi", "34 ce 00": "Xiaomi", "50 8f 4c": "Xiaomi",
	"00 1e 42": "Teltonika", "20 f8 5e": "ZTE", "00 1e 73": "ZTE",
	"e0 28 6d": "AVM", "5c 49 79": "Huawei", "00 e0 fc": "Huawei",
	"48 46 fb": "Huawei", "80 fb 06": "Huawei", "10 47 80": "Huawei",
	"00 24 e4": "Withings", "b0 c5 54": "D-Link",
	"00 21 5a": "Hewlett Packard", "9c 8e 99": "Hewlett Packard",
	"00 04 4b": "NVIDIA", "48 b0 2d": "NVIDIA",
	"00 1b 21": "Intel", "00 1e 67": "Intel", "3c a9 f4": "Intel",
	"7c 5c f8": "Intel", "94 65 9c": "Intel", "a4 c3 f0": "Intel",
	"e4 a4 71": "Intel", "b4 2e 99": "Giga-Byte", "1c 69 7a": "Elitegroup",
	"00 1c 25": "Hon Hai (Foxconn)", "00 22 68": "Hon Hai (Foxconn)",
	"90 2b 34": "Giga-Byte", "50 e5 49": "Giga-Byte",
	"00 0a f7": "Broadcom", "44 d2 44": "Sonos", "5c aa fd": "Sonos",
	"b8 e9 37": "Sonos", "94 9f 3e": "Sonos", "78 28 ca": "Sonos",
	"18 b4 30": "Nest", "64 16 66": "Nest",
	"00 17 88": "Philips (Hue)", "ec b5 fa": "Philips (Hue)",
	"d0 73 d5": "LIFX", "00 1d c9": "GainSpan", "24 fd 5b": "SmartThings",
	"cc 50 e3": "Espressif (ESP8266/ESP32)", "68 c6 3a": "Espressif (ESP8266/ESP32)",
	"d8 f1 5b": "Espressif (ESP8266/ESP32)", "24 62 ab": "Espressif (ESP8266/ESP32)",
	"00 26 5a": "D-Link", "c4 a8 1d": "Ubiquiti", "04 18 d6": "Ubiquiti",
	"18 e8 29": "Ubiquiti", "f0 9f c2": "Ubiquiti", "24 5a 4c": "Ubiquiti",
	"00 27 22": "Ubiquiti", "80 2a a8 1": "Ubiquiti",
}

var (
	cargaOUI      sync.Once
	fabricantesOI map[string]string
)

// Fabricante devuelve el fabricante de una MAC, o vacio si no se conoce.
func Fabricante(mac string) string {
	cargaOUI.Do(cargarOUI)

	prefijo := normalizarPrefijo(mac)
	if prefijo == "" {
		return ""
	}
	if nombre, hay := fabricantesOI[prefijo]; hay {
		return nombre
	}

	// Las MAC administradas localmente (bit 2 del primer byte) las inventa el
	// propio aparato: las usan los telefonos para no ser rastreados. No hay
	// fabricante que buscar, y decirlo es mas util que dejarlo vacio.
	if esLocalmenteAdministrada(prefijo) {
		return "MAC aleatoria (privacidad)"
	}
	return ""
}

func cargarOUI() {
	fabricantesOI = make(map[string]string, len(fabricantesComunes)+1024)
	for prefijo, nombre := range fabricantesComunes {
		limpio := normalizarPrefijo(prefijo)
		if limpio != "" {
			fabricantesOI[limpio] = nombre
		}
	}

	// Si el equipo tiene la lista oficial de la IEEE, manda sobre la embebida.
	archivo, err := os.Open(rutaOUI)
	if err != nil {
		return
	}
	defer archivo.Close()

	lector := bufio.NewScanner(archivo)
	for lector.Scan() {
		linea := strings.TrimSpace(lector.Text())
		if linea == "" || strings.HasPrefix(linea, "#") {
			continue
		}
		partes := strings.SplitN(linea, "\t", 2)
		if len(partes) != 2 {
			partes = strings.SplitN(linea, " ", 2)
			if len(partes) != 2 {
				continue
			}
		}
		prefijo := normalizarPrefijo(partes[0])
		if prefijo != "" {
			fabricantesOI[prefijo] = strings.TrimSpace(partes[1])
		}
	}
}

// normalizarPrefijo deja los primeros seis caracteres hexadecimales, sin
// separadores: "B8:27:EB:1A:2B:3C" y "b827eb" dan lo mismo.
func normalizarPrefijo(mac string) string {
	var limpio strings.Builder
	for _, letra := range strings.ToLower(mac) {
		if (letra >= '0' && letra <= '9') || (letra >= 'a' && letra <= 'f') {
			limpio.WriteRune(letra)
		}
		if limpio.Len() == 6 {
			break
		}
	}
	if limpio.Len() < 6 {
		return ""
	}
	return limpio.String()
}

func esLocalmenteAdministrada(prefijo string) bool {
	if len(prefijo) < 2 {
		return false
	}
	var primero int
	for _, letra := range prefijo[:2] {
		primero *= 16
		switch {
		case letra >= '0' && letra <= '9':
			primero += int(letra - '0')
		case letra >= 'a' && letra <= 'f':
			primero += int(letra-'a') + 10
		}
	}
	return primero&0x02 != 0
}
