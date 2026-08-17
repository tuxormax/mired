package basedatos

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tuxormax/mired/internal/importacion"
	"github.com/tuxormax/mired/internal/secreto"
)

// La hoja de la clinica, ya con las dos columnas que le faltaban. Es el caso que
// justifica todo esto: 24 aparatos capturados a mano son 24 formularios.
const hojaDeLaClinica = `RED ALAMBRICA CLINICA ESQUIPULAS,,,,,,
,,,,,,
NOMBRE,QUE_ES,PUERTOS,CUELGA_DE,PUERTO,UBICACION,NOTAS
switch site,switch,24,,,site,
router,modem,4,switch site,1,site,
serv1,servidor,,switch site,2,site,servidor 1
D01,pc,,switch site,5,admon,
D06,otro,,switch site,10,dental,fortinet lan 4
D15,pc,,switch site,19,farmacia,se cambio jack
D19,otro,,router,2,usg,antena deco
`

func planDe(t *testing.T, base *Base, texto string) importacion.Plan {
	t.Helper()
	hoja, err := importacion.Leer([]byte(texto))
	if err != nil {
		t.Fatalf("no se pudo leer la hoja: %v", err)
	}
	existentes, err := base.NombresDeEquipos(context.Background())
	if err != nil {
		t.Fatalf("no se pudieron leer los nombres: %v", err)
	}
	return importacion.Planear(hoja, existentes)
}

func TestImportarLaHojaDeUnaClinicaDejaElMapaArmado(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	resumen, err := base.AplicarImportacion(ctx, nil,
		planDe(t, base, hojaDeLaClinica), importacion.RepetidosActualizar)
	if err != nil {
		t.Fatalf("no se pudo importar: %v", err)
	}

	if resumen.Creados != 7 {
		t.Fatalf("se esperaban 7 aparatos y se crearon %d", resumen.Creados)
	}
	// Los 24 del switch mas los 4 del router. El puerto 2 del router ya estaba
	// entre esos cuatro, asi que no se declara dos veces.
	if resumen.Puertos != 28 {
		t.Fatalf("se esperaban 28 puertos declarados y hay %d", resumen.Puertos)
	}
	if resumen.Cables != 6 {
		t.Fatalf("se esperaban 6 cables y hay %d", resumen.Cables)
	}

	topologia, err := base.LeerTopologiaManual(ctx)
	if err != nil {
		t.Fatalf("no se pudo leer la topologia: %v", err)
	}
	if len(topologia.Enlaces) != 6 {
		t.Fatalf("la topologia tiene %d cables", len(topologia.Enlaces))
	}
	// Y quedan puertos LIBRES: 24 menos los cinco usados. Es la mitad de la
	// utilidad de la hoja cuando hay que conectar algo nuevo.
	libres := 0
	for _, puerto := range topologia.Puertos {
		ocupado := false
		for _, enlace := range topologia.Enlaces {
			if enlace.PuertoOrigenID == puerto.ID {
				ocupado = true
				break
			}
		}
		if !ocupado {
			libres++
		}
	}
	if libres != 22 {
		t.Fatalf("se esperaban 22 puertos libres y hay %d", libres)
	}
}

func TestLaUbicacionSeGuardaYSeLee(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	if _, err := base.AplicarImportacion(ctx, nil,
		planDe(t, base, hojaDeLaClinica), importacion.RepetidosActualizar); err != nil {
		t.Fatalf("no se pudo importar: %v", err)
	}

	equipos, err := base.ListarEquipos(ctx, false)
	if err != nil {
		t.Fatalf("no se pudieron listar: %v", err)
	}
	porNombre := map[string]Equipo{}
	for _, equipo := range equipos {
		porNombre[equipo.ComoSeLlama()] = equipo
	}

	// La ubicacion es el dato por el que se busca en una instalacion con rosetas:
	// «que hay en farmacia». Antes solo cabia en las notas, revuelta con «se
	// cambio jack».
	if porNombre["D15"].Ubicacion != "farmacia" {
		t.Fatalf("no se guardo la ubicacion: %q", porNombre["D15"].Ubicacion)
	}
	if porNombre["D15"].Notas != "se cambio jack" {
		t.Fatalf("la nota tenia que quedar aparte: %q", porNombre["D15"].Notas)
	}
	if porNombre["D06"].Categoria != "otro" {
		t.Fatalf("la categoria no se guardo: %q", porNombre["D06"].Categoria)
	}
}

