# Histórico de mudanças — go-loghub-uuid

Guia do histórico do projeto: o que mudou em cada versão, por quê, e o
que isso significa para quem consome a biblioteca. As versões seguem o
versionamento semântico e as tags publicadas são imutáveis (nunca são
movidas com `-f`, para não quebrar o `sum.golang.org` dos usuários).

Convenções de cada seção:

- **Adicionado**: API ou capacidade nova.
- **Alterado**: comportamento existente que mudou; leia com atenção ao
  atualizar.
- **Corrigido**: defeito removido.
- **Documentação**: mudanças só em texto.
- **Decisões**: propostas avaliadas e rejeitadas, com o motivo, para que
  não voltem a ser sugeridas sem argumento novo.

---

## [Não publicado]

### Adicionado

- **`BinaryUUID` e `NullBinaryUUID` em `sql.go`: escrita em coluna
  binária de 16 bytes.** A integração com `database/sql` era
  assimétrica: `Scan` já aceitava 16 bytes crus na leitura, mas `Value`
  sempre escrevia a string canônica. Em MySQL, MariaDB e SQLite, onde
  não existe tipo nativo de UUID e a coluna costuma ser `BINARY(16)` ou
  `BLOB` justamente para economizar espaço e acelerar o índice, isso
  obrigava o chamador a contornar a interface passando `u.Bytes()` na
  consulta, o que derrota o propósito de implementar `driver.Valuer`.
  Trinta e seis bytes de texto contra dezesseis é mais que o dobro por
  linha, replicado em todo índice secundário que referencie a chave.

  A escolha é por conversão no ponto da consulta,
  `db.Exec(..., loghubuuid.BinaryUUID(u))`, sem estado global e sem
  efeito sobre quem não usa. O `Scan` dos dois tipos delega ao de `UUID`,
  então a leitura continua aceitando texto e binário: o tipo existe para
  a escrita, não para restringir a leitura. `BinaryUUID` também tem
  `String`, para continuar legível em log e em mensagem de erro.

  **`Value` de `UUID` não mudou e não vai mudar.** Uma coluna que já
  recebeu texto e passasse a receber binário ficaria com dois formatos
  misturados, e nenhuma consulta acharia as linhas antigas. O comentário
  de `Value` passou a dizer isso e a apontar para `BinaryUUID`, em vez
  do conselho antigo de fatiar o valor à mão.

  **A armadilha está testada explicitamente:** ausência de valor e UUID
  nulo são coisas diferentes e viram a mesma linha se forem confundidas.
  `NullBinaryUUID` com `Valid` falso grava `NULL`; dezesseis bytes
  zerados só saem com `Valid` verdadeiro e o UUID igual a `Nil`.

- **`GenerateAt` e `GenerateAtString` em `construct.go`: geração de
  UUIDv7 para um instante informado pelo chamador.** A biblioteca só
  sabia ler tempo de dentro de um UUID. `Import` e `ImportBinary`
  devolvem os quatro campos de tempo, mas não existia o caminho de
  volta: nenhuma função pública recebia um instante e devolvia um UUID,
  e os dois pontos que leem o relógio eram internos e sem parâmetro.
  Quem reprocessa um histórico, semeia dados de teste ou importa
  registros antigos preservando a ordenação da chave montava os bytes à
  mão.

  As duas existem como funções de pacote e como métodos de `*Generator`,
  seguindo o par `Generate`/`GenerateString`. A forma de método importa
  porque os bits livres são sorteados: quem configurou
  `NewCryptoGenerator` ou `NewGeneratorWith` continua valendo aqui. O
  consumo de entropia por nível é o mesmo de `Generate`, uma palavra de
  64 bits nos níveis 2 e 3 e duas no Nível 1, travado por teste.

  **É gerador, não construtor determinístico.** Duas chamadas com o
  mesmo instante devolvem UUIDs diferentes, com os mesmos campos de
  tempo. A forma determinística de um instante é `MinAt`/`MaxAt`. A
  unicidade vem inteiramente dos bits livres, 74 no Nível 1, 62 no Nível
  2 e 52 no Nível 3; gerando pelo relógio isso nunca é escolha do
  chamador, porque o instante avança, e aqui passa a ser.

  Bordas idênticas às das fronteiras: instante anterior à época degrada
  para a própria época, como em `Generate`; instante posterior a
  `10889-08-02T05:31:50.655999999Z` satura no último representável;
  nível desconhecido vira Nível 1. Nada disso devolve erro, porque
  nenhum gerador desta biblioteca devolve erro.

  Zero alocações na forma binária e uma na forma em texto, travadas em
  `tests/alloc_test.go`. Sai mais barata que `Generate` por não pagar a
  leitura do relógio: cerca de 11 ns no Nível 3 contra cerca de 40 ns.

- **O empacotamento dos 16 bytes passou a existir em um lugar só**, na
  função interna `packV7` de `construct.go`, parametrizada pelos bits
  livres: `MinAt` passa zeros, `MaxAt` passa uns e `GenerateAt` passa
  bits sorteados. A decomposição do instante com saturação nas duas
  pontas virou `saturatedInstant`, no mesmo arquivo, e `bounds.go` ficou
  com as três funções públicas e três linhas de cola. Duas cópias dessa
  aritmética divergindo é um defeito que só apareceria em produção, no
  nível menos usado.

  `Generate` mantém a sua própria cópia do empacotamento, de propósito,
  porque compartilhar poria uma chamada no caminho quente. Essa é a
  única cópia tolerada, e `TestGenerateAtMatchesGenerateLayout` trava a
  divergência: gera pelo relógio com entropia constante, lê o instante
  embutido de volta, regera para ele e exige os 16 bytes idênticos.

- **`MinAt`, `MaxAt` e `RangeAt` em `bounds.go`: as fronteiras de tempo
  de um instante, para consulta por intervalo.** O argumento central
  para adotar UUIDv7 como chave primária é responder a uma janela de
  tempo com o índice da própria chave, sem coluna nem índice de carimbo
  temporal — e a biblioteca não oferecia caminho nenhum para obter os
  dois identificadores que delimitam a janela. `NewV7FromReader` recebe
  entropia, não tempo; `GenerateV8` aceita 16 bytes mas produz outra
  versão. O usuário montava os bytes à mão.

  `MinAt(nível, t)` devolve o menor UUIDv7 que a biblioteca poderia
  gerar em `t` naquele nível, com os bits livres de entropia em zero;
  `MaxAt` devolve o maior, com esses bits em um. `RangeAt(nível, from,
  to)` devolve o par de um intervalo **semiaberto** `[from, to)`, que é
  a forma do SQL que motiva a função:

  ```sql
  SELECT * FROM eventos WHERE id >= ? AND id < ? ORDER BY id
  ```

  As três preservam a versão 7 e a variante RFC, e é isso que as torna
  limites corretos: a fronteira superior do Nível 1 termina em
  `7fff-bfff`, não em `ffff-ffff`. Como todo UUIDv7 válido tem o nibble
  de versão em `7` e o byte 8 na faixa `0x80..0xbf`, as fronteiras de
  fato contêm todos os valores geráveis naquele instante.

  **O cálculo respeita o nível**, que é a parte que o usuário erra
  sozinho: no Nível 2 o campo `rand_a` carrega os microssegundos exatos
  do instante e zerá-lo produziria uma fronteira errada; no Nível 3 ele
  carrega os microssegundos e os 10 bits altos de `rand_b` carregam os
  nanossegundos, restando 52 bits livres. Níveis desconhecidos caem no
  Nível 1, como em `Generate`.

  **Fronteiras de níveis distintos não compõem**, e este é o erro mais
  provável do chamador: uma fronteira de Nível 3 não delimita
  identificadores gravados em Nível 1, porque os bits abaixo do
  milissegundo significam coisas diferentes em cada nível. A falha se
  manifesta como linhas faltando, sem erro nenhum. O aviso está em
  destaque no comentário das três funções, em `docs/DEPLOY-FULL.md` e
  como regra normativa em `docs/SPEC.md` seção 3.5.

  O caminho quente não foi tocado: `uuid.go`, `conversion.go` e
  `import.go` estão idênticos ao commit anterior. As fronteiras recebem
  o instante por parâmetro, são funções de pacote separadas e `Generate`
  nunca as chama. Medem cerca de 5 ns com zero alocações, travadas em
  `tests/alloc_test.go`.

  **As fronteiras saturam nas duas pontas da faixa representável**, ao
  contrário da geração, que só tem piso na época. Abaixo de
  `1970-01-01T00:00:00Z` o resultado é o da própria época, reaproveitando
  `splitUnixInstant` para herdar exatamente o comportamento de
  `Generate`. Acima de `10889-08-02T05:31:50.655999999Z`, o último
  instante que cabe em 48 bits de milissegundos, o resultado é o desse
  instante, **com os microssegundos e nanossegundos em 999**: zerá-los
  faria a fronteira regredir ao cruzar a borda. A saturação é decidida
  sobre os **segundos**, antes da multiplicação por mil, porque um
  `time.Time` comporta anos muito além da faixa do UUIDv7 e o produto
  estoura o inteiro com sinal nas duas direções. Sem essa guarda, um
  instante remoto no futuro cairia no piso da época e um remoto no
  passado produziria um carimbo enorme. Há teste para cada um dos dois
  casos, e o motivo da escolha está em "Decisões".

  **Nada de comportamento existente mudou.** A API é puramente aditiva e
  nenhum caminho anterior foi tocado: quem atualiza da `v0.4.0` não
  precisa mudar nada.

