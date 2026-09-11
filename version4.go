package loghubuuid

import "encoding/binary"

// GenerateV4 produz um UUID de versão 4: 122 bits aleatórios, sem qualquer
// informação de tempo, com versão e variante fixas.
//
// A entropia vem da fonte deste Generator. No gerador padrão isso
// significa o ChaCha8 do runtime do Go, que resiste a predição mas cuja
// documentação recomenda crypto/rand para uso sensível a segurança: para
// identificadores que precisem ser segredo, monte o gerador com
// NewCryptoGenerator ou NewGeneratorWithReader.
//
// O caminho é livre de alocações, como o dos demais geradores binários.
func (g *Generator) GenerateV4() UUID {
	if g == nil || g.oneWord == nil || g.twoWords == nil {
		g = defaultGenerator
	}
	a, b := g.twoWords()

	var u UUID
	binary.BigEndian.PutUint64(u[0:8], a)
	binary.BigEndian.PutUint64(u[8:16], b)
	u[6] = (u[6] & 0x0F) | 0x40 // nibble alto = versão 4
	u[8] = (u[8] & 0x3F) | 0x80 // bits 7..6 = variante "10"
	return u
}

// GenerateV4 produz um UUID de versão 4 usando o gerador padrão do pacote.
func GenerateV4() UUID { return defaultGenerator.GenerateV4() }
