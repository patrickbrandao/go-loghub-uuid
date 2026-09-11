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

// TestSequenceFloorSurvivesRoundTrip confere, sem depender da velocidade
// do host, a invariante que garante a unicidade dos UUIDv1/v6: o piso do
// relógio é guardado por sequência, e voltar a uma sequência antiga
// restaura o último instante emitido com ela.
//
// O adiantamento é simulado gravando lastClockTime um segundo à frente do
// relógio real, o que uma rajada só produziria em hosts muito rápidos. É
// por isso que o teste vive na raiz: o estado do relógio não é injetável
// de fora do pacote.
//
// REGRESSÃO: antes da correção, qualquer troca de sequência zerava o
// piso, e a volta para a sequência antiga reemitia instantes já usados.
func TestSequenceFloorSurvivesRoundTrip(t *testing.T) {
	clockMu.Lock()
	defer clockMu.Unlock()
	defer setClockSequenceLocked(-1)

	const seqA, seqB = 0x0AAA, 0x0BBB
	n := time.Now()
	ahead := gregorianFromUnix(n.Unix(), int64(n.Nanosecond())) + 10_000_000 // +1 s

	setClockSequenceLocked(seqA)
	lastClockTime = ahead
	lastA, seq := timeAndSequenceLocked()
	if seq&0x3FFF != seqA || lastA != ahead+1 {
		t.Fatalf("sequência A: instante %d com sequência %#x, esperado %d com %#x", lastA, seq&0x3FFF, ahead+1, seqA)
	}

	// B é inédita: o piso é descartado e o instante volta ao relógio real.
	setClockSequenceLocked(seqB)
	if lastClockTime != 0 {
		t.Fatalf("sequência inédita deveria zerar o piso, obtido %d", lastClockTime)
	}
	fromB, _ := timeAndSequenceLocked()
	if fromB >= lastA {
		t.Fatalf("sequência B deveria acompanhar o relógio real (%d), mas ficou em %d", fromB, lastA)
	}

	// De volta a A: o piso de A precisa ser exatamente o último instante
	// emitido com A, e o próximo instante tem de superá-lo.
	setClockSequenceLocked(seqA)
	if lastClockTime != lastA {
		t.Fatalf("piso da sequência A: %d, esperado %d", lastClockTime, lastA)
	}
	if again, _ := timeAndSequenceLocked(); again != lastA+1 {
		t.Fatalf("instante após voltar para A: %d, esperado %d", again, lastA+1)
	}

	// Reaplicar a sequência em uso não mexe no piso.
	before := lastClockTime
	setClockSequenceLocked(seqA)
	if lastClockTime != before {
		t.Fatalf("reaplicar a sequência em uso alterou o piso: %d, esperado %d", lastClockTime, before)
	}
}

// TestUnusedSequenceSkipsUsedOnes confere que o sorteio de sequência
// inédita nunca devolve uma sequência em uso ou já usada no processo.
func TestUnusedSequenceSkipsUsedOnes(t *testing.T) {
	clockMu.Lock()
	defer clockMu.Unlock()
	defer setClockSequenceLocked(-1)

	// Marca como usadas todas as sequências menos duas, para que o sorteio
	// quase sempre precise avançar até uma livre.
	const freeA, freeB = 0x1234, 0x2345
	seqLastTime = make(map[uint16]uint64, 1<<14)
	for s := 0; s < 1<<14; s++ {
		if s == freeA || s == freeB {
			continue
		}
		seqLastTime[uint16(s)|0x8000] = 1
	}
	setClockSequenceLocked(freeA) // freeA passa a estar em uso

	for i := 0; i < 20; i++ {
		got := unusedSequenceLocked()
		if got != freeB {
			t.Fatalf("sorteio %d devolveu %#x, esperado a única sequência livre %#x", i, got, freeB)
		}
	}

	// Com todas usadas, o sorteio ainda devolve um valor válido de 14 bits.
	seqLastTime[uint16(freeB)|0x8000] = 1
	if got := unusedSequenceLocked(); got < 0 || got > 0x3FFF {
		t.Fatalf("sorteio com todas as sequências usadas devolveu %#x, fora de 14 bits", got)
	}
	seqLastTime = nil
}
