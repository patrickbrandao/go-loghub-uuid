# Testes e Benchmark

Como rodar a suíte de testes, os benchmarks e a geração em massa de até
1.000.000 de UUIDv7 por nível, medindo o tempo consumido.

Todos os arquivos de teste ficam na pasta `tests/` (fora da raiz de
produção) e importam a biblioteca pelo seu caminho público, como um
consumidor externo faria.

---

## 1. Testes funcionais

```bash
go test ./tests/ -v
```

Cobrem versão/variante, round-trip de conversão, faixas de micro/nano,
coerência da importação, rejeição de strings inválidas, ordenação e
concorrência.

Cobrem também as demais versões de UUID: os bits de versão e variante de
cada uma, os vetores de teste da RFC 9562 para as versões 3 e 5, a
ordenação da versão 6, o adiantamento do relógio compartilhado, a análise
permissiva de texto nos quatro formatos, a serialização em JSON e
binário, a integração com `database/sql` e os apelidos de
compatibilidade.

Para rodar apenas um desses grupos:

```bash
go test ./tests/ -run 'TestAllVersions|TestNameBased|TestTimestamp' -v   # versoes
go test ./tests/ -run 'TestParse|TestJSON|TestSQL|TestCompatibility' -v  # API de apoio
```

Modo rápido (pula os testes de massa de 1 milhão):

```bash
go test ./tests/ -short -v
```

### Detector de corrida (race detector)

A biblioteca é concorrente por projeto. A suíte inteira deve passar sob
o detector de corrida sem registrar alertas:

```bash
go test ./... -race
```

As travas de alocação passam com e sem o detector. Até a `v0.3.0` três
delas eram puladas sob `-race`, porque o `sync.Pool` do gerador padrão
descartava itens de propósito com o detector ativo e as realocações
entravam na conta; com o pool removido em favor do gerador do runtime,
não há mais estado a recriar. O passo dedicado, sem detector, continua
sendo a medição de referência:

```bash
go test ./tests/ -short -run 'Allocations|SingleAllocation' -v
```

### Fuzzing

São cinco alvos, em dois grupos. Os três primeiros varrem **texto**,
onde o risco é leitura fora dos limites; os dois últimos varrem
**aritmética de tempo**, onde o risco é saturação, estouro de sinal e
resto negativo.

```bash
# Fuzz do analisador estrito
go test ./tests/ -run '^$' -fuzz FuzzFromString -fuzztime 60s

# Fuzz do analisador permissivo (quatro formatos)
go test ./tests/ -run '^$' -fuzz FuzzParse -fuzztime 60s

# Fuzz do leitor de JSON de NullUUID (concordância com o tipo UUID)
go test ./tests/ -run '^$' -fuzz FuzzNullUUIDJSON -fuzztime 60s

# Fuzz das fronteiras e da geração por instante (seções 3.5 e 3.6)
go test ./tests/ -run '^$' -fuzz FuzzInstantArithmetic -fuzztime 60s

# Fuzz da conversão gregoriana inversa (seção 4.1)
go test ./tests/ -run '^$' -fuzz FuzzGregorianUnixTime -fuzztime 60s
```

Os dois alvos de aritmética existem porque a suíte cobre essas bordas
por tabela, com valores escolhidos à mão, e tabela não varre faixa.
`FuzzInstantArithmetic` recebe dois instantes e exige versão 7, variante
RFC, fronteira inferior nunca acima da superior, o valor gerado dentro
das fronteiras e monotonicidade quando o instante não regride, nos três
níveis mais um nível desconhecido. `FuzzGregorianUnixTime` recebe um
instante gregoriano em toda a faixa do inteiro com sinal e exige o par
canônico, com a fração entre zero e um segundo e múltipla de 100 ns: é a
metade da faixa anterior a 1970 que a divisão truncada quebraria. Ver
[04-uuidv7-formato-e-niveis.md](04-uuidv7-formato-e-niveis.md) e
[05-outras-versoes-uuid.md](05-outras-versoes-uuid.md) para as fórmulas
correspondentes.

Quando uma campanha encontra uma entrada que quebra o alvo, o Go a grava
em `tests/testdata/fuzz/<Alvo>/<hash>` (diretório ignorado pelo Git) e a
partir daí ela passa a rodar como caso de teste comum, sem `-fuzz`:

```bash
go test ./tests/ -run 'FuzzParse/<hash>' -v
```

### Exemplos executáveis

As funções `Example` de `example_test.go`, na raiz, reproduzem trechos
da documentação de uso e são compiladas e executadas pela suíte; as que
declaram `// Output:` usam só vetores fixos. Elas aparecem na página do
pacote em pkg.go.dev.

```bash
go test ./ -run Example -v     # executa
go test ./ -list Example       # lista
```

### Linter

Além do `go vet`, o projeto roda o `golangci-lint` (versão 2) com a
configuração de `.golangci.yml`, a mesma usada pelo CI. Instalação e
uso:

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
golangci-lint run ./...
```

Toda marcação `//nolint` precisa nomear o linter e trazer o motivo; o
próprio linter (`nolintlint`) recusa marcações sem motivo ou que não
silenciam nada. A regra `G115` do `gosec` (truncamento em conversão de
inteiro) está desligada de propósito: empacotar campos em bytes por
deslocamento e truncamento é o que a biblioteca faz, e o caminho quente
não ganha máscaras para calar um aviso.

### Cobertura

A suíte vive em `./tests/`, outro pacote, então o `-cover` padrão não
conta o pacote da raiz: é preciso `-coverpkg`.

```bash
go test ./tests/ ./ -short -coverpkg=github.com/patrickbrandao/go-loghub-uuid -coverprofile=cover.out
go tool cover -func=cover.out            # por função, com o total na última linha
go tool cover -html=cover.out            # abre o relatório no navegador
```

O CI publica o relatório (`cover.out` e `cover.html`) como artefato
`cobertura` do job `test` e falha se o total ficar abaixo de 95%. Os
ramos que ficam de fora são **apenas** as falhas de leitura de
`crypto/rand` (`fillRandom` e os `recover` de `NewRandom` e `NewV7`),
inalcançáveis a partir do Go 1.24, e ficam de fora por decisão, não por
esquecimento. Não há nenhum outro bloco descoberto: se `go tool cover`
apontar um, ele é lacuna de teste e não exceção documentada.

### Integração contínua

O arquivo `.github/workflows/ci.yml` define três jobs.

**`test`**, em Linux, a cada push, pull request e tag, na versão mínima
declarada em `go.mod` (1.22) e na versão estável mais recente:

```bash
gofmt -l .                      # só na versão estável
go vet ./...
go build ./...
GOOS=windows GOARCH=amd64 go build ./... && go vet ./...   # idem para darwin/arm64 e linux/arm64
golangci-lint run ./...         # só na versão estável
go test ./... -race -short
go test ./tests/ -short -run 'Allocations|SingleAllocation' -v   # travas de alocação, sem -race
go test ./tests/ -run '^$' -bench 'BenchmarkGenerateLevel|BenchmarkFromString' -benchmem -benchtime 200000x
go test ./tests/ ./ -short -coverpkg=... -coverprofile=cover.out   # só na versão estável; falha abaixo de 95%
```

**`test-os`**, em `windows-latest` e `macos-latest` com a versão estável:
`go vet`, `go build` e `go test ./... -short`, sem `-race` (no Windows o
detector exige CGO e é bem mais lento; a corrida já é verificada em
Linux). Para poupar os runners mais caros, este job não roda a cada push
em `main`: só em pull request, tag, no agendamento semanal e sob demanda.
É ele que verifica a resolução do relógio e o comportamento específico de
cada sistema (por exemplo, `GenerateV2Person` no Windows).

**`deep`**, toda segunda-feira e sob demanda pela aba Actions: a suíte
completa (com os testes de massa de 1.000.000) sob o detector de corrida
e 60 segundos de fuzzing em cada um dos cinco alvos. Os cinco rodam
sempre, mesmo que um falhe, e o job falha ao final se algum tiver
falhado.

Para disparar `test-os` e `deep` manualmente:

```bash
gh workflow run ci.yml
```

Uma tag só deve ser publicada com o fluxo `test` verde no commit
correspondente; o procedimento está em
[13-processo-de-release.md](13-processo-de-release.md).

#### Reproduzir uma falha de fuzzing do CI

Quando uma campanha do job `deep` falha, a entrada que quebrou o alvo é
publicada como artefato `fuzz-corpus` do job (retenção de 30 dias), com a
mesma árvore que o Go usa localmente: `<Alvo>/<hash>`. Para reproduzir:

