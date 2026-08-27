# Relatório de correções — go-loghub-uuid

Registro do que foi efetivamente corrigido a partir dos três relatórios de
análise deste diretório.

- **Data**: 2026-08-27
- **Commit base**: `0e1b31e`
- **Ambiente de verificação**: Apple M2, macOS (Darwin 25.6.0), Go 1.27.0,
  `GOMAXPROCS=8`
- **Relatórios de entrada**: [REPORT.md](REPORT.md),
  [REPORT-CRITIC.md](REPORT-CRITIC.md), [NEWS.md](NEWS.md)

> Observação sobre os nomes dos arquivos: o pedido mencionava
> `tasks/REPORT-NEW.md`, que não existe. A segunda análise está em
> `tasks/REPORT-CRITIC.md`, e foi essa que se usou.

---

## Sumário

| # | Item | Origem | Situação |
|---|------|--------|----------|
| 1 | Pânico de índice fora de faixa em `FromString` | REPORT BUG-1 / CRITIC BUG-1 / NEWS BUG-01 | Corrigido |
| 2 | Relógio anterior a 1970 corrompe o UUID | REPORT MEL-3 / NEWS BUG-02 | Corrigido |
| 3 | `Generator` zerado e `NewGeneratorWith(nil)` causam pânico | NEWS BUG-03 | Corrigido |
| 4 | Corrida de dados na suíte de testes | REPORT BUG-3 / NEWS BUG-04 | Corrigido |
| 5 | Limite real de data é 2262, não 10889 | NEWS BUG-05 | Corrigido na causa |
| 6 | Benchmarks sem aquecimento e taxa distorcida | NEWS BUG-06 | Corrigido |
| 7 | Links quebrados e árvore desatualizada no STARTHERE | NEWS BUG-07 | Corrigido |
| 8 | Trechos não compiláveis no README e no DEPLOY-FULL | NEWS BUG-08 | Corrigido |
| 9 | `%x` em `uuid.UUID` imprime o hex da string | NEWS BUG-09 | Corrigido |
| 10 | `.DS_Store` versionado; sem `.gitignore` | NEWS BUG-10 | Corrigido |
| 11 | `TestMonotonicity` falha permanentemente | REPORT BUG-2 / NEWS MELHORIA-01 | Corrigido |
| 12 | Duas palavras de 64 bits sorteadas onde uma basta | NEWS PERF-01 | Corrigido |
| 13 | Gerador padrão previsível, sem advertência | NEWS SEC-01 | Documentado |
| 14 | Contradição na documentação do tipo `Time` | NEWS MELHORIA-04 | Corrigido |
| 15 | Lacunas de cobertura de testes | REPORT MEL-7 | Corrigido |
| 16 | Promessa de ordenação imprecisa na especificação | NEWS MELHORIA-01 | Corrigido |
| — | `go.mod` para `go 1.23` | REPORT MEL-6 | **Descartado** |
| — | `unsafe.String` em `String()` | REPORT MEL-2 | **Descartado** |
| — | Trocar `sync.Pool`+PCG pelo gerador global | NEWS PERF-02 | Não aplicado |
| — | Adições de API (`MarshalText`, `Bytes`, `AppendTo`, …) | REPORT MEL-1/4/5 / NEWS MELHORIA-02 / PERF-04 | Não aplicado |
| — | Relógio injetável | NEWS MELHORIA-03 | Não aplicado |
| — | Contador monotônico | NEWS MELHORIA-01 (código) | Não aplicado |

---

## Correções aplicadas

### 1. Pânico de índice fora de faixa em `FromString` — crítico

**Arquivo**: `conversion.go`

O laço validava o tamanho (36) e os quatro hifens canônicos, mas depois
percorria a string pulando **qualquer** hifen encontrado, um a um. Um
hifen extra em deslocamento par do último grupo desalinhava os pares: o
índice chegava a 35, o laço entrava (`35 < 36`) e lia `s[36]`.

