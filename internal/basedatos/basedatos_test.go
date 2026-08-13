package basedatos

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// carpetaDePrueba devuelve un directorio temporal, preferiblemente en memoria.
//
// Estas pruebas comprueban SQL, no que el disco aguante un corte de corriente, y
// cada una crea dos bases con dos docenas de migraciones. Sobre disco eso son
// cientos de sincronizaciones que no prueban nada y que, en un equipo ocupado,
// hacen que abrir una base tarde mas de la cuenta y la prueba falle por algo que
// no tiene que ver con lo que estaba probando. En `/dev/shm` el mismo trabajo no
// toca el disco.
//
// Si no hay memoria compartida —otro sistema, o montada de otra forma— se cae al
// directorio temporal normal: la prueba sigue siendo valida, solo mas lenta.
func carpetaDePrueba(t *testing.T) string {
	t.Helper()

	if info, err := os.Stat("/dev/shm"); err == nil && info.IsDir() {
		carpeta, err := os.MkdirTemp("/dev/shm", "mired-prueba-")
		if err == nil {
			t.Cleanup(func() { os.RemoveAll(carpeta) })
			return carpeta
		}
	}
	return t.TempDir()
}

// enrutadorDePrueba arma un catalogo y una carpeta de redes en un directorio
// temporal que se limpia al terminar.
func enrutadorDePrueba(t *testing.T) *Enrutador {
	t.Helper()

	carpeta := carpetaDePrueba(t)
	enrutador, err := NuevoEnrutador(context.Background(),
		filepath.Join(carpeta, "mired.db"),
		filepath.Join(carpeta, "redes"),
		2, time.Minute)
	if err != nil {
		t.Fatalf("no se pudo armar el enrutador: %v", err)
	}
	t.Cleanup(func() { enrutador.Cerrar() })
	return enrutador
}

func TestCatalogoSeMigraAlAbrir(t *testing.T) {
	enrutador := enrutadorDePrueba(t)

	version, err := enrutador.Catalogo.VersionEsquema(context.Background())
	if err != nil {
		t.Fatalf("no se pudo leer la version del esquema: %v", err)
	}
	if version < 1 {
		t.Fatalf("el catalogo quedo sin migrar (version %d)", version)
	}
}

func TestMigrarDosVecesNoDuplica(t *testing.T) {
	// Migrar es idempotente a proposito: se llama cada vez que se abre una base,
	// incluida una restaurada de un respaldo.
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()

	antes, _ := enrutador.Catalogo.VersionEsquema(ctx)
	if err := enrutador.Catalogo.Migrar(ctx, EsquemaCatalogo); err != nil {
		t.Fatalf("la segunda migracion fallo: %v", err)
	}
	despues, _ := enrutador.Catalogo.VersionEsquema(ctx)

	if antes != despues {
		t.Fatalf("migrar dos veces cambio la version: %d -> %d", antes, despues)
	}
}

