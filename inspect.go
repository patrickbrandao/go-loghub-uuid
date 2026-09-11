package loghubuuid

import (
	"encoding/binary"
	"time"
)

// GregorianTime extrai o campo de tempo dos UUIDs de versão 1 e 6, em
// intervalos de 100 nanossegundos desde 15/10/1582.
//
// O segundo retorno é falso para as demais versões, inclusive a 2: o
// UUIDv2 sacrifica os 32 bits baixos do tempo para guardar o identificador
// local, e o que sobra não é um carimbo utilizável.
func (u UUID) GregorianTime() (GregorianTime, bool) {
	switch u.Version() {
	case 1:
		t := uint64(binary.BigEndian.Uint32(u[0:4])) |
			uint64(binary.BigEndian.Uint16(u[4:6]))<<32 |
			uint64(binary.BigEndian.Uint16(u[6:8])&0x0FFF)<<48
		return GregorianTime(t), true

	case 6:
		t := uint64(binary.BigEndian.Uint32(u[0:4]))<<28 |
			uint64(binary.BigEndian.Uint16(u[4:6]))<<12 |
			uint64(binary.BigEndian.Uint16(u[6:8])&0x0FFF)
		return GregorianTime(t), true
	}
	return 0, false
}

// Timestamp devolve o instante de criação embutido no UUID, em UTC.
//
// Funciona para as versões 1 e 6, com resolução de 100 nanossegundos, e
// para a versão 7, com resolução de milissegundo. Devolve falso para as
// versões que não carregam tempo — 2, 3, 4, 5 e 8.
//
// Para recuperar também os microssegundos e nanossegundos gravados pelos
// níveis 2 e 3 desta biblioteca, use TimestampWithLevel.
func (u UUID) Timestamp() (time.Time, bool) {
	if u.Version() == 7 {
		return time.UnixMilli(milliFromV7(u)).UTC(), true
	}
	g, ok := u.GregorianTime()
	if !ok {
		return time.Time{}, false
	}
	return g.Time(), true
}

// TimestampWithLevel devolve o instante de criação de um UUIDv7 em UTC,
// reconstruindo a precisão sub-milissegundo gravada pelo nível informado.
//
// O nível não pode ser deduzido do UUID, por isso precisa ser informado
// pelo chamador. Se algum campo lido estiver fora da faixa 0 a 999 — o
// que denuncia bits aleatórios, e não tempo — todos os campos
// sub-milissegundo são descartados e devolve-se apenas o milissegundo.
// No nível 3 os dois campos vêm da mesma geração: um rand_a fora da
// faixa prova que o topo de rand_b também é ruído, mesmo que caiba em
// 0 a 999 por acaso.
//
// Devolve falso se o UUID não for de versão 7.
func (u UUID) TimestampWithLevel(level Level) (time.Time, bool) {
	if u.Version() != 7 {
		return time.Time{}, false
	}
	t := time.UnixMilli(milliFromV7(u)).UTC()

	micro := int(uint16(u[6]&0x0F)<<8 | uint16(u[7]))
	nano := int(uint16(u[8]&0x3F)<<4 | uint16(u[9])>>4)

	switch level {
	case Level2:
		if micro <= 999 {
			t = t.Add(time.Duration(micro) * time.Microsecond)
		}
	case Level3:
		if micro <= 999 && nano <= 999 {
			t = t.Add(time.Duration(micro)*time.Microsecond + time.Duration(nano)*time.Nanosecond)
		}
	}
	return t, true
}

// milliFromV7 lê os 48 bits de milissegundos de um UUIDv7.
func milliFromV7(u UUID) int64 {
	return int64(u[0])<<40 |
		int64(u[1])<<32 |
		int64(u[2])<<24 |
		int64(u[3])<<16 |
		int64(u[4])<<8 |
		int64(u[5])
}

// ClockSequence extrai a sequência de relógio dos UUIDs de versão 1, 2 e
// 6. Nas versões 1 e 6 ela tem 14 bits. Na versão 2 o byte baixo da
// sequência é ocupado pelo domínio, portanto restam apenas os 6 bits
// altos (0 a 63). O segundo retorno é falso para as demais versões.
func (u UUID) ClockSequence() (int, bool) {
	switch u.Version() {
	case 1, 6:
		return int(uint16(u[8]&0x3F)<<8 | uint16(u[9])), true
	case 2:
		// O byte 9 é o domínio, não a sequência: devolve só os 6 bits
		// que a versão 2 preserva.
		return int(u[8] & 0x3F), true
	}
	return 0, false
}

// NodeID extrai uma cópia dos 6 bytes de identificador de nó dos UUIDs de
// versão 1, 2 e 6. Devolve nulo para as demais versões.
func (u UUID) NodeID() []byte {
	switch u.Version() {
	case 1, 2, 6:
		out := make([]byte, 6)
		copy(out, u[10:])
		return out
	}
	return nil
}

// Domain extrai o domínio de um UUID de versão 2. O segundo retorno é
// falso para as demais versões.
func (u UUID) Domain() (Domain, bool) {
	if u.Version() != 2 {
		return 0, false
	}
	return Domain(u[9]), true
}

// ID extrai o identificador local de um UUID de versão 2. O segundo
// retorno é falso para as demais versões.
func (u UUID) ID() (uint32, bool) {
	if u.Version() != 2 {
		return 0, false
	}
	return binary.BigEndian.Uint32(u[0:4]), true
}
