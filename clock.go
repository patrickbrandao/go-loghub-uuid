package loghubuuid

import (
	crand "crypto/rand"
	"math"
	"sync"
	"time"
)

// Constantes de conversão entre a época gregoriana — usada pelo campo de
// tempo dos UUIDs de versão 1, 2 e 6 — e a época Unix.
const (
	// lillianDay é o dia juliano de 15/10/1582, início do calendário gregoriano.
	lillianDay = 2299160

	// unixDay é o dia juliano de 01/01/1970, início da época Unix.
	unixDay = 2440587

	// gregorianDays é a distância entre as duas épocas, em dias.
	gregorianDays = unixDay - lillianDay

	// gregorianSeconds é a mesma distância, em segundos.
	gregorianSeconds = gregorianDays * 86400

	// gregorian100ns é a mesma distância, em intervalos de 100 nanossegundos.
	gregorian100ns = gregorianSeconds * 10_000_000

	// minGregorianTicks é o menor valor de GregorianTime para o qual a
	// subtração de gregorian100ns não estoura int64. Valores abaixo deste
	// limiar são saturados no piso representável por UnixTime.
	minGregorianTicks = math.MinInt64 + gregorian100ns
)

// GregorianTime conta intervalos de 100 nanossegundos desde 15/10/1582.
// É o formato do campo de tempo dos UUIDs de versão 1, 2 e 6.
type GregorianTime int64

// UnixTime converte o instante gregoriano para a época Unix, devolvendo
// segundos e a fração em nanossegundos.
//
// O par devolvido é canônico: nsec fica sempre em 0..999_999_999, também
// para instantes anteriores a 1970, que existem no campo (ele começa em
// 1582). A divisão é euclidiana, com resto não negativo, e não a truncada
// da linguagem: com a truncada um tique antes da época sairia como
// (0, -100) em vez de (-1, 999_999_900). time.Unix normalizaria os dois
// para o mesmo instante, mas quem consome sec e nsec diretamente não
// deve receber resto negativo. A resolução é a do campo, 100 ns: os dois
// dígitos finais de nsec são sempre zero. Ver
// docs/05-outras-versoes-uuid.md seção 4.1.
//
// Valores fora do domínio do campo (0 .. 2^60-1) são aceitos, mas
// saturados: se int64(t) < minGregorianTicks, a subtração estouraria
// int64 e o resultado daria a volta para o futuro distante; nesse caso
// o cálculo parte de math.MinInt64, devolvendo o menor instante
// representável em vez de um valor absurdo. O custo é uma comparação
// que o preditor de desvios elimina no caso comum.
func (t GregorianTime) UnixTime() (sec, nsec int64) {
	ticks := int64(t)
	if ticks < minGregorianTicks {
		ticks = math.MinInt64
	} else {
		ticks -= gregorian100ns
	}
	sec = ticks / 10_000_000
	rem := ticks % 10_000_000
	if rem < 0 {
		rem += 10_000_000
		sec--
	}
	return sec, rem * 100
}

// Time devolve o instante como time.Time em UTC.
func (t GregorianTime) Time() time.Time {
	sec, nsec := t.UnixTime()
	return time.Unix(sec, nsec).UTC()
}

// Estado compartilhado pelos geradores de versão 1, 2 e 6.
//
// Este estado tem trava própria e é totalmente independente do caminho de
// geração do UUIDv7: nada aqui é tocado por Generate, GenerateString ou
// pelas funções de conversão, que continuam sem lock algum.
var (
	clockMu       sync.Mutex
	lastClockTime uint64
	clockSeq      uint16 // valor zero significa "ainda não inicializada"
	nodeIdentity  [6]byte
	nodeAssigned  bool

	// seqLastTime guarda, para cada sequência de relógio já usada neste
	// processo (com o bit 15 ligado, como em clockSeq), o último instante
	// emitido com ela. É o que garante a invariante de unicidade: os
	// instantes emitidos com uma mesma sequência são estritamente
	// crescentes durante toda a vida do processo, mesmo que o chamador
	// alterne entre sequências. Só é tocado nas trocas de sequência, nunca
	// por geração; cresce no máximo até 16384 entradas.
	seqLastTime map[uint16]uint64
)

