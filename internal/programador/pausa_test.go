package programador

import (
	"testing"
	"time"
)

func TestLaAgendaSePausaYSeReanuda(t *testing.T) {
	servicio := &Servicio{}

	if servicio.EnPausa("casa") {
		t.Fatal("una red recien vista no puede estar pausada")
	}

	servicio.PausarAgenda("casa", 5*time.Minute)
	if !servicio.EnPausa("casa") {
		t.Fatal("deberia quedar pausada")
	}
	// Pausar una red no calla a las demas.
	if servicio.EnPausa("oficina") {
		t.Fatal("se pauso una red que nadie pidio")
	}

	servicio.ReanudarAgenda("casa")
	if servicio.EnPausa("casa") {
		t.Fatal("deberia haberse reanudado")
	}
}

func TestLaPausaVenceSola(t *testing.T) {
	// Si el programa se cierra de golpe —o alguien se deja el mapa abierto y se
	// va— la red no puede quedarse sin vigilancia para siempre por un aviso que
	// nadie retiro.
	servicio := &Servicio{}
	servicio.PausarAgenda("casa", time.Millisecond)

	time.Sleep(5 * time.Millisecond)
	if servicio.EnPausa("casa") {
		t.Fatal("la pausa tenia que haber vencido sola")
	}
}

func TestNadiePausaUnaRedMedioDia(t *testing.T) {
	servicio := &Servicio{}

	hasta := servicio.PausarAgenda("casa", 12*time.Hour)
	if hasta.After(time.Now().Add(topeDePausa + time.Minute)) {
		t.Fatalf("la pausa se acepto hasta %s: pasa del tope", hasta)
	}

	// Y sin decir cuanto, tambien queda acotada.
	hasta = servicio.PausarAgenda("otra", 0)
	if hasta.After(time.Now().Add(topeDePausa + time.Minute)) {
		t.Fatalf("una pausa sin plazo quedo hasta %s", hasta)
	}
}
