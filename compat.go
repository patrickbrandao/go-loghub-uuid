package loghubuuid

// Este arquivo reúne apelidos com os nomes e as assinaturas usados pelo
// pacote github.com/google/uuid. Servem para migrar código existente com
// o mínimo de edição: troque o caminho do import e, na maioria dos casos,
// as chamadas continuam compilando.
//
// Os apelidos que produzem bits aleatórios (New, NewString, NewRandom e
// NewV7) leem de crypto/rand, como faz o pacote de origem, e por isso são
// mais lentos que Generate, GenerateV7 e GenerateV4 do gerador padrão.
// Os erros devolvidos são nulos sempre que a fonte criptográfica
// responde; uma falha de leitura, impossível a partir do Go 1.24, vira
// ErrEntropySource em NewRandom e NewV7 e pânico em New e NewString,
// espelhando o pacote de origem.
//
// O que deliberadamente NÃO foi trazido:
//
//	SetRand              — trocaria a entropia do gerador padrão em tempo
//	                       de execução, exigindo uma leitura atômica no
//	                       caminho quente de Generate. Use
//	                       NewGeneratorWithReader ou NewCryptoGenerator.
//	EnableRandPool       — o gerador padrão já lê do gerador do runtime,
//	DisableRandPool        que tem uma instância por thread e nenhuma trava
//	                       global; não há o que ligar ou desligar.
//	SetNodeInterface     — leria interfaces de rede, arrastando o pacote
//	NodeInterface          net para dentro de quem só gera UUIDv7. Leia o
//	                       endereço com net.Interfaces e passe a SetNodeID.

import (
	"hash"
	"io"
)

// compatGenerator alimenta os apelidos de compatibilidade com crypto/rand,
// preservando a garantia de imprevisibilidade do pacote
// github.com/google/uuid. Não é usado por Generate nem por GenerateString.
var compatGenerator = NewCryptoGenerator()

// New produz um UUID de versão 4 com entropia criptográfica. Entra em
// pânico se a leitura de crypto/rand falhar, como uuid.New de origem.
func New() UUID { return compatGenerator.GenerateV4() }

// NewString produz um UUID de versão 4 com entropia criptográfica, já em
// formato canônico de texto.
func NewString() string { return New().String() }

// NewRandom produz um UUID de versão 4 com entropia criptográfica. Devolve
// ErrEntropySource se a leitura falhar.
func NewRandom() (u UUID, err error) {
	defer func() {
		if p := recover(); p != nil {
			u, err = Nil, ErrEntropySource
		}
	}()
	return compatGenerator.GenerateV4(), nil
}

// NewRandomFromReader produz um UUID de versão 4 com 16 bytes lidos do
// leitor informado.
//
// Monta um Generator descartável a cada chamada. Para uso repetido,
// prefira guardar o resultado de NewGeneratorWithReader.
func NewRandomFromReader(r io.Reader) (UUID, error) {
	u, err := fromReader(r, func(g *Generator) UUID { return g.GenerateV4() })
	return u, err
}

// NewUUID produz um UUID de versão 1. O erro é sempre nulo.
func NewUUID() (UUID, error) { return GenerateV1(), nil }

// NewV6 produz um UUID de versão 6. O erro é sempre nulo.
func NewV6() (UUID, error) { return GenerateV6(), nil }

// NewV7 produz um UUID de versão 7 padrão (Level1) com entropia
// criptográfica, como o pacote de origem. Devolve ErrEntropySource se a
// leitura falhar. Para velocidade máxima use GenerateV7 ou
// Generate(Level1), que leem do gerador do runtime.
//
// Para gravar precisão sub-milissegundo, use GenerateV7Level2 ou
// GenerateV7Level3, ou Generate com Level2 ou Level3.
func NewV7() (u UUID, err error) {
	defer func() {
		if p := recover(); p != nil {
			u, err = Nil, ErrEntropySource
		}
	}()
	return compatGenerator.Generate(Level1), nil
}

// NewV7FromReader produz um UUID de versão 7 com 16 bytes lidos do leitor
// informado.
//
// Monta um Generator descartável a cada chamada. Para uso repetido,
// prefira guardar o resultado de NewGeneratorWithReader.
func NewV7FromReader(r io.Reader) (UUID, error) {
	return fromReader(r, func(g *Generator) UUID { return g.Generate(Level1) })
}

// NewMD5 produz um UUID de versão 3.
func NewMD5(space UUID, data []byte) UUID { return GenerateV3(space, data) }

// NewSHA1 produz um UUID de versão 5.
func NewSHA1(space UUID, data []byte) UUID { return GenerateV5(space, data) }

// NewHash produz um UUID baseado em nome com a função de resumo informada.
func NewHash(h hash.Hash, space UUID, data []byte, version int) UUID {
	return GenerateHash(h, space, data, byte(version))
}

// NewDCESecurity produz um UUID de versão 2. O erro é sempre nulo.
func NewDCESecurity(domain Domain, id uint32) (UUID, error) {
	return GenerateV2(domain, id), nil
}

// NewDCEPerson produz um UUID de versão 2 no domínio Person. O erro é
// sempre nulo.
func NewDCEPerson() (UUID, error) { return GenerateV2Person(), nil }

// NewDCEGroup produz um UUID de versão 2 no domínio Group. O erro é
// sempre nulo.
func NewDCEGroup() (UUID, error) { return GenerateV2Group(), nil }

// fromReader executa build sobre um gerador temporário alimentado pelo
// leitor, convertendo o pânico de leitura em erro devolvido.
func fromReader(r io.Reader, build func(*Generator) UUID) (u UUID, err error) {
	if r == nil {
		return Nil, ErrEntropySource
	}
	defer func() {
		if p := recover(); p != nil {
			u = Nil
			err = ErrEntropySource
		}
	}()
	return build(NewGeneratorWithReader(r)), nil
}
