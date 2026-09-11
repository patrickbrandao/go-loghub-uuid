package loghubuuid

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
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
//	string   — interpretado por Parse, em qualquer formato aceito;
//	           a string vazia grava o UUID nulo, sem erro
//	[]byte   — 16 bytes viram o valor binário; vazio grava o UUID nulo;
//	           outro tamanho é texto
//
// Texto vazio equivale a ausência de valor, como no pacote
// github.com/google/uuid: colunas de texto cujo valor padrão é a string
// vazia não devem falhar na leitura.
//
// Em caso de erro o receptor não é alterado.
func (u *UUID) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*u = Nil
		return nil

	case string:
		if v == "" {
			*u = Nil
			return nil
		}
		parsed, err := Parse(v)
		if err != nil {
			return err
		}
		*u = parsed
		return nil

	case []byte:
		if len(v) == 0 {
			*u = Nil
			return nil
		}
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

// isAbsentScanValue informa se o valor vindo do banco representa ausência
// de UUID: NULL, string vazia ou fatia de bytes vazia.
func isAbsentScanValue(src any) bool {
	switch v := src.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case []byte:
		return len(v) == 0
	}
	return false
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

// Scan implementa sql.Scanner. NULL, string vazia e fatia de bytes vazia
// produzem valor ausente (Valid falso), sem erro.
func (n *NullUUID) Scan(src any) error {
	if isAbsentScanValue(src) {
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

// UnmarshalJSON lê uma string JSON em qualquer formato aceito por Parse,
// ou null.
//
// O caso comum, uma string sem sequências de escape, é lido direto dos
// bytes, sem alocar. Se a string contiver escapes JSON (por exemplo
// \u0030 no lugar de 0), a decodificação é delegada ao encoding/json,
// que os interpreta como faria para o tipo UUID; isso custa uma alocação
// e só acontece nesse caso raro. Erros de sintaxe JSON viram
// ErrInvalidFormat.
//
// Em caso de erro o receptor não é alterado.
func (n *NullUUID) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.UUID, n.Valid = Nil, false
		return nil
	}
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return ErrInvalidFormat
	}
	raw := data[1 : len(data)-1]
	if bytes.IndexByte(raw, '\\') >= 0 {
		var parsed UUID
		if err := json.Unmarshal(data, &parsed); err != nil {
			if errors.Is(err, ErrInvalidFormat) {
				return err
			}
			return ErrInvalidFormat
		}
		n.UUID, n.Valid = parsed, true
		return nil
	}
	parsed, err := ParseBytes(raw)
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