### Infraestrutura

- **`.github/workflows/ci.yml`: ações atualizadas e aviso de cache
  silenciado pela causa.** A execução em `ea6eb0f` passava nos dois jobs
  mas emitia dois avisos em toda rodada.

  O primeiro era a descontinuação do Node 20: `actions/checkout@v4`,
  `actions/setup-go@v5` e `actions/upload-artifact@v4` ainda pediam Node
  20 e o GitHub as forçava em Node 24. Quando esse apoio for retirado as
  ações param, e como `docs/RELEASE.md` proíbe etiquetar sem o job `test`
  verde, um CI quebrado bloquearia a publicação de versão. As três
  subiram para `@v7`, cuja `action.yml` declara `using: node24`,
  conferida no repositório de cada ação. `golangci/golangci-lint-action`
  já estava em `@v9`, que resolve para a v9.3.0, também em Node 24.

  As três só usam entradas estáveis: `checkout` não recebe nenhuma,
  `setup-go` recebe `go-version` e `check-latest`, `upload-artifact`
  recebe `name` e `path`. Todas continuam existindo nas versões novas. A
  única mudança de comportamento da `checkout@v7` é bloquear o checkout
  de fork em `pull_request_target` e `workflow_run`, e este fluxo usa
  `pull_request`.

  O segundo aviso era o cache sem arquivo de dependências, consequência
  direta de a biblioteca não ter nenhuma. `setup-go` passou a receber
  `cache: false` nos três jobs, que é o correto para um módulo sem
  dependências: não há o que cachear e o passo deixa de tentar. Criar um
  `go.sum` vazio só para calar o aviso seria um arquivo mentiroso. Um log
  cheio de avisos inofensivos é como um aviso real passa despercebido.

  **Falta exercitar.** Os jobs `test-os` e `deep` não rodam em push para
  `main`, então precisam de `workflow_dispatch` para serem verificados
  com as versões novas.

### Alterado

- **`GregorianTime.UnixTime` em `clock.go` devolve o par canônico
  também antes de 1970.** A conversão usava divisão truncada, como o
  pacote `github.com/google/uuid`, e para carimbos anteriores à época
  Unix devolvia resto negativo: um tique antes da época saía como
  `(0, -100)` em vez de `(-1, 999999900)`. O instante devolvido por
  `Time()` já era correto, porque `time.Unix` normaliza componentes
  negativas, mas o método é público e quem consumia `sec` e `nsec`
  diretamente recebia um par não canônico. A divisão passou a ser
  euclidiana, com resto sempre não negativo; custa uma comparação, fora
  do caminho quente, e `Time()` devolve exatamente o mesmo instante de
  antes para todo o campo de 60 bits, o que está travado por teste. A
  única saída que muda é a de `UnixTime` para carimbos pré-1970. Veio da
  auditoria de suficiência da especificação (ver Documentação): a volta
  de gregoriano para Unix não estava escrita em lugar nenhum, e a forma
  escolhida para o texto normativo é a que não depende de a
  linguagem-alvo normalizar. `docs/MIGRATION.md` registra a diferença.

### Corrigido

- **A explicação de como o pacote `github.com/google/uuid` repete um
  UUIDv1 estava errada**, em `docs/SPEC.md` seção 4.2 e no `CLAUDE.md`.
  O texto dizia que a repetição vinha de **adiantamento acumulado**: uma
  sequência adiantada até um instante futuro, trocada e retomada com o
  piso zerado, reemitiria instantes que já usou.

  O pacote do Google **não adianta o relógio**. Quando o instante não
  avança, ele incrementa a sequência de 14 bits e mantém o instante de
  parede, então nenhuma sequência fica adiantada e o cenário descrito
  não pode ocorrer lá. Medido: 200 mil gerações seguidas deixam o
  adiantamento em zero e movem a sequência de 2570 para 12379.

  **A conclusão estava certa, o mecanismo não.** A repetição existe e foi
  medida em cerca de 7% das tentativas, mas por outra via: o piso do
  relógio é a única proteção contra reemitir um instante, e zerá-lo na
  troca de sequência desarma essa proteção. Duas gerações que caiam no
  mesmo tique de 100 nanossegundos, com a mesma sequência, devolvem o
  mesmo instante e o mesmo UUID.

  As duas escolhas andam juntas e a distinção importa: quem zera o piso
  normalmente não adianta o relógio. É justamente por **esta** biblioteca
  adiantar que zerar o piso aqui seria muito pior do que é lá — a janela
  deixaria de ser um tique e passaria a ser todo o adiantamento
  acumulado. A especificação agora diz isso, com um aviso explícito de
  que o mecanismo é fácil de descrever errado.

### Documentação

- **`README.md` e `docs/DEPLOY-FAST.md` passaram a citar as APIs novas.**
  A consulta por intervalo é o argumento prático para adotar UUIDv7 como
  chave primária, e não aparecia em nenhuma das duas portas de entrada do
  projeto: quem chegava pelo README não via a única capacidade que as
  bibliotecas concorrentes não têm. O README ganhou um item na lista de
  recursos e uma seção com a consulta SQL; o guia rápido ganhou duas
  seções curtas, uma de consulta por faixa e outra de geração por
  instante ao importar histórico. As duas repetem o aviso sobre misturar
  níveis na mesma coluna.
- **`tests/compare/`, módulo aninhado que prova em código a diferença de
  comportamento com o pacote `github.com/google/uuid`.** As afirmações
  comparativas do projeto eram todas qualitativas, e uma delas, sobre
  software alheio e usada como diferencial, estava com o mecanismo
  errado (ver Corrigido). Agora há três testes: a repetição de UUIDv1 do
  outro pacote ao sair de uma sequência de relógio e voltar, a ausência
  dela aqui sob o roteiro idêntico, e a medição lado a lado das duas
  escolhas de avanço de relógio.

  A ausência de dependências é característica do projeto, então a
  dependência fica em quarentena num `go.mod` próprio: `go test ./...` na
  raiz não desce em módulos aninhados, `go list -m all` na raiz continua
  imprimindo uma linha só e não há `go.sum` na raiz. O CI rápido não roda
  este módulo, porque ele precisa de rede e o lançamento de uma versão do
  pacote de terceiros não pode quebrar o CI desta biblioteca.

  **Sem tabela de benchmark comparativo, por decisão** — ver Decisões.
  `docs/MIGRATION.md` ganhou a seção 4.1, com a tabela das duas escolhas
  de relógio lado a lado e o aviso de conferir o código de quem chama
  `SetClockSequence`.