Como `FromString` / `StringToBinary` / `Import` são justamente as funções
que recebem dado externo (UUID vindo de requisição HTTP, linha de log,
coluna de banco), uma string de 36 caracteres com um hifen a mais
derrubava o processo.

Reprodução confirmada antes da correção, em seis deslocamentos (24, 26,
28, 30, 32 e 34):

```
FromString("0192f7c5-1a2b-7c3d-8e4f--abbccddeeff")
panic: runtime error: index out of range [36] with length 36
```

A correção decodifica a partir de deslocamentos fixos, já que as posições
dos 16 bytes são conhecidas. O maior índice lido passa a ser 35, seguro
por construção; hifens fora do lugar são rejeitados porque não são
dígitos hexadecimais; e o teste de hifen por caractere desaparece.

Verificação:

- `TestFromStringNeverPanics` (toda mutação de um byte sobre uma string
  canônica: 36 × 256 entradas) e `TestFromStringExtraHyphen` passam;
- `FuzzFromString` executou **17.113.431** entradas sem falha;
- o desempenho **melhorou**: 40,49 ns/op → **31,60 ns/op** (−22%),
  mantendo zero alocações.

### 2. Relógio anterior a 1970 corrompia o UUID silenciosamente

**Arquivo**: `uuid.go`, em `Generate`

`ms`, `rem`, `micro` e `nano` vinham de `time.Now().UnixNano()`. Em Go o
operador `%` preserva o sinal do dividendo, então com o relógio ajustado
para antes da época Unix (máquina sem RTC, contêiner mal inicializado,
NTP em falha) os campos sub-milissegundo ficavam negativos e, ao virarem
`uint16`, sofriam wrap-around. O UUID resultante ainda tinha versão 7 e
variante RFC — passava em qualquer validador — mas com campos fora da
faixa documentada e timestamp sem sentido.

Medido antes da correção, para `1969-12-31T23:59:59.123456789Z`:

| Grandeza | Antes | Faixa esperada | Agora |
|----------|------:|---------------:|------:|
| `ms`     | −876  | ≥ 0            | 0     |
| `micro & 0x0FFF` | 3553 | 0..999  | 0     |
| `nano & 0x03FF`  | 813  | 0..999  | 0     |

A correção fixa o piso do timestamp na própria época. Verificado também
que **nenhum** instante pré-1970 (600.000 combinações testadas) produz
campos fora de 0..999.

### 3. `Generator` zerado e `NewGeneratorWith(nil)` causavam pânico

**Arquivo**: `uuid.go`

Dois usos plausíveis estouravam com *nil pointer dereference*:

```go
var g loghubuuid.Generator      // valor zero, ex.: campo de outra struct
g.Generate(loghubuuid.Level1)   // panic

loghubuuid.NewGeneratorWith(nil).Generate(loghubuuid.Level1) // panic
```

Correções:

- `NewGeneratorWith(nil)` agora entra em pânico **na construção**, com
  mensagem clara. É um erro de configuração: deve aparecer no boot, não
  na primeira geração em produção.
- `Generate` recorre à entropia do gerador padrão do pacote quando o
  receptor não tem fonte (valor zero, ou ponteiro nulo), em vez de
  derrubar o processo. O comportamento está documentado no comentário do
  tipo `Generator`.

Travado por `TestNewGeneratorWithNilSourcePanics` e
`TestZeroGeneratorUsesDefaultEntropy`.

### 4. Corrida de dados na suíte de testes

**Arquivo**: `tests/benchmark_test.go`

Em `TestMassConcurrent`, 256 goroutines escreviam na variável global
`sinkU` sem sincronização; em `BenchmarkGenerateLevel3Parallel`, todas as
goroutines faziam o mesmo. `go test -race` acusava a corrida e derrubava
a suíte, impedindo que o detector fosse usado como rotina — e, portanto,
mascarando qualquer corrida real futura.

A biblioteca em si estava limpa: o defeito era só do consumidor de
resultado nos testes. Correção:

