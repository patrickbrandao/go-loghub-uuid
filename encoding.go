package loghubuuid

import "slices"

// Este arquivo dá ao tipo UUID as quatro interfaces de serialização da
// biblioteca padrão: encoding.TextMarshaler, encoding.TextUnmarshaler,
// encoding.BinaryMarshaler e encoding.BinaryUnmarshaler.
//
// ATENÇÃO — mudança de formato de dados. Antes destes métodos existirem,
// encoding/json serializava um UUID como lista de 16 números, porque o
// tipo é um vetor de bytes. Com MarshalText presente, o mesmo valor passa
// a ser serializado como a string canônica entre aspas. O mesmo vale para
// encoding/gob, que passa a usar MarshalBinary. A API não mudou, mas dados
// já gravados no formato antigo precisam de atenção na migração.

// encodeHex escreve a forma canônica 8-4-4-4-12 nos 36 primeiros bytes de
// dst, que precisa ter ao menos esse tamanho.
//
// É a mesma lógica de String, deliberadamente duplicada: String é caminho
// quente com zero alocação além do resultado, e não deve passar a pagar
// uma chamada de função por causa dos serializadores.
func encodeHex(dst []byte, u UUID) {
	j := 0
	for i := 0; i < 16; i++ {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			dst[j] = '-'
			j++
		}
		dst[j] = hexDigits[u[i]>>4]
		dst[j+1] = hexDigits[u[i]&0x0F]
		j += 2
	}
}

// AppendTo escreve a forma canônica de 36 bytes no fim de dst e devolve o
// slice estendido, como fazem as funções Append da biblioteca padrão.
//
// É o caminho mais barato para serializar muitos identificadores: não
// aloca nada quando dst tem capacidade sobrando, enquanto String paga uma
// alocação por chamada. Reaproveite o mesmo buffer entre chamadas:
//
//	buf = u.AppendTo(buf[:0])
//
// Passar nil é válido e faz a função alocar os 36 bytes.
func (u UUID) AppendTo(dst []byte) []byte {
	n := len(dst)
	dst = slices.Grow(dst, 36)[:n+36]
	encodeHex(dst[n:], u)
	return dst
}

// AppendText é AppendTo com a assinatura da interface encoding.TextAppender,
// introduzida no Go 1.24. O erro devolvido é sempre nulo.
//
// A interface não é referenciada em lugar nenhum do pacote, então o método
// compila também nas versões anteriores, onde simplesmente não satisfaz
// interface alguma. Isso mantém o go.mod em 1.22.
func (u UUID) AppendText(dst []byte) ([]byte, error) {
	return u.AppendTo(dst), nil
}

// MarshalText devolve a representação canônica em texto, com 36 bytes.
// Implementa encoding.TextMarshaler, o que faz encoding/json gravar o
// UUID como string.
func (u UUID) MarshalText() ([]byte, error) {
	return u.AppendTo(make([]byte, 0, 36)), nil
}

// UnmarshalText lê a representação em texto, aceitando os mesmos quatro
// formatos de Parse. Implementa encoding.TextUnmarshaler.
//
// Em caso de erro o receptor não é alterado.
func (u *UUID) UnmarshalText(data []byte) error {
	parsed, err := ParseBytes(data)
	if err != nil {
		return err
	}
	*u = parsed
	return nil
}

// MarshalBinary devolve os 16 bytes do UUID em ordem de rede. Implementa
// encoding.BinaryMarshaler.
func (u UUID) MarshalBinary() ([]byte, error) {
	return u[:], nil
}

// UnmarshalBinary lê exatamente 16 bytes em ordem de rede. Implementa
// encoding.BinaryUnmarshaler.
//
// Em caso de erro o receptor não é alterado.
func (u *UUID) UnmarshalBinary(data []byte) error {
	if len(data) != 16 {
		return ErrInvalidLength
	}
	copy(u[:], data)
	return nil
}
