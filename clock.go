package loghubuuid

import (
	crand "crypto/rand"
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
)

// GregorianTime conta intervalos de 100 nanossegundos desde 15/10/1582.
// É o formato do campo de tempo dos UUIDs de versão 1, 2 e 6.
type GregorianTime int64

// UnixTime converte o instante gregoriano para a época Unix, devolvendo
// segundos e a fração em nanossegundos.
func (t GregorianTime) UnixTime() (sec, nsec int64) {
	sec = int64(t) - gregorian100ns
	nsec = (sec % 10_000_000) * 100
	sec /= 10_000_000
	return sec, nsec
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
func gregorianFromUnix(sec, nsec int64) uint64 {
	if sec < 0 {
		// Relógio ajustado para antes de 1970: piso na própria época.
		sec = 0
	}
	return uint64(sec)*10_000_000 + uint64(nsec/100) + gregorian100ns
}

// setClockSequenceLocked grava a sequência de relógio. O valor -1 pede um
// sorteio. Exige clockMu travada.
func setClockSequenceLocked(seq int) {
	if seq == -1 {
		var b [2]byte
		fillRandom(b[:])
		seq = int(b[0])<<8 | int(b[1])
	}
	old := clockSeq
	// O bit 15 marca "inicializada", já que só 14 bits vão para o UUID.
	clockSeq = uint16(seq&0x3FFF) | 0x8000
	if old != clockSeq {
		// Sequência nova: o relógio anterior deixa de valer como piso.
		lastClockTime = 0
	}
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
// usados. O valor -1 pede um sorteio novo.
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
// fonte criptográfica falhar: a sequência de relógio e o nó precisam ser
// imprevisíveis, e o gerador padrão (PCG) não serve de substituto. A
// partir do Go 1.24 a leitura nunca falha.
func fillRandom(b []byte) {
	if _, err := crand.Read(b); err != nil {
		panic("loghubuuid: falha ao ler crypto/rand para a sequência de relógio ou o nó: " + err.Error())
	}
}