- `TestMassConcurrent` publica em uma posição própria de um slice, por
  goroutine (mesmo padrão de `tests/ordering_test.go`);
- o benchmark paralelo usa `runtime.KeepAlive`, que impede a eliminação
  pelo compilador sem escrever em estado compartilhado.

Verificado: `go test ./tests/ -race` passa por inteiro, sem avisos.

### 5. Limite real de data era 2262, não 10889

**Arquivo**: `uuid.go`

O comentário prometia validade até o ano 10889 — correto para o campo de
48 bits, mas não para a **fonte** do valor: `time.Now().UnixNano()` é um
`int64` de nanossegundos que satura em **2262-04-11**, e a partir daí
vira negativo, recaindo no item 2.

Em vez de apenas corrigir o texto, corrigiu-se a causa: a leitura passou
a ser `time.Now().Unix()` mais `time.Now().Nanosecond()`, que não satura,
e cujo segundo termo nunca é negativo. Medido para `2300-01-01T00:00:00Z`:

| | `ms` produzido |
|---|---:|
| Antes | −8032952073709 (lixo) |
| Agora | 10413792000000 (correto, cabe nos 48 bits) |

Confirmado que a mudança **não altera o comportamento no domínio normal**:
zero divergências em 3.000.000 de instantes pós-1970 comparando a
aritmética antiga com a nova, campo a campo.

### 6. Benchmarks sem aquecimento e taxa distorcida

**Arquivos**: `tests/benchmark_test.go`, `tests/benchmark-bulk/main.go`,
`docs/TEST-AND-BENCHMARK.md`

Os medidores executavam os cenários em sequência sem passagem de
aquecimento, então o **primeiro** cenário pagava sozinho o custo de
aquecer cache de instruções, escalonamento de frequência e preenchimento
do `sync.Pool`. Como o Nível 1 é sempre o primeiro, a documentação
registrava o Nível 1 como o mais lento por um motivo que não era real.

Além disso, `TestMassOneMillion` calculava a taxa como
`float64(total) / float64(dur.Milliseconds()+1)` — arredondamento para
baixo somado a 1 ms fixo, com erro da ordem de 2% em execuções curtas.

Correções: passagem de aquecimento descartada em ambos os medidores, e
taxa calculada a partir de nanossegundos. As tabelas de
`docs/TEST-AND-BENCHMARK.md` foram remedidas com o código corrigido e
agora identificam o host de cada medição.

Curiosamente, depois da correção do item 12 o Nível 1 **é** de fato o
mais lento — mas por um motivo verdadeiro e explicado na documentação:
ele consome duas palavras do gerador pseudoaleatório, e os níveis 2 e 3,
apenas uma.

### 7. Links quebrados e árvore de arquivos desatualizada

**Arquivo**: `STARTHERE.md`

O mapa do projeto apontava duas vezes para
`especificacao/ESPECIFICACAO-DESENVOLVIMENTO.md`, que não existe; o
arquivo real é `docs/SPEC.md`. A árvore de arquivos também omitia
`docs/SPEC.md`, `docs/git.md`, `CLAUDE.md`, `PROMPT.md`, o diretório
`tasks/` e sete dos dez arquivos de teste.

Árvore e links atualizados para o estado real do repositório. A prosa que
descrevia a raiz foi ajustada para reconhecer `CLAUDE.md` e `PROMPT.md`
(ver "Não aplicado" adiante).

### 8. Trechos de código não compiláveis na documentação

**Arquivos**: `README.md`, `docs/DEPLOY-FULL.md`

- `README.md`: o roteiro de compilação incluía `go get uuid-test;`, onde
  `uuid-test` é o nome do próprio módulo local do exemplo — o comando
  tenta baixar um módulo inexistente e falha. Linha removida.
- `README.md`: o `go.mod` de exemplo fixa `v0.1.0` enquanto o texto logo
  abaixo instrui `go get` sem versão. Acrescentada uma nota explicando
  que o `require` é preenchido pelo próprio `go get`.
