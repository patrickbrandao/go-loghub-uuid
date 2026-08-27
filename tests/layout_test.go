package tests

import (
	"testing"
	"time"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// constantSource devolve uma fonte de entropia determinística, para que
// os testes possam isolar os bits de tempo dos bits aleatórios.
func constantSource(v uint64) func() uint64 {
	return func() uint64 { return v }
}

// TestLayoutZeroEntropy fixa o layout de bits usando entropia nula: tudo
// o que não for zero nos 16 bytes é, necessariamente, versão, variante
// ou tempo.
func TestLayoutZeroEntropy(t *testing.T) {
	g := uuid.NewGeneratorWith(constantSource(0))

	// Nível 1: sem entropia, rand_a e rand_b ficam zerados; sobram apenas
	// os milissegundos, a versão e a variante.
	u1 := g.Generate(uuid.Level1)
	if u1[6] != 0x70 {
		t.Errorf("nível 1: byte 6 = %#x, esperado 0x70 (versão 7 + rand_a zerado)", u1[6])
	}
	if u1[7] != 0x00 {
		t.Errorf("nível 1: byte 7 = %#x, esperado 0x00", u1[7])
	}
	if u1[8] != 0x80 {
		t.Errorf("nível 1: byte 8 = %#x, esperado 0x80 (variante 10 + rand_b zerado)", u1[8])
	}
	for i := 9; i < 16; i++ {
		if u1[i] != 0x00 {
			t.Errorf("nível 1: byte %d = %#x, esperado 0x00", i, u1[i])
		}
	}

	// Nível 3: rand_a carrega os microssegundos e o topo de rand_b os
	// nanossegundos; os 52 bits baixos permanecem zerados.
	u3 := g.Generate(uuid.Level3)
	for i := 10; i < 16; i++ {
		if u3[i] != 0x00 {
			t.Errorf("nível 3: byte %d = %#x, esperado 0x00 (parte aleatória de rand_b)", i, u3[i])
		}
	}
	// Os 4 bits baixos do byte 9 também pertencem à parte aleatória.
	if u3[9]&0x0F != 0x00 {
		t.Errorf("nível 3: nibble baixo do byte 9 = %#x, esperado 0x0", u3[9]&0x0F)
	}
}

// TestLayoutFullEntropy fixa as máscaras de versão e variante usando
// entropia com todos os bits em 1: os únicos bits que podem permanecer
// fixos são os 4 da versão e os 2 da variante.
func TestLayoutFullEntropy(t *testing.T) {
	g := uuid.NewGeneratorWith(constantSource(^uint64(0)))

	u := g.Generate(uuid.Level1)
	if u[6] != 0x7F {
		t.Errorf("byte 6 = %#x, esperado 0x7f (versão 7 + rand_a em 1)", u[6])
	}
	if u[7] != 0xFF {
		t.Errorf("byte 7 = %#x, esperado 0xff", u[7])
	}
	if u[8] != 0xBF {
		t.Errorf("byte 8 = %#x, esperado 0xbf (variante 10 + rand_b em 1)", u[8])
	}
	for i := 9; i < 16; i++ {
		if u[i] != 0xFF {
			t.Errorf("byte %d = %#x, esperado 0xff", i, u[i])
		}
	}
	if u.Version() != 7 {
		t.Errorf("versão = %d, esperado 7", u.Version())
	}
	if u.Variant() != 0b10 {
		t.Errorf("variante = %b, esperado 10", u.Variant())
	}
}

// TestVersionVariantAllLevels amplia TestVersionVariant para muitas
// amostras e para níveis desconhecidos, que devem cair em Nível 1.
func TestVersionVariantAllLevels(t *testing.T) {
	g := uuid.NewGenerator()
	levels := []uuid.Level{uuid.Level1, uuid.Level2, uuid.Level3, uuid.Level(0), uuid.Level(99), uuid.Level(255)}
	for _, level := range levels {
		for i := 0; i < 20_000; i++ {
			u := g.Generate(level)
			if u.Version() != 7 {
				t.Fatalf("nível %d: versão = %d, esperado 7", level, u.Version())
			}
			if u.Variant() != 0b10 {
				t.Fatalf("nível %d: variante = %b, esperado 10", level, u.Variant())
			}
		}
	}
}

// TestTimestampMatchesClock confere que os 48 bits de milissegundos
// gravados correspondem ao relógio no momento da geração.
func TestTimestampMatchesClock(t *testing.T) {
	g := uuid.NewGenerator()
	for _, level := range []uuid.Level{uuid.Level1, uuid.Level2, uuid.Level3} {
		before := time.Now().UnixMilli()
		u := g.Generate(level)
		after := time.Now().UnixMilli()

		ms := int64(u[0])<<40 | int64(u[1])<<32 | int64(u[2])<<24 |
			int64(u[3])<<16 | int64(u[4])<<8 | int64(u[5])
		if ms < before || ms > after {
			t.Fatalf("nível %d: unix_ts_ms = %d fora do intervalo [%d, %d]", level, ms, before, after)
		}
	}
}

// TestSubMillisecondMatchesClock confere que os microssegundos (níveis 2
// e 3) e os nanossegundos (nível 3) gravados correspondem ao relógio.
// Usa entropia nula para que nenhum bit aleatório interfira na leitura.
func TestSubMillisecondMatchesClock(t *testing.T) {
	g := uuid.NewGeneratorWith(constantSource(0))

	for i := 0; i < 1_000; i++ {
		before := time.Now().UnixNano()
		u := g.Generate(uuid.Level3)
		after := time.Now().UnixNano()

		imported := uuid.ImportBinary(u)
		reconstructed := imported.Seconds*1_000_000_000 +
			int64(imported.Milliseconds)*1_000_000 +
			int64(imported.Microseconds)*1_000 +
			int64(imported.Nanoseconds)

		if reconstructed < before || reconstructed > after {
			t.Fatalf("instante reconstruído %d fora do intervalo [%d, %d] (uuid %s)",
				reconstructed, before, after, u)
		}
	}
}
