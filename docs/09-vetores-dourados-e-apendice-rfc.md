# Casos de Teste Obrigatórios, Vetores Dourados e Apêndice da RFC 9562

> Parte da especificação de desenvolvimento agnóstica de linguagem.
> Reproduz a seção 10 de `docs/SPEC.md` (documento original, já removido
> — ver [INDEX.md](INDEX.md)): os 21 casos de teste obrigatórios para
> validar uma implementação, incluindo os vetores dourados da extensão
> multinível (exclusivos deste projeto) e das versões baseadas em tempo
> gregoriano (v1, v2 e v6), além dos vetores da própria RFC 9562 para as
> versões baseadas em hash (v3 e v5). Estes vetores são **contrato**: uma
> implementação que produza valores diferentes para as mesmas entradas
> está errada, e mudá-los é mudança de formato de dados, não ajuste de
> teste.

## Casos de Teste Obrigatórios para Validação

Toda reimplementação deve garantir proteção contra estes 12 defeitos
reais listados no catálogo de armadilhas (ver
[10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)),
através dos 21 casos de teste a seguir.

1. **Conformidade de Versão e Variante**:
   - Validar que cada gerador (V1 a V8, os Níveis 1 a 3 de V7, o nome
     por versão `GenerateV7` e os nomes por nível `GenerateV7Level1`,
     `GenerateV7Level2` e `GenerateV7Level3`) define exatamente sua
     respectiva versão e variante `0b10`.
   - Validar que o nome por versão é o Nível 1: com entropia constante,
     `rand_a` e o topo de `rand_b` saem inteiros da fonte, sem campo de
     tempo sub-milissegundo, e o carimbo é o do relógio.
   - Validar que cada nome gera no nível que declara, e não em outro:
     com entropia constante em um, o identificador gerado pelo nome é
     lido de volta no nível declarado (seção 7, em
     [08-inspecao-serializacao-banco.md](08-inspecao-serializacao-banco.md))
     e regerado por instante nesse mesmo nível (seção 3.6, em
     [04-uuidv7-formato-e-niveis.md](04-uuidv7-formato-e-niveis.md)), e
     os 16 bytes **DEVEM** coincidir. A entropia em um deixa `rand_a` em
     `0xfff` e o topo de `rand_b` em `0x3ff`, valores que nenhum campo de
     tempo em 0..999 assume, então um nome que chamasse outro nível
     diverge em pelo menos um campo. Conferir também cada campo: `rand_a`
     em `0xfff` no Nível 1 e em 0..999 nos demais; topo de `rand_b` em
     `0x3ff` nos níveis 1 e 2 e em 0..999 no Nível 3.
2. **Robustez do Analisador contra Mutações**:
   - Executar teste cobrindo **todas as 36 × 256 mutações de um único byte**
     sobre uma string canônica válida: nenhuma mutação pode causar pânico.
   - Submeter o analisador a campanhas de *fuzzing* contínuo.
   - **O fuzzing não para no texto.** A aritmética temporal **DEVE**
     receber campanhas próprias, e por um motivo diferente: no texto o
     risco é leitura fora dos limites, e aqui é saturação, estouro de
     sinal e resto negativo. Tabela de casos escolhidos à mão não varre
     faixa, e é precisamente nas duas metades do inteiro com sinal que o
     estouro da multiplicação por mil (seção 3.2, item 3, em
     [04-uuidv7-formato-e-niveis.md](04-uuidv7-formato-e-niveis.md)) e o
     resto negativo da conversão inversa (seção 4.1, em
     [05-outras-versoes-uuid.md](05-outras-versoes-uuid.md)) se
     manifestam. São dois alvos:
     - **Construção por instante**: recebe dois instantes arbitrários e
       exige, em todos os níveis mais um nível desconhecido, versão 7 e
       variante `0b10` nas duas fronteiras, fronteira inferior nunca
       acima da superior, o valor gerado sempre dentro das fronteiras do
       próprio instante, e monotonicidade quando o segundo instante não
       é anterior ao primeiro.
     - **Conversão gregoriana inversa**: recebe um instante gregoriano em
       toda a faixa do inteiro com sinal e exige o par canônico, com a
       fração entre zero e um segundo e múltipla de 100 ns. **DEVE**
       também detectar estouro: para qualquer entrada negativa, o
       resultado em segundos não pode ser positivo e astronômico
       (indicando que a subtração do deslocamento gregoriano deu a
       volta em inteiro com sinal). Semear o limiar exato de saturação
       e sua vizinhança (um abaixo e um acima).
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
     Nível 3 consomem 1 chamada; Nível 1 consome 2 chamadas. Os nomes
     consomem o mesmo que o nível que apelidam: `GenerateV7` e
     `GenerateV7Level1` consomem 2, `GenerateV7Level2` e
     `GenerateV7Level3` consomem 1.
