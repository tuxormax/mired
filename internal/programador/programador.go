// Paquete programador corre los barridos de red: los que pide una persona desde
// la interfaz y los que van solos por agenda.
//
// Los dos caminos pasan por aqui a proposito. Si el boton de la interfaz y el
// reloj del sistema lanzaran barridos por su cuenta, nada impediria que corran
// dos a la vez sobre la misma red, y al marcar los equipos ausentes se pisarian:
// el resultado seria una red donde los equipos parpadean entre presente y
// ausente sin razon.
package programador

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/tuxormax/mired/internal/avisos"
	"github.com/tuxormax/mired/internal/basedatos"
	"github.com/tuxormax/mired/internal/catalogo"
	"github.com/tuxormax/mired/internal/snmp"
	"github.com/tuxormax/mired/internal/sonda"
)

// Errores que la capa de arriba distingue para responder con el mensaje justo.
var (
	// ErrYaEnCurso es que esa red ya se esta escaneando.
	ErrYaEnCurso = errors.New("ya hay un escaneo en curso en esta red")
	// ErrSinSubredes es que no hay nada que barrer y tampoco se pudo preguntar.
	ErrSinSubredes = errors.New("no hay nada que escanear en esta red")
	// ErrEquipoSinRed es que este equipo no esta conectado a ninguna red.
	//
	// Va aparte de ErrSinSubredes porque **son problemas distintos y se arreglan
	// en sitios distintos**: uno se arregla configurando MiRed, el otro
	// enchufando un cable o conectandose al WiFi. Darles el mismo mensaje manda a
	// buscar donde no hay nada.
	ErrEquipoSinRed = errors.New("este equipo no esta conectado a ninguna red")
)

// esperaEscaneo es lo que se le da a la sonda para terminar un barrido antes de
// darlo por perdido. Un /24 con puertos tarda minutos, no segundos.
const esperaEscaneo = 20 * time.Minute

// Servicio lanza y vigila los barridos.
type Servicio struct {
	Datos       *basedatos.Enrutador
	SocketSonda string
	Bitacora    *slog.Logger
	// Catalogo reconoce que es cada aparato. Puede ser nil: sin catalogo el
	// servicio funciona igual, solo que los equipos quedan sin tipo.
	Catalogo *catalogo.Catalogo

	mu         sync.Mutex
	enCurso    map[string]bool
	mapaFlujos *mapaDeSubredes
}

// Nuevo arma el servicio.
func Nuevo(datos *basedatos.Enrutador, socketSonda string, bitacora *slog.Logger) *Servicio {
	return &Servicio{
		Datos:       datos,
		SocketSonda: socketSonda,
		Bitacora:    bitacora,
		enCurso:     map[string]bool{},
	}
}

// EnCurso dice si esa red se esta escaneando ahora mismo.
func (s *Servicio) EnCurso(clave string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enCurso[clave]
}

func (s *Servicio) apartar(clave string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.enCurso[clave] {
		return false
	}
	s.enCurso[clave] = true
	return true
}

func (s *Servicio) liberar(clave string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.enCurso, clave)
}

// Lanzar arranca un barrido y devuelve el identificador del escaneo. No espera a
// que termine: un barrido tarda minutos y dejar la peticion HTTP abierta todo
// ese rato serviria para que cualquier corte la mate sin que nadie sepa si quedo.
func (s *Servicio) Lanzar(ctx context.Context, clave string, soloPresencia bool) (int64, []string, error) {
	subredes, err := s.subredesDe(ctx, clave)
	if err != nil {
		return 0, nil, err
	}
	// Un sitio recien creado no tiene nada configurado: se adopta la red de este
	// equipo, que es lo que el usuario quiere decir con "escanear toda la red".
	// No se le pregunta —el programa ya lo sabe— pero SI se dice, porque en un
	// sitio remoto la red de este equipo no seria la correcta.
	if len(subredes) == 0 {
		adoptadas, err := s.adoptarLaRedDetectada(ctx, clave)
		if err != nil {
			s.Bitacora.Warn("no se pudo adoptar la red de este equipo",
				"red", clave, "error", err)
			return 0, nil, err
		}
		subredes = adoptadas
	}

	if !s.apartar(clave) {
		return 0, nil, ErrYaEnCurso
	}

	tipo := basedatos.TipoProfundo
	if soloPresencia {
		tipo = basedatos.TipoPresencia
	}

	var escaneoID int64
	err = s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
		var err error
		escaneoID, err = base.IniciarEscaneo(ctx, tipo)
		return err
	})
	if err != nil {
		s.liberar(clave)
		return 0, nil, err
	}

	go s.correr(clave, escaneoID, subredes, soloPresencia)
	return escaneoID, subredes, nil
}