- **Registro de decisões firmadas em `docs/SPEC.md` seção 11.** As
  decisões tomadas até aqui viviam só no histórico deste arquivo, que é
  cronológico: uma auditoria que lesse a especificação encontrava o
  código e o padrão, mas não o motivo de ele ser assim, e reabria a
  discussão. A seção nova é normativa e organizada por tema (entropia e
  desempenho, testabilidade, contrato público), com quatro colunas:
  decisão, data, motivo e **o que justificaria revê-la**. Cobre a fonte
  de entropia do gerador padrão, a ausência de contador monotônico, a
  proibição de custo novo no caminho quente, o relógio não injetável, as
  duas duplicações deliberadas de código, o erro sentinela puro do
  analisador estrito, a semântica da validação de forma, a recusa de ler
  interfaces de rede e a permanência em `v0.x`. O último item da seção
  obriga a registrar ali toda decisão nova, inclusive recusas.
- **Regra de ordenação promovida a normativa**, em `docs/SPEC.md` seção
  3.4. A especificação descrevia o layout de bits e a ordenação temporal,
  mas nunca dizia o que acontece dentro do mesmo instante embutido — o
  vazio exato que fazia a proposta do contador monotônico voltar. Agora
  está escrito que o desempate é aleatório, que empates entre gerações
  consecutivas são o caso comum, que contador é proibido no gerador
  padrão, e que teste de ordenação em laço apertado mede o relógio do
  host, não a biblioteca.
- **Caso de teste obrigatório 9** em `docs/SPEC.md` seção 10: restaurar o
  estado global de relógio ao fim de cada teste, sem paralelismo, com a
  suíte verde sob repetição e ordem embaralhada.
- `CONTRIBUTING.md` e `STARTHERE.md` apontam para o registro de decisões
  antes de propor mudança de projeto, e dizem o que conta como argumento
  novo (medição própria, caso de uso concreto, mudança na RFC) e o que
  não conta (preferência de estilo, "outro pacote faz diferente").
- `CLAUDE.md`: regra de isolamento do estado global nos testes, com
  `withIsolatedClockState` e a proibição de `t.Parallel()` nas versões 1,
  2 e 6; ponteiro para a seção 11 antes de propor mudanças; a nota de
  ordenação passou a dizer que o contador foi **decidido** contra, com a
  medição, e não apenas que não existe.
- Referências corrigidas a símbolos que deixaram de existir na `v0.4.0`:
  `strongSeed` em `CLAUDE.md` e `docs/TEST-AND-BENCHMARK.md`, e os
  arquivos `race_enabled_test.go` e `race_disabled_test.go` na árvore do
  `STARTHERE.md`, que agora lista `clockstate_test.go`.
- **`docs/DEPLOY-FULL.md` ganhou a seção "Consultar por intervalo de
  tempo"**, com a consulta SQL completa, a variante fechada com
  `BETWEEN`, a tabela de precisão por nível e o aviso destacado sobre
  misturar níveis. Nenhum documento do projeto mencionava consulta por
  intervalo até aqui, que é o principal motivo de a biblioteca existir.
  `docs/MIGRATION.md` seção 5 registra que o pacote do Google não tem
  equivalente; `STARTHERE.md` lista as três funções e o arquivo novo;
  `docs/TEST-AND-BENCHMARK.md` lista os quatro benchmarks novos.
- **`docs/SPEC.md` atualizado em seis pontos**, para que a
  especificação continue bastando por si só para reimplementar a
  biblioteca do zero:
  - **Seção 1** (escopo) ganhou o item 5, "Construção a Partir de um
    Instante Explícito", que é a operação inversa da extração do item 4:
    ali se lê o tempo de um identificador, aqui se constroem
    identificadores para um tempo. Cobre as fronteiras e a geração por
    instante. "Serialização e Integração" passou de item 5 para 6.
  - **Seção 3.5**, nova, com o cálculo normativo por nível, a tabela de
    preenchimento dos bits livres, as regras de precisão, de níveis que
    não compõem, de saturação nas duas pontas e do intervalo semiaberto.
  - **Seção 3.2** (aritmética temporal) ganhou a regra 3, sobre o
    estouro da multiplicação por mil quando o instante vem por
    parâmetro. A regra antiga de decomposição pura virou item 4.
  - **Seção 7** (contrato de API) passou a listar `MinAt`, `MaxAt` e
    `RangeAt`, separadas das demais porque não são métodos de um UUID e
    sim derivações a partir de um instante.
  - **Seção 9** (catálogo de armadilhas) passou de 10 para 12 linhas,
    com o estouro de `sec * 1000` e a fronteira que trunca em vez de
    saturar. As duas devolvem resultado errado em silêncio, que é o
    critério da tabela.
  - **Seção 10** ganhou o caso de teste obrigatório 10, com as sete
    verificações exigidas das fronteiras.
- Dois exemplos executáveis novos em `example_test.go`, `ExampleMinAt` e
  `ExampleRangeAt`. Como as fronteiras recebem o instante por parâmetro,
  eles têm saída verificável sem depender do relógio.
- **`docs/DEPLOY-FULL.md` ganhou a seção "Gerar a partir de um instante
  conhecido"**, logo depois da importação, porque é o sentido inverso
  dela. Traz o exemplo de reprocessar um histórico preservando a ordem
  da chave, o aviso de que a função é geradora e não determinística, e a
  explicação de por que gerar com `Generate` na importação faria a chave
  refletir a ordem de importação em vez da do histórico.
- **`docs/SPEC.md` atualizado em mais seis pontos** pela geração por
  instante:
  - **Seção 1** teve o item 5 reescrito para cobrir as duas operações de
    construção a partir de um instante, e não só as fronteiras.
  - **Seção 3.6**, nova, com as cinco regras normativas: escopo restrito
    ao UUIDv7, bits livres sorteados, entropia vinda do gerador do
    chamador, mesma decomposição com saturação das fronteiras e um só
    empacotamento compartilhado.
  - **Seção 7** passou a listar `GenerateAt` e `GenerateAtString`.
  - **Seção 10** ganhou o caso de teste obrigatório 11, com a trava de
    não divergência entre as duas cópias do empacotamento.
  - **Seção 11.2** teve a linha da duplicação reescrita: o limite passou
    a ser explícito, uma cópia privada no caminho quente e nenhuma outra.
  - **Seção 11.3** ganhou as três decisões desta rodada.
- `STARTHERE.md` lista `construct.go` na árvore e as cinco funções de
  construção por instante; `docs/MIGRATION.md` seção 5 registra que o
  pacote do Google não tem equivalente, porque lá o único ponto que lê o
  relógio é interno e sem parâmetro; `docs/TEST-AND-BENCHMARK.md` lista
  os três benchmarks novos; `CLAUDE.md` descreve `construct.go` e corrige
  a nota de sincronia do layout, que agora aponta para `packV7`.
- `ExampleGenerateAt` em `example_test.go`, com entropia fixa para ter
  saída verificável, mostrando a ida e volta com `ImportBinary`.
- **`docs/SPEC.md` publica os vetores dourados da extensão multinível**,
  no caso de teste obrigatório 12. A especificação se apresenta como
  suficiente para reimplementar a biblioteca do zero, e cumpria isso
  para as versões da RFC, que têm vetores publicados no Apêndice A da
  RFC 9562. Para a extensão multinível, que é o diferencial do projeto e
  não tem vetor publicado em lugar nenhum, não havia nada contra o que
  uma reimplementação pudesse se conferir — justamente na parte que não
  é padrão, onde é mais fácil errar.

  São dezoito vetores: três instantes de referência, os três níveis e as
  duas pontas de entropia, com os 16 bytes e a string canônica de cada
  um. As entradas são um instante e o valor dos bits livres, que é
  exatamente o que `MinAt` e `MaxAt` produzem, então os vetores saem sem
  relógio e sem tocar em `Generate`. Vale lembrar que a proposta
  arquivada de relógio injetável existia para viabilizá-los, e foi
  recusada na seção 11.2: as fronteiras resolveram o mesmo problema de
  graça.

  Os valores foram calculados por uma implementação independente,
  escrita a partir das regras das seções 3.1, 3.2 e 3.5, e só então
  conferidos contra esta biblioteca. As duas concordaram nos dezoito. Um
  vetor produzido pela própria implementação e conferido contra ela
  mesma não provaria nada, e a especificação agora diz isso no texto.

  `tests/golden_test.go` guarda a mesma tabela e quebra se qualquer bit
  mudar de lugar, incluindo uma verificação de que a tabela não perdeu
  linhas e outra de que um instante pré-época produz exatamente os
  vetores da própria época.

