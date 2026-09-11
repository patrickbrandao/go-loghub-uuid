# Histórico de mudanças — go-loghub-uuid

Guia do histórico do projeto: o que mudou em cada versão, por quê, e o
que isso significa para quem consome a biblioteca. As versões seguem o
versionamento semântico e as tags publicadas são imutáveis (nunca são
movidas com `-f`, para não quebrar o `sum.golang.org` dos usuários).

Convenções de cada seção:

- **Adicionado**: API ou capacidade nova.
- **Alterado**: comportamento existente que mudou; leia com atenção ao
  atualizar.
- **Corrigido**: defeito removido.
- **Documentação**: mudanças só em texto.
- **Decisões**: propostas avaliadas e rejeitadas, com o motivo, para que
  não voltem a ser sugeridas sem argumento novo.

---

## [Não publicado]

Passagem de auditoria estática de 2026-09-10 sobre a `v0.3.0`: seis
problemas encontrados e corrigidos, todos verificados com `gofmt`,
`go vet` e a suíte sob detector de corrida no Go 1.22 (mínimo declarado)
e no Go 1.27. Em 2026-09-11, a integração contínua foi ampliada (linter,
outros sistemas, cobertura, corpus de fuzzing) e a documentação ganhou
exemplos executáveis, guia de release, política de segurança e guia de
contribuição. Nenhuma mudança de comportamento na biblioteca.

### Alterado

- **`Scan` trata texto vazio como ausência de valor.** `UUID.Scan`
  passou a gravar o UUID nulo, sem erro, para `""` e para `[]byte{}`,
  como já fazia para `NULL` e como faz o pacote `github.com/google/uuid`.
  `NullUUID.Scan` produz `Valid` falso nesses casos. Antes, uma coluna de
  texto com valor vazio fazia a leitura falhar com erro de comprimento,
  o que quebrava aplicações migradas do pacote de origem. Texto inválido
  continua devolvendo erro sem alterar o receptor. (`sql.go`)
- **`IsInvalidLengthError` reconhece erros embrulhados.** Passou a usar
  `errors.Is`, então um erro de comprimento envolvido por
  `fmt.Errorf("...: %w", err)` continua sendo reconhecido, como no pacote
  de origem. Resultado idêntico para erros não embrulhados. (`parse.go`)
- **`TimestampWithLevel(Level3)` descarta os dois campos quando um
  denuncia ruído.** Se os microssegundos lidos de `rand_a` estiverem
  fora de 0..999, os nanossegundos do topo de `rand_b` também são
  ignorados, porque os dois vêm da mesma geração e um `rand_a` aleatório
  prova que `rand_b` também é. Antes, o campo que por acaso coubesse na
  faixa era somado ao instante. Só muda o resultado para UUIDs que não
  são do nível informado. (`inspect.go`)
- **`SetClockSequence` mantém um piso de relógio por sequência.** Cada
  sequência de relógio lembra o último instante que emitiu; voltar a uma
  sequência já usada continua a partir dali, e só a entrada em uma
  sequência inédita descarta o adiantamento acumulado do relógio
  interno. `SetClockSequence(-1)` passou a sortear sempre uma sequência
  ainda não usada no processo, e por isso é a forma determinística de
  ressincronizar com o relógio do sistema. Antes, qualquer troca zerava
  o piso, e voltar a uma sequência antiga enquanto o relógio real ainda
  estava atrás do adiantamento podia repetir um UUIDv1 ou UUIDv6. A
  invariante garantida agora: para cada sequência, os instantes
  emitidos são estritamente crescentes durante toda a vida do processo,
  logo nenhum par (instante, sequência) se repete. Difere do pacote
  `github.com/google/uuid`, que tem a fraqueza antiga. O caminho do
  UUIDv7 não foi tocado. (`clock.go`)

### Corrigido

- **`NullUUID.UnmarshalJSON` interpreta escapes JSON.** Uma string com
  sequências como `\u0030` era passada crua ao analisador e rejeitada,
  enquanto o tipo `UUID`, que usa o `encoding/json`, a aceitava. Agora,
  se houver barra invertida no conteúdo, a decodificação é delegada ao
  `encoding/json`; sem escapes, o caminho continua direto e sem
  alocação. Erros de sintaxe JSON viram `ErrInvalidFormat`. (`sql.go`)

### Adicionado