// timeAndSequenceLocked devolve o instante gregoriano atual e a sequência
// de relógio, garantindo valor estritamente crescente entre chamadas.
// Exige clockMu travada.
func timeAndSequenceLocked() (uint64, uint16) {
	if clockSeq == 0 {
		setClockSequenceLocked(-1)
	}

	// Unix()/Nanosecond() em vez de UnixNano(): a leitura em duas partes
	// não satura em 2262, ao contrário do inteiro de nanossegundos.
	n := time.Now()
	now := gregorianFromUnix(n.Unix(), int64(n.Nanosecond()))

	// Relógio parado ou para trás: avança um tique para preservar a ordem.
	if now <= lastClockTime {
		now = lastClockTime + 1
	}
	lastClockTime = now
	return now, clockSeq
}

// gregorianFromUnix converte segundos desde a época Unix e a fração do
// segundo em nanossegundos para intervalos de 100 ns desde 15/10/1582.
// Relógios anteriores a 1970 recebem piso na própria época Unix.
//
// O piso zera as duas componentes, como em splitUnixInstant: a fração de
// segundo que o Go devolve para um instante pré-época é positiva, então
// zerar só os segundos projetaria o carimbo em até um segundo à frente
// da época e faria o relógio regredir ao cruzar a fronteira. Ver
// docs/05-outras-versoes-uuid.md seção 4.1.
func gregorianFromUnix(sec, nsec int64) uint64 {
	if sec < 0 {
		// Relógio ajustado para antes de 1970: piso na própria época.
		return gregorian100ns
	}
	return uint64(sec)*10_000_000 + uint64(nsec/100) + gregorian100ns
}

// setClockSequenceLocked grava a sequência de relógio. O valor -1 pede o
// sorteio de uma sequência ainda não usada neste processo. Exige clockMu
// travada.
//
// Ao trocar de sequência, o último instante emitido com a sequência
// anterior é guardado, e o piso do relógio passa a ser o último instante
// já emitido com a sequência nova (zero se ela nunca foi usada). Assim o
// adiantamento acumulado só é descartado ao entrar em uma sequência
// inédita, e voltar a uma sequência antiga nunca repete um instante dela.
func setClockSequenceLocked(seq int) {
	if seq == -1 {
		seq = unusedSequenceLocked()
	}
	// O bit 15 marca "inicializada", já que só 14 bits vão para o UUID.
	next := uint16(seq&0x3FFF) | 0x8000
	if next == clockSeq {
		return
	}
	if seqLastTime == nil {
		seqLastTime = make(map[uint16]uint64)
	}
	if clockSeq != 0 {
		seqLastTime[clockSeq] = lastClockTime
	}
	clockSeq = next
	lastClockTime = seqLastTime[next]
}

// sequenceUsedLocked informa se a sequência (com o bit 15 ligado) está em
// uso ou já foi usada neste processo. Exige clockMu travada.
func sequenceUsedLocked(seq uint16) bool {
	if seq == clockSeq {
		return true
	}
	_, used := seqLastTime[seq]
	return used
}

// unusedSequenceLocked sorteia uma sequência de 14 bits que ainda não foi
// usada neste processo: parte de um valor aleatório e avança até a
// primeira livre. Se todas as 16384 já tiverem sido usadas, devolve o
// sorteio original; o piso por sequência continua impedindo repetição.
// Exige clockMu travada.
func unusedSequenceLocked() int {
	var b [2]byte
	fillRandom(b[:])
	start := (int(b[0])<<8 | int(b[1])) & 0x3FFF
	for i := 0; i < 1<<14; i++ {
		candidate := (start + i) & 0x3FFF
		if !sequenceUsedLocked(uint16(candidate) | 0x8000) {
			return candidate
		}
	}
	return start
}

// nodeLocked devolve o identificador de nó de 48 bits, sorteando um na
// primeira chamada. Exige clockMu travada.
func nodeLocked() [6]byte {
	if !nodeAssigned {
		var b [6]byte
		fillRandom(b[:])
		// Bit multicast ligado: sinaliza nó aleatório, não um endereço MAC
		// real, conforme recomendado pela RFC 9562 seção 6.10.
		b[0] |= 0x01
		nodeIdentity = b
		nodeAssigned = true
	}
	return nodeIdentity
}