func (s *Servicio) subredesDe(ctx context.Context, clave string) ([]string, error) {
	var subredes []string
	err := s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
		lista, err := base.ListarSubredes(ctx)
		if err != nil {
			return err
		}
		for _, subred := range lista {
			if subred.Escanear {
				subredes = append(subredes, subred.CIDR)
			}
		}
		return nil
	})
	return subredes, err
}

// correr le pide el barrido a la sonda y guarda lo que traiga.
func (s *Servicio) correr(clave string, escaneoID int64, subredes []string, soloPresencia bool) {
	defer s.liberar(clave)

	// Contexto propio: la peticion que lo lanzo ya termino hace rato, y el
	// programador no tiene ninguna.
	ctx, cancelar := context.WithTimeout(context.Background(), esperaEscaneo+time.Minute)
	defer cancelar()

	resultado, err := sonda.PedirEscaneo(s.SocketSonda, sonda.PeticionEscaneo{
		Subredes:      subredes,
		SoloPresencia: soloPresencia,
	}, esperaEscaneo)
	if err != nil {
		s.Bitacora.Error("el escaneo fallo", "red", clave, "escaneo", escaneoID, "error", err)
		s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
			return base.FallarEscaneo(ctx, escaneoID, err.Error())
		})
		return
	}
	for _, advertencia := range resultado.Advertencias {
		s.Bitacora.Warn("aviso del escaneo", "red", clave, "aviso", advertencia)
	}

	equipos := convertir(resultado.Equipos)

	err = s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
		resumen, err := base.GuardarDescubrimiento(ctx, escaneoID, !soloPresencia, equipos)
		if err != nil {
			return err
		}
		s.Bitacora.Info("escaneo terminado", "red", clave, "vistos", resumen.Vistos,
			"nuevos", resumen.Nuevos, "ausentes", resumen.Ausentes, "ms", resultado.DuracionMs)

		// El resumen del catalogo se actualiza al terminar cada barrido: es lo
		// que permite que el panel de inicio no abra el archivo de cada red.
		total, presentes, ultimo, err := base.ResumenDeRed(ctx)
		if err != nil {
			return err
		}
		return s.Datos.ActualizarResumen(ctx, clave, total, presentes, ultimo)
	})

	// El interrogatorio SNMP va DESPUES del barrido y solo en el profundo: solo
	// tiene sentido preguntarle a equipos que ya se sabe que existen, y es lo
	// que da el mapa de puertos.
	if err == nil && !soloPresencia {
		s.consultarSNMP(ctx, clave, resultado.Equipos)
		// Las controladoras WiFi van DESPUES de SNMP: lo que dicen se guarda por
		// el mismo camino, y asi el punto de acceso ya existe como equipo con sus
		// bocas antes de colgarle los equipos inalambricos.
		s.consultarControladoras(ctx, clave)
		s.reconocer(ctx, clave)
	}
	// Las alertas se revisan tambien tras un barrido de presencia: enterarse de
	// que se conecto algo desconocido es justo lo que no puede esperar al
	// escaneo profundo de dentro de seis horas.
	if err == nil {
		s.revisarAlertas(ctx, clave, nombreRed(s, ctx, clave), escaneoID)
	}
	if err != nil {
		s.Bitacora.Error("no se pudo guardar el escaneo", "red", clave, "error", err)
		s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
			return base.FallarEscaneo(ctx, escaneoID, err.Error())
		})
	}
}