- **Integração contínua** em `.github/workflows/ci.yml`. A cada push,
  pull request e tag: `gofmt` (na versão estável), `go vet`, `go build`,
  suíte em `-short` sob `-race`, travas de alocação em passo próprio e
  um benchmark curto com `-benchmem`, em matriz com Go 1.22 e a versão
  estável. Semanalmente e sob demanda: suíte completa com os testes de
  massa e 60 segundos de fuzzing em cada analisador.
- **Travas de alocação puladas sob `-race`.** A matriz revelou que, no
  Go 1.22, `go test -race` falhava de forma intermitente nas travas de
  alocação do gerador padrão. Não é defeito da biblioteca: o `sync.Pool`
  compilado com o detector de corrida descarta de propósito um em cada
  quatro itens devolvidos, e `testing.AllocsPerRun` contava as
  realocações do PRNG. Os arquivos `tests/race_enabled_test.go` e
  `tests/race_disabled_test.go` (tags de compilação) sinalizam o
  detector, e as três travas que dependem do pool são puladas sob
  `-race`; a medição válida é a feita sem o detector, que o CI executa.
- `FuzzNullUUIDJSON`: alvo de fuzzing para o leitor de JSON de
  `NullUUID`, que exige concordância com o tipo `UUID` lido pelo
  `encoding/json` em aceitar, recusar e no valor produzido. Incluído no
  job semanal `deep` ao lado dos outros dois alvos.
- **Linter estático** com `golangci-lint` (versão 2), configurado em
  `.golangci.yml` na raiz: `errcheck`, `govet`, `staticcheck`, `unused`,
  `ineffassign`, `gosec`, `errorlint`, `revive` (comentário em todo
  exportado) e `nolintlint`. `misspell` fica desligado porque só conhece
  inglês. A regra `G115` do `gosec` (truncamento em conversão de inteiro)
  é excluída de propósito: empacotar campos em bytes por deslocamento e
  truncamento é o que a biblioteca faz, e o caminho quente não ganhou
  máscara alguma (benchmarks idênticos antes e depois). Roda no CI na
  versão estável. As duas marcações `//nolint:errcheck` de
  `namebased.go` eram desnecessárias (`errcheck` já ignora `hash.Hash.Write`)
  e foram trocadas por comentário comum; as demais ganharam o linter e o
  motivo que o `nolintlint` exige. Em testes, três comparações diretas
  com `ErrInvalidFormat` passaram a `errors.Is` (a comparação direta,
  que é o contrato de `FromString`, continua testada em
  `TestFromStringErrorUnchanged`), o auxiliar `safeFromString` passou a
  devolver o erro por último e o aquecimento de `benchmark-bulk` deixou
  de fazer uma atribuição inútil. (`.golangci.yml`, `ci.yml`,
  `namebased.go`, `uuid.go` só em comentário, `tests/`)
- **Compilação cruzada no CI** para `windows/amd64`, `darwin/arm64` e
  `linux/arm64` (`go build` e `go vet`) no job `test`, e o job novo
  `test-os`, que roda `go vet`, `go build` e `go test ./... -short` em
  `windows-latest` e `macos-latest` com a versão estável, em pull
  request, tag, no agendamento semanal e sob demanda. (`ci.yml`)
- **Cobertura de testes no CI**: o job `test`, na versão estável, gera o
  perfil com `-coverpkg` (a suíte é outro pacote), imprime
  `go tool cover -func` no log, publica `cover.out` e `cover.html` como
  artefato `cobertura` e falha abaixo de 95%. Medida em 2026-09-11:
  98,2% das instruções do pacote da raiz. (`ci.yml`)
- **Corpus de fuzzing preservado**: no job `deep`, os três alvos rodam
  sempre (`continue-on-error`), o diretório `tests/testdata/fuzz/` é
  publicado como artefato `fuzz-corpus` (30 dias) quando existe, e um
  passo final falha o job se alguma campanha tiver falhado. Antes, a
  entrada que quebrava um alvo morria com o runner. (`ci.yml`)
- **Exemplos executáveis** em `example_test.go`, na raiz, pacote externo
  `loghubuuid_test`: `ExampleGenerateString`, `ExampleGenerator_Generate`,
  `ExampleFromString`, `ExampleImportBinary`, `ExampleParse`,
  `ExampleUUID_TimestampWithLevel`, `ExampleGenerateV5`,
  `ExampleNullUUID` e `ExampleNewCryptoGenerator`. Os que declaram
  saída usam só vetores fixos. Ficam na raiz porque o godoc só associa
  exemplos ao pacote quando estão no mesmo diretório; a exceção à regra
  da raiz mínima está registrada em `CLAUDE.md` e em `STARTHERE.md`.
