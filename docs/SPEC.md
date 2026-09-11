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
5. **Serialização e Integração**:
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
3. **Decomposição pura**:
   - `unix_ts_ms = (uint64(sec) * 1000) + (uint64(nsec) / 1_000_000)`
   - `sub_ms = uint64(nsec) % 1_000_000`
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
   - Zerar o piso do relógio em **qualquer** troca de sequência (como faz
     o pacote `google/uuid`) também repete UUIDs: com a sequência `A`
     adiantada até o instante 1000, trocar para `B` (piso zerado, relógio
     real em 900) e voltar para `A` faz `A` emitir de novo instantes que
     já emitiu.
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

- **Regra Crítica**: Se a fonte forte de entropia do sistema operacional
  falhar ao semear um gerador ou ao sortear dados para nós e sequências:
  - **A biblioteca DEVE falhar alto e imediatamente (pânico / exceção /
    encerramento)**.
  - **JAMAIS recorra ao relógio do sistema como fallback silencioso**.
    Semear múltiplos geradores com o horário atual produz sequências
    idênticas ou correlacionadas entre threads, levando a colisões
    maciças de UUIDs e previsibilidade total de chaves e identificadores.

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
- **`TimestampWithLevel(Level)`**: recupera microssegundos e
  nanossegundos caso o nível informado tenha embutido esses dados.
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

---

## 8. Serialização, Banco de Dados e Valores Especiais

1. **Serialização em Texto e JSON**:
   - Um UUID serializado em JSON **DEVE ser formatado como string
     canônica entre aspas** (`"0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff"`).
   - **NUNCA** serialize como um vetor/array de 16 números inteiros.
2. **Integração com Banco de Dados**:
   - Suporte a leitura e escrita de UUIDs como string canônica ou binário
     de 16 bytes.
   - Fornecer um tipo `NullUUID` contendo o UUID e um booleano `Valid`
     para campos de tabela que permitem valor `NULL`.
3. **Valores Especiais**:
   - `Nil`: todos os 16 bytes em zero (`00000000-0000-0000-0000-000000000000`).
   - `Max`: todos os 16 bytes em `0xFF` (`ffffffff-ffff-ffff-ffff-ffffffffffff`).
   - `Compare(a, b)`: comparação byte a byte em ordem lexicográfica
     retornando `-1`, `0` ou `1`.

---

## 9. Catálogo de Armadilhas Evitadas (Guia Anti-Regressão)

Toda reimplementação deve garantir proteção contra estes 10 defeitos
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