### Decisões

- **As fronteiras de tempo não reabrem a decisão do relógio não
  injetável** (`docs/SPEC.md` seção 11.2). O que aquela decisão recusou
  foi um campo de função de relógio dentro do `Generator`, no caminho
  quente, como costura de teste. Aqui o instante é parâmetro de funções
  separadas, a geração nunca as chama e o caminho quente não ganha
  desvio nem indireção. A adjacência entre os dois assuntos e a decisão
  de implementar estão registradas na seção 11.3.
- **Nomenclatura `MinAt`/`MaxAt`**, escolhida sobre `FloorAt`/`CeilAt` e
  `LowerBound`/`UpperBound`. Ela conversa com o `Max` que já existe em
  `values.go`: `Max` é o maior UUID absoluto, `MaxAt` o maior de um
  instante. `FloorAt`/`CeilAt` é vocabulário de arredondamento e sugere
  ajustar um valor existente, não derivar uma fronteira.
- **`RangeAt` devolve intervalo semiaberto**, não fechado, porque
  `id >= lo AND id < hi` é a forma da consulta que motiva a função. Para
  o intervalo fechado, `MinAt` e `MaxAt` continuam disponíveis.
- **O layout de bytes é escrito duas vezes, e só duas**, registrado em
  `docs/SPEC.md` seção 11.2 ao lado da duplicação já existente entre a
  formatação canônica do caminho quente e a dos serializadores. Uma
  cópia é privada de `Generate`, no caminho quente; a outra é `packV7`,
  compartilhada por tudo que constrói a partir de um instante. Fundir as
  duas poria uma chamada ou um desvio no caminho quente, que a decisão
  11.1 proíbe. Sem esse registro, a próxima auditoria abriria um achado
  de DRY contra `packV7`. `CLAUDE.md` passou a listar três invariantes de
  duplicação deliberada, não duas, e a avisar que `Generate` não tem
  ponteiro de volta: quem mudar o layout lá precisa seguir até
  `construct.go` à mão, e é `TestGenerateAtMatchesGenerateLayout` que
  pega o esquecimento.
- **Saturar em vez de truncar** acima da faixa representável. Truncar os
  bits excedentes sairia de graça do empacotamento por deslocamento e
  concordaria com `Generate`, mas uma fronteira é predicado de consulta:
  o que a torna correta é nunca regredir quando o instante avança. Uma
  fronteira que dá a volta devolve as linhas erradas em silêncio.
  Registrado na seção 11.3.
- **`GenerateAt` vale só para o UUIDv7.** Estender para as versões 1, 2
  e 6 foi recusado: elas usam a época gregoriana e têm a unicidade
  garantida pelo piso de relógio por sequência, segundo o qual os
  instantes emitidos com cada sequência são estritamente crescentes
  durante toda a vida do processo. Aceitar um instante arbitrário do
  chamador fura essa invariante e permite reemitir um UUIDv1 já
  produzido. É justamente o ponto em que a biblioteca se diferencia do
  pacote do Google, que zera o piso em qualquer troca de sequência.
  Reabrir exige receber sequência e nó explicitamente e transferir a
  responsabilidade pela unicidade ao chamador. Registrado na seção 11.3.
- **Os bits livres de `GenerateAt` são sorteados, não zerados.** A
  alternativa determinística foi recusada porque já existe, e se chama
  `MinAt`. Expor uma segunda forma determinística com o verbo "gerar"
  convidaria ao mal-entendido mais caro da API, o de usar como
  identificador único algo que colide na primeira repetição de instante.
  Registrado na seção 11.3.
- **Sem tabela de benchmark comparativo contra `github.com/google/uuid`.**
  A metade de valor duradouro da comparação é a diferença de
  comportamento, que é uma afirmação de correção e agora está provada em
  código. A tabela de velocidade envelheceria a cada versão do pacote de
  terceiros, exigiria medir dois pares de fonte de entropia para não
  favorecer esta biblioteca por um motivo que não é mérito de projeto, e
  o custo de manutenção recorrente não se paga. Uma tabela que só mostra
  vitórias não é medição, é anúncio.
- **A lista `UUIDs` não vai implementar `sort.Interface`.** Recusado: a
  biblioteca padrão já ordena com função de comparação desde o Go 1.21 e
  o `go.mod` está em 1.22, então
  `slices.SortFunc(lista, uuid.UUID.Compare)` resolve em uma linha, sem
  alocação, reusando o `Compare` que já existe e já é testado. A
  expressão de método vale `func(UUID, UUID) int`, que é a assinatura
  esperada. Implementar `Len`, `Less` e `Swap` acrescentaria três
  métodos exportados para oferecer um caminho mais verboso e mais lento
  que o que o chamador já tem. O que faltava era documentação, não API:
  `docs/DEPLOY-FULL.md` ganhou a seção "Ordenar uma lista". Registrado na
  seção 11.3.
- **A forma é `GenerateAt(nível, instante)`, e não uma família de quatro
  aridades.** A proposta original mapeava o número de argumentos no
  nível, somando dezesseis símbolos novos entre funções de pacote,
  variantes em texto e métodos. A forma escolhida reusa o par
  nível-instante que `Generate(nível)` e `MinAt(nível, instante)` já
  usam, custa quatro símbolos e deixa uma única maneira de dizer nível na
  biblioteca inteira. A `v0.x` existe para a superfície assentar, e
  quadruplicar a superfície de geração do UUIDv7 de uma vez ia na direção
  contrária. Registrado na seção 11.3.

---

## [v0.4.0] — 2026-09-11

Passagem de auditoria estática de 2026-09-10 sobre a `v0.3.0`: seis
problemas encontrados e corrigidos, todos verificados com `gofmt`,
`go vet` e a suíte sob detector de corrida no Go 1.22 (mínimo declarado)
e no Go 1.27. Em 2026-09-11, a integração contínua foi ampliada (linter,
outros sistemas, cobertura, corpus de fuzzing), a documentação ganhou
exemplos executáveis, guia de release, política de segurança e guia de
contribuição, e as propostas em aberto das revisões de 2026-08-27 foram
decididas — entre elas a troca da fonte de entropia do gerador padrão.

### Alterado

- **O gerador padrão passou a ler do gerador do runtime do Go.**
  `NewGenerator` tirava entropia de um `sync.Pool` de PRNGs PCG, cada um
  semeado de `crypto/rand`; agora usa as funções de pacote de
  `math/rand/v2`, que desde o Go 1.22 leem uma instância de ChaCha8 por
  thread, semeada pelo sistema operacional. O ChaCha8 é uma cifra de
  fluxo e resiste a predição, enquanto o PCG podia ter o estado
  reconstruído a partir de poucas amostras — era o único ponto fraco de
  segurança que a biblioteca documentava. A recomendação para segredos
  continua sendo `NewCryptoGenerator`, e o UUIDv7 continua expondo o
  instante de criação. Medido neste repositório (Apple M2, Go 1.27,
  `GOGC=off GOMAXPROCS=4`, seis execuções): `GenerateLevel3Parallel` caiu
  de 16,93 ns para 11,06 ns (-34,7%), `GenerateV4` de 12,36 ns para
  11,65 ns (-5,7%) e a geração em série de 2% a 6%; as versões 1, 2, 5 e
  6, que não usam essa fonte, não mudaram. A tabela completa está em
  `docs/TEST-AND-BENCHMARK.md` seção 4. `NewGeneratorWith`,
  `NewGeneratorWithReader` e `NewCryptoGenerator` não mudaram, e o número
  de palavras sorteadas por nível continua o mesmo (uma nos níveis 2 e 3,
  duas no nível 1). Sem o pool, `strongSeed` e o import de `crypto/rand`
  saíram de `uuid.go`. (`uuid.go`)
- **As travas de alocação passaram a valer também sob `-race`.** Com o
  `sync.Pool`, o detector de corrida descartava itens de propósito e as
  realocações do PRNG entravam na conta de `AllocsPerRun`, o que obrigava
  a pular três travas sob o detector. Sem pool não há estado a recriar, e
  as travas passam nos dois modos. A integração contínua mantém o passo
  dedicado sem detector, que continua sendo a medição de referência.
  (`tests/alloc_test.go`, `.github/workflows/ci.yml`)