- Testes novos para os ramos que a medição de cobertura apontou:
  `TestNullUUIDTextAndBinary` (texto e binário de `NullUUID`, com valor,
  ausente e inválido), `TestMustPropagatesError`,
  `TestVariantStringCoversAllCodes`, `TestNewHashMatchesGenerateHash`
  (inclui resumo menor que 16 bytes) em `tests/api_test.go`;
  `TestDomainString` e `TestInspectorsRejectOtherVersions` em
  `tests/versions_test.go`.
- Testes novos: `TestClockSequenceReuseNeverRepeats`,
  `TestSetClockSequenceRandomIsFresh`,
  `TestTimestampWithLevelDiscardsOutOfRangeFields` e casos adicionais em
  `TestSQLScanAndValue`, `TestNullUUID` e
  `TestInvalidLengthIsDistinguishable` (em `tests/`);
  `TestSequenceFloorSurvivesRoundTrip` e
  `TestUnusedSequenceSkipsUsedOnes` (internos, em
  `clock_internal_test.go`, porque simulam o adiantamento diretamente no
  estado do relógio, sem depender da velocidade do host).

### Documentação

- `docs/SPEC.md` alinhada ao código, com a RFC 9562 como critério de
  desempate: layout da versão 6 reescrito (`time_high` 32 bits,
  `time_mid` 16 bits, `time_low` 12 bits, conforme §5.6); assinaturas
  reais de `GetTime` e `NodeID`; nó padrão sorteado com bit multicast
  (§6.10) em vez de leitura de MAC; vetores dourados restritos aos que a
  RFC publica (Apêndice A, espaço DNS); regra normativa do piso de
  relógio por sequência em §4.2.
- `docs/MIGRATION.md`: substituída a afirmação de que os apelidos com
  erro "nunca falham" pela regra real (`NewRandom`, `NewV7`,
  `NewRandomFromReader` e `NewV7FromReader` podem devolver
  `ErrEntropySource`); notas sobre `Scan` com texto vazio,
  `IsInvalidLengthError` e o piso por sequência.
- `docs/DEPLOY-FULL.md`: faixas reais dos campos `Microseconds` (até
  4095 em Nível 1) e `Nanoseconds` (até 1023 em Níveis 1 e 2) da struct
  `Time`; texto vazio em `Scan`; como ressincronizar o relógio das
  versões 1 e 6.
- `docs/TEST-AND-BENCHMARK.md`: seção de integração contínua e nota
  sobre as travas de alocação sob `-race`. `docs/git.md`: só etiquetar
  com o fluxo verde. `README.md`: selo do CI. `STARTHERE.md`: árvore
  atualizada.
- `docs/DEPLOY-FULL.md`: avisos sobre a resolução do relógio do host
  (o campo de nanossegundos do Nível 3 é sempre zero em hosts com relógio
  de microssegundo) e sobre relógio do sistema atrasado, no UUIDv7 e nas
  versões 1 e 6. Comentário de `SetNodeID` explicita que o bit multicast
  de um nó fornecido pelo chamador é responsabilidade dele.
- `CLAUDE.md` atualizado: raiz com `CHANGELOG.md` e `.github/`, comandos
  de CI e de fuzzing, o piso de relógio por sequência, `Scan` com texto
  vazio e a regra sobre as travas de alocação sob `-race`.
- `docs/git.md` virou `docs/RELEASE.md`: guia de release executável do
  início ao fim (pré-requisitos, tag anotada, `gh release`, verificação
  pelo proxy de módulos e a regra de imutabilidade das tags com o motivo),
  sem configuração de máquina, sem `git config --global` e sem os blocos
  repetidos ou vazios do rascunho anterior.
- `SECURITY.md` na raiz: versões suportadas, relato privado pelo recurso
  de aviso de segurança do GitHub, prazo de resposta e o resumo do modelo
  de ameaça já documentado (gerador padrão não serve para segredo, o
  UUIDv7 expõe o instante, as versões 1, 2 e 6 expõem nó e sequência,
  `NewCryptoGenerator` para identificadores inadivinháveis). O recurso
  "Report a vulnerability" precisa ser ligado nas configurações do
  repositório, ação do dono.