func convertir(vistos []sonda.EquipoVisto) []basedatos.EquipoDescubierto {
	equipos := make([]basedatos.EquipoDescubierto, 0, len(vistos))
	for _, visto := range vistos {
		puertos := make([]basedatos.PuertoDescubierto, 0, len(visto.Puertos))
		for _, puerto := range visto.Puertos {
			puertos = append(puertos, basedatos.PuertoDescubierto{
				Numero:    puerto.Numero,
				Protocolo: puerto.Protocolo,
				Servicio:  puerto.Servicio,
				Banner:    puerto.Banner,
			})
		}
		equipos = append(equipos, basedatos.EquipoDescubierto{
			IP:         visto.IP,
			MAC:        visto.MAC,
			Nombre:     visto.Nombre,
			Fabricante: visto.Fabricante,
			Metodo:     visto.Metodo,
			Subred:     visto.Subred,
			Puertos:    puertos,
		})
	}
	return equipos
}

// Vigilar corre la agenda hasta que se cancele el contexto.
//
// Revisa cada pocos segundos, no cada segundo: la agenda mas apretada que se
// puede configurar es de quince segundos, y preguntar mas seguido que eso solo
// gasta.
func (s *Servicio) Vigilar(ctx context.Context) {
	reloj := time.NewTicker(10 * time.Second)
	defer reloj.Stop()

	// La comprobacion de "redes que dejaron de reportar" va mucho mas espaciada:
	// implica abrir el archivo de cada red programada, y preguntarlo cada diez
	// segundos seria trabajo constante para contestar casi siempre que no.
	vueltas := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-reloj.C:
			s.revisarAgenda(ctx)
			vueltas++
			if vueltas%30 == 0 { // cada cinco minutos
				s.revisarRedesCaidas(ctx)
			}
		}
	}
}

// revisarRedesCaidas avisa de las redes programadas que llevan demasiado sin un
// escaneo terminado.
//
// Es la unica alerta que no nace de un escaneo, por definicion: nace de que no
// hubo ninguno. Si la sonda se cayo o el sitio quedo incomunicado, esta es la
// unica forma de enterarse.
func (s *Servicio) revisarRedesCaidas(ctx context.Context) {
	redes, err := s.Datos.ListarRedes(ctx)
	if err != nil {
		s.Bitacora.Warn("no se pudieron listar las redes para revisar caidas", "error", err)
		return
	}

	for _, red := range redes {
		if !red.Programado || s.EnCurso(red.Clave) {
			continue
		}

		clave := red.Clave
		nombre := red.Nombre
		err := s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
			nuevas, err := base.AlertaSiDejoDeReportar(ctx)
			if err != nil || len(nuevas) == 0 {
				return err
			}
			s.Bitacora.Warn("una red dejo de reportar", "red", clave)

			abiertas, err := base.ContarAlertasAbiertas(ctx)
			if err != nil {
				return err
			}
			return s.Datos.ActualizarAlertasEnCatalogo(ctx, clave, abiertas)
		})
		if err != nil {
			s.Bitacora.Warn("no se pudo revisar si la red reporta", "red", clave, "error", err)
			continue
		}

		destinos, err := s.destinosDe(ctx, clave)
		if err == nil && len(destinos) > 0 {
			s.enviarAlertas(ctx, clave, nombre, destinos)
		}
	}
}

func (s *Servicio) destinosDe(ctx context.Context, clave string) ([]basedatos.DestinoAlerta, error) {
	var destinos []basedatos.DestinoAlerta
	err := s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
		var err error
		destinos, err = base.ListarDestinos(ctx)
		return err
	})
	return destinos, err
}

func (s *Servicio) revisarAgenda(ctx context.Context) {
	tareas, err := s.Datos.TareasPendientes(ctx)
	if err != nil {
		s.Bitacora.Warn("no se pudo revisar la agenda", "error", err)
		return
	}

	for _, tarea := range tareas {
		// La agenda se corre ANTES de lanzar el barrido, no despues. Si se
		// corriera al terminar, un barrido que tarda mas que su intervalo
		// dispararia el siguiente apenas acabe, y la red quedaria escaneandose
		// sin parar.
		if err := s.Datos.ProgramarSiguiente(ctx, tarea.Clave, tarea.Tipo); err != nil {
			s.Bitacora.Warn("no se pudo recorrer la agenda", "red", tarea.Clave, "error", err)
			continue
		}

		_, _, err := s.Lanzar(ctx, tarea.Clave, tarea.Tipo == basedatos.TipoPresencia)
		switch {
		case errors.Is(err, ErrYaEnCurso):
			// El barrido anterior todavia corre: se salta este turno sin ruido.
			// Pasa cuando el intervalo quedo mas corto que lo que tarda la red.
			s.Bitacora.Debug("se salto un barrido programado porque el anterior sigue",
				"red", tarea.Clave)
		case errors.Is(err, ErrSinSubredes):
			s.Bitacora.Warn("red programada sin subredes que escanear", "red", tarea.Clave)
		case err != nil:
			s.Bitacora.Error("no se pudo lanzar el barrido programado",
				"red", tarea.Clave, "error", fmt.Sprintf("%v", err))
		default:
			s.Bitacora.Debug("barrido programado lanzado", "red", tarea.Clave, "tipo", tarea.Tipo)
		}
	}
}