// GetTime devolve o instante gregoriano corrente e a sequência de relógio
// em uso, avançando o relógio interno como faria uma geração de UUIDv1.
//
// O segundo retorno traz os 14 bits da sequência com os dois bits de
// variante da RFC já posicionados (bit 15 ligado, bit 14 desligado),
// pronto para ser gravado nos bytes 8 e 9 de um UUIDv1 ou UUIDv6 — o
// mesmo formato devolvido pelo pacote github.com/google/uuid. Para obter
// apenas a sequência, mascare com 0x3FFF ou use ClockSequence.
func GetTime() (GregorianTime, uint16) {
	clockMu.Lock()
	defer clockMu.Unlock()
	now, seq := timeAndSequenceLocked()
	return GregorianTime(now), seq
}

// ClockSequence devolve a sequência de relógio de 14 bits em uso pelos
// geradores de versão 1, 2 e 6, inicializando-a se ainda não houver uma.
func ClockSequence() int {
	clockMu.Lock()
	defer clockMu.Unlock()
	if clockSeq == 0 {
		setClockSequenceLocked(-1)
	}
	return int(clockSeq & 0x3FFF)
}

// SetClockSequence fixa a sequência de relógio. Só os 14 bits baixos são
// usados. O valor -1 sorteia uma sequência ainda não usada neste processo.
//
// A biblioteca mantém um piso de relógio por sequência: ao entrar em uma
// sequência, o próximo instante emitido é estritamente maior que qualquer
// instante já emitido com ela. Consequências:
//
//   - Um valor explícito pode ser reutilizado à vontade; voltar a uma
//     sequência antiga herda o piso dela e nunca repete um UUIDv1/v6.
//   - O adiantamento acumulado do relógio interno só é descartado ao
//     entrar em uma sequência inédita. Para ressincronizar com o relógio
//     do sistema (por exemplo, após detectar que ele foi atrasado), use
//     -1, que garante uma sequência inédita enquanto houver alguma livre.
//
// Isto difere do pacote github.com/google/uuid, que descarta o piso em
// qualquer troca de sequência e por isso pode repetir um UUIDv1 ao voltar
// a uma sequência já usada.
func SetClockSequence(seq int) {
	clockMu.Lock()
	defer clockMu.Unlock()
	setClockSequenceLocked(seq)
}

// NodeID devolve uma cópia dos 6 bytes de identificador de nó usados pelos
// UUIDs de versão 1, 2 e 6.
//
// Por padrão o nó é sorteado uma única vez e marcado como aleatório. Esta
// biblioteca não lê interfaces de rede, para não arrastar o pacote net
// para dentro de quem só gera UUIDv7. Para usar um endereço MAC real,
// leia-o com net.Interfaces e entregue-o a SetNodeID.
func NodeID() []byte {
	clockMu.Lock()
	defer clockMu.Unlock()
	n := nodeLocked()
	out := make([]byte, 6)
	copy(out, n[:])
	return out
}

// SetNodeID fixa o identificador de nó a partir dos 6 primeiros bytes de
// id. Devolve falso, sem alterar nada, se id tiver menos de 6 bytes.
//
// A chamada não mexe no piso do relógio interno: os instantes já emitidos
// continuam reservados, portanto reaplicar o mesmo nó nunca repete um
// UUIDv1 ou UUIDv6. Para descartar o adiantamento acumulado do relógio,
// sorteie uma sequência nova com SetClockSequence(-1).
//
// Os bytes são copiados como estão. Se o valor não for um endereço MAC
// real, a RFC 9562 seção 6.10 pede o bit 0 do primeiro byte (multicast)
// ligado, para sinalizar que o nó não identifica uma placa de rede; essa
// marcação fica a cargo do chamador.
func SetNodeID(id []byte) bool {
	if len(id) < 6 {
		return false
	}
	clockMu.Lock()
	defer clockMu.Unlock()
	copy(nodeIdentity[:], id[:6])
	nodeAssigned = true
	return true
}

// fillRandom preenche b com bytes de crypto/rand. Entra em pânico se a
// fonte criptográfica falhar: a sequência de relógio e o nó são sorteados
// uma única vez e precisam ser imprevisíveis, então não podem degradar em
// silêncio para o gerador padrão. A partir do Go 1.24 a leitura nunca
// falha.
func fillRandom(b []byte) {
	if _, err := crand.Read(b); err != nil {
		panic("loghubuuid: falha ao ler crypto/rand para a sequência de relógio ou o nó: " + err.Error())
	}
}
