# Outras Versões da RFC 9562 (1, 2, 3, 4, 5, 6 e 8)

> Parte da especificação de desenvolvimento agnóstica de linguagem.
> Reproduz a seção 4 de `docs/SPEC.md` (documento original, já removido
> — ver [INDEX.md](INDEX.md)), com exceção da seção 4.2 ("Estado
> Monotônico Compartilhado"), que por ser um mecanismo de concorrência
> compartilhado por v1, v2 e v6 foi movida para
> [06-relogio-e-concorrencia.md](06-relogio-e-concorrencia.md) — leia
> aquele documento junto deste para o quadro completo das versões
> baseadas em relógio.

## 4.1 Versão 1 e Versão 6 (Tempo Gregoriano)

Utilizam o relógio gregoriano em tiques de **100 nanossegundos** desde a
reforma do calendário gregoriano em **1582-10-15T00:00:00Z**.

- **Constante de deslocamento gregoriano**:
  `0x01B21DD213814000` = `122.192.928.000.000.000` tiques até a época
  Unix (1970-01-01).
- **Cálculo do carimbo de 60 bits (`now`)**:
  `now = (uint64(sec) * 10_000_000) + (uint64(nsec) / 100) + gregorianOffset`

  **Regra de piso anterior à época Unix.** Se o relógio do sistema
  devolver um instante anterior a 1970 (`sec < 0`), a geração **DEVE**
  aplicar piso na própria época e produzir exatamente
  `now = gregorianOffset`. O piso **DEVE** zerar as **duas** componentes,
  os segundos e a fração: a fração que um instante pré-época devolve é
  positiva (em Go, `1969-12-31T23:59:59,5Z` é `sec = -1` com
  `nsec = 500.000.000`), então zerar só os segundos projetaria o carimbo
  em até 0,9999999 s **à frente** da época e faria o relógio **regredir**
  ao cruzar a fronteira, do último instante antes dela para o primeiro
  depois. É a mesma regra do item 2 da seção 3.2 (ver
  [04-uuidv7-formato-e-niveis.md](04-uuidv7-formato-e-niveis.md)),
  aplicada à outra época; zerar só os segundos é a armadilha simétrica à
  da divisão truncada na conversão inversa, descrita adiante.

  A conversão sem sinal é o segundo motivo de a regra ser obrigatória:
  em linguagens onde `uint64(sec)` com `sec` negativo dá a volta, o
  resultado fica perto de 1,8 × 10¹⁹, extrapola o campo de 60 bits e
  corrompe a versão e a variante junto.

### Estrutura da Versão 1:
- `time_low` (32 bits, bytes 0..3): 32 bits baixos de `now`.
- `time_mid` (16 bits, bytes 4..5): bits 32..47 de `now`.
- `time_hi_and_ver` (16 bits, bytes 6..7): 4 bits de versão (`0x1`) + bits 48..59 de `now`.
- `clock_seq_and_var` (16 bits, bytes 8..9): 2 bits de variante (`0b10`) + 14 bits de sequência.
- `node` (48 bits, bytes 10..15): identificador de nó (endereço MAC ou pseudoaleatório).

### Estrutura da Versão 6 (K-Sortable Gregoriano):
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

### Conversão Inversa (Gregoriano para Unix)

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

**Armadilha de estouro.** O tipo de tempo gregoriano é um inteiro com
sinal de 64 bits e é público: um chamador pode construí-lo a partir de
dados externos não validados. Quando o valor é menor que
`MinInt64 + gregorianOffset` (aproximadamente −9,10 × 10¹⁸), a
subtração do passo 1 estoura o inteiro com sinal e produz um resultado
positivo grande, mapeando para um instante no futuro distante (~ano
30.800) sem qualquer erro. A implementação **DEVE** saturar: se o valor
cai abaixo do limiar, o cálculo parte de `MinInt64`, devolvendo o menor
instante representável em vez de girar para o futuro. Este padrão é o
mesmo da saturação nas fronteiras da seção 3.5.

**Armadilha de linguagem.** A divisão truncada em direção a zero, que é
o padrão em C, Go, Java e Rust, produz resto **negativo** quando `ticks`
é negativo, e portanto nanossegundos negativos: um tique antes da época
Unix sai como `(0, -100)` em vez de `(-1, 999.999.900)`. Esse par só é
utilizável se a construção de data da linguagem normalizar componentes
negativas, como `time.Unix` faz em Go; uma linguagem que não normalize
produz instante errado ou rejeita a entrada, e o defeito aparece
exatamente na borda pré-1970 que o caso 3 da seção 10 manda testar (ver
[09-vetores-dourados-e-apendice-rfc.md](09-vetores-dourados-e-apendice-rfc.md)).
A divisão euclidiana acima remove a dependência: o par é canônico por
construção, e um chamador que consuma `sec` e `nsec` diretamente, sem
passar por uma construção de data, recebe valores corretos.

O tipo de tempo gregoriano e as suas duas conversões, para o par Unix e
para o instante da linguagem, são públicos (seção 7, em
[08-inspecao-serializacao-banco.md](08-inspecao-serializacao-banco.md)).

O estado compartilhado que sustenta a unicidade de v1 e v6 sob
concorrência — o adiantamento de relógio, a proteção contra repetição em
`SetNodeID`, o piso de relógio por sequência e o identificador de nó —
está descrito em
[06-relogio-e-concorrencia.md](06-relogio-e-concorrencia.md) seção 4.2.

## 4.3 Versão 2 (DCE 1.1 Security)

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
- **Descrição em texto do domínio**: ver a regra dos rótulos na seção 7
  (em
  [08-inspecao-serializacao-banco.md](08-inspecao-serializacao-banco.md)).

## 4.4 Versão 3 e Versão 5 (Baseadas em Espaço de Nomes)

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

Os quatro espaços de nomes predefinidos são os da tabela 3 da seção 6.6
da RFC 9562. Os valores são contrato: um dígito errado muda todo
identificador de versão 3 e 5 derivado do espaço, sem erro nenhum.

| Espaço de nomes | Valor |
|:---|:---|
| `NameSpaceDNS` | `6ba7b810-9dad-11d1-80b4-00c04fd430c8` |
| `NameSpaceURL` | `6ba7b811-9dad-11d1-80b4-00c04fd430c8` |
| `NameSpaceOID` | `6ba7b812-9dad-11d1-80b4-00c04fd430c8` |
| `NameSpaceX500` | `6ba7b814-9dad-11d1-80b4-00c04fd430c8` |

## 4.5 Versão 4 e Versão 8

- **Versão 4**: 16 bytes preenchidos com aleatoriedade. Sobrescrever o
  nibble alto do byte 6 com `0x4` e os bits altos do byte 8 com `0b10`.
- **Versão 8**: Formato livre da RFC 9562 para uso específico de
  aplicações. Manter os 122 bits fornecidos ou preenchê-los com
  aleatoriedade, sobrescrevendo a versão com `0x8` e a variante `0b10`.

---

Ver [03-guia-completo.md](03-guia-completo.md) para exemplos de uso em
Go de cada uma destas versões, e
[06-relogio-e-concorrencia.md](06-relogio-e-concorrencia.md) para o
mecanismo de relógio compartilhado por v1, v2 e v6.
