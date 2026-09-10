// Pacote loghubuuid implementa a geração de identificadores UUIDv7
// (RFC 9562) com três níveis de precisão temporal.
//
// O UUIDv7 é um identificador de 128 bits ordenável no tempo: os bits
// mais significativos carregam o instante de criação, de modo que a
// ordenação lexicográfica das strings coincide com a ordem cronológica.
//
// Esta biblioteca estende o padrão guardando precisão sub-milissegundo
// dentro dos campos "aleatórios" do UUID, em três níveis:
//
//	Level1: apenas milissegundos (48 bits padrão). rand_a e rand_b
//	        totalmente aleatórios. Compatível 100% com UUIDv7 comum.
//
//	Level2: invade rand_a (12 bits) para guardar os microssegundos
//	        (0-999) do instante. rand_b continua aleatório.
//
//	Level3: invade rand_a para os microssegundos e ocupa os 10 bits
//	        mais altos de rand_b para os nanossegundos (0-999). Os
//	        52 bits restantes de rand_b continuam aleatórios.
//
// Em todos os níveis a versão (7) e a variante (RFC) são preservadas,
// portanto o resultado é sempre um UUID válido.
//
// Concorrência: o tipo Generator é seguro para uso simultâneo por
// centenas de goroutines. Crie um único Generator no boot da aplicação
// (ou use as funções de pacote, que usam um gerador padrão interno).
package loghubuuid

import (
	crand "crypto/rand"
	"encoding/binary"
	rand "math/rand/v2"
	"sync"
	"time"
)

// Level define quanta precisão temporal é embutida no UUIDv7.
type Level uint8

const (
	// Level1 grava apenas milissegundos (48 bits). rand_a e rand_b
	// são totalmente aleatórios. É o UUIDv7 padrão da RFC 9562.
	Level1 Level = 1

	// Level2 grava milissegundos + microssegundos (0-999) em rand_a.
	Level2 Level = 2

	// Level3 grava milissegundos + microssegundos (em rand_a) +
	// nanossegundos (0-999) nos 10 bits mais altos de rand_b.
	Level3 Level = 3
)

// UUID é o valor binário de 128 bits (16 bytes, big-endian / ordem de rede).
type UUID [16]byte

// Generator é o objeto que produz UUIDs. Deve ser criado uma única vez,
// no boot da aplicação, e pode ser compartilhado por muitas goroutines.
//
// Internamente o gerador padrão mantém um pool de geradores
// pseudoaleatórios rápidos (PCG) — um por thread em uso — eliminando
// contenção de lock e permitindo gerar milhões de IDs por segundo.
//
// O valor zero de Generator NÃO é utilizável: use NewGenerator ou
// NewGeneratorWith. Por segurança, um Generator sem fonte de entropia
// (valor zero, ou ponteiro nulo) recorre ao gerador padrão do pacote em
// vez de entrar em pânico.
type Generator struct {
	// oneWord devolve um bloco de 64 bits aleatórios. É usado nos níveis
	// 2 e 3, onde rand_a carrega o tempo e só rand_b precisa de entropia.
	oneWord func() uint64

	// twoWords devolve dois blocos de 64 bits aleatórios. É usado no
	// nível 1, o único em que rand_a também é aleatório.
	twoWords func() (uint64, uint64)
}

// NewGenerator cria um Generator rápido e seguro para concorrência.
//
// A entropia em tempo de execução vem de geradores PCG mantidos em um
// sync.Pool (sem locks compartilhados). Cada PCG é semeado uma única
// vez, na criação, a partir de crypto/rand (alta qualidade), e depois
// avança de forma puramente local — barato e contention-free.
//
// ATENÇÃO — o PCG é um gerador pseudoaleatório estatístico, NÃO
// criptográfico: quem observar alguns UUIDs produzidos por este gerador
// consegue reconstruir o estado interno e prever os seguintes. Além
// disso, todo UUIDv7 expõe o instante de criação por construção. Não use
// estes identificadores como segredo (token de sessão, link privado,
// chave de recuperação); para esse fim, monte o gerador com entropia
// criptográfica através de NewGeneratorWith.
func NewGenerator() *Generator {
	pool := &sync.Pool{
		New: func() any {
			// Semeia cada PRNG do pool com bytes de crypto/rand.
			return rand.New(rand.NewPCG(strongSeed(), strongSeed()))
		},
	}
	return &Generator{
		oneWord: func() uint64 {
			r := pool.Get().(*rand.Rand)
			a := r.Uint64()
			pool.Put(r)
			return a
		},
		twoWords: func() (uint64, uint64) {
			r := pool.Get().(*rand.Rand)
			a, b := r.Uint64(), r.Uint64()
			pool.Put(r)
			return a, b
		},
	}
}

