package loghubuuid

import (
	"database/sql/driver"
	"errors"
)

// ErrInvalidScanType indica que o valor lido do banco não é conversível
// para UUID.
var ErrInvalidScanType = errors.New("loghubuuid: tipo nao suportado na leitura de UUID")

// Scan lê um UUID vindo do banco de dados, implementando sql.Scanner.
//
// Aceita:
//
//	nil      — grava o UUID nulo
//	string   — interpretado por Parse, em qualquer formato aceito
//	[]byte   — 16 bytes viram o valor binário; outro tamanho é texto
//
// Em caso de erro o receptor não é alterado.
func (u *UUID) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*u = Nil
		return nil

	case string:
		parsed, err := Parse(v)
		if err != nil {
			return err
		}
		*u = parsed
		return nil

	case []byte:
		if len(v) == 16 {
			copy(u[:], v)
			return nil
		}
		parsed, err := ParseBytes(v)
		if err != nil {
			return err
		}
		*u = parsed
		return nil
	}
	return ErrInvalidScanType
}

// Value entrega o UUID ao banco de dados como a string canônica,
// implementando driver.Valuer.
//
// Para gravar em coluna binária de 16 bytes, passe o valor fatiado pelo
// chamador em vez de confiar nesta conversão.
func (u UUID) Value() (driver.Value, error) {
	return u.String(), nil
}

// NullUUID representa um UUID que pode ser nulo no banco de dados, no
// mesmo espírito de sql.NullString.
type NullUUID struct {
	UUID  UUID
	Valid bool // Valid é falso quando a coluna era NULL
}

// Scan implementa sql.Scanner.
func (n *NullUUID) Scan(src any) error {
	if src == nil {
		n.UUID, n.Valid = Nil, false
		return nil
	}
	if err := n.UUID.Scan(src); err != nil {
		n.Valid = false
		return err
	}
	n.Valid = true
	return nil
}

// Value implementa driver.Valuer.
func (n NullUUID) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.UUID.String(), nil
}

// MarshalJSON grava a string canônica, ou null quando não há valor.
func (n NullUUID) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	buf := make([]byte, 38)
	buf[0] = '"'
	encodeHex(buf[1:], n.UUID)
	buf[37] = '"'
	return buf, nil
}

// UnmarshalJSON lê a string canônica, ou null.
func (n *NullUUID) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.UUID, n.Valid = Nil, false
		return nil
	}
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return ErrInvalidFormat
	}
	parsed, err := ParseBytes(data[1 : len(data)-1])
	if err != nil {
		return err
	}
	n.UUID, n.Valid = parsed, true
	return nil
}

// MarshalText grava a string canônica, ou vazio quando não há valor.
func (n NullUUID) MarshalText() ([]byte, error) {
	if !n.Valid {
		return []byte{}, nil
	}
	return n.UUID.MarshalText()
}

// UnmarshalText lê a string canônica; entrada vazia produz valor ausente.
func (n *NullUUID) UnmarshalText(data []byte) error {
	if len(data) == 0 {
		n.UUID, n.Valid = Nil, false
		return nil
	}
	if err := n.UUID.UnmarshalText(data); err != nil {
		n.Valid = false
		return err
	}
	n.Valid = true
	return nil
}

// MarshalBinary grava os 16 bytes, ou nada quando não há valor.
func (n NullUUID) MarshalBinary() ([]byte, error) {
	if !n.Valid {
		return []byte{}, nil
	}
	return n.UUID.MarshalBinary()
}

// UnmarshalBinary lê 16 bytes; entrada vazia produz valor ausente.
func (n *NullUUID) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		n.UUID, n.Valid = Nil, false
		return nil
	}
	if err := n.UUID.UnmarshalBinary(data); err != nil {
		n.Valid = false
		return err
	}
	n.Valid = true
	return nil
}
