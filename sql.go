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
// O texto é o formato gravado desde a primeira versão e não vai mudar:
// uma coluna que já recebeu texto e passasse a receber binário ficaria
// com dois formatos misturados, e nenhuma consulta acharia as linhas
// antigas. Para gravar em coluna binária de 16 bytes, converta para
// BinaryUUID no ponto da consulta.
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
//
// Em caso de erro o identificador não é alterado, mas o booleano cai para
// falso. É a única exceção à regra de receptor inalterado que vale para
// os desserializadores (docs/07-parsing-e-conversao.md seção 6.4), e tem
// motivo próprio:
// database/sql reaproveita o mesmo destino a cada linha, então um
// chamador que ignore o erro leria o valor da linha anterior como se
// fosse o da linha que falhou. Zerar o booleano transforma esse descuido
// em ausência de valor, e não em dado errado.
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
//
// Em caso de erro o receptor não é alterado, como em UnmarshalJSON e no
// tipo simples: o booleano de um valor que já estava presente continua
// verdadeiro. Ao contrário de Scan, aqui não há destino reaproveitado
// entre linhas que justifique a exceção.
func (n *NullUUID) UnmarshalText(data []byte) error {
	if len(data) == 0 {
		n.UUID, n.Valid = Nil, false
		return nil
	}
	if err := n.UUID.UnmarshalText(data); err != nil {
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
//
// Em caso de erro o receptor não é alterado, pelo mesmo motivo de
// UnmarshalText.
func (n *NullUUID) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		n.UUID, n.Valid = Nil, false
		return nil
	}
	if err := n.UUID.UnmarshalBinary(data); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

// --- gravação em coluna binária de 16 bytes ---

// BinaryUUID é um UUID que se entrega ao banco de dados como 16 bytes
// crus, em ordem de rede, em vez da string canônica de 36 caracteres.
//
// Existe porque a integração padrão é assimétrica de propósito: Scan já
// aceita 16 bytes crus na leitura, mas Value sempre escreve texto, e
// mudar Value quebraria em silêncio quem já tem texto gravado — dados
// novos deixariam de casar com os antigos na mesma coluna.
//
// Use quando a coluna for BINARY(16) ou BLOB, o que é o habitual em
// MySQL, MariaDB e SQLite, onde não existe tipo nativo de UUID e a forma
// binária economiza mais da metade do espaço por linha, replicado em
// todo índice secundário que referencie a chave. No PostgreSQL o tipo
// uuid é nativo e o driver converte o texto, então não há ganho.
//
// A escolha é por conversão no ponto da consulta, sem estado global e
// sem efeito sobre quem não usa:
//
//	_, err := db.Exec(
//	    "INSERT INTO eventos (id, corpo) VALUES (?, ?)",
//	    loghubuuid.BinaryUUID(u), corpo,
//	)
//
// A ordem dos bytes é a de rede, a mesma de MarshalBinary. Algumas
// receitas de MySQL sugerem rotacionar os campos do UUIDv1 para melhorar
// a localidade do índice; não faça isso aqui. O UUIDv7 já nasce
// ordenado, e o valor rotacionado não seria lido por nenhuma outra
// ferramenta.
type BinaryUUID UUID

// Value entrega os 16 bytes ao banco, implementando driver.Valuer.
//
// Aloca a fatia devolvida, porque driver.Value é uma interface e o
// driver pode reter o valor depois do retorno. É uma alocação por
// parâmetro de consulta, não por identificador gerado.
func (u BinaryUUID) Value() (driver.Value, error) {
	out := make([]byte, 16)
	copy(out, u[:])
	return out, nil
}

// Scan lê um UUID vindo do banco, implementando sql.Scanner. Delega ao
// Scan de UUID, portanto aceita as mesmas formas: NULL, texto em
// qualquer formato de Parse, e 16 bytes crus.
//
// Ler por BinaryUUID não exige que a coluna seja binária: o tipo existe
// para a escrita, e a leitura continua aceitando as duas formas.
func (u *BinaryUUID) Scan(src any) error {
	return (*UUID)(u).Scan(src)
}

// String devolve a forma canônica, como em UUID. Existe para que um
// BinaryUUID em mensagem de log ou de erro apareça legível, e não como
// vetor de bytes.
func (u BinaryUUID) String() string {
	return UUID(u).String()
}

// MarshalText devolve a forma canônica, como em UUID, para que o tipo
// participe de encoding/json e encoding/gob como string.
func (u BinaryUUID) MarshalText() ([]byte, error) {
	return UUID(u).MarshalText()
}

// UnmarshalText lê a forma canônica, aceitando os formatos de Parse.
func (u *BinaryUUID) UnmarshalText(data []byte) error {
	return (*UUID)(u).UnmarshalText(data)
}

// MarshalBinary entrega os 16 bytes crus em ordem de rede.
func (u BinaryUUID) MarshalBinary() ([]byte, error) {
	return UUID(u).MarshalBinary()
}

// UnmarshalBinary lê exatamente 16 bytes crus em ordem de rede.
func (u *BinaryUUID) UnmarshalBinary(data []byte) error {
	return (*UUID)(u).UnmarshalBinary(data)
}

// NullBinaryUUID é o equivalente de NullUUID para coluna binária de 16
// bytes que também aceita NULL.
//
// ATENÇÃO — ausência de valor e UUID nulo são coisas diferentes, e
// confundi-las produz a mesma linha no banco. Com Valid falso, Value
// devolve NULL; para gravar dezesseis bytes zerados é preciso Valid
// verdadeiro com UUID igual a Nil. Uma coluna que misture os dois casos
// não consegue mais distinguir "não havia valor" de "o valor era o UUID
// nulo".
type NullBinaryUUID struct {
	UUID  UUID
	Valid bool // Valid é falso quando a coluna era NULL
}

// Scan implementa sql.Scanner. NULL, string vazia e fatia de bytes vazia
// produzem valor ausente (Valid falso), sem erro, como em NullUUID.
func (n *NullBinaryUUID) Scan(src any) error {
	return (*NullUUID)(n).Scan(src)
}

// Value implementa driver.Valuer: NULL quando não há valor, e os 16
// bytes crus quando há.
func (n NullBinaryUUID) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return BinaryUUID(n.UUID).Value()
}

// MarshalJSON grava a string canônica, ou null quando não há valor.
func (n NullBinaryUUID) MarshalJSON() ([]byte, error) {
	return NullUUID(n).MarshalJSON()
}

// UnmarshalJSON lê uma string JSON ou null.
func (n *NullBinaryUUID) UnmarshalJSON(data []byte) error {
	return (*NullUUID)(n).UnmarshalJSON(data)
}

// MarshalText grava a string canônica, ou vazio quando não há valor.
func (n NullBinaryUUID) MarshalText() ([]byte, error) {
	return NullUUID(n).MarshalText()
}

// UnmarshalText lê a string canônica; vazio produz ausência.
func (n *NullBinaryUUID) UnmarshalText(data []byte) error {
	return (*NullUUID)(n).UnmarshalText(data)
}

// MarshalBinary grava os 16 bytes, ou nada quando não há valor.
func (n NullBinaryUUID) MarshalBinary() ([]byte, error) {
	return NullUUID(n).MarshalBinary()
}

// UnmarshalBinary lê 16 bytes; vazio produz ausência.
func (n *NullBinaryUUID) UnmarshalBinary(data []byte) error {
	return (*NullUUID)(n).UnmarshalBinary(data)
}
