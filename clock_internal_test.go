package loghubuuid

// Testes internos das funções puras de decomposição do instante. Este é o
// único arquivo de teste autorizado na raiz: as bordas do relógio (antes
// de 1970, ano 2300, viradas de milissegundo e de segundo) não podem ser
// exercitadas de fora do pacote, porque não há relógio injetável.

import (
	"testing"
	"time"
)

func TestSplitUnixInstantFloorsPreEpoch(t *testing.T) {
	// 1969-12-31T23:59:59.5Z: Unix() devolve -1 e Nanosecond() 500_000_000.
	ms, micro, nano := splitUnixInstant(-1, 500_000_000)
	if ms != 0 || micro != 0 || nano != 0 {
		t.Fatalf("pré-época: ms=%d micro=%d nano=%d, esperado 0/0/0", ms, micro, nano)
	}
}

func TestSplitUnixInstantYear2300FitsIn48Bits(t *testing.T) {
	at := time.Date(2300, 1, 1, 0, 0, 0, 0, time.UTC)
	ms, _, _ := splitUnixInstant(at.Unix(), 0)
	if ms != 10_413_792_000_000 {
		t.Fatalf("2300-01-01: ms=%d, esperado 10413792000000", ms)
	}
	if ms >= 1<<48 {
		t.Fatalf("2300-01-01: ms=%d não cabe em 48 bits", ms)
	}
}

func TestSplitUnixInstantRollovers(t *testing.T) {
	cases := []struct {
		sec, nsec int64
		ms        int64
		micro     uint16
		nano      uint16
	}{
		{0, 0, 0, 0, 0},
		{0, 999_999, 0, 999, 999},
		{0, 1_000_000, 1, 0, 0},
		{1, 999_999_999, 1_999, 999, 999},
		{2, 0, 2_000, 0, 0},
	}
	for _, c := range cases {
		ms, micro, nano := splitUnixInstant(c.sec, c.nsec)
		if ms != c.ms || micro != c.micro || nano != c.nano {
			t.Errorf("sec=%d nsec=%d: obtido %d/%d/%d, esperado %d/%d/%d",
				c.sec, c.nsec, ms, micro, nano, c.ms, c.micro, c.nano)
		}
	}
}

func TestGregorianFromUnixFloorsPreEpoch(t *testing.T) {
	if got := gregorianFromUnix(-1, 0); got != gregorian100ns {
		t.Fatalf("pré-época: %d, esperado %d", got, uint64(gregorian100ns))
	}
	if got := gregorianFromUnix(0, 100); got != gregorian100ns+1 {
		t.Fatalf("época + 100 ns: %d, esperado %d", got, uint64(gregorian100ns)+1)
	}
}