- `CONTRIBUTING.md` na raiz: as convenções de `CLAUDE.md` que valem para
  humanos, os comandos de verificação local e a regra de que toda
  mudança de comportamento vem com teste e entrada neste arquivo.
- `docs/TEST-AND-BENCHMARK.md`: tabela de referência das versões 1, 4, 5
  e 6, de `GenerateV1Parallel`, `Parse` e `ImportBinary` (Apple M2, Go
  1.27, mediana de três execuções), com a leitura do custo do mutex em
  paralelo (~148 ns contra ~10,8 ns do UUIDv7) e a confirmação de que o
  piso por sequência não custa nada por geração (`GenerateV1` ~46,8 ns
  contra ~48,3 ns na `v0.3.0`, mesma sessão); seções novas sobre
  exemplos executáveis, linter, cobertura, os três jobs do CI e como
  reproduzir uma falha de fuzzing a partir do artefato.
- `STARTHERE.md`: árvore com `.golangci.yml`, `example_test.go`,
  `SECURITY.md`, `CONTRIBUTING.md` e `docs/RELEASE.md`; caminhos de
  leitura para release e contribuição. `README.md`: links para
  `CONTRIBUTING.md` e `SECURITY.md`. `CLAUDE.md`: exceções da raiz,
  comandos de linter e cobertura, descrição dos três jobs do CI.

---

## [v0.3.0] — 2026-09-10

Cobertura completa da RFC 9562 e API de apoio, sem tocar no caminho
quente do UUIDv7, que continua sem trava e sem alocação. Tudo o que foi
acrescentado vive em arquivos próprios.

### Adicionado

- **Todas as demais versões de UUID.** `GenerateV1` e `GenerateV6`
  (tempo gregoriano em tiques de 100 ns, sequência de relógio de 14 bits
  e nó de 48 bits, com relógio interno estritamente crescente e trava
  própria em `clock.go`); `GenerateV2`, `GenerateV2Person` e
  `GenerateV2Group` (DCE Security); `GenerateV3` (MD5) e `GenerateV5`
  (SHA-1) com os quatro espaços de nomes da RFC e `GenerateHash` para
  esquemas próprios; `GenerateV4` no `Generator` e como função de
  pacote; `GenerateV8` de conteúdo livre e `GenerateV8Random`.
- **Estado do relógio compartilhado**: `GetTime`, `ClockSequence`,
  `SetClockSequence`, `NodeID` e `SetNodeID`. O nó padrão é sorteado uma
  vez com o bit multicast ligado; a biblioteca não lê interfaces de rede
  para não arrastar o pacote `net`.
- **Análise permissiva de texto** em `parse.go`: `Parse` e `ParseBytes`
  aceitam a forma canônica, entre chaves, com prefixo `urn:uuid:` e
  hexadecimal cru de 32 dígitos, sem alocar; `MustParse`, `Must`,
  `Validate`, `FromBytes`. Erros específicos `ErrInvalidLength` e
  `ErrInvalidBrackets` embrulham `ErrInvalidFormat`, então
  `errors.Is(err, ErrInvalidFormat)` continua valendo. `FromString` não
  mudou e continua devolvendo exatamente o sentinela antigo.
- **Serialização** em `encoding.go`: `MarshalText`, `UnmarshalText`,
  `MarshalBinary`, `UnmarshalBinary`.
- **Banco de dados** em `sql.go`: `Scan`, `Value` e o tipo `NullUUID`
  com serializações em JSON, texto e binário.
- **Inspeção** em `inspect.go`: `Timestamp` (versões 1, 6 e 7),
  `TimestampWithLevel` (recupera a precisão sub-milissegundo dos níveis
  2 e 3), `GregorianTime`, `ClockSequence`, `NodeID`, `Domain` e `ID`,
  todos com segundo retorno booleano quando a versão não carrega o
  campo. Para a versão 2, `ClockSequence` devolve só os 6 bits altos,
  porque o byte baixo é o domínio.
- **Valores e utilitários** em `values.go`: `Nil`, `Max`, `IsZero`,
  `IsMax`, `Compare`, `URN`, o tipo `UUIDs` com `Strings`,
  `VersionString` e `VariantString`.
