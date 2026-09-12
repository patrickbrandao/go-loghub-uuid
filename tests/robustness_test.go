package tests

import (
	"sync/atomic"
	"testing"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// TestNewGeneratorWithNilSourcePanics confere que uma fonte de entropia
// nula é rejeitada na configuração, e não na primeira geração.
//
// REGRESSÃO: antes da correção, NewGeneratorWith(nil) devolvia um
// Generator aparentemente válido que só estourava com "nil pointer
// dereference" dentro de Generate — ou seja, o erro de configuração
// aparecia em produção, e não no boot.
func TestNewGeneratorWithNilSourcePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewGeneratorWith(nil) deveria entrar em pânico")
		}
	}()
	uuid.NewGeneratorWith(nil)
}

// TestZeroGeneratorUsesDefaultEntropy confere que um Generator obtido
// fora dos construtores — valor zero embutido em outra struct, ou
// ponteiro nulo — ainda produz UUIDs válidos em vez de derrubar o
// processo.
//
// REGRESSÃO: antes da correção, ambos os casos causavam "nil pointer
// dereference". O primeiro é comum: como Generator é exportado, embuti-lo
// por valor em outra struct compila e parece correto.
func TestZeroGeneratorUsesDefaultEntropy(t *testing.T) {
	var byValue uuid.Generator
	var byPointer *uuid.Generator

	check := func(name string, u uuid.UUID) {
		t.Helper()
		if u.Version() != 7 || u.Variant() != 0b10 {
			t.Fatalf("%s: UUID inválido %s (versão %d, variante %b)",
				name, u, u.Version(), u.Variant())
		}
		if u == (uuid.UUID{}) {
			t.Fatalf("%s: devolveu o UUID zerado", name)
		}
	}

	for _, level := range []uuid.Level{uuid.Level1, uuid.Level2, uuid.Level3} {
		check("Generator zerado", byValue.Generate(level))
		check("ponteiro nulo", byPointer.Generate(level))

		s := byPointer.GenerateString(level)
		if len(s) != 36 {
			t.Fatalf("ponteiro nulo: GenerateString devolveu %d caracteres: %q", len(s), s)
		}
	}

	// A tolerância é regra do tipo inteiro (docs/SPEC.md seção 5.2, caso
	// 3): vale também para as versões 4 e 8, que não passam por Generate.
	for name, g := range map[string]*uuid.Generator{"Generator zerado": &byValue, "ponteiro nulo": byPointer} {
		if u := g.GenerateV4(); u.Version() != 4 || u.Variant() != 0b10 || u.IsZero() {
			t.Fatalf("%s: GenerateV4 devolveu %s", name, u)
		}
		if u := g.GenerateV8(); u.Version() != 8 || u.Variant() != 0b10 || u.IsZero() {
			t.Fatalf("%s: GenerateV8 devolveu %s", name, u)
		}
	}
}

// countingSource devolve uma fonte de entropia determinística que conta
// quantas vezes foi chamada.
func countingSource(calls *atomic.Int64, v uint64) func() uint64 {
	return func() uint64 {
		calls.Add(1)
		return v
	}
}

// TestEntropyDrawsPerLevel trava quantas palavras de 64 bits cada nível
// consome. Nos níveis 2 e 3 rand_a carrega os microssegundos, portanto
// uma única palavra basta; só o nível 1 (e os níveis desconhecidos, que
// se comportam como ele) precisa de duas.
//
// Importa para quem usa NewGeneratorWith com crypto/rand: cada chamada
// extra é uma leitura de entropia criptográfica desperdiçada.
func TestEntropyDrawsPerLevel(t *testing.T) {
	cases := []struct {
		level uuid.Level
		draws int64
	}{
		{uuid.Level1, 2},
		{uuid.Level2, 1},
		{uuid.Level3, 1},
		{uuid.Level(0), 2},
		{uuid.Level(99), 2},
	}
	for _, c := range cases {
		var calls atomic.Int64
		g := uuid.NewGeneratorWith(countingSource(&calls, 0))
		g.Generate(c.level)
		if got := calls.Load(); got != c.draws {
			t.Errorf("nível %d: fonte chamada %d vezes, esperado %d", c.level, got, c.draws)
		}
	}
}

// TestUnknownLevelBehavesAsLevel1 confere que um nível desconhecido não
// grava tempo em rand_a nem no topo de rand_b: os bits têm de vir da
// entropia, exatamente como no nível 1.
func TestUnknownLevelBehavesAsLevel1(t *testing.T) {
	g := uuid.NewGeneratorWith(constantSource(^uint64(0)))
	for _, level := range []uuid.Level{uuid.Level(0), uuid.Level(4), uuid.Level(99), uuid.Level(255)} {
		u := g.Generate(level)
		if u[6] != 0x7F || u[7] != 0xFF {
			t.Errorf("nível %d: rand_a = %#x%02x, esperado 0xfff (aleatório, como no nível 1)",
				level, u[6]&0x0F, u[7])
		}
		if u[8] != 0xBF {
			t.Errorf("nível %d: byte 8 = %#x, esperado 0xbf (variante 10 + rand_b aleatório)", level, u[8])
		}
	}
}

// TestPackageLevelShortcuts cobre diretamente os atalhos de pacote, que
// usam o gerador padrão interno e até aqui só eram exercitados de forma
// indireta.
func TestPackageLevelShortcuts(t *testing.T) {
	for _, level := range []uuid.Level{uuid.Level1, uuid.Level2, uuid.Level3} {
		u := uuid.Generate(level)
		if u.Version() != 7 || u.Variant() != 0b10 {
			t.Fatalf("Generate(nível %d): UUID inválido %s", level, u)
		}

		s := uuid.GenerateString(level)
		back, err := uuid.FromString(s)
		if err != nil {
			t.Fatalf("GenerateString(nível %d) devolveu %q, que FromString rejeitou: %v", level, s, err)
		}
		if back.String() != s {
			t.Fatalf("GenerateString(nível %d): round-trip divergiu (%q -> %q)", level, s, back.String())
		}
		if back.Version() != 7 || back.Variant() != 0b10 {
			t.Fatalf("GenerateString(nível %d): UUID inválido %s", level, s)
		}

		// Os atalhos precisam produzir valores distintos a cada chamada.
		if uuid.Generate(level) == u {
			t.Fatalf("Generate(nível %d) repetiu o mesmo valor em chamadas consecutivas", level)
		}
	}
}
