package loghubuuid

// As versões 3 e 5 são definidas pela RFC 9562 sobre MD5 e SHA-1. Os dois
// algoritmos são fracos para uso criptográfico, mas obrigatórios aqui: são
// o que a especificação manda, e o objetivo é derivar um identificador
// estável, não proteger um segredo.
import (
	"crypto/md5"  //nolint:gosec // exigido pela RFC 9562 para a versão 3
	"crypto/sha1" //nolint:gosec // exigido pela RFC 9562 para a versão 5
	"hash"
)

// Espaços de nomes conhecidos, definidos na seção 6.6 da RFC 9562.
var (
	// NameSpaceDNS é o espaço de nomes de domínio.
	NameSpaceDNS = MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")

	// NameSpaceURL é o espaço de nomes de URL.
	NameSpaceURL = MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8")

	// NameSpaceOID é o espaço de nomes de OID da ISO.
	NameSpaceOID = MustParse("6ba7b812-9dad-11d1-80b4-00c04fd430c8")

	// NameSpaceX500 é o espaço de nomes de nome distinto X.500.
	NameSpaceX500 = MustParse("6ba7b814-9dad-11d1-80b4-00c04fd430c8")
)

// GenerateV3 produz um UUID de versão 3: o resumo MD5 do espaço de nomes
// concatenado com o nome, com versão e variante sobrescritas.
//
// A geração é determinística e não usa entropia: o mesmo par espaço e nome
// sempre devolve o mesmo UUID, em qualquer máquina e a qualquer momento.
// Prefira a versão 5, que usa SHA-1.
func GenerateV3(space UUID, name []byte) UUID {
	return GenerateHash(md5.New(), space, name, 3) //nolint:gosec // exigido pela RFC 9562 para a versão 3
}

// GenerateV5 produz um UUID de versão 5: os 16 primeiros bytes do resumo
// SHA-1 do espaço de nomes concatenado com o nome, com versão e variante
// sobrescritas.
//
// A geração é determinística e não usa entropia: o mesmo par espaço e nome
// sempre devolve o mesmo UUID, em qualquer máquina e a qualquer momento.
func GenerateV5(space UUID, name []byte) UUID {
	return GenerateHash(sha1.New(), space, name, 5) //nolint:gosec // exigido pela RFC 9562 para a versão 5
}

// GenerateHash produz um UUID baseado em nome com a função de resumo
// informada, gravando o número de versão pedido.
//
// Serve para reproduzir esquemas próprios de derivação. Para as versões
// da RFC use GenerateV3 ou GenerateV5. Apenas os 4 bits baixos de version
// são aproveitados, e a variante RFC é sempre aplicada.
func GenerateHash(h hash.Hash, space UUID, name []byte, version byte) UUID {
	h.Reset()
	// hash.Hash nunca devolve erro em Write; o errcheck já sabe disso.
	h.Write(space[:])
	h.Write(name)
	sum := h.Sum(nil)

	var u UUID
	copy(u[:], sum)
	u[6] = (u[6] & 0x0F) | (version&0x0F)<<4
	u[8] = (u[8] & 0x3F) | 0x80
	return u
}
