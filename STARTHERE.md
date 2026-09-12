# START HERE — Mapa da biblioteca go-loghub-uuid

Ponto de partida para entender o projeto inteiro: estrutura, API, layout
de bits, níveis e onde encontrar cada coisa.

---

## 1. O que é

Biblioteca Go para gerar **UUIDv7** (RFC 9562) com **três níveis** de
precisão temporal, converter entre string e binário e importar as
propriedades de tempo. Gera também **todas as demais versões** da RFC
9562 — 1, 2, 3, 4, 5, 6 e 8 — e traz análise permissiva de texto,
serialização em JSON e integração com `database/sql`.

Sem dependências externas; segura para concorrência; otimizada para alto
throughput. O caminho do UUIDv7 continua sem trava e sem alocações: tudo
que foi acrescentado vive em arquivos próprios e não o atravessa.

---

## 2. Estrutura de arquivos

```
go-loghub-uuid/
│
├── go.mod                  # módulo: github.com/patrickbrandao/go-loghub-uuid
├── .github/workflows/ci.yml # integração contínua (lint, vet, build, testes, cobertura, Windows/macOS, fuzzing semanal)
├── .golangci.yml           # configuração do linter (golangci-lint v2), a mesma do CI
│
│   # PRODUÇÃO — núcleo do UUIDv7 (caminho quente, sem trava, sem alocação)
├── uuid.go                 # tipos, Generator, geração por nível
├── conversion.go           # String()/FromString + apelidos de conversão
├── import.go               # Time, Import, ImportBinary
│
│   # PRODUÇÃO — demais versões de UUID
├── clock.go                # relógio gregoriano, sequência e nó (versões 1, 2, 6)
├── version1.go             # GenerateV1 e GenerateV6
├── version2.go             # Domain, GenerateV2 e atalhos DCE
├── namebased.go            # espaços de nomes, GenerateV3 e GenerateV5
├── version4.go             # GenerateV4
├── version8.go             # GenerateV8
│
│   # PRODUÇÃO — API de apoio
├── construct.go            # GenerateAt: UUIDv7 de um instante informado, e o empacotamento compartilhado
├── bounds.go               # MinAt, MaxAt e RangeAt: fronteiras para consulta por intervalo
├── parse.go                # Parse permissivo, Validate, FromBytes, MustParse
├── values.go               # Nil, Max, Compare, URN, Bytes, IsValid, UUIDs
├── encoding.go             # AppendTo/AppendBinary, MarshalText/Binary e as leituras
├── sql.go                  # Scan, Value e NullUUID
├── inspect.go              # Timestamp, GregorianTime, ClockSequence, NodeID
├── entropy.go              # NewGeneratorWithReader e NewCryptoGenerator
├── compat.go               # apelidos com os nomes do pacote google/uuid
├── clock_internal_test.go  # teste interno das funções puras de relógio
├── example_test.go         # funções Example para o pkg.go.dev (pacote externo loghubuuid_test)
│
├── README.md               # descrição rápida + uso rápido
├── STARTHERE.md            # este mapa
├── CHANGELOG.md            # histórico de mudanças e decisões por versão
├── LICENSE                 # MIT
├── SECURITY.md             # como relatar vulnerabilidade e o modelo de ameaça documentado
├── CONTRIBUTING.md         # convenções e verificação local para quem contribui
├── CLAUDE.md               # instruções de manutenção (ferramental)
│
├── docs/                   # documentação de uso
│   ├── DEPLOY-FAST.md
│   ├── DEPLOY-FULL.md
│   ├── MIGRATION.md        # vindo do pacote github.com/google/uuid
│   ├── TEST-AND-BENCHMARK.md
│   ├── SPEC.md             # reimplementar do zero; §11 = decisões firmadas
│   └── RELEASE.md          # procedimento de publicação de versão
│
└── tests/                  # tudo que NÃO vai para produção
    ├── doc.go
    ├── generation_test.go      # testes funcionais
    ├── versions_test.go        # versões 1 a 8, tempo, nó e sequência
    ├── api_test.go             # análise, serialização, SQL e apelidos
    ├── parsing_test.go         # robustez de FromString/String
    ├── layout_test.go          # layout de bits com entropia determinística
    ├── import_test.go          # extração das propriedades de tempo
    ├── ordering_test.go        # ordenação, unicidade e concorrência
    ├── robustness_test.go      # bordas do Generator e consumo de entropia
    ├── alloc_test.go           # trava de zero alocações
    ├── bounds_test.go          # fronteiras de tempo: MinAt, MaxAt e RangeAt
    ├── construct_test.go       # geração por instante explícito: GenerateAt
    ├── binary_sql_test.go      # BinaryUUID e NullBinaryUUID
    ├── golden_test.go          # vetores dourados: extensão multinível e versões 1, 2 e 6
    ├── binary_serialization_test.go # serialização JSON, texto e binário dos tipos binários
    ├── clockstate_test.go      # isolamento do nó e da sequência entre testes
    ├── fuzz_test.go            # FuzzFromString, FuzzParse, FuzzNullUUIDJSON,
    │                            # FuzzInstantArithmetic e FuzzGregorianUnixTime
    ├── benchmark_test.go       # benchmarks + massa de 1.000.000
    ├── benchmark-bulk/
    │   └── main.go             # executável: go run ./tests/benchmark-bulk
    └── compare/                # módulo aninhado (go.mod próprio): comparação com github.com/google/uuid
        ├── clockfloor_test.go  # piso de relógio por sequência, lado a lado
        └── golden_test.go      # vetores v1/v2 lidos pelo outro pacote; layout v6 dele medido
```