// consultarSNMP le pregunta por SNMP a los equipos recien descubiertos y guarda
// el mapa de puertos que salga de ahi.
//
// Que un equipo no conteste es lo normal: solo los administrables hablan SNMP.
// Por eso esto nunca marca el escaneo como fallido — el barrido ya termino bien,
// y esto es informacion adicional que puede haber o no.
func (s *Servicio) consultarSNMP(ctx context.Context, clave string, vistos []sonda.EquipoVisto) {
	credenciales, err := s.Datos.ListarCredencialesSNMP(ctx)
	if err != nil {
		s.Bitacora.Warn("no se pudieron leer las credenciales SNMP", "error", err)
		return
	}
	if len(credenciales) == 0 {
		// Sin credenciales no hay nada que preguntar. No es un error: es una red
		// que todavia no tiene configurado el acceso a sus switches.
		return
	}

	destinos := make([]string, 0, len(vistos))
	for _, equipo := range vistos {
		destinos = append(destinos, equipo.IP)
	}
	if len(destinos) == 0 {
		return
	}

	peticion := sonda.PeticionSNMP{Destinos: destinos, EsperaMs: 2000}
	for _, credencial := range credenciales {
		peticion.Credenciales = append(peticion.Credenciales, snmp.Credencial{
			Nombre:                 credencial.Nombre,
			Version:                credencial.Version,
			Comunidad:              credencial.Comunidad,
			Usuario:                credencial.Usuario,
			AutenticacionProtocolo: credencial.AutenticacionProtocolo,
			AutenticacionClave:     credencial.AutenticacionClave,
			PrivacidadProtocolo:    credencial.PrivacidadProtocolo,
			PrivacidadClave:        credencial.PrivacidadClave,
		})
	}

	resultado, err := sonda.PedirSNMP(s.SocketSonda, peticion, esperaEscaneo)
	if err != nil {
		s.Bitacora.Warn("no se pudo consultar SNMP", "red", clave, "error", err)
		return
	}
	for _, advertencia := range resultado.Advertencias {
		s.Bitacora.Warn("aviso de SNMP", "red", clave, "aviso", advertencia)
	}
	if len(resultado.Fichas) == 0 {
		s.Bitacora.Info("ningun equipo contesto SNMP", "red", clave,
			"consultados", resultado.Consultados)
		// Que nadie conteste ES una respuesta: esta red no tiene switches
		// administrables al alcance. Se anota, para que la interfaz lo explique
		// en vez de dejar la pantalla en "todavia no se ha consultado" para
		// siempre.
		err := s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
			_, err := base.CalcularCapacidades(ctx)
			return err
		})
		if err != nil {
			s.Bitacora.Warn("no se pudo anotar el perfil de capacidades", "red", clave, "error", err)
		}
		return
	}

	fichas := make([]basedatos.FichaSNMP, 0, len(resultado.Fichas))
	// Los contadores de trafico van aparte de la ficha: son una MEDICION del
	// momento, no una descripcion del equipo, y por eso se acumulan en su propia
	// tabla en vez de sobrescribirse.
	porIP := map[string][]basedatos.ContadorPuerto{}
	for _, ficha := range resultado.Fichas {
		interfaces := make([]basedatos.InterfazSNMP, 0, len(ficha.Interfaces))
		for _, puerto := range ficha.Interfaces {
			interfaces = append(interfaces, basedatos.InterfazSNMP{
				Indice:        puerto.Indice,
				Nombre:        puerto.Nombre,
				Descripcion:   puerto.Descripcion,
				Alias:         puerto.Alias,
				MAC:           puerto.MAC,
				Tipo:          puerto.Tipo,
				Activa:        puerto.Activa,
				VelocidadMbps: puerto.VelocidadMbps,
			})
		}
		vecinos := make([]basedatos.VecinoSNMP, 0, len(ficha.Vecinos))
		for _, vecino := range ficha.Vecinos {
			vecinos = append(vecinos, basedatos.VecinoSNMP{
				InterfazLocal: vecino.InterfazLocal,
				Nombre:        vecino.Nombre,
				Descripcion:   vecino.Descripcion,
				PuertoRemoto:  vecino.PuertoRemoto,
				ChasisID:      vecino.ChasisID,
				Origen:        vecino.Origen,
				DireccionIP:   vecino.DireccionIP,
			})
		}
		contadores := make([]basedatos.ContadorPuerto, 0, len(ficha.Contadores))
		for indice, contador := range ficha.Contadores {
			contadores = append(contadores, basedatos.ContadorPuerto{
				Indice:         indice,
				Entrada:        contador.Entrada,
				Salida:         contador.Salida,
				SesentaYCuatro: contador.SesentaYCuatro,
			})
		}
		porIP[ficha.IP] = contadores

		fichas = append(fichas, basedatos.FichaSNMP{
			IP:            ficha.IP,
			Nombre:        ficha.Nombre,
			Descripcion:   ficha.Descripcion,
			Contacto:      ficha.Contacto,
			Ubicacion:     ficha.Ubicacion,
			ObjectID:      ficha.ObjectID,
			EncendidoMs:   ficha.EncendidoMs,
			EsSwitch:      ficha.EsSwitch,
			Credencial:    ficha.Credencial,
			Interfaces:    interfaces,
			MacsPorPuerto: ficha.MacsPorPuerto,
			Vecinos:       vecinos,
		})
	}

	err = s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
		movimientos, err := base.GuardarSNMP(ctx, fichas)
		if err != nil {
			return err
		}
		// Que un equipo se cambie de boca es un hecho que solo se ve aqui, y en
		// una red con puertos documentados es justo lo que interesa saber.
		if len(movimientos) > 0 {
			if err := base.AlertasDeMovimiento(ctx, movimientos); err != nil {
				return err
			}
			s.Bitacora.Info("equipos que cambiaron de puerto", "red", clave,
				"cuantos", len(movimientos))
		}
		for ip, contadores := range porIP {
			if err := base.GuardarTrafico(ctx, ip, contadores); err != nil {
				return err
			}
		}
		// Las muestras viejas se podan aqui: sin esto la tabla crece para
		// siempre, y en una Raspberry eso importa.
		if err := base.PodarTrafico(ctx, 90); err != nil {
			s.Bitacora.Warn("no se pudo podar el trafico viejo", "red", clave, "error", err)
		}

		capacidad, err := base.CalcularCapacidades(ctx)
		if err != nil {
			return err
		}
		s.Bitacora.Info("SNMP guardado", "red", clave, "equipos", len(fichas),
			"mapa de puertos", capacidad)
		return nil
	})
	if err != nil {
		s.Bitacora.Error("no se pudo guardar lo consultado por SNMP", "red", clave, "error", err)
	}
}

