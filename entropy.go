package loghubuuid

import (
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"io"
)

// ErrEntropySource indica que a fonte de entropia informada pelo chamador
// não conseguiu entregar os bytes pedidos.
var ErrEntropySource = errors.New("loghubuuid: falha ao ler da fonte de entropia")

// NewGeneratorWithReader cria um Generator que retira entropia de um
// io.Reader, formato usual das fontes da biblioteca padrão.
//
// O leitor PRECISA ser seguro para uso concorrente, porque será chamado
// por várias goroutines ao mesmo tempo. crypto/rand.Reader satisfaz esse
// requisito.
//
// Entra em pânico se reader for nulo, e também se uma leitura falhar
// durante a geração: uma fonte de entropia quebrada não pode degradar em
// silêncio para um gerador previsível.
//
// Para a fonte criptográfica padrão, prefira NewCryptoGenerator.
func NewGeneratorWithReader(reader io.Reader) *Generator {
	if reader == nil {
		panic("loghubuuid: NewGeneratorWithReader recebeu um leitor nulo")
	}
	source := func() uint64 {
		var b [8]byte
		if _, err := io.ReadFull(reader, b[:]); err != nil {
			panic(ErrEntropySource)
		}
		return binary.BigEndian.Uint64(b[:])
	}
	return NewGeneratorWith(source)
}

// NewCryptoGenerator cria um Generator cuja entropia vem inteiramente de
// crypto/rand, tornando os bits aleatórios imprevisíveis.
//
// É mais lento que NewGenerator, que usa PCG semeado uma única vez. Use
// este quando o identificador precisar resistir a quem tenta adivinhar o
// próximo valor. Lembre-se de que, mesmo assim, todo UUIDv7 revela o
// instante de criação por construção.
func NewCryptoGenerator() *Generator {
	return NewGeneratorWithReader(crand.Reader)
}