A **raiz** contém apenas o necessário para usar a biblioteca em produção
(os arquivos `.go`, o `go.mod`, README/STARTHERE/CHANGELOG/LICENSE) mais
os arquivos que o GitHub ou o ferramental exigem nesse local: `SECURITY.md`
e `CONTRIBUTING.md` (abas de segurança e de contribuição), `CLAUDE.md`
(instruções de manutenção), `.golangci.yml` (configuração do linter) e a
integração contínua em `.github/`. Dois arquivos de teste também ficam na
raiz por necessidade: `clock_internal_test.go`, que exercita funções
internas do relógio, e `example_test.go`, cujas funções `Example` só
aparecem na documentação do pacote se estiverem no mesmo diretório.
Documentação, especificação, relatórios e testes ficam em pastas
próprias.

---

## 3. API pública (pacote `loghubuuid`)

### 3.1 Núcleo do UUIDv7

**Tipos**
- `Level` — `Level1`, `Level2`, `Level3`.
- `UUID` — `[16]byte`, binário de 128 bits.
- `Generator` — objeto criado no boot, seguro para concorrência.
- `Time` — `Seconds`, `Milliseconds`, `Microseconds`, `Nanoseconds`.

**Construtores**
- `NewGenerator() *Generator` — padrão rápido (ChaCha8 do runtime, sem lock).
- `NewGeneratorWith(source func() uint64) *Generator` — entropia
  personalizada; `source` precisa ser segura para concorrência e entra em
  pânico se for `nil`.
- `NewGeneratorWithReader(r io.Reader) *Generator` — entropia a partir de
  um `io.Reader` seguro para concorrência.
- `NewCryptoGenerator() *Generator` — entropia de `crypto/rand`.

> O gerador padrão usa o ChaCha8 do runtime do Go, que resiste a
> predição, mas a recomendação para segredos continua sendo
> `crypto/rand`, e o UUIDv7 expõe o instante de criação de qualquer
> forma. Ver a seção "Aviso de segurança" do [README.md](README.md).

**Geração**
- `(*Generator) Generate(Level) UUID`
- `(*Generator) GenerateString(Level) string`
- `Generate(Level) UUID` — atalho de pacote (gerador padrão interno)
- `GenerateString(Level) string` — atalho de pacote

**Conversão**
- `(UUID) String() string`
- `(UUID) AppendTo(dst []byte) []byte` — escreve os 36 bytes no buffer do
  chamador; sem alocação quando há capacidade.
- `(UUID) AppendBinary(dst []byte) ([]byte, error)` — o mesmo com os 16
  bytes crus; o erro é sempre nulo.
- `FromString(string) (UUID, error)`
- `BinaryToString(UUID) string` — apelido de `String()`
- `StringToBinary(string) (UUID, error)` — apelido de `FromString`

**Importação**
- `Import(string) (Time, error)`
- `ImportBinary(UUID) Time`

**Construção a partir de um instante**
- `GenerateAt(Level, time.Time) UUID` — um UUIDv7 daquele instante, com
  os bits livres **sorteados**. Também como
  `(*Generator) GenerateAt`, para valer a entropia configurada.
- `GenerateAtString(Level, time.Time) string` — o mesmo, em texto.
- `MinAt(Level, time.Time) UUID` — o menor UUIDv7 gerável naquele
  instante e naquele nível; bits livres de entropia em zero.
