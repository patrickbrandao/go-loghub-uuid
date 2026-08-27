# Especificação de Desenvolvimento — Biblioteca UUIDv7 Multinível

> Documento de implementação **agnóstico de linguagem**. Descreve, sem
> código, tudo o que é necessário para construir a biblioteca do zero em
> qualquer linguagem (Go, Rust, C, Java, Python, etc.). Um modelo de IA
> ou um desenvolvedor deve conseguir produzir uma implementação completa
> e correta seguindo apenas este texto.

---

## 1. Objetivo

Construir uma biblioteca leve e rápida para **gerar, converter e
interpretar** identificadores **UUIDv7** (RFC 9562), com **três níveis**
de precisão temporal embutida. A biblioteca deve:

- Gerar UUIDv7 do nível desejado, tanto em **binário (128 bits)** quanto
  em **string canônica**.
- Converter entre **string** e **binário** nos dois sentidos.
- **Importar** as propriedades de tempo de um UUID (segundos,
  milissegundos, microssegundos, nanossegundos).
- Ser **segura para concorrência** (um objeto criado no boot, usado por
  centenas de threads) e capaz de gerar **milhares de identificadores
  por milissegundo**.

---

## 2. Conceitos do UUIDv7

Um UUID tem **128 bits** = **16 bytes**, escritos em ordem de rede
(big-endian: o byte 0 é o mais significativo). A representação canônica
em texto tem **36 caracteres**: 32 dígitos hexadecimais minúsculos
agrupados como **8-4-4-4-12** e separados por hifens.

O UUIDv7 organiza esses 128 bits assim (numeração de bits da esquerda,
mais significativo, para a direita):

| Campo         | Tamanho | Posição (bits) | Conteúdo no padrão                          |
|---------------|---------|----------------|---------------------------------------------|
| `unix_ts_ms`  | 48 bits | 0–47           | Milissegundos desde a época Unix (UTC)      |
| `ver`         | 4 bits  | 48–51          | Versão, valor fixo **7** (binário `0111`)   |
| `rand_a`      | 12 bits | 52–63          | Aleatório no padrão                          |
| `var`         | 2 bits  | 64–65          | Variante, valor fixo **`10`** (binário)     |
| `rand_b`      | 62 bits | 66–127         | Aleatório no padrão                          |

Mapeando para os 16 bytes:

- **bytes 0..5** → `unix_ts_ms` (48 bits).
- **byte 6** → nibble alto = `ver` (`0x7`); nibble baixo = 4 bits mais
  altos de `rand_a`.
- **byte 7** → 8 bits baixos de `rand_a` (totalizando 12 bits).
- **byte 8** → 2 bits mais altos = `var` (`10`); 6 bits baixos = 6 bits
  mais altos de `rand_b`.
- **bytes 9..15** → 56 bits restantes de `rand_b` (totalizando 62 bits).

A ordenação cronológica é garantida porque os bits de tempo ocupam as
posições mais significativas: comparar dois UUIDs byte a byte equivale a
compará-los no tempo.

Com uma ressalva importante: a ordenação é cronológica **na resolução do
nível**, com desempate **aleatório** dentro do mesmo instante embutido —
não é monotonicidade estrita. Não há contador de desempate. Como gerar um
UUID é mais rápido que o passo do relógio da maioria dos hosts, dois
identificadores consecutivos frequentemente caem no mesmo instante e sua
ordem relativa passa a ser aleatória. No Nível 1 isso vale para
praticamente todo par consecutivo, porque a resolução é o milissegundo.
Uma implementação que precise de monotonicidade estrita deve acrescentar
um contador (RFC 9562, seção 6.2, método 1), ciente do custo de estado
compartilhado.

---

## 3. Os três níveis

A biblioteca aproveita os campos `rand_a` e `rand_b` para guardar
precisão **sub-milissegundo**, sempre preservando `ver=7` e `var=10`.

Decomposição do instante de criação:

- `unix_ts_ms` = nanossegundos_desde_epoch ÷ 1.000.000.
- resto_sub_ms = nanossegundos_desde_epoch **mod** 1.000.000 (faixa
  0..999.999).