func TestCadaRedTieneSuArchivo(t *testing.T) {
	// Es la decision estructural del proyecto: nada de una red debe aparecer en
	// el archivo de otra.
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()

	matriz, err := enrutador.CrearRed(ctx, "Matriz", "Oficina central", []string{"192.168.1.0/24"})
	if err != nil {
		t.Fatalf("no se pudo crear Matriz: %v", err)
	}
	sucursal, err := enrutador.CrearRed(ctx, "Sucursal Ñuñoa", "", []string{"10.20.0.0/16"})
	if err != nil {
		t.Fatalf("no se pudo crear la sucursal: %v", err)
	}

	if matriz.Archivo == sucursal.Archivo {
		t.Fatalf("las dos redes comparten archivo: %s", matriz.Archivo)
	}
	for _, red := range []Red{matriz, sucursal} {
		if _, err := os.Stat(enrutador.ArchivoDeRed(red.Clave)); err != nil {
			t.Fatalf("no se creo el archivo de %s: %v", red.Nombre, err)
		}
	}

	// Lo de una red no se ve desde la otra.
	err = enrutador.ConRed(ctx, sucursal.Clave, func(base *Base) error {
		subredes, err := base.ListarSubredes(ctx)
		if err != nil {
			return err
		}
		if len(subredes) != 1 || subredes[0].CIDR != "10.20.0.0/16" {
			t.Fatalf("la sucursal ve subredes que no son suyas: %+v", subredes)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("no se pudo leer la sucursal: %v", err)
	}
}

func TestClaveDeRedSinAcentosNiEspacios(t *testing.T) {
	enrutador := enrutadorDePrueba(t)

	red, err := enrutador.CrearRed(context.Background(), "Sucursal Ñuñoa 2", "", nil)
	if err != nil {
		t.Fatalf("no se pudo crear la red: %v", err)
	}

	esperado := "sucursal-nunoa-2-"
	if len(red.Clave) <= len(esperado) || red.Clave[:len(esperado)] != esperado {
		t.Fatalf("la clave no se limpio bien: %s", red.Clave)
	}
}

func TestNombreDeRedRepetido(t *testing.T) {
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()

	if _, err := enrutador.CrearRed(ctx, "Matriz", "", nil); err != nil {
		t.Fatalf("no se pudo crear la red: %v", err)
	}
	if _, err := enrutador.CrearRed(ctx, "Matriz", "", nil); !errors.Is(err, ErrNombreRepetido) {
		t.Fatalf("se permitio repetir el nombre: %v", err)
	}
}

func TestRedBorradaSeReactivaConSuArchivo(t *testing.T) {
	// El borrado suave existe justamente para esto: recuperar un sitio borrado
	// por error sin perder su historico.
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()

	original, err := enrutador.CrearRed(ctx, "Bodega", "", []string{"192.168.9.0/24"})
	if err != nil {
		t.Fatalf("no se pudo crear la red: %v", err)
	}
	if err := enrutador.BorrarRed(ctx, original.Clave); err != nil {
		t.Fatalf("no se pudo borrar la red: %v", err)
	}
	if _, err := enrutador.BuscarRed(ctx, original.Clave); !errors.Is(err, ErrRedNoExiste) {
		t.Fatalf("una red borrada no deberia poder abrirse: %v", err)
	}

	revivida, err := enrutador.CrearRed(ctx, "Bodega", "", nil)
	if err != nil {
		t.Fatalf("no se pudo reactivar la red: %v", err)
	}
	if revivida.Clave != original.Clave {
		t.Fatalf("la reactivacion creo un archivo nuevo: %s -> %s", original.Clave, revivida.Clave)
	}

	// Y con su archivo viejo vienen sus subredes.
	err = enrutador.ConRed(ctx, revivida.Clave, func(base *Base) error {
		subredes, err := base.ListarSubredes(ctx)
		if err != nil {
			return err
		}
		if len(subredes) != 1 {
			t.Fatalf("se perdieron las subredes al reactivar: %+v", subredes)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("no se pudo leer la red reactivada: %v", err)
	}
}

func TestRenombrarNoCambiaElArchivo(t *testing.T) {
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()

	original, err := enrutador.CrearRed(ctx, "Matriz", "", nil)
	if err != nil {
		t.Fatalf("no se pudo crear la red: %v", err)
	}

	renombrada, err := enrutador.RenombrarRed(ctx, original.Clave, "Corporativo", "Nuevo nombre")
	if err != nil {
		t.Fatalf("no se pudo renombrar: %v", err)
	}
	if renombrada.Archivo != original.Archivo {
		t.Fatalf("renombrar movio el archivo: %s -> %s", original.Archivo, renombrada.Archivo)
	}

	// La ficha de dentro tambien cambio: el archivo sigue siendo autodescriptivo.
	err = enrutador.ConRed(ctx, original.Clave, func(base *Base) error {
		var nombre string
		if err := base.QueryRowContext(ctx, `SELECT nombre FROM ficha WHERE id = 1`).Scan(&nombre); err != nil {
			return err
		}
		if nombre != "Corporativo" {
			t.Fatalf("la ficha interna quedo con el nombre viejo: %s", nombre)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("no se pudo leer la ficha: %v", err)
	}
}

func TestNormalizarCIDR(t *testing.T) {
	casos := []struct {
		entrada  string
		esperado string
		falla    bool
	}{
		{entrada: "192.168.1.34/24", esperado: "192.168.1.0/24"},
		{entrada: "  10.0.0.0/8  ", esperado: "10.0.0.0/8"},
		{entrada: "172.16.5.77/22", esperado: "172.16.4.0/22"},
		{entrada: "fe80::1/64", esperado: "fe80::/64"},
		{entrada: "192.168.1.999/24", falla: true},
		{entrada: "192.168.1.1", falla: true},
		{entrada: "", falla: true},
	}

	for _, caso := range casos {
		obtenido, err := NormalizarCIDR(caso.entrada)
		if caso.falla {
			if err == nil {
				t.Errorf("%q deberia ser invalida y devolvio %q", caso.entrada, obtenido)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q fallo: %v", caso.entrada, err)
			continue
		}
		if obtenido != caso.esperado {
			t.Errorf("%q dio %q y se esperaba %q", caso.entrada, obtenido, caso.esperado)
		}
	}
}

func TestEnrutadorCierraLasQueSobran(t *testing.T) {
	// El tope de bases abiertas es 2 en las pruebas: al abrir la tercera, la mas
	// vieja sin prestamos vivos tiene que cerrarse sola.
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()

	for _, nombre := range []string{"Uno", "Dos", "Tres"} {
		red, err := enrutador.CrearRed(ctx, nombre, "", nil)
		if err != nil {
			t.Fatalf("no se pudo crear %s: %v", nombre, err)
		}
		// CrearRed presta y devuelve la base, asi que al salir no queda prestamo.
		_ = red
	}

	enrutador.mu.Lock()
	abiertas := len(enrutador.abiertas)
	enrutador.mu.Unlock()

	if abiertas > 2 {
		t.Fatalf("quedaron %d bases abiertas con tope de 2", abiertas)
	}
}

func TestNoSeCierraUnaBasePrestada(t *testing.T) {
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()

	red, err := enrutador.CrearRed(ctx, "Prestada", "", nil)
	if err != nil {
		t.Fatalf("no se pudo crear la red: %v", err)
	}

	base, devolver, err := enrutador.Red(ctx, red.Clave)
	if err != nil {
		t.Fatalf("no se pudo prestar la base: %v", err)
	}
	defer devolver()

	// Aunque se pida cerrar todo lo inactivo, esta sigue prestada.
	enrutador.CerrarInactivas()

	if err := base.PingContext(ctx); err != nil {
		t.Fatalf("se cerro una base que estaba prestada: %v", err)
	}
}

func TestUsuariosYPermisos(t *testing.T) {
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()

	usuario, err := enrutador.CrearUsuario(ctx, "Pedro", "Pedro Ramirez", "hash", "pedro@ejemplo.mx", false)
	if err != nil {
		t.Fatalf("no se pudo crear el usuario: %v", err)
	}
	if usuario.Usuario != "pedro" {
		t.Fatalf("el usuario de acceso deberia guardarse en minusculas: %s", usuario.Usuario)
	}

	if _, err := enrutador.CrearUsuario(ctx, "PEDRO", "Otro", "hash", "", false); !errors.Is(err, ErrUsuarioRepetido) {
		t.Fatalf("se permitio repetir el usuario cambiando mayusculas: %v", err)
	}

	red, err := enrutador.CrearRed(ctx, "Matriz", "", nil)
	if err != nil {
		t.Fatalf("no se pudo crear la red: %v", err)
	}

	permisos, err := enrutador.RedesDeUsuario(ctx, usuario)
	if err != nil {
		t.Fatalf("no se pudieron leer los permisos: %v", err)
	}
	if len(permisos) != 0 {
		t.Fatalf("un usuario nuevo no deberia ver ninguna red: %+v", permisos)
	}

	if err := enrutador.OtorgarPermiso(ctx, usuario.ID, red.Clave, NivelEscritura); err != nil {
		t.Fatalf("no se pudo otorgar el permiso: %v", err)
	}
	permisos, _ = enrutador.RedesDeUsuario(ctx, usuario)
	if permisos[red.Clave] != NivelEscritura {
		t.Fatalf("el permiso no quedo: %+v", permisos)
	}

	// El superadministrador ve todas sin necesidad de fila de permisos.
	jefe, err := enrutador.CrearUsuario(ctx, "jefe", "Jefa", "hash", "", true)
	if err != nil {
		t.Fatalf("no se pudo crear al superadministrador: %v", err)
	}
	permisos, _ = enrutador.RedesDeUsuario(ctx, jefe)
	if permisos[red.Clave] != NivelAdministra {
		t.Fatalf("el superadministrador deberia administrar todas las redes: %+v", permisos)
	}
}

func TestSesionVencidaNoSirve(t *testing.T) {
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()

	usuario, err := enrutador.CrearUsuario(ctx, "ana", "Ana", "hash", "", false)
	if err != nil {
		t.Fatalf("no se pudo crear el usuario: %v", err)
	}

	sesion, err := enrutador.CrearSesion(ctx, usuario.ID, -time.Minute, "127.0.0.1", "prueba")
	if err != nil {
		t.Fatalf("no se pudo crear la sesion: %v", err)
	}
	if _, err := enrutador.BuscarSesion(ctx, sesion.ID, time.Hour); !errors.Is(err, ErrSesionInvalida) {
		t.Fatalf("una sesion vencida no deberia servir: %v", err)
	}
}

func TestAbrirRespetaElContextoDelQueLlama(t *testing.T) {
	// Antes, Abrir se inventaba su propio plazo y se desentendia del contexto.
	// Eso tiene dos consecuencias malas: al apagar el servicio hay que esperar a
	// que termine, y quien tiene su propio presupuesto de tiempo no puede
	// imponerlo.
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()

	archivo := filepath.Join(carpetaDePrueba(t), "cancelada.db")
	base, err := Abrir(ctx, archivo)
	if err == nil {
		base.Close()
		t.Fatal("con el contexto ya cancelado no deberia abrir")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("deberia decir que se cancelo: %v", err)
	}
}

func TestUnaBaseQueTardaDemasiadoLoDiceClaro(t *testing.T) {
	// "No se pudo conectar" a secas manda a buscar un archivo corrupto cuando en
	// realidad el equipo estaba saturado. Son problemas distintos y se arreglan
	// en sitios distintos, asi que el mensaje tiene que separarlos.
	ctx, cancelar := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancelar()

	archivo := filepath.Join(carpetaDePrueba(t), "lenta.db")
	_, err := Abrir(ctx, archivo)
	if err == nil {
		t.Fatal("con un plazo de un nanosegundo no deberia dar tiempo")
	}
	if !strings.Contains(err.Error(), "no contesto en") ||
		!strings.Contains(err.Error(), "saturado") {
		t.Fatalf("el mensaje deberia apuntar a que el equipo esta ocupado: %v", err)
	}
}

func TestUnPlazoPropioDelQueLlamaManda(t *testing.T) {
	// Cuando el contexto ya trae plazo, Abrir NO le pone otro encima: el que
	// llama sabe cuanto tiempo tiene y esto no puede pasarse de ahi.
	ctx, cancelar := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelar()

	plazo, hay := ctx.Deadline()
	if !hay {
		t.Fatal("el contexto de la prueba deberia traer plazo")
	}

	archivo := filepath.Join(carpetaDePrueba(t), "conplazo.db")
	base, err := Abrir(ctx, archivo)
	if err != nil {
		t.Fatalf("deberia abrir sin problema: %v", err)
	}
	defer base.Close()

	// El plazo del contexto no se movio: si Abrir hubiera puesto el suyo, este
	// seria otro.
	nuevoPlazo, _ := ctx.Deadline()
	if !nuevoPlazo.Equal(plazo) {
		t.Fatal("Abrir no deberia cambiar el plazo del que llama")
	}
}

func TestElHistorialDeVersionesSeSiembraSinDuplicar(t *testing.T) {
	// Se siembra en CADA arranque, asi que tiene que poder correrse mil veces
	// sin dejar filas repetidas. Por eso la clave es version+revision y no un
	// identificador nuevo cada vez.
	enrutador := enrutadorDePrueba(t)
	ctx := context.Background()

	sistema := []EntradaDeVersion{
		{Version: "1.14", Revision: 1, Fecha: "2026-08-12", Tipo: "feature",
			Modulo: "Cimientos", Notas: "primera entrega"},
		{Version: "1.14", Revision: 2, Fecha: "2026-08-13", Tipo: "fix",
			Modulo: "Redes", Notas: "una correccion"},
	}
	base := []EntradaDeVersion{
		{Version: "1.14", Revision: 1, Fecha: "2026-08-12", Tipo: "feature",
			Modulo: "Cimientos", Notas: "esquema inicial"},
	}

	for i := 0; i < 3; i++ {
		if err := enrutador.SembrarVersiones(ctx, sistema, base); err != nil {
			t.Fatalf("vuelta %d: %v", i, err)
		}
	}

	leidoSistema, leidoBase, err := enrutador.HistorialDeVersiones(ctx)
	if err != nil {
		t.Fatalf("no se pudo leer el historial: %v", err)
	}
	if len(leidoSistema) != 2 || len(leidoBase) != 1 {
		t.Fatalf("sembrar tres veces duplico filas: %d y %d",
			len(leidoSistema), len(leidoBase))
	}
	// De lo mas nuevo a lo mas viejo: es el orden en que se quiere leer.
	if leidoSistema[0].Revision != 2 {
		t.Fatalf("deberia venir lo mas nuevo primero: %+v", leidoSistema)
	}

	// Y si una nota se corrige en el repo, la siembra la actualiza en vez de
	// dejar la vieja para siempre.
	sistema[1].Notas = "la correccion, mejor explicada"
	if err := enrutador.SembrarVersiones(ctx, sistema, base); err != nil {
		t.Fatalf("no se pudo resembrar: %v", err)
	}
	leidoSistema, _, _ = enrutador.HistorialDeVersiones(ctx)
	if leidoSistema[0].Notas != "la correccion, mejor explicada" {
		t.Fatalf("la nota no se actualizo: %q", leidoSistema[0].Notas)
	}
}
