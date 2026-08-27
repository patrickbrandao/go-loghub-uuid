package tests

import (
	"testing"
	"time"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// TestVersionVariant garante que todos os níveis produzem UUIDs válidos
// (versão 7 e variante RFC 0b10).
func TestVersionVariant(t *testing.T) {
	g := uuid.NewGenerator()
	for _, level := range []uuid.Level{uuid.Level1, uuid.Level2, uuid.Level3} {
		u := g.Generate(level)
		if u.Version() != 7 {
			t.Fatalf("nível %d: versão = %d, esperado 7", level, u.Version())
		}
		if u.Variant() != 0b10 {
			t.Fatalf("nível %d: variante = %b, esperado 10", level, u.Variant())
		}
	}
}

// TestRoundTripString garante que binário -> string -> binário não altera os bits.
func TestRoundTripString(t *testing.T) {
	g := uuid.NewGenerator()
	for i := 0; i < 10_000; i++ {
		original := g.Generate(uuid.Level3)
		s := original.String()
		if len(s) != 36 {
			t.Fatalf("string com tamanho %d: %q", len(s), s)
		}
		back, err := uuid.FromString(s)
		if err != nil {
			t.Fatalf("FromString(%q) falhou: %v", s, err)
		}
		if back != original {
			t.Fatalf("round-trip divergiu:\n  ant: %x\n  dep: %x", original[:], back[:])
		}
	}
}

// TestConversionAliases garante que StringToBinary/BinaryToString
// se comportam como FromString/String.
func TestConversionAliases(t *testing.T) {
	u := uuid.Generate(uuid.Level2)
	s := uuid.BinaryToString(u)
	v, err := uuid.StringToBinary(s)
	if err != nil || v != u {
		t.Fatalf("apelidos de conversão divergiram: err=%v v=%x u=%x", err, v[:], u[:])
	}
}

// TestFromStringInvalid confirma a rejeição de entradas malformadas.
func TestFromStringInvalid(t *testing.T) {
	cases := []string{
		"",
		"abc",
		"0192f7c5-1a2b-7c3d-8e4f-aabbccddeef",    // curto
		"0192f7c5-1a2b-7c3d-8e4f-aabbccddeeffff", // longo
		"0192f7c5+1a2b-7c3d-8e4f-aabbccddeeff",   // hifen errado
		"0192f7c5-1a2b-7c3d-8e4f-aabbccddeegg",   // dígito inválido
	}
	for _, c := range cases {
		if _, err := uuid.FromString(c); err == nil {
			t.Fatalf("esperava erro para %q", c)
		}
	}
}

// TestImportLevel3 verifica que micro/nano gravados na geração
// reaparecem corretamente na importação (margem de tolerância para o
// avanço do relógio entre medições).
func TestImportLevel3(t *testing.T) {
	g := uuid.NewGenerator()
	before := time.Now()
	u := g.Generate(uuid.Level3)
	after := time.Now()

	tm, err := uuid.Import(u.String())
	if err != nil {
		t.Fatalf("Import falhou: %v", err)
	}

	// micro e nano precisam estar na faixa válida (0..999) para o nível 3.
	if tm.Microseconds < 0 || tm.Microseconds > 999 {
		t.Fatalf("microssegundos fora da faixa: %d", tm.Microseconds)
	}
	if tm.Nanoseconds < 0 || tm.Nanoseconds > 999 {
		t.Fatalf("nanossegundos fora da faixa: %d", tm.Nanoseconds)
	}

	// Reconstrói o instante importado e confere que cai no intervalo medido.
	reconstructed := time.Unix(tm.Seconds, int64(tm.Milliseconds)*1_000_000+
		int64(tm.Microseconds)*1_000+int64(tm.Nanoseconds))
	if reconstructed.Before(before.Add(-time.Millisecond)) ||
		reconstructed.After(after.Add(time.Millisecond)) {
		t.Fatalf("instante reconstruído %v fora do intervalo [%v, %v]",
			reconstructed, before, after)
	}
}

// TestMonotonicity confere a garantia real da biblioteca: quando o
// relógio avança entre duas gerações, a string do UUID posterior é
// estritamente maior que a do anterior.
//
// A versão anterior deste teste gerava 5.000 UUIDs em sequência fechada e
// tolerava no máximo 50 regressões. Isso media a resolução do relógio do
// host, e não a biblioteca: gerar um UUID custa ~45 ns, bem menos que o
// passo do relógio, então a maioria dos pares consecutivos cai no mesmo
// instante embutido e é desempatada por bits aleatórios — não existe
// contador monotônico. Em hosts com relógio de microssegundo (macOS) o
// teste falhava sempre, com ~2.000 regressões.
//
// A pausa entre gerações garante que o instante embutido avance em todos
// os hosts. Para a invariante de ordenação medida sem depender do
// relógio, ver TestOrderingFollowsEmbeddedTime; para o diagnóstico da
// taxa de empates, ver TestTieRateReport (ambos em ordering_test.go).
func TestMonotonicity(t *testing.T) {
	g := uuid.NewGenerator()
	previous := g.GenerateString(uuid.Level3)
	for i := 0; i < 200; i++ {
		// 200 µs: acima da resolução do relógio de qualquer host suportado,
		// e o nível 3 registra microssegundos, logo a chave de tempo avança.
		time.Sleep(200 * time.Microsecond)
		current := g.GenerateString(uuid.Level3)
		if current <= previous {
			t.Fatalf("iteração %d: o relógio avançou mas a ordenação regrediu:\n  ant: %s\n  dep: %s",
				i, previous, current)
		}
		previous = current
	}
}