- **Entropia** em `entropy.go`: `NewGeneratorWithReader` e
  `NewCryptoGenerator`, com pânico se a fonte falhar durante a geração
  (uma fonte quebrada não pode degradar em silêncio); `ErrEntropySource`.
- **Compatibilidade com `github.com/google/uuid`** em `compat.go`: `New`,
  `NewString`, `NewRandom`, `NewRandomFromReader`, `NewUUID`, `NewV6`,
  `NewV7`, `NewV7FromReader`, `NewMD5`, `NewSHA1`, `NewHash`,
  `NewDCESecurity`, `NewDCEPerson`, `NewDCEGroup`. Os apelidos que
  produzem bits aleatórios leem de `crypto/rand`, como o pacote de
  origem. Guia em `docs/MIGRATION.md`.
- Testes: `tests/versions_test.go` (versões 1 a 8, vetores da RFC 9562
  para as versões 3 e 5, deriva do relógio, não repetição ao reaplicar o
  nó) e `tests/api_test.go` (análise, serialização, SQL, apelidos);
  `FuzzParse`; `clock_internal_test.go` na raiz para as bordas do
  relógio que não são alcançáveis de fora do pacote.

### Alterado

- **Formato de dados em JSON e gob.** Com `MarshalText` presente, o
  `encoding/json` passou a gravar um `UUID` como a string canônica entre
  aspas. Antes ele gravava uma lista de 16 números, por ser um vetor de
  bytes. O mesmo vale para `encoding/gob`, que passa a usar
  `MarshalBinary`. A API é aditiva; os dados já gravados no formato
  antigo precisam ser convertidos. Para quem vem do pacote do Google,
  nada muda.
- **Deriva de relógio nas versões 1, 2 e 6.** Em rajadas mais rápidas
  que o tique de 100 ns, o relógio interno avança um tique por geração e
  o instante embutido fica à frente do relógio do sistema. A RFC 9562
  permite; difere do pacote do Google, que mantém o instante e
  incrementa a sequência.

### Documentação

- `PROMPT.md` foi reescrito como especificação completa de reconstrução
  e em seguida convertido em `docs/SPEC.md`, deixando a raiz só com os
  arquivos de produção. `docs/MIGRATION.md` criado. `STARTHERE.md`,
  `README.md` e `docs/DEPLOY-FULL.md` ampliados para a nova API.

### Decisões

- Não foram trazidos do pacote do Google: `SetRand` (trocaria a entropia
  do gerador padrão em tempo de execução, exigindo uma leitura atômica
  no caminho quente; use `NewGeneratorWithReader` ou
  `NewCryptoGenerator`), `EnableRandPool` e `DisableRandPool` (o gerador
  padrão já mantém um pool por thread), `SetNodeInterface` e
  `NodeInterface` (arrastariam o pacote `net`).
- `String()` e `encodeHex` duplicam as mesmas oito linhas de propósito:
  `String()` é caminho quente e não deve pagar uma chamada por causa dos
  serializadores.

---

## [v0.2.0] — 2026-08-27

Passagem de correção a partir de três relatórios de revisão sobre a
`v0.1.0`. Nenhuma correção alterou o layout de bits, a semântica dos
níveis ou o formato de saída.

### Corrigido

- **Pânico de índice fora de faixa em `FromString`** (crítico). O laço
  pulava qualquer hífen encontrado, então um hífen extra em
  deslocamento par do último grupo (24, 26, 28, 30, 32 ou 34) fazia ler
  `s[36]` e derrubava o processo com entrada externa de 36 caracteres.
  A decodificação passou a usar a tabela fixa `hexOffsets`, que
  elimina a leitura fora dos limites por construção e rejeita hífens
  fora do lugar. O desempenho melhorou 22%, com zero alocações
  preservadas. Travado por `TestFromStringNeverPanics` (todas as
  36 × 256 mutações de um byte), `TestFromStringExtraHyphen` e
  `FuzzFromString`.
- **Relógio anterior a 1970 corrompia o UUID em silêncio.** O operador
  `%` preserva o sinal do dividendo, então os campos sub-milissegundo
  ficavam negativos e sofriam wrap-around ao virar `uint16`. O
  timestamp passou a ter piso na própria época Unix, com os campos
  sub-milissegundo zerados.