- **`Scan` trata texto vazio como ausência de valor.** `UUID.Scan`
  passou a gravar o UUID nulo, sem erro, para `""` e para `[]byte{}`,
  como já fazia para `NULL` e como faz o pacote `github.com/google/uuid`.
  `NullUUID.Scan` produz `Valid` falso nesses casos. Antes, uma coluna de
  texto com valor vazio fazia a leitura falhar com erro de comprimento,
  o que quebrava aplicações migradas do pacote de origem. Texto inválido
  continua devolvendo erro sem alterar o receptor. (`sql.go`)
- **`IsInvalidLengthError` reconhece erros embrulhados.** Passou a usar
  `errors.Is`, então um erro de comprimento envolvido por
  `fmt.Errorf("...: %w", err)` continua sendo reconhecido, como no pacote
  de origem. Resultado idêntico para erros não embrulhados. (`parse.go`)
- **`TimestampWithLevel(Level3)` descarta os dois campos quando um
  denuncia ruído.** Se os microssegundos lidos de `rand_a` estiverem
  fora de 0..999, os nanossegundos do topo de `rand_b` também são
  ignorados, porque os dois vêm da mesma geração e um `rand_a` aleatório
  prova que `rand_b` também é. Antes, o campo que por acaso coubesse na
  faixa era somado ao instante. Só muda o resultado para UUIDs que não
  são do nível informado. (`inspect.go`)
- **`SetClockSequence` mantém um piso de relógio por sequência.** Cada
  sequência de relógio lembra o último instante que emitiu; voltar a uma
  sequência já usada continua a partir dali, e só a entrada em uma
  sequência inédita descarta o adiantamento acumulado do relógio
  interno. `SetClockSequence(-1)` passou a sortear sempre uma sequência
  ainda não usada no processo, e por isso é a forma determinística de
  ressincronizar com o relógio do sistema. Antes, qualquer troca zerava
  o piso, e voltar a uma sequência antiga enquanto o relógio real ainda
  estava atrás do adiantamento podia repetir um UUIDv1 ou UUIDv6. A
  invariante garantida agora: para cada sequência, os instantes
  emitidos são estritamente crescentes durante toda a vida do processo,
  logo nenhum par (instante, sequência) se repete. Difere do pacote
  `github.com/google/uuid`, que tem a fraqueza antiga. O caminho do
  UUIDv7 não foi tocado. (`clock.go`)

### Corrigido

- **`NullUUID.UnmarshalJSON` interpreta escapes JSON.** Uma string com
  sequências como `\u0030` era passada crua ao analisador e rejeitada,
  enquanto o tipo `UUID`, que usa o `encoding/json`, a aceitava. Agora,
  se houver barra invertida no conteúdo, a decodificação é delegada ao
  `encoding/json`; sem escapes, o caminho continua direto e sem
  alocação. Erros de sintaxe JSON viram `ErrInvalidFormat`. (`sql.go`)

### Adicionado

- **Integração contínua** em `.github/workflows/ci.yml`. A cada push,
  pull request e tag: `gofmt` (na versão estável), `go vet`, `go build`,
  suíte em `-short` sob `-race`, travas de alocação em passo próprio e
  um benchmark curto com `-benchmem`, em matriz com Go 1.22 e a versão
  estável. Semanalmente e sob demanda: suíte completa com os testes de
  massa e 60 segundos de fuzzing em cada analisador.
- **Isolamento do estado global de relógio entre os testes.** O nó e a
  sequência de relógio das versões 1, 2 e 6 são globais do pacote, e
  vários testes os alteravam; `TestNodeIDIsRecoverable` fixava um nó com
  cara de endereço MAC real e nunca o devolvia. Nada quebrava porque a
  suíte é sequencial e nenhum teste posterior conferia o nó — uma falha
  esperando `t.Parallel()` ou um teste novo para aparecer longe da causa.
  O auxiliar `withIsolatedClockState(t)` guarda o nó e, por `t.Cleanup`,
  devolve-o e entra em uma sequência inédita; restaurar é seguro por
  causa do piso de relógio por sequência. Com o nó padrão preservado,
  ficou possível conferi-lo: `TestDefaultNodeIsMulticast` checa o bit
  multicast que a RFC 9562 §6.10 pede para nós sorteados. A suíte passa
  com `-count 3` e com `-shuffle on`. Sem mudança em código de produção.
  (`tests/clockstate_test.go`, `tests/versions_test.go`)
- **`AppendTo(dst []byte) []byte`** escreve a forma canônica de 36 bytes
  no fim do buffer do chamador e devolve o slice estendido, sem alocar
  quando há capacidade. É o caminho para serializar grandes volumes:
  medido aqui (Apple M2, Go 1.27) em ~18,8 ns e zero alocações, contra
  ~26 ns e uma alocação de 48 bytes de `String`. Reutilize o buffer com
  `buf = u.AppendTo(buf[:0])`; `dst` nulo é válido. `AppendText` expõe a
  mesma escrita com a assinatura de `encoding.TextAppender`, do Go 1.24 —
  a interface não é referenciada em lugar nenhum, então o método compila
  também no Go 1.22 e o `go.mod` não sobe. `MarshalText` passou a
  delegar a `AppendTo`, com a mesma alocação única de antes; `URN` e
  `String` ficaram como estavam, porque ali o `AppendTo` custaria uma
  alocação a mais. (`encoding.go`)
- **`Bytes() []byte`** devolve uma cópia dos 16 bytes. Ao contrário de
  `u[:]`, não aponta para o valor de origem, e ao contrário de
  `MarshalBinary` não carrega um erro sempre nulo. Serve também para
  contornar a armadilha de `%x` sobre um `UUID`, que formata a string
  canônica por causa de `fmt.Stringer`. (`values.go`)
- **`IsValid() bool`** confere variante e versão em uma chamada:
  verdadeiro para a variante RFC com versão de 1 a 8. `Nil` e `Max` são
  aceitos, porque a RFC 9562 seções 5.9 e 5.10 os define como valores
  especiais válidos apesar de não carregarem versão nem variante; use
  `IsZero` e `IsMax` para distingui-los. Não é uma verificação de
  UUIDv7: um UUIDv4 de outra origem também é válido. (`values.go`)
- `FuzzNullUUIDJSON`: alvo de fuzzing para o leitor de JSON de
  `NullUUID`, que exige concordância com o tipo `UUID` lido pelo
  `encoding/json` em aceitar, recusar e no valor produzido. Incluído no
  job semanal `deep` ao lado dos outros dois alvos.
- **Linter estático** com `golangci-lint` (versão 2), configurado em
  `.golangci.yml` na raiz: `errcheck`, `govet`, `staticcheck`, `unused`,
  `ineffassign`, `gosec`, `errorlint`, `revive` (comentário em todo
  exportado) e `nolintlint`. `misspell` fica desligado porque só conhece
  inglês. A regra `G115` do `gosec` (truncamento em conversão de inteiro)
  é excluída de propósito: empacotar campos em bytes por deslocamento e
  truncamento é o que a biblioteca faz, e o caminho quente não ganhou
  máscara alguma (benchmarks idênticos antes e depois). Roda no CI na
  versão estável. As duas marcações `//nolint:errcheck` de
  `namebased.go` eram desnecessárias (`errcheck` já ignora `hash.Hash.Write`)
  e foram trocadas por comentário comum; as demais ganharam o linter e o
  motivo que o `nolintlint` exige. Em testes, três comparações diretas
  com `ErrInvalidFormat` passaram a `errors.Is` (a comparação direta,
  que é o contrato de `FromString`, continua testada em
  `TestFromStringErrorUnchanged`), o auxiliar `safeFromString` passou a
  devolver o erro por último e o aquecimento de `benchmark-bulk` deixou
  de fazer uma atribuição inútil. (`.golangci.yml`, `ci.yml`,
  `namebased.go`, `uuid.go` só em comentário, `tests/`)