1. Baixe o artefato pela página da execução na aba Actions, ou pela CLI:

   ```bash
   gh run list --workflow ci.yml --limit 5          # identifique a execução
   gh run download <id> --name fuzz-corpus --dir tests/testdata/fuzz
   ```

2. Confirme que o arquivo ficou em `tests/testdata/fuzz/<Alvo>/<hash>` e
   rode só ele, como caso de teste comum:

   ```bash
   go test ./tests/ -run 'FuzzParse/<hash>' -v
   ```

3. Corrija o defeito e transforme a entrada em caso de regressão
   permanente com `f.Add(...)` na função de fuzzing correspondente em
   `tests/fuzz_test.go`; o diretório `tests/testdata/fuzz/` continua fora
   do Git.

---

## 1.1 Comparação com o pacote github.com/google/uuid

`tests/compare/` é um **módulo aninhado**, com `go.mod` próprio. Ele
existe para provar em código a diferença de comportamento que a
documentação afirma, e a dependência de terceiros fica contida ali: o
`go test ./...` da raiz não desce em módulos aninhados, e o `go.mod` da
raiz continua sem nenhum `require`, sem `go.sum`.

```bash
cd tests/compare && go test -v ./...
```

Confira a quarentena a qualquer momento, da raiz:

```bash
go list -m all
```

A saída tem de ser uma linha só, o próprio módulo.

O escopo é **comportamento, não velocidade**. Há cinco testes:

- `TestGoogleRepeatsV1OnSequenceReturn` reproduz a repetição de UUIDv1
  do outro pacote quando o chamador sai de uma sequência de relógio e
  volta. Medido em cerca de 7% das tentativas num Apple M2. O mecanismo
  é a janela do mesmo tique de 100 ns com o piso do relógio zerado, e
  **não** adiantamento acumulado — ver
  [06-relogio-e-concorrencia.md](06-relogio-e-concorrencia.md) seção
  4.2.
- `TestLoghubDoesNotRepeatV1OnSequenceReturn` submete esta biblioteca ao
  roteiro idêntico e exige zero repetições.
- `TestClockAdvanceDiffersBetweenLibraries` mede as duas escolhas de
  projeto lado a lado: o outro pacote não adianta o relógio e incrementa
  a sequência, esta adianta o relógio e mantém a sequência.
- `TestGoogleReadsGregorianGoldenVectors` submete os vetores dourados
  das versões 1 e 2
  ([09-vetores-dourados-e-apendice-rfc.md](09-vetores-dourados-e-apendice-rfc.md),
  caso 18) ao leitor do outro pacote e exige os mesmos campos: instante,
  sequência, nó, domínio e identificador. São os mesmos bytes lidos por
  dois leitores independentes.
- `TestGoogleV6LayoutDiffersFromRFC9562` mede por que a versão 6 fica
  fora dessa conferência: na v1.6.0 o outro pacote grava o carimbo de 64
  bits inteiro nos bytes 0 a 7 e sobrepõe a versão, o que não é a ordem
  de campos da RFC 9562 §5.6. Lido pela ordem da RFC, o UUIDv6 dele cai
  séculos atrás do relógio. Se esse teste falhar por o instante passar a
  bater, o pacote corrigiu o layout e a documentação precisa ser
  atualizada.

**Não há tabela de benchmark comparativo, por decisão.** Ela envelheceria
a cada versão do pacote de terceiros, exigiria medir dois pares de fonte
de entropia para não favorecer esta biblioteca, e o custo de manutenção
não se paga. A decisão está em `CHANGELOG.md`.

**O CI não roda este módulo** no fluxo rápido: ele precisa de rede, e o
lançamento de uma versão nova do pacote de terceiros não pode quebrar o
CI desta biblioteca.

---

## 2. Benchmarks (estilo `go test -bench`)

```bash
go test ./tests/ -run '^$' -bench Benchmark -benchmem
```

Mede nanossegundos por operação e alocações:

- `BenchmarkGenerateLevel1/2/3` — geração binária por nível.
- `BenchmarkGenerateStringLevel1/3` — geração com serialização em string.
- `BenchmarkGenerateLevel3Parallel` — throughput com várias goroutines.
- `BenchmarkFromString` — análise estrita no formato canônico.
- `BenchmarkImportBinary` — leitura dos campos de tempo.
- `BenchmarkGenerateV1/V4/V5/V6` — as demais versões de UUID.
- `BenchmarkGenerateV7` — o nome por versão do UUIDv7; deve medir o mesmo
  que `BenchmarkGenerateLevel1`, porque é um apelido de `Generate(Level1)`
  embutido pelo compilador.
