package tests

import (
	"sync/atomic"
	"testing"
	"time"

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

	// A tolerância é regra do tipo inteiro (docs/06-relogio-e-concorrencia.md
	// seção 5.2, caso 3): vale também para os nomes do UUIDv7, por versão e
	// por nível, e para as versões 4 e 8, que não passam por Generate.
	for name, g := range map[string]*uuid.Generator{"Generator zerado": &byValue, "ponteiro nulo": byPointer} {
		if u := g.GenerateV7(); u.Version() != 7 || u.Variant() != 0b10 || u.IsZero() {
			t.Fatalf("%s: GenerateV7 devolveu %s", name, u)
		}
		if u := g.GenerateV7Level1(); u.Version() != 7 || u.Variant() != 0b10 || u.IsZero() {
			t.Fatalf("%s: GenerateV7Level1 devolveu %s", name, u)
		}
		if u := g.GenerateV7Level2(); u.Version() != 7 || u.Variant() != 0b10 || u.IsZero() {
			t.Fatalf("%s: GenerateV7Level2 devolveu %s", name, u)
		}
		if u := g.GenerateV7Level3(); u.Version() != 7 || u.Variant() != 0b10 || u.IsZero() {
			t.Fatalf("%s: GenerateV7Level3 devolveu %s", name, u)
		}
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

	// Cada nome consome o mesmo que o nível que ele apelida: duas palavras
	// no nome por versão e no Nível 1, uma nos níveis 2 e 3.
	names := []struct {
		name  string
		gen   func(*uuid.Generator) uuid.UUID
		draws int64
	}{
		{"GenerateV7", (*uuid.Generator).GenerateV7, 2},
		{"GenerateV7Level1", (*uuid.Generator).GenerateV7Level1, 2},
		{"GenerateV7Level2", (*uuid.Generator).GenerateV7Level2, 1},
		{"GenerateV7Level3", (*uuid.Generator).GenerateV7Level3, 1},
	}
	for _, n := range names {
		var calls atomic.Int64
		g := uuid.NewGeneratorWith(countingSource(&calls, 0))
		n.gen(g)
		if got := calls.Load(); got != n.draws {
			t.Errorf("%s: fonte chamada %d vezes, esperado %d", n.name, got, n.draws)
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

// TestGenerateV7IsLevel1 confere que o nome por versão do UUIDv7 é
// exatamente Generate(Level1): com entropia constante em um, rand_a e o
// topo de rand_b saem inteiros da fonte, sem microssegundos nem
// nanossegundos gravados, e o carimbo de milissegundos é o do relógio.
func TestGenerateV7IsLevel1(t *testing.T) {
	g := uuid.NewGeneratorWith(constantSource(^uint64(0)))
	before := time.Now().Truncate(time.Millisecond)
	u := g.GenerateV7()
	after := time.Now()

	checkShape(t, "GenerateV7", u, 7)
	if u[6] != 0x7F || u[7] != 0xFF {
		t.Errorf("rand_a = %#x%02x, esperado 0xfff (aleatório, como no Nível 1)", u[6]&0x0F, u[7])
	}
	if u[8] != 0xBF {
		t.Errorf("byte 8 = %#x, esperado 0xbf (variante 10 + rand_b aleatório)", u[8])
	}

	ts, ok := u.Timestamp()
	if !ok {
		t.Fatal("Timestamp deveria devolver verdadeiro para a versão 7")
	}
	if ts.Before(before) || ts.After(after) {
		t.Errorf("carimbo %s fora da janela [%s, %s]",
			ts.Format(time.RFC3339Nano), before.Format(time.RFC3339Nano), after.Format(time.RFC3339Nano))
	}
}

// TestGenerateV7LevelNamesMatchLevels prova que cada nome gera no nível
// que o nome diz, e não em outro. Com entropia constante em um, o UUID
// gerado pelo nome é lido de volta com o nível esperado e regerado por
// instante nesse mesmo nível: os 16 bytes têm de coincidir. Um nome que
// chamasse outro nível seria pego, porque os campos de tempo
// sub-milissegundo ficam em 0..999 e a entropia constante os deixa em
// 0xfff e 0x3ff, valores que nenhum campo de tempo assume. Os campos
// também são conferidos um a um, para a falha dizer qual bit divergiu.
func TestGenerateV7LevelNamesMatchLevels(t *testing.T) {
	names := []struct {
		name  string
		level uuid.Level
		gen   func(*uuid.Generator) uuid.UUID
	}{
		{"GenerateV7", uuid.Level1, (*uuid.Generator).GenerateV7},
		{"GenerateV7Level1", uuid.Level1, (*uuid.Generator).GenerateV7Level1},
		{"GenerateV7Level2", uuid.Level2, (*uuid.Generator).GenerateV7Level2},
		{"GenerateV7Level3", uuid.Level3, (*uuid.Generator).GenerateV7Level3},
	}
	for _, n := range names {
		g := uuid.NewGeneratorWith(constantSource(^uint64(0)))
		u := n.gen(g)
		checkShape(t, n.name, u, 7)

		// rand_a: entropia (0xfff) no Nível 1, microssegundos (0..999) nos
		// níveis 2 e 3.
		randA := uint16(u[6]&0x0F)<<8 | uint16(u[7])
		if n.level == uuid.Level1 && randA != 0x0FFF {
			t.Errorf("%s: rand_a = %#x, esperado 0xfff (aleatório, como no Nível 1)", n.name, randA)
		}
		if n.level != uuid.Level1 && randA > 999 {
			t.Errorf("%s: rand_a = %#x, esperado microssegundos em 0..999", n.name, randA)
		}

		// Topo de rand_b: entropia (0x3ff) nos níveis 1 e 2, nanossegundos
		// (0..999) no Nível 3.
		topB := uint16(u[8]&0x3F)<<4 | uint16(u[9]>>4)
		if n.level != uuid.Level3 && topB != 0x03FF {
			t.Errorf("%s: topo de rand_b = %#x, esperado 0x3ff (aleatório)", n.name, topB)
		}
		if n.level == uuid.Level3 && topB > 999 {
			t.Errorf("%s: topo de rand_b = %#x, esperado nanossegundos em 0..999", n.name, topB)
		}

		// Ida e volta pelo instante embutido, no nível que o nome declara.
		instante, ok := u.TimestampWithLevel(n.level)
		if !ok {
			t.Fatalf("%s: TimestampWithLevel recusou um UUID recém-gerado", n.name)
		}
		if porInstante := g.GenerateAt(n.level, instante); porInstante != u {
			t.Errorf("%s: gerou em outro nível que não o do nome\n  pelo nome:    %s\n  por instante: %s",
				n.name, u, porInstante)
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

	// Os nomes do UUIDv7, por versão e por nível, também têm atalho de
	// pacote sobre o gerador padrão.
	names := []struct {
		name string
		gen  func() uuid.UUID
	}{
		{"GenerateV7", uuid.GenerateV7},
		{"GenerateV7Level1", uuid.GenerateV7Level1},
		{"GenerateV7Level2", uuid.GenerateV7Level2},
		{"GenerateV7Level3", uuid.GenerateV7Level3},
	}
	for _, n := range names {
		u := n.gen()
		if u.Version() != 7 || u.Variant() != 0b10 {
			t.Fatalf("%s: UUID inválido %s", n.name, u)
		}
		if n.gen() == u {
			t.Fatalf("%s repetiu o mesmo valor em chamadas consecutivas", n.name)
		}
	}
}
