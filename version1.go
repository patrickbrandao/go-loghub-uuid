package loghubuuid

import "encoding/binary"

// GenerateV1 produz um UUID de versão 1 (RFC 9562): tempo gregoriano de
// 60 bits, sequência de relógio de 14 bits e identificador de nó de 48
// bits.
//
// O tempo, a sequência e o nó vêm do estado compartilhado do pacote — veja
// SetNodeID e SetClockSequence. O relógio interno é estritamente
// crescente, portanto dois UUIDv1 gerados no mesmo processo nunca carregam
// o mesmo instante.
//
// Layout dos 16 bytes (big-endian):
//
//	bytes 0..3 : time_low ................ 32 bits baixos do tempo
//	bytes 4..5 : time_mid ................ 16 bits intermediários
//	byte  6    : 0x1_ | time_hi[11:8] .... versão(4) + topo do tempo
//	byte  7    : time_hi[7:0] ............ resto do topo (12 bits ao todo)
//	byte  8    : 10_ | clock_seq[13:8] ... variante(2) + topo da sequência
//	byte  9    : clock_seq[7:0] .......... resto da sequência
//	bytes 10..15: node ................... 48 bits de identificador de nó
//
// Atenção: o UUIDv1 não é ordenável lexicograficamente, porque os bits
// menos significativos do tempo vêm primeiro. Para ordenação cronológica
// use a versão 6 ou a versão 7.
//
// Custo da ordem estrita: quando a geração é mais rápida que o tique de
// 100 nanossegundos, o relógio interno avança sozinho e o instante
// embutido fica à frente do relógio do sistema. Em rajadas longas esse
// adiantamento se acumula, na proporção de um tique por UUID. A RFC 9562
// permite o comportamento, mas não conte com o carimbo de tempo de um
// UUIDv1 ou UUIDv6 como leitura fiel do relógio sob carga sustentada.
func GenerateV1() UUID {
	clockMu.Lock()
	now, seq := timeAndSequenceLocked()
	node := nodeLocked()
	clockMu.Unlock()

	var u UUID
	binary.BigEndian.PutUint32(u[0:4], uint32(now))
	binary.BigEndian.PutUint16(u[4:6], uint16(now>>32))
	binary.BigEndian.PutUint16(u[6:8], uint16((now>>48)&0x0FFF))
	u[6] = (u[6] & 0x0F) | 0x10 // nibble alto = versão 1
	u[8] = byte(seq>>8)&0x3F | 0x80
	u[9] = byte(seq)
	copy(u[10:], node[:])
	return u
}

// GenerateV6 produz um UUID de versão 6 (RFC 9562): os mesmos campos do
// UUIDv1, porém com o tempo reordenado do bit mais significativo para o
// menos significativo, de modo que a ordenação lexicográfica coincide com
// a ordem cronológica.
//
// Layout dos 16 bytes (big-endian):
//
//	bytes 0..3 : time_high ............... 32 bits altos do tempo
//	bytes 4..5 : time_mid ................ 16 bits intermediários
//	byte  6    : 0x6_ | time_low[11:8] ... versão(4) + topo dos 12 bits baixos
//	byte  7    : time_low[7:0] ........... resto dos 12 bits baixos
//	byte  8    : 10_ | clock_seq[13:8] ... variante(2) + topo da sequência
//	byte  9    : clock_seq[7:0] .......... resto da sequência
//	bytes 10..15: node ................... 48 bits de identificador de nó
//
// A resolução do campo é de 100 nanossegundos, mais fina que a do UUIDv7
// padrão. Ainda assim, prefira o UUIDv7 em bases novas: ele não expõe
// identificador de nó e é o recomendado pela RFC 9562.
//
// Custo da ordem estrita: quando a geração é mais rápida que o tique de
// 100 nanossegundos, o relógio interno avança sozinho e o instante
// embutido fica à frente do relógio do sistema. Em rajadas longas esse
// adiantamento se acumula, na proporção de um tique por UUID. A RFC 9562
// permite o comportamento, mas não conte com o carimbo de tempo de um
// UUIDv1 ou UUIDv6 como leitura fiel do relógio sob carga sustentada.
func GenerateV6() UUID {
	clockMu.Lock()
	now, seq := timeAndSequenceLocked()
	node := nodeLocked()
	clockMu.Unlock()

	var u UUID
	binary.BigEndian.PutUint32(u[0:4], uint32(now>>28))
	binary.BigEndian.PutUint16(u[4:6], uint16(now>>12))
	binary.BigEndian.PutUint16(u[6:8], uint16(now&0x0FFF))
	u[6] = (u[6] & 0x0F) | 0x60 // nibble alto = versão 6
	u[8] = byte(seq>>8)&0x3F | 0x80
	u[9] = byte(seq)
	copy(u[10:], node[:])
	return u
}
