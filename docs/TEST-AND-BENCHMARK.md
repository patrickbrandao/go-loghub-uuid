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

Sob o detector, as travas de alocação que dependem do gerador padrão
(`TestGenerateZeroAllocations`, `TestGenerateStringSingleAllocation` e
`TestGenerateV4ZeroAllocations`) são puladas: o `sync.Pool` compilado com
`-race` descarta de propósito um em cada quatro itens devolvidos, e as
realocações resultantes seriam contadas como se fossem da biblioteca.
Meça as alocações sem o detector:

```bash
go test ./tests/ -short -run 'Allocations|SingleAllocation' -v
```

### Fuzzing

Cobrem mutações e entradas arbitrárias contra o parser:

```bash
# Fuzz do analisador estrito
go test ./tests/ -run '^$' -fuzz FuzzFromString -fuzztime 60s

# Fuzz do analisador permissivo (quatro formatos)
go test ./tests/ -run '^$' -fuzz FuzzParse -fuzztime 60s
```

### Integração contínua

O fluxo em `.github/workflows/ci.yml` executa automaticamente, a cada
push, pull request e tag, na versão mínima declarada em `go.mod` (1.22)
e na versão estável mais recente:

```bash
gofmt -l .                      # só na versão estável
go vet ./...
go build ./...
go test ./... -race -short
go test ./tests/ -short -run 'Allocations|SingleAllocation' -v   # travas de alocação, sem -race
go test ./tests/ -run '^$' -bench 'BenchmarkGenerateLevel|BenchmarkFromString' -benchmem -benchtime 200000x
```

Toda segunda-feira, e sob demanda pela aba Actions, um segundo fluxo roda
a suíte completa (com os testes de massa de 1.000.000) sob o detector de
corrida e 60 segundos de fuzzing em cada analisador. Uma tag só deve ser
publicada com o fluxo `test` verde no commit correspondente.

---

## 2. Benchmarks (estilo `go test -bench`)

```bash
go test ./tests/ -run '^$' -bench Benchmark -benchmem
```

Mede nanossegundos por operação e alocações:

- `BenchmarkGenerateLevel1/2/3` — geração binária por nível.
- `BenchmarkGenerateStringLevel1/3` — geração com serialização em string.
- `BenchmarkGenerateLevel3Parallel` — throughput com várias goroutines.
- `BenchmarkGenerateV1/V4/V5/V6` — as demais versões de UUID.
- `BenchmarkGenerateV1Parallel` — custo do lock compartilhado pelas
  versões 1, 2 e 6, em contraste com o UUIDv7, que não tem lock.
- `BenchmarkParse` — análise permissiva no formato canônico.

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
> pagava sozinho o custo de aquecer cache de instruções, escalonamento de
> frequência da CPU e preenchimento do `sync.Pool` — e como o Nível 1 é
> sempre o primeiro, aparecia mais lento do que realmente era. Ambos os
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

---

## 5. Reproduzir

```bash
git clone https://github.com/patrickbrandao/go-loghub-uuid
cd go-loghub-uuid
go test ./tests/ -v
go run ./tests/benchmark-bulk
```