- `docs/DEPLOY-FULL.md`: o trecho declarava `u, err :=` duas vezes no
  mesmo escopo, o que não compila. A segunda linha virou comentário.

### 9. `%x` em `uuid.UUID` imprime o hex da string, não os 16 bytes

**Arquivos**: `conversion.go`, `tests/generation_test.go`

Como `UUID` satisfaz `fmt.Stringer`, o pacote `fmt` usa `String()`
também para os verbos `%x` e `%X`: o resultado são 72 caracteres com o
hexadecimal do **texto**, e não os 16 bytes. Duas mensagens de falha da
suíte caíam nessa armadilha e imprimiriam saída ilegível justamente no
momento em que fossem necessárias.

Mensagens corrigidas para `%x` sobre a fatia (`u[:]`), e a armadilha
passou a estar documentada no comentário de `String()`, onde o usuário da
biblioteca a encontra.

### 10. `.DS_Store` versionado e ausência de `.gitignore`

`.DS_Store` estava rastreado na raiz e `docs/.DS_Store` estava pendente
para ser adicionado. Arquivos rastreados são baixados por `go get` e
ficam no cache de módulos de todos os consumidores.

- Criado `.gitignore` cobrindo `.DS_Store`, binários e perfis de teste
  (`*.test`, `*.out`, `*.prof`), o executável de exemplo e o corpus de
  fuzzing.
- `.DS_Store` foi **removido do índice** do Git (`git rm --cached`). O
  arquivo permanece no disco; apenas deixou de ser versionado. A remoção
  está preparada (*staged*) e será efetivada no próximo commit.

### 11. `TestMonotonicity` falhava permanentemente

**Arquivo**: `tests/generation_test.go`

O teste gerava 5.000 UUIDs de Nível 3 em sequência fechada e tolerava no
máximo 50 regressões de ordenação. A premissa está errada: gerar um UUID
custa cerca de 45 ns, bem menos que o passo do relógio da maioria dos
hosts, então a maioria dos pares consecutivos cai no **mesmo instante
embutido** e é desempatada por bits aleatórios. O teste media a resolução
do relógio do host, não a biblioteca.

Medição neste host (`TestTieRateReport`, agora parte da suíte):

| Nível | Pares no mesmo instante | Desses, fora de ordem |
|-------|------------------------:|----------------------:|
| 1 | 99.975 / 100.000 (100,0%) | 49.947 |
| 2 | 85.613 / 100.000 (85,6%) | 42.907 |
| 3 | 87.538 / 100.000 (87,5%) | 43.938 |

O relógio deste host oferece apenas 4.935 instantes distintos em 100.000
leituras. Antes da correção o teste acusava cerca de 2.000 regressões e
falhava sempre.

O teste foi reescrito para verificar a garantia real: com uma pausa de
200 µs entre gerações — acima da resolução de relógio de qualquer host
suportado — o instante embutido avança e a ordenação passa a ser
**estrita**. A invariante medida sem depender do relógio já está em
`TestOrderingFollowsEmbeddedTime`, e `TestTieRateReport` permanece como
diagnóstico informativo.

`CLAUDE.md` foi atualizado: a ressalva que descrevia a falha como
"ambiental, não uma regressão" saiu, e no lugar entrou a explicação de
por que esse formato de teste não deve ser reintroduzido.

### 12. Duas palavras de 64 bits sorteadas onde uma basta

**Arquivo**: `uuid.go`

`Generate` sempre chamava `g.twoWords()`, mas `r1` só é usado no Nível 1,
para extrair 12 bits. Nos níveis 2 e 3 o campo `rand_a` recebe os
microssegundos e `r1` era descartado inteiro — um sorteio por UUID jogado
fora. Em `NewGeneratorWith`, isso significava chamar a fonte do usuário
(possivelmente `crypto/rand`) duas vezes quando uma bastava.

O `Generator` passou a expor duas fontes internas (`oneWord` e
`twoWords`) e `Generate` sorteia apenas o necessário. Ganho medido neste
host, com `-benchtime=2s`:

| Benchmark | Antes | Depois | Variação |
|-----------|------:|-------:|---------:|
| `GenerateLevel1` | 44,36 ns | 43,47 ns | −2,0% |
| `GenerateLevel2` | 44,42 ns | 41,07 ns | **−7,5%** |
| `GenerateLevel3` | 45,13 ns | 41,97 ns | **−7,0%** |
| `GenerateStringLevel3` | 68,45 ns | 64,42 ns | −5,9% |
| `GenerateLevel3Parallel` | 11,85 ns | 9,74 ns | **−17,8%** |

Zero alocações preservadas em todos os casos. `TestEntropyDrawsPerLevel`
trava o número de sorteios por nível (2 no Nível 1 e nos níveis
desconhecidos, 1 nos níveis 2 e 3).

### 13. Gerador padrão previsível, sem advertência

**Arquivos**: `uuid.go`, `README.md`, `docs/DEPLOY-FULL.md`,
`STARTHERE.md`

`NewGenerator` usa PCG, um PRNG estatístico e **não** criptográfico: a
partir de poucas amostras observadas é possível reconstruir o estado
interno e prever os UUIDs seguintes. Isso é uma escolha de projeto
legítima e já registrada na especificação, e `NewGeneratorWith` existe
justamente para trocá-la — mas nenhum documento voltado ao usuário
advertia contra usar esses identificadores como segredo.

Nenhum comportamento foi alterado. Foram acrescentados:

- advertência no comentário de `NewGenerator` (aparece no `go doc`);
- seção "Aviso de segurança" no `README.md`, com o exemplo pronto de
  gerador com `crypto/rand`;
- nota em `docs/DEPLOY-FULL.md` deixando claro que, para identificadores
  que precisem ser inadivinháveis, o construtor com entropia
  criptográfica é **requisito**, não conveniência;
- ponteiro para o aviso em `STARTHERE.md`.

### 14. Contradição na documentação do tipo `Time`

**Arquivo**: `import.go`

Os comentários de campo diziam `0..999` para `Microseconds` e
`Nanoseconds`, mas `ImportBinary` é deliberadamente cego quanto ao nível:
para UUIDs de Nível 1 esses campos carregam bits aleatórios, chegando a
4095 e 1023 respectivamente. O comentário do tipo, três linhas acima, já
explicava o comportamento correto — a contradição estava nos comentários
de campo, que são justamente os que aparecem no `go doc` e no
autocompletar.

Os comentários de campo passaram a declarar as duas faixas e a condição
de cada uma. Nenhum comportamento mudou: o código já seguia a
especificação.

### 15. Lacunas de cobertura de testes

**Arquivo**: `tests/robustness_test.go` (novo)

As APIs listadas em REPORT MEL-7 sem teste dedicado agora têm um:

| API | Teste |
|-----|-------|
| `NewGeneratorWith(source)` | `TestNewGeneratorWithNilSourcePanics`, `TestEntropyDrawsPerLevel` |
| `Generate` / `GenerateString` de pacote | `TestPackageLevelShortcuts` |
| `Level` inválido | `TestUnknownLevelBehavesAsLevel1` |
| `ImportBinary` | `TestImportKnownVector` (em `import_test.go`) |

Acrescentados também `BenchmarkFromString` e `BenchmarkImportBinary`, que
faltavam para os dois caminhos que não eram medidos.

### 16. Promessa de ordenação imprecisa na especificação

**Arquivos**: `docs/SPEC.md`, `CLAUDE.md`

A especificação afirmava que comparar dois UUIDs byte a byte equivale a
compará-los no tempo, sem a ressalva do desempate. E o caso de teste
obrigatório nº 6 pedia exatamente o formato de teste defeituoso do item
11 ("em larga maioria, não decrescentes", com limiar tolerado) — ou seja,
a especificação induzia ao erro.

Correções em `docs/SPEC.md`:

