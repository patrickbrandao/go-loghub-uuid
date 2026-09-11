package loghubuuid

import "strconv"

// Nil é o UUID de valor zero, com os 128 bits em zero. Não altere o valor
// desta variável.
var Nil UUID

// Max é o UUID com todos os 128 bits em um, definido pela RFC 9562 como
// limite superior de faixa. Não altere o valor desta variável.
var Max = UUID{
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
}

// IsZero informa se o UUID é o valor nulo.
func (u UUID) IsZero() bool { return u == Nil }

// IsMax informa se o UUID tem todos os bits em um.
func (u UUID) IsMax() bool { return u == Max }

// Bytes devolve uma cópia dos 16 bytes em ordem de rede. Alterar o
// resultado não afeta o UUID de origem.
//
// Equivale a MarshalBinary sem o erro sempre nulo. Dentro do pacote do
// chamador, u[:] é mais barato e não copia, mas devolve um slice apoiado
// no próprio valor: use Bytes quando o destino guardar a referência.
//
// Serve também para contornar uma armadilha de formatação: como UUID
// satisfaz fmt.Stringer, %x sobre um UUID formata a string canônica, não
// os bytes. fmt.Printf("%x", u.Bytes()) imprime os 32 dígitos esperados.
func (u UUID) Bytes() []byte {
	out := make([]byte, 16)
	copy(out, u[:])
	return out
}

// IsValid informa se o UUID tem a forma prevista pela RFC 9562: variante
// RFC (0b10) e número de versão entre 1 e 8.
//
// Nil e Max são aceitos apesar de não carregarem versão nem variante:
// a RFC 9562 seção 5.9 e 5.10 os define como valores especiais válidos.
// Para distinguir esses dois casos, use IsZero e IsMax.
//
// A verificação não é específica do UUIDv7: um UUIDv4 gerado em outro
// lugar também é válido. Para exigir a versão 7, compare Version com 7.
func (u UUID) IsValid() bool {
	if u == Nil || u == Max {
		return true
	}
	return u.Variant() == 0b10 && u.Version() >= 1 && u.Version() <= 8
}

// Compare ordena dois UUIDs pela sequência de bytes, devolvendo -1, 0 ou
// 1. Para UUIDs de versão 6 ou 7 a ordem coincide com a ordem cronológica.
//
// Para simples igualdade não é preciso chamar nada: UUID é um vetor de
// bytes e aceita o operador de igualdade diretamente.
func (u UUID) Compare(other UUID) int {
	for i := 0; i < 16; i++ {
		switch {
		case u[i] < other[i]:
			return -1
		case u[i] > other[i]:
			return 1
		}
	}
	return 0
}

// URN devolve a representação como URN da RFC 8141, ou seja, a string
// canônica prefixada por "urn:uuid:".
func (u UUID) URN() string {
	var buf [45]byte
	copy(buf[:9], "urn:uuid:")
	encodeHex(buf[9:], u)
	return string(buf[:])
}

// UUIDs é uma lista de UUIDs, com conveniências para logs e consultas.
type UUIDs []UUID

// Strings converte a lista para as representações canônicas em texto.
func (list UUIDs) Strings() []string {
	out := make([]string, len(list))
	for i, u := range list {
		out[i] = u.String()
	}
	return out
}

// VersionString descreve em texto o número de versão devolvido por
// Version.
func VersionString(v byte) string {
	if v >= 1 && v <= 8 {
		return "versao " + strconv.Itoa(int(v))
	}
	return "versao desconhecida " + strconv.Itoa(int(v))
}

// VariantString descreve em texto o código de variante devolvido por
// Variant.
//
// Variant expõe apenas os 2 bits altos do byte 8, portanto o código 3
// cobre tanto a variante reservada à Microsoft quanto a reservada para uso
// futuro; distinguir as duas exigiria um terceiro bit.
func VariantString(v byte) string {
	switch v {
	case 0, 1:
		return "reservada para compatibilidade NCS"
	case 2:
		return "RFC 9562"
	case 3:
		return "reservada para a Microsoft ou para uso futuro"
	}
	return "variante desconhecida " + strconv.Itoa(int(v))
}