- `BenchmarkGenerateV7Level1/2/3` — os nomes por nível; cada um deve medir
  o mesmo que `BenchmarkGenerateLevel1/2/3`, pelo mesmo motivo.
- `BenchmarkGenerateV1Parallel` — custo do lock compartilhado pelas
  versões 1, 2 e 6, em contraste com o UUIDv7, que não tem lock.
- `BenchmarkParse` — análise permissiva no formato canônico.
- `BenchmarkMinAtLevel1/3`, `BenchmarkMaxAtLevel3` e
  `BenchmarkRangeAtLevel3` — fronteiras de tempo para consulta por
  intervalo. Medem sobre um instante fixo, para não somar a leitura do
  relógio ao resultado.
- `BenchmarkGenerateAtLevel1/3` e `BenchmarkGenerateAtStringLevel3` —
  geração a partir de instante explícito. Saem mais baratas que
  `BenchmarkGenerateLevel1/3` porque não pagam a leitura do relógio.

Os números de referência de todos eles estão na seção 4.

O caminho do UUIDv7 não foi tocado pela inclusão das outras versões. Para
conferir isso em uma máquina qualquer, compare os benchmarks do UUIDv7
antes e depois de uma alteração, com o coletor de lixo desligado para
reduzir o ruído:

```bash
GOGC=off GOMAXPROCS=4 go test ./tests/ -run '^$' \
  -bench 'BenchmarkGenerateLevel3' -benchtime 5000000x -count 6
```

---

## 3. Geração em massa de 1.000.000 por nível

### Via teste com relatório

```bash
go test ./tests/ -run 'TestMassOneMillion|TestMassConcurrent' -v
```

Gera 1.000.000 de UUIDs em cada cenário (binário e string, por nível) e
registra tempo total, ns por UUID e UUIDs/ms; além de uma variante
concorrente com 256 goroutines.

### Via executável autônomo

```bash
go run ./tests/benchmark-bulk            # 1.000.000 por cenário
go run ./tests/benchmark-bulk -n 5000000 # quantidade personalizada
```

Imprime uma tabela com tempo total, ns por UUID e UUIDs/ms para cada
nível, em binário e string.

---

## 4. Resultados de referência

> **Sobre as tabelas anteriores.** Até a revisão de 2026-08-27, tanto
> `TestMassOneMillion` quanto `benchmark-bulk` mediam os cenários em
> sequência **sem passagem de aquecimento**. O primeiro cenário medido
> pagava sozinho o custo de aquecer cache de instruções e o escalonamento
> de frequência da CPU — e como o Nível 1 é sempre o primeiro, aparecia
> mais lento do que realmente era. Ambos os
> medidores passaram a descartar uma passagem de aquecimento, e o cálculo
> da taxa deixou de usar `dur.Milliseconds()+1` (que arredondava para
> baixo e ainda somava 1 ms) em favor de nanossegundos. Os números abaixo
> foram remedidos com o código corrigido.

### Host A — Apple M2, macOS, Go 1.27, GOMAXPROCS=8

Benchmark (`go test ./tests/ -run '^$' -bench Benchmark -benchmem -benchtime=2s`):

| Benchmark                  | ns/op | alloc/op | bytes/op |
| -------------------------- | ----: | -------: | -------: |
| `GenerateLevel1` (binário) | ~43,5 |        0 |        0 |
| `GenerateLevel2` (binário) | ~41,1 |        0 |        0 |
| `GenerateLevel3` (binário) | ~42,0 |        0 |        0 |
| `GenerateStringLevel1`     | ~67,9 |        1 |       48 |
| `GenerateStringLevel3`     | ~64,4 |        1 |       48 |
| `GenerateLevel3Parallel`   |  ~9,7 |        0 |        0 |
| `String` (UUID já pronto)  | ~26,3 |        1 |       48 |
| `AppendTo` (buffer reusado)| ~18,8 |        0 |        0 |
| `FromString`               | ~31,6 |        0 |        0 |

