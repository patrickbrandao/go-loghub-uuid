package loghubuuid

import "errors"

// ErrInvalidFormat indica que a string fornecida não está no formato
// canônico de UUID (8-4-4-4-12 dígitos hexadecimais).
var ErrInvalidFormat = errors.New("loghubuuid: string de UUID em formato inválido")

const hexDigits = "0123456789abcdef"

// String converte o UUID binário (16 bytes) na representação canônica
// em texto: 32 dígitos hexadecimais minúsculos agrupados como
// 8-4-4-4-12 e separados por hifens. Exemplo:
//
//	0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff
//
// A implementação escreve diretamente em um buffer de 36 bytes, sem
// alocações intermediárias, para ser barata em caminhos quentes.
func (u UUID) String() string {
	var buf [36]byte
	j := 0
	for i := 0; i < 16; i++ {
		// Hifens antes dos bytes 4, 6, 8 e 10.
		if i == 4 || i == 6 || i == 8 || i == 10 {
			buf[j] = '-'
			j++
		}
		buf[j] = hexDigits[u[i]>>4]
		buf[j+1] = hexDigits[u[i]&0x0F]
		j += 2
	}
	return string(buf[:])
}

// FromString interpreta uma string canônica de UUID e devolve o valor
// binário de 128 bits. Aceita maiúsculas ou minúsculas. Retorna
// ErrInvalidFormat se o tamanho, os hifens ou os dígitos forem inválidos.
func FromString(s string) (UUID, error) {
	var u UUID
	if len(s) != 36 {
		return u, ErrInvalidFormat
	}
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return u, ErrInvalidFormat
	}
	j := 0
	for i := 0; i < 36; {
		if s[i] == '-' {
			i++
			continue
		}
		high, ok1 := fromHex(s[i])
		low, ok2 := fromHex(s[i+1])
		if !ok1 || !ok2 {
			return UUID{}, ErrInvalidFormat
		}
		u[j] = high<<4 | low
		j++
		i += 2
	}
	return u, nil
}

// fromHex converte um único caractere hexadecimal em seu valor 0..15.
func fromHex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// StringToBinary é um apelido explícito para FromString: converte a
// string canônica no binário de 128 bits.
func StringToBinary(s string) (UUID, error) { return FromString(s) }

// BinaryToString é um apelido explícito para o método String():
// converte o binário de 128 bits na string canônica.
func BinaryToString(u UUID) string { return u.String() }