// reconocer le pone tipo a los equipos usando el catalogo de dispositivos.
//
// Va al final del barrido profundo, despues de SNMP, porque para entonces ya se
// sabe todo lo que se puede saber del aparato: puertos, banners, fabricante y lo
// que dijo por SNMP. Reconocer antes seria hacerlo con menos datos.
func (s *Servicio) reconocer(ctx context.Context, clave string) {
	if s.Catalogo == nil || len(s.Catalogo.Definiciones()) == 0 {
		return
	}

	err := s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
		equipos, err := base.ParaReconocer(ctx)
		if err != nil {
			return err
		}

		tipos := make(map[int64]string, len(equipos))
		for _, equipo := range equipos {
			definicion := s.Catalogo.Reconocer(catalogo.Equipo{
				IP:         equipo.IP,
				MAC:        equipo.MAC,
				Fabricante: equipo.Fabricante,
				Nombre:     equipo.Nombre,
				Puertos:    equipo.Puertos,
				Banners:    equipo.Banners,
				SnmpDescr:  equipo.SnmpDescr,
			})
			if definicion == nil {
				// Sin coincidencia se deja vacio a proposito: la interfaz lo usa
				// para ofrecer "proponer definicion", que es como crece el catalogo.
				tipos[equipo.ID] = ""
				continue
			}
			tipos[equipo.ID] = definicion.Nombre
		}

		cambiados, err := base.PonerTipos(ctx, tipos)
		if err != nil {
			return err
		}
		if cambiados > 0 {
			s.Bitacora.Info("equipos reconocidos", "red", clave, "cambiados", cambiados)
		}
		return nil
	})
	if err != nil {
		s.Bitacora.Warn("no se pudo reconocer los equipos", "red", clave, "error", err)
	}
}

