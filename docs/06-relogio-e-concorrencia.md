# Relógio Compartilhado e Arquitetura de Concorrência

> Parte da especificação de desenvolvimento agnóstica de linguagem.
> Reproduz a seção 4.2 ("Estado Monotônico Compartilhado") e a seção 5
> ("Arquitetura de Entropia e Política de Falha Rápida") de
> `docs/SPEC.md` (documento original, já removido — ver
> [INDEX.md](INDEX.md)). As duas seções tratam do mesmo tema visto de
> dois ângulos: como esta biblioteca mantém estado compartilhado entre
> threads em segurança — o relógio interno de v1/v2/v6 de um lado, a
> fonte de entropia do gerador padrão de outro — sem pagar lock global
> no caminho quente do UUIDv7.

## 4.2 Estado Monotônico Compartilhado (v1, v2 e v6)

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
     o relógio, como esta (item 2 acima e o adiantamento descrito
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

Este estado — relógio, sequência e nó — é totalmente independente do
caminho de geração do UUIDv7 (ver
[04-uuidv7-formato-e-niveis.md](04-uuidv7-formato-e-niveis.md)), que não
tem lock algum. Na implementação de referência ele vive em `clock.go`,
protegido por um único mutex (`clockMu`).

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
    converter esse pânico no erro de fonte de entropia (seção 6.4, em
    [07-parsing-e-conversao.md](07-parsing-e-conversao.md)); a geração
    propriamente dita nunca devolve erro.

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
vale para a geração pelo relógio, para a geração por instante (seção 3.6
em [04-uuidv7-formato-e-niveis.md](04-uuidv7-formato-e-niveis.md)) e para
as versões 4 e 8.

A diferença entre os casos 2 e 3 é o momento e a intenção: quem pede uma
fonte inválida recebe o erro de imediato; quem simplesmente não construiu
o gerador recebe um resultado correto. Ver
[10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)
seção 11.3.

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

Ver [03-guia-completo.md](03-guia-completo.md) para exemplos práticos de
`NewGenerator`, `NewGeneratorWith`, `NewCryptoGenerator` e
`NewGeneratorWithReader`, e
[12-migracao-google-uuid.md](12-migracao-google-uuid.md) para como esta
arquitetura difere da do pacote `github.com/google/uuid`.
