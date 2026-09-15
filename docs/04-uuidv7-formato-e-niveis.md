# UUIDv7 Multinível: Formato, Níveis, Fronteiras e Geração por Instante

> Parte da especificação de desenvolvimento agnóstica de linguagem.
> Reproduz a seção 3 (3.1 a 3.6) de `docs/SPEC.md` (documento original,
> já removido — ver [INDEX.md](INDEX.md)). Cobre o núcleo desta
> biblioteca: o UUIDv7 com três níveis de precisão temporal embutida, a
> aritmética de tempo segura que os sustenta, a política de ordenação, e
> as duas operações que giram em torno de um instante explícito em vez
> do relógio do sistema — as fronteiras para consulta por intervalo e a
> geração para um instante conhecido.

O UUIDv7 dedica os 48 bits iniciais ao carimbo Unix em milissegundos,
garantindo ordenação temporal natural por comparação byte a byte.

## 3.1 Distribuição de Bits por Nível

| Campo         | Tamanho | Posição (bits) | Bytes   | Nível 1 (RFC)        | Nível 2 (+us)          | Nível 3 (+us +ns)      |
|:--------------|:--------|:---------------|:--------|:---------------------|:-----------------------|:-----------------------|
| `unix_ts_ms`  | 48 bits | 0–47           | 0..5    | ms Unix (UTC)        | ms Unix (UTC)          | ms Unix (UTC)          |
| `ver`         | 4 bits  | 48–51          | 6 (alto)| `0x7`                | `0x7`                  | `0x7`                  |
| `rand_a`      | 12 bits | 52–63          | 6(b)..7 | aleatório            | microssegundos (0..999)| microssegundos (0..999)|
| `var`         | 2 bits  | 64–65          | 8 (alto)| `0b10`               | `0b10`                 | `0b10`                 |
| `rand_b`      | 62 bits | 66–127         | 8(b)..15| aleatório (62 bits)  | aleatório (62 bits)    | 10b ns (0..999) + 52b  |

**Nome por versão e nome por nível.** O Nível 1 **DEVE** existir também
sob o nome que as demais versões têm (`GenerateV7`), e cada um dos três
níveis **DEVE** existir sob um nome que diga o nível (`GenerateV7Level1`,
`GenerateV7Level2`, `GenerateV7Level3`), todos como método do gerador e
como função de pacote, sem parâmetro de nível e sem forma em texto.
`GenerateV7` e `GenerateV7Level1` são o mesmo. Cada nome é um apelido
exato da geração no nível correspondente: mesmos bytes, mesmo consumo de
entropia (seção 3.3) e mesma trava de alocação (seção 10, caso 16, em
[09-vetores-dourados-e-apendice-rfc.md](09-vetores-dourados-e-apendice-rfc.md)).
O nome por versão designa o UUIDv7 da RFC; os nomes por nível existem para
que o nível, que é contrato de toda a coluna (seção 3.5: fronteiras de
níveis distintos não compõem), fique legível no ponto da chamada em vez
de numa constante. A constante continua sendo a única forma de dizer o
nível **como valor**, que é o que a geração por nível, a construção por
instante e a leitura por nível recebem; os nomes são grafias fixas das
três constantes na geração pelo relógio, e nada mais. Os apelidos **NÃO
DEVEM** acrescentar custo ao caminho quente: a implementação permanece
na geração por nível, e os nomes apenas a chamam. Um nome que gere em
nível diferente do que declara é defeito, travado pelo caso 1 da seção
10. Ver
[10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)
seção 11.3.

## 3.2 Aritmética Temporal Segura (Evitando Armadilhas de Relógio)

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
     seção 9 (ver
     [10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)),
     pelo mesmo mecanismo. Em linguagens onde a aritmética sem sinal é o
     caminho natural, como C e Rust, este é o ponto exato em que uma
     transcrição literal da fórmula do item 4 perde a proteção.
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

## 3.3 Sorteio de Entropia Otimizado e Normativo

