package loghubuuid

import "time"

// boundAt monta a fronteira do instante t no nível informado,
// preenchendo com fill todos os bits que a geração real sortearia: fill
// zerado produz o menor UUID possível daquele instante, fill com todos
// os bits em um produz o maior.
//
// A decomposição do instante e o empacotamento dos bytes são os mesmos
// usados por GenerateAt, em construct.go: a fronteira é o caso em que
// os bits livres, em vez de sorteados, são constantes. Compartilhar o
// empacotamento é o que garante que a fronteira e a geração por
// instante não possam divergir.
func boundAt(level Level, t time.Time, fill uint64) UUID {
	ms, micro, nano := saturatedInstant(t)
	return packV7(level, ms, micro, nano, uint16(fill), fill)
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
