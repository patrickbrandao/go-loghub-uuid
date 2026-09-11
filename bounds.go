package loghubuuid

import "time"

// maxBoundMilli é o maior carimbo que cabe no campo de 48 bits de
// milissegundos do UUIDv7, equivalente a 10889-08-02T05:31:50.655Z.
const maxBoundMilli = int64(1)<<48 - 1

// maxBoundSec é o mesmo limite em segundos. Serve de guarda antes da
// multiplicação por mil feita na decomposição do instante: um time.Time
// comporta anos muito além disso, e o produto estouraria o inteiro com
// sinal, dando a volta para um valor negativo.
const maxBoundSec = maxBoundMilli / 1_000

// boundInstant decompõe o instante t nos campos gravados pelo UUIDv7,
// saturando nas duas pontas da faixa representável.
//
// Abaixo da época Unix o resultado é a própria época, exatamente como em
// splitUnixInstant e, portanto, como em Generate. Acima da faixa o
// resultado é o último instante representável,
// 10889-08-02T05:31:50.655999999Z, e os campos abaixo do milissegundo
// saturam junto com ele: zerá-los faria a fronteira regredir ao cruzar a
// borda.
//
// Saturar nas duas pontas mantém as fronteiras monotônicas para qualquer
// entrada, que é o que torna a consulta por intervalo correta. Truncar
// os bits excedentes, como o empacotamento por deslocamento do caminho
// quente faria, deixaria a fronteira dar a volta e a consulta passaria a
// devolver as linhas erradas em silêncio.
func boundInstant(t time.Time) (ms int64, micro, nano uint16) {
	sec := t.Unix()
	switch {
	case sec < 0:
		// Qualquer segundo negativo produz milissegundo negativo, então
		// o piso na época já está decidido aqui. Tratá-lo antes da
		// multiplicação também protege contra o estouro ao contrário.
		return 0, 0, 0
	case sec > maxBoundSec:
		return maxBoundMilli, 999, 999
	}

	ms, micro, nano = splitUnixInstant(sec, int64(t.Nanosecond()))
	if ms > maxBoundMilli {
		return maxBoundMilli, 999, 999
	}
	return ms, micro, nano
}

// boundAt monta a fronteira do instante t no nível informado,
// preenchendo com fill todos os bits que a geração real sortearia: fill
// zerado produz o menor UUID possível daquele instante, fill com todos
// os bits em um produz o maior.
//
// O corpo espelha campo a campo o de Generate, em uuid.go: os bits de
// tempo são calculados do mesmo jeito e só a origem dos bits livres
// muda. Se o layout mudar lá, muda aqui.
func boundAt(level Level, t time.Time, fill uint64) UUID {
	ms, micro, nano := boundInstant(t)

	var u UUID

	// --- 48 bits de milissegundos (bytes 0..5) ---
	u[0] = byte(ms >> 40)
	u[1] = byte(ms >> 32)
	u[2] = byte(ms >> 24)
	u[3] = byte(ms >> 16)
	u[4] = byte(ms >> 8)
	u[5] = byte(ms)

	// --- rand_a (12 bits): versão + microssegundos OU bits livres ---
	var randA uint16
	switch level {
	case Level2, Level3:
		randA = micro & 0x0FFF
	default: // Level1 e desconhecidos
		randA = uint16(fill) & 0x0FFF
	}
	u[6] = 0x70 | byte((randA>>8)&0x0F) // nibble alto = versão 7
	u[7] = byte(randA)

	// --- rand_b (62 bits): variante + nanossegundos (nível 3) + livres ---
	var randB uint64
	switch level {
	case Level3:
		randB = (uint64(nano&0x03FF) << 52) | (fill & ((1 << 52) - 1))
	default: // Level1 e Level2
		randB = fill & ((1 << 62) - 1)
	}
	u[8] = 0x80 | byte((randB>>56)&0x3F) // bits 7..6 = variante "10"
	u[9] = byte(randB >> 48)
	u[10] = byte(randB >> 40)
	u[11] = byte(randB >> 32)
	u[12] = byte(randB >> 24)
	u[13] = byte(randB >> 16)
	u[14] = byte(randB >> 8)
	u[15] = byte(randB)

	return u
}

// MinAt devolve o menor UUIDv7 que a biblioteca poderia gerar no
// instante t, no nível informado — todos os bits livres de entropia em
// zero, com versão 7 e variante RFC preservadas.
//
// Serve de limite inferior em consulta por intervalo usando o índice da
// própria chave primária, sem coluna nem índice de carimbo temporal:
//
//	SELECT * FROM eventos WHERE id >= ? AND id < ?
//
// ATENÇÃO — a fronteira só vale para UUIDs gravados no MESMO nível. Os
// bits abaixo do milissegundo significam coisas diferentes em cada
// nível, então uma fronteira de Nível 3 não delimita corretamente
// identificadores gravados em Nível 1. Nunca misture níveis na mesma
// coluna.
//
// A precisão da fronteira é a do nível: no Nível 1 ela delimita o
// milissegundo inteiro, no Nível 2 o microssegundo e no Nível 3 o
// nanossegundo. Níveis desconhecidos são tratados como Level1, como em
// Generate.
//
// Instantes anteriores a 1970-01-01T00:00:00Z devolvem a fronteira da
// própria época, e instantes posteriores a 10889-08-02T05:31:50.655999999Z
// devolvem a do último instante representável, porque o campo de
// milissegundos tem 48 bits. Nas duas pontas a saturação mantém a
// fronteira monotônica, mas dois instantes distintos fora da faixa
// passam a devolver o mesmo valor.
func MinAt(level Level, t time.Time) UUID {
	return boundAt(level, t, 0)
}

// MaxAt devolve o maior UUIDv7 que a biblioteca poderia gerar no
// instante t, no nível informado — todos os bits livres de entropia em
// um, com versão 7 e variante RFC preservadas. É o limite superior
// fechado do instante.
//
// Valem aqui as mesmas observações de MinAt sobre mistura de níveis,
// resolução da fronteira e saturação nas pontas da faixa representável.
func MaxAt(level Level, t time.Time) UUID {
	return boundAt(level, t, ^uint64(0))
}

// RangeAt devolve as duas fronteiras de um intervalo SEMIABERTO
// [from, to) no nível informado, prontas para a comparação usual:
//
//	lo, hi := loghubuuid.RangeAt(loghubuuid.Level2, inicio, fim)
//	rows, err := db.Query(
//	    "SELECT * FROM eventos WHERE id >= $1 AND id < $2",
//	    lo.String(), hi.String(),
//	)
//
// O limite inferior é MinAt(level, from) e o superior é MinAt(level,
// to): identificadores gerados no instante to ficam de fora, os
// gerados em from ficam dentro. Para um intervalo fechado, use MinAt e
// MaxAt diretamente.
//
// A exclusão vale na resolução do nível. No Nível 1 o intervalo termina
// no início do milissegundo de to, e portanto exclui esse milissegundo
// inteiro; no Nível 2 termina no microssegundo de to e no Nível 3 no
// nanossegundo.
//
// RangeAt não ordena os argumentos: passar to anterior a from devolve um
// intervalo vazio, que é o que a comparação vai refletir.
//
// Valem aqui as mesmas observações de MinAt sobre mistura de níveis e
// saturação nas pontas da faixa representável.
func RangeAt(level Level, from, to time.Time) (lo, hi UUID) {
	return boundAt(level, from, 0), boundAt(level, to, 0)
}
