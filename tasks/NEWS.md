# Revisão técnica — go-loghub-uuid

Revisão completa do código de produção (`uuid.go`, `conversion.go`,
`import.go`), da suíte de testes e da documentação, em busca de defeitos,
riscos e oportunidades de desempenho.

- **Data**: 2026-08-27
- **Commit base**: `0e1b31e`
- **Ambiente de medição**: Apple M2, macOS (Darwin 25.6.0), Go 1.27.0,
  `GOMAXPROCS=8`
- **Ferramentas**: `go vet`, `go test -race`, `go test -fuzz`,
  `testing.AllocsPerRun`, benchmarks dedicados

Todos os números abaixo foram **medidos**, não estimados. Os protótipos
das correções foram construídos e executados antes de serem recomendados.

---

## Sumário

| ID | Severidade | Assunto |
|----|-----------|---------|
| [BUG-01](#bug-01) | **Crítica** | Pânico de índice fora de faixa em `FromString` |
| [BUG-02](#bug-02) | Média | Relógio anterior a 1970 corrompe o UUID silenciosamente |
| [BUG-03](#bug-03) | Média | `Generator` zerado e `NewGeneratorWith(nil)` causam pânico |
| [BUG-04](#bug-04) | Média | Corrida de dados na própria suíte de testes |
| [BUG-05](#bug-05) | Baixa | Comentário promete validade até 10889; a fonte falha em 2262 |
| [BUG-06](#bug-06) | Baixa | Benchmarks sem aquecimento inflam o primeiro cenário em ~60% |
| [BUG-07](#bug-07) | Baixa | Links quebrados e árvore de arquivos desatualizada no STARTHERE |
| [BUG-08](#bug-08) | Baixa | Trechos de código não compiláveis no README e no DEPLOY-FULL |
| [BUG-09](#bug-09) | Baixa | `%x` em `uuid.UUID` imprime o hex da string, não os 16 bytes |
| [BUG-10](#bug-10) | Baixa | `.DS_Store` versionado; repositório sem `.gitignore` |
| [PERF-01](#perf-01) | — | Duas palavras de 64 bits sorteadas onde uma basta (−10% no Nível 3) |
| [PERF-02](#perf-02) | — | `sync.Pool` + PCG mais lento que o gerador global (−39% em paralelo) |
| [PERF-03](#perf-03) | — | `time.Now()` consome 73% do custo — teto prático da geração |
| [PERF-04](#perf-04) | — | Falta API de serialização sem alocação (`AppendTo`) |
| [SEC-01](#sec-01) | Média | Gerador padrão é previsível (PCG não é criptográfico) |
| [SEC-02](#sec-02) | Informativa | Entropia efetiva cai a 52 bits no Nível 3 |
| [MELHORIA-01](#melhoria-01) | — | Sem contador monotônico: 90% dos pares consecutivos empatam |
| [MELHORIA-02](#melhoria-02) | — | Lacunas de API (`encoding`, `database/sql`, `Time.ToTime`) |
| [MELHORIA-03](#melhoria-03) | — | Relógio não injetável impede teste determinístico do tempo |
| [MELHORIA-04](#melhoria-04) | — | Contradição na documentação do tipo `Time` |

---

## Defeitos

### BUG-01 — Pânico de índice fora de faixa em `FromString` {#bug-01}

**Severidade: crítica.** `FromString` lê fora dos limites da string e
provoca `panic: runtime error: index out of range [36] with length 36`.

Arquivo: [conversion.go:53](../conversion.go:53)

```go
high, ok1 := fromHex(s[i])
low, ok2 := fromHex(s[i+1])   // <-- i pode valer 35, e s[36] não existe
```

**Causa.** O laço valida o tamanho (36) e os quatro hífens canônicos, mas
depois percorre a string pulando **qualquer** hífen, um a um. Um hífen
extra num deslocamento par do último grupo desalinha os pares: o índice
chega a 35, o laço entra (`35 < 36`) e lê `s[36]`.

**Reprodução.** Exatamente **6 mutações de um único byte** sobre uma
string canônica válida disparam o pânico — trocar por `-` os
deslocamentos **24, 26, 28, 30, 32 e 34**:

```
FromString("0192f7c5-1a2b-7c3d-8e4f--abbccddeeff")  -> panic
FromString("0192f7c5-1a2b-7c3d-8e4f-aabbccddee-f")  -> panic
```

**Impacto.** `FromString`/`StringToBinary`/`Import` são exatamente as
funções que recebem dado externo — UUID vindo de requisição HTTP, de
linha de log, de coluna de banco. Uma string de 36 caracteres com um
hífen a mais derruba o processo. É uma negação de serviço remota trivial
de acionar, inclusive por acidente.

**Correção proposta.** Decodificar a partir de deslocamentos fixos, já
que as posições dos 16 bytes são conhecidas. Elimina a leitura fora dos
limites por construção, rejeita hífens extras (o hífen não é dígito
hexadecimal) e dispensa o teste de hífen por caractere:

```go
// hexOffsets guarda o deslocamento do dígito alto de cada um dos 16
// bytes dentro da string canônica de 36 caracteres.
var hexOffsets = [16]int{0, 2, 4, 6, 9, 11, 14, 16, 19, 21, 24, 26, 28, 30, 32, 34}

func FromString(s string) (UUID, error) {
	var u UUID
	if len(s) != 36 {
		return UUID{}, ErrInvalidFormat
	}
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return UUID{}, ErrInvalidFormat
	}
	for j, p := range hexOffsets {
		high, ok1 := fromHex(s[p])
		low, ok2 := fromHex(s[p+1])
		if !ok1 || !ok2 {
			return UUID{}, ErrInvalidFormat
		}
		u[j] = high<<4 | low
	}
	return u, nil
}
```

O maior índice lido passa a ser `34+1 = 35`, seguro por construção.

**Validação da correção** (protótipo construído e executado):

- as duas regressões novas (`TestFromStringNeverPanics`,
  `TestFromStringExtraHyphen`) passam;
- `FuzzFromString` rodou **14.315.461 execuções** sobre a versão
  corrigida sem encontrar nenhuma outra falha;
- o desempenho **melhora**: 40,81 ns/op → 39,69 ns/op, mantendo zero
  alocações.

**Testes.** [tests/parsing_test.go](../tests/parsing_test.go),
[tests/fuzz_test.go](../tests/fuzz_test.go) — hoje **falham**, por
projeto: são as regressões que provam o defeito.

---

### BUG-02 — Relógio anterior a 1970 corrompe o UUID silenciosamente {#bug-02}

**Severidade: média.** `Generate` assume `time.Now().UnixNano() >= 0`.
Com o relógio do sistema ajustado para antes de 1970 (máquina sem RTC,
container mal inicializado, NTP em falha), a aritmética de decomposição
produz lixo sem erro nem aviso.

Arquivo: [uuid.go:133](../uuid.go:133)

Medição da aritmética para `now = -876543211` (31/12/1969 23:59:59):

| Grandeza | Valor produzido | Faixa esperada |
|----------|----------------|----------------|
| `ms`     | `-876` → byte 0 vira `0xff` | 48 bits positivos |
| `micro`  | `64993`, e `& 0x0FFF` = **3553** | 0..999 |
| `nano`   | `65325`, e `& 0x03FF` = **813** | 0..999 |

O resultado ainda tem versão 7 e variante RFC — passa em qualquer
validador — mas o timestamp aponta para o ano 10889 e os campos sub-ms
saem da faixa documentada. Pior: os UUIDs deixam de ser ordenáveis em
relação a todos os anteriores.

**Correção sugerida.** Fixar o piso em zero antes de decompor, e
documentar o comportamento:

```go
now := time.Now().UnixNano()
if now < 0 { // relógio anterior à época Unix: degrada para a época
	now = 0
}
```

---

### BUG-03 — `Generator` zerado e `NewGeneratorWith(nil)` causam pânico {#bug-03}

**Severidade: média.** O campo `twoWords` não tem validação nem valor de
fallback, então dois usos plausíveis explodem com
`invalid memory address or nil pointer dereference`:

```go
var g loghubuuid.Generator      // valor zero
g.Generate(loghubuuid.Level1)   // panic

g := loghubuuid.NewGeneratorWith(nil)
g.Generate(loghubuuid.Level1)   // panic
```

Arquivos: [uuid.go:62](../uuid.go:62), [uuid.go:99](../uuid.go:99)

O primeiro caso é comum: `Generator` é exportado como struct, então
embuti-lo por valor (`type App struct { Gen loghubuuid.Generator }`) é
natural e compila. O segundo é um erro de chamada que deveria ser
reportado, não postergado até o primeiro `Generate`.

**Correção sugerida.** Duas opções, ambas baratas:

1. `NewGeneratorWith` entra em pânico imediatamente se `source == nil`,
   com mensagem clara (falha na configuração, não em produção); e
   `Generate` cai na entropia padrão quando `g.twoWords == nil`.
2. Ou tornar o tipo inutilizável fora dos construtores (campo obrigatório
   verificado), documentando que o valor zero não é válido.

---

### BUG-04 — Corrida de dados na própria suíte de testes {#bug-04}

**Severidade: média** (afeta a capacidade de auditar a biblioteca).

`go test ./tests/ -race` acusa corrida de dados:

```
WARNING: DATA RACE
Write at 0x0001026ce0a0 by goroutine 13:
  tests.TestMassConcurrent.func1()
      tests/benchmark_test.go:135
```

Arquivos: [tests/benchmark_test.go:135](../tests/benchmark_test.go:135)
e [tests/benchmark_test.go:61](../tests/benchmark_test.go:61) — 256
goroutines (e, no benchmark paralelo, todas as goroutines) escrevem na
mesma variável global `sinkU` sem sincronização.

A biblioteca em si **está limpa**: o novo `TestConcurrentUniqueness`, que
usa buffers por goroutine, passa sob `-race` sem qualquer aviso. O
problema é só do consumidor de resultado dos testes — mas, como derruba a
suíte, impede que `-race` seja usado como rotina e mascararia uma corrida
real futura.

**Correção sugerida.** Acumular em variável local e publicar uma única
vez, por goroutine, em posição própria de um slice (como faz
[tests/ordering_test.go](../tests/ordering_test.go)).

**Testes.** [tests/ordering_test.go](../tests/ordering_test.go) —
`TestConcurrentUniqueness` (passa, inclusive com `-race`).

---

### BUG-05 — Comentário promete validade até 10889; a fonte falha em 2262 {#bug-05}

**Severidade: baixa** (documentação incorreta sobre um limite real).

Arquivo: [uuid.go:135](../uuid.go:135)

```go
ms := now / 1_000_000  // milissegundos (cabem em 48 bits até o ano 10889)
```

O campo de 48 bits realmente cobre até **10889-08-02** — verificado. Mas
a **fonte** do valor é `time.Now().UnixNano()`, um `int64` de
nanossegundos que satura em **2262-04-11 23:47:16 UTC** — também
verificado. A partir daí o valor vira negativo e recai no BUG-02.

O limite prático da biblioteca é 2262, não 10889. O comentário, como
está, sugere uma folga que não existe. Corrigir o texto e, se o horizonte
importar, trocar a leitura por `time.Now()` decomposto em
`Unix()`/`Nanosecond()`, que não satura.

---

### BUG-06 — Benchmarks sem aquecimento inflam o primeiro cenário em ~60% {#bug-06}

**Severidade: baixa** (metodologia de medição; conclusões publicadas
estão erradas).

Tanto `TestMassOneMillion` quanto `benchmark-bulk` medem os cenários em
sequência, sem passagem de aquecimento. O **primeiro** cenário medido
paga sozinho o custo de aquecimento (cache de instruções, escalonamento
de frequência, preenchimento do `sync.Pool`) e aparece muito mais lento.

Como o Nível 1 é sempre o primeiro, a documentação registra o Nível 1
como o nível mais lento — o que é falso. Medição controlada, invertendo a
ordem dos cenários:

| Ordem de execução | Nível 1 | Nível 2 | Nível 3 |
|-------------------|--------:|--------:|--------:|
| Original (1,2,3)  | **72,8 ns** | 46,3 ns | 45,6 ns |
| Invertida (3,2,1) | **44,2 ns** | 44,9 ns | 45,6 ns |
| Segunda passada   | **45,1 ns** | 45,0 ns | 45,5 ns |

Aquecidos, os três níveis custam o mesmo (~45 ns). O `go test -bench`
concorda (43,99 / 44,61 / 45,16 ns), porque amortiza o aquecimento em
milhões de iterações.

O mesmo viés está publicado em
[docs/TEST-AND-BENCHMARK.md](../docs/TEST-AND-BENCHMARK.md): a tabela
mostra `GenerateLevel1` a ~98 ns contra ~85 ns dos níveis 2 e 3, e a
tabela em massa mostra ~90 ms contra ~88 ms.

**Correção sugerida.** Executar uma passagem de descarte (por exemplo,
10.000 gerações de cada nível) antes da primeira medição, em
[tests/benchmark-bulk/main.go](../tests/benchmark-bulk/main.go) e em
`TestMassOneMillion`; depois, refazer as tabelas da documentação.

Detalhe adicional: em `TestMassOneMillion`, a taxa é calculada como
`float64(total) / float64(dur.Milliseconds()+1)`
([tests/benchmark_test.go:81](../tests/benchmark_test.go:81)). O `+1`
distorce o resultado — em 44 ms de execução, o erro é de ~2%. Usar
`dur.Nanoseconds()`, como já faz `benchmark-bulk`.

---

### BUG-07 — Links quebrados e árvore de arquivos desatualizada no STARTHERE {#bug-07}

**Severidade: baixa.** [STARTHERE.md](../STARTHERE.md) — que se apresenta
como "mapa do projeto" e é a porta de entrada recomendada — aponta duas
vezes para um arquivo que não existe:

- linha 37-38: a árvore lista `especificacao/ESPECIFICACAO-DESENVOLVIMENTO.md`
- linha 126: link `[especificacao/ESPECIFICACAO-DESENVOLVIMENTO.md](...)`

O arquivo real é [docs/SPEC.md](../docs/SPEC.md) (o README já aponta
corretamente). A árvore de arquivos também omite `docs/SPEC.md`,
`docs/git.md`, `CLAUDE.md` e `PROMPT.md`.

Relacionado: `CLAUDE.md` estabelece que a raiz contém **apenas** o
necessário para produção, mas `PROMPT.md` está na raiz e não é arquivo de
produção — ou vai para `docs/`, ou a regra precisa ser ajustada.

---

### BUG-08 — Trechos de código não compiláveis no README e no DEPLOY-FULL {#bug-08}

**Severidade: baixa.**

1. [README.md:69](../README.md:69) — o roteiro de compilação inclui
   `go get uuid-test;`, onde `uuid-test` é o nome do **próprio módulo
   local** do exemplo. O comando tenta baixar um módulo inexistente e
   falha. A seção "Compilar (alternativa)", logo abaixo, está correta.

2. [README.md:44](../README.md:44) — o `go.mod` de exemplo fixa
   `v0.1.0`. A tag existe no repositório, mas o exemplo instrui
   `go get github.com/patrickbrandao/go-loghub-uuid` (sem versão) logo
   antes; convém alinhar os dois trechos.

3. `docs/DEPLOY-FULL.md`, linhas 111-112 — o trecho declara `u, err :=`
   duas vezes no mesmo escopo:

   ```go
   u, err := uuid.FromString(s)        // método de fábrica
   u, err := uuid.StringToBinary(s)    // função equivalente
   ```

   Não compila. A segunda linha precisa ser comentário ou usar `=`.

---

### BUG-09 — `%x` em `uuid.UUID` imprime o hex da string, não os 16 bytes {#bug-09}

**Severidade: baixa** (armadilha de API, hoje presente nos testes).

Como `UUID` implementa `fmt.Stringer`, o pacote `fmt` usa `String()`
também para os verbos `%x` e `%X`. O resultado é o hexadecimal **da
representação textual** — 72 caracteres — e não os 16 bytes:

```
%s -> 01a04448-0e84-72be-8008-3b23842a6bb0
%x -> 30316130343434382d306538342d373262652d383030382d336232333834326136626230
```

Os testes existentes caem nessa armadilha em
[tests/generation_test.go:39](../tests/generation_test.go:39) e
[tests/generation_test.go:51](../tests/generation_test.go:51): quando
falharem, imprimirão a saída ilegível acima em vez dos bytes divergentes.

**Correção sugerida.** Nos testes, converter antes:
`%x` sobre `[16]byte(u)`. Na biblioteca, documentar a armadilha no
comentário de `String()` e considerar expor `Bytes() []byte`.

---

### BUG-10 — `.DS_Store` versionado; repositório sem `.gitignore` {#bug-10}

**Severidade: baixa** (higiene; afeta quem consome a biblioteca).

`git ls-files` mostra `.DS_Store` rastreado na raiz, e `docs/.DS_Store`
está pendente para ser adicionado. Não existe `.gitignore` no
repositório.

Arquivos rastreados são baixados por `go get` e ficam no cache de módulos
de todos os consumidores. Remover do índice (`git rm --cached .DS_Store`)
e criar um `.gitignore` com, no mínimo, `.DS_Store`, binários de teste
(`*.test`), perfis (`*.out`) e `testdata/fuzz/` se o corpus não for
versionado.

---

## Desempenho

Linha de base medida (`go test -bench -benchmem -benchtime=2s`, Apple M2):

| Benchmark | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| `GenerateLevel1` | 43,99 | 0 | 0 |
| `GenerateLevel2` | 44,61 | 0 | 0 |
| `GenerateLevel3` | 45,16 | 0 | 0 |
| `GenerateStringLevel1` | 66,39 | 48 | 1 |
| `GenerateStringLevel3` | 69,04 | 48 | 1 |
| `GenerateLevel3Parallel` | 12,83 | 0 | 0 |

As propriedades de zero alocação prometidas pela especificação **estão
cumpridas** e agora ficam travadas por teste
([tests/alloc_test.go](../tests/alloc_test.go)).

### PERF-01 — Duas palavras de 64 bits sorteadas onde uma basta {#perf-01}

`Generate` sempre chama `g.twoWords()`, obtendo `r1` e `r2`
([uuid.go:140](../uuid.go:140)). Mas `r1` só é usado no **Nível 1**, para
extrair 12 bits (`uint16(r1) & 0x0FFF`). Nos níveis 2 e 3, `rand_a`
recebe os microssegundos e `r1` é descartado inteiro — uma chamada ao
PRNG por UUID, jogada fora.

Custo medido do sorteio: 9,63 ns para duas palavras contra 7,85 ns para
uma.

Ganho medido ao sortear a segunda palavra apenas quando necessária:

| Cenário | Atual | Otimizado | Ganho |
|---------|------:|----------:|------:|
| Nível 3, serial | 44,96 ns | **40,23 ns** | −10,5% |
| Nível 3, paralelo | 12,24 ns | **7,42 ns** | −39,4% |
| Nível 1, serial | 44,79 ns | 44,00 ns | ~0 (usa as duas) |

Também elimina o desperdício em `NewGeneratorWith`, onde a fonte
personalizada — que pode ser `crypto/rand` — é chamada **duas vezes** por
UUID mesmo quando uma basta.

### PERF-02 — `sync.Pool` + PCG é mais lento que o gerador global {#perf-02}

O `sync.Pool` de PCGs foi escolhido para evitar contenção de lock. A
premissa não se sustenta mais: desde o Go 1.22, as funções de pacote de
`math/rand/v2` usam `runtime.rand()`, que já é por thread, sem lock, e
mais barato que o par `Get`/`Put` do pool.

| Fonte de entropia | Serial | Paralelo (8 núcleos) |
|-------------------|-------:|---------------------:|
| `sync.Pool` + PCG, 2 palavras | 9,63 ns | 2,75 ns |
| `sync.Pool` + PCG, 1 palavra | 7,85 ns | — |
| `math/rand/v2` global, 2 palavras | 9,23 ns | — |
| `math/rand/v2` global, 1 palavra | **4,49 ns** | **0,91 ns** |

Combinado com PERF-01, é o que produz os 40,23 ns / 7,42 ns da tabela
anterior. Benefício adicional: o gerador global do runtime é ChaCha8
semeado pelo sistema operacional, o que **também** mitiga o SEC-01.

O `sync.Pool` traz ainda um custo escondido: o GC esvazia o pool
periodicamente, e cada recriação chama `strongSeed()` duas vezes, ou seja
duas leituras de `crypto/rand`. Em serviço com GC frequente, esse custo
reaparece de forma irregular.

### PERF-03 — `time.Now()` consome 73% do custo {#perf-03}

Medição isolada: `time.Now().UnixNano()` custa **32,29 ns/op** neste
host, contra 44 ns do `Generate` completo.

Ou seja: **73% do custo de gerar um UUID é ler o relógio**, e esse custo
é irredutível — a precisão sub-milissegundo é a razão de ser da
biblioteca. Somadas, PERF-01 e PERF-02 removem quase toda a fração
restante que é otimizável.

Vale registrar isso na documentação: qualquer micro-otimização adicional
na montagem dos 16 bytes é ruído estatístico, e a documentação atual não
deixa claro onde está o teto.

### PERF-04 — Falta API de serialização sem alocação {#perf-04}

A implementação atual de `String()` **já é a melhor das testadas** — não
mexer. A variante com `encoding/hex` é mais lenta:

| Implementação | ns/op | B/op | allocs/op |
|---------------|------:|-----:|----------:|
| `String()` atual (laço) | **26,70** | 48 | 1 |
| Variante com `hex.Encode` | 29,45 | 48 | 1 |
| Escrita em buffer do chamador | **18,92** | **0** | **0** |

A única alocação é a string devolvida, inevitável na assinatura atual.
Mas falta a API que permite evitá-la: um método que escreve num buffer do
chamador reduz o custo em 29% e zera as alocações — relevante para quem
serializa milhões de identificadores em log ou JSON.

```go
// AppendTo acrescenta a forma canônica de u a dst e devolve o slice
// estendido, sem alocar quando dst tem capacidade suficiente.
func (u UUID) AppendTo(dst []byte) []byte
```

Convém que `AppendText`/`MarshalText` (ver MELHORIA-02) sejam construídos
sobre esse método.

---

## Segurança

### SEC-01 — O gerador padrão é previsível {#sec-01}

**Severidade: média**, condicionada ao uso.

`NewGenerator` usa PCG (`math/rand/v2`), que é um PRNG estatístico e
**não** criptográfico. Sua saída de 64 bits expõe o estado interno: a
partir de poucas amostras observadas, um adversário reconstrói o estado
daquele gerador e prevê todos os UUIDs seguintes produzidos por ele — e,
no Nível 1, 62 dos bits de `rand_b` vêm diretamente de uma única saída do
PCG.

Isso é uma **escolha de projeto legítima e documentada** na
especificação (seção 8, "a entropia de execução pode ser pseudoaleatória
rápida"), e `NewGeneratorWith` existe justamente para trocá-la. O
problema é que nenhum documento voltado ao usuário — README, DEPLOY-FAST,
DEPLOY-FULL — **adverte** contra usar esses UUIDs como token de sessão,
link secreto, chave de recuperação ou qualquer identificador que precise
ser inadivinhável. `DEPLOY-FULL.md` apresenta o construtor com entropia
criptográfica como uma opção de conveniência, não como requisito de
segurança.

**Correção sugerida.** Um aviso explícito no README e no DEPLOY-FULL, no
formato "não use o gerador padrão para segredos; use
`NewGeneratorWith(crypto/rand)`". Adotar PERF-02 (ChaCha8 do runtime)
melhora bastante o quadro sem custo, mas o aviso continua necessário,
porque UUIDv7 expõe o timestamp por construção.

### SEC-02 — Entropia efetiva por nível {#sec-02}

Informativo, para dimensionar o risco de colisão e de adivinhação:

| Nível | Bits aleatórios | Observação |
|-------|----------------:|------------|
| 1 | 74 | 12 de `rand_a` + 62 de `rand_b` |
| 2 | 62 | `rand_a` vira tempo |
| 3 | **52** | `rand_a` e o topo de `rand_b` viram tempo |

`docs/DEPLOY-FULL.md` já traz essa tabela — o que falta é a leitura
prática: no Nível 3, dentro de um mesmo nanossegundo, restam 52 bits, e o
paradoxo do aniversário coloca a probabilidade de colisão em ~50% na
ordem de 2^26 ≈ 67 milhões de UUIDs **gerados no mesmo nanossegundo** —
inatingível na prática. A unicidade está confortável; o que não está é a
imprevisibilidade (SEC-01).

**Verificação executada.** Nenhuma duplicata em 2.000.000 de UUIDs por
nível, e nenhuma em 1.280.000 gerados por 64 goroutines simultâneas.

---

## Melhorias

### MELHORIA-01 — Sem contador monotônico: 90% dos pares consecutivos empatam {#melhoria-01}

Este é o achado funcional mais relevante depois do BUG-01.

A promessa central da biblioteca — "a ordenação lexicográfica coincide
com a ordem cronológica" — **só vale quando o relógio avança entre duas
gerações**. Não há contador de desempate: dois UUIDs do mesmo instante
embutido são ordenados por bits aleatórios, ou seja, aleatoriamente.

O problema não é teórico. Gerar um UUID custa ~45 ns; o relógio do host
avança bem mais devagar que isso. Medição (`TestTieRateReport`):

| Nível | Pares consecutivos no mesmo instante embutido | Desses, fora de ordem |
|-------|---------------------------------------------:|----------------------:|
| 1 | 99.990 / 100.000 (**100,0%**) | 49.954 |
| 2 | 90.249 / 100.000 (**90,2%**) | 45.134 |
| 3 | 90.098 / 100.000 (**90,1%**) | 44.794 |

Relógio deste host: apenas **3.946 instantes distintos** em 100.000
leituras de `time.Now()` — resolução de microssegundo, com o dígito de
nanossegundo sempre zero. É o comportamento já registrado em `CLAUDE.md`
como causa da falha de `TestMonotonicity` no macOS.

**A ressalva importante:** isto é mais amplo do que "uma peculiaridade do
macOS". Mesmo num host com relógio de nanossegundo, `time.Now()` custa
~32 ns e a geração inteira ~45 ns; instantes repetidos continuam
acontecendo, só que com menos frequência. E o Nível 1 empata **100% das
vezes** em qualquer plataforma, porque sua resolução é o milissegundo.

O que **está** correto, e agora fica travado por teste: sempre que o
instante embutido de fato avança, a ordem é respeitada — em binário e em
string, nos três níveis, em 200.000 amostras cada
(`TestOrderingFollowsEmbeddedTime`). O layout de bits está certo; falta o
desempate.

**Protótipo medido.** Contador de 16 bits ocupando o topo dos 52 bits
aleatórios do Nível 3 (restando 36 bits aleatórios), com estado
compartilhado em um `atomic.Uint64` e laço de CAS — a Method 1 da RFC
9562, seção 6.2, combinada com a Method 3 já em uso:

| Métrica | Sem contador | Com contador |
|---------|-------------:|-------------:|
| Pares fora de ordem (200.000) | 88.017 (**44,0%**) | **0 (0,0%)** |
| Custo serial | 40,83 ns | 44,29 ns (**+8,5%**) |
| Custo paralelo (8 núcleos) | 7,40 ns | **233,6 ns (32× pior)** |

**Recomendação: oferecer como opção, nunca como padrão.** O contador
resolve a ordenação por completo e custa pouco em fluxo serial, mas o
estado compartilhado destrói a escalabilidade que é o principal atrativo
da biblioteca. Um construtor dedicado deixa a escolha explícita:

```go
// NewMonotonicGenerator devolve um gerador que garante ordenação
// estrita mesmo dentro do mesmo instante do relógio, ao custo de estado
// compartilhado (throughput bem menor sob concorrência alta).
func NewMonotonicGenerator() *Generator
```

Enquanto isso não existir, a documentação precisa ser precisa: a
ordenação é **cronológica na resolução do nível**, com desempate
aleatório dentro do mesmo instante — e não "monotônica".

**Sobre `TestMonotonicity`.** O teste atual
([tests/generation_test.go:107](../tests/generation_test.go:107)) tolera
50 regressões em 5.000 amostras, ou seja assume que empates são raros.
Como se vê, empates são o caso comum. O teste falha neste host (~2.300
regressões) e é frágil em qualquer host: mede a resolução do relógio, não
a biblioteca. Sugestão: substituí-lo por
`TestOrderingFollowsEmbeddedTime`, que testa a invariante real e é
independente do relógio, mantendo `TestTieRateReport` como diagnóstico
informativo.

**Testes.** [tests/ordering_test.go](../tests/ordering_test.go).

### MELHORIA-02 — Lacunas de API {#melhoria-02}

Ausências que forçam todo consumidor a escrever o mesmo código auxiliar:

| Faltando | Por quê |
|----------|---------|
| `MarshalText` / `UnmarshalText` | JSON, YAML e `encoding` em geral. Hoje um `UUID` dentro de uma struct serializa como array de 16 números. |
| `MarshalJSON` / `UnmarshalJSON` | Idem, para controle direto. |
| `driver.Valuer` / `sql.Scanner` | Gravar e ler de banco sem conversão manual — caso de uso central de um projeto chamado *loghub*. |
| `Time.ToTime() time.Time` | A aritmética de reconstrução está duplicada em `docs/DEPLOY-FULL.md` e em `tests/generation_test.go`. É exatamente o tipo de código que o usuário erra. |
| `AppendTo([]byte) []byte` | Ver PERF-04: 29% mais rápido, zero alocação. |
| `Bytes() []byte` | Contorna a armadilha do BUG-09. |
| `IsValid() bool` | `Version() == 7 && Variant() == 2` num só lugar. |
| `Compare(a, b UUID) int` | Ordenação explícita, sem depender de `String()`. |

Nenhuma delas altera o layout de bits nem o desempenho do caminho
quente; são adições puras.

### MELHORIA-03 — Relógio não injetável impede teste determinístico {#melhoria-03}

`Generate` chama `time.Now()` diretamente
([uuid.go:133](../uuid.go:133)). Não há como fixar o instante, e portanto
**não há como testar de forma determinística** que os 48 bits de
milissegundo, os 12 de microssegundo e os 10 de nanossegundo caem nas
posições certas para um instante conhecido.

A suíte nova contorna isso injetando entropia constante
(`NewGeneratorWith`) e comparando com `time.Now()` medido em torno da
geração — funciona, e os testes
[tests/layout_test.go](../tests/layout_test.go) e
[tests/import_test.go](../tests/import_test.go) travam o layout por esse
caminho. Mas um relógio injetável permitiria vetores dourados de verdade,
além de testar as bordas (viradas de segundo, de milissegundo, ano 2262,
BUG-02) sem mexer no relógio da máquina.

Sugestão de forma minimamente invasiva: campo opcional `now func() int64`
no `Generator`, com `time.Now().UnixNano()` como padrão, e um construtor
interno para testes.

### MELHORIA-04 — Contradição na documentação do tipo `Time` {#melhoria-04}

[import.go:13-14](../import.go:13) documenta os campos como:

```go
Microseconds int // 0..999  (fração do milissegundo) — lido de rand_a
Nanoseconds  int // 0..999  (fração do microssegundo) — lido de rand_b[61:52]
```

Mas `ImportBinary` é deliberadamente cego quanto ao nível, então para
UUIDs de Nível 1 esses campos carregam bits aleatórios. Valores máximos
medidos em 500.000 amostras de Nível 1: **micro = 4095** (12 bits) e
**nano = 1023** (10 bits) — bem fora da faixa anunciada.

O comentário do próprio tipo, três linhas acima, já explica o
comportamento correto; são os comentários de campo que contradizem. Como
o comentário de campo é o que aparece no autocompletar da IDE e no
`go doc`, ele é o que será lido. Corrigir para algo como
`0..999 quando o UUID é de Nível 2/3; 0..4095 (bits aleatórios) no Nível 1`.

O comportamento do código está certo e segue a especificação (seção 5) —
o defeito é só de documentação.

---

## Testes criados

Todos em `./tests/`, importando a biblioteca pelo caminho de módulo, como
faz o restante da suíte.

| Arquivo | Conteúdo | Situação |
|---------|----------|----------|
| [tests/parsing_test.go](../tests/parsing_test.go) | Robustez de `FromString`: varredura de todas as mutações de 1 byte, hífen extra, fronteiras de tamanho, maiúsculas/minúsculas, valor devolvido junto com erro, vetor conhecido de `String()` | **2 falham** (BUG-01), 6 passam |
| [tests/fuzz_test.go](../tests/fuzz_test.go) | `FuzzFromString`: ausência de pânico, round-trip e coerência com `Import` | **Falha** (BUG-01) |
| [tests/layout_test.go](../tests/layout_test.go) | Trava o layout de bits com entropia constante (zero e todos-uns), versão/variante em 6 níveis incluindo desconhecidos, timestamp e campos sub-ms conferidos contra o relógio | Passam |
| [tests/import_test.go](../tests/import_test.go) | Vetor conhecido de `ImportBinary`, propagação de erro, faixas sub-ms por nível (documentando MELHORIA-04), reconstrução do instante nos três níveis | Passam |
| [tests/ordering_test.go](../tests/ordering_test.go) | Invariante de ordenação independente do relógio, equivalência entre ordem binária e de string, relatório de empates (MELHORIA-01), unicidade em 1.000.000 por nível, unicidade concorrente limpa sob `-race` | Passam |
| [tests/alloc_test.go](../tests/alloc_test.go) | Trava zero alocações em `Generate`, `FromString` e `ImportBinary`; no máximo uma em `GenerateString` | Passam |

### Como executar

Suíte completa (as falhas esperadas são as regressões do BUG-01 e o
`TestMonotonicity` preexistente):

```bash
go test ./tests/ -v
```

Apenas as regressões do defeito crítico:

```bash
go test ./tests/ -run 'TestFromStringNeverPanics|TestFromStringExtraHyphen' -v
```

Detector de corrida (acusa o BUG-04 na suíte antiga):

```bash
go test ./tests/ -race -run 'TestConcurrentUniqueness|TestMassConcurrent'
```

Fuzzing de `FromString`:

```bash
go test ./tests/ -run '^$' -fuzz FuzzFromString -fuzztime 60s
```

Sem os testes de massa (1.000.000 de amostras):

```bash
go test ./tests/ -short -v
```

---

## Ordem sugerida de execução

1. **BUG-01** — pânico com entrada externa; correção pequena, isolada e
   ainda por cima mais rápida. Nada mais deveria passar na frente.
2. **BUG-04** — sem ele, `-race` não roda, e o item 1 merece ser
   validado sob `-race`.
3. **PERF-01 + PERF-02** — mesma região de código, ganho medido de −10%
   serial e −39% paralelo, e mitigam parcialmente o SEC-01.
4. **BUG-02, BUG-03, BUG-05** — robustez do gerador; agrupáveis em uma
   passagem por `uuid.go`.
5. **SEC-01 + MELHORIA-01 (documentação)** — corrigir o que a
   documentação promete antes de mudar comportamento: avisar sobre
   previsibilidade e descrever a ordenação com precisão.
6. **BUG-06** — refazer as medições com aquecimento e atualizar as
   tabelas; convém fazer **depois** do item 3, para publicar os números
   já otimizados.
7. **BUG-07, BUG-08, BUG-09, BUG-10, MELHORIA-04** — documentação e
   higiene; baratos, agrupáveis.
8. **MELHORIA-02, MELHORIA-03, MELHORIA-01 (código)** — adições de API,
   sem urgência.
