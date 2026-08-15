package api

import (
	"testing"

	"github.com/tuxormax/mired/internal/basedatos"
)

func TestDeQuienEsLaAntena(t *testing.T) {
	equipos := []basedatos.Equipo{
		{ID: 7, MAC: "f4:92:bf:0b:8c:b6", Alias: "Antena del patio", IP: "192.168.1.86"},
		{ID: 9, MAC: "a0:95:7f:bf:5c:60", Nombre: "modem", IP: "192.168.1.254"},
	}

	// 1) La MAC del radio ES la del equipo: eso es un hecho.
	equipo, comoSeSupo := deQuienEsLaAntena("F4:92:BF:0B:8C:B6", equipos)
	if equipo == nil || equipo.ID != 7 {
		t.Fatalf("no encontro la antena por su propia MAC: %+v", equipo)
	}
	if comoSeSupo != "medido" {
		t.Errorf("una coincidencia exacta es un hecho, no una deduccion: %q", comoSeSupo)
	}

	// 2) El radio suele estar a uno o dos numeros de la MAC del cable. Es una
	// DEDUCCION y tiene que decirse asi: presentarla como medida seria hacer
	// pasar una costumbre de los fabricantes por un dato.
	equipo, comoSeSupo = deQuienEsLaAntena("f4:92:bf:0b:8c:b8", equipos)
	if equipo == nil || equipo.ID != 7 {
		t.Fatalf("no dedujo la antena por cercania: %+v", equipo)
	}
	if comoSeSupo == "medido" {
		t.Error("una MAC cercana NO es lo mismo que la MAC del equipo")
	}

	// 3) Otro fabricante no se deduce por mucho que el numero quede cerca.
	if equipo, _ := deQuienEsLaAntena("aa:bb:cc:0b:8c:b7", equipos); equipo != nil {
		t.Errorf("se atribuyo una antena de otra marca: %+v", equipo)
	}

	// 4) Lo que no se parece a nada, no se atribuye a nadie.
	if equipo, _ := deQuienEsLaAntena("f4:92:bf:99:99:99", equipos); equipo != nil {
		t.Errorf("se atribuyo una antena lejana: %+v", equipo)
	}

	// 5) Una MAC ilegible no puede tumbar nada ni inventar duenos.
	if equipo, _ := deQuienEsLaAntena("no-es-una-mac", equipos); equipo != nil {
		t.Error("una MAC ilegible no puede tener dueno")
	}
}

func TestElNombreDeLaAntenaSaleDeLoQuePusoUnaPersona(t *testing.T) {
	// El alias que teclea alguien manda sobre el nombre descubierto, y la IP es
	// el ultimo recurso: es el mismo orden que en toda la interfaz.
	if nombre := nombreDeEquipo(basedatos.Equipo{
		Alias: "Antena del patio", Nombre: "ubnt", IP: "192.168.1.86",
	}); nombre != "Antena del patio" {
		t.Errorf("gano %q sobre el alias", nombre)
	}
	if nombre := nombreDeEquipo(basedatos.Equipo{Nombre: "ubnt", IP: "192.168.1.86"}); nombre != "ubnt" {
		t.Errorf("sin alias deberia salir el nombre descubierto: %q", nombre)
	}
	if nombre := nombreDeEquipo(basedatos.Equipo{IP: "192.168.1.86"}); nombre != "192.168.1.86" {
		t.Errorf("sin nombre queda la IP: %q", nombre)
	}
}
