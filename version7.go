package loghubuuid

// Este arquivo reúne os nomes do UUIDv7: o nome por versão, GenerateV7,
// que designa o UUIDv7 padrão da RFC 9562, e os três nomes por nível,
// GenerateV7Level1, GenerateV7Level2 e GenerateV7Level3, que designam os
// níveis de precisão desta biblioteca sem o argumento de nível. Todos
// existem como método do Generator e como função de pacote, e todos são
// apelidos de uma linha de Generate, que continua sendo a única
// implementação: o compilador os embute, e o caminho quente não ganha
// chamada, desvio nem custo algum. Nenhum recebe parâmetro e nenhum tem
// forma em texto; para a string canônica use GenerateString com o nível,
// ou o método String do valor devolvido. Ver
// docs/04-uuidv7-formato-e-niveis.md seção 3.1.

// GenerateV7 produz um UUID de versão 7 padrão da RFC 9562: carimbo Unix
// em milissegundos nos 48 bits altos, versão 7, variante RFC e os 74 bits
// restantes aleatórios. É exatamente Generate(Level1), sob o nome que as
// demais versões já têm (GenerateV1, GenerateV4, GenerateV6), para que o
// UUIDv7 comum possa ser pedido pela versão, sem mencionar nível. É o
// mesmo que GenerateV7Level1.
//
// A precisão embutida é a do milissegundo. Para gravar também os
// microssegundos ou os nanossegundos, use GenerateV7Level2 ou
// GenerateV7Level3, ou Generate com Level2 ou Level3. Não há forma em
// texto, como nas demais funções por versão: use GenerateString(Level1)
// ou o método String do valor devolvido.
//
// A entropia vem da fonte deste Generator, com o mesmo consumo do Nível 1
// (duas palavras de 64 bits), e o caminho é livre de alocações. Vale o
// mesmo aviso de GenerateV4: no gerador padrão a fonte é o ChaCha8 do
// runtime do Go, e um identificador que precise ser segredo deve vir de
// NewCryptoGenerator ou de NewGeneratorWithReader.
//
// O apelido não acrescenta custo ao caminho quente: a implementação
// continua em Generate, e o compilador embute esta chamada. Ver
// docs/04-uuidv7-formato-e-niveis.md seção 3.1.
func (g *Generator) GenerateV7() UUID { return g.Generate(Level1) }

// GenerateV7 produz um UUID de versão 7 padrão (Nível 1) usando o gerador
// padrão do pacote.
func GenerateV7() UUID { return defaultGenerator.GenerateV7() }

// GenerateV7Level1 produz um UUIDv7 de Nível 1: carimbo Unix em
// milissegundos e os 74 bits restantes aleatórios, o UUIDv7 padrão da RFC
// 9562. É exatamente Generate(Level1), e portanto o mesmo que GenerateV7;
// existe para que os três níveis tenham nome, e o nível escolhido fique
// legível no ponto da chamada em vez de numa constante.
//
// Consome duas palavras de 64 bits da fonte de entropia, como o Nível 1,
// e não aloca. Não há forma em texto: use GenerateString(Level1) ou o
// método String do valor devolvido.
func (g *Generator) GenerateV7Level1() UUID { return g.Generate(Level1) }

// GenerateV7Level2 produz um UUIDv7 de Nível 2: carimbo Unix em
// milissegundos e os microssegundos do instante (0 a 999) em rand_a, com
// os 62 bits de rand_b aleatórios. É exatamente Generate(Level2), sob um
// nome que diz o nível.
//
// Consome uma única palavra de 64 bits da fonte de entropia, como o Nível
// 2, e não aloca. Não há forma em texto: use GenerateString(Level2) ou o
// método String do valor devolvido.
func (g *Generator) GenerateV7Level2() UUID { return g.Generate(Level2) }

// GenerateV7Level3 produz um UUIDv7 de Nível 3: carimbo Unix em
// milissegundos, os microssegundos do instante (0 a 999) em rand_a e os
// nanossegundos (0 a 999) nos 10 bits mais altos de rand_b, com os 52
// bits restantes aleatórios. É exatamente Generate(Level3), sob um nome
// que diz o nível.
//
// Consome uma única palavra de 64 bits da fonte de entropia, como o Nível
// 3, e não aloca. Não há forma em texto: use GenerateString(Level3) ou o
// método String do valor devolvido.
func (g *Generator) GenerateV7Level3() UUID { return g.Generate(Level3) }

// GenerateV7Level1 produz um UUIDv7 de Nível 1 usando o gerador padrão do
// pacote. É o mesmo que GenerateV7 e que Generate(Level1).
func GenerateV7Level1() UUID { return defaultGenerator.GenerateV7Level1() }

// GenerateV7Level2 produz um UUIDv7 de Nível 2 usando o gerador padrão do
// pacote. É o mesmo que Generate(Level2).
func GenerateV7Level2() UUID { return defaultGenerator.GenerateV7Level2() }

// GenerateV7Level3 produz um UUIDv7 de Nível 3 usando o gerador padrão do
// pacote. É o mesmo que Generate(Level3).
func GenerateV7Level3() UUID { return defaultGenerator.GenerateV7Level3() }
