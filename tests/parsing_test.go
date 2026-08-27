package tests

import (
	"strings"
	"testing"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// canonical é uma string de UUID válida usada como base para mutações.
const canonical = "0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff"

// mustParse falha o teste se a string não for aceita.
func mustParse(t *testing.T, s string) uuid.UUID {
	t.Helper()
	u, err := uuid.FromString(s)
	if err != nil {
		t.Fatalf("FromString(%q) devolveu erro inesperado: %v", s, err)
	}
	return u
}

// safeFromString executa FromString capturando pânico, para que um
// defeito de limite de índice apareça como falha de teste e não derrube
// toda a suíte.
func safeFromString(s string) (u uuid.UUID, err error, panicked any) {
	defer func() { panicked = recover() }()
	u, err = uuid.FromString(s)
	return
}

// TestFromStringNeverPanics verifica que nenhuma entrada malformada de 36
// caracteres provoca pânico. Toda mutação de um único byte sobre uma
// string canônica deve devolver ErrInvalidFormat ou um UUID válido.
//
// REGRESSÃO: hoje falha. Um hífen colocado nos deslocamentos pares do
// último grupo (24, 26, 28, 30, 32 e 34) faz o laço de FromString ler
// s[36] e estourar o índice. Ver tasks/NEWS.md, defeito BUG-01.
func TestFromStringNeverPanics(t *testing.T) {
	base := []byte(canonical)
	for i := 0; i < len(base); i++ {
		for c := 0; c < 256; c++ {
			mutated := make([]byte, len(base))
			copy(mutated, base)
			mutated[i] = byte(c)
			s := string(mutated)

			_, _, panicked := safeFromString(s)
			if panicked != nil {
				t.Fatalf("FromString entrou em pânico na posição %d com o byte %#x (%q): %v",
					i, c, s, panicked)
			}
		}
	}
}

// TestFromStringExtraHyphen isola o caso mínimo do defeito acima: uma
// string de 36 caracteres com os quatro hífens canônicos nas posições
// corretas mais um hífen extra dentro do último grupo.
//
// REGRESSÃO: hoje falha (pânico). Ver tasks/NEWS.md, defeito BUG-01.
func TestFromStringExtraHyphen(t *testing.T) {
	for _, pos := range []int{24, 26, 28, 30, 32, 34} {
		mutated := []byte(canonical)
		mutated[pos] = '-'
		s := string(mutated)

		_, err, panicked := safeFromString(s)
		if panicked != nil {
			t.Errorf("hífen extra na posição %d causou pânico: %v", pos, panicked)
			continue
		}
		if err == nil {
			t.Errorf("hífen extra na posição %d deveria produzir ErrInvalidFormat", pos)
		}
	}
}

// TestFromStringLengthBoundaries confere que tamanhos diferentes de 36
// são rejeitados, inclusive a string vazia e entradas muito longas.
func TestFromStringLengthBoundaries(t *testing.T) {
	for n := 0; n <= 72; n++ {
		if n == 36 {
			continue
		}
		s := strings.Repeat("a", n)
		if _, err := uuid.FromString(s); err != uuid.ErrInvalidFormat {
			t.Fatalf("tamanho %d: esperava ErrInvalidFormat, obteve %v", n, err)
		}
	}
}

// TestFromStringCaseInsensitive garante que maiúsculas e minúsculas
// produzem exatamente os mesmos 16 bytes.
func TestFromStringCaseInsensitive(t *testing.T) {
	lower := mustParse(t, canonical)
	upper := mustParse(t, strings.ToUpper(canonical))
	if lower != upper {
		t.Fatalf("maiúsculas e minúsculas divergiram: %v vs %v", lower, upper)
	}
	// String() sempre reemite em minúsculas.
	if got := upper.String(); got != canonical {
		t.Fatalf("String() = %q, esperado %q", got, canonical)
	}
}

// TestFromStringHyphenPositions confere que os quatro hífens canônicos
// são obrigatórios exatamente nas posições 8, 13, 18 e 23.
func TestFromStringHyphenPositions(t *testing.T) {
	for _, pos := range []int{8, 13, 18, 23} {
		mutated := []byte(canonical)
		mutated[pos] = 'a'
		if _, err := uuid.FromString(string(mutated)); err != uuid.ErrInvalidFormat {
			t.Fatalf("hífen removido da posição %d deveria ser rejeitado", pos)
		}
	}
}

// TestFromStringErrorReturnsZero garante que o valor devolvido junto com
// um erro é sempre o UUID zerado, sem bytes parcialmente preenchidos.
func TestFromStringErrorReturnsZero(t *testing.T) {
	invalid := []string{
		"",
		"abc",
		"0192f7c5-1a2b-7c3d-8e4f-aabbccddeegg",
		"0192f7c5+1a2b-7c3d-8e4f-aabbccddeeff",
		"zzzzzzzz-1a2b-7c3d-8e4f-aabbccddeeff",
	}
	for _, s := range invalid {
		u, err := uuid.FromString(s)
		if err == nil {
			t.Fatalf("esperava erro para %q", s)
		}
		if u != (uuid.UUID{}) {
			t.Fatalf("FromString(%q) devolveu %v junto com o erro; esperado UUID zerado", s, u)
		}
	}
}

// TestStringRoundTripExtremes exercita os valores de borda dos 16 bytes.
func TestStringRoundTripExtremes(t *testing.T) {
	cases := []uuid.UUID{
		{},
		{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x76, 0x77, 0x88, 0x99, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
	}
	for _, want := range cases {
		s := want.String()
		if len(s) != 36 {
			t.Fatalf("String() devolveu %d caracteres: %q", len(s), s)
		}
		got, err := uuid.FromString(s)
		if err != nil {
			t.Fatalf("FromString(%q) falhou: %v", s, err)
		}
		if got != want {
			t.Fatalf("round-trip divergiu: %v -> %q -> %v", want, s, got)
		}
	}
}

// TestStringKnownVector fixa a formatação canônica para um valor
// conhecido, travando a posição dos hífens e a ordem dos bytes.
func TestStringKnownVector(t *testing.T) {
	u := uuid.UUID{
		0x01, 0x92, 0xf7, 0xc5, 0x1a, 0x2b, 0x7c, 0x3d,
		0x8e, 0x4f, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
	}
	if got := u.String(); got != canonical {
		t.Fatalf("String() = %q, esperado %q", got, canonical)
	}
	if got := uuid.BinaryToString(u); got != canonical {
		t.Fatalf("BinaryToString() = %q, esperado %q", got, canonical)
	}
}