- **microssegundos** = resto_sub_ms ÷ 1.000 (faixa **0..999**).
- **nanossegundos** = resto_sub_ms **mod** 1.000 (faixa **0..999**).

### Nível 1 — só milissegundos
- `rand_a` (12 bits): **aleatório**.
- `rand_b` (62 bits): **aleatório**.
- É o UUIDv7 padrão, 100% compatível com a RFC.

### Nível 2 — + microssegundos
- `rand_a` (12 bits): recebe o valor **microssegundos** (0..999). Como
  999 cabe em 10 bits, os 12 bits acomodam o valor diretamente, com os 2
  bits superiores em zero.
- `rand_b` (62 bits): **aleatório**.

### Nível 3 — + microssegundos e nanossegundos
- `rand_a` (12 bits): recebe **microssegundos** (0..999), igual ao nível 2.
- `rand_b` (62 bits): os **10 bits mais altos** (bits 66–75 do UUID, ou
  seja, os 6 bits baixos do byte 8 mais os 4 bits altos do byte 9)
  recebem **nanossegundos** (0..999); os **52 bits restantes** são
  **aleatórios**.

Como microssegundos ocupam `rand_a` (logo após os milissegundos) e
nanossegundos ocupam o topo de `rand_b`, a ordenação cronológica
permanece válida até a resolução de nanossegundo, com desempate
aleatório.

> Observação: a resolução real de nanossegundos depende do relógio do
> sistema operacional. A biblioteca grava o que o relógio fornece; em
> plataformas com granularidade mais grossa, o dígito de nanossegundo
> pode ser menos preciso, sem afetar a validade do UUID.

---

## 4. Algoritmo de geração

Entrada: o nível desejado. Saída: 16 bytes.

1. **Ler o relógio** em nanossegundos desde a época Unix (UTC).
2. Calcular `unix_ts_ms`, `microssegundos` e `nanossegundos` conforme a
   Seção 3.
3. Obter dois blocos de **64 bits aleatórios** (chamados aqui `r1` e
   `r2`) da fonte de entropia.
4. **Gravar `unix_ts_ms`** nos bytes 0..5 (big-endian, 48 bits).
5. **Montar `rand_a` (12 bits)**:
   - Níveis 2 e 3: `rand_a = microssegundos` (limitado a 12 bits).
   - Nível 1: `rand_a` = 12 bits baixos de `r1`.
6. Gravar **byte 6** = `0x70` **OU** os 4 bits altos de `rand_a`.
   Gravar **byte 7** = 8 bits baixos de `rand_a`.
7. **Montar `rand_b` (62 bits)**:
   - Nível 3: `rand_b` = (`nanossegundos` limitado a 10 bits, deslocado
     52 bits à esquerda) **OU** (52 bits baixos de `r2`).
   - Níveis 1 e 2: `rand_b` = 62 bits baixos de `r2`.
8. Gravar **byte 8** = `0x80` **OU** os 6 bits mais altos de `rand_b`
   (assim os 2 bits superiores formam a variante `10`).
   Gravar **bytes 9..15** com os 56 bits restantes de `rand_b`
   (big-endian).
9. Devolver os 16 bytes.

Para gerar a **string**, gerar o binário e convertê-lo (Seção 6).

Níveis desconhecidos devem ser tratados como Nível 1.

---

## 5. Algoritmo de importação (extrair tempo)

Entrada: um UUID (string ou binário). Saída: quatro componentes:
**segundos** (timestamp Unix), **milissegundos** (0..999),
**microssegundos** (0..999) e **nanossegundos** (0..999).

A importação é **cega quanto ao nível**: sempre lê os mesmos campos,
considerando os dados ali presentes como se fossem o tempo preciso
(para UUIDs de Nível 1, micro/nano serão bits aleatórios — isso é
esperado e aceito).

1. Se a entrada for string, convertê-la em binário (Seção 6).
2. Ler `unix_ts_ms` dos bytes 0..5 (48 bits, big-endian).
3. `segundos` = `unix_ts_ms` ÷ 1000.
4. `milissegundos` = `unix_ts_ms` **mod** 1000.
5. Ler `rand_a` (12 bits) = (4 bits baixos do byte 6, deslocados 8 à
   esquerda) **OU** (byte 7). Esse valor é **microssegundos**.