// NewGeneratorWith cria um Generator usando uma fonte de aleatoriedade
// personalizada. A função "source" deve devolver 64 bits aleatórios e
// PRECISA ser segura para uso concorrente (ela será chamada por várias
// goroutines ao mesmo tempo).
//
// Use isto, por exemplo, para forçar entropia criptográfica
// (crypto/rand) em todas as gerações, abrindo mão de parte da
// velocidade em troca de imprevisibilidade total.
//
// A fonte é chamada uma única vez por UUID nos níveis 2 e 3 (onde
// rand_a carrega o tempo) e duas vezes no nível 1.
//
// Entra em pânico se source for nula: é um erro de configuração, que
// deve aparecer no boot e não na primeira geração.
func NewGeneratorWith(source func() uint64) *Generator {
	if source == nil {
		panic("loghubuuid: NewGeneratorWith recebeu uma fonte de entropia nula")
	}
	return &Generator{
		oneWord:  source,
		twoWords: func() (uint64, uint64) { return source(), source() },
	}
}

// strongSeed lê 8 bytes de crypto/rand e os converte em uint64.
//
// Entra em pânico se a leitura falhar: uma fonte de entropia quebrada não
// pode degradar em silêncio para sementes derivadas do relógio, que
// tornariam todos os PRNGs do pool correlacionados e previsíveis. A partir
// do Go 1.24 crypto/rand.Read nunca devolve erro (o próprio runtime
// encerra o processo), então este ramo só é alcançável em toolchains
// anteriores.
func strongSeed() uint64 {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		panic("loghubuuid: falha ao ler crypto/rand para semear o gerador padrão: " + err.Error())
	}
	return binary.LittleEndian.Uint64(b[:])
}