- `MaxAt(Level, time.Time) UUID` — o maior; bits livres em um.
- `RangeAt(Level, from, to time.Time) (lo, hi UUID)` — o par de um
  intervalo **semiaberto** `[from, to)`, pronto para
  `WHERE id >= lo AND id < hi`.

> `GenerateAt` é gerador: duas chamadas com o mesmo instante devolvem
> valores diferentes. `MinAt` e `MaxAt` são determinísticas e servem de
> fronteira, não de identificador.
>
> Todas preservam versão 7 e variante RFC. A fronteira só vale para
> UUIDs do **mesmo nível**: os bits abaixo do milissegundo significam
> coisas diferentes em cada um. Ver
> [docs/DEPLOY-FULL.md](docs/DEPLOY-FULL.md).

### 3.2 Demais versões de UUID

- `GenerateV1() UUID` — tempo gregoriano, sequência e nó.
- `GenerateV2(Domain, uint32) UUID` — DCE Security.
- `GenerateV2Person() UUID`, `GenerateV2Group() UUID` — atalhos que usam
  o usuário e o grupo do processo.
- `GenerateV3(space UUID, name []byte) UUID` — MD5, determinística.
- `(*Generator) GenerateV4() UUID` e `GenerateV4() UUID` — aleatória.
- `GenerateV5(space UUID, name []byte) UUID` — SHA-1, determinística.
- `GenerateHash(h hash.Hash, space UUID, name []byte, version byte) UUID`
- `GenerateV6() UUID` — versão 1 com tempo reordenado, ordenável.
- `GenerateV8(data [16]byte) UUID`, `(*Generator) GenerateV8() UUID`,
  `GenerateV8Random() UUID` — 122 bits livres.

**Tipos e estado de apoio**
- `Domain` — `Person`, `Group`, `Org`.
- `NameSpaceDNS`, `NameSpaceURL`, `NameSpaceOID`, `NameSpaceX500`.
- `NodeID() []byte`, `SetNodeID([]byte) bool`.
- `ClockSequence() int`, `SetClockSequence(int)`.
- `GetTime() (GregorianTime, uint16)` — o `uint16` traz os 14 bits da
  sequência com os 2 bits de variante já posicionados (`0x8000`).

As versões 1, 2 e 6 compartilham um relógio interno protegido por trava
própria, independente do caminho do UUIDv7.

### 3.3 Análise de texto

- `Parse(string) (UUID, error)` — aceita a forma canônica, entre chaves,
  com prefixo `urn:uuid:` e hexadecimal cru de 32 dígitos.
- `ParseBytes([]byte) (UUID, error)` — o mesmo, sem alocar.
- `MustParse(string) UUID`, `Must(UUID, error) UUID`.
- `Validate(string) error`.
- `FromBytes([]byte) (UUID, error)` — 16 bytes crus.

`FromString` **não mudou**: continua aceitando somente a forma canônica.

### 3.4 Serialização e banco de dados

- `(UUID) MarshalText`, `(*UUID) UnmarshalText`
- `(UUID) AppendText(dst []byte) ([]byte, error)` — `encoding.TextAppender`
  do Go 1.24; o erro é sempre nulo.
- `(UUID) MarshalBinary`, `(*UUID) UnmarshalBinary`
- `(UUID) AppendBinary(dst []byte) ([]byte, error)` —
  `encoding.BinaryAppender` do Go 1.24, o par binário de `AppendText`;
  mesmo conteúdo de `MarshalBinary`, sem alocar quando `dst` tem
  capacidade.
- `(*UUID) Scan(any) error`, `(UUID) Value() (driver.Value, error)`
- `NullUUID` — coluna que aceita `NULL`, com `Scan`, `Value` e as
  serializações em JSON, texto e binário.
- `BinaryUUID` — mesmo UUID, gravado como 16 bytes crus em vez da string
  canônica. Para `BINARY(16)` e `BLOB`, onde não há tipo nativo de UUID.
  A conversão é no ponto da consulta: `uuid.BinaryUUID(u)`. Tem
  `MarshalText`/`UnmarshalText` e `MarshalBinary`/`UnmarshalBinary`,
  delegando para `UUID`, para que `encoding/json` grave a string
  canônica e não um vetor de 16 números.
- `NullBinaryUUID` — o mesmo, para coluna que também aceita `NULL`, com
  `Scan`, `Value` e as serializações em JSON, texto e binário,
  espelhando `NullUUID`.

> `Value` de `UUID` continua gravando texto e não vai mudar: misturar os
> dois formatos na mesma coluna faria as linhas antigas sumirem das
> consultas. Cuidado ainda com a distinção entre coluna nula e UUID
> nulo, que viram a mesma linha se forem confundidos.