- **Compilação cruzada no CI** para `windows/amd64`, `darwin/arm64` e
  `linux/arm64` (`go build` e `go vet`) no job `test`, e o job novo
  `test-os`, que roda `go vet`, `go build` e `go test ./... -short` em
  `windows-latest` e `macos-latest` com a versão estável, em pull
  request, tag, no agendamento semanal e sob demanda. (`ci.yml`)
- **Cobertura de testes no CI**: o job `test`, na versão estável, gera o
  perfil com `-coverpkg` (a suíte é outro pacote), imprime
  `go tool cover -func` no log, publica `cover.out` e `cover.html` como
  artefato `cobertura` e falha abaixo de 95%. Medida em 2026-09-11:
  98,2% das instruções do pacote da raiz. (`ci.yml`)
- **Corpus de fuzzing preservado**: no job `deep`, os três alvos rodam
  sempre (`continue-on-error`), o diretório `tests/testdata/fuzz/` é
  publicado como artefato `fuzz-corpus` (30 dias) quando existe, e um
  passo final falha o job se alguma campanha tiver falhado. Antes, a
  entrada que quebrava um alvo morria com o runner. (`ci.yml`)
- **Exemplos executáveis** em `example_test.go`, na raiz, pacote externo
  `loghubuuid_test`: `ExampleGenerateString`, `ExampleGenerator_Generate`,
  `ExampleFromString`, `ExampleImportBinary`, `ExampleParse`,
  `ExampleUUID_TimestampWithLevel`, `ExampleGenerateV5`,
  `ExampleNullUUID` e `ExampleNewCryptoGenerator`. Os que declaram
  saída usam só vetores fixos. Ficam na raiz porque o godoc só associa
  exemplos ao pacote quando estão no mesmo diretório; a exceção à regra
  da raiz mínima está registrada em `CLAUDE.md` e em `STARTHERE.md`.
- Testes novos para os ramos que a medição de cobertura apontou:
  `TestNullUUIDTextAndBinary` (texto e binário de `NullUUID`, com valor,
  ausente e inválido), `TestMustPropagatesError`,
  `TestVariantStringCoversAllCodes`, `TestNewHashMatchesGenerateHash`
  (inclui resumo menor que 16 bytes) em `tests/api_test.go`;
  `TestDomainString` e `TestInspectorsRejectOtherVersions` em
  `tests/versions_test.go`.
- Testes novos: `TestClockSequenceReuseNeverRepeats`,
  `TestSetClockSequenceRandomIsFresh`,
  `TestTimestampWithLevelDiscardsOutOfRangeFields` e casos adicionais em
  `TestSQLScanAndValue`, `TestNullUUID` e
  `TestInvalidLengthIsDistinguishable` (em `tests/`);
  `TestSequenceFloorSurvivesRoundTrip` e
  `TestUnusedSequenceSkipsUsedOnes` (internos, em
  `clock_internal_test.go`, porque simulam o adiantamento diretamente no
  estado do relógio, sem depender da velocidade do host).

### Documentação

- `docs/SPEC.md` alinhada ao código, com a RFC 9562 como critério de
  desempate: layout da versão 6 reescrito (`time_high` 32 bits,
  `time_mid` 16 bits, `time_low` 12 bits, conforme §5.6); assinaturas
  reais de `GetTime` e `NodeID`; nó padrão sorteado com bit multicast
  (§6.10) em vez de leitura de MAC; vetores dourados restritos aos que a
  RFC publica (Apêndice A, espaço DNS); regra normativa do piso de
  relógio por sequência em §4.2.
- `docs/MIGRATION.md`: substituída a afirmação de que os apelidos com
  erro "nunca falham" pela regra real (`NewRandom`, `NewV7`,
  `NewRandomFromReader` e `NewV7FromReader` podem devolver
  `ErrEntropySource`); notas sobre `Scan` com texto vazio,
  `IsInvalidLengthError` e o piso por sequência.
- `docs/DEPLOY-FULL.md`: faixas reais dos campos `Microseconds` (até
  4095 em Nível 1) e `Nanoseconds` (até 1023 em Níveis 1 e 2) da struct
  `Time`; texto vazio em `Scan`; como ressincronizar o relógio das
  versões 1 e 6.
- `docs/TEST-AND-BENCHMARK.md`: seção de integração contínua e nota
  sobre as travas de alocação sob o detector de corrida. `docs/git.md`:
  só etiquetar com o fluxo verde. `README.md`: selo do CI.
  `STARTHERE.md`: árvore atualizada.
- `docs/DEPLOY-FULL.md`: avisos sobre a resolução do relógio do host
  (o campo de nanossegundos do Nível 3 é sempre zero em hosts com relógio
  de microssegundo) e sobre relógio do sistema atrasado, no UUIDv7 e nas
  versões 1 e 6. Comentário de `SetNodeID` explicita que o bit multicast
  de um nó fornecido pelo chamador é responsabilidade dele.
- `CLAUDE.md` atualizado: raiz com `CHANGELOG.md` e `.github/`, comandos
  de CI e de fuzzing, o piso de relógio por sequência, `Scan` com texto
  vazio e a regra sobre as travas de alocação sob o detector de corrida.
- `docs/git.md` virou `docs/RELEASE.md`: guia de release executável do
  início ao fim (pré-requisitos, tag anotada, `gh release`, verificação
  pelo proxy de módulos e a regra de imutabilidade das tags com o motivo),
  sem configuração de máquina, sem `git config --global` e sem os blocos
  repetidos ou vazios do rascunho anterior.
- `SECURITY.md` na raiz: versões suportadas, relato privado pelo recurso
  de aviso de segurança do GitHub, prazo de resposta e o resumo do modelo
  de ameaça já documentado (gerador padrão não serve para segredo, o
  UUIDv7 expõe o instante, as versões 1, 2 e 6 expõem nó e sequência,
  `NewCryptoGenerator` para identificadores inadivinháveis). O recurso
  "Report a vulnerability" precisa ser ligado nas configurações do
  repositório, ação do dono.
- `CONTRIBUTING.md` na raiz: as convenções de `CLAUDE.md` que valem para
  humanos, os comandos de verificação local e a regra de que toda
  mudança de comportamento vem com teste e entrada neste arquivo.
- `docs/TEST-AND-BENCHMARK.md`: tabela de referência das versões 1, 4, 5
  e 6, de `GenerateV1Parallel`, `Parse` e `ImportBinary` (Apple M2, Go
  1.27, mediana de três execuções), com a leitura do custo do mutex em
  paralelo (~148 ns contra ~10,8 ns do UUIDv7) e a confirmação de que o
  piso por sequência não custa nada por geração (`GenerateV1` ~46,8 ns
  contra ~48,3 ns na `v0.3.0`, mesma sessão); seções novas sobre
  exemplos executáveis, linter, cobertura, os três jobs do CI e como
  reproduzir uma falha de fuzzing a partir do artefato.
- `STARTHERE.md`: árvore com `.golangci.yml`, `example_test.go`,
  `SECURITY.md`, `CONTRIBUTING.md` e `docs/RELEASE.md`; caminhos de
  leitura para release e contribuição. `README.md`: links para
  `CONTRIBUTING.md` e `SECURITY.md`. `CLAUDE.md`: exceções da raiz,
  comandos de linter e cobertura, descrição dos três jobs do CI.
- `docs/TEST-AND-BENCHMARK.md`: tabela comparando o gerador padrão antes
  e depois da troca da fonte de entropia, com o comando exato e a leitura
  dos números. `docs/DEPLOY-FULL.md`, `docs/SPEC.md` §7,
  `docs/MIGRATION.md` §5, `STARTHERE.md` §3 e `CLAUDE.md`: as três
  adições de API e a descrição do gerador padrão.

### Decisões

