# START HERE — Mapa da biblioteca go-loghub-uuid

Ponto de partida para entender o projeto inteiro: estrutura, API, layout
de bits, níveis e onde encontrar cada coisa.

---

## 1. O que é

Biblioteca Go para gerar **UUIDv7** (RFC 9562) com **três níveis** de
precisão temporal, converter entre string e binário e importar as
propriedades de tempo. Sem dependências externas; segura para
concorrência; otimizada para alto throughput.

---

## 2. Estrutura de arquivos

```
go-loghub-uuid/
│
├── go.mod                  # módulo: github.com/patrickbrandao/go-loghub-uuid
│
├── uuid.go                 # PRODUÇÃO: tipos, Generator, geração por nível
├── conversion.go           # PRODUÇÃO: String()/FromString + apelidos de conversão
├── import.go               # PRODUÇÃO: Time, Import, ImportBinary
│
├── README.md               # descrição rápida + uso rápido
├── STARTHERE.md            # este mapa
├── LICENSE                 # MIT
│
├── docs/                   # documentação de uso
│   ├── DEPLOY-FAST.md
│   ├── DEPLOY-FULL.md
│   └── TEST-AND-BENCHMARK.md
│
├── especificacao/          # como reimplementar do zero (sem código)
│   └── ESPECIFICACAO-DESENVOLVIMENTO.md
│
└── tests/                  # tudo que NÃO vai para produção
    ├── doc.go
    ├── generation_test.go      # testes funcionais
    ├── benchmark_test.go       # benchmarks + massa de 1.000.000
    └── benchmark-bulk/
        └── main.go             # executável: go run ./tests/benchmark-bulk
```

A **raiz** contém apenas o necessário para usar a biblioteca em produção
(os três `.go`, o `go.mod`, README/STARTHERE/LICENSE). Documentação,
especificação e testes ficam em pastas próprias.

---

## 3. API pública (pacote `loghubuuid`)

**Tipos**
- `Level` — `Level1`, `Level2`, `Level3`.
- `UUID` — `[16]byte`, binário de 128 bits.
- `Generator` — objeto criado no boot, seguro para concorrência.
- `Time` — `Seconds`, `Milliseconds`, `Microseconds`, `Nanoseconds`.

**Construtores**
- `NewGenerator() *Generator` — padrão rápido (pool de PCG, sem lock).
- `NewGeneratorWith(source func() uint64) *Generator` — entropia personalizada.

**Geração**
- `(*Generator) Generate(Level) UUID`
- `(*Generator) GenerateString(Level) string`
- `Generate(Level) UUID` — atalho de pacote (gerador padrão interno)
- `GenerateString(Level) string` — atalho de pacote

**Conversão**
- `(UUID) String() string`
- `FromString(string) (UUID, error)`
- `BinaryToString(UUID) string` — apelido de `String()`
- `StringToBinary(string) (UUID, error)` — apelido de `FromString`

**Importação**
- `Import(string) (Time, error)`
- `ImportBinary(UUID) Time`

**Inspeção**
- `(UUID) Version() byte` — 7
- `(UUID) Variant() byte` — 2 (binário `10`)

**Erros**
- `ErrInvalidFormat` — string de UUID em formato inválido.

---

## 4. Layout de bits (128 bits, big-endian)

```
byte:  0    1    2    3    4    5    6    7    8    9   10   11   12   13   14   15
      [-------- unix_ts_ms (48) --------][V|aa][ aa ][v|bb][----------- rand_b -----------]
                                          7  ^             10 ^
                                          versão            variante
```

- bytes 0..5 → `unix_ts_ms` (milissegundos desde epoch).
- byte 6 → nibble alto `0x7` (versão); nibble baixo = topo de `rand_a`.
- byte 7 → resto de `rand_a` (12 bits no total).
- byte 8 → 2 bits altos `10` (variante); 6 bits baixos = topo de `rand_b`.
- bytes 9..15 → resto de `rand_b` (62 bits no total).

**Onde cada nível grava o tempo sub-ms:**

| Campo            | Nível 1   | Nível 2        | Nível 3                          |
|------------------|-----------|----------------|----------------------------------|
| `rand_a` (12 b)  | aleatório | microssegundos | microssegundos                   |
| `rand_b` topo 10 | aleatório | aleatório      | nanossegundos                    |
| `rand_b` resto   | aleatório | aleatório      | aleatório (52 bits)              |

Microssegundos e nanossegundos são sempre 0..999 (fração do nível
acima). Como ocupam as posições logo após os milissegundos, a ordenação
por string continua cronológica.

---

## 5. Caminhos de leitura recomendados

- **Só quero gerar**: [docs/DEPLOY-FAST.md](docs/DEPLOY-FAST.md)
- **Quero usar tudo**: [docs/DEPLOY-FULL.md](docs/DEPLOY-FULL.md)
- **Quero medir desempenho**: [docs/TEST-AND-BENCHMARK.md](docs/TEST-AND-BENCHMARK.md)
- **Quero reimplementar em outra linguagem**:
  [especificacao/ESPECIFICACAO-DESENVOLVIMENTO.md](especificacao/ESPECIFICACAO-DESENVOLVIMENTO.md)
- **Quero ler o código**: comece por `uuid.go` (geração), depois
  `conversion.go` e `import.go`.

---

## 6. Comandos úteis

```bash
go get github.com/patrickbrandao/go-loghub-uuid   # instalar
go build ./...                                     # compilar
go vet ./...                                        # análise estática
go test ./tests/ -v                                 # testes
go test ./tests/ -run '^$' -bench Benchmark -benchmem   # benchmarks
go run ./tests/benchmark-bulk                       # 1.000.000 por nível
```

---

## 7. Desempenho de referência

Em VM modesta (Xeon 2.80 GHz): geração binária ~85–98 ns/UUID com **zero
alocações** e mais de **11 mil UUIDs/ms** por núcleo; string ~165 ns com
1 alocação de 48 bytes. Detalhes e tabelas em
[docs/TEST-AND-BENCHMARK.md](docs/TEST-AND-BENCHMARK.md).
