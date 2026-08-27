package tests

import (
	"runtime"
	"sync"
	"testing"
	"time"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

var g = uuid.NewGenerator()

// sink evita que o compilador elimine as gerações nos benchmarks.
var sinkU uuid.UUID
var sinkS string

func BenchmarkGenerateLevel1(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkU = g.Generate(uuid.Level1)
	}
}

func BenchmarkGenerateLevel2(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkU = g.Generate(uuid.Level2)
	}
}

func BenchmarkGenerateLevel3(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkU = g.Generate(uuid.Level3)
	}
}

func BenchmarkGenerateStringLevel1(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkS = g.GenerateString(uuid.Level1)
	}
}

func BenchmarkGenerateStringLevel3(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkS = g.GenerateString(uuid.Level3)
	}
}

// BenchmarkGenerateLevel3Parallel mede o throughput com várias
// goroutines, simulando "centenas de threads" usando o mesmo Generator.
func BenchmarkGenerateLevel3Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var u uuid.UUID
		for pb.Next() {
			u = g.Generate(uuid.Level3)
		}
		// Mantém o valor vivo sem escrever no sink global: publicar em
		// sinkU a partir de várias goroutines é uma corrida de dados.
		runtime.KeepAlive(u)
	})
}

// BenchmarkFromString mede o caminho de análise da string canônica —
// o único que recebe dado externo, e por isso o que mais importa manter
// rápido e sem alocações.
func BenchmarkFromString(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkU, _ = uuid.FromString(canonical)
	}
}

// BenchmarkImportBinary mede a extração das propriedades de tempo.
func BenchmarkImportBinary(b *testing.B) {
	u := uuid.Generate(uuid.Level3)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkT = uuid.ImportBinary(u)
	}
}

// TestMassOneMillion gera 1.000.000 de UUIDs de cada nível (binário e
// string) e imprime o tempo consumido e o throughput. Rode com:
//
//	go test ./tests/ -run TestMassOneMillion -v
func TestMassOneMillion(t *testing.T) {
	if testing.Short() {
		t.Skip("pulado em modo -short")
	}
	const total = 1_000_000
	gen := uuid.NewGenerator()

	// Passagem de aquecimento: sem ela o primeiro cenário medido paga
	// sozinho o custo de aquecer cache de instruções, frequência de CPU e
	// o sync.Pool, e aparece artificialmente mais lento que os demais.
	for i := 0; i < 100_000; i++ {
		sinkU = gen.Generate(uuid.Level1)
		sinkU = gen.Generate(uuid.Level3)
		sinkS = gen.GenerateString(uuid.Level3)
	}

	measure := func(name string, fn func()) {
		start := time.Now()
		fn()
		dur := time.Since(start)
		perUUID := dur / total
		// Taxa a partir de nanossegundos: dur.Milliseconds()+1 arredonda
		// para baixo e ainda soma 1 ms, distorcendo execuções curtas.
		perMs := float64(total) / (float64(dur.Nanoseconds()) / 1e6)
		t.Logf("%-26s %10d UUIDs em %12v  (%8v/UUID, ~%.0f UUIDs/ms)",
			name, total, dur, perUUID, perMs)
	}

	measure("Nível 1 binário", func() {
		for i := 0; i < total; i++ {
			sinkU = gen.Generate(uuid.Level1)
		}
	})
	measure("Nível 2 binário", func() {
		for i := 0; i < total; i++ {
			sinkU = gen.Generate(uuid.Level2)
		}
	})
	measure("Nível 3 binário", func() {
		for i := 0; i < total; i++ {
			sinkU = gen.Generate(uuid.Level3)
		}
	})
	measure("Nível 1 string", func() {
		for i := 0; i < total; i++ {
			sinkS = gen.GenerateString(uuid.Level1)
		}
	})
	measure("Nível 3 string", func() {
		for i := 0; i < total; i++ {
			sinkS = gen.GenerateString(uuid.Level3)
		}
	})
}

// TestMassConcurrent gera 1.000.000 de UUIDs de nível 3 distribuídos
// entre muitas goroutines, confirmando segurança e medindo throughput
// agregado.
func TestMassConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("pulado em modo -short")
	}
	const total = 1_000_000
	const goroutines = 256
	gen := uuid.NewGenerator()

	perGoroutine := total / goroutines
	// Cada goroutine publica em sua própria posição: escrever todas no
	// mesmo sink global é uma corrida de dados acusada por -race.
	last := make([]uuid.UUID, goroutines)
	var wg sync.WaitGroup
	start := time.Now()
	for w := 0; w < goroutines; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			var u uuid.UUID
			for i := 0; i < perGoroutine; i++ {
				u = gen.Generate(uuid.Level3)
			}
			last[w] = u
		}(w)
	}
	wg.Wait()
	dur := time.Since(start)
	sinkU = last[goroutines-1]
	t.Logf("concorrente: %d UUIDs em %d goroutines = %v (~%.0f UUIDs/ms)",
		goroutines*perGoroutine, goroutines, dur,
		float64(goroutines*perGoroutine)/(float64(dur.Nanoseconds())/1e6))
}