func TestVolverASubirLaHojaNoDuplicaNada(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	if _, err := base.AplicarImportacion(ctx, nil,
		planDe(t, base, hojaDeLaClinica), importacion.RepetidosActualizar); err != nil {
		t.Fatalf("no se pudo importar: %v", err)
	}

	// La misma hoja con una ubicacion corregida. Es lo que pasa de verdad:
	// alguien arregla un dato en la hoja y la vuelve a subir.
	corregida := strings.Replace(hojaDeLaClinica,
		"D15,pc,,switch site,19,farmacia,", "D15,pc,,switch site,19,farmacia 2,", 1)

	resumen, err := base.AplicarImportacion(ctx, nil,
		planDe(t, base, corregida), importacion.RepetidosActualizar)
	if err != nil {
		t.Fatalf("no se pudo reimportar: %v", err)
	}
	if resumen.Creados != 0 || resumen.Actualizados != 7 {
		t.Fatalf("la segunda vez no se crea nada: %+v", resumen)
	}

	equipos, _ := base.ListarEquipos(ctx, false)
	if len(equipos) != 7 {
		t.Fatalf("se duplicaron aparatos: hay %d", len(equipos))
	}
	topologia, _ := base.LeerTopologiaManual(ctx)
	if len(topologia.Enlaces) != 6 {
		t.Fatalf("se duplicaron cables: hay %d", len(topologia.Enlaces))
	}
	for _, equipo := range equipos {
		if equipo.ComoSeLlama() == "D15" && equipo.Ubicacion != "farmacia 2" {
			t.Fatalf("no se actualizo la ubicacion: %q", equipo.Ubicacion)
		}
	}
}

func TestSaltarDejaComoEstabaLoQueYaHabia(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	if _, err := base.AplicarImportacion(ctx, nil,
		planDe(t, base, hojaDeLaClinica), importacion.RepetidosActualizar); err != nil {
		t.Fatalf("no se pudo importar: %v", err)
	}

	corregida := strings.Replace(hojaDeLaClinica, "farmacia", "otro sitio", 1)
	resumen, err := base.AplicarImportacion(ctx, nil,
		planDe(t, base, corregida), importacion.RepetidosSaltar)
	if err != nil {
		t.Fatalf("no se pudo reimportar: %v", err)
	}
	if resumen.Saltados != 7 || resumen.Actualizados != 0 {
		t.Fatalf("con «saltar» no se toca nada: %+v", resumen)
	}

	equipos, _ := base.ListarEquipos(ctx, false)
	for _, equipo := range equipos {
		if equipo.ComoSeLlama() == "D15" && equipo.Ubicacion != "farmacia" {
			t.Fatalf("se piso un dato que se habia mandado saltar: %q", equipo.Ubicacion)
		}
	}
}

func TestUnaCeldaVaciaNoBorraLoQueYaEstaba(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	primera := "NOMBRE,QUE_ES,UBICACION,MODELO\nD01,pc,farmacia,Dell 3020\n"
	if _, err := base.AplicarImportacion(ctx, nil,
		planDe(t, base, primera), importacion.RepetidosActualizar); err != nil {
		t.Fatalf("no se pudo importar: %v", err)
	}

	// Una celda en blanco significa «no lo se», no «borralo»: quien deja MODELO
	// vacio no esta pidiendo que se borre lo que alguien tecleo hace un mes.
	segunda := "NOMBRE,QUE_ES,UBICACION,MODELO\nD01,pc,dental,\n"
	if _, err := base.AplicarImportacion(ctx, nil,
		planDe(t, base, segunda), importacion.RepetidosActualizar); err != nil {
		t.Fatalf("no se pudo reimportar: %v", err)
	}

	equipos, _ := base.ListarEquipos(ctx, false)
	if equipos[0].Modelo != "Dell 3020" {
		t.Fatalf("una celda vacia borro el modelo: %q", equipos[0].Modelo)
	}
	if equipos[0].Ubicacion != "dental" {
		t.Fatalf("lo que si venia lleno tenia que actualizarse: %q", equipos[0].Ubicacion)
	}
}

