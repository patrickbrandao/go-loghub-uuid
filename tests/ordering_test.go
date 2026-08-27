package tests

import (
	"sync"
	"testing"
	"time"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// timeKey reduz um UUID ao instante nele embutido, na resolução do nível
// informado. É o valor que precisa ditar a ordenação.
func timeKey(u uuid.UUID, level uuid.Level) int64 {
	tm := uuid.ImportBinary(u)
	key := tm.Seconds*1_000 + int64(tm.Milliseconds)
	if level == uuid.Level2 || level == uuid.Level3 {
		key = key*1_000 + int64(tm.Microseconds)
	}
	if level == uuid.Level3 {
		key = key*1_000 + int64(tm.Nanoseconds)
	}
	return key
}

// TestOrderingFollowsEmbeddedTime confere a garantia central da
// biblioteca, de forma independente da resolução do relógio: sempre que
// o instante embutido em B for maior que o de A, a string de B tem de
// ser maior que a de A (e o binário também).
//
// Empates de instante não são avaliados aqui: nesses casos o desempate é
// aleatório, por não existir contador monotônico. Ver TestTieRateReport.
func TestOrderingFollowsEmbeddedTime(t *testing.T) {
	g := uuid.NewGenerator()
	for _, level := range []uuid.Level{uuid.Level1, uuid.Level2, uuid.Level3} {
		prev := g.Generate(level)
		for i := 0; i < 200_000; i++ {
			cur := g.Generate(level)

			prevKey, curKey := timeKey(prev, level), timeKey(cur, level)
			if curKey > prevKey {
				if cur.String() <= prev.String() {
					t.Fatalf("nível %d: instante avançou (%d -> %d) mas a string regrediu:\n  %s\n  %s",
						level, prevKey, curKey, prev.String(), cur.String())
				}
				if !greaterBytes(cur, prev) {
					t.Fatalf("nível %d: instante avançou (%d -> %d) mas o binário regrediu:\n  %x\n  %x",
						level, prevKey, curKey, [16]byte(prev), [16]byte(cur))
				}
			}
			prev = cur
		}
	}
}

// greaterBytes compara dois UUIDs byte a byte (ordem de rede).
func greaterBytes(a, b uuid.UUID) bool {
	for i := 0; i < 16; i++ {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// TestStringOrderMatchesBinaryOrder confere que comparar as strings
// canônicas equivale a comparar os 16 bytes — base da promessa de
// "ordenação lexicográfica é ordenação cronológica".
func TestStringOrderMatchesBinaryOrder(t *testing.T) {
	g := uuid.NewGenerator()
	samples := make([]uuid.UUID, 5_000)
	for i := range samples {
		samples[i] = g.Generate(uuid.Level3)
	}
	for i := 1; i < len(samples); i++ {
		a, b := samples[i-1], samples[i]
		binGreater := greaterBytes(a, b)
		strGreater := a.String() > b.String()
		if binGreater != strGreater {
			t.Fatalf("ordem binária e ordem de string divergiram:\n  %s\n  %s", a, b)
		}
	}
}

// TestTieRateReport mede quantos UUIDs consecutivos caem no mesmo
// instante embutido — situação em que a ordenação passa a depender de
// bits aleatórios. Não falha: apenas registra o número, que depende
// inteiramente da resolução do relógio do host.
//
// Ver tasks/NEWS.md, item MELHORIA-01 (contador monotônico).
func TestTieRateReport(t *testing.T) {
	g := uuid.NewGenerator()
	for _, level := range []uuid.Level{uuid.Level1, uuid.Level2, uuid.Level3} {
		const samples = 100_000
		prev := g.Generate(level)
		ties, regressions := 0, 0
		for i := 0; i < samples; i++ {
			cur := g.Generate(level)
			if timeKey(cur, level) == timeKey(prev, level) {
				ties++
				if cur.String() < prev.String() {
					regressions++
				}
			}
			prev = cur
		}
		t.Logf("nível %d: %d/%d pares no mesmo instante (%.1f%%), %d deles fora de ordem",
			level, ties, samples, 100*float64(ties)/float64(samples), regressions)
	}

	distinct := map[int64]bool{}
	for i := 0; i < 100_000; i++ {
		distinct[time.Now().UnixNano()] = true
	}
	t.Logf("relógio do host: %d instantes distintos em 100000 leituras de time.Now()", len(distinct))
}

// TestUniqueness confere que não há UUIDs repetidos em um volume alto,
// para cada nível.
func TestUniqueness(t *testing.T) {
	if testing.Short() {
		t.Skip("pulado em modo -short")
	}
	const samples = 1_000_000
	for _, level := range []uuid.Level{uuid.Level1, uuid.Level2, uuid.Level3} {
		g := uuid.NewGenerator()
		seen := make(map[uuid.UUID]struct{}, samples)
		for i := 0; i < samples; i++ {
			u := g.Generate(level)
			if _, dup := seen[u]; dup {
				t.Fatalf("nível %d: UUID duplicado após %d gerações: %s", level, i, u)
			}
			seen[u] = struct{}{}
		}
	}
}

// TestConcurrentUniqueness gera de várias goroutines ao mesmo tempo e
// confere que nada se repete nem se corrompe. Rode também com -race:
//
//	go test ./tests/ -race -run TestConcurrentUniqueness
func TestConcurrentUniqueness(t *testing.T) {
	const goroutines = 64
	const perGoroutine = 20_000

	g := uuid.NewGenerator()
	results := make([][]uuid.UUID, goroutines)
	var wg sync.WaitGroup
	for w := 0; w < goroutines; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			batch := make([]uuid.UUID, perGoroutine)
			for i := range batch {
				batch[i] = g.Generate(uuid.Level3)
			}
			results[w] = batch
		}(w)
	}
	wg.Wait()

	seen := make(map[uuid.UUID]struct{}, goroutines*perGoroutine)
	for _, batch := range results {
		for _, u := range batch {
			if u.Version() != 7 || u.Variant() != 0b10 {
				t.Fatalf("UUID corrompido gerado em paralelo: %s", u)
			}
			if _, dup := seen[u]; dup {
				t.Fatalf("UUID duplicado gerado em paralelo: %s", u)
			}
			seen[u] = struct{}{}
		}
	}
}