Geração em massa de 1.000.000 (`go run ./tests/benchmark-bulk`):

| Cenário           | Tempo total | ns/UUID | UUIDs/ms |
| ----------------- | ----------: | ------: | -------: |
| Nível 1 (binário) |      ~52 ms |     ~52 |  ~19.100 |
| Nível 2 (binário) |      ~41 ms |     ~41 |  ~24.500 |
| Nível 3 (binário) |      ~41 ms |     ~41 |  ~24.300 |
| Nível 1 (string)  |      ~68 ms |     ~68 |  ~14.600 |
| Nível 2 (string)  |      ~65 ms |     ~65 |  ~15.400 |
| Nível 3 (string)  |      ~69 ms |     ~69 |  ~14.400 |

Concorrente (1.000.000 de Nível 3 em 256 goroutines): ~7,8 ms,
~128.000 UUIDs/ms agregados.

**Por que o Nível 1 agora é o mais lento.** Não é mais viés de
aquecimento: é real e esperado. Nos níveis 2 e 3 o campo `rand_a` carrega
os microssegundos, então basta **uma** palavra de 64 bits do gerador
pseudoaleatório; só o Nível 1 (e os níveis desconhecidos, que se
comportam como ele) precisa de **duas**. A diferença é exatamente o custo
de um sorteio extra.

#### Demais versões e API de apoio (Host A, 2026-09-11)

Medido no mesmo host, com Go 1.27.0 e `GOMAXPROCS=8`, por
`go test ./tests/ -run '^$' -bench 'BenchmarkGenerateV|BenchmarkParse|BenchmarkImportBinary' -benchmem -benchtime 2s -count 3`;
cada linha é a mediana das três execuções.

| Benchmark                 |  ns/op | alloc/op | bytes/op |
| ------------------------- | -----: | -------: | -------: |
| `GenerateV1`              |  ~46,8 |        0 |        0 |
| `GenerateV6`              |  ~46,7 |        0 |        0 |
| `GenerateV1Parallel`      | ~148,1 |        0 |        0 |
| `GenerateV4`              |  ~13,5 |        0 |        0 |
| `GenerateV5`              | ~116,5 |        3 |      152 |
| `Parse` (canônico)        |  ~36,5 |        0 |        0 |
| `ImportBinary`            |   ~1,7 |        0 |        0 |

Para comparar com o UUIDv7 na mesma sessão: `GenerateLevel1` ~44,8 ns,
`GenerateLevel3` ~42,9 ns e `GenerateLevel3Parallel` ~10,8 ns, todos sem
alocação.

Leitura dos números:

- **As versões 1 e 6 custam o mesmo que o UUIDv7 em série** (a diferença
  de 2 a 4 ns é a trava e o avanço do relógio interno) e também não
  alocam. A diferença aparece **em paralelo**: `GenerateV1Parallel` fica
  em ~148 ns por UUID com 8 goroutines, contra ~10,8 ns de
  `GenerateLevel3Parallel`. É o custo do mutex compartilhado pelas
  versões 1, 2 e 6, que serializa todas as goroutines; o UUIDv7 não tem
  trava e escala com os núcleos. Quem precisa de ordenação com alto
  throughput deve preferir a versão 7 (ou a 6 só em baixo volume).
- **O piso de relógio por sequência não custa nada por geração.**
  `GenerateV1` na `v0.3.0`, medida na mesma sessão e no mesmo host, deu
  ~48,3 ns em série e ~165,5 ns em paralelo; a versão atual, com o mapa
  de pisos em `clock.go`, deu ~46,8 e ~148,1. O mapa só é tocado na
  troca de sequência, nunca na geração, e os números confirmam.
- **A versão 4 é a mais barata** (~13,5 ns): não lê o relógio, só
  sorteia duas palavras e ajusta dois bytes. O custo das versões
  baseadas em tempo é majoritariamente `time.Now()`.
- **A versão 5 é a única que aloca**: três alocações e 152 bytes por
  chamada vêm do `sha1.New()` e do `Sum(nil)` da biblioteca padrão, não
  desta biblioteca. A versão 3 (MD5) tem perfil equivalente. Ambas são
  determinísticas e raramente estão em caminho quente.
- `Parse` custa ~5 ns a mais que `FromString` (~31,6 ns) pelo despacho
  entre os quatro formatos; nenhum dos dois aloca. `ImportBinary` é
  aritmética pura sobre 10 bytes.