> **Mudança de formato de dados.** Com `MarshalText` presente, o
> `encoding/json` passa a gravar um UUID como string canônica. Antes ele
> gravava uma lista de 16 números, por ser um vetor de bytes. O mesmo
> vale para `encoding/gob`.

### 3.5 Inspeção

- `(UUID) Version() byte` — o número da versão.
- `(UUID) Variant() byte` — 2 (binário `10`) para a variante RFC.
- `(UUID) Timestamp() (time.Time, bool)` — versões 1, 6 e 7.
- `(UUID) TimestampWithLevel(Level) (time.Time, bool)` — recupera a
  precisão sub-milissegundo dos níveis 2 e 3.
- `(UUID) GregorianTime() (GregorianTime, bool)` — versões 1 e 6.
- `(UUID) ClockSequence() (int, bool)`, `(UUID) NodeID() []byte`
- `(UUID) Domain() (Domain, bool)`, `(UUID) ID() (uint32, bool)`
- `(UUID) IsZero() bool`, `(UUID) IsMax() bool`
- `(UUID) IsValid() bool` — variante RFC e versão de 1 a 8; `Nil` e `Max`
  contam como válidos.
- `(UUID) Bytes() []byte` — cópia dos 16 bytes.
- `(UUID) Compare(UUID) int`, `(UUID) URN() string`
- `Nil`, `Max`, `UUIDs` com `Strings() []string`
- `VersionString(byte) string`, `VariantString(byte) string`

### 3.6 Compatibilidade com github.com/google/uuid

`compat.go` reproduz os nomes e as assinaturas do pacote do Google:
`New`, `NewString`, `NewRandom`, `NewRandomFromReader`, `NewUUID`,
`NewV6`, `NewV7`, `NewV7FromReader`, `NewMD5`, `NewSHA1`, `NewHash`,
`NewDCESecurity`, `NewDCEPerson`, `NewDCEGroup`. Veja
[docs/MIGRATION.md](docs/MIGRATION.md).

### 3.7 Erros

- `ErrInvalidFormat` — string de UUID em formato inválido.
- `ErrInvalidLength` — comprimento incompatível; embrulha o anterior.
- `ErrInvalidBrackets` — forma entre chaves malformada; idem.
- `ErrInvalidScanType` — tipo não suportado em `Scan`.
- `ErrEntropySource` — falha ao ler da fonte de entropia do chamador.
- `IsInvalidLengthError(error) bool`.

Todos os erros de formato continuam reconhecíveis por
`errors.Is(err, ErrInvalidFormat)`.

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
  [docs/SPEC.md](docs/SPEC.md)
- **Quero propor uma mudança de projeto, ou vou auditar a biblioteca**:
  [docs/SPEC.md](docs/SPEC.md) seção 11, o registro de decisões firmadas
  — o que já foi decidido, por quê, e o que justificaria rever.
- **Quero ler o código**: comece por `uuid.go` (geração), depois
  `conversion.go` e `import.go`.
- **Quero as outras versões**: `clock.go` primeiro (o relógio
  compartilhado), depois `version1.go`.
- **Venho do pacote google/uuid**: [docs/MIGRATION.md](docs/MIGRATION.md),
  e `tests/compare/` para a diferença de comportamento provada em código.
  É um módulo aninhado, com `go.mod` próprio: a raiz continua sem
  nenhuma dependência.
- **Vou publicar uma versão**: [docs/RELEASE.md](docs/RELEASE.md)
- **Vou contribuir ou relatar um problema de segurança**:
  [CONTRIBUTING.md](CONTRIBUTING.md) e [SECURITY.md](SECURITY.md)

---

## 6. Comandos úteis

```bash
go get github.com/patrickbrandao/go-loghub-uuid   # instalar
go build ./...                                     # compilar
go vet ./...                                        # análise estática
go test ./tests/ -v                                 # testes
go test ./tests/ -run '^$' -bench Benchmark -benchmem   # benchmarks
go run ./tests/benchmark-bulk                       # 1.000.000 por nível
golangci-lint run ./...                             # linter (configuração em .golangci.yml)
go test ./ -run Example -v                          # exemplos executáveis
```

---

## 7. Desempenho de referência

Em VM modesta (Xeon 2.80 GHz): geração binária ~85–98 ns/UUID com **zero
alocações** e mais de **11 mil UUIDs/ms** por núcleo; string ~165 ns com
1 alocação de 48 bytes. Detalhes e tabelas em
[docs/TEST-AND-BENCHMARK.md](docs/TEST-AND-BENCHMARK.md).