- **Gerador monotônico opcional: recusado.** A proposta era um
  `NewMonotonicGenerator` com contador no topo dos bits aleatórios,
  conforme o método 1 da RFC 9562 §6.2, para dar ordem estrita dentro do
  mesmo instante embutido. O protótipo de 2026-08-27 zerava as 44% de
  inversões em 200.000 pares, mas custava 8,5% em série e **32 vezes**
  em paralelo (7,40 ns para 233,6 ns com 8 núcleos), porque o contador
  exige estado compartilhado e o laço de CAS degrada sob contenção. Uma
  biblioteca cuja razão de ser é gerar milhões de identificadores por
  segundo não deve carregar um caminho com esse perfil, nem a
  complexidade de decidir layout de bits por nível, política de estouro e
  interação com `ImportBinary` — que é cego quanto ao nível e leria o
  contador como microssegundos ou nanossegundos. Quem precisa de ordem
  estrita dentro do processo tem alternativas fora da biblioteca (um
  contador próprio ao lado do UUID, ou a versão 6, que já é estritamente
  crescente por construção). A ordenação continua cronológica na
  resolução do nível, com desempate aleatório, como `CLAUDE.md` e
  `docs/SPEC.md` documentam.
- **Relógio injetável no `Generator`: recusado.** A proposta era permitir
  fixar o instante de fora do pacote, para escrever vetores dourados de
  `Generate` de ponta a ponta. A motivação original — a falta de teste de
  regressão para o relógio pré-1970 — já tinha sido resolvida pelo teste
  interno de `splitUnixInstant` em `clock_internal_test.go`, e o layout
  de bits já está travado por `TestLayoutZeroEntropy` e
  `TestLayoutFullEntropy`. O que sobrava era um campo de função no
  caminho quente, com risco de impedir o inline, em troca de cobertura
  que o conjunto atual de testes já dá por outro caminho. `Generate`
  continua chamando `time.Now()` diretamente.
- **`v1.0.0` adiada.** A biblioteca fica na linha `v0.x` por enquanto. A
  `v0.4.0` mudou o gerador padrão e ampliou a API; um compromisso de
  estabilidade faz sentido depois que essas mudanças tiverem uso real,
  não no mesmo ciclo em que foram feitas. Quando for o caso, a `v1`
  precisa vir com a revisão da superfície pública inteira (inclusive
  `Nil` e `Max` como variáveis mutáveis, e os tipos de retorno `byte` de
  `Version` e `Variant`) e com a política de compatibilidade publicada em
  `README.md`.

---

## [v0.3.0] — 2026-09-10

Cobertura completa da RFC 9562 e API de apoio, sem tocar no caminho
quente do UUIDv7, que continua sem trava e sem alocação. Tudo o que foi
acrescentado vive em arquivos próprios.

### Adicionado

- **Todas as demais versões de UUID.** `GenerateV1` e `GenerateV6`
  (tempo gregoriano em tiques de 100 ns, sequência de relógio de 14 bits
  e nó de 48 bits, com relógio interno estritamente crescente e trava
  própria em `clock.go`); `GenerateV2`, `GenerateV2Person` e
  `GenerateV2Group` (DCE Security); `GenerateV3` (MD5) e `GenerateV5`
  (SHA-1) com os quatro espaços de nomes da RFC e `GenerateHash` para
  esquemas próprios; `GenerateV4` no `Generator` e como função de
  pacote; `GenerateV8` de conteúdo livre e `GenerateV8Random`.
- **Estado do relógio compartilhado**: `GetTime`, `ClockSequence`,
  `SetClockSequence`, `NodeID` e `SetNodeID`. O nó padrão é sorteado uma
  vez com o bit multicast ligado; a biblioteca não lê interfaces de rede
  para não arrastar o pacote `net`.
- **Análise permissiva de texto** em `parse.go`: `Parse` e `ParseBytes`
  aceitam a forma canônica, entre chaves, com prefixo `urn:uuid:` e
  hexadecimal cru de 32 dígitos, sem alocar; `MustParse`, `Must`,
  `Validate`, `FromBytes`. Erros específicos `ErrInvalidLength` e
  `ErrInvalidBrackets` embrulham `ErrInvalidFormat`, então
  `errors.Is(err, ErrInvalidFormat)` continua valendo. `FromString` não
  mudou e continua devolvendo exatamente o sentinela antigo.
- **Serialização** em `encoding.go`: `MarshalText`, `UnmarshalText`,
  `MarshalBinary`, `UnmarshalBinary`.
- **Banco de dados** em `sql.go`: `Scan`, `Value` e o tipo `NullUUID`
  com serializações em JSON, texto e binário.
- **Inspeção** em `inspect.go`: `Timestamp` (versões 1, 6 e 7),
  `TimestampWithLevel` (recupera a precisão sub-milissegundo dos níveis
  2 e 3), `GregorianTime`, `ClockSequence`, `NodeID`, `Domain` e `ID`,
  todos com segundo retorno booleano quando a versão não carrega o
  campo. Para a versão 2, `ClockSequence` devolve só os 6 bits altos,
  porque o byte baixo é o domínio.
- **Valores e utilitários** em `values.go`: `Nil`, `Max`, `IsZero`,
  `IsMax`, `Compare`, `URN`, o tipo `UUIDs` com `Strings`,
  `VersionString` e `VariantString`.
- **Entropia** em `entropy.go`: `NewGeneratorWithReader` e
  `NewCryptoGenerator`, com pânico se a fonte falhar durante a geração
  (uma fonte quebrada não pode degradar em silêncio); `ErrEntropySource`.
- **Compatibilidade com `github.com/google/uuid`** em `compat.go`: `New`,
  `NewString`, `NewRandom`, `NewRandomFromReader`, `NewUUID`, `NewV6`,
  `NewV7`, `NewV7FromReader`, `NewMD5`, `NewSHA1`, `NewHash`,
  `NewDCESecurity`, `NewDCEPerson`, `NewDCEGroup`. Os apelidos que
  produzem bits aleatórios leem de `crypto/rand`, como o pacote de
  origem. Guia em `docs/MIGRATION.md`.
- Testes: `tests/versions_test.go` (versões 1 a 8, vetores da RFC 9562
  para as versões 3 e 5, deriva do relógio, não repetição ao reaplicar o
  nó) e `tests/api_test.go` (análise, serialização, SQL, apelidos);
  `FuzzParse`; `clock_internal_test.go` na raiz para as bordas do
  relógio que não são alcançáveis de fora do pacote.

### Alterado

- **Formato de dados em JSON e gob.** Com `MarshalText` presente, o
  `encoding/json` passou a gravar um `UUID` como a string canônica entre
  aspas. Antes ele gravava uma lista de 16 números, por ser um vetor de
  bytes. O mesmo vale para `encoding/gob`, que passa a usar
  `MarshalBinary`. A API é aditiva; os dados já gravados no formato
  antigo precisam ser convertidos. Para quem vem do pacote do Google,
  nada muda.
- **Deriva de relógio nas versões 1, 2 e 6.** Em rajadas mais rápidas
  que o tique de 100 ns, o relógio interno avança um tique por geração e
  o instante embutido fica à frente do relógio do sistema. A RFC 9562
  permite; difere do pacote do Google, que mantém o instante e
  incrementa a sequência.

### Documentação

- `PROMPT.md` foi reescrito como especificação completa de reconstrução
  e em seguida convertido em `docs/SPEC.md`, deixando a raiz só com os
  arquivos de produção. `docs/MIGRATION.md` criado. `STARTHERE.md`,
  `README.md` e `docs/DEPLOY-FULL.md` ampliados para a nova API.

### Decisões

- Não foram trazidos do pacote do Google: `SetRand` (trocaria a entropia
  do gerador padrão em tempo de execução, exigindo uma leitura atômica
  no caminho quente; use `NewGeneratorWithReader` ou
  `NewCryptoGenerator`), `EnableRandPool` e `DisableRandPool` (o gerador
  padrão já mantém um pool por thread), `SetNodeInterface` e
  `NodeInterface` (arrastariam o pacote `net`).
- `String()` e `encodeHex` duplicam as mesmas oito linhas de propósito:
  `String()` é caminho quente e não deve pagar uma chamada por causa dos
  serializadores.

---

## [v0.2.0] — 2026-08-27

Passagem de correção a partir de três relatórios de revisão sobre a
`v0.1.0`. Nenhuma correção alterou o layout de bits, a semântica dos
níveis ou o formato de saída.

### Corrigido

