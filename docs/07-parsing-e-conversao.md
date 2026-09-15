# Algoritmos de Conversão e Parsing Seguro

> Parte da especificação de desenvolvimento agnóstica de linguagem.
> Reproduz a seção 6 de `docs/SPEC.md` (documento original, já removido
> — ver [INDEX.md](INDEX.md)): formatação binário→texto, o analisador
> estrito, o analisador permissivo com seus quatro formatos e a
> taxonomia de erros que os dois compartilham.

## 6.1 Formatação (Binário → String Canônica)

- Utilizar uma tabela de caracteres hexadecimais minúsculos
  `"0123456789abcdef"`.
- Gravar diretamente em um buffer fixo de 36 caracteres, inserindo os
  hifens nos índices 8, 13, 18 e 23 sem alocações intermediárias.

## 6.2 Análise Estrita (String Canônica → Binário)

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

## 6.3 Analisador Permissivo (4 Formatos)

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

## 6.4 Taxonomia de Erros

A biblioteca **DEVE** expor um erro sentinela de formato e erros mais
específicos que o **embrulham**, de modo que a verificação pelo
sentinela (`errors.Is` em Go, ou o equivalente da linguagem) continue
verdadeira para qualquer um deles. Quem trata só a presença de erro não
precisa conhecer a tabela; quem decide pelo tipo, precisa, e é para esse
chamador que ela existe. A tabela é normativa: duas implementações
conformes devolvem o mesmo erro para a mesma entrada.

| Operação | Entrada | Erro devolvido |
|:---|:---|:---|
| Analisador estrito (6.2) e a extração de tempo em texto (seção 7, em [08-inspecao-serializacao-banco.md](08-inspecao-serializacao-banco.md)) | qualquer recusa, inclusive comprimento | sentinela de formato, **puro** |
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
| Fonte de entropia do chamador (leitor) | leitor nulo, ou leitura que falhou | erro de fonte de entropia nos apelidos de compatibilidade que devolvem erro; pânico com o mesmo valor no gerador construído sobre o leitor (seção 5.2, caso 1, em [06-relogio-e-concorrencia.md](06-relogio-e-concorrencia.md)) |

Todo erro de comprimento e de chaves **DEVE** embrulhar o sentinela de
formato. O erro de tipo não suportado e o de fonte de entropia são
famílias à parte e **NÃO** embrulham o sentinela: não são recusas de
texto. A desserialização JSON do tipo simples delega ao codificador da
linguagem, que devolve o erro dele para valores que não são string e o
erro do analisador permissivo para strings recusadas.

**Regra normativa — o analisador estrito não embrulha.** Ele devolve o
sentinela puro em todos os casos, inclusive comprimento errado, para que
a comparação por igualdade direta continue valendo em código existente
(ver
[10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)
seção 11.3). A implementação de referência oferece também um predicado
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
justificaria revê-la estão em
[10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)
seção 11.3.

Em todos os casos de recusa o valor devolvido **DEVE** ser o UUID zerado,
e nenhum byte parcialmente decodificado pode vazar; nas desserializações
com receptor, o receptor **NÃO DEVE** ser alterado. A regra vale para os
dois tipos: no anulável ela alcança **as duas** componentes, o
identificador e o booleano de presença, que **NÃO DEVEM** mudar quando a
entrada é recusada.

**Exceção única — a leitura de valor de banco do tipo anulável.** Ali o
identificador continua intacto, mas o booleano **DEVE** cair para falso.
Não é inconsistência: a interface de leitura de banco recebe um destino
**reaproveitado a cada linha**, e um chamador que ignore o erro leria o
valor da linha anterior como se fosse o da linha que falhou. Derrubar o
booleano transforma esse descuido em ausência de valor, e não em dado
errado. As desserializações não têm destino reaproveitado, e por isso não
têm a exceção. A decisão está em
[10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)
seção 11.3.

Os dois auxiliares que entram em pânico em vez de devolver erro, um para
constantes do próprio código (`MustParse`) e um para encadear com
funções que devolvem par de valores (`Must`), não fazem parte da tabela.
O segundo propaga o próprio erro recebido, para que quem recupere o
pânico o reconheça pelo sentinela.

---

Ver [03-guia-completo.md](03-guia-completo.md) seção "Ler UUIDs escritos
em outros formatos" para exemplos de uso em Go de `Parse`, `ParseBytes`,
`FromString`, `FromBytes`, `MustParse`, `Must` e `Validate`.
