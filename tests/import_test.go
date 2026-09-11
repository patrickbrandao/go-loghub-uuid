package tests

import (
	"errors"
	"testing"
	"time"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// TestImportKnownVector fixa a extração de tempo para um UUID conhecido,
// travando a leitura dos 48 bits de milissegundos, de rand_a e do topo
// de rand_b.
func TestImportKnownVector(t *testing.T) {
	// unix_ts_ms = 0x0192f7c51a2b = 1728000000043
	// rand_a     = 0xc3d          = 3133
	// rand_b topo 10 bits = (0x0e << 4) | (0x4f >> 4) = 0xE4 = 228
	u := uuid.UUID{
		0x01, 0x92, 0xf7, 0xc5, 0x1a, 0x2b, 0x7c, 0x3d,
		0x8e, 0x4f, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
	}
	const totalMs = int64(0x0192f7c51a2b)

	got := uuid.ImportBinary(u)
	want := struct {
		seconds               int64
		millis, micros, nanos int
	}{
		seconds: totalMs / 1000,
		millis:  int(totalMs % 1000),
		micros:  0xC3D,
		nanos:   0xE4,
	}

	if got.Seconds != want.seconds {
		t.Errorf("Seconds = %d, esperado %d", got.Seconds, want.seconds)
	}
	if got.Milliseconds != want.millis {
		t.Errorf("Milliseconds = %d, esperado %d", got.Milliseconds, want.millis)
	}
	if got.Microseconds != want.micros {
		t.Errorf("Microseconds = %d, esperado %d", got.Microseconds, want.micros)
	}
	if got.Nanoseconds != want.nanos {
		t.Errorf("Nanoseconds = %d, esperado %d", got.Nanoseconds, want.nanos)
	}

	// Import (a partir da string) precisa concordar com ImportBinary.
	fromText, err := uuid.Import(u.String())
	if err != nil {
		t.Fatalf("Import falhou: %v", err)
	}
	if fromText != got {
		t.Fatalf("Import e ImportBinary divergiram: %+v vs %+v", fromText, got)
	}
}

// TestImportInvalidString confere que Import propaga o erro de formato e
// devolve a estrutura zerada.
func TestImportInvalidString(t *testing.T) {
	tm, err := uuid.Import("nao-e-um-uuid")
	if !errors.Is(err, uuid.ErrInvalidFormat) {
		t.Fatalf("erro = %v, esperado ErrInvalidFormat", err)
	}
	if tm != (uuid.Time{}) {
		t.Fatalf("Time = %+v, esperado zerado", tm)
	}
}

// TestImportSubMillisecondRanges confere as faixas dos campos sub-ms por
// nível, em centenas de milhares de amostras (caso obrigatório 3 da
// especificação).
//
// Atenção: para o Nível 1 os campos Microseconds e Nanoseconds carregam
// bits aleatórios e NÃO respeitam a faixa 0..999 anunciada nos
// comentários do tipo Time — podem chegar a 4095 e 1023. Esse teste
// registra o comportamento real.
func TestImportSubMillisecondRanges(t *testing.T) {
	const samples = 200_000
	g := uuid.NewGenerator()

	t.Run("Level2", func(t *testing.T) {
		for i := 0; i < samples; i++ {
			tm := uuid.ImportBinary(g.Generate(uuid.Level2))
			if tm.Microseconds < 0 || tm.Microseconds > 999 {
				t.Fatalf("microssegundos fora de 0..999: %d", tm.Microseconds)
			}
		}
	})

	t.Run("Level3", func(t *testing.T) {
		for i := 0; i < samples; i++ {
			tm := uuid.ImportBinary(g.Generate(uuid.Level3))
			if tm.Microseconds < 0 || tm.Microseconds > 999 {
				t.Fatalf("microssegundos fora de 0..999: %d", tm.Microseconds)
			}
			if tm.Nanoseconds < 0 || tm.Nanoseconds > 999 {
				t.Fatalf("nanossegundos fora de 0..999: %d", tm.Nanoseconds)
			}
		}
	})

	t.Run("Level1PodeExcederAFaixa", func(t *testing.T) {
		maxMicro, maxNano := 0, 0
		for i := 0; i < samples; i++ {
			tm := uuid.ImportBinary(g.Generate(uuid.Level1))
			if tm.Microseconds > maxMicro {
				maxMicro = tm.Microseconds
			}
			if tm.Nanoseconds > maxNano {
				maxNano = tm.Nanoseconds
			}
		}
		t.Logf("nível 1 (bits aleatórios lidos como tempo): micro máx = %d, nano máx = %d",
			maxMicro, maxNano)
		if maxMicro <= 999 && maxNano <= 999 {
			t.Errorf("esperava valores acima de 999 no nível 1 (rand_a tem 12 bits e o topo de rand_b tem 10); "+
				"micro máx = %d, nano máx = %d", maxMicro, maxNano)
		}
	})
}

// TestImportRoundTripFromClock confere que o instante reconstruído a
// partir de segundos+ms+us+ns cai no intervalo medido em torno da
// geração, para os três níveis (com a tolerância adequada a cada um).
func TestImportRoundTripFromClock(t *testing.T) {
	g := uuid.NewGenerator()
	cases := []struct {
		level     uuid.Level
		tolerance time.Duration
	}{
		{uuid.Level1, time.Millisecond}, // só o ms é confiável
		{uuid.Level2, time.Microsecond}, // ms + us
		{uuid.Level3, time.Nanosecond},  // ms + us + ns
	}

	for _, c := range cases {
		for i := 0; i < 1_000; i++ {
			before := time.Now()
			u := g.Generate(c.level)
			after := time.Now()

			tm := uuid.ImportBinary(u)
			// Para o nível 1 os campos sub-ms são ruído: descarta-os.
			extra := int64(0)
			if c.level == uuid.Level2 {
				extra = int64(tm.Microseconds) * 1_000
			}
			if c.level == uuid.Level3 {
				extra = int64(tm.Microseconds)*1_000 + int64(tm.Nanoseconds)
			}
			reconstructed := time.Unix(tm.Seconds, int64(tm.Milliseconds)*1_000_000+extra)

			if reconstructed.Before(before.Add(-c.tolerance)) ||
				reconstructed.After(after.Add(c.tolerance)) {
				t.Fatalf("nível %d: instante %v fora de [%v, %v]",
					c.level, reconstructed, before, after)
			}
		}
	}
}