- **Pânico de índice fora de faixa em `FromString`** (crítico). O laço
  pulava qualquer hífen encontrado, então um hífen extra em
  deslocamento par do último grupo (24, 26, 28, 30, 32 ou 34) fazia ler
  `s[36]` e derrubava o processo com entrada externa de 36 caracteres.
  A decodificação passou a usar a tabela fixa `hexOffsets`, que
  elimina a leitura fora dos limites por construção e rejeita hífens
  fora do lugar. O desempenho melhorou 22%, com zero alocações
  preservadas. Travado por `TestFromStringNeverPanics` (todas as
  36 × 256 mutações de um byte), `TestFromStringExtraHyphen` e
  `FuzzFromString`.
- **Relógio anterior a 1970 corrompia o UUID em silêncio.** O operador
  `%` preserva o sinal do dividendo, então os campos sub-milissegundo
  ficavam negativos e sofriam wrap-around ao virar `uint16`. O
  timestamp passou a ter piso na própria época Unix, com os campos
  sub-milissegundo zerados.
- **Limite real de data era 2262, não 10889.** O comentário prometia
  10889 (correto para o campo de 48 bits), mas a leitura por
  `UnixNano()` satura em 2262-04-11. A leitura passou a ser
  `Unix()` mais `Nanosecond()`, que não satura, e a decomposição foi
  isolada em `splitUnixInstant`.
- **`Generator` zerado e `NewGeneratorWith(nil)` causavam pânico.**
  `NewGeneratorWith(nil)` passou a entrar em pânico na construção, com
  mensagem clara; `Generate` recorre à entropia do gerador padrão quando
  o receptor não tem fonte (valor zero embutido em outra struct, ou
  ponteiro nulo).
- **Corrida de dados na suíte de testes.** `TestMassConcurrent` e o
  benchmark paralelo escreviam em um sink global a partir de várias
  goroutines. Cada goroutine passou a publicar em posição própria, e o
  benchmark usa `runtime.KeepAlive`. A biblioteca em si estava limpa.
- **`TestMonotonicity` falhava permanentemente** em hosts com relógio
  de microssegundo, como o macOS: contava regressões em laço apertado,
  medindo o relógio do host e não a biblioteca. Reescrito com pausa
  entre gerações; a invariante independente do relógio está em
  `TestOrderingFollowsEmbeddedTime` e `TestTieRateReport` mede a taxa
  de empates como diagnóstico.
- **Benchmarks sem aquecimento** inflavam o primeiro cenário, sempre o
  Nível 1, em cerca de 60%; e `TestMassOneMillion` calculava a taxa com
  `dur.Milliseconds()+1`. Passagem de aquecimento acrescentada e taxa
  em nanossegundos; tabelas remedidas.
- **`%x` em `uuid.UUID` imprime o hexadecimal da string**, não os 16
  bytes, porque o tipo satisfaz `fmt.Stringer`. Mensagens de teste
  corrigidas e armadilha documentada em `String()`.
- Links quebrados e árvore desatualizada no `STARTHERE.md`; trechos não
  compiláveis no `README.md` e no `docs/DEPLOY-FULL.md`; contradição
  nas faixas documentadas dos campos do tipo `Time`.
- `.DS_Store` removido do índice do Git e `.gitignore` criado.

### Alterado

- **Uma palavra de entropia nos níveis 2 e 3.** `Generate` sorteava duas
  palavras de 64 bits sempre, mas `rand_a` só é aleatório no Nível 1.
  O `Generator` passou a ter as fontes `oneWord` e `twoWords`, e os
  níveis 2 e 3 consomem uma única palavra. Ganho medido de 7% serial e
  18% em paralelo; importa também para quem fornece `crypto/rand` por
  `NewGeneratorWith`. Travado por `TestEntropyDrawsPerLevel`.

### Adicionado

- Suíte de testes em `tests/`: `parsing_test.go`, `fuzz_test.go`,
  `layout_test.go` (layout de bits com entropia determinística),
  `import_test.go`, `ordering_test.go` (ordenação, unicidade em
  1.000.000 por nível, concorrência sob `-race`), `alloc_test.go`
  (trava de zero alocações) e `robustness_test.go` (bordas do
  `Generator`, consumo de entropia, níveis desconhecidos, atalhos de
  pacote); `BenchmarkFromString` e `BenchmarkImportBinary`.
- **Aviso de segurança.** O gerador padrão usa PCG, um PRNG estatístico
  e não criptográfico: quem observar alguns UUIDs consegue prever os
  seguintes, e todo UUIDv7 expõe o instante de criação. Advertência no
  comentário de `NewGenerator`, no `README.md`, no `docs/DEPLOY-FULL.md`
  e no `STARTHERE.md`, com o exemplo de gerador com `crypto/rand`.

### Documentação

- `docs/SPEC.md`: a promessa de ordenação passou a ser precisa
  (cronológica na resolução do nível, com desempate aleatório dentro do
  mesmo instante, e não monotonicidade estrita); casos de teste
  obrigatórios reescritos; requisitos de robustez do relógio, do
  analisador e da fonte de entropia. `docs/DEPLOY-FAST.md` com exemplos
  dos três níveis. `docs/git.md` com notas de release.

### Decisões

- **Não subir o `go.mod` para 1.23.** `math/rand/v2` existe desde o
  1.22, o código não usa API posterior e o campo declara a versão
  mínima suportada; subir o piso reduziria a base de usuários sem
  ganho.
- **Não usar `unsafe.String` em `String()`.** O buffer de 36 bytes vive
  na pilha; devolver uma string que aponta para ele é comportamento
  indefinido, e `String()` é API pública.
- **Não adotar contador monotônico por padrão.** O protótipo zera as
  regressões de ordenação, mas custa 32 vezes mais em paralelo e
  destrói a escalabilidade que é o principal atrativo. Fica registrado
  como proposta opcional (ver abaixo).

---

## [v0.1.0] — 2026-06-05

Primeira versão publicada.

### Adicionado

- Geração de **UUIDv7** (RFC 9562) com três níveis de precisão temporal:
  `Level1` (milissegundos, UUIDv7 padrão), `Level2` (microssegundos em
  `rand_a`) e `Level3` (microssegundos em `rand_a` e nanossegundos nos
  10 bits altos de `rand_b`). Versão e variante preservadas em todos os
  níveis; ordenação lexicográfica cronológica na resolução do nível.
- Tipos `Level`, `UUID` (`[16]byte`), `Generator` e `Time`.
- `NewGenerator` com pool de PRNGs PCG por thread (`sync.Pool`), cada um
  semeado uma vez de `crypto/rand`, sem trava compartilhada;
  `NewGeneratorWith` para entropia personalizada; atalhos de pacote
  `Generate` e `GenerateString`.
- Conversão `String` e `FromString`, com os apelidos `BinaryToString` e
  `StringToBinary`; importação das propriedades de tempo por `Import` e
  `ImportBinary`, cega quanto ao nível.
- Geração binária sem alocações e `String` com uma única alocação.
- Documentação: `README.md`, `STARTHERE.md`, `docs/DEPLOY-FAST.md`,
  `docs/DEPLOY-FULL.md`, `docs/SPEC.md`, `docs/TEST-AND-BENCHMARK.md`;
  testes funcionais, benchmarks e o executável
  `tests/benchmark-bulk`.

---

## Propostas em aberto

Nenhuma. As quatro propostas levantadas nas revisões de 2026-08-27 foram
decididas em 2026-09-11 e publicadas na `v0.4.0`: a troca da fonte de
entropia e as três adições de API foram feitas; o gerador monotônico e o
relógio injetável foram recusados.

**O registro canônico de decisões é a seção 11 do
[docs/SPEC.md](docs/SPEC.md)**, que lista cada uma com o motivo e o que
justificaria revê-la. Este arquivo guarda o histórico — quando cada
decisão foi tomada e o que mudou junto —, mas quem for propor ou auditar
deve ler a especificação primeiro. Decisão nova entra nos dois lugares.

[Não publicado]: https://github.com/patrickbrandao/go-loghub-uuid/compare/v0.4.0...HEAD
[v0.4.0]: https://github.com/patrickbrandao/go-loghub-uuid/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/patrickbrandao/go-loghub-uuid/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/patrickbrandao/go-loghub-uuid/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/patrickbrandao/go-loghub-uuid/releases/tag/v0.1.0
