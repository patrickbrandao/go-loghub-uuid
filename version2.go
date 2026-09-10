package loghubuuid

import (
	"encoding/binary"
	"os"
	"strconv"
)

// Domain é o domínio de identificação de um UUID de versão 2 (DCE
// Security), gravado no byte 9.
type Domain byte

const (
	// Person identifica um usuário do sistema operacional.
	Person Domain = 0

	// Group identifica um grupo do sistema operacional.
	Group Domain = 1

	// Org identifica uma organização, com significado local.
	Org Domain = 2
)

// String descreve o domínio em texto.
func (d Domain) String() string {
	switch d {
	case Person:
		return "pessoa"
	case Group:
		return "grupo"
	case Org:
		return "organizacao"
	}
	return "dominio " + strconv.Itoa(int(d))
}

// GenerateV2 produz um UUID de versão 2 (DCE Security). Ele parte de um
// UUIDv1 e substitui dois campos: os 32 bits baixos do tempo passam a
// carregar o identificador local, e o byte baixo da sequência de relógio
// passa a carregar o domínio.
//
// Consequência dessa troca: o UUIDv2 perde os 32 bits menos
// significativos do tempo, portanto o instante embutido tem resolução de
// poucos minutos e não deve ser lido como carimbo de tempo. Timestamp
// devolve falso para esta versão.
//
// Atenção: o UUIDv2 identifica um principal — usuário, grupo ou
// organização — e não um evento. Os bits de tempo que sobram só mudam a
// cada 2^32 tiques de 100 nanossegundos, cerca de sete minutos, e o
// avanço de um tique por chamada do relógio interno cai justamente nos
// bits descartados. Portanto, chamadas repetidas com o mesmo domínio e o
// mesmo identificador, dentro da mesma janela, devolvem o MESMO UUID;
// duas chamadas consecutivas são iguais. Sem trocar a sequência de
// relógio, existem no máximo 64 valores distintos por janela.
//
// A versão 2 é legado do DCE 1.1 e não é recomendada para sistemas novos.
func GenerateV2(domain Domain, id uint32) UUID {
	u := GenerateV1()
	u[6] = (u[6] & 0x0F) | 0x20 // nibble alto = versão 2
	u[9] = byte(domain)
	binary.BigEndian.PutUint32(u[0:4], id)
	return u
}

// GenerateV2Person produz um UUID de versão 2 no domínio Person usando o
// identificador de usuário do processo.
//
// Em Windows a chamada de sistema devolve -1, e o identificador gravado
// fica sendo 0xFFFFFFFF.
func GenerateV2Person() UUID {
	return GenerateV2(Person, uint32(os.Getuid()))
}

// GenerateV2Group produz um UUID de versão 2 no domínio Group usando o
// identificador de grupo do processo.
//
// Em Windows a chamada de sistema devolve -1, e o identificador gravado
// fica sendo 0xFFFFFFFF.
func GenerateV2Group() UUID {
	return GenerateV2(Group, uint32(os.Getgid()))
}
