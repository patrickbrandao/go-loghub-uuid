# Especificação de Desenvolvimento — Biblioteca UUID Completa e Multinível (RFC 9562)

> Documento de implementação **agnóstico de linguagem**. Descreve, sem
> depender de bibliotecas externas ou de recursos exclusivos do Go, tudo
> o que é necessário para construir a biblioteca do zero em qualquer
> linguagem (Go, Rust, C, C++, Java, C#, Python, etc.).
>
> Um desenvolvedor ou modelo de IA deve conseguir produzir uma
> implementação completa, interoperável, de alto desempenho e
> estritamente correta seguindo apenas este texto — **sem cometer
> nenhum dos erros históricos já encontrados e corrigidos neste projeto**.

---

## 1. Objetivo e Escopo da Biblioteca

Construir uma biblioteca leve, de altíssimo desempenho (dezenas de
nanossegundos por identificador, zero alocações de heap no caminho
quente) e segura para concorrência pesada, cobrindo todo o padrão
**RFC 9562** com extensão de precisão temporal sub-milissegundo:

1. **UUIDv7 Multinível**:
   - **Nível 1**: UUIDv7 padrão RFC 9562 com carimbo de milissegundos
     Unix e 74 bits de entropia.
   - **Nível 2**: UUIDv7 com carimbo de milissegundos e
     **microssegundos** embutidos em `rand_a` (62 bits de entropia).
   - **Nível 3**: UUIDv7 com carimbo de milissegundos,
     **microssegundos** em `rand_a` e **nanossegundos** no topo de
     `rand_b` (52 bits de entropia).
2. **Todas as demais versões da RFC 9562**:
   - **Versão 1**: Baseada em carimbo de tempo gregoriano (100 ns desde
     1582), sequência de relógio de 14 bits e nó MAC/aleatório de 48 bits.
   - **Versão 2**: DCE 1.1 Security, associando domínio de segurança
     (Pessoa, Grupo, Org) e identificador local de 32 bits a um carimbo
     gregoriano.
   - **Versão 3**: Baseada em hash MD5 sobre um espaço de nomes e nome.
   - **Versão 4**: 122 bits puramente aleatórios.
   - **Versão 5**: Baseada em hash SHA-1 sobre um espaço de nomes e nome.
   - **Versão 6**: Tempo gregoriano reordenado para ordenação k-sortable.
   - **Versão 8**: Formato livre / personalizado ou 122 bits aleatórios.
3. **Conversão e Análise (Parsing) Segura**:
   - Analisador estrito: formato canônico `8-4-4-4-12`.
   - Analisador permissivo: 4 formatos aceitos (canônico com hífens, sem
     hífens com 32 dígitos, entre chaves `{...}`, e prefixo URN
     `urn:uuid:...`).
   - Algoritmo de parsing imune a pânicos e leituras fora dos limites
     (out-of-bounds).
4. **Inspeção e Extração**:
   - Extração de versão, variante, instante temporal (Unix/Gregorian),
     sequência de relógio, nó de rede, domínio e ID local.
   - Duas leituras de tempo do UUIDv7 com políticas opostas, ambas
     normativas: a leitura por nível, que devolve um instante e descarta
     o que não pode ser tempo, e a extração completa, que devolve os
     campos crus sem julgar a origem dos bits (seção 7).
5. **Construção a Partir de um Instante Explícito**:
   - É a operação **inversa** da extração do item 4: ali se lê o tempo de
     um identificador, aqui se constroem identificadores para um tempo.
     Toda ela recebe o instante por parâmetro, e nenhuma parte dela é
     chamada pela geração pelo relógio.
   - **Fronteiras** (consulta por intervalo): o menor e o maior UUIDv7
     que a biblioteca poderia gerar naquele instante e naquele nível,
     para que uma janela de tempo seja respondida pelo índice da própria
     chave primária, sem coluna nem índice de carimbo temporal.
   - **Geração por instante**: um UUIDv7 daquele instante com os bits
     livres sorteados, para reprocessar histórico, semear dados de teste
     e importar registros antigos preservando a ordenação da chave.
   - Aplica-se **somente ao UUIDv7**, a única versão com época Unix. As
     versões 1, 2 e 6 ficam de fora por decisão registrada na seção 11.3.
6. **Serialização e Integração**:
   - Serialização de texto e JSON como string canônica entre aspas.
   - Suporte a identificadores nulos em banco de dados (`NullUUID`).
   - Operações de ordenação lexicográfica e constantes `Nil` e `Max`.

---

## 2. Conceitos Gerais e Layout de Bits

Um UUID é composto exatamente por **128 bits** = **16 bytes**, dispostos
em ordem de rede (*big-endian*, byte 0 mais significativo).

### 2.1 Campos Fixos Universais (RFC 9562)

Em qualquer versão do padrão, dois campos são invioláveis:

- **Versão (`ver`, 4 bits)**: Ocupa o nibble mais alto do **byte 6**
  (bits 48 a 51 do UUID). Valores válidos: `1` a `8`.
- **Variante (`var`, 2 bits)**: Ocupa os 2 bits mais significativos do
  **byte 8** (bits 64 e 65 do UUID). O padrão RFC 9562 exige variante
  `10` em binário (`0b10` nos bits superiores, correspondendo à máscara
  `0x80` ou nibble alto `8`, `9`, `A` ou `B`).

### 2.2 Representação Canônica em Texto

Consiste em **36 caracteres** hexadecimais minúsculos agrupados como
**`8-4-4-4-12`** com hifens nas posições 8, 13, 18 e 23 (índices
iniciados em zero):

```text
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   time_low    | - | time_mid  | - |ver|time_hi| - |var|clk_seq| - |   node    |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

---

## 3. Especificação Detalhada do UUIDv7 Multinível

O UUIDv7 dedica os 48 bits iniciais ao carimbo Unix em milissegundos,
garantindo ordenação temporal natural por comparação byte a byte.

### 3.1 Distribuição de Bits por Nível

| Campo         | Tamanho | Posição (bits) | Bytes   | Nível 1 (RFC)        | Nível 2 (+us)          | Nível 3 (+us +ns)      |
|:--------------|:--------|:---------------|:--------|:---------------------|:-----------------------|:-----------------------|
| `unix_ts_ms`  | 48 bits | 0–47           | 0..5    | ms Unix (UTC)        | ms Unix (UTC)          | ms Unix (UTC)          |
| `ver`         | 4 bits  | 48–51          | 6 (alto)| `0x7`                | `0x7`                  | `0x7`                  |
| `rand_a`      | 12 bits | 52–63          | 6(b)..7 | aleatório            | microssegundos (0..999)| microssegundos (0..999)|
| `var`         | 2 bits  | 64–65          | 8 (alto)| `0b10`               | `0b10`                 | `0b10`                 |
| `rand_b`      | 62 bits | 66–127         | 8(b)..15| aleatório (62 bits)  | aleatório (62 bits)    | 10b ns (0..999) + 52b  |

### 3.2 Aritmética Temporal Segura (Evitando Armadilhas de Relógio)

A decomposição do instante em milissegundos, microssegundos e
nanossegundos deve obedecer a regras estritas de robustez:

1. **Prevenção de estouro em 2038 e 2262**:
   - **NUNCA** leia o relógio como um único número inteiro de
     nanossegundos de 64 bits (`UnixNano()`). Esse valor satura e
     transborda em **2262-04-11**.
   - Obtenha o tempo lendo **duas partes separadas**: segundos Unix
     inteiros de 64 bits (`sec`) e nanossegundos residuais dentro do
     segundo atual (`0 <= nsec < 1.000.000.000`).
2. **Proteção contra instantes anteriores a 1970 (Época Unix)**:
   - Se o relógio do sistema estiver configurado para uma data anterior a
     1970-01-01 (`sec < 0`), a aritmética de divisão e módulo padrão de
     várias linguagens produz restos negativos, corrompendo os bytes do
     UUID.
   - **Regra**: se `sec < 0`, fixe `unix_ts_ms = 0`, `micro = 0` e
     `nano = 0`. Um relógio quebrado ou pré-época não pode gerar campos
     fora da faixa `0..999`.
   - **A verificação de sinal vem antes de qualquer conversão para tipo
     sem sinal.** Em aritmética sem sinal o valor negativo não existe:
     ele vira um número enorme, a guarda nunca dispara e o carimbo sai
     grande e errado, sem erro nenhum. É a mesma armadilha do item 11 da
     seção 9, pelo mesmo mecanismo. Em linguagens onde a aritmética sem
     sinal é o caminho natural, como C e Rust, este é o ponto exato em
     que uma transcrição literal da fórmula do item 4 perde a proteção.
   - **Forma equivalente aceita.** Decidir o piso sobre `unix_ts_ms` já
     calculado, em aritmética **com** sinal, é equivalente e também
     conforme: como o item 1 garante `0 <= nsec < 1.000.000.000`,
     qualquer `sec` negativo produz `unix_ts_ms` negativo, e o piso
     dispara igual. As duas formas coexistem na implementação de
     referência, uma no caminho quente e outra no lado do parâmetro
     (seção 3.5), e a diferença entre elas é de custo, não de resultado.
3. **Proteção contra instantes muito além da faixa do UUIDv7**:
   - A decomposição multiplica os segundos por mil. Quando o instante vem
     do relógio do sistema isso é inofensivo, mas quando vem **por
     parâmetro** (seção 3.5) o tipo de data da linguagem costuma comportar
     anos muito além dos 48 bits de milissegundos do UUIDv7, e o produto
     **estoura o inteiro com sinal de 64 bits**.
   - O estouro dá a volta trocando o sinal, e o resultado é pior que um
     valor grande errado: um instante remoto no **futuro** vira negativo e
     cai no piso da época, e um remoto no **passado** vira positivo e
     produz um carimbo enorme. Nos dois casos a regra do item 2 deixa de
     disparar, porque o sinal já foi invertido.
   - **Regra**: decida a saturação sobre os **segundos**, antes da
     multiplicação, **nas duas pontas**: o piso do item 2 e o teto deste
     item são a mesma guarda, aplicada aos dois sinais. Ver a seção 3.5,
     onde essa saturação é obrigatória.
4. **Decomposição pura**, em aritmética **com sinal**, convertida para
   os campos sem sinal **somente depois** de o piso do item 2 e a
   saturação do item 3 terem sido aplicados:
   - `unix_ts_ms = (sec * 1000) + (nsec / 1_000_000)`, gravado nos 48
     bits do campo apenas após as guardas acima.
   - `sub_ms = nsec % 1_000_000`
   - `micro = sub_ms / 1000` (faixa 0..999, cabe em 10 bits; campo tem 12 bits)
   - `nano = sub_ms % 1000` (faixa 0..999, cabe em 10 bits; campo tem 10 bits)

### 3.3 Sorteio de Entropia Otimizado e Normativo

- **Nível 2 e Nível 3**: o campo `rand_a` carrega os microssegundos
  (não é aleatório). Portanto, a biblioteca **DEVE sortear exatamente uma
  única palavra de 64 bits aleatórios (`r2`)**. O sorteio de uma segunda
  palavra é desperdício de CPU e de entropia criptográfica do sistema.
- **Nível 1 (e níveis desconhecidos)**: sortear **duas palavras de 64 bits
  (`r1` e `r2`)**, usando os 12 bits inferiores de `r1` para preencher
  `rand_a`.
- Essa economia é normativa e deve ser travada por testes de contagem.

### 3.4 Ordenação e Desempate (Sem Contador Monotônico)

A garantia de ordenação do UUIDv7 multinível é **exatamente esta, e não
mais que esta**:

- Se o instante embutido de `B` for estritamente maior que o de `A`,
  então `B > A` tanto na comparação byte a byte quanto na comparação
  lexicográfica das strings canônicas.
- Se `A` e `B` carregarem o **mesmo instante embutido**, a ordem entre
  eles é **aleatória**, decidida pelos bits de entropia. Não há contador,
  nem sequência, nem qualquer outro desempate determinístico.

O instante embutido tem a resolução do nível: milissegundo no Nível 1,
microssegundo no Nível 2, nanossegundo no Nível 3. Como gerar um UUID
custa dezenas de nanossegundos — menos que o passo do relógio da maioria
dos hosts —, **empates entre gerações consecutivas são o caso comum**,
não a exceção: são universais no Nível 1 e frequentes nos demais.

**Regra normativa**: a implementação **NÃO DEVE** introduzir contador
monotônico, nem os métodos 1 ou 2 da RFC 9562 §6.2, no gerador padrão.
Ambos exigem estado compartilhado entre threads, e o custo sob
concorrência inviabiliza o objetivo de desempenho da seção 1. O
raciocínio completo e a medição estão na seção 11.

**Consequência para testes**: é proibido escrever teste de ordenação que
gere UUIDs em laço apertado e conte "regressões" contra um limite
tolerado. Esse teste mede a resolução do relógio do host, não a
biblioteca, e falha de forma permanente em hosts com relógio de
microssegundo. Teste a invariante acima fazendo o instante avançar de
verdade entre as gerações.

---

### 3.5 Fronteiras de Tempo para Consulta por Intervalo

O motivo prático de adotar UUIDv7 como chave primária é responder a uma
janela de tempo com o índice da própria chave, sem coluna nem índice de
carimbo temporal. Para isso a implementação **DEVE** oferecer as duas
fronteiras de um instante, e elas dependem do nível.

Sejam `ms`, `micro` e `nano` os campos produzidos pela decomposição da
seção 3.2 aplicada ao instante `t`. Define-se um valor de preenchimento
`fill`: **todos os bits em zero** para a fronteira inferior e **todos os
bits em um** para a superior. A fronteira é então montada exatamente
como a geração da seção 3.1, trocando a entropia por `fill`:

| Nível    | `rand_a` (12 bits) | `rand_b[61:52]` | `rand_b[51:0]` |
|----------|--------------------|-----------------|----------------|
| `Level1` | `fill`             | `fill`          | `fill`         |
| `Level2` | `micro`            | `fill`          | `fill`         |
| `Level3` | `micro`            | `nano`          | `fill`         |

Níveis desconhecidos são tratados como `Level1`, como na geração.

**Versão e variante são preservadas nas duas fronteiras**, e é isso que
as torna limites corretos. O byte 6 recebe `0x70 | (rand_a >> 8)` e o
byte 8 recebe `0x80 | (rand_b >> 56)`, de modo que a fronteira superior
do Nível 1 termina em `0x7F` no byte 6 e `0xBF` no byte 8, não em `0xFF`.
Como **todo** UUIDv7 válido tem o nibble de versão em `7` e o byte 8 na
faixa `0x80..0xBF`, e como a comparação é byte a byte a partir do mais
significativo, as duas fronteiras de fato contêm todos os valores
geráveis naquele instante e naquele nível.

**Regra normativa — a precisão da fronteira é a do nível.** No Nível 1 a
faixa delimita o milissegundo inteiro; no Nível 2, o microssegundo; no
Nível 3, o nanossegundo.

**Regra normativa — fronteiras de níveis distintos não compõem.** Os
bits abaixo do milissegundo significam coisas diferentes em cada nível,
então uma fronteira calculada para um nível só delimita identificadores
gravados naquele mesmo nível. Uma fronteira superior de Nível 3 fica
abaixo de parte dos identificadores de Nível 1 do mesmo instante, porque
nela `rand_a` vale os microssegundos reais (0 a 999) enquanto no Nível 1
é aleatório (0 a 4095). A implementação **DEVE** documentar isso de
forma destacada: é o erro mais provável do chamador, e ele se manifesta
como linhas faltando, sem erro nenhum.

**Regra normativa — saturação nas duas pontas.** Diferentemente da
geração, que só tem piso na época, as fronteiras **DEVEM** saturar
também no teto:

- Instante anterior a `1970-01-01T00:00:00Z`: resultado igual ao da
  própria época, com os campos abaixo do milissegundo zerados. É o mesmo
  comportamento da seção 3.2, e a coerência com a geração é obrigatória.
- Instante posterior a `10889-08-02T05:31:50.655999999Z`, o último que
  cabe em 48 bits de milissegundos: resultado igual ao desse instante,
  **com `micro` e `nano` em 999**. Zerá-los faria a fronteira regredir ao
  cruzar a borda, quebrando a monotonicidade.

Truncar os bits excedentes, como o empacotamento por deslocamento faria
naturalmente, é **proibido**: a fronteira daria a volta e a consulta
passaria a devolver as linhas erradas em silêncio. A propriedade a
preservar é que a fronteira nunca regride quando o instante avança.

**Cuidado de implementação.** É aqui que a regra 3 da seção 3.2 passa a
valer: como o instante vem por parâmetro e não do relógio do sistema, ele
pode estar longe o bastante para estourar a multiplicação por mil. A
saturação **DEVE** ser decidida sobre os segundos, antes dela, nas duas
direções.

**Intervalo semiaberto.** A conveniência que devolve o par de um
intervalo `[from, to)` usa a fronteira **inferior** nas duas pontas:
`lo = MinAt(nível, from)` e `hi = MinAt(nível, to)`. Isso corresponde
diretamente a `WHERE id >= lo AND id < hi`. Ela não reordena os
argumentos: `to` anterior a `from` produz um intervalo vazio.

**Esta funcionalidade não toca o caminho quente.** As fronteiras recebem
o instante por parâmetro, são funções de pacote separadas e a geração
nunca as chama. Ver a seção 11.3.

---

### 3.6 Geração a Partir de um Instante Explícito

A biblioteca **DEVE** oferecer a geração de um UUIDv7 para um instante
informado pelo chamador, no lugar do instante atual. É o que fecha a
assimetria com a extração da seção 7: sem ela, quem reprocessa um
histórico, semeia dados de teste ou importa registros antigos
preservando a ordenação da chave monta os 16 bytes à mão.

**Regra normativa — escopo.** Esta operação vale **apenas para o
UUIDv7**. As versões 1, 2 e 6 usam a época gregoriana e têm a unicidade
garantida pelo piso de relógio por sequência da seção 4.2, segundo o qual
os instantes emitidos com cada sequência são estritamente crescentes
durante toda a vida do processo. Uma função que aceite um instante
arbitrário do chamador **fura essa invariante** e pode reemitir um UUIDv1
já produzido. O UUIDv7 não tem estado compartilhado nem piso, então nada
se perde ao aceitar o instante.

**Regra normativa — os bits livres são sorteados.** Preenchidos os campos
de tempo do nível, os bits restantes recebem entropia: 74 no Nível 1, 62
no Nível 2 e 52 no Nível 3. Duas chamadas com o mesmo instante **DEVEM**
devolver identificadores diferentes. É um gerador, não um construtor
determinístico: a forma determinística de um instante já existe na seção
3.5, e expor uma segunda com o verbo "gerar" convidaria ao pior
mal-entendido possível, o de usar como identificador único algo que
colide na primeira repetição de instante.

A entropia **DEVE** vir do mesmo gerador do resto da biblioteca, para
que uma fonte criptográfica configurada pelo chamador continue valendo
aqui. O consumo por nível é o mesmo da seção 3.3: uma palavra de 64 bits
nos níveis 2 e 3, duas no Nível 1 e nos níveis desconhecidos.

**Regra normativa — mesma decomposição das fronteiras.** O instante é
decomposto pela regra da seção 3.5, com saturação nas duas pontas, e não
pela decomposição do caminho quente, que só tem piso. Os dois motivos
são os mesmos: o instante vem por parâmetro e pode estourar a
multiplicação por mil, e um carimbo que dá a volta destrói a ordenação
que é a razão de existir do UUIDv7.

**Regra normativa — um só empacotamento.** O empacotamento dos 16 bytes
**DEVE** existir em um único lugar, compartilhado pelas fronteiras da
seção 3.5 e pela geração desta seção, parametrizado pelos bits livres:
constantes em um caso, sorteados no outro. A geração pelo relógio pode
manter cópia própria, pela regra de custo da seção 11.1, mas as duas
construções a partir de instante **não** podem divergir uma da outra.
Duas cópias dessa aritmética divergindo é um defeito que só aparece em
produção, no nível menos usado.

**Consequência para a unicidade.** Como o instante deixa de vir do
relógio, nada impede o chamador de gerar em volume para um único
instante, e aí a margem passa a ser só a dos bits livres. No Nível 3 são
52 bits, o que põe a probabilidade de colisão na casa de um em dois
elevado a 26 gerações **para o mesmo nanossegundo**. É folgado na
prática e **DEVE** estar documentado, porque a geração pelo relógio
nunca expõe o chamador a essa escolha.

A forma da API, a aridade e a nomenclatura estão na seção 11.3.

---

## 4. Especificação das Demais Versões (1, 2, 3, 4, 5, 6 e 8)

### 4.1 Versão 1 e Versão 6 (Tempo Gregoriano)

Utilizam o relógio gregoriano em tiques de **100 nanossegundos** desde a
reforma do calendário gregoriano em **1582-10-15T00:00:00Z**.

- **Constante de deslocamento gregoriano**:
  `0x01B21DD213814000` = `122.192.928.000.000.000` tiques até a época
  Unix (1970-01-01).
- **Cálculo do carimbo de 60 bits (`now`)**:
  `now = (uint64(sec) * 10_000_000) + (uint64(nsec) / 100) + gregorianOffset`

#### Estrutura da Versão 1:
- `time_low` (32 bits, bytes 0..3): 32 bits baixos de `now`.
- `time_mid` (16 bits, bytes 4..5): bits 32..47 de `now`.
- `time_hi_and_ver` (16 bits, bytes 6..7): 4 bits de versão (`0x1`) + bits 48..59 de `now`.
- `clock_seq_and_var` (16 bits, bytes 8..9): 2 bits de variante (`0b10`) + 14 bits de sequência.
- `node` (48 bits, bytes 10..15): identificador de nó (endereço MAC ou pseudoaleatório).

#### Estrutura da Versão 6 (K-Sortable Gregoriano):
Reorganiza os 60 bits de tempo em ordem natural do mais ao menos
significativo, conforme a RFC 9562 §5.6 (`time_high`, `time_mid`, `ver`,
`time_low`):
- `time_high` (32 bits, bytes 0..3): bits 59..28 de `now` (`now >> 28`).
- `time_mid` (16 bits, bytes 4..5): bits 27..12 de `now` (`now >> 12`).
- Byte 6: nibble alto = versão `0x6`; nibble baixo = bits 11..8 de `now`.
- Byte 7: bits 7..0 de `now`. Juntos, o nibble baixo do byte 6 e o byte 7
  formam `time_low` (12 bits, `now & 0x0FFF`).
- Bytes 8..15: idênticos à versão 1 (sequência de relógio e nó).

A leitura inversa recompõe `now` como
`time_high << 28 | time_mid << 12 | time_low`.

#### Conversão Inversa (Gregoriano para Unix)

Recompor `now` devolve tiques desde 1582; a leitura das versões 1 e 6
ainda precisa voltar para a época Unix, e é essa etapa que a fórmula da
ida não cobre. A conversão inversa **DEVE** produzir o par canônico
`(sec, nsec)` com `0 <= nsec < 1.000.000.000`, inclusive para instantes
anteriores à época Unix, que existem no campo: ele começa em 1582.

1. `ticks = now - gregorianOffset`, em inteiro **com sinal** de 64 bits.
   O resultado é negativo para qualquer instante anterior a 1970.
2. `sec = floorDiv(ticks, 10_000_000)` e
   `rem = floorMod(ticks, 10_000_000)`, com divisão **euclidiana**, isto
   é, resto sempre não negativo. Para `ticks` negativo com resto
   diferente de zero, isso significa um segundo a menos e o resto somado
   ao divisor.
3. `nsec = rem * 100`.

A resolução devolvida é a do campo, cem nanossegundos: os dois dígitos
decimais finais de `nsec` são sempre zero.

**Armadilha de linguagem.** A divisão truncada em direção a zero, que é
o padrão em C, Go, Java e Rust, produz resto **negativo** quando `ticks`
é negativo, e portanto nanossegundos negativos: um tique antes da época
Unix sai como `(0, -100)` em vez de `(-1, 999.999.900)`. Esse par só é
utilizável se a construção de data da linguagem normalizar componentes
negativas, como `time.Unix` faz em Go; uma linguagem que não normalize
produz instante errado ou rejeita a entrada, e o defeito aparece
exatamente na borda pré-1970 que o caso 3 da seção 10 manda testar. A
divisão euclidiana acima remove a dependência: o par é canônico por
construção, e um chamador que consuma `sec` e `nsec` diretamente, sem
passar por uma construção de data, recebe valores corretos.

O tipo de tempo gregoriano e as suas duas conversões, para o par Unix e
para o instante da linguagem, são públicos (seção 7).

### 4.2 Estado Monotônico Compartilhado (v1, v2 e v6)

A geração de UUIDs baseados em tempo requer sincronização segura:

1. **Adiantamento de relógio em alta frequência (Drift)**:
   - Se sucessivas chamadas ocorrerem no mesmo tique de 100 ns, a
     biblioteca **avança o relógio interno em 1 tique (+100 ns) por
     geração**, em vez de bloquear em espera (*sleep*).
   - Isso garante ordenação estrita e unicidade absoluta mesmo em
     geração massiva sob lock.
2. **Regra Anti-Repetição em `SetNodeID` (Evitando Repetição de UUID)**:
   - Uma falha comum em implementações é resetar o último instante
     registrado (`lastClockTime = 0`) ao trocar ou reaplicar o identificador
     de nó.
   - **Regra**: Se o chamador reaplicar o mesmo nó já em uso, ou trocar
     de nó, o carimbo de tempo **NÃO PODE ser zerado**. Zerar o carimbo
     permite que a próxima chamada leia o relógio físico atual que pode
     estar atrás do tempo acumulado por adiantamento, gerando colisões de
     identificadores.
3. **Piso de Relógio por Sequência (Troca de Sequência sem Repetição)**:
   - Zerar o piso do relógio em **qualquer** troca de sequência também
     repete UUIDs, porque o piso é a única proteção contra reemitir um
     instante: com ele zerado, duas gerações que caiam no mesmo tique de
     100 ns com a mesma sequência devolvem o mesmo instante e, com o
     mesmo nó, o mesmo UUID. Basta o chamador trocar de sequência e
     voltar entre as duas gerações.
   - Isso não é hipótese. O pacote `github.com/google/uuid` v1.6.0 zera
     o piso sempre que a sequência muda de valor, e a repetição foi
     **medida** em `tests/compare`: cerca de 7% das tentativas do
     roteiro acima, num Apple M2.
   - **Atenção ao mecanismo, que é fácil de descrever errado.** A
     repetição vem da janela do mesmo tique, **não** de adiantamento
     acumulado. Uma implementação que zera o piso normalmente não
     adianta o relógio: quando o instante não avança, ela incrementa a
     sequência em vez de avançar o tempo, e assim nunca fica adiantada.
     São escolhas que andam juntas. Já uma implementação que **adianta**
     o relógio, como esta (item 2 da seção 4.2 e o adiantamento descrito
     adiante), não pode zerar o piso de jeito nenhum: ali a janela de
     repetição deixaria de ser um tique e passaria a ser todo o
     adiantamento acumulado.
   - **Invariante obrigatória**: para cada sequência de relógio, os
     instantes emitidos com ela são **estritamente crescentes durante
     toda a vida do processo**. Como o par (instante, sequência) nunca se
     repete, nenhum UUIDv1/v6 se repete, independentemente de trocas de
     nó ou de sequência.
   - **Implementação**: guardar, por sequência já usada, o último instante
     emitido. Ao trocar de sequência, salvar o último instante da
     sequência que sai e adotar como piso o último instante da sequência
     que entra (zero se inédita). Só a entrada em uma sequência inédita
     descarta o adiantamento acumulado.
   - O pedido de sequência aleatória (`-1`) **DEVE** sortear uma sequência
     ainda não usada no processo (sorteio de 14 bits e avanço até a
     primeira livre), para que ele sirva de ressincronização
     determinística com o relógio do sistema. Se todas as 16384 estiverem
     usadas, aceitar o sorteio; o piso por sequência continua impedindo a
     repetição.
4. **Identificador de Nó**:
   - A biblioteca **NÃO lê interfaces de rede**. O nó padrão é sorteado
     uma única vez, a partir da fonte criptográfica do sistema, com o
     **bit 0 do byte 10 (bit multicast) setado em 1**, indicando que não
     é um endereço MAC físico real. É a forma recomendada pela RFC 9562
     §6.10 para quando o endereço MAC não está disponível ou não é
     desejado; ela também evita expor a identidade da máquina.
   - O chamador pode fornecer um nó próprio (por exemplo, um MAC real
     lido fora da biblioteca) por `SetNodeID`, que copia os 6 primeiros
     bytes sem alterá-los. A responsabilidade pelo bit multicast, nesse
     caso, é do chamador.
   - **Regra normativa — recusa do nó curto.** Uma entrada com menos de
     seis bytes **DEVE** ser recusada sem alterar o estado, e a recusa
     **DEVE** ser observável pelo chamador (retorno booleano falso).
     Preencher com zeros, aceitar parcialmente ou entrar em pânico são
     proibidos: os dois primeiros produziriam um nó silenciosamente
     diferente do pedido, e o terceiro derrubaria o processo por um erro
     que o chamador consegue tratar. Uma entrada com mais de seis bytes é
     aceita, e só os seis primeiros são copiados.

### 4.3 Versão 2 (DCE 1.1 Security)

- **Objetivo**: Identificar uma credencial ou entidade (Pessoa, Grupo,
  Organização) dentro de um domínio de segurança, **não um evento
  individual**.
- **Modificações sobre o UUIDv1**:
  - Bytes 0..3 (`time_low`): substituídos pelo identificador local de
    32 bits (ex.: UID ou GID do sistema operacional).
  - Byte 9 (`clock_seq_low`): substituído pelo domínio (Person=`0`,
    Group=`1`, Org=`2`).
  - O carimbo temporal preserva apenas os 28 bits altos do tempo
    gregoriano (resolução aproximada de 7 minutos = ~429 segundos).
- **Propriedade Normativa**:
  - Chamadas sucessivas com o mesmo domínio e mesmo identificador dentro
    do mesmo intervalo de ~7 minutos **devolvem intencionalmente o mesmo
    UUID**.
- **Inspeção de Sequência de Relógio**:
  - Ao inspecionar `ClockSequence()` de um UUIDv2, devolver **apenas os 6
    bits mais altos** (faixa 0..63 do byte 8), pois o byte 9 é ocupado
    pelo domínio.
- **Conveniências não normativas**: a biblioteca **PODE** oferecer
  atalhos que preencham o identificador local com o usuário ou o grupo
  do processo, lidos do sistema operacional. Uma reimplementação que os
  omita continua conforme.
  - **Advertência de portabilidade.** Em sistemas sem identificador
    numérico de usuário e de grupo, a consulta devolve `-1`, e o
    identificador local gravado passa a ser `0xFFFFFFFF` para todos os
    processos: é o que acontece em Windows com a implementação de
    referência. Nesse ambiente o atalho deixa de identificar o
    principal, que é o propósito inteiro da versão 2, e o identificador
    **DEVE** ser informado explicitamente pelo chamador. Os atalhos não
    escondem isso: a conversão de `-1` para 32 bits sem sinal é
    silenciosa, e a advertência precisa estar na documentação deles.
- **Descrição em texto do domínio**: ver a regra dos rótulos na seção 7.

### 4.4 Versão 3 e Versão 5 (Baseadas em Espaço de Nomes)

1. Obter os 16 bytes do UUID de espaço de nomes (ex.: `NameSpaceDNS`,
   `NameSpaceURL`, `NameSpaceOID`, `NameSpaceX500`).
2. Concatenar os 16 bytes do espaço de nomes com os bytes do nome
   fornecido.
3. Calcular o hash criptográfico do conjunto:
   - **Versão 3**: MD5 (produz exatamente 16 bytes).
   - **Versão 5**: SHA-1 (produz 20 bytes; descartar os 4 bytes
     finais).
4. Gravar a versão no nibble alto do byte 6 (`0x3` ou `0x5`).
5. Gravar a variante nos 2 bits superiores do byte 8 (`0b10`).

### 4.5 Versão 4 e Versão 8

- **Versão 4**: 16 bytes preenchidos com aleatoriedade. Sobrescrever o
  nibble alto do byte 6 com `0x4` e os bits altos do byte 8 com `0b10`.
- **Versão 8**: Formato livre da RFC 9562 para uso específico de
  aplicações. Manter os 122 bits fornecidos ou preenchê-los com
  aleatoriedade, sobrescrevendo a versão com `0x8` e a variante `0b10`.

---

## 5. Arquitetura de Entropia e Política de Falha Rápida (Fail-Fast)

### 5.1 O Gerador Padrão (Alta Concorrência e Zero Contenção)

- Para geração rápida (milhões de UUIDs/s), não utilize um lock global em
  torno de uma fonte compartilhada.
- Use uma fonte **local por thread**. Se a linguagem já oferecer uma no
  runtime, prefira-a: em Go, as funções de pacote de `math/rand/v2` leem
  de uma instância de ChaCha8 por thread, semeada pelo sistema
  operacional, sem trava e sem estado a manter pela biblioteca.
- Se a plataforma não oferecer uma fonte por thread, mantenha um **pool
  de geradores locais**, cada um instanciado sob demanda e semeado **uma
  única vez** com 128 bits obtidos da fonte criptográfica forte do
  sistema operacional.

### 5.2 Política Anti-Degradação Silenciosa (Evitando Falha Grave de Segurança)

A política tem **três casos distintos**, e confundi-los leva a
implementações que param o processo onde não deveriam, ou que seguem
onde não podem. O vocabulário de "falhar alto" vale para o primeiro; os
outros dois são erros de configuração do chamador, e recebem tratamento
próprio.

**Caso 1 — falha da fonte durante a operação.** Se a fonte forte de
entropia do sistema operacional falhar ao semear um gerador, ao sortear
dados para nós e sequências, ou ao alimentar um gerador construído sobre
um leitor do chamador:
  - **A biblioteca DEVE falhar alto e imediatamente (pânico / exceção /
    encerramento)**.
  - **JAMAIS recorra ao relógio do sistema como fallback silencioso**.
    Semear múltiplos geradores com o horário atual produz sequências
    idênticas ou correlacionadas entre threads, levando a colisões
    maciças de UUIDs e previsibilidade total de chaves e identificadores.
  - Os apelidos de compatibilidade que devolvem erro (seção 5.3) podem
    converter esse pânico no erro de fonte de entropia (seção 6.4); a
    geração propriamente dita nunca devolve erro.

**Caso 2 — configuração inválida explícita.** Uma função de construção
que receba fonte nula, ou leitor nulo, **DEVE** entrar em pânico na
própria construção. O erro é de configuração e precisa aparecer na
carga do programa, não na primeira geração, onde apareceria em produção
como uma falha de ponteiro nulo longe da causa.

**Caso 3 — ausência de fonte por construção omitida.** Um gerador que
chegue a uma geração sem fonte, por ter sido montado fora das funções de
construção (valor zero embutido em outra estrutura, ou referência nula
usada como receptor), **DEVE** recorrer à fonte padrão do pacote em vez
de entrar em pânico. Aqui não há degradação: a fonte padrão é exatamente
a que a construção correta teria instalado, então a imprevisibilidade
dos bits não muda. O caso 1 não se aplica, porque nenhuma fonte falhou.
A tolerância é regra do tipo gerador inteiro, sem exceção por versão:
vale para a geração pelo relógio, para a geração por instante (seção
3.6) e para as versões 4 e 8.

A diferença entre os casos 2 e 3 é o momento e a intenção: quem pede uma
fonte inválida recebe o erro de imediato; quem simplesmente não construiu
o gerador recebe um resultado correto. Ver a seção 11.3.

### 5.3 Fontes Criptográficas Dedicadas

- O gerador padrão não promete força criptográfica, mesmo quando a fonte
  do runtime é resistente a predição.
- Para casos que exigem imprevisibilidade (tokens de sessão, links
  secretos), a biblioteca deve fornecer um gerador explícito que utiliza
  exclusivamente entropia criptográfica (`NewCryptoGenerator`).
- As funções de compatibilidade com pacotes legados (como `google/uuid`:
  `New`, `NewString`, `NewRandom`, `NewV7`) **DEVEM utilizar entropia
  criptográfica** para não violar o contrato de segurança esperado por
  códigos migrados.

---

## 6. Algoritmos de Conversão e Parsing Seguro

### 6.1 Formatação (Binário → String Canônica)

- Utilizar uma tabela de caracteres hexadecimais minúsculos
  `"0123456789abcdef"`.
- Gravar diretamente em um buffer fixo de 36 caracteres, inserindo os
  hifens nos índices 8, 13, 18 e 23 sem alocações intermediárias.

### 6.2 Análise Estrita (String Canônica → Binário)

- Validar comprimento exato de 36 caracteres.
- Validar se `s[8] == '-'`, `s[13] == '-'`, `s[18] == '-'` e
  `s[23] == '-'`.
- **REGRA CRÍTICA DE SEGURANÇA (Eliminação de Defeito Crítico de Pânico)**:
  - **NUNCA** decodifique a string percorrendo caractere por caractere e
    avançando um índice quando encontrar um hífen. Se a entrada contiver
    hifens extras em posições inesperadas, o laço desalinha e tenta ler
    `s[36]`, resultando em pânico de estouro de array (*index out of
    range*) e derrubando o processo da aplicação.
  - **DECODIFIQUE SEMPRE via tabela de deslocamentos fixos conhecidos**:
    `hexOffsets = [16]int{0, 2, 4, 6, 9, 11, 14, 16, 19, 21, 24, 26, 28, 30, 32, 34}`
  - Para cada byte `i` de 0 a 15, decodifique os dois caracteres
    hexadecimais localizados em `s[hexOffsets[i]]` e
    `s[hexOffsets[i]+1]`.
  - Se qualquer caractere não for hexadecimal válido (0..9, a..f, A..F),
    retorne erro de formato e devolva o UUID zerado.

### 6.3 Analisador Permissivo (4 Formatos)

A função `Parse` deve aceitar quatro formatos distintos (maiúsculas ou
minúsculas):

1. **Canônico com hífens** (36 caracteres): decodificado conforme 6.2.
2. **Sem hífens** (32 caracteres): decodificado diretamente a cada 2
   caracteres (`i * 2`).
3. **Entre chaves** (38 caracteres): deve iniciar com `{` e terminar com
   `}`, contendo os 36 caracteres canônicos internamente.
4. **Prefixo URN** (45 caracteres): deve iniciar com `urn:uuid:`
   (insensível a maiúsculas), seguido dos 36 caracteres canônicos.
- Qualquer outro comprimento deve ser rejeitado imediatamente como
  comprimento inválido.

### 6.4 Taxonomia de Erros

A biblioteca **DEVE** expor um erro sentinela de formato e erros mais
específicos que o **embrulham**, de modo que a verificação pelo
sentinela (`errors.Is` em Go, ou o equivalente da linguagem) continue
verdadeira para qualquer um deles. Quem trata só a presença de erro não
precisa conhecer a tabela; quem decide pelo tipo, precisa, e é para esse
chamador que ela existe. A tabela é normativa: duas implementações
conformes devolvem o mesmo erro para a mesma entrada.

| Operação | Entrada | Erro devolvido |
|:---|:---|:---|
| Analisador estrito (6.2) e a extração de tempo em texto (seção 7) | qualquer recusa, inclusive comprimento | sentinela de formato, **puro** |
| Analisador permissivo (6.3) | comprimento fora de 32, 36, 38 e 45 | comprimento inválido |
| Analisador permissivo (6.3) | 38 caracteres sem `{` na primeira posição ou sem `}` na última | chaves inválidas |
| Analisador permissivo (6.3) | 45 caracteres sem o prefixo `urn:uuid:` | sentinela de formato |
| Analisador permissivo (6.3) | dígito não hexadecimal, ou hífen fora das posições 8, 13, 18 e 23 | sentinela de formato |
| Construção a partir de bytes crus | tamanho diferente de 16 | comprimento inválido |
| Desserialização binária | tamanho diferente de 16 | comprimento inválido |
| Desserialização de texto | as mesmas entradas do analisador permissivo | os mesmos erros do analisador permissivo |
| Desserialização JSON do tipo anulável | valor que não é string nem `null`, ou sintaxe JSON inválida | sentinela de formato |
| Desserialização JSON do tipo anulável | string cujo conteúdo o analisador permissivo recusa | os erros do analisador permissivo |
| Leitura de valor de banco | texto, ou bytes em tamanho diferente de 16, que o analisador permissivo recusa | os erros do analisador permissivo |
| Leitura de valor de banco | tipo que não é texto, bytes nem nulo | tipo não suportado |
| Fonte de entropia do chamador (leitor) | leitor nulo, ou leitura que falhou | erro de fonte de entropia nos apelidos de compatibilidade que devolvem erro; pânico com o mesmo valor no gerador construído sobre o leitor (seção 5.2, caso 1) |

Todo erro de comprimento e de chaves **DEVE** embrulhar o sentinela de
formato. O erro de tipo não suportado e o de fonte de entropia são
famílias à parte e **NÃO** embrulham o sentinela: não são recusas de
texto. A desserialização JSON do tipo simples delega ao codificador da
linguagem, que devolve o erro dele para valores que não são string e o
erro do analisador permissivo para strings recusadas.

**Regra normativa — o analisador estrito não embrulha.** Ele devolve o
sentinela puro em todos os casos, inclusive comprimento errado, para que
a comparação por igualdade direta continue valendo em código existente
(seção 11.3). A implementação de referência oferece também um predicado
que reconhece o erro de comprimento através de camadas de embrulho.

**Assimetria registrada.** No analisador permissivo, as chaves
malformadas têm erro próprio e o prefixo URN inválido **não** tem: ele
devolve o sentinela puro, na mesma classe do dígito inválido. As duas
situações são análogas e a assimetria não tem motivo técnico; veio com a
primeira versão do analisador permissivo e é mantida por
compatibilidade, porque criar o erro de prefixo mudaria o valor
devolvido para uma entrada que hoje recebe o sentinela puro. Uma
reimplementação **DEVE** reproduzi-la, para que os dois analisadores
classifiquem a mesma entrada do mesmo modo. A decisão e o que
justificaria revê-la estão na seção 11.3.

Em todos os casos de recusa o valor devolvido **DEVE** ser o UUID zerado,
e nenhum byte parcialmente decodificado pode vazar; nas desserializações
com receptor, o receptor **NÃO DEVE** ser alterado.

Os dois auxiliares que entram em pânico em vez de devolver erro, um para
constantes do próprio código (`MustParse`) e um para encadear com
funções que devolvem par de valores (`Must`), não fazem parte da tabela.
O segundo propaga o próprio erro recebido, para que quem recupere o
pânico o reconheça pelo sentinela.

---

## 7. Inspeção e Contrato de API

A biblioteca deve disponibilizar operações de consulta:

- **`Version() byte`**: retorna o número da versão (1 a 8).
- **`Variant() byte`**: retorna a variante RFC (geralmente `10` binário =
  `VariantRFC4122`).
- **`Timestamp() (time.Time, bool)`**:
  - Para UUIDv7: extrai os milissegundos Unix.
  - Para UUIDv1 e UUIDv6: extrai o carimbo gregoriano e converte para
    UTC.
  - Para demais versões: devolve booleano falso.
- **`TimestampWithLevel(Level) (time.Time, bool)`**: devolve o instante
  de um UUIDv7 somando ao milissegundo a precisão sub-milissegundo
  gravada pelo nível informado. O nível **não** é dedutível do
  identificador, e por isso vem por parâmetro. Devolve falso para
  versões diferentes de 7; no Nível 1 e em níveis desconhecidos devolve
  apenas o milissegundo.

  **Regra normativa — descarte por faixa.** A condição de aproveitamento
  é sobre o **valor lido**, não sobre o nível pedido. Se um campo
  sub-milissegundo estiver fora da faixa de 0 a 999, ele denuncia
  entropia em vez de tempo, e a implementação **DEVE** descartar a
  precisão sub-milissegundo, devolvendo apenas o milissegundo do
  carimbo de 48 bits. Informar o nível errado degrada para o
  milissegundo, que é um resultado utilizável; não devolve erro nem
  lixo.

  **Regra normativa — no Nível 3 os dois campos caem juntos.** Um
  `rand_a` fora da faixa prova que o topo de `rand_b` também é ruído,
  porque os dois vêm da mesma geração. A implementação **NÃO DEVE**
  aproveitar `nano` quando `micro` foi descartado, mesmo que `nano`
  caiba em 0 a 999 por acaso, o que ocorre em cerca de 98% dos casos
  (1000 valores válidos em 1024 possíveis). O caso simétrico vale
  igualmente: `nano` fora da faixa descarta também `micro`. No Nível 2
  o `nano` não é lido, e só o `micro` decide.

  Esta política é o **oposto** da extração completa descrita a seguir,
  que entrega os bits sem julgar a faixa. A divergência é deliberada:
  uma operação entrega um instante, a outra entrega os bits. Ver a
  seção 11.3.
- **`ImportBinary(UUID) Time`** e **`Import(texto) (Time, erro)`**:
  extração completa dos campos de tempo de um UUIDv7, devolvendo uma
  estrutura com quatro campos. A forma em texto analisa a string pelo
  analisador **estrito** da seção 6.2, devolve o sentinela puro com a
  estrutura zerada em caso de recusa, e delega à forma binária.
  - `Seconds`: segundos Unix, por divisão inteira do carimbo de 48 bits
    por mil.
  - `Milliseconds`: o resto dessa divisão, sempre na faixa 0 a 999. Os
    dois campos saem do mesmo carimbo, e o recorte entre segundo e
    fração é decisão de contrato, não consequência do layout.
  - `Microseconds`: os 12 bits de `rand_a`, lidos como estão.
  - `Nanoseconds`: os 10 bits altos de `rand_b`, lidos como estão.

  **Regra normativa — a extração é cega quanto ao nível.** Ela **DEVE**
  sempre interpretar `rand_a` como microssegundos e o topo de `rand_b`
  como nanossegundos, e **NÃO DEVE** validar faixa nem descartar campo
  algum. A operação não recebe o nível, e ele não é dedutível do
  identificador: em um UUIDv7 de Nível 1 esses campos carregam entropia,
  e a extração devolve os bits lidos sem julgar a origem.
  Consequentemente `Microseconds` vai de 0 a 4095 e `Nanoseconds` de 0 a
  1023 quando a origem é aleatória, e é o chamador, que sabe o nível,
  quem decide o que aproveitar. É a operação inversa da geração por
  instante da seção 3.6, e a estrutura devolvida **NÃO** passa pelo
  descarte da leitura por nível acima.
- **`GregorianTime() (GregorianTime, bool)`** (método de `UUID`): o
  campo de tempo de 60 bits das versões 1 e 6, recomposto pela leitura
  inversa da seção 4.1. Devolve falso para as demais versões, **inclusive
  a 2**, cujo carimbo perdeu os 32 bits baixos e não é utilizável. O tipo
  devolvido conta tiques de cem nanossegundos desde 1582 e oferece as
  duas conversões: `UnixTime() (sec, nsec)`, o par canônico da conversão
  inversa da seção 4.1, e `Time()`, o instante da linguagem em UTC.
- **`ClockSequence() (int, bool)`**:
  - Para v1 e v6: retorna os 14 bits completos da sequência.
  - Para v2: retorna **apenas os 6 bits mais altos** (0 a 63).
  - Para demais versões: retorna falso.
- **`GetTime() (GregorianTime, uint16)`**: devolve o tempo gregoriano
  corrente, avançando o relógio interno como faria uma geração de UUIDv1,
  e a sequência de relógio de 14 bits **com os dois bits de variante já
  posicionados** (bit `0x8000` ligado, bit `0x4000` desligado), pronta
  para ser gravada nos bytes 8 e 9. Para obter só a sequência, mascare
  com `0x3FFF` ou use `ClockSequence()`.
- **`Domain() (Domain, bool)`** e **`ID() (uint32, bool)`**: para UUIDv2.
- **`NodeID() []byte`** (método de `UUID`): devolve uma cópia dos 6 bytes
  de nó para v1, v2 e v6, e `nil` para as demais versões.
- **`IsValid() bool`**: verdadeiro quando a variante é a da RFC (`10`
  binário) **e** a versão está entre 1 e 8. Os valores especiais `Nil` e
  `Max` contam como válidos, apesar de não carregarem versão nem
  variante, porque a RFC 9562 seções 5.9 e 5.10 os define como válidos;
  `IsZero` e `IsMax` distinguem os dois casos. A verificação **não** é
  específica de uma versão: um UUIDv4 gerado em outro sistema é válido.
- **`Bytes() []byte`**: cópia dos 16 bytes em ordem de rede. Existe
  separado do acesso direto ao vetor porque este último devolve uma
  referência ao próprio valor, e porque em linguagens onde o tipo tem
  formatação própria (como `fmt.Stringer` em Go) os verbos hexadecimais
  formatam o texto, não os bytes.
- **`AppendTo(dst) dst`**: escreve os 36 bytes da forma canônica no fim
  do buffer do chamador e devolve o buffer estendido, **sem alocar**
  quando houver capacidade. É o caminho previsto para serializar grandes
  volumes; a conversão que devolve string aloca a cada chamada.
- **Descrições em texto de versão, variante e domínio**
  (`VersionString`, `VariantString` e o texto do domínio da versão 2): a
  biblioteca **PODE** oferecer descrições legíveis para os três códigos.
  Elas são texto de apresentação e **não** são contrato de formato: podem
  ser traduzidas ou reescritas sem quebrar dados, e uma reimplementação
  escolhe o idioma dela. Dois pontos têm conteúdo técnico e **DEVEM** ser
  respeitados: os códigos de variante 0 e 1 são a mesma variante herdada
  (compatibilidade NCS) e compartilham a descrição; o código 3 é ambíguo
  entre a variante reservada à Microsoft e a reservada para uso futuro,
  porque a consulta de variante expõe apenas os dois bits altos do byte
  8, e distinguir as duas exigiria um terceiro. Valores fora da faixa
  produzem um texto de "desconhecido" com o número, nunca erro.

A biblioteca deve disponibilizar também as operações de **derivação**
da seção 3.5, que são o sentido inverso das acima: recebem um instante
e um nível e devolvem o identificador que os delimita. Diferentemente
de todas as operações desta seção, elas não são métodos de um UUID.

- **`MinAt(Level, instante) UUID`**: o menor UUIDv7 que a biblioteca
  poderia gerar naquele instante e naquele nível, com os bits livres de
  entropia em zero.
- **`MaxAt(Level, instante) UUID`**: o maior, com os bits livres em um.
  É o limite superior **fechado** do instante.
- **`RangeAt(Level, from, to) (lo, hi)`**: o par de um intervalo
  **semiaberto** `[from, to)`, com `lo = MinAt(nível, from)` e
  `hi = MinAt(nível, to)`, para `id >= lo AND id < hi`.
- **`GenerateAt(Level, instante) UUID`** e
  **`GenerateAtString(Level, instante) string`** (seção 3.6): um UUIDv7
  daquele instante com os bits livres **sorteados**. Duas chamadas com o
  mesmo instante devolvem valores diferentes. Existem também como
  métodos do gerador, para que uma fonte de entropia configurada pelo
  chamador continue valendo.

As cinco preservam versão e variante, decompõem o instante com saturação
nas duas pontas da faixa representável e valem apenas para
identificadores gravados no **mesmo nível**. As regras normativas estão
nas seções 3.5 e 3.6.

---

## 8. Serialização, Banco de Dados e Valores Especiais

1. **Serialização em Texto e JSON**:
   - Um UUID serializado em JSON **DEVE ser formatado como string
     canônica entre aspas** (`"0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff"`).
   - **NUNCA** serialize como um vetor/array de 16 números inteiros.
2. **Integração com Banco de Dados**:
   - A **leitura DEVE aceitar as duas formas**: string canônica em
     qualquer formato aceito pelo analisador permissivo, e 16 bytes
     crus. Texto vazio e fatia vazia equivalem a ausência de valor, sem
     erro.
   - A **escrita padrão DEVE ser a string canônica**, e esse formato é
     estável: trocá-lo deixaria duas representações na mesma coluna e as
     linhas antigas parariam de casar com as consultas.
   - A escrita binária **DEVE existir como tipo distinto**, escolhido por
     conversão no ponto da consulta, e nunca por configuração global. Ela
     entrega os 16 bytes em ordem de rede, sem rotacionar campos: a
     rotação que algumas receitas sugerem para o UUIDv1 é desnecessária
     no UUIDv7, que já nasce ordenado, e produziria um valor ilegível
     para outras ferramentas.
   - Fornecer um tipo `NullUUID` contendo o UUID e um booleano `Valid`
     para campos de tabela que permitem valor `NULL`, e o equivalente
     para a escrita binária.
   - **Ausência de valor e UUID nulo são valores distintos** e **NÃO
     DEVEM** colapsar um no outro. Com o booleano falso a escrita produz
     `NULL`; dezesseis bytes zerados só saem com o booleano verdadeiro e
     o UUID igual a `Nil`.
   - A representação da **ausência** no tipo anulável depende do formato
     de destino, e não se deduz de uma regra só: cada formato usa a
     convenção própria de "nada aqui".

     | Destino | Ausência produz | Leitura de entrada vazia |
     |:---|:---|:---|
     | Banco de dados | `NULL` | ausência, sem erro |
     | JSON | o literal `null`, sem aspas | ausência, sem erro |
     | Texto | sequência vazia (zero bytes) | ausência, sem erro |
     | Binário | sequência vazia (zero bytes) | ausência, sem erro |

     Na leitura, a entrada vazia em qualquer dos quatro produz ausência
     de valor, sem erro, coerente com a regra de texto vazio acima. Um
     destino que espere a sequência vazia e receba o literal `null`, ou
     o contrário, falha na desserialização, e é por isso que a tabela é
     normativa. Entrada inválida devolve erro e deixa o booleano falso.
3. **Valores Especiais**:
   - `Nil`: todos os 16 bytes em zero (`00000000-0000-0000-0000-000000000000`).
   - `Max`: todos os 16 bytes em `0xFF` (`ffffffff-ffff-ffff-ffff-ffffffffffff`).
   - `Compare(a, b)`: comparação byte a byte em ordem lexicográfica
     retornando `-1`, `0` ou `1`.

---

## 9. Catálogo de Armadilhas Evitadas (Guia Anti-Regressão)

Toda reimplementação deve garantir proteção contra estes 12 defeitos
reais:

| # | Armadilha Histórica | Consequência | Solução Obrigatória |
|---|:---|:---|:---|
| 1 | **Parser pulando hífens em laço** | Entrada maliciosa com hífen extra causava pânico e crash por `index out of range` | Decodificar exclusivamente por tabela fixa de 16 posições (`hexOffsets`) |
| 2 | **Relógio anterior a 1970** | Módulo de números negativos corrompia `rand_a` e `rand_b` | Fixar piso em 0 para `unix_ts_ms`, microssegundos e nanossegundos se `sec < 0` |
| 3 | **Inteiro de 64 bits para nanos** | `UnixNano()` estoura em 2262-04-11 | Ler segundos e nanossegundos em duas partes separadas |
| 4 | **Reset de relógio em `SetNodeID`** | Reaplicar nó zerava `lastClockTime`, gerando repetição de UUIDv1 | Manter o carimbo de relógio inalterado ao configurar ou reaplicar o nó |
| 5 | **Fallback de entropia no relógio** | Falha de `crypto/rand` degradava silenciosamente para o relógio, gerando colisões | Falhar imediatamente com pânico (fail-fast); proibido degradar em silêncio |
| 6 | **Escrita em sink global em teste concorrente** | Detector de corrida (`-race`) disparava falso alerta em benchmarks | Usar `runtime.KeepAlive(u)` por goroutine em vez de escrever em variável compartilhada |
| 7 | **`ClockSequence` na versão 2** | Retornava o identificador de domínio mascarado dentro da sequência | Isolar apenas os 6 bits superiores (0..63) para a versão 2 |
| 8 | **Desperdício de entropia no v7** | Sortear duas palavras de 64 bits nos Níveis 2 e 3 | Sortear apenas 1 palavra nos Níveis 2 e 3 (economia de 17% em concorrência) |
| 9 | **Serialização JSON como array** | `[1, 146, 247, ...]` em vez de `"0192f7c5-..."` quebrava interoperabilidade | Implementar `MarshalText`/`MarshalBinary` canônicos |
| 10 | **Tags sobrescritas com `-f`** | Quebrava a verificação de integridade no registro público (`sum.golang.org`) | Tags publicadas são estritamente imutáveis; nunca mover com `-f` |
| 11 | **Estouro de `sec * 1000` com instante fora da faixa** | Com o instante vindo por parâmetro, o produto estoura o inteiro com sinal e troca de sinal: data remota no futuro cai no piso da época, data remota no passado vira carimbo enorme. A guarda de `sec < 0` não dispara, porque o sinal já foi invertido | Decidir a saturação sobre os **segundos**, antes da multiplicação (seções 3.2 e 3.5) |
| 12 | **Fronteira de intervalo truncando em vez de saturar** | O empacotamento por deslocamento descarta os bits acima de 48 de graça: a fronteira dá a volta e a consulta por faixa devolve as linhas erradas **em silêncio**, sem erro nenhum. Zerar `micro` e `nano` na saturação tem o mesmo efeito na travessia da borda | Saturar nas duas pontas, levando `micro` e `nano` a 999 no teto; a fronteira nunca pode regredir quando o instante avança (seção 3.5) |

---

## 10. Casos de Teste Obrigatórios para Validação

1. **Conformidade de Versão e Variante**:
   - Validar que cada gerador (V1 a V8 e Níveis 1 a 3 de V7) define
     exatamente sua respectiva versão e variante `0b10`.
2. **Robustez do Analisador contra Mutações**:
   - Executar teste cobrindo **todas as 36 × 256 mutações de um único byte**
     sobre uma string canônica válida: nenhuma mutação pode causar pânico.
   - Submeter o analisador a campanhas de *fuzzing* contínuo.
3. **Bordas Temporais Extremas**:
   - Testar instantes com data anterior a 1970 (ex.: ano 1969 e ano 1800).
   - O caso pré-1970 **DEVE** usar um segundo negativo com fração
     positiva (por exemplo `sec = -1`, `nsec = 500.000.000`): é a entrada
     que uma transcrição da fórmula com conversão sem sinal antes da
     guarda (seção 3.2, item 2) transforma em carimbo enorme, e as duas
     formas de piso aceitas devem devolver a época.
   - Testar instantes além do ano 2262 (ex.: ano 2300).
   - Validar viradas de segundo (`nsec = 999_999_999`) e viradas de
     milissegundo (`sub_ms = 999_999`).
4. **Contagem de Sorteios de Entropia**:
   - Com gerador de contagem determinística, verificar que Nível 2 e
     Nível 3 consomem 1 chamada; Nível 1 consome 2 chamadas.
5. **Vetores Dourados da RFC 9562 para Versões Baseadas em Hash**:
   - Validar que V3 e V5 produzem exatamente os vetores publicados na
     RFC 9562 (Apêndice A), que usam o espaço `NameSpaceDNS` e o nome
     `www.example.com`:
     - V3: `5df41881-3aed-3515-88a7-2f4a814cf09e`
     - V5: `2ed6657d-e927-568b-95e1-2665a8aea6a2`
   - A RFC não publica vetores para os demais espaços de nomes; vetores
     adicionais só devem entrar na suíte se forem calculados por uma
     implementação independente.
6. **Ordenação Temporal Coerente**:
   - Testar que se o instante de B for estritamente superior ao instante de
     A, a comparação de strings e de bytes de B é estritamente maior que a
     de A.
   - Não contar regressões em laço apertado na mesma thread: se o tempo não
     avança na resolução do host, o desempate por entropia é aleatório.
7. **Teste de Não-Repetição de Nó**:
   - Gerar uma rajada de UUIDv1, chamar `SetNodeID` com o mesmo nó e gerar
     outra rajada: garantir unicidade absoluta de todos os UUIDs gerados.
8. **Concorrência e Ausência de Corridas de Dados**:
   - Gerar 1.000.000 de UUIDs divididos entre centenas de threads
     simultâneas sem nenhuma colisão e sem nenhum alerta no detector de
     corridas (*race detector*).
9. **Isolamento do Estado Global de Relógio**:
   - Os testes das versões 1, 2 e 6 compartilham nó e sequência de
     relógio. Todo teste que alterar qualquer um dos dois **DEVE**
     restaurá-lo ao terminar, e nenhum deles pode rodar em paralelo.
   - A suíte deve passar com repetição (`-count 3`) e com ordem
     embaralhada (`-shuffle on`). Sem isso, um teste que fixa o nó faz os
     seguintes rodarem com um nó que não é o padrão, e a falha aparece
     longe da causa.
10. **Fronteiras de Tempo (seção 3.5)**:
    - Gerar uma rajada entre dois instantes lidos do relógio e conferir
      que **toda** ela cai dentro das fronteiras desses instantes, nos
      três níveis. É o teste que prova a fronteira, e não a inspeção do
      layout de bits.
    - Conferir versão 7 e variante `0b10` nas duas fronteiras, nos três
      níveis, inclusive nos instantes saturados.
    - Conferir a ordem: a fronteira inferior nunca passa da superior no
      mesmo instante, e instantes separados pela resolução do nível
      produzem faixas disjuntas e em ordem.
    - Conferir a monotonicidade sobre uma lista de instantes que inclua
      as duas pontas saturadas e a travessia da borda superior. Zerar
      `micro` e `nano` na saturação **deve** fazer este teste falhar.
    - Conferir o estouro da multiplicação por mil descrito na seção 3.5,
      com instantes grandes o bastante para provocá-lo nas duas direções.
    - Conferir a ida e volta: ler a fronteira com a leitura por nível
      devolve o instante de origem, truncado à resolução do nível.
    - Travar o layout com vetores fixos, calculados fora da
      implementação.
11. **Geração por Instante Explícito (seção 3.6)**:
    - **Ida e volta com a extração**: gerar para segundos, milissegundos,
      microssegundos e nanossegundos conhecidos e conferir que a leitura
      devolve exatamente esses campos, em cada nível que os grava. É o
      teste central, porque prova a simetria que motiva a operação.
    - **Não divergência com a geração pelo relógio**: com a mesma fonte
      de entropia constante, gerar pelo relógio, ler o instante embutido
      de volta e regerar para ele. Os 16 bytes **devem** ser idênticos. É
      esta a trava da duplicação deliberada da seção 11.2, e ela falha se
      as duas cópias do empacotamento se separarem.
    - **Contenção pelas fronteiras**: o valor gerado para um instante cai
      sempre dentro das fronteiras daquele instante (seção 3.5).
    - **Não determinismo**: muitas chamadas com o mesmo instante
      devolvem valores todos distintos, e com os campos de tempo iguais.
    - Consumo de entropia por nível igual ao da seção 3.3.
    - Bordas: pré-1970, saturação acima da faixa e níveis desconhecidos.
12. **Vetores Dourados da Extensão Multinível**:
    - A RFC 9562 publica vetores para as versões 3 e 5 (caso 5), mas a
      extensão multinível é deste projeto e não tem vetor publicado em
      lugar nenhum. Sem eles, uma reimplementação em outra linguagem não
      tem contra o que se conferir justamente na parte que não é padrão,
      onde é mais fácil errar.
    - Os vetores abaixo **são contrato**. Uma implementação que produza
      outra coisa para as mesmas entradas está errada. Mudá-los é
      mudança de formato de dados, não ajuste de teste.
    - As entradas são um instante e o valor dos **bits livres de
      entropia**, todos em zero ou todos em um. São exatamente o que as
      duas fronteiras da seção 3.5 produzem, e é assim que se obtêm sem
      relógio: `MinAt` para os bits em zero, `MaxAt` para os bits em um.

    **Grupo A** — 2026-01-01T00:00:00.123456789Z  (sec=1767225600, nsec=123456789)

    | Nível | Bits livres | 16 bytes | String canônica |
    |:---|:---|:---|:---|
    | 1 | zero | `019b76daa87b70008000000000000000` | `019b76da-a87b-7000-8000-000000000000` |
    | 1 | um | `019b76daa87b7fffbfffffffffffffff` | `019b76da-a87b-7fff-bfff-ffffffffffff` |
    | 2 | zero | `019b76daa87b71c88000000000000000` | `019b76da-a87b-71c8-8000-000000000000` |
    | 2 | um | `019b76daa87b71c8bfffffffffffffff` | `019b76da-a87b-71c8-bfff-ffffffffffff` |
    | 3 | zero | `019b76daa87b71c8b150000000000000` | `019b76da-a87b-71c8-b150-000000000000` |
    | 3 | um | `019b76daa87b71c8b15fffffffffffff` | `019b76da-a87b-71c8-b15f-ffffffffffff` |

    **Grupo B** — 2026-01-01T00:00:00.000000000Z  (sec=1767225600, nsec=0)

    | Nível | Bits livres | 16 bytes | String canônica |
    |:---|:---|:---|:---|
    | 1 | zero | `019b76daa80070008000000000000000` | `019b76da-a800-7000-8000-000000000000` |
    | 1 | um | `019b76daa8007fffbfffffffffffffff` | `019b76da-a800-7fff-bfff-ffffffffffff` |
    | 2 | zero | `019b76daa80070008000000000000000` | `019b76da-a800-7000-8000-000000000000` |
    | 2 | um | `019b76daa8007000bfffffffffffffff` | `019b76da-a800-7000-bfff-ffffffffffff` |
    | 3 | zero | `019b76daa80070008000000000000000` | `019b76da-a800-7000-8000-000000000000` |
    | 3 | um | `019b76daa8007000800fffffffffffff` | `019b76da-a800-7000-800f-ffffffffffff` |

    **Grupo C** — 1970-01-01T00:00:00.000000000Z  (sec=0, nsec=0)

    | Nível | Bits livres | 16 bytes | String canônica |
    |:---|:---|:---|:---|
    | 1 | zero | `00000000000070008000000000000000` | `00000000-0000-7000-8000-000000000000` |
    | 1 | um | `0000000000007fffbfffffffffffffff` | `00000000-0000-7fff-bfff-ffffffffffff` |
    | 2 | zero | `00000000000070008000000000000000` | `00000000-0000-7000-8000-000000000000` |
    | 2 | um | `0000000000007000bfffffffffffffff` | `00000000-0000-7000-bfff-ffffffffffff` |
    | 3 | zero | `00000000000070008000000000000000` | `00000000-0000-7000-8000-000000000000` |
    | 3 | um | `0000000000007000800fffffffffffff` | `00000000-0000-7000-800f-ffffffffffff` |

    **O que cada coisa prova.**

    - **Grupo A**, os três níveis: os microssegundos 456 aparecem como
      `1c8` em `rand_a` nos níveis 2 e 3, e não no nível 1, onde o campo
      é entropia. É a prova da posição dos microssegundos.
    - **Grupo A**, nível 3: os nanossegundos 789 (`0x315`, dez bits)
      aparecem repartidos entre os seis bits baixos do byte 8 (`0x31`,
      somados à variante dão `0xb1`) e o nibble alto do byte 9 (`0x5`).
      É a prova da posição dos nanossegundos nos dez bits altos de
      `rand_b`.
    - **Todas as linhas com bits livres em um**: o byte 6 nunca passa de
      `0x7f` e o byte 8 fica sempre na faixa `0x80..0xbf`. É a prova de
      que a versão e a variante sobrevivem ao preenchimento, e é o que
      torna as fronteiras da seção 3.5 limites corretos.
    - **Grupo B contra o grupo A**: com o sub-milissegundo zerado, os
      níveis 2 e 3 devolvem `rand_a` em zero mesmo com os bits livres em
      um, enquanto o nível 1 devolve `0xfff`. É a prova de que os níveis
      2 e 3 **não** sorteiam `rand_a`, e pega erro de sinal e de
      deslocamento que um instante com todos os campos preenchidos
      esconderia.
    - **Grupo C**: a época Unix com os 48 bits de carimbo zerados. Um
      instante **anterior** a 1970 tem de produzir exatamente estes
      mesmos valores, pelo piso da seção 3.2. Esse é o par que prova o
      piso, e por isso a suíte confere o grupo C duas vezes, uma com a
      época e outra com `1969-12-31T23:59:59.999999999Z`.

    Os valores publicados aqui foram calculados por uma implementação
    independente, escrita a partir das regras das seções 3.1, 3.2 e 3.5,
    e só então conferidos contra a implementação de referência. Um vetor
    produzido pela própria implementação e conferido contra ela mesma
    não prova nada.
13. **Extração Cega de Nível (seção 7)**:
    - Fixar um vetor com os campos sub-milissegundo **fora** da faixa de
      0 a 999 e exigir que a extração completa os devolva crus. Um vetor
      com valores dentro da faixa não distingue as duas políticas de
      leitura e por isso não prova nada aqui.
    - Vetor: os 16 bytes `0192f7c51a2b7c3d8e4faabbccddeeff`, string
      `0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff`, produzem carimbo
      `0x0192f7c51a2b` = 1.730.733.742.635 ms, portanto `Seconds` =
      1.730.733.742 e `Milliseconds` = 635; `Microseconds` = `0xc3d` =
      3133 e `Nanoseconds` = `0xe4` = 228, os dois fora da faixa de
      tempo real.
    - Conferir que a forma em texto concorda com a binária, campo a
      campo, e que uma string recusada devolve o sentinela puro com a
      estrutura zerada.
    - Conferir, em volume, as faixas por nível: gerado em Nível 2 ou 3, o
      campo de microssegundos fica em 0 a 999; gerado em Nível 3, o de
      nanossegundos também; gerado em Nível 1, os dois **excedem** 999 em
      alguma amostra, o que prova que a extração não filtra.
14. **Descarte por Faixa na Leitura por Nível (seção 7)**:
    - Montar um UUIDv7 com `rand_a` acima de 999 e exigir que a leitura
      em Nível 2 devolva o instante truncado ao milissegundo.
    - Montar um UUIDv7 com `rand_a` acima de 999 e `nano` dentro de 0 a
      999 e exigir que a leitura em Nível 3 descarte **os dois** campos.
      É este o caso que distingue as duas regras: uma implementação que
      valide os campos separadamente passa no anterior e falha neste.
    - Montar o caso simétrico, `micro` válido e `nano` acima de 999, e
      exigir que o Nível 3 devolva só o milissegundo enquanto o Nível 2
      soma os microssegundos, porque não lê `nano`.
    - Montar o caso inteiramente válido e exigir a soma dos dois.
    - Conferir que a extração cega, sobre os mesmos bytes, devolve os
      valores crus. As duas leituras divergindo é o comportamento
      correto.
15. **Política de Entropia em Três Casos (seção 5.2)**:
    - Fonte nula ou leitor nulo entregue a uma função de construção
      **DEVE** entrar em pânico na construção, antes de qualquer
      geração.
    - Um gerador de valor zero, e uma referência nula usada como
      receptor, **DEVEM** produzir identificadores válidos e distintos
      de zero em todas as gerações do tipo: pelo relógio nos três
      níveis, por instante, versão 4 e versão 8. Nenhuma pode derrubar o
      processo.
    - Uma leitura que falhe em um gerador construído sobre um leitor
      **DEVE** entrar em pânico; nos apelidos de compatibilidade que
      devolvem erro, o pânico vira o erro de fonte de entropia, e um
      leitor esgotado é o jeito de provocá-lo.
16. **Travas de Alocação (seção 1)**:
    - O objetivo de zero alocação de heap no caminho quente é
      **verificável e obrigatório**, não aspiracional. Estas operações
      **DEVEM** ser livres de alocação, medidas em laço com contagem de
      alocações por iteração: geração binária pelo relógio nos três
      níveis; geração binária por instante (seção 3.6); as duas
      fronteiras e o intervalo (seção 3.5); análise estrita (seção 6.2);
      análise permissiva nos quatro formatos, em texto e em bytes
      (seção 6.3); extração completa dos campos de tempo (seção 7);
      escrita da forma canônica em buffer do chamador com capacidade
      sobrando; geração de versão 4; e as geradoras de tempo gregoriano
      (versões 1, 2 e 6).
    - **Exceção única.** A conversão que devolve uma string, pelo relógio
      ou por instante, pode alocar **exatamente uma vez**, porque o
      resultado é a alocação. Exigir zero aqui é impossível sem mudar a
      assinatura, e quem precisa de zero usa a escrita em buffer do
      chamador, que está na lista acima.
    - **Condição de medição.** A contagem de referência é obtida **sem**
      detector de corrida. O detector altera o código gerado e pode
      contar alocações que não existem em produção; a integração
      contínua da implementação de referência repete as travas em um
      passo próprio, sem o detector, por esse motivo.
    - **Proibição.** Uma falha nestas travas **NÃO DEVE** ser resolvida
      elevando o limite tolerado. A trava existe para denunciar
      regressão de desempenho introduzida por refatoração, e relaxá-la
      remove a única defesa contra ela. Ver a seção 11.1.
    - Em linguagens cujo tipo do identificador só existe no heap, o
      limite passa a ser uma alocação por operação, a do próprio
      resultado, e a divergência **DEVE** ser registrada junto da trava.
17. **Taxonomia de Erros (seção 6.4)**:
    - Para cada linha da tabela da seção 6.4, submeter a entrada
      descrita e exigir o erro descrito, reconhecido pelo tipo e não só
      pela presença. Entradas mínimas: comprimento errado (`abc`); 38
      caracteres com as chaves trocadas por outro caractere; 45
      caracteres com o prefixo `urn:uiid:`; dígito inválido na forma
      canônica; hífen fora de lugar; 15 bytes na construção binária e na
      desserialização binária; um número no lugar da string no JSON do
      tipo anulável; um tipo estranho na leitura de banco.
    - Exigir que todo erro de comprimento e de chaves seja reconhecível
      pelo sentinela de formato, e que o analisador estrito e a extração
      de tempo em texto devolvam exatamente o sentinela, por igualdade
      direta, para todas essas entradas.
    - Exigir que o prefixo URN inválido **não** seja classificado como
      comprimento inválido nem como chaves inválidas: é a assimetria
      registrada, e o teste a trava.
    - Exigir o UUID zerado junto de todo erro, e o receptor inalterado
      nas desserializações. Exigir que o erro de tipo não suportado e o
      de fonte de entropia **não** sejam reconhecidos como erro de
      formato.
18. **Vetores Dourados das Versões de Tempo Gregoriano (seção 4.1)**:
    - A RFC 9562 não publica vetores para as versões 1, 2 e 6, e a
      propriedade de ordenação **não** substitui um vetor: um
      deslocamento errado por uma casa, aplicado igualmente na geração e
      na leitura, satisfaz a ordenação, satisfaz a ida e volta e produz
      um identificador ilegível para qualquer outra implementação. Essa
      classe de defeito só é detectável por tabela externa, e o
      cancelamento simétrico é exatamente o que uma refatoração cuidadosa
      dos dois lados produz.
    - Os vetores abaixo **são contrato**, como os do caso 12. Para uma
      sequência de relógio fixa `0x33c8`, um nó fixo
      `02:11:22:33:44:55` (bit multicast ligado) e, na versão 2, domínio
      Grupo (`1`) com identificador local `1000` (`0x000003e8`):

    **Grupo A** — 2026-01-01T00:00:00.123456789Z (sec=1767225600,
    nsec=123456789), `now` = 139.865.184.001.234.567 = `0x1f0e6a4d0d69687`

    | Versão | 16 bytes | String canônica |
    |:---|:---|:---|
    | 1 | `d0d69687e6a411f0b3c8021122334455` | `d0d69687-e6a4-11f0-b3c8-021122334455` |
    | 6 | `1f0e6a4d0d696687b3c8021122334455` | `1f0e6a4d-0d69-6687-b3c8-021122334455` |
    | 2 | `000003e8e6a421f0b301021122334455` | `000003e8-e6a4-21f0-b301-021122334455` |

    **Grupo C** — 1970-01-01T00:00:00.000000000Z (sec=0, nsec=0), `now`
    = 122.192.928.000.000.000 = `0x1b21dd213814000`, a própria constante
    de deslocamento gregoriano

    | Versão | 16 bytes | String canônica |
    |:---|:---|:---|
    | 1 | `138140001dd211b2b3c8021122334455` | `13814000-1dd2-11b2-b3c8-021122334455` |
    | 6 | `1b21dd2138146000b3c8021122334455` | `1b21dd21-3814-6000-b3c8-021122334455` |
    | 2 | `000003e81dd221b2b301021122334455` | `000003e8-1dd2-21b2-b301-021122334455` |

    - **O que cada linha prova.** A versão 1 fixa a ordem invertida dos
      três campos de tempo: no grupo A, `time_low` = `0xd0d69687`,
      `time_mid` = `0xe6a4` e `time_hi` = `0x1f0`, que aparece como `1f0`
      logo após o nibble de versão. A versão 6 fixa os deslocamentos de
      28 e 12 bits da reordenação, o ponto mais fácil de errar de toda a
      seção 4.1: `time_high` = `0x1f0e6a4d`, `time_mid` = `0x0d69` e
      `time_low` = `0x687`, e os mesmos 60 bits do vetor da versão 1
      reaparecem em outra ordem. A versão 2 fixa a substituição dos
      bytes 0 a 3 pelo identificador local e do byte 9 pelo domínio, com
      os bytes 4 a 8 idênticos aos da versão 1 do mesmo instante, exceto
      o nibble de versão. O grupo C fixa a constante gregoriana nos
      próprios bytes: a versão 1 mostra `0x13814000`, `0x1dd2` e `0x1b2`,
      e a versão 6 mostra `0x1b21dd21`, `0x3814` e `0x000`. A sequência
      `0x33c8` fixa a variante somada aos seis bits altos (`0xb3`) e o
      byte baixo (`0xc8`); na versão 2 o byte 9 vira o domínio e a
      leitura da sequência devolve só `0x33` = 51.
    - **Leitura obrigatória.** A leitura dos campos a partir desses bytes
      **DEVE** devolver os valores de entrada: o instante gregoriano nas
      versões 1 e 6, a sequência de 14 bits (6 na versão 2), o nó, e na
      versão 2 o domínio e o identificador. A leitura de instante da
      versão 2 **DEVE** devolver falso.
    - **Geração obrigatória.** Como o relógio não é injetável (seção
      11.2), a geração é conferida em duas partes: com a sequência e o nó
      fixados nos valores acima, o identificador gerado carrega esses
      campos; e o instante lido de volta, reempacotado por uma função
      escrita **no teste** a partir das fórmulas da seção 4.1, sem chamar
      a biblioteca, reproduz os 16 bytes gerados. É essa segunda parte
      que fecha o cancelamento simétrico: a tabela fixa o leitor e o
      reempacotamento independente fixa o escritor.
    - Os valores **DEVEM** ser calculados fora da implementação: à mão
      pelas fórmulas da seção 4.1, ou conferidos contra outra
      implementação que aceite os campos por parâmetro. Os publicados
      aqui foram calculados por um programa escrito a partir das
      fórmulas, conferidos na versão 1 contra a biblioteca padrão do
      Python (que constrói UUIDv1 a partir dos campos e lê o tempo, a
      sequência e o nó de volta) e, na leitura das versões 1 e 2, contra
      o pacote `github.com/google/uuid`, em `tests/compare`. A versão 6
      **não** tem verificação externa: o pacote do Google, na versão
      1.6.0, escreve o UUIDv6 com o carimbo de 64 bits gravado inteiro
      nos bytes 0 a 7 e a versão sobreposta por cima, que não é a ordem
      de campos da RFC 9562 §5.6, e por isso não serve de referência
      para esta linha; `tests/compare` mede essa diferença. A linha da
      versão 6 vale pela fórmula e pela relação com a linha da versão 1,
      que carrega os mesmos 60 bits.
19. **Conversão Gregoriana Inversa Antes de 1970 (seção 4.1)**:
    - Montar UUIDs de versão 1 e 6 com carimbos anteriores à época Unix e
      exigir da conversão inversa o par **canônico**: um tique antes da
      época devolve `(-1, 999.999.900)`, e nunca `(0, -100)`; a época
      gregoriana de 1582 devolve `(-12.219.292.800, 0)`; meio segundo
      antes da época devolve `(-1, 500.000.000)`.
    - Exigir que o instante construído a partir do par seja o mesmo que
      o obtido pela conversão direta para o tipo de data da linguagem,
      e que a ida (seção 4.1) e a volta se anulem para uma lista de
      carimbos que cubra os dois lados da época.
    - É o único caso que distingue a divisão euclidiana da truncada;
      instantes posteriores a 1970 passam nas duas.
20. **Ausência de Valor por Formato e Recusa do Nó Curto (seções 4.2 e 8)**:
    - Serializar o tipo anulável sem valor para banco, JSON, texto e
      binário e exigir, respectivamente, `NULL`, o literal `null`, a
      sequência vazia e a sequência vazia; desserializar a entrada vazia
      nos quatro e exigir ausência sem erro. Desserializar entrada
      inválida e exigir erro com o booleano falso.
    - Configurar o nó com menos de seis bytes e exigir a recusa
      observável e o estado inalterado; com seis bytes, exigir que os
      identificadores seguintes o carreguem e que a consulta devolva uma
      cópia, não o estado interno.

---

## 11. Decisões de Projeto Firmadas (Não Reabrir)

Esta seção existe para **encerrar** discussões, não para abri-las. Cada
item abaixo foi avaliado, medido quando cabia, e decidido. Uma auditoria
ou revisão que encontre um destes pontos **não deve abrir tarefa pedindo
a mudança** apenas por reconhecer o padrão: a decisão já é a resposta.

Reabrir um item exige **argumento novo**, e a coluna "o que justificaria
rever" diz qual. Preferência documentada, simetria de código ou "o outro
pacote faz diferente" não são argumento novo.

### 11.1 Entropia e desempenho

| Decisão | Data | Motivo | O que justificaria rever |
|:---|:---|:---|:---|
| **A fonte de entropia padrão é o gerador por thread do runtime**, não um pool mantido pela biblioteca nem `crypto/rand`. | 2026-09-11 | O pool custava o par `Get`/`Put`, era esvaziado pelo GC (pagando duas leituras de `crypto/rand` a cada recriação) e descartava itens sob o detector de corrida. A troca mediu -34,7% em paralelo e -5,7% em `GenerateV4`. `crypto/rand` em toda geração custa multiplicado, e existe explicitamente como gerador dedicado. | O runtime da linguagem deixar de oferecer fonte por thread, ou medição mostrando regressão. |
| **Não há contador monotônico** no gerador padrão nem como construtor opcional. | 2026-09-11 | Protótipo com contador de 16 bits e estado atômico: +8,5% em série e **32 vezes** pior em paralelo (7,40 ns para 233,6 ns em 8 núcleos). Exige ainda decidir layout por nível, política de estouro e conviver com a leitura cega de nível na importação, que leria o contador como tempo. Ver 3.4. | Uma construção que dê ordem estrita **sem** estado compartilhado entre threads. |
| **O caminho quente não ganha desvio, indireção nem alocação** para acomodar funcionalidade nova. | permanente | Dezenas de nanossegundos por identificador é o objetivo declarado na seção 1. Uma chamada indireta a mais pode impedir a embutição e custar 1 a 2 ns em um caminho de 40 ns. | Medição antes e depois mostrando custo nulo. |
| **As travas de alocação são requisito verificável da seção 10 (caso 16), não só propriedade desta implementação.** | 2026-09-12 | O objetivo de zero alocação estava declarado na seção 1 sem nenhum caso de teste que o cobrasse, e a exigência de "medição antes e depois" desta tabela não tinha procedimento que a sustentasse. A lista de operações, a exceção única da conversão para texto e a condição de medir sem o detector de corrida viviam só nas instruções de manutenção, que não são especificação. A alternativa, tratar desempenho como propriedade da implementação e não do formato, foi recusada: uma reimplementação poderia alocar em toda geração e se dizer conforme. | Uma linguagem-alvo em que o limite de uma alocação por operação seja comprovadamente inatingível, caso em que a divergência é registrada junto da trava, e não a trava removida. |

### 11.2 Testabilidade

| Decisão | Data | Motivo | O que justificaria rever |
|:---|:---|:---|:---|
| **O relógio não é injetável.** A geração lê o relógio do sistema diretamente. | 2026-09-11 | A motivação original era testar bordas de relógio; isso foi resolvido isolando a decomposição do instante (3.2) em função pura, testada de dentro do pacote. O layout de bits está travado por testes de entropia fixa. Sobrava apenas o vetor dourado de ponta a ponta, que não paga um campo de função no caminho quente. | Necessidade de teste que a função pura de decomposição comprovadamente não cobre. |
| **A duplicação entre a formatação canônica do caminho quente e a dos serializadores é deliberada.** | permanente | A conversão para texto é caminho quente e não deve pagar uma chamada de função por causa dos serializadores. As duas cópias são pequenas e travadas pelos mesmos testes. | Compilador que comprovadamente embuta a chamada sem custo. |
| **O layout de bytes é escrito duas vezes, e só duas: uma no caminho quente e uma compartilhada por tudo que constrói a partir de um instante (seções 3.5 e 3.6).** | 2026-09-11 | O empacotamento compartilhado recebe os bits livres por parâmetro: constantes na fronteira, sorteados na geração por instante. Fundi-lo com a geração pelo relógio poria uma chamada ou um desvio no caminho quente, que a decisão 11.1 proíbe. O limite é esse: **uma** cópia privada, no caminho quente, e nenhuma outra duplicação tolerada. A divergência entre as duas é travada por teste, que gera pelo relógio, lê o instante de volta e regera para ele exigindo bytes idênticos. | Compilador que comprovadamente embuta a chamada sem custo, medido antes e depois. |

### 11.3 Contrato público

| Decisão | Data | Motivo | O que justificaria rever |
|:---|:---|:---|:---|
| **O analisador estrito devolve o erro sentinela puro**, enquanto o permissivo devolve erros embrulhados e específicos. | v0.3.0 | Código existente compara o erro do analisador estrito por igualdade direta. Embrulhar quebraria esses chamadores sem ganho para eles. | Uma versão maior que aceite quebra de compatibilidade. |
| **A validação de forma aceita os valores especiais** nulo e máximo, além de variante RFC com versão de 1 a 8. | 2026-09-11 | A RFC 9562 seções 5.9 e 5.10 define os dois como válidos apesar de não carregarem versão nem variante. Predicados separados distinguem os casos. | Mudança na própria RFC. |
| **A biblioteca não lê interfaces de rede** para obter o nó. | v0.2.0 | Arrastaria a biblioteca de rede para dentro de quem só gera UUIDv7, e expõe a identidade da máquina. O nó sorteado com bit multicast é o caminho recomendado pela RFC 9562 §6.10. Quem quiser um endereço real o lê fora e o entrega. | Nada previsto. |
| **O versionamento permanece em `v0.x`** até a superfície pública assentar. | 2026-09-11 | A `v0.4.0` mudou o gerador padrão e ampliou a API no mesmo ciclo. Um compromisso de estabilidade só faz sentido depois de uso real. | Uso em produção estabilizado, mais revisão da superfície pública inteira e política de compatibilidade publicada. |
| **A construção a partir de um instante — `MinAt`, `MaxAt`, `RangeAt` e `GenerateAt` — recebe o instante por parâmetro, e isso não reabre a decisão 11.2.** | 2026-09-11 | O que a 11.2 recusou foi um campo de função de relógio dentro do `Generator`, no caminho quente, como costura de teste. Aqui o instante é parâmetro de funções separadas, a geração nunca as chama e o caminho quente não ganha desvio nem indireção, então a regra da 11.1 continua satisfeita. Sem elas, a consulta por intervalo — o argumento central para adotar UUIDv7 como chave primária — exige que o chamador monte os 16 bytes à mão, e é justamente o cálculo por nível que ele erra. Os nomes `MinAt`/`MaxAt` foram escolhidos sobre `FloorAt`/`CeilAt` e `LowerBound`/`UpperBound`: conversam com o `Max` que já existe em `values.go`, onde `Max` é o maior UUID absoluto e `MaxAt` o maior de um instante. `RangeAt` devolve intervalo **semiaberto**, que é a forma do SQL que motiva a função. | Uma proposta de fazer a geração chamar estas funções, ou de mover o instante para dentro do `Generator`, que aí sim seria a 11.2. |
| **As fronteiras saturam nas duas pontas da faixa representável**, inclusive levando `micro` e `nano` a 999 no teto. | 2026-09-11 | Uma fronteira é predicado de consulta: o que a torna correta é nunca regredir quando o instante avança. Truncar os bits excedentes, como o empacotamento por deslocamento faria de graça, deixaria a fronteira dar a volta e a consulta devolveria as linhas erradas em silêncio. Saturar no teto é a escolha simétrica ao piso na época que a seção 3.2 já faz embaixo. Zerar `micro` e `nano` na saturação quebraria a monotonicidade na travessia da borda, e há teste para isso. | Nada previsto: a alternativa é aceitar resposta errada em silêncio. |
| **A geração por instante explícito (`GenerateAt`, seção 3.6) vale só para o UUIDv7.** As versões 1, 2 e 6 ficam de fora. | 2026-09-11 | Só o UUIDv7 usa época Unix. A unicidade de v1 e v6 vem do piso de relógio por sequência da seção 4.2, e aceitar um instante arbitrário do chamador fura essa invariante: dá para reemitir um UUIDv1 já produzido. É justamente onde a biblioteca se diferencia do pacote do Google, que zera o piso em qualquer troca de sequência. O UUIDv7 não tem estado compartilhado nem piso, então aceitar o instante não custa nada. | Um caso de uso concreto para v1/v6 por instante, com a função recebendo também sequência e nó, e a responsabilidade pela unicidade transferida ao chamador em letras garrafais. |
| **Os bits livres de `GenerateAt` são sorteados, não zerados.** | 2026-09-11 | O verbo pedido é gerar, e um gerador que devolve o mesmo valor para o mesmo instante colide na primeira repetição. A forma determinística de um instante já existe, e é a seção 3.5: expor uma segunda com nome de gerador convidaria ao mal-entendido mais caro possível. A entropia vem do mesmo gerador do resto da biblioteca, por isso as funções também existem como métodos. | Nada previsto: a alternativa determinística já está coberta por `MinAt`. |
| **A forma é `GenerateAt(nível, instante)`, e não uma família de quatro aridades.** | 2026-09-11 | A proposta original mapeava a aridade no nível: quatro funções por número de argumentos, cada uma com variante em texto, em função de pacote e em método, somando dezesseis símbolos novos. A forma escolhida usa o mesmo par nível-instante que `Generate(nível)` e `MinAt(nível, instante)` já usam, custa quatro símbolos e deixa uma única maneira de dizer nível na biblioteca inteira. A `v0.x` existe para a superfície assentar, e quadruplicar a superfície de geração do UUIDv7 de uma vez vai na direção oposta. | Uso real mostrando que a forma posicional por campos de tempo é necessária, e não só conveniente. |
| **A lista de UUIDs não implementa a interface de ordenação da linguagem.** | 2026-09-11 | A biblioteca padrão do Go ordena com uma função de comparação desde a 1.21, e o `go.mod` já está em 1.22: `slices.SortFunc(lista, UUID.Compare)` resolve em uma linha, sem alocação, reusando o `Compare` que já existe e já é testado. Implementar a interface acrescentaria três métodos exportados para oferecer um caminho mais verboso e mais lento que o que o chamador já tem. A pergunta não é se seria útil, é se seria mais útil que a linha que ele já pode escrever. O que faltava era documentação, não API, e ela foi acrescentada. | Uma interface de terceiros que exija a interface de ordenação clássica e não aceite função de comparação. |
| **A escrita em banco continua sendo texto por padrão, e a forma binária é um tipo à parte.** | 2026-09-11 | Trocar o formato de `Value` quebraria em silêncio quem já tem texto gravado: a mesma coluna passaria a ter duas representações e nenhuma consulta acharia as linhas antigas. Tornar o formato configurável é pior ainda, porque estado global mudaria o comportamento de bibliotecas de terceiros no mesmo processo, e é exatamente o que a recusa de importar `SetRand` já rejeitou. O tipo à parte deixa a escolha explícita no ponto da consulta, sem efeito sobre quem não usa. A leitura sempre aceitou as duas formas e continua aceitando. | Nada previsto: unificar os dois caminhos é a quebra que a decisão evita. |
| **A extração completa (`ImportBinary`/`Import`) é cega quanto ao nível e a leitura por nível (`TimestampWithLevel`) descarta por faixa; as duas políticas são opostas de propósito e não serão unificadas.** | 2026-09-12 | Uma operação entrega os bits, a outra entrega um instante. Unificar tiraria do chamador a única leitura que devolve `rand_a` e o topo de `rand_b` crus, quebraria o vetor do caso 13 da seção 10 e mudaria o resultado público das duas. A leitura cega existe desde a primeira versão e a regra de descarte foi corrigida na v0.4.0; nenhuma das duas estava escrita como regra normativa, e por isso a divergência parecia incoerência em auditoria. Agora as duas estão na seção 7, com a remissão cruzada. | Uma terceira operação que receba o nível e devolva os campos crus com um indicador de validade, sem alterar as duas existentes. |
| **O gerador sem fonte de entropia (valor zero, ou referência nula) recorre à fonte padrão do pacote; fonte nula ou leitor nulo na construção entram em pânico.** | 2026-09-12 | A regra crítica da seção 5.2 manda falhar alto quando a **fonte falha**, e o valor zero era a única exceção não escrita: um auditor que a comparasse com a regra tinha fundamento textual para propor o pânico. Os três casos foram separados na seção 5.2. No valor zero não há degradação, porque a fonte padrão é a mesma que a construção correta instalaria; derrubar o processo em produção por um campo não inicializado penalizaria o chamador por um erro que não afeta a qualidade dos bits. A tolerância vale para o tipo inteiro, sem exceção por versão, e é travada por teste nas versões 7, 4 e 8. | Uma versão maior que aceite quebra de compatibilidade e um caso real em que o silêncio tenha escondido um erro de configuração. |
| **O prefixo URN inválido devolve o sentinela de formato puro, e não um erro próprio, ao contrário das chaves malformadas.** | 2026-09-12 | A assimetria não tem motivo técnico: veio com a primeira versão do analisador permissivo (v0.3.0) e nunca foi registrada. Criar o erro de prefixo é acréscimo de símbolo público e muda o valor devolvido para uma entrada que hoje recebe o sentinela puro; quem verifica pelo sentinela não quebraria, quem compara por igualdade direta, sim. Como a lacuna era de registro e não de comportamento, a escolha foi documentar a assimetria na seção 6.4 e travá-la no caso 17 da seção 10, sem mudar a API na `v0.x`. | Um chamador real que precise distinguir prefixo inválido de dígito inválido, ou a próxima versão maior, quando a taxonomia inteira for revista de uma vez. |
| **A conversão gregoriana inversa usa divisão euclidiana e devolve o par canônico**, com nanossegundos sempre em 0 a 999.999.999, também antes de 1970. | 2026-09-12 | A implementação anterior usava divisão truncada e devolvia resto negativo para instantes pré-1970, igual ao pacote `github.com/google/uuid`; o resultado só era correto porque `time.Unix` normaliza componentes negativas, e a especificação não tinha a volta escrita em lugar nenhum. Uma reimplementação em linguagem que não normalize erraria exatamente na borda que o caso 3 da seção 10 manda testar, e um chamador que consumisse `sec` e `nsec` diretamente recebia um par não canônico de um método público. A troca custa uma comparação fora do caminho quente e não altera o instante devolvido por `Time()`. | Nada previsto: a alternativa é publicar um par não canônico como contrato. |
| **Os vetores dourados das versões 1, 2 e 6 são de leitura e de reempacotamento independente, não de ida e volta por uma geração a partir de campos.** | 2026-09-12 | Uma geração de v1/v6 que aceitasse instante, sequência e nó por parâmetro seria o caminho mais direto para vetores de ida e volta, mas é exatamente o que a decisão sobre `GenerateAt` acima recusou, pelo piso de relógio por sequência. A tabela do caso 18 fixa o leitor; o reempacotamento por uma função escrita no teste, a partir das fórmulas da seção 4.1, fixa o escritor. Juntos fecham a classe de defeito do deslocamento errado aplicado simetricamente nos dois lados, que nenhum teste anterior detectava. | O mesmo que justificaria rever a decisão sobre `GenerateAt` para as versões 1, 2 e 6. |
| **Os rótulos de texto de versão, variante e domínio são apresentação, não contrato; os atalhos de UUIDv2 pelo usuário e grupo do processo são conveniências não normativas.** | 2026-09-12 | Nenhum dos dois entra nos bytes do identificador. Os rótulos podem ser traduzidos; o que é técnico neles (fusão dos códigos de variante 0 e 1, ambiguidade do código 3) está na seção 7. Os atalhos dependem de o sistema ter identificador numérico de usuário, e em Windows gravam `0xFFFFFFFF` para todo processo: por isso são opcionais e carregam a advertência da seção 4.3, em vez de serem exigidos de uma reimplementação. | Nada previsto. |

### 11.4 Como registrar uma decisão nova

Toda decisão de projeto — inclusive a recusa de uma proposta — entra
**nesta seção** e no histórico de mudanças, com a data e o motivo. Uma
proposta recusada sem registro volta na auditoria seguinte, e o custo de
reavaliá-la é pago de novo.