- **Nível 2 e Nível 3**: o campo `rand_a` carrega os microssegundos
  (não é aleatório). Portanto, a biblioteca **DEVE sortear exatamente uma
  única palavra de 64 bits aleatórios (`r2`)**. O sorteio de uma segunda
  palavra é desperdício de CPU e de entropia criptográfica do sistema.
- **Nível 1 (e níveis desconhecidos)**: sortear **duas palavras de 64 bits
  (`r1` e `r2`)**, usando os 12 bits inferiores de `r1` para preencher
  `rand_a`.
- Essa economia é normativa e deve ser travada por testes de contagem.

## 3.4 Ordenação e Desempate (Sem Contador Monotônico)

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
concorrência inviabiliza o objetivo de desempenho da seção 1
([01-visao-geral.md](01-visao-geral.md)). O raciocínio completo e a
medição estão na seção 11
([10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)).

**Consequência para testes**: é proibido escrever teste de ordenação que
gere UUIDs em laço apertado e conte "regressões" contra um limite
tolerado. Esse teste mede a resolução do relógio do host, não a
biblioteca, e falha de forma permanente em hosts com relógio de
microssegundo. Teste a invariante acima fazendo o instante avançar de
verdade entre as gerações.

---

## 3.5 Fronteiras de Tempo para Consulta por Intervalo

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
|----------|--------------------|------------------|-----------------|
| `Level1` | `fill`             | `fill`           | `fill`          |
| `Level2` | `micro`            | `fill`           | `fill`          |
| `Level3` | `micro`            | `nano`           | `fill`          |

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
nunca as chama. Ver
[10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)
seção 11.3.

---

## 3.6 Geração a Partir de um Instante Explícito

A biblioteca **DEVE** oferecer a geração de um UUIDv7 para um instante
informado pelo chamador, no lugar do instante atual. É o que fecha a
assimetria com a extração da seção 7 (ver
[08-inspecao-serializacao-banco.md](08-inspecao-serializacao-banco.md)):
sem ela, quem reprocessa um histórico, semeia dados de teste ou importa
registros antigos preservando a ordenação da chave monta os 16 bytes à
mão.

**Regra normativa — escopo.** Esta operação vale **apenas para o
UUIDv7**. As versões 1, 2 e 6 usam a época gregoriana e têm a unicidade
garantida pelo piso de relógio por sequência (ver
[06-relogio-e-concorrencia.md](06-relogio-e-concorrencia.md) seção 4.2),
segundo o qual os instantes emitidos com cada sequência são estritamente
crescentes durante toda a vida do processo. Uma função que aceite um
instante arbitrário do chamador **fura essa invariante** e pode reemitir
um UUIDv1 já produzido. O UUIDv7 não tem estado compartilhado nem piso,
então nada se perde ao aceitar o instante.

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
manter cópia própria, pela regra de custo da seção 11.1 (ver
[10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)),
mas as duas construções a partir de instante **não** podem divergir uma
da outra. Duas cópias dessa aritmética divergindo é um defeito que só
aparece em produção, no nível menos usado.

**Consequência para a unicidade.** Como o instante deixa de vir do
relógio, nada impede o chamador de gerar em volume para um único
instante, e aí a margem passa a ser só a dos bits livres. No Nível 3 são
52 bits, o que põe a probabilidade de colisão na casa de um em dois
elevado a 26 gerações **para o mesmo nanossegundo**. É folgado na
prática e **DEVE** estar documentado, porque a geração pelo relógio
nunca expõe o chamador a essa escolha.

A forma da API, a aridade e a nomenclatura estão registradas em
[10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)
seção 11.3.

---

Ver também [03-guia-completo.md](03-guia-completo.md) para exemplos de
uso em Go de `Generate`, `MinAt`/`MaxAt`/`RangeAt` e `GenerateAt`, e
[08-inspecao-serializacao-banco.md](08-inspecao-serializacao-banco.md)
seção 7 para as operações de leitura correspondentes
(`TimestampWithLevel`, `Import`/`ImportBinary`).