5. **Vetores Dourados da RFC 9562 para Versões Baseadas em Hash**:
   - Validar que V3 e V5 produzem exatamente os vetores publicados na
     RFC 9562 (Apêndice A), que usam o espaço `NameSpaceDNS` e o nome
     `www.example.com`:
     - V3: `5df41881-3aed-3515-88a7-2f4a814cf09e`
     - V5: `2ed6657d-e927-568b-95e1-2665a8aea6a2`
   - Validar os quatro espaços de nomes contra a tabela da seção 4.4 (em
     [05-outras-versoes-uuid.md](05-outras-versoes-uuid.md)), pela forma
     canônica em texto. O vetor acima só alcança `NameSpaceDNS`: um
     dígito errado em qualquer dos outros três passaria por todos os
     demais casos desta seção.
   - A RFC não publica vetores de geração para os demais espaços de
     nomes; vetores adicionais só devem entrar na suíte se forem
     calculados por uma implementação independente.
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
10. **Fronteiras de Tempo (seção 3.5, em
    [04-uuidv7-formato-e-niveis.md](04-uuidv7-formato-e-niveis.md))**:
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
      esta a trava da duplicação deliberada (ver
      [10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)
      seção 11.2), e ela falha se as duas cópias do empacotamento se
      separarem.
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
15. **Política de Entropia em Três Casos (seção 5.2, em
    [06-relogio-e-concorrencia.md](06-relogio-e-concorrencia.md))**:
    - Fonte nula ou leitor nulo entregue a uma função de construção
      **DEVE** entrar em pânico na construção, antes de qualquer
      geração.
    - Um gerador de valor zero, e uma referência nula usada como
      receptor, **DEVEM** produzir identificadores válidos e distintos
      de zero em todas as gerações do tipo: pelo relógio nos três
      níveis, pelo nome por versão `GenerateV7` e pelos nomes por nível
      `GenerateV7Level1` a `GenerateV7Level3`, por instante, versão 4 e
      versão 8. Nenhuma pode derrubar o processo.
    - Uma leitura que falhe em um gerador construído sobre um leitor
      **DEVE** entrar em pânico; nos apelidos de compatibilidade que
      devolvem erro, o pânico vira o erro de fonte de entropia, e um
      leitor esgotado é o jeito de provocá-lo.
16. **Travas de Alocação (seção 1, em
    [01-visao-geral.md](01-visao-geral.md))**:
    - O objetivo de zero alocação de heap no caminho quente é
      **verificável e obrigatório**, não aspiracional. Estas operações
      **DEVEM** ser livres de alocação, medidas em laço com contagem de
      alocações por iteração: geração binária pelo relógio nos três
      níveis, pelo nome por versão `GenerateV7` e pelos nomes por nível
      `GenerateV7Level1` a `GenerateV7Level3`; geração binária por
      instante (seção 3.6); as duas fronteiras e o intervalo (seção
      3.5); análise estrita (seção 6.2, em
      [07-parsing-e-conversao.md](07-parsing-e-conversao.md)); análise
      permissiva nos quatro formatos, em texto e em bytes (seção 6.3);
      extração completa dos campos de tempo (seção 7); escrita da forma
      canônica em buffer do chamador com capacidade sobrando; **escrita
      dos 16 bytes em buffer do chamador com capacidade sobrando**;
      geração de versão 4; geração de versão 8, com os bits do chamador
      e com os bits sorteados; e as geradoras de tempo gregoriano
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
      remove a única defesa contra ela. Ver
      [10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)
      seção 11.1.
    - Em linguagens cujo tipo do identificador só existe no heap, o
      limite passa a ser uma alocação por operação, a do próprio
      resultado, e a divergência **DEVE** ser registrada junto da trava.