// revisarAlertas genera los avisos de este escaneo y los manda hacia afuera.
//
// Va despues de todo lo demas: para entonces la foto de la red ya esta completa
// y guardada, que es lo unico contra lo que se puede comparar sin inventar.
func (s *Servicio) revisarAlertas(ctx context.Context, clave, nombreRed string, escaneoID int64) {
	var nuevas []basedatos.Alerta
	var destinos []basedatos.DestinoAlerta
	var abiertas int

	err := s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
		var err error
		nuevas, err = base.GenerarAlertas(ctx, escaneoID)
		if err != nil {
			return err
		}
		abiertas, err = base.ContarAlertasAbiertas(ctx)
		if err != nil {
			return err
		}
		destinos, err = base.ListarDestinos(ctx)
		return err
	})
	if err != nil {
		s.Bitacora.Error("no se pudieron generar las alertas", "red", clave, "error", err)
		return
	}

	// El contador vive en el catalogo para que el panel de inicio lo pinte sin
	// abrir el archivo de cada red.
	if err := s.Datos.ActualizarAlertasEnCatalogo(ctx, clave, abiertas); err != nil {
		s.Bitacora.Warn("no se pudo actualizar el contador de alertas", "red", clave, "error", err)
	}

	if len(nuevas) > 0 {
		s.Bitacora.Info("alertas generadas", "red", clave, "nuevas", len(nuevas), "abiertas", abiertas)
	}
	if len(destinos) == 0 {
		// Sin destinos las alertas se ven en la interfaz y ya. No es un error:
		// es una red a la que todavia no se le dijo a donde avisar.
		return
	}

	s.enviarAlertas(ctx, clave, nombreRed, destinos)
}

// enviarAlertas manda lo pendiente a cada destino activo.
func (s *Servicio) enviarAlertas(ctx context.Context, clave, nombreRed string, destinos []basedatos.DestinoAlerta) {
	var pendientes []basedatos.Alerta
	err := s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
		var err error
		pendientes, err = base.AlertasSinEnviar(ctx)
		return err
	})
	if err != nil || len(pendientes) == 0 {
		return
	}

	for _, alerta := range pendientes {
		mensaje := avisos.Alerta{
			Tipo:    alerta.Tipo,
			Momento: alerta.Momento,
			Titulo:  alerta.Titulo,
			Detalle: alerta.Detalle,
			Red:     nombreRed,
		}

		// Se marca como enviada aunque algun destino falle: reintentar contra
		// todos por culpa de uno significaria avisar tres veces a los que si
		// funcionan, que es peor que perder un aviso en el que fallo.
		for _, destino := range destinos {
			if !destino.Activo {
				continue
			}
			problema := ""
			if err := avisos.Enviar(ctx, avisos.Destino{
				Nombre:  destino.Nombre,
				Tipo:    destino.Tipo,
				Destino: destino.Destino,
				Extra:   destino.Extra,
			}, mensaje); err != nil {
				problema = err.Error()
				s.Bitacora.Warn("no se pudo avisar", "red", clave,
					"destino", destino.Nombre, "error", err)
			}

			id := destino.ID
			texto := problema
			s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
				return base.AnotarEnvio(ctx, id, texto)
			})
		}

		idAlerta := alerta.ID
		s.Datos.ConRed(ctx, clave, func(base *basedatos.Base) error {
			return base.MarcarEnviada(ctx, idAlerta)
		})
	}
}

// nombreRed devuelve el nombre bonito de la red, para que los avisos digan de
// que sitio hablan y no una clave de archivo.
func nombreRed(s *Servicio, ctx context.Context, clave string) string {
	red, err := s.Datos.BuscarRed(ctx, clave)
	if err != nil {
		return clave
	}
	return red.Nombre
}
