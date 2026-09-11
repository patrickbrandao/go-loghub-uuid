package loghubuuid

import "time"

// maxBoundMilli é o maior carimbo que cabe no campo de 48 bits de
// milissegundos do UUIDv7, equivalente a 10889-08-02T05:31:50.655Z.
const maxBoundMilli = int64(1)<<48 - 1

// maxBoundSec é o mesmo limite em segundos. Serve de guarda antes da
// multiplicação por mil feita na decomposição do instante: um time.Time
// comporta anos muito além disso, e o produto estouraria o inteiro com
// sinal, dando a volta para um valor negativo.
const maxBoundSec = maxBoundMilli / 1_000

// saturatedInstant decompõe um instante recebido do chamador nos campos
// gravados pelo UUIDv7, saturando nas duas pontas da faixa representável.
//
// Abaixo da época Unix o resultado é a própria época, exatamente como em
// splitUnixInstant e, portanto, como em Generate. Acima da faixa o
// resultado é o último instante representável,
// 10889-08-02T05:31:50.655999999Z, e os campos abaixo do milissegundo
// saturam junto com ele: zerá-los faria o resultado regredir ao cruzar a
// borda.
//
// Saturar nas duas pontas mantém monotônico tudo que é construído a
// partir de um instante, que é o que torna a consulta por intervalo
// correta. Truncar os bits excedentes, como o empacotamento por
// deslocamento do caminho quente faria, deixaria o carimbo dar a volta.
//
// É a versão para instante vindo por parâmetro: splitUnixInstant atende
// o caminho quente, onde o instante vem do relógio do sistema e nunca
// chega perto do teto, e por isso não paga estas duas comparações.
func saturatedInstant(t time.Time) (ms int64, micro, nano uint16) {
	sec := t.Unix()
	switch {
	case sec < 0:
		// Qualquer segundo negativo produz milissegundo negativo, então
		// o piso na época já está decidido aqui. Tratá-lo antes da
		// multiplicação também protege contra o estouro ao contrário.
		return 0, 0, 0
	case sec > maxBoundSec:
		return maxBoundMilli, 999, 999
	}

	ms, micro, nano = splitUnixInstant(sec, int64(t.Nanosecond()))
	if ms > maxBoundMilli {
		return maxBoundMilli, 999, 999
	}
	return ms, micro, nano
}

// packV7 monta os 16 bytes de um UUIDv7 a partir dos campos de tempo já
// decompostos e dos bits livres do nível.
//
// É o empacotamento compartilhado por tudo que constrói um UUIDv7 a
// partir de um instante explícito: GenerateAt passa bits sorteados,
// MinAt passa zeros e MaxAt passa uns. O caminho quente NÃO passa por
// aqui: Generate mantém a sua própria cópia do empacotamento, para não
// pagar uma chamada. Ver docs/SPEC.md seção 11.2.
//
// freeA são os 12 bits de rand_a, usados só no Nível 1 — nos níveis 2 e
// 3 o campo carrega os microssegundos. freeB são os bits livres de
// rand_b: 62 nos níveis 1 e 2, e apenas os 52 baixos no Nível 3, onde os
// 10 altos carregam os nanossegundos.
//
// Versão 7 e variante RFC são gravadas sempre, qualquer que seja o
// conteúdo de freeA e freeB.
func packV7(level Level, ms int64, micro, nano, freeA uint16, freeB uint64) UUID {
	var u UUID

	// --- 48 bits de milissegundos (bytes 0..5) ---
	u[0] = byte(ms >> 40)
	u[1] = byte(ms >> 32)
	u[2] = byte(ms >> 24)
	u[3] = byte(ms >> 16)
	u[4] = byte(ms >> 8)
	u[5] = byte(ms)

	// --- rand_a (12 bits): versão + microssegundos OU bits livres ---
	var randA uint16
	switch level {
	case Level2, Level3:
		randA = micro & 0x0FFF
	default: // Level1 e desconhecidos
		randA = freeA & 0x0FFF
	}
	u[6] = 0x70 | byte((randA>>8)&0x0F) // nibble alto = versão 7
	u[7] = byte(randA)

	// --- rand_b (62 bits): variante + nanossegundos (nível 3) + livres ---
	var randB uint64
	switch level {
	case Level3:
		randB = (uint64(nano&0x03FF) << 52) | (freeB & ((1 << 52) - 1))
	default: // Level1 e Level2
		randB = freeB & ((1 << 62) - 1)
	}
	u[8] = 0x80 | byte((randB>>56)&0x3F) // bits 7..6 = variante "10"
	u[9] = byte(randB >> 48)
	u[10] = byte(randB >> 40)
	u[11] = byte(randB >> 32)
	u[12] = byte(randB >> 24)
	u[13] = byte(randB >> 16)
	u[14] = byte(randB >> 8)
	u[15] = byte(randB)

	return u
}