6. Ler os **10 bits mais altos de `rand_b`** = (6 bits baixos do byte 8,
   deslocados 4 à esquerda) **OU** (4 bits altos do byte 9). Esse valor
   é **nanossegundos**.
7. Devolver os quatro componentes.

---

## 6. Conversões string ⇄ binário

### Binário → string
- Para cada um dos 16 bytes, emitir dois dígitos hexadecimais
  minúsculos (alto e baixo).
- Inserir hifens **antes** dos bytes de índice 4, 6, 8 e 10.
- Resultado: 36 caracteres no formato `8-4-4-4-12`.

### String → binário
- Validar o tamanho (36 caracteres) e a presença de hifens nas posições
  8, 13, 18 e 23. Em caso de violação, retornar erro de formato.
- Percorrer a string ignorando os hifens; a cada par de dígitos
  hexadecimais, produzir um byte. Aceitar maiúsculas e minúsculas.
- Qualquer caractere não hexadecimal (fora dos hifens válidos) gera erro
  de formato.

> Opcional: a implementação pode também aceitar a forma sem hifens (32
> dígitos) e/ou entre chaves; não é obrigatório.

---

## 7. Contrato de API (conceitual)

Independente da linguagem, a biblioteca deve expor, com nomes
equivalentes:

- **Tipo do nível**: três valores — Nível 1, Nível 2, Nível 3.
- **Tipo do UUID binário**: 16 bytes.
- **Tipo de tempo importado**: estrutura com segundos, milissegundos,
  microssegundos e nanossegundos.
- **Objeto gerador**: criado uma vez, seguro para concorrência.
  - Construtor padrão (fonte de entropia rápida).
  - Construtor com fonte de entropia personalizada (que devolve 64 bits
    e é segura para concorrência) — útil para forçar entropia
    criptográfica.
  - Operação: gerar binário a partir de um nível.
  - Operação: gerar string a partir de um nível.
- **Funções/atalhos de pacote** que usam um gerador padrão interno para
  gerar binário e string sem instanciar nada (uso rápido).
- **Conversões**: binário → string; string → binário (com erro).
- **Importação**: a partir de string (com erro) e a partir de binário.
- **Acessores** opcionais: versão e variante de um UUID.

Mensagens de erro devem distinguir, no mínimo, "formato inválido".

---

## 8. Requisitos não-funcionais

### Desempenho
- A geração do binário deve ser **sem alocações de heap** no caminho
  quente (montar 16 bytes em buffer fixo).
- A geração da string deve usar buffer de 36 bytes preenchido
  diretamente, com no máximo uma alocação (a string final).
- Meta: **milhares de UUIDs por milissegundo** por núcleo em CPUs
  rápidas; ordem de **dezenas de nanossegundos** por UUID binário.

### Concorrência
- O gerador é criado **uma vez no boot** e compartilhado por **centenas
  de threads** simultâneas.
- A fonte de entropia padrão **não deve ter contenção de lock global**.
  Recomendação: manter um **conjunto de geradores pseudoaleatórios
  rápidos por thread** (por exemplo, um pool), cada um semeado uma única
  vez a partir de uma fonte de alta qualidade (gerador criptográfico do
  sistema). Em tempo de execução, cada thread avança seu PRNG localmente.
- Alternativa aceitável: um PRNG global já seguro para concorrência e de
  baixa contenção, desde que atinja a meta de desempenho.

### Qualidade da aleatoriedade
- A entropia de **semeadura** deve vir de uma fonte forte do sistema.
- A entropia de **execução** pode ser pseudoaleatória rápida (prioriza
  velocidade). A biblioteca deve permitir trocar a fonte por uma
  criptográfica via o construtor personalizado, para quem precisar de
  imprevisibilidade total.

### Robustez
- O relógio pode, raramente, retroceder. A implementação básica não
  precisa tratar isso, mas deve documentar a possibilidade. Uma extensão
  opcional é manter monotonicidade por contador, ao custo de estado
  compartilhado.