### Host B — VM modesta, Intel Xeon @ 2.80 GHz, Go 1.22, núcleo único

Piso de referência para máquinas lentas (números da revisão anterior,
medidos sem aquecimento — leia o Nível 1 com essa ressalva):

| Cenário           | Tempo total | ns/UUID | UUIDs/ms |
| ----------------- | ----------: | ------: | -------: |
| Nível 1 (binário) |      ~90 ms |     ~90 |  ~11.100 |
| Nível 2 (binário) |      ~88 ms |     ~88 |  ~11.330 |
| Nível 3 (binário) |      ~88 ms |     ~88 |  ~11.370 |
| Nível 1 (string)  |     ~165 ms |    ~165 |   ~6.060 |
| Nível 2 (string)  |     ~163 ms |    ~163 |   ~6.120 |
| Nível 3 (string)  |     ~165 ms |    ~165 |   ~6.045 |

Leitura dos números: a geração binária custa **dezenas de
nanossegundos** e **zero alocações**; passa de **11 mil UUIDs por
milissegundo** por núcleo até na VM lenta, e de **24 mil** em CPU
moderna. A serialização em string adiciona uma alocação de 48 bytes (a
própria string). A meta de "milhares de UUIDs por milissegundo" é
atingida com folga.

**Onde está o teto.** Cerca de dois terços do custo de gerar um UUID é a
leitura do relógio (`time.Now()`), e esse custo é irredutível — a
precisão sub-milissegundo é a razão de ser da biblioteca. Micro-otimizar
a montagem dos 16 bytes não move o número.

### Troca da fonte de entropia do gerador padrão

Medição da substituição do `sync.Pool` de PRNGs PCG pelo gerador do
runtime (`math/rand/v2`, ChaCha8 por thread). Apple M2, macOS, Go 1.27,
`GOGC=off GOMAXPROCS=4`, média de seis execuções de 5.000.000 iterações
cada:

```bash
GOGC=off GOMAXPROCS=4 go test ./tests/ -run '^$' -bench 'BenchmarkGenerate' \
  -benchmem -benchtime 5000000x -count 6
```

| Benchmark                  | Pool + PCG | ChaCha8 do runtime |      Δ |
| -------------------------- | ---------: | -----------------: | -----: |
| `GenerateLevel1`           |   45,55 ns |           44,85 ns |  -1,5% |
| `GenerateLevel2`           |   41,46 ns |           39,31 ns |  -5,2% |
| `GenerateLevel3`           |   42,05 ns |           39,80 ns |  -5,4% |
| `GenerateLevel3Parallel`   |   16,93 ns |           11,06 ns | -34,7% |
| `GenerateStringLevel1`     |   69,92 ns |           66,71 ns |  -4,6% |
| `GenerateStringLevel3`     |   65,73 ns |           67,06 ns |  +2,0% |
| `GenerateV4`               |   12,36 ns |           11,65 ns |  -5,7% |
| `GenerateV1`               |   44,70 ns |           44,63 ns |  -0,2% |
| `GenerateV6`               |   44,58 ns |           44,73 ns |  +0,3% |
| `GenerateV5`               |   91,70 ns |           91,89 ns |  +0,2% |
| `GenerateV1Parallel`       |  131,40 ns |          131,45 ns |  +0,0% |

Leitura dos números:

- **O ganho real está no paralelo**: -34,7% no `Level3Parallel`. O par
  `Get`/`Put` do pool era o gargalo sob concorrência; o gerador do
  runtime não tem nenhum.
- **Em série o ganho é modesto**, 2% a 6%, porque `time.Now()` domina o
  custo. `GenerateV4`, que não lê o relógio, mostra o efeito isolado da
  fonte de entropia: -5,7%.
- **As versões 1, 2, 5 e 6 não mudam**, como esperado: nenhuma delas usa
  a fonte de entropia do `Generator`. As variações de ±0,3% são ruído, e
  o mesmo vale para o +2,0% de `GenerateStringLevel3`, que destoa do
  `GenerateLevel3` (-5,4%) logo acima.

---

## 5. Reproduzir

```bash
git clone https://github.com/patrickbrandao/go-loghub-uuid
cd go-loghub-uuid
go test ./tests/ -v
go run ./tests/benchmark-bulk
```
