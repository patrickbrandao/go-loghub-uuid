package loghubuuid

import "errors"

// Erros mais específicos devolvidos por Parse e ParseBytes. Todos embrulham
// ErrInvalidFormat, portanto errors.Is(err, ErrInvalidFormat) continua
// verdadeiro e quem já trata o sentinela antigo não precisa mudar nada.
//
// FromString e StringToBinary não mudaram: continuam devolvendo exatamente
// ErrInvalidFormat, inclusive para comparação direta com o operador de
// igualdade.
var (
	// ErrInvalidLength indica comprimento incompatível com todos os
	// formatos aceitos.
	ErrInvalidLength error = &formatError{"loghubuuid: comprimento invalido para string de UUID"}

	// ErrInvalidBrackets indica a forma entre chaves malformada.
	ErrInvalidBrackets error = &formatError{"loghubuuid: chaves invalidas em string de UUID"}
)

// formatError é um erro de formato que se apresenta como ErrInvalidFormat
// para errors.Is, preservando compatibilidade com o tratamento antigo.
type formatError struct{ msg string }

func (e *formatError) Error() string { return e.msg }
func (e *formatError) Unwrap() error { return ErrInvalidFormat }

// Parse interpreta um UUID em qualquer um dos quatro formatos usuais:
//
//	xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx        canônico, 36 caracteres
//	urn:uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx   URN, 45 caracteres
//	{xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}      entre chaves, 38 caracteres
//	xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx            hexadecimal cru, 32 caracteres
//
// Maiúsculas e minúsculas são aceitas, inclusive no prefixo URN. Em caso
// de erro o UUID devolvido é sempre Nil.
//
// Use FromString quando quiser aceitar somente o formato canônico.
func Parse(s string) (UUID, error) { return parseAny(s) }

// ParseBytes faz o mesmo que Parse a partir de uma sequência de bytes,
// sem converter para string e sem alocar.
func ParseBytes(b []byte) (UUID, error) { return parseAny(b) }

// MustParse interpreta um UUID em qualquer formato aceito por Parse e
// entra em pânico se a string for inválida.
//
// Destina-se a constantes do próprio código, como espaços de nomes; nunca
// use com entrada vinda de fora do programa.
func MustParse(s string) UUID {
	u, err := Parse(s)
	if err != nil {
		panic("loghubuuid: MustParse recebeu um UUID invalido: " + s)
	}
	return u
}

// Must devolve u, ou entra em pânico se err não for nula. Serve para
// encadear com funções que devolvem par de valores.
func Must(u UUID, err error) UUID {
	if err != nil {
		panic(err)
	}
	return u
}

// Validate informa se a string é um UUID em algum dos formatos aceitos por
// Parse, devolvendo apenas o erro.
func Validate(s string) error {
	_, err := parseAny(s)
	return err
}

// FromBytes constrói um UUID a partir de exatamente 16 bytes em ordem de
// rede. Devolve ErrInvalidLength se o tamanho for outro.
func FromBytes(b []byte) (UUID, error) {
	if len(b) != 16 {
		return Nil, ErrInvalidLength
	}
	var u UUID
	copy(u[:], b)
	return u, nil
}

// IsInvalidLengthError informa se o erro veio de um comprimento incorreto,
// mesmo que tenha sido embrulhado por outra camada com fmt.Errorf e %w.
// Equivale a errors.Is(err, ErrInvalidLength), como no pacote
// github.com/google/uuid.
func IsInvalidLengthError(err error) bool {
	return errors.Is(err, ErrInvalidLength)
}

// parseAny concentra a lógica de Parse e ParseBytes. O parâmetro de tipo
// evita a conversão de []byte para string, que alocaria.
func parseAny[T ~string | ~[]byte](v T) (UUID, error) {
	switch len(v) {
	case 36:
		return fromCanonical(v)
	case 45:
		if !hasURNPrefix(v) {
			return Nil, ErrInvalidFormat
		}
		return fromCanonical(v[9:])
	case 38:
		if v[0] != '{' || v[37] != '}' {
			return Nil, ErrInvalidBrackets
		}
		return fromCanonical(v[1:37])
	case 32:
		return fromCompact(v)
	}
	return Nil, ErrInvalidLength
}

// fromCanonical decodifica a forma 8-4-4-4-12. Repete a lógica de
// FromString em vez de chamá-la porque FromString recebe string e este
// caminho também atende []byte — e porque o caminho quente de FromString
// deve continuar exatamente como está.
func fromCanonical[T ~string | ~[]byte](v T) (UUID, error) {
	if v[8] != '-' || v[13] != '-' || v[18] != '-' || v[23] != '-' {
		return Nil, ErrInvalidFormat
	}
	var u UUID
	for j, p := range hexOffsets {
		high, ok1 := fromHex(v[p])
		low, ok2 := fromHex(v[p+1])
		if !ok1 || !ok2 {
			return Nil, ErrInvalidFormat
		}
		u[j] = high<<4 | low
	}
	return u, nil
}

// fromCompact decodifica 32 dígitos hexadecimais sem hifens.
func fromCompact[T ~string | ~[]byte](v T) (UUID, error) {
	var u UUID
	for i := 0; i < 16; i++ {
		high, ok1 := fromHex(v[i*2])
		low, ok2 := fromHex(v[i*2+1])
		if !ok1 || !ok2 {
			return Nil, ErrInvalidFormat
		}
		u[i] = high<<4 | low
	}
	return u, nil
}

// hasURNPrefix compara os 9 primeiros caracteres com "urn:uuid:" sem
// diferenciar maiúsculas de minúsculas.
func hasURNPrefix[T ~string | ~[]byte](v T) bool {
	const prefix = "urn:uuid:"
	for i := 0; i < len(prefix); i++ {
		c := v[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != prefix[i] {
			return false
		}
	}
	return true
}