- **Limite real de data era 2262, não 10889.** O comentário prometia
  10889 (correto para o campo de 48 bits), mas a leitura por
  `UnixNano()` satura em 2262-04-11. A leitura passou a ser
  `Unix()` mais `Nanosecond()`, que não satura, e a decomposição foi
  isolada em `splitUnixInstant`.
- **`Generator` zerado e `NewGeneratorWith(nil)` causavam pânico.**
  `NewGeneratorWith(nil)` passou a entrar em pânico na construção, com
  mensagem clara; `Generate` recorre à entropia do gerador padrão quando
  o receptor não tem fonte (valor zero embutido em outra struct, ou
  ponteiro nulo).
- **Corrida de dados na suíte de testes.** `TestMassConcurrent` e o
  benchmark paralelo escreviam em um sink global a partir de várias
  goroutines. Cada goroutine passou a publicar em posição própria, e o
  benchmark usa `runtime.KeepAlive`. A biblioteca em si estava limpa.
- **`TestMonotonicity` falhava permanentemente** em hosts com relógio
  de microssegundo, como o macOS: contava regressões em laço apertado,
  medindo o relógio do host e não a biblioteca. Reescrito com pausa
  entre gerações; a invariante independente do relógio está em
  `TestOrderingFollowsEmbeddedTime` e `TestTieRateReport` mede a taxa
  de empates como diagnóstico.
- **Benchmarks sem aquecimento** inflavam o primeiro cenário, sempre o
  Nível 1, em cerca de 60%; e `TestMassOneMillion` calculava a taxa com
  `dur.Milliseconds()+1`. Passagem de aquecimento acrescentada e taxa
  em nanossegundos; tabelas remedidas.
- **`%x` em `uuid.UUID` imprime o hexadecimal da string**, não os 16
  bytes, porque o tipo satisfaz `fmt.Stringer`. Mensagens de teste
  corrigidas e armadilha documentada em `String()`.
- Links quebrados e árvore desatualizada no `STARTHERE.md`; trechos não
  compiláveis no `README.md` e no `docs/DEPLOY-FULL.md`; contradição
  nas faixas documentadas dos campos do tipo `Time`.
- `.DS_Store` removido do índice do Git e `.gitignore` criado.

### Alterado

- **Uma palavra de entropia nos níveis 2 e 3.** `Generate` sorteava duas
  palavras de 64 bits sempre, mas `rand_a` só é aleatório no Nível 1.
  O `Generator` passou a ter as fontes `oneWord` e `twoWords`, e os
  níveis 2 e 3 consomem uma única palavra. Ganho medido de 7% serial e
  18% em paralelo; importa também para quem fornece `crypto/rand` por
  `NewGeneratorWith`. Travado por `TestEntropyDrawsPerLevel`.

### Adicionado

- Suíte de testes em `tests/`: `parsing_test.go`, `fuzz_test.go`,
  `layout_test.go` (layout de bits com entropia determinística),
  `import_test.go`, `ordering_test.go` (ordenação, unicidade em
  1.000.000 por nível, concorrência sob `-race`), `alloc_test.go`
  (trava de zero alocações) e `robustness_test.go` (bordas do
  `Generator`, consumo de entropia, níveis desconhecidos, atalhos de
  pacote); `BenchmarkFromString` e `BenchmarkImportBinary`.
- **Aviso de segurança.** O gerador padrão usa PCG, um PRNG estatístico
  e não criptográfico: quem observar alguns UUIDs consegue prever os
  seguintes, e todo UUIDv7 expõe o instante de criação. Advertência no
  comentário de `NewGenerator`, no `README.md`, no `docs/DEPLOY-FULL.md`
  e no `STARTHERE.md`, com o exemplo de gerador com `crypto/rand`.

### Documentação

- `docs/SPEC.md`: a promessa de ordenação passou a ser precisa
  (cronológica na resolução do nível, com desempate aleatório dentro do
  mesmo instante, e não monotonicidade estrita); casos de teste
  obrigatórios reescritos; requisitos de robustez do relógio, do
  analisador e da fonte de entropia. `docs/DEPLOY-FAST.md` com exemplos
  dos três níveis. `docs/git.md` com notas de release.

### Decisões

- **Não subir o `go.mod` para 1.23.** `math/rand/v2` existe desde o
  1.22, o código não usa API posterior e o campo declara a versão
  mínima suportada; subir o piso reduziria a base de usuários sem
  ganho.