- a promessa de ordenação passou a ser descrita com precisão:
  cronológica **na resolução do nível**, com desempate **aleatório**
  dentro do mesmo instante — não monotonicidade estrita;
- o caso de teste nº 6 foi reescrito para a invariante correta, com
  advertência explícita contra o formato antigo;
- o caso nº 7 passou a exigir suíte limpa sob detector de corrida;
- acrescentados os casos nº 8 (robustez do analisador, incluindo toda
  mutação de um byte e fuzzing) e nº 9 (bordas do gerador);
- a seção "Robustez" ganhou os requisitos de piso de época, saturação da
  leitura de relógio em 64 bits, ausência de leitura fora dos limites no
  analisador e validação da fonte de entropia na construção.

---

## Descartado (problemas que não existem)

### `go.mod` deveria declarar `go 1.23` — REPORT MEL-6

Descartado, como já apontava o `REPORT-CRITIC.md`. `math/rand/v2` está
disponível desde o Go 1.22, o código compila e passa nos testes sem
nenhuma API posterior, e o campo `go` declara a versão **mínima**
suportada. Subir o piso reduziria a base de consumidores sem ganho
concreto. `go 1.22` está correto.

### `unsafe.String` em `String()` — REPORT MEL-2

Descartado. O buffer de 36 bytes vive na pilha da função; devolver uma
`unsafe.String` apontando para ele produz comportamento indefinido assim
que o quadro for reutilizado. Como `String()` é API pública, não há como
garantir que o consumidor não retenha o resultado. O próprio relatório
reconhecia o risco, e o `REPORT-CRITIC.md` classificou a sugestão como
incorreta — com razão.

A alternativa segura (`AppendString` / `AppendTo`) é uma **adição de
API**, não uma correção, e está listada abaixo.

---

## Não aplicado (fora do escopo de correção de defeitos)

Itens legítimos, mas que mudam projeto ou ampliam a superfície pública em
vez de consertar algo quebrado. Ficam registrados para decisão do dono do
projeto.

**NEWS PERF-02 — trocar `sync.Pool`+PCG pelo gerador global de
`math/rand/v2`.** A medição do relatório é convincente (e o ChaCha8 do
runtime ainda mitigaria o SEC-01), mas isto é uma decisão de arquitetura,
não um defeito: o desenho atual está descrito em `CLAUDE.md` como
característica do projeto. Recomenda-se avaliar em separado, com
benchmark próprio, agora que o PERF-01 já entregou parte do ganho
(−17,8% no caminho paralelo).

**NEWS MELHORIA-02, PERF-04, REPORT MEL-1/MEL-4/MEL-5 — adições de API**
(`MarshalText`/`UnmarshalText`, `MarshalJSON`, `driver.Valuer`/
`sql.Scanner`, `Bytes()`/`FromBytes()`, `IsZero()`, `IsValid()`,
`Compare()`, `AppendTo()`, `Time.ToTime()`). Todas úteis e nenhuma delas
altera o layout de bits ou o caminho quente — mas são funcionalidade
nova, não correção. O pedido aqui era corrigir defeitos.

**NEWS MELHORIA-03 — relógio injetável.** Também é ampliação de API.
Consequência prática: a correção do item 2 (relógio pré-época) **não tem
teste de regressão na suíte**, porque não há como fixar o instante de
fora do pacote. Ela foi verificada por um programa auxiliar que replica
as duas aritméticas e as compara em 3.000.000 de instantes pós-1970
(zero divergências) e em 600.000 combinações pré-1970 (zero campos fora
da faixa). Se um relógio injetável for adicionado no futuro, este é o
primeiro teste a escrever.

**NEWS MELHORIA-01 (parte de código) — contador monotônico.** O protótipo
do relatório zera as regressões de ordenação, mas custa 32× mais no
caminho paralelo, destruindo a escalabilidade que é o principal atrativo
da biblioteca. A recomendação do próprio relatório — oferecer como
construtor opcional, nunca como padrão — é acertada, e continua sendo
funcionalidade nova. Nesta passagem, apenas a documentação foi corrigida
para descrever a ordenação com precisão (item 16).