17. **Taxonomia de Erros (seção 6.4, em
    [07-parsing-e-conversao.md](07-parsing-e-conversao.md))**:
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
18. **Vetores Dourados das Versões de Tempo Gregoriano (seção 4.1, em
    [05-outras-versoes-uuid.md](05-outras-versoes-uuid.md))**:
    - A RFC 9562 publica vetor de versão 1 (Apêndice A.1) e de versão 6
      (Apêndice A.5), com o mesmo tempo, a mesma sequência de relógio e
      o mesmo nó — travados em `tests/rfc_appendix_test.go`. Ela **não**
      publica vetor de versão 2, fora do escopo da RFC (seção 5.2), e é
      só para essa versão que a tabela abaixo é a única fonte externa.
      Em qualquer caso a propriedade de ordenação **não** substitui um
      vetor: um deslocamento errado por uma casa, aplicado igualmente na
      geração e na leitura, satisfaz a ordenação, satisfaz a ida e volta
      e produz um identificador ilegível para qualquer outra
      implementação. Essa classe de defeito só é detectável por tabela
      externa, e o cancelamento simétrico é exatamente o que uma
      refatoração cuidadosa dos dois lados produz. A tabela abaixo cobre
      também a sequência e o nó do vetor da RFC, mas com valores
      diferentes dos publicados nela — o vetor 18 é independente, não uma
      cópia do Apêndice A.
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
    - **Geração obrigatória.** Como o relógio não é injetável (ver
      [10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)
      seção 11.2), a geração é conferida em duas partes: com a sequência
      e o nó fixados nos valores acima, o identificador gerado carrega
      esses campos; e o instante lido de volta, reempacotado por uma
      função escrita **no teste** a partir das fórmulas da seção 4.1,
      sem chamar a biblioteca, reproduz os 16 bytes gerados. É essa
      segunda parte que fecha o cancelamento simétrico: a tabela fixa o
      leitor e o reempacotamento independente fixa o escritor.
    - Os valores **DEVEM** ser calculados fora da implementação: à mão
      pelas fórmulas da seção 4.1, ou conferidos contra outra
      implementação que aceite os campos por parâmetro. Os publicados
      aqui foram calculados por um programa escrito a partir das
      fórmulas, conferidos na versão 1 contra a biblioteca padrão do
      Python (que constrói UUIDv1 a partir dos campos e lê o tempo, a
      sequência e o nó de volta) e, na leitura das versões 1 e 2, contra
      o pacote `github.com/google/uuid`, em `tests/compare`. A versão 6
      **não** tem verificação contra esse pacote: o pacote do Google, na
      versão 1.6.0, escreve o UUIDv6 com o carimbo de 64 bits gravado
      inteiro nos bytes 0 a 7 e a versão sobreposta por cima, que não é a
      ordem de campos da RFC 9562 §5.6, e por isso não serve de referência
      para esta linha; `tests/compare` mede essa diferença. Ainda assim a
      versão 6 tem verificação externa independente do pacote do Google:
      o vetor do Apêndice A.5 da própria RFC 9562, travado em
      `tests/rfc_appendix_test.go` (o vetor deste caso 18 é distinto,
      com sequência e nó próprios deste projeto, não uma cópia do
      Apêndice A). A linha da versão 6 também vale pela fórmula e pela
      relação com a linha da versão 1, que carrega os mesmos 60 bits.
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
20. **Ausência de Valor por Formato e Recusa do Nó Curto (seções 4.2 e
    8)**:
    - Serializar o tipo anulável sem valor para banco, JSON, texto e
      binário e exigir, respectivamente, `NULL`, o literal `null`, a
      sequência vazia e a sequência vazia; desserializar a entrada vazia
      nos quatro e exigir ausência sem erro. Desserializar entrada
      inválida em JSON, texto e binário, a partir de um receptor que
      **já continha valor presente**, e exigir erro com o receptor
      inteiro intacto; fazer a mesma leitura recusada pela via de banco e
      exigir erro com o identificador intacto e o booleano falso, que é a
      exceção da seção 6.4.
    - Configurar o nó com menos de seis bytes e exigir a recusa
      observável e o estado inalterado; com seis bytes, exigir que os
      identificadores seguintes o carreguem e que a consulta devolva uma
      cópia, não o estado interno.
21. **Piso Gregoriano Anterior a 1970 na Geração (seção 4.1)**:
    - Alimentar a conversão direta de Unix para tique gregoriano com
      segundos negativos e fração **positiva** — no mínimo
      `(-1, 1)`, `(-1, 500.000.000)` e `(-1, 999.999.900)` — e exigir de
      **todos** exatamente `gregorianOffset`, o deslocamento puro.
    - Exigir separadamente a **não regressão na fronteira**: o resultado
      do último instante antes da época **NÃO PODE** ser maior que o do
      primeiro instante depois dela. É a asserção que distingue o piso
      correto do piso que zera só os segundos, e a única que falha de
      forma inequívoca — as demais podem ser lidas como imprecisão
      tolerável, esta não.
    - **Não é o caso 3, e não pode ser dobrado nele.** O caso 3 cobre o
      piso da época Unix no UUIDv7, onde a seção 3.2 aceita **duas**
      formas de piso; aqui só uma é aceitável, porque zerar apenas os
      segundos projeta o carimbo em até 0,9999999 s à frente da época.
      Também não é o caso 19, que cobre a conversão **inversa**: uma
      implementação pode passar nos dois e ainda assim errar este.
    - **Motivo de o caso existir.** Este defeito esteve presente da
      primeira versão à `v0.5.0` com o caso de teste passando, porque o
      único instante exercitado era `(-1, 0)`, cuja fração já é zero. Um
      caso pré-época sem fração positiva **não** cobre a regra.

---

Ver [11-testes-e-benchmark.md](11-testes-e-benchmark.md) para como rodar
a suíte de testes que implementa estes 21 casos, e
[10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)
para o catálogo de armadilhas históricas que estes casos previnem.
