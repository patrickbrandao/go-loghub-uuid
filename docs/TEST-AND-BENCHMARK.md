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

Medições reais obtidas em uma VM modesta (Intel Xeon @ 2.80 GHz, Go
1.22, núcleo único exceto onde indicado). **Em CPUs de altíssima
velocidade os números são bem melhores** — estes servem apenas de piso.

Benchmark (`-benchmem`):

| Benchmark                       | ns/op | alloc/op | bytes/op |
|---------------------------------|------:|---------:|---------:|
| `GenerateLevel1` (binário)      | ~98   | 0        | 0        |
| `GenerateLevel2` (binário)      | ~85   | 0        | 0        |
| `GenerateLevel3` (binário)      | ~86   | 0        | 0        |
| `GenerateStringLevel1`          | ~161  | 1        | 48       |
| `GenerateStringLevel3`          | ~165  | 1        | 48       |
| `GenerateLevel3Parallel`        | ~88   | 0        | 0        |

Geração em massa de 1.000.000 (executável autônomo):

| Cenário            | Tempo total | ns/UUID | UUIDs/ms |
|--------------------|------------:|--------:|---------:|
| Nível 1 (binário)  | ~90 ms      | ~90     | ~11.100  |
| Nível 2 (binário)  | ~88 ms      | ~88     | ~11.330  |
| Nível 3 (binário)  | ~88 ms      | ~88     | ~11.370  |
| Nível 1 (string)   | ~165 ms     | ~165    | ~6.060   |
| Nível 2 (string)   | ~163 ms     | ~163    | ~6.120   |
| Nível 3 (string)   | ~165 ms     | ~165    | ~6.045   |

Concorrente (1.000.000 de Nível 3 em 256 goroutines): ~86 ms,
~11.600 UUIDs/ms agregados.

Leitura dos números: a geração binária custa **dezenas de
nanossegundos** e **zero alocações**; já passa de **11 mil UUIDs por
milissegundo** por núcleo nesta VM lenta. A serialização em string
adiciona uma alocação de 48 bytes (a própria string). A meta de
"milhares de UUIDs por milissegundo" é atingida com folga.

---

## 5. Reproduzir

```bash
git clone https://github.com/patrickbrandao/go-loghub-uuid
cd go-loghub-uuid
go test ./tests/ -v
go run ./tests/benchmark-bulk
```