**NEWS SEC-02 — entropia efetiva por nível.** Informativo. A verificação
confirma unicidade confortável (nenhuma duplicata em 1.000.000 por nível
e em 1.280.000 concorrentes); o risco real era de imprevisibilidade, e
esse foi tratado no item 13.

**NEWS BUG-07 (parte) — `PROMPT.md` na raiz.** `CLAUDE.md` estabelece que
a raiz contém apenas o necessário para produção, e `PROMPT.md` não é
arquivo de produção. Mover arquivos é decisão de layout do dono do
projeto, não correção de defeito; o `STARTHERE.md` foi ajustado para
descrever a raiz como ela é.

---

## Verificação final

Tudo abaixo foi executado neste host, com o código corrigido.

```bash
go build ./...                                              # sem erros
go vet ./...                                                # sem apontamentos
gofmt -l .                                                  # sem saída
go test ./tests/                                            # 40 testes, todos passam
go test ./tests/ -race                                      # suíte inteira limpa
go test ./tests/ -run '^$' -fuzz FuzzFromString -fuzztime 45s   # 17.113.431 execuções, sem falha
```

Situação da suíte antes e depois:

| | Antes | Depois |
|---|---|---|
| `go test ./tests/` | 4 falhas (`TestMonotonicity`, `TestFromStringNeverPanics`, `TestFromStringExtraHyphen`, `FuzzFromString` — esta com pânico que derrubava a suíte) | 40 testes, todos passam |
| `go test ./tests/ -race` | corrida de dados acusada | limpa |

Desempenho, `-benchtime=2s`, zero alocações preservadas:

| Benchmark | Antes | Depois |
|-----------|------:|-------:|
| `GenerateLevel1` | 44,36 ns | 43,47 ns |
| `GenerateLevel2` | 44,42 ns | 41,07 ns |
| `GenerateLevel3` | 45,13 ns | 41,97 ns |
| `GenerateStringLevel1` | 66,25 ns | 67,91 ns |
| `GenerateStringLevel3` | 68,45 ns | 64,42 ns |
| `GenerateLevel3Parallel` | 11,85 ns | 9,74 ns |
| `FromString` | 40,49 ns | 31,60 ns |

Nenhuma correção alterou o layout de bits, a semântica dos níveis ou o
formato de saída. Os UUIDs produzidos antes e depois são
indistinguíveis para qualquer instante posterior a 1970.

---

## Arquivos alterados

**Produção**

| Arquivo | Itens |
|---------|-------|
| `uuid.go` | 2, 3, 5, 12, 13 |
| `conversion.go` | 1, 9 |
| `import.go` | 14 |

**Testes**

| Arquivo | Itens |
|---------|-------|
| `tests/robustness_test.go` (novo) | 3, 12, 15 |
| `tests/benchmark_test.go` | 4, 6, 15 |
| `tests/generation_test.go` | 9, 11 |
| `tests/benchmark-bulk/main.go` | 6 |

**Documentação e higiene**

| Arquivo | Itens |
|---------|-------|
| `README.md` | 8, 13 |
| `STARTHERE.md` | 7, 13 |
| `CLAUDE.md` | 11, 12, 16 |
| `docs/SPEC.md` | 16 |
| `docs/DEPLOY-FULL.md` | 8, 13 |
| `docs/TEST-AND-BENCHMARK.md` | 6 |
| `.gitignore` (novo) | 10 |
| `.DS_Store` | 10 (removido do índice do Git; permanece no disco) |

Os arquivos `docs/DEPLOY-FAST.md` e `docs/git.md` aparecem como
modificados no `git status`, mas já estavam assim antes desta passagem e
não foram tocados aqui.

Os arquivos de teste `alloc_test.go`, `fuzz_test.go`, `import_test.go`,
`layout_test.go`, `ordering_test.go` e `parsing_test.go` acompanhavam o
`NEWS.md` como regressões e não foram alterados — apenas passaram a
passar.
