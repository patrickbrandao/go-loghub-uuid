package tests

import (
	"sync/atomic"
	"testing"
	"time"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// instanteConhecido tem os quatro campos de tempo distintos e diferentes
// de zero — 123 ms, 456 us, 789 ns —, de modo que uma troca de campos no
// empacotamento apareça no teste em vez de passar despercebida.
var instanteConhecido = time.Date(2026, 9, 11, 12, 34, 56, 123_456_789, time.UTC)

// TestGenerateAtRoundTripsThroughImport é o teste central da API: gerar
// para um instante conhecido e ler de volta com ImportBinary devolve
// exatamente os campos que o nível grava.
//
// ImportBinary é cega quanto ao nível por projeto: ela sempre lê rand_a
// como microssegundos e o topo de rand_b como nanossegundos. Nos níveis
// em que esses campos são entropia, o teste confere apenas que o valor
// lido cabe na faixa dos bits, e não que ele seja o tempo de origem.
func TestGenerateAtRoundTripsThroughImport(t *testing.T) {
	const (
		segundos = int64(1_789_130_096)
		milis    = 123
		micros   = 456
		nanos    = 789
	)

	for _, caso := range []struct {
		level         uuid.Level
		microEmbutido bool
		nanoEmbutido  bool
	}{
		{uuid.Level1, false, false},
		{uuid.Level2, true, false},
		{uuid.Level3, true, true},
	} {
		lido := uuid.ImportBinary(uuid.GenerateAt(caso.level, instanteConhecido))

		if lido.Seconds != segundos {
			t.Errorf("nível %d: Seconds = %d, esperado %d", caso.level, lido.Seconds, segundos)
		}
		if lido.Milliseconds != milis {
			t.Errorf("nível %d: Milliseconds = %d, esperado %d", caso.level, lido.Milliseconds, milis)
		}

		if caso.microEmbutido {
			if lido.Microseconds != micros {
				t.Errorf("nível %d: Microseconds = %d, esperado %d", caso.level, lido.Microseconds, micros)
			}
		} else if lido.Microseconds > 4095 {
			t.Errorf("nível %d: Microseconds = %d, acima dos 12 bits de rand_a", caso.level, lido.Microseconds)
		}

		if caso.nanoEmbutido {
			if lido.Nanoseconds != nanos {
				t.Errorf("nível %d: Nanoseconds = %d, esperado %d", caso.level, lido.Nanoseconds, nanos)
			}
		} else if lido.Nanoseconds > 1023 {
			t.Errorf("nível %d: Nanoseconds = %d, acima dos 10 bits lidos", caso.level, lido.Nanoseconds)
		}
	}
}

// TestGenerateAtMatchesGenerateLayout prova que o empacotamento
// compartilhado de construct.go não divergiu da cópia que Generate
// mantém no caminho quente.
//
// Com a mesma fonte de entropia constante, gera pelo relógio, lê o
// instante embutido de volta e regera para aquele instante: os 16 bytes
// têm de ser idênticos. É esta a trava contra a duplicação deliberada
// registrada em docs/10-armadilhas-e-decisoes-de-projeto.md seção 11.2.
func TestGenerateAtMatchesGenerateLayout(t *testing.T) {
	for _, level := range allLevels {
		g := uuid.NewGeneratorWith(constantSource(0xA5A5_5A5A_C3C3_3C3C))

		peloRelogio := g.Generate(level)
		instante, ok := peloRelogio.TimestampWithLevel(level)
		if !ok {
			t.Fatalf("nível %d: TimestampWithLevel recusou um UUID recém-gerado", level)
		}

		porInstante := g.GenerateAt(level, instante)
		if porInstante != peloRelogio {
			t.Errorf("nível %d: o empacotamento divergiu do caminho quente\n  Generate:   %s\n  GenerateAt: %s",
				level, peloRelogio, porInstante)
		}
	}
}

// TestGenerateAtFallsInsideBounds amarra esta API à das fronteiras: o
// que se gera para um instante tem de caber entre o mínimo e o máximo
// daquele mesmo instante, por construção.
func TestGenerateAtFallsInsideBounds(t *testing.T) {
	instantes := []time.Time{
		time.Unix(0, 0).UTC(),
		instanteConhecido,
		time.Now(),
	}
	for _, level := range allLevels {
		for _, instante := range instantes {
			lo, hi := uuid.MinAt(level, instante), uuid.MaxAt(level, instante)
			for i := 0; i < 2_000; i++ {
				u := uuid.GenerateAt(level, instante)
				if u.Compare(lo) < 0 || u.Compare(hi) > 0 {
					t.Fatalf("nível %d, %s: %s ficou fora de [%s, %s]", level, instante, u, lo, hi)
				}
			}
		}
	}
}

// TestGenerateAtIsNotDeterministic confere a decisão de projeto: os bits
// livres são sorteados, então duas chamadas com o mesmo instante dão
// UUIDs diferentes. A forma determinística é MinAt, não esta.
func TestGenerateAtIsNotDeterministic(t *testing.T) {
	for _, level := range allLevels {
		vistos := make(map[uuid.UUID]struct{}, 1_000)
		for i := 0; i < 1_000; i++ {
			vistos[uuid.GenerateAt(level, instanteConhecido)] = struct{}{}
		}
		// Mil sorteios sobre 52 bits livres (o pior caso, no Nível 3)
		// não repetem na prática; exigir unicidade total é seguro.
		if len(vistos) != 1_000 {
			t.Errorf("nível %d: %d valores distintos em 1000 chamadas, esperado 1000",
				level, len(vistos))
		}

		// Os campos de tempo, esses, têm de ser sempre os mesmos.
		referencia := uuid.ImportBinary(uuid.GenerateAt(level, instanteConhecido))
		for u := range vistos {
			if lido := uuid.ImportBinary(u); lido.Seconds != referencia.Seconds ||
				lido.Milliseconds != referencia.Milliseconds {
				t.Fatalf("nível %d: o carimbo variou entre chamadas do mesmo instante", level)
			}
		}
	}
}

// TestGenerateAtOrdering confere que instantes crescentes produzem
// UUIDs crescentes, na comparação de bytes e de strings, quando a
// diferença é maior que a resolução do nível.
func TestGenerateAtOrdering(t *testing.T) {
	for _, level := range allLevels {
		anterior := uuid.GenerateAt(level, instanteConhecido)
		instante := instanteConhecido
		for i := 0; i < 500; i++ {
			instante = instante.Add(levelResolution[level])
			atual := uuid.GenerateAt(level, instante)

			if atual.Compare(anterior) <= 0 {
				t.Fatalf("nível %d, passo %d: o binário regrediu\n  %s\n  %s",
					level, i, anterior, atual)
			}
			if atual.String() <= anterior.String() {
				t.Fatalf("nível %d, passo %d: a string regrediu\n  %s\n  %s",
					level, i, anterior, atual)
			}
			anterior = atual
		}
	}
}

// TestGenerateAtEntropyDrawsPerLevel trava o mesmo consumo de entropia
// de Generate: uma palavra de 64 bits nos níveis 2 e 3, onde rand_a
// carrega os microssegundos, e duas no nível 1 e nos desconhecidos.
func TestGenerateAtEntropyDrawsPerLevel(t *testing.T) {
	casos := []struct {
		level    uuid.Level
		sorteios int64
	}{
		{uuid.Level1, 2},
		{uuid.Level2, 1},
		{uuid.Level3, 1},
		{uuid.Level(0), 2},
		{uuid.Level(99), 2},
	}
	for _, caso := range casos {
		var chamadas atomic.Int64
		g := uuid.NewGeneratorWith(countingSource(&chamadas, 0))
		g.GenerateAt(caso.level, instanteConhecido)
		if got := chamadas.Load(); got != caso.sorteios {
			t.Errorf("nível %d: fonte chamada %d vezes, esperado %d", caso.level, got, caso.sorteios)
		}
	}
}

// TestGenerateAtBeforeEpoch confere o piso na época Unix, idêntico ao de
// Generate: instantes anteriores a 1970 zeram os 48 bits de
// milissegundos em vez de corromperem os campos.
func TestGenerateAtBeforeEpoch(t *testing.T) {
	var valorZero time.Time
	anteriores := []time.Time{
		time.Date(1969, 12, 31, 23, 59, 59, 999_999_999, time.UTC),
		time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
		valorZero,
		time.Date(-300_000_000, 1, 1, 0, 0, 0, 0, time.UTC), // estoura a multiplicação
	}
	for _, level := range allLevels {
		for _, instante := range anteriores {
			u := uuid.GenerateAt(level, instante)
			for i := 0; i < 6; i++ {
				if u[i] != 0 {
					t.Errorf("nível %d, %s: byte %d = %#x, esperado 0", level, instante, i, u[i])
				}
			}
			if u.Version() != 7 || u.Variant() != 0b10 {
				t.Errorf("nível %d, %s: versão %d e variante %b", level, instante, u.Version(), u.Variant())
			}
		}
	}
}

// TestGenerateAtSaturatesAboveRange confere o teto: instantes além de
// 10889-08-02 saturam no último milissegundo representável em vez de
// darem a volta, inclusive quando o instante estoura a multiplicação por
// mil dentro da decomposição.
func TestGenerateAtSaturatesAboveRange(t *testing.T) {
	ultimoMilissegundo := time.UnixMilli(int64(1)<<48 - 1).UTC()
	acima := []time.Time{
		ultimoMilissegundo.Add(time.Millisecond),
		time.Date(20_000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(300_000_000, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	for _, level := range allLevels {
		for _, instante := range acima {
			u := uuid.GenerateAt(level, instante)
			for i := 0; i < 6; i++ {
				if u[i] != 0xFF {
					t.Errorf("nível %d, %s: byte %d = %#x, esperado 0xff", level, instante, i, u[i])
				}
			}
			// Continua dentro da fronteira do instante saturado.
			teto := uuid.MaxAt(level, instante)
			if u.Compare(teto) > 0 {
				t.Errorf("nível %d, %s: %s passou do teto %s", level, instante, u, teto)
			}
		}
	}
}

// TestGenerateAtUnknownLevelBehavesAsLevel1 confere que um nível
// desconhecido não grava tempo em rand_a nem no topo de rand_b: os bits
// vêm da entropia, como no Nível 1 e como já faz Generate.
func TestGenerateAtUnknownLevelBehavesAsLevel1(t *testing.T) {
	for _, level := range []uuid.Level{0, 4, 99, 255} {
		g := uuid.NewGeneratorWith(constantSource(^uint64(0)))
		u := g.GenerateAt(level, instanteConhecido)
		if u[6] != 0x7F || u[7] != 0xFF {
			t.Errorf("nível %d: rand_a = %#x%02x, esperado 0xfff (aleatório)", level, u[6]&0x0F, u[7])
		}
		if u[8] != 0xBF {
			t.Errorf("nível %d: byte 8 = %#x, esperado 0xbf", level, u[8])
		}
	}
}

// TestGenerateAtStringMatchesBinary confere que a variante em texto é a
// forma canônica do mesmo empacotamento, e não outro caminho.
func TestGenerateAtStringMatchesBinary(t *testing.T) {
	g := uuid.NewGeneratorWith(constantSource(0x0123_4567_89AB_CDEF))
	for _, level := range allLevels {
		binario := g.GenerateAt(level, instanteConhecido)
		texto := g.GenerateAtString(level, instanteConhecido)
		if binario.String() != texto {
			t.Errorf("nível %d: GenerateAtString devolveu %s, esperado %s", level, texto, binario)
		}
		if len(texto) != 36 {
			t.Errorf("nível %d: GenerateAtString devolveu %d caracteres", level, len(texto))
		}
	}
}

// TestGenerateAtFixedVectors trava o layout com entropia constante e um
// instante fixo, sem depender do relógio. O instante está a
// 1_789_130_096_123 ms da época (0x01A0_9076_BDFB nos 48 bits), com
// microssegundos 456 (0x1C8) e nanossegundos 789 (0x315).
func TestGenerateAtFixedVectors(t *testing.T) {
	casos := []struct {
		nome  string
		level uuid.Level
		quero string
	}{
		// Entropia toda em zero: só sobram os campos de tempo, a versão
		// e a variante.
		{"Nível 1, entropia nula", uuid.Level1, "01a09076-bdfb-7000-8000-000000000000"},
		{"Nível 2, entropia nula", uuid.Level2, "01a09076-bdfb-71c8-8000-000000000000"},
		{"Nível 3, entropia nula", uuid.Level3, "01a09076-bdfb-71c8-b150-000000000000"},
	}
	for _, caso := range casos {
		g := uuid.NewGeneratorWith(constantSource(0))
		if got := g.GenerateAtString(caso.level, instanteConhecido); got != caso.quero {
			t.Errorf("%s: %s, esperado %s", caso.nome, got, caso.quero)
		}
	}

	// Com a entropia toda em um, o resultado é exatamente o teto do
	// instante, que MaxAt calcula pelo outro caminho.
	for _, level := range allLevels {
		g := uuid.NewGeneratorWith(constantSource(^uint64(0)))
		got := g.GenerateAt(level, instanteConhecido)
		if want := uuid.MaxAt(level, instanteConhecido); got != want {
			t.Errorf("nível %d, entropia toda em um: %s, esperado o teto %s", level, got, want)
		}
	}
}

// TestGenerateAtOnZeroGenerator confere que a geração por instante tem a
// mesma proteção de Generate contra um Generator obtido fora dos
// construtores: valor zero embutido em outra struct, ou ponteiro nulo.
// Nos dois casos ela recorre ao gerador padrão do pacote em vez de
// derrubar o processo.
func TestGenerateAtOnZeroGenerator(t *testing.T) {
	var porValor uuid.Generator
	var porPonteiro *uuid.Generator

	confere := func(nome string, u uuid.UUID) {
		t.Helper()
		if u.Version() != 7 || u.Variant() != 0b10 {
			t.Fatalf("%s: UUID inválido %s (versão %d, variante %b)",
				nome, u, u.Version(), u.Variant())
		}
		// O instante pedido tem os 48 bits de milissegundos diferentes
		// de zero, então um UUID zerado denunciaria que nada foi gravado.
		if u == (uuid.UUID{}) {
			t.Fatalf("%s: devolveu o UUID zerado", nome)
		}
	}

	for _, level := range allLevels {
		confere("Generator zerado", porValor.GenerateAt(level, instanteConhecido))
		confere("ponteiro nulo", porPonteiro.GenerateAt(level, instanteConhecido))

		if s := porPonteiro.GenerateAtString(level, instanteConhecido); len(s) != 36 {
			t.Fatalf("ponteiro nulo: GenerateAtString devolveu %d caracteres: %q", len(s), s)
		}
	}
}

// TestGenerateAtPackageShortcuts confere que os atalhos de pacote usam o
// gerador padrão interno e concordam entre si, como Generate e
// GenerateString já fazem.
func TestGenerateAtPackageShortcuts(t *testing.T) {
	for _, level := range allLevels {
		u := uuid.GenerateAt(level, instanteConhecido)
		s := uuid.GenerateAtString(level, instanteConhecido)

		if len(s) != 36 {
			t.Errorf("nível %d: GenerateAtString devolveu %d caracteres: %q", level, len(s), s)
		}
		lido, err := uuid.FromString(s)
		if err != nil {
			t.Fatalf("nível %d: GenerateAtString devolveu texto que não analisa: %v", level, err)
		}
		if lido.Version() != 7 || lido.Variant() != 0b10 {
			t.Errorf("nível %d: atalho em texto produziu versão %d e variante %b",
				level, lido.Version(), lido.Variant())
		}

		// Os bits livres são sorteados, então os dois valores diferem;
		// os campos de tempo, não.
		if a, b := uuid.ImportBinary(u), uuid.ImportBinary(lido); a.Seconds != b.Seconds ||
			a.Milliseconds != b.Milliseconds {
			t.Errorf("nível %d: os atalhos discordaram no carimbo", level)
		}
	}
}