- **Não usar `unsafe.String` em `String()`.** O buffer de 36 bytes vive
  na pilha; devolver uma string que aponta para ele é comportamento
  indefinido, e `String()` é API pública.
- **Não adotar contador monotônico por padrão.** O protótipo zera as
  regressões de ordenação, mas custa 32 vezes mais em paralelo e
  destrói a escalabilidade que é o principal atrativo. Fica registrado
  como proposta opcional (ver abaixo).

---

## [v0.1.0] — 2026-06-05

Primeira versão publicada.

### Adicionado

- Geração de **UUIDv7** (RFC 9562) com três níveis de precisão temporal:
  `Level1` (milissegundos, UUIDv7 padrão), `Level2` (microssegundos em
  `rand_a`) e `Level3` (microssegundos em `rand_a` e nanossegundos nos
  10 bits altos de `rand_b`). Versão e variante preservadas em todos os
  níveis; ordenação lexicográfica cronológica na resolução do nível.
- Tipos `Level`, `UUID` (`[16]byte`), `Generator` e `Time`.
- `NewGenerator` com pool de PRNGs PCG por thread (`sync.Pool`), cada um
  semeado uma vez de `crypto/rand`, sem trava compartilhada;
  `NewGeneratorWith` para entropia personalizada; atalhos de pacote
  `Generate` e `GenerateString`.
- Conversão `String` e `FromString`, com os apelidos `BinaryToString` e
  `StringToBinary`; importação das propriedades de tempo por `Import` e
  `ImportBinary`, cega quanto ao nível.
- Geração binária sem alocações e `String` com uma única alocação.
- Documentação: `README.md`, `STARTHERE.md`, `docs/DEPLOY-FAST.md`,
  `docs/DEPLOY-FULL.md`, `docs/SPEC.md`, `docs/TEST-AND-BENCHMARK.md`;
  testes funcionais, benchmarks e o executável
  `tests/benchmark-bulk`.

---

## Propostas em aberto

Itens levantados nas revisões de 2026-08-27, ainda sem decisão. Nenhum
é defeito; todos mudam projeto ou ampliam a superfície pública. As
medições citadas foram feitas em Apple M2, Go 1.27, e não foram
refeitas depois.

- **Trocar o `sync.Pool` de PCG pelo gerador global de `math/rand/v2`.**
  Desde o Go 1.22 as funções de pacote usam `runtime.rand()`, por thread
  e sem trava; a medição isolada da fonte de entropia deu 4,49 ns contra
  7,85 ns em série e 0,91 ns contra 2,75 ns em paralelo. O gerador do
  runtime é ChaCha8 semeado pelo sistema, o que também reduziria a
  previsibilidade do gerador padrão, e eliminaria as releituras de
  `crypto/rand` quando o GC esvazia o pool. É decisão de arquitetura: o
  desenho atual está descrito em `CLAUDE.md` como característica do
  projeto e precisa de benchmark próprio antes de mudar.
- **Gerador monotônico opcional** (`NewMonotonicGenerator`), com contador
  de 16 bits no topo dos bits aleatórios do Nível 3 e estado
  compartilhado atômico, conforme o método 1 da RFC 9562 §6.2. Zera as
  regressões de ordenação dentro do mesmo instante ao custo de 8,5% em
  série e 32 vezes em paralelo. Só faz sentido como construtor
  dedicado, nunca como padrão.
- **Relógio injetável no `Generator`** para vetores dourados de tempo.
  A motivação original, a falta de teste para o relógio pré-1970, já
  foi resolvida pelo teste interno de `splitUnixInstant` na raiz; resta
  apenas o valor de testar `Generate` de ponta a ponta com instante
  fixo.
- **Adições de API** ainda não feitas: `AppendTo(dst []byte) []byte`
  (escrita no buffer do chamador, medida 29% mais rápida que `String` e
  sem alocação), `Bytes() []byte` e `IsValid() bool`. As demais da
  lista original (`MarshalText`, JSON, `database/sql`, `Compare`,
  `FromBytes`, `IsZero`, `TimestampWithLevel`) entraram na `v0.3.0`.

[Não publicado]: https://github.com/patrickbrandao/go-loghub-uuid/compare/v0.3.0...HEAD
[v0.3.0]: https://github.com/patrickbrandao/go-loghub-uuid/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/patrickbrandao/go-loghub-uuid/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/patrickbrandao/go-loghub-uuid/releases/tag/v0.1.0