func TestLaClaveDelArchivoSeGuardaCifrada(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()
	caja := secreto.NuevaCaja(filepath.Join(t.TempDir(), "llave"))

	hoja := "NOMBRE,QUE_ES,IP,USUARIO,CLAVE\nswitch site,switch,192.168.1.2,admin,laclave\n"
	resumen, err := base.AplicarImportacion(ctx, caja, planDe(t, base, hoja),
		importacion.RepetidosActualizar)
	if err != nil {
		t.Fatalf("no se pudo importar: %v", err)
	}
	if resumen.Credenciales != 1 {
		t.Fatalf("no se guardo el acceso: %+v", resumen)
	}

	// En la base NUNCA hay una contrasena en claro, venga de donde venga.
	var guardada string
	if err := base.QueryRowContext(ctx,
		`SELECT clave FROM credenciales_equipo`).Scan(&guardada); err != nil {
		t.Fatalf("no se pudo leer la credencial: %v", err)
	}
	if guardada == "laclave" || guardada == "" {
		t.Fatalf("la clave tenia que quedar cifrada: %q", guardada)
	}
	claro, err := caja.Descifrar(guardada)
	if err != nil || claro != "laclave" {
		t.Fatalf("la clave cifrada no se puede recuperar: %q, %v", claro, err)
	}
}

func TestUnCableQueCambiaDePuertoPisaAlAnteriorYSeAvisa(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	primera := "NOMBRE,QUE_ES,PUERTOS,CUELGA_DE,PUERTO\nsw,switch,8,,\nD01,pc,,sw,3\n"
	if _, err := base.AplicarImportacion(ctx, nil,
		planDe(t, base, primera), importacion.RepetidosActualizar); err != nil {
		t.Fatalf("no se pudo importar: %v", err)
	}

	// Se movio el cable del puerto 3 al 6. Manda el archivo: es lo que alguien
	// acaba de mirar con el cable en la mano.
	segunda := "NOMBRE,QUE_ES,PUERTOS,CUELGA_DE,PUERTO\nsw,switch,8,,\nD01,pc,,sw,6\n"
	resumen, err := base.AplicarImportacion(ctx, nil,
		planDe(t, base, segunda), importacion.RepetidosActualizar)
	if err != nil {
		t.Fatalf("no se pudo reimportar: %v", err)
	}

	topologia, _ := base.LeerTopologiaManual(ctx)
	if len(topologia.Enlaces) != 1 {
		t.Fatalf("el aparato quedo con %d cables de subida", len(topologia.Enlaces))
	}
	if topologia.Enlaces[0].NumeroOrigen != 6 {
		t.Fatalf("el cable tenia que quedar en el puerto 6: %d",
			topologia.Enlaces[0].NumeroOrigen)
	}
	_ = resumen
}

func TestSiUnRenglonRevientaNoSeGuardaNingunOtro(t *testing.T) {
	_, base, devolver := conRedDePrueba(t)
	defer devolver()
	ctx := context.Background()

	// Tres renglones buenos y, al final, uno que no se puede escribir: trae clave
	// y no hay con que cifrarla. Importar 23 aparatos y que falle el numero 18
	// dejaria media hoja dentro y la otra media fuera, sin forma de saber por
	// donde iba. O entra el archivo entero, o no entra nada.
	plan := planDe(t, base,
		"NOMBRE,QUE_ES,CLAVE\nuno,pc,\ndos,pc,\ntres,pc,\ncuatro,pc,secreta\n")

	if _, err := base.AplicarImportacion(ctx, nil, plan,
		importacion.RepetidosActualizar); err == nil {
		t.Fatal("tenia que fallar: no hay caja con que cifrar la clave")
	}

	equipos, err := base.ListarEquipos(ctx, false)
	if err != nil {
		t.Fatalf("no se pudieron listar: %v", err)
	}
	if len(equipos) != 0 {
		t.Fatalf("la transaccion no revirtio: quedaron %d aparatos", len(equipos))
	}
}