// GenerateAt produz um UUIDv7 para um instante informado pelo chamador,
// em vez do instante atual. É o sentido inverso de Import: ali se lê o
// tempo de um identificador, aqui se constrói um identificador para um
// tempo.
//
// Serve para reprocessar um histórico, semear dados de teste e importar
// registros antigos preservando a ordenação cronológica da chave.
//
// Os campos de tempo são gravados exatamente como em Generate, com a
// mesma distribuição por nível, e os bits livres recebem entropia deste
// gerador. Duas chamadas com o mesmo instante devolvem UUIDs
// DIFERENTES: é um gerador, não um construtor determinístico. Para a
// forma determinística de um instante, use MinAt e MaxAt.
//
// A unicidade vem inteiramente dos bits livres, que são 74 no Nível 1,
// 62 no Nível 2 e 52 no Nível 3. Como o instante deixa de vir do
// relógio, nada impede o chamador de gerar em volume para um único
// instante, e é só aí que essa margem começa a importar.
//
// Instantes anteriores a 1970-01-01T00:00:00Z degradam para a própria
// época, como em Generate. Instantes posteriores a
// 10889-08-02T05:31:50.655999999Z saturam no último instante
// representável, porque o campo de milissegundos tem 48 bits.
//
// Níveis desconhecidos são tratados como Level1, como em Generate.
func (g *Generator) GenerateAt(level Level, t time.Time) UUID {
	// Mesma proteção de Generate: um Generator montado fora dos
	// construtores não tem fonte de entropia e usa a do gerador padrão
	// em vez de derrubar o processo.
	if g == nil || g.oneWord == nil || g.twoWords == nil {
		g = defaultGenerator
	}

	ms, micro, nano := saturatedInstant(t)

	// Mesma economia de Generate: nos níveis 2 e 3 rand_a carrega os
	// microssegundos, então basta uma palavra de 64 bits.
	var r1, r2 uint64
	if level == Level2 || level == Level3 {
		r2 = g.oneWord()
	} else {
		r1, r2 = g.twoWords()
	}

	return packV7(level, ms, micro, nano, uint16(r1), r2)
}

// GenerateAtString produz, para um instante informado, um UUIDv7 já no
// formato string canônico. Valem as mesmas observações de GenerateAt.
func (g *Generator) GenerateAtString(level Level, t time.Time) string {
	return g.GenerateAt(level, t).String()
}

// GenerateAt produz um UUIDv7 para o instante informado usando o
// gerador padrão do pacote. Ver o método de mesmo nome em Generator.
func GenerateAt(level Level, t time.Time) UUID {
	return defaultGenerator.GenerateAt(level, t)
}

// GenerateAtString produz, para o instante informado, um UUIDv7 em
// string usando o gerador padrão do pacote.
func GenerateAtString(level Level, t time.Time) string {
	return defaultGenerator.GenerateAtString(level, t)
}
