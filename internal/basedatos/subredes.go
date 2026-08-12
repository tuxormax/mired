package basedatos

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

// ErrCIDRInvalido lo devuelve el alta cuando lo capturado no es una subred.
var ErrCIDRInvalido = errors.New("la subred no tiene un formato valido")

// Subred es un rango de direcciones que se escanea dentro de una red.
type Subred struct {
	ID          int64   `json:"id"`
	CIDR        string  `json:"cidr"`
	Nombre      string  `json:"nombre"`
	Descripcion string  `json:"descripcion"`
	VLAN        *int    `json:"vlan"`
	Escanear    bool    `json:"escanear"`
	Estatus     int     `json:"estatus"`
	Creada      string  `json:"creada"`
	Modificada  *string `json:"modificada"`
}

// NormalizarCIDR valida y deja la subred en su forma canonica: 192.168.1.34/24
// se guarda como 192.168.1.0/24.
//
// Guardar la forma canonica evita tener la misma subred dos veces escrita
// distinto, que es el tipo de dato sucio que despues nadie entiende de donde
// salio.
func NormalizarCIDR(texto string) (string, error) {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return "", ErrCIDRInvalido
	}
	prefijo, err := netip.ParsePrefix(texto)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrCIDRInvalido, texto)
	}
	return prefijo.Masked().String(), nil
}

// ListarSubredes devuelve las subredes activas de la red abierta.
func (b *Base) ListarSubredes(ctx context.Context) ([]Subred, error) {
	filas, err := b.QueryContext(ctx, `
		SELECT id, cidr, COALESCE(nombre, ''), COALESCE(descripcion, ''), vlan,
		       escanear, estatus, creada, modificada
		  FROM subredes
		 WHERE estatus = 1
		 ORDER BY cidr`)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron listar las subredes: %w", err)
	}
	defer filas.Close()

	subredes := []Subred{}
	for filas.Next() {
		var s Subred
		var escanear int
		if err := filas.Scan(&s.ID, &s.CIDR, &s.Nombre, &s.Descripcion, &s.VLAN,
			&escanear, &s.Estatus, &s.Creada, &s.Modificada); err != nil {
			return nil, err
		}
		s.Escanear = escanear == 1
		subredes = append(subredes, s)
	}
	return subredes, filas.Err()
}

// CrearSubred agrega un rango a escanear. Si la subred ya existia borrada, se
// reactiva en vez de duplicarla.
func (b *Base) CrearSubred(ctx context.Context, cidr, nombre, descripcion string, vlan *int) (Subred, error) {
	canonico, err := NormalizarCIDR(cidr)
	if err != nil {
		return Subred{}, err
	}

	momento := Ahora()
	_, err = b.ExecContext(ctx, `
		INSERT INTO subredes (cidr, nombre, descripcion, vlan, escanear, estatus, creada)
		VALUES (?, ?, ?, ?, 1, 1, ?)
		ON CONFLICT (cidr) WHERE estatus >= 0 DO UPDATE SET
			nombre = excluded.nombre,
			descripcion = excluded.descripcion,
			vlan = excluded.vlan,
			estatus = 1,
			modificada = ?`,
		canonico, nombre, descripcion, vlan, momento, momento)
	if err != nil {
		return Subred{}, fmt.Errorf("no se pudo guardar la subred: %w", err)
	}

	var s Subred
	var escanear int
	err = b.QueryRowContext(ctx, `
		SELECT id, cidr, COALESCE(nombre, ''), COALESCE(descripcion, ''), vlan,
		       escanear, estatus, creada, modificada
		  FROM subredes WHERE cidr = ?`, canonico).
		Scan(&s.ID, &s.CIDR, &s.Nombre, &s.Descripcion, &s.VLAN, &escanear,
			&s.Estatus, &s.Creada, &s.Modificada)
	if err != nil {
		return Subred{}, fmt.Errorf("no se pudo leer la subred guardada: %w", err)
	}
	s.Escanear = escanear == 1
	return s, nil
}

// BorrarSubred deja de escanear un rango, con borrado suave.
func (b *Base) BorrarSubred(ctx context.Context, id int64) error {
	resultado, err := b.ExecContext(ctx,
		`UPDATE subredes SET estatus = -1, modificada = ? WHERE id = ? AND estatus >= 0`,
		Ahora(), id)
	if err != nil {
		return fmt.Errorf("no se pudo borrar la subred: %w", err)
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		return fmt.Errorf("la subred no existe")
	}
	return nil
}
