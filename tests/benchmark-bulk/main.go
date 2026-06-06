// Comando benchmark-bulk gera 1.000.000 de UUIDv7 de cada nível e
// imprime um relatório de tempo e throughput.
//
// Uso:
//
//	go run ./tests/benchmark-bulk            # 1.000.000 por nível
//	go run ./tests/benchmark-bulk -n 5000000 # quantidade personalizada
package main

import (
	"flag"
	"fmt"
	"time"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

func main() {
	n := flag.Int("n", 1_000_000, "quantidade de UUIDs por cenário")
	flag.Parse()

	gen := uuid.NewGenerator()

	// evita eliminação por otimização
	var bu uuid.UUID
	var bs string

	fmt.Printf("== Benchmark em massa — %d UUIDs por cenário ==\n\n", *n)
	fmt.Printf("%-22s %14s %16s %16s\n", "Cenário", "Tempo total", "ns por UUID", "UUIDs por ms")
	fmt.Printf("%-22s %14s %16s %16s\n", "----------------------", "--------------", "----------------", "----------------")

	report := func(name string, fn func()) {
		start := time.Now()
		fn()
		dur := time.Since(start)
		nsPerUUID := float64(dur.Nanoseconds()) / float64(*n)
		perMs := float64(*n) / (float64(dur.Nanoseconds()) / 1e6)
		fmt.Printf("%-22s %14s %16.1f %16.0f\n", name, dur.Round(time.Microsecond), nsPerUUID, perMs)
	}

	report("Nível 1 (binário)", func() {
		for i := 0; i < *n; i++ {
			bu = gen.Generate(uuid.Level1)
		}
	})
	report("Nível 2 (binário)", func() {
		for i := 0; i < *n; i++ {
			bu = gen.Generate(uuid.Level2)
		}
	})
	report("Nível 3 (binário)", func() {
		for i := 0; i < *n; i++ {
			bu = gen.Generate(uuid.Level3)
		}
	})
	report("Nível 1 (string)", func() {
		for i := 0; i < *n; i++ {
			bs = gen.GenerateString(uuid.Level1)
		}
	})
	report("Nível 2 (string)", func() {
		for i := 0; i < *n; i++ {
			bs = gen.GenerateString(uuid.Level2)
		}
	})
	report("Nível 3 (string)", func() {
		for i := 0; i < *n; i++ {
			bs = gen.GenerateString(uuid.Level3)
		}
	})

	_ = bu
	_ = bs
	fmt.Printf("\nExemplos gerados agora:\n")
	fmt.Printf("  Nível 1: %s\n", gen.GenerateString(uuid.Level1))
	fmt.Printf("  Nível 2: %s\n", gen.GenerateString(uuid.Level2))
	fmt.Printf("  Nível 3: %s\n", gen.GenerateString(uuid.Level3))
}