- O relógio pode estar ajustado para **antes da época Unix**. A
  decomposição do instante deve garantir que os campos sub-milissegundo
  permaneçam em 0..999 nesse caso (por exemplo, fixando o piso do
  timestamp na própria época); aritmética com resto de números negativos
  produz valores fora da faixa que corrompem o UUID silenciosamente.
- Se a leitura do relógio for feita como um único inteiro de
  nanossegundos com 64 bits, ela satura em 2262-04-11. Ler segundos e
  fração do segundo em separado evita esse limite.
- O analisador de string **jamais** pode ler fora dos limites da entrada,
  qualquer que seja o conteúdo: ele recebe dado externo. Decodificar a
  partir de deslocamentos fixos e conhecidos (em vez de percorrer a
  string pulando separadores) elimina a classe inteira de erro.
- A fonte de entropia deve ser validada na construção do gerador, não na
  primeira geração; e um gerador obtido pelo valor zero do tipo não pode
  derrubar o processo.

---

## 9. Casos de teste obrigatórios

1. **Versão e variante**: todo UUID gerado (qualquer nível) tem `ver=7`
   e `var=10`.
2. **Round-trip de conversão**: binário → string → binário devolve os
   mesmos 16 bytes, para milhares de amostras.
3. **Faixas sub-ms**: para Níveis 2 e 3, os microssegundos importados
   ficam em 0..999; para Nível 3, os nanossegundos importados ficam em
   0..999 (validar em centenas de milhares de amostras).
4. **Importação coerente**: o instante reconstruído a partir de
   segundos+ms+us+ns cai dentro do intervalo de tempo medido em torno da
   geração (com tolerância de 1 ms).
5. **Strings inválidas**: tamanho errado, hifens errados ou dígitos não
   hexadecimais produzem erro de formato.
6. **Ordenação**: sempre que o instante embutido de B for maior que o de
   A, a string de B tem de ser maior que a de A (e o binário também), nos
   três níveis. Este é o teste correto. **Não** testar contando
   "regressões em uma sequência fechada com limiar tolerado": isso mede a
   resolução do relógio do host, não a biblioteca — sem contador de
   desempate, a maioria dos pares consecutivos cai no mesmo instante e é
   ordenada aleatoriamente. Se for desejável um teste sobre o relógio
   real, inserir uma pausa maior que a resolução do relógio entre as
   gerações e então exigir ordem estrita.
7. **Concorrência**: gerar 1.000.000 de UUIDs distribuídos por centenas
   de threads não causa erro nem corrupção, e o resultado permanece
   válido. A própria suíte deve estar limpa sob detector de corrida —
   nada de sumidouro global escrito por várias threads.
8. **Robustez do analisador**: nenhuma entrada de qualquer tamanho ou
   conteúdo pode causar acesso fora dos limites. Cobrir, no mínimo, toda
   mutação de um byte sobre uma string canônica válida (inclusive
   separadores extras em posições inesperadas) e, se a linguagem
   oferecer, uma campanha de *fuzzing*.
9. **Bordas do gerador**: fonte de entropia nula rejeitada na
   construção; gerador obtido pelo valor zero do tipo não derruba o
   processo; níveis desconhecidos se comportam exatamente como o
   Nível 1.

---

## 10. Benchmark obrigatório

Medir o tempo para gerar **1.000.000** de UUIDs em cada cenário e
reportar tempo total, nanossegundos por UUID e UUIDs por milissegundo:

- Nível 1 binário, Nível 2 binário, Nível 3 binário.
- Nível 1 string, Nível 2 string, Nível 3 string.
- Variante concorrente do Nível 3 (centenas de threads), reportando
  throughput agregado.

O benchmark deve impedir que o compilador elimine o trabalho (consumir
os resultados em uma variável "sumidouro").

---

## 11. Organização de arquivos sugerida

- **Raiz**: apenas o necessário para produção (código da biblioteca,
  manifesto de build, README, mapa do projeto, licença).
- **Subpasta de documentação**: guias de uso rápido, uso completo, de
  testes/benchmark e este documento.
- **Subpasta de testes**: testes funcionais, benchmarks e um executável
  autônomo de benchmark em massa, todos importando a biblioteca pelo seu
  caminho público (como um consumidor externo faria).