// Generate produz um UUID binário (128 bits) do nível informado, usando
// o instante atual (time.Now). Níveis desconhecidos são tratados como
// Level1 (totalmente padrão).
//
// Layout dos 16 bytes (big-endian):
//
//	bytes 0..5 : unix_ts_ms .......... 48 bits  (milissegundos desde epoch)
//	byte  6    : 0x7_ | rand_a[11:8] . versão(4) + 4 bits altos de rand_a
//	byte  7    : rand_a[7:0] ......... 8 bits baixos de rand_a (total 12 bits)
//	byte  8    : 10_ | rand_b[61:56] . variante(2) + 6 bits altos de rand_b
//	bytes 9..15: rand_b[55:0] ........ 56 bits restantes de rand_b (total 62 bits)
//
// Onde, conforme o nível:
//
//	rand_a (12 bits) = microssegundos (0-999) nos níveis 2 e 3; aleatório no nível 1.
//	rand_b[61:52]    = nanossegundos  (0-999) no nível 3; aleatório nos níveis 1 e 2.
//	demais bits      = aleatórios.
//
// O campo de 48 bits de milissegundos comporta datas até 10889-08-02.
// Relógios anteriores à época Unix (1970-01-01) degradam para a própria
// época, em vez de produzirem um timestamp corrompido.
func (g *Generator) Generate(level Level) UUID {
	// Um Generator montado fora dos construtores (valor zero embutido em
	// outra struct, ou ponteiro nulo) não tem fonte de entropia: usa a do
	// gerador padrão do pacote em vez de derrubar o processo.
	if g == nil || g.oneWord == nil || g.twoWords == nil {
		g = defaultGenerator
	}

	// Unix()/Nanosecond() em vez de UnixNano(): o inteiro de nanossegundos
	// satura em 2262-04-11, enquanto a leitura em duas partes não tem esse
	// limite e Nanosecond() nunca devolve valor negativo.
	now := time.Now()
	ms, micro, nano := splitUnixInstant(now.Unix(), int64(now.Nanosecond()))

	// Nos níveis 2 e 3 rand_a carrega os microssegundos, portanto r1 seria
	// descartado: nesses casos sorteia-se uma única palavra de 64 bits.
	var r1, r2 uint64
	if level == Level2 || level == Level3 {
		r2 = g.oneWord()
	} else {
		r1, r2 = g.twoWords()
	}

	var u UUID

	// --- 48 bits de milissegundos (bytes 0..5) ---
	u[0] = byte(ms >> 40)
	u[1] = byte(ms >> 32)
	u[2] = byte(ms >> 24)
	u[3] = byte(ms >> 16)
	u[4] = byte(ms >> 8)
	u[5] = byte(ms)

	// --- rand_a (12 bits): versão + microssegundos OU aleatório ---
	var randA uint16
	switch level {
	case Level2, Level3:
		randA = micro & 0x0FFF // 0..999 cabe folgado em 12 bits
	default: // Level1 e desconhecidos
		randA = uint16(r1) & 0x0FFF
	}
	u[6] = 0x70 | byte((randA>>8)&0x0F) // nibble alto = versão 7
	u[7] = byte(randA)

	// --- rand_b (62 bits): variante + nanossegundos (nível 3) + aleatório ---
	var randB uint64
	switch level {
	case Level3:
		// nano ocupa os 10 bits mais altos do campo de 62 bits (bits 61..52);
		// os 52 bits inferiores recebem aleatoriedade.
		randB = (uint64(nano&0x03FF) << 52) | (r2 & ((1 << 52) - 1))
	default: // Level1 e Level2
		randB = r2 & ((1 << 62) - 1)
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

// splitUnixInstant decompõe um instante lido em duas partes — segundos
// desde a época Unix e fração do segundo em nanossegundos (0..999_999_999)
// — nos campos gravados pelo UUIDv7: milissegundos desde a época,
// microssegundos dentro do milissegundo e nanossegundos dentro do
// microssegundo. Instantes anteriores à época degradam para a própria
// época, com os campos sub-milissegundo zerados. É pura e pequena o
// bastante para ser embutida pelo compilador no caminho quente.
func splitUnixInstant(sec, nsec int64) (ms int64, micro, nano uint16) {
	ms = sec*1_000 + nsec/1_000_000 // milissegundos desde a época
	if ms < 0 {
		// Relógio ajustado para antes de 1970: sem esse piso, os campos
		// sub-milissegundo estourariam a faixa 0..999 ao virarem uint16.
		ms, nsec = 0, 0
	}
	rem := nsec % 1_000_000 // parte sub-milissegundo: 0..999_999 ns
	return ms, uint16(rem / 1_000), uint16(rem % 1_000)
}

// GenerateString produz um UUID do nível informado já no formato string
// canônico (8-4-4-4-12, minúsculas).
func (g *Generator) GenerateString(level Level) string {
	return g.Generate(level).String()
}

// Version devolve o nibble de versão (deve ser 7 para UUIDs gerados aqui).
func (u UUID) Version() byte { return u[6] >> 4 }

// Variant devolve os 2 bits altos do byte 8 (deve ser 0b10 = 2 = variante RFC).
func (u UUID) Variant() byte { return u[8] >> 6 }

// --- gerador padrão de pacote (para uso rápido) ---

// defaultGenerator é um Generator único compartilhado, criado na carga do pacote.
var defaultGenerator = NewGenerator()

// Generate produz um UUID binário usando o gerador padrão do pacote.
func Generate(level Level) UUID { return defaultGenerator.Generate(level) }

// GenerateString produz um UUID em string usando o gerador padrão do pacote.
func GenerateString(level Level) string { return defaultGenerator.GenerateString(level) }
