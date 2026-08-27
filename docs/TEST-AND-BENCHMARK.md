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

Modo rápido (pula os testes de massa de 1 milhão):

```bash
go test ./tests/ -short -v
```

---

## 2. Benchmarks (estilo `go test -bench`)

```bash
go test ./tests/ -run '^$' -bench Benchmark -benchmem
```

Mede nanossegundos por operação e alocações:

- `BenchmarkGenerateLevel1/2/3` — geração binária por nível.
- `BenchmarkGenerateStringLevel1/3` — geração com serialização em string.
- `BenchmarkGenerateLevel3Parallel` — throughput com várias goroutines.

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

| Benchmark                       | ns/op | alloc/op | bytes/op |
|---------------------------------|------:|---------:|---------:|
| `GenerateLevel1` (binário)      | ~43,5 | 0        | 0        |
| `GenerateLevel2` (binário)      | ~41,1 | 0        | 0        |
| `GenerateLevel3` (binário)      | ~42,0 | 0        | 0        |
| `GenerateStringLevel1`          | ~67,9 | 1        | 48       |
| `GenerateStringLevel3`          | ~64,4 | 1        | 48       |
| `GenerateLevel3Parallel`        | ~9,7  | 0        | 0        |
| `FromString`                    | ~31,6 | 0        | 0        |

Geração em massa de 1.000.000 (`go run ./tests/benchmark-bulk`):

| Cenário            | Tempo total | ns/UUID | UUIDs/ms |
|--------------------|------------:|--------:|---------:|
| Nível 1 (binário)  | ~52 ms      | ~52     | ~19.100  |
| Nível 2 (binário)  | ~41 ms      | ~41     | ~24.500  |
| Nível 3 (binário)  | ~41 ms      | ~41     | ~24.300  |
| Nível 1 (string)   | ~68 ms      | ~68     | ~14.600  |
| Nível 2 (string)   | ~65 ms      | ~65     | ~15.400  |
| Nível 3 (string)   | ~69 ms      | ~69     | ~14.400  |

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

| Cenário            | Tempo total | ns/UUID | UUIDs/ms |
|--------------------|------------:|--------:|---------:|
| Nível 1 (binário)  | ~90 ms      | ~90     | ~11.100  |
| Nível 2 (binário)  | ~88 ms      | ~88     | ~11.330  |
| Nível 3 (binário)  | ~88 ms      | ~88     | ~11.370  |
| Nível 1 (string)   | ~165 ms     | ~165    | ~6.060   |
| Nível 2 (string)   | ~163 ms     | ~163    | ~6.120   |
| Nível 3 (string)   | ~165 ms     | ~165    | ~6.045   |

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
