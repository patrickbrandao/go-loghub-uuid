package tests

import (
	"errors"
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
func safeFromString(s string) (u uuid.UUID, panicked any, err error) {
	defer func() { panicked = recover() }()
	u, err = uuid.FromString(s)
	return
}

// TestFromStringNeverPanics verifica que nenhuma entrada malformada de 36
// caracteres provoca pânico. Toda mutação de um único byte sobre uma
// string canônica deve devolver ErrInvalidFormat ou um UUID válido.
//
// REGRESSÃO: antes da correção, um hífen colocado nos deslocamentos
// pares do último grupo (24, 26, 28, 30, 32 e 34) fazia o laço de
// FromString ler s[36] e estourar o índice.
func TestFromStringNeverPanics(t *testing.T) {
	base := []byte(canonical)
	for i := 0; i < len(base); i++ {
		for c := 0; c < 256; c++ {
			mutated := make([]byte, len(base))
			copy(mutated, base)
			mutated[i] = byte(c)
			s := string(mutated)

			_, panicked, _ := safeFromString(s)
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
// REGRESSÃO: antes da correção, cada um desses casos causava pânico.
func TestFromStringExtraHyphen(t *testing.T) {
	for _, pos := range []int{24, 26, 28, 30, 32, 34} {
		mutated := []byte(canonical)
		mutated[pos] = '-'
		s := string(mutated)

		_, panicked, err := safeFromString(s)
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
		if _, err := uuid.FromString(s); !errors.Is(err, uuid.ErrInvalidFormat) {
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
		if _, err := uuid.FromString(string(mutated)); !errors.Is(err, uuid.ErrInvalidFormat) {
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

// TestParseHyphenPositions confere que o analisador permissivo, como o
// estrito (TestFromStringHyphenPositions), exige os quatro hífens
// canônicos exatamente nas posições 8, 13, 18 e 23 do miolo de 36
// caracteres — nas três formas que carregam esse miolo: canônica, entre
// chaves e URN.
//
// REGRESSÃO: fromCanonical (usado por Parse, ParseBytes e Validate) e
// FromString compartilham a mesma lógica de checagem de hífen, mas em
// cópias de código separadas (parse.go documenta o motivo). Um teste de
// mutação que removeu a checagem de fromCanonical passou pela suíte
// inteira sem esta trava: qualquer separador — "x", "+", espaço — no
// lugar de um hífen passava a ser aceito, e Parse devolvia um UUID cujos
// 16 bytes vinham só dos 32 dígitos hexadecimais restantes.
func TestParseHyphenPositions(t *testing.T) {
	for _, pos := range []int{8, 13, 18, 23} {
		mutated := []byte(canonical)
		mutated[pos] = 'x'
		miolo := string(mutated)

		for nome, entrada := range map[string]string{
			"canônico":     miolo,
			"entre chaves": "{" + miolo + "}",
			"URN":          "urn:uuid:" + miolo,
		} {
			if _, err := uuid.Parse(entrada); !errors.Is(err, uuid.ErrInvalidFormat) {
				t.Errorf("posição %d, %s: hífen substituído deveria ser rejeitado, obtido erro %v", pos, nome, err)
			}
			if _, err := uuid.ParseBytes([]byte(entrada)); !errors.Is(err, uuid.ErrInvalidFormat) {
				t.Errorf("posição %d, %s (ParseBytes): hífen substituído deveria ser rejeitado", pos, nome)
			}
			if err := uuid.Validate(entrada); !errors.Is(err, uuid.ErrInvalidFormat) {
				t.Errorf("posição %d, %s (Validate): hífen substituído deveria ser rejeitado", pos, nome)
			}
		}
	}

	// Controle: a forma compacta (32 dígitos, sem hífen algum) não tem
	// posição de hífen para exigir, e continua aceita.
	compact := strings.ReplaceAll(canonical, "-", "")
	if _, err := uuid.Parse(compact); err != nil {
		t.Errorf("forma compacta sem hífens deveria ser aceita, erro %v", err)
	}
}
