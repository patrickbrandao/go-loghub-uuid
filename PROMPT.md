# Prompt de construção — Biblioteca UUIDv7 multinível (go-loghub-uuid)

Você é um engenheiro de software especializado em Go (GoLang). Construa,
do zero, a biblioteca descrita neste documento.

Este texto substitui o prompt original do projeto. Ele foi reescrito
depois de várias rodadas de revisão de código, correção de defeitos e
medição de desempenho sobre a primeira implementação. **Cada requisito de
robustez, segurança e desempenho listado aqui corresponde a um defeito
real que já foi encontrado, reproduzido e corrigido** — nenhum deles é
opcional, e a seção 14 registra explicitamente as sugestões que foram
avaliadas e **descartadas**, para que não sejam reintroduzidas.

Não há código-fonte neste documento, por projeto: ele especifica
comportamento, nomes e contratos. A implementação é sua.

---

## 1. Identidade do projeto

| Item | Valor |
|------|-------|
| Repositório | `go-loghub-uuid` |
| Caminho do módulo | `github.com/patrickbrandao/go-loghub-uuid` |
| Nome do pacote Go | `loghubuuid` |
| Alias de importação usado na documentação | `uuid` |
| Licença | MIT |
| Versão mínima declarada em `go.mod` | `go 1.22` |
| Dependências externas | **nenhuma** |

Somente a biblioteca padrão pode ser usada. No código de produção, os
únicos pacotes importados devem ser: `crypto/rand` (semeadura),
`encoding/binary`, `math/rand/v2`, `sync`, `time` e `errors`. Qualquer
importação além dessas precisa de justificativa explícita.

---

## 2. Convenções de escrita (obrigatórias)

- **Identificadores** (tipos, funções, métodos, variáveis, constantes,
  nomes de arquivo e de pacote) em **inglês**.
- **Comentários, documentação, mensagens de erro e rótulos de relatório**
  em **português do Brasil**.
- **Sem emojis** em qualquer arquivo. Texto formal.
- Todo símbolo exportado tem comentário de documentação no formato do
  `go doc`, começando pelo próprio nome do símbolo.
- Mensagens de erro e de pânico prefixadas com `loghubuuid: `.
- Código formatado por `gofmt`; `go vet ./...` sem apontamentos.
- Comentários explicam **o porquê** (especialmente aritmética de bits e
  decisões de robustez), não a sintaxe.

---

## 3. O que a biblioteca faz

Três capacidades, e nada além disso:

1. **Gerar** UUIDv7 (RFC 9562) em um de três níveis de precisão temporal,
   devolvendo o valor binário de 128 bits ou a string canônica.
2. **Converter** entre o binário de 128 bits e a string canônica, nos
   dois sentidos.
3. **Importar** de um UUIDv7 as propriedades de tempo embutidas,
   separadas em segundos, milissegundos, microssegundos e nanossegundos.

---

## 4. Conceito: UUIDv7 e os três níveis

Um UUIDv7 carrega, nos 48 bits mais significativos, o instante de criação
em milissegundos desde a época Unix. Os bits restantes são divididos em
`rand_a` (12 bits) e `rand_b` (62 bits), normalmente aleatórios, mais 4
bits de versão e 2 bits de variante.

Esta biblioteca opcionalmente **sobrescreve parte desses bits aleatórios
com precisão sub-milissegundo**, sempre preservando versão (7) e variante
(RFC, `0b10`). O resultado é sempre um UUIDv7 válido para qualquer
validador.

| Nível | `rand_a` (12 bits) | topo de `rand_b` (10 bits) | resto de `rand_b` | Bits aleatórios efetivos |
|-------|--------------------|----------------------------|-------------------|-------------------------:|
| Nível 1 | aleatório | aleatório | aleatório (52 bits) | 74 |
| Nível 2 | microssegundos 0..999 | aleatório | aleatório (52 bits) | 62 |
| Nível 3 | microssegundos 0..999 | nanossegundos 0..999 | aleatório (52 bits) | 52 |

- **Nível 1** é o UUIDv7 padrão da RFC, sem nenhuma extensão.
- Como os bits de precisão ficam **imediatamente depois** dos
  milissegundos, a ordenação lexicográfica da string continua
  cronológica — com a ressalva da seção 15.
- Um valor de nível desconhecido (por exemplo, `Level(99)`) **deve se
  comportar exatamente como o Nível 1**, sem erro e sem pânico.

---

## 5. Layout de bits exato (128 bits, big-endian / ordem de rede)

```
byte:  0    1    2    3    4    5    6    7    8    9   10   11   12   13   14   15
      [-------- unix_ts_ms (48) --------][V|aa][ aa ][v|bb][----------- rand_b -----------]
                                          7  ^             10 ^
                                        versão            variante
```

- **bytes 0..5** — `unix_ts_ms`: 48 bits de milissegundos desde
  1970-01-01 UTC.
- **byte 6** — nibble alto fixo em `0x7` (versão); nibble baixo recebe os
  4 bits mais altos de `rand_a`.
- **byte 7** — os 8 bits baixos de `rand_a` (12 bits no total).
- **byte 8** — os 2 bits mais altos fixos em `10` (variante RFC); os 6
  bits baixos recebem o topo de `rand_b`.
- **bytes 9..15** — os 56 bits restantes de `rand_b` (62 bits no total).

Posição do tempo sub-milissegundo:

- **microssegundos** ocupam `rand_a` inteiro (0..999 cabe folgado em 12
  bits) nos Níveis 2 e 3;
- **nanossegundos** ocupam os bits 61..52 de `rand_b` (os 10 bits mais
  altos do campo) no Nível 3;
- os 52 bits inferiores de `rand_b` são sempre aleatórios.

Este layout é normativo. Se ele mudar, precisam mudar junto: o comentário
de documentação da função de geração, o mapa do projeto (`STARTHERE.md`)
e a especificação (`docs/SPEC.md`).

---

## 6. Contrato de API pública — nomes exatos

Os nomes abaixo são normativos; uma reconstrução deve expor exatamente
estes símbolos, com estes significados.

### 6.1 Tipos

- **`Level`** — inteiro sem sinal de 8 bits que seleciona a precisão.
- **`UUID`** — arranjo fixo de 16 bytes (não um slice), big-endian.
- **`Generator`** — objeto gerador, criado uma vez no boot e
  compartilhado. Campos internos **não exportados**: um que devolve uma
  palavra de 64 bits aleatórios (`oneWord`) e outro que devolve duas
  (`twoWords`).
- **`Time`** — estrutura com as propriedades temporais importadas, com os
  campos exportados `Seconds` (inteiro de 64 bits, segundos desde a
  época), `Milliseconds`, `Microseconds` e `Nanoseconds` (inteiros).

### 6.2 Constantes

- **`Level1`** = 1, **`Level2`** = 2, **`Level3`** = 3.

### 6.3 Construtores

- **`NewGenerator`** — sem parâmetros, devolve ponteiro para
  `Generator`. Entropia rápida e sem contenção (seção 11).
- **`NewGeneratorWith`** — recebe uma função sem parâmetros que devolve
  64 bits aleatórios, devolve ponteiro para `Generator`. A função
  fornecida **precisa ser segura para uso concorrente**; isso deve estar
  escrito no comentário de documentação. Ver a regra de rejeição de fonte
  nula em 13.5.

### 6.4 Geração

- Método **`Generate`** sobre `*Generator` — recebe `Level`, devolve
  `UUID`.
- Método **`GenerateString`** sobre `*Generator` — recebe `Level`,
  devolve a string canônica.
- Função de pacote **`Generate`** — mesmo contrato, usando o gerador
  padrão interno.
- Função de pacote **`GenerateString`** — idem.

### 6.5 Conversão

- Método **`String`** sobre `UUID` — devolve a forma canônica
  `8-4-4-4-12`, 32 dígitos hexadecimais **minúsculos**, separados por
  hifens.
- Função **`FromString`** — recebe string, devolve `UUID` e erro.
- Função **`BinaryToString`** — apelido explícito do método `String`.
- Função **`StringToBinary`** — apelido explícito de `FromString`.

Os dois apelidos existem por legibilidade em código de chamada e devem
delegar, sem duplicar lógica.

### 6.6 Importação

- Função **`Import`** — recebe a string canônica, devolve `Time` e erro.
- Função **`ImportBinary`** — recebe `UUID`, devolve `Time` (sem erro).

### 6.7 Inspeção

- Método **`Version`** sobre `UUID` — devolve o nibble de versão
  (esperado: 7).
- Método **`Variant`** sobre `UUID` — devolve os 2 bits altos do byte 8
  (esperado: 2, isto é, `0b10`).

### 6.8 Erros

- **`ErrInvalidFormat`** — variável de erro exportada, devolvida sempre
  que a string não estiver no formato canônico (tamanho, hifens ou
  dígitos inválidos). Texto em português.

### 6.9 Símbolos internos esperados

Não exportados, mas com nomes fixados para orientar a reconstrução:
**`strongSeed`** (semeadura a partir do gerador criptográfico do
sistema), **`fromHex`** (conversão de um caractere hexadecimal),
**`hexDigits`** (tabela de dígitos minúsculos), **`hexOffsets`** (tabela
de deslocamentos fixos do analisador — ver 13.1) e
**`defaultGenerator`** (gerador padrão do pacote, criado na carga).

---

## 7. Algoritmo de geração (passo a passo)

1. Se o receptor não tiver fonte de entropia (ponteiro nulo ou valor zero
   do tipo), **usar a entropia do gerador padrão do pacote** — nunca
   entrar em pânico (ver 13.4).
2. Ler o relógio **uma única vez**, em duas partes: segundos desde a
   época e fração do segundo em nanossegundos (0..999.999.999). **Não**
   ler o instante como um único inteiro de nanossegundos (ver 13.3).
3. Compor os milissegundos: `segundos × 1000 + fração / 1.000.000`.
4. Se os milissegundos resultarem **negativos** (relógio anterior a
   1970), fixar o piso na própria época: milissegundos e fração viram
   zero (ver 13.2).
5. Da fração do segundo, extrair o resto sub-milissegundo (0..999.999),
   dele os **microssegundos** (0..999) e os **nanossegundos** (0..999).
6. Sortear entropia **conforme o nível**: uma palavra de 64 bits nos
   Níveis 2 e 3; duas palavras no Nível 1 e nos níveis desconhecidos
   (ver 10.3). Este ponto é normativo e é travado por teste.
7. Escrever os 48 bits de milissegundos nos bytes 0..5.
8. Montar `rand_a`: microssegundos nos Níveis 2 e 3; 12 bits aleatórios
   caso contrário. Gravar com a versão 7 no nibble alto do byte 6.
9. Montar `rand_b`: no Nível 3, os nanossegundos nos 10 bits altos e 52
   bits aleatórios abaixo; nos demais níveis, 62 bits aleatórios. Gravar
   com a variante `10` nos 2 bits altos do byte 8.
10. Devolver os 16 bytes. **Nenhuma alocação de heap** neste caminho.

A geração de string é a geração binária seguida da serialização descrita
em 9.1 — sem caminho alternativo e sem duplicação de lógica.

---

## 8. Algoritmo de importação

`ImportBinary` é deliberadamente **cego quanto ao nível**: ele não tem
como saber qual nível produziu o UUID, então sempre lê `rand_a` como
microssegundos e os 10 bits altos de `rand_b` como nanossegundos,
tratando bits aleatórios como se fossem tempo preciso. Esse é o
comportamento pedido, não um defeito.

Passos:

1. Recompor os 48 bits de milissegundos a partir dos bytes 0..5.
2. `Seconds` = milissegundos inteiros divididos por 1000;
   `Milliseconds` = resto dessa divisão (sempre 0..999).
3. `Microseconds` = os 12 bits de `rand_a` (4 bits baixos do byte 6 mais
   o byte 7).
4. `Nanoseconds` = os 10 bits altos de `rand_b` (6 bits baixos do byte 8
   mais os 4 bits altos do byte 9).

`Import` faz o parsing da string e delega a `ImportBinary`, propagando
`ErrInvalidFormat` e devolvendo a estrutura zerada em caso de erro.

**Documentação dos campos (obrigatório).** Os comentários de campo de
`Microseconds` e `Nanoseconds` — que são o que aparece no `go doc` e no
autocompletar — precisam declarar **as duas faixas**: 0..999 quando o
UUID é de Nível 2/3 (micro) ou Nível 3 (nano); 0..4095 e 0..1023,
respectivamente, quando os bits são aleatórios. Anunciar apenas 0..999
é contradição com o comportamento e já foi corrigido uma vez.

---

## 9. Conversões

### 9.1 Binário para string

- Saída canônica `8-4-4-4-12`, 36 caracteres, hexadecimal **minúsculo**.
- Preencher diretamente um buffer fixo de 36 bytes, escrevendo os hifens
  antes dos bytes de índice 4, 6, 8 e 10. Nenhum buffer intermediário,
  nenhuma formatação por `fmt`, no máximo **uma** alocação (a string
  devolvida).
- **Documentar a armadilha do `%x`**: como `UUID` satisfaz a interface de
  formatação por string, os verbos `%x` e `%X` do pacote de formatação
  imprimem o hexadecimal **do texto** (72 caracteres), e não os 16 bytes.
  O comentário de `String` deve avisar e indicar o caminho correto
  (formatar a fatia dos bytes). A própria suíte de testes precisa seguir
  essa regra em suas mensagens de falha.

### 9.2 String para binário

- Rejeitar de imediato entradas cujo tamanho seja diferente de 36.
- Validar que existem hifens exatamente nas posições 8, 13, 18 e 23.
- **Decodificar a partir de uma tabela de deslocamentos fixos** dos 16
  bytes (0, 2, 4, 6, 9, 11, 14, 16, 19, 21, 24, 26, 28, 30, 32, 34) — e
  **jamais** percorrendo a string e pulando qualquer hifen encontrado.
  Ver 13.1: essa é a origem do único defeito crítico já registrado no
  projeto.
- Aceitar maiúsculas e minúsculas.
- **Em caso de erro, devolver sempre o UUID zerado** — nenhum byte
  parcialmente decodificado pode vazar para o chamador.
- Zero alocações de heap.

---

## 10. Requisitos de desempenho

### 10.1 Metas

- Geração binária na ordem de **dezenas de nanossegundos** por UUID, com
  **zero alocações**.
- Geração de string com **no máximo uma** alocação (a string devolvida,
  48 bytes).
- `FromString` e `ImportBinary` com **zero alocações**.
- Vazão de **milhares de UUIDs por milissegundo por núcleo**; em CPU
  moderna, mais de 20 mil por milissegundo por núcleo, e mais de 100 mil
  agregados em máquina de 8 núcleos.

Referências medidas na implementação corrigida (Apple M2, Go 1.27,
`GOMAXPROCS=8`), para calibrar uma reconstrução: geração binária 41 a 43
ns/op; geração de string 64 a 68 ns/op com 1 alocação de 48 bytes;
`FromString` 31,6 ns/op; Nível 3 em paralelo 9,7 ns/op. Em VM modesta
(Xeon 2,80 GHz, núcleo único): 88 a 90 ns/op no binário e 163 a 165 ns/op
na string.

### 10.2 Onde está o teto

Cerca de **dois terços a três quartos** do custo de gerar um UUID é a
leitura do relógio (medido: 32 ns de um total de 44 ns). Esse custo é
irredutível — a precisão sub-milissegundo é a razão de ser da
biblioteca. Micro-otimizar a montagem dos 16 bytes é ruído estatístico.
A documentação de desempenho deve registrar isso, para não induzir
otimizações inúteis.

### 10.3 Sorteios de entropia por nível (normativo)

Os Níveis 2 e 3 colocam os microssegundos em `rand_a` e por isso
consomem **uma** palavra de 64 bits; apenas o Nível 1 (e os níveis
desconhecidos, que se comportam como ele) consome **duas**. Sortear duas
sempre desperdiça um sorteio por UUID — e, em um gerador construído com
fonte personalizada, chama a fonte do usuário (possivelmente
criptográfica) duas vezes quando uma basta. O ganho medido ao corrigir
isso foi de 7,5% no Nível 2, 7,0% no Nível 3 e **17,8% no caminho
paralelo**. Isso deve ser travado por teste (seção 16).

**Consequência esperada e documentada:** o Nível 1 é, de fato, o nível
mais lento — por um sorteio extra, não por viés de medição.

### 10.4 Metodologia de medição (normativa)

- Todo medidor que executa cenários em sequência **deve fazer uma
  passagem de aquecimento descartada** antes da primeira medição
  (sugestão: um décimo da amostra, limitado a 100.000 gerações). Sem
  isso, o primeiro cenário paga sozinho o aquecimento de cache, o
  escalonamento de frequência e o preenchimento do pool, e aparece cerca
  de 60% mais lento do que é.
- A taxa deve ser calculada a partir de **nanossegundos**. Calcular como
  amostras divididas por milissegundos inteiros somados de 1 introduz
  erro da ordem de 2% em execuções curtas.
- As tabelas publicadas na documentação devem **identificar o host** de
  cada medição.

---

## 11. Requisitos de concorrência

- O `Generator` é criado **uma única vez no boot** e compartilhado por
  centenas de goroutines simultâneas. Nenhuma trava global no caminho
  quente.
- Entropia padrão: um **conjunto de geradores pseudoaleatórios rápidos
  por thread** — um pool de PCG (`math/rand/v2`), cada instância semeada
  uma única vez a partir do gerador criptográfico do sistema e depois
  avançando localmente.
- A semeadura lê 8 bytes do gerador criptográfico do sistema; em caso de
  falha (extremamente rara), degrada para o relógio em vez de entrar em
  pânico.
- `NewGeneratorWith` permite substituir a fonte; o comentário de
  documentação deve deixar explícito que a função fornecida **precisa ser
  segura para uso concorrente** e informar quantas vezes ela é chamada
  por UUID em cada nível (uma nos Níveis 2 e 3, duas no Nível 1).
- As funções de pacote usam um `defaultGenerator` interno criado na carga
  do pacote.

**Alternativa conhecida, deliberadamente não adotada:** as funções de
pacote de `math/rand/v2` são, desde o Go 1.22, por thread e sem trava, e
medem mais rápido que o par de operações do pool (4,49 ns contra 7,85 ns
por palavra em fluxo serial; 0,91 ns contra 2,75 ns em paralelo), além de
usarem ChaCha8 semeado pelo sistema operacional, o que mitigaria a
questão de segurança da seção 12. O pool também tem um custo escondido: o
coletor de lixo o esvazia periodicamente, e cada recriação faz duas
leituras do gerador criptográfico. **Trocar o desenho é uma decisão de
arquitetura, não uma correção** — se for feita, faça-a de forma
consciente, com benchmark próprio, e atualize toda a documentação que
descreve o pool.

---

## 12. Requisitos de segurança

### 12.1 O gerador padrão é previsível — e isso precisa estar escrito

PCG é um gerador pseudoaleatório **estatístico, não criptográfico**. Sua
saída de 64 bits expõe o estado interno: a partir de poucas amostras
observadas, um adversário reconstrói o estado e prevê os UUIDs seguintes
daquele gerador — e, no Nível 1, 62 bits de `rand_b` vêm diretamente de
uma única saída do PCG. Além disso, **todo UUIDv7 expõe o instante de
criação por construção**.

Essa é uma escolha de projeto legítima (velocidade), mas a advertência é
obrigatória e precisa aparecer em **quatro lugares**:

1. no comentário de documentação de `NewGenerator` (para aparecer no
   `go doc`);
2. em uma seção "Aviso de segurança" do `README.md`, com o exemplo pronto
   de gerador com entropia criptográfica;
3. no guia de uso completo, deixando claro que, para identificadores que
   precisem ser inadivinháveis, o construtor com entropia criptográfica é
   **requisito**, não conveniência;
4. com um ponteiro para o aviso no mapa do projeto.

Redação recomendada: **não use estes identificadores como segredo** —
token de sessão, link privado, chave de recuperação ou senha de uso
único.

### 12.2 Entropia efetiva e risco de colisão

Bits aleatórios por nível: 74 (Nível 1), 62 (Nível 2), 52 (Nível 3). No
Nível 3, dentro de um mesmo nanossegundo, restam 52 bits; pelo paradoxo
do aniversário, a probabilidade de colisão chega a 50% na ordem de 2^26
(cerca de 67 milhões) de UUIDs gerados **no mesmo nanossegundo** —
inatingível na prática. A unicidade é confortável; o que não é garantido
é a **imprevisibilidade**. A documentação deve trazer a tabela e essa
leitura prática.

### 12.3 Superfície de entrada externa

`FromString`, `StringToBinary` e `Import` são as funções que recebem dado
externo (UUID vindo de requisição HTTP, de linha de log, de coluna de
banco). Todo requisito de robustez do analisador (13.1) é, na prática,
requisito de segurança: um pânico ali é negação de serviço remota trivial
de acionar, inclusive por acidente.

---

## 13. Robustez obrigatória — catálogo de defeitos já corrigidos

Cada item abaixo já ocorreu na primeira implementação. Trate-os como
requisitos de primeira classe.

### 13.1 O analisador jamais pode ler fora dos limites

**Defeito original (severidade crítica).** O analisador validava o
tamanho e os quatro hifens canônicos, mas depois percorria a string
pulando **qualquer** hifen encontrado, um a um. Um hifen extra em
deslocamento par do último grupo desalinhava os pares: o índice chegava a
35, o laço entrava e lia a posição 36 de uma string de 36 caracteres —
pânico de índice fora de faixa, derrubando o processo. Exatamente **seis
mutações de um único byte** sobre uma string canônica disparavam o
problema (trocar por hifen os deslocamentos 24, 26, 28, 30, 32 e 34).

**Requisito.** Decodificar a partir de deslocamentos fixos e conhecidos,
o que elimina a classe inteira de erro por construção (o maior índice
lido passa a ser 35) e rejeita hifens fora do lugar naturalmente, já que
hifen não é dígito hexadecimal. A correção também é **mais rápida**:
40,49 ns/op para 31,60 ns/op, mantendo zero alocações.

### 13.2 Relógio anterior à época Unix não pode corromper o UUID

Em Go, o operador de resto preserva o sinal do dividendo. Com o relógio
ajustado para antes de 1970 (máquina sem RTC, contêiner mal inicializado,
NTP em falha), os campos sub-milissegundo ficavam negativos e, ao serem
convertidos para inteiro sem sinal de 16 bits, sofriam wrap-around:
medidos 3553 microssegundos e 813 nanossegundos, ambos fora da faixa
0..999. O UUID resultante ainda tinha versão 7 e variante RFC — passava
em qualquer validador — mas com timestamp sem sentido e deixando de ser
ordenável em relação a todos os anteriores.

**Requisito.** Fixar o piso do timestamp na própria época antes de
decompor, e documentar o comportamento.

### 13.3 A leitura do relógio não pode saturar em 2262

Ler o instante como um único inteiro de 64 bits em nanossegundos satura
em **2262-04-11**, quando o valor vira negativo e recai em 13.2 — embora
o campo de 48 bits comporte datas até 10889-08-02. Prometer 10889 na
documentação enquanto a fonte falha em 2262 é incorreto.

**Requisito.** Ler segundos e fração do segundo em separado. A fração
nunca é negativa e a composição não satura. Verificado: para
2300-01-01, a leitura antiga produzia lixo negativo e a nova produz o
valor correto, que cabe nos 48 bits. Verificado também que a mudança
**não altera o comportamento no domínio normal** — zero divergências em
3.000.000 de instantes pós-1970, campo a campo.

### 13.4 O valor zero do tipo gerador não pode derrubar o processo

`Generator` é exportado como estrutura, então embuti-lo por valor em
outra estrutura é natural e compila. Sem tratamento, a primeira geração
estourava com desreferência de ponteiro nulo.

**Requisito.** Quando o receptor não tem fonte de entropia (valor zero ou
ponteiro nulo), a geração recorre à entropia do gerador padrão do pacote.
O comportamento deve estar documentado no comentário do tipo.

### 13.5 Fonte de entropia nula é erro de configuração, não de execução

**Requisito.** `NewGeneratorWith` entra em pânico **na construção** se
receber fonte nula, com mensagem clara em português. Um erro de
configuração deve aparecer no boot, não na primeira geração em produção.

### 13.6 Erro de parsing não vaza estado parcial

**Requisito.** Ver 9.2: em qualquer caminho de erro, o UUID devolvido é o
valor zero.

### 13.7 A própria suíte de testes precisa estar limpa sob detector de corrida

Na primeira implementação, 256 goroutines escreviam em uma variável
global de sumidouro sem sincronização, e o benchmark paralelo fazia o
mesmo. A biblioteca estava limpa; o defeito era só do consumidor de
resultado nos testes — mas derrubava a suíte e impedia que o detector de
corrida fosse usado como rotina, o que mascararia uma corrida real
futura.

**Requisito.** Cada goroutine acumula localmente e publica uma única vez
em posição própria de um slice; benchmarks paralelos impedem a eliminação
pelo compilador sem escrever em estado compartilhado (usando o mecanismo
da linguagem para manter o valor vivo). A suíte inteira deve passar com o
detector de corrida ligado.

### 13.8 Higiene de repositório

Arquivos rastreados são baixados por `go get` e ficam no cache de módulos
de todos os consumidores. O repositório deve ter `.gitignore` cobrindo,
no mínimo: arquivos de metadados do sistema operacional (`.DS_Store`),
binários e perfis de teste (`*.test`, `*.out`, `*.prof`), o executável de
exemplo e o corpus de fuzzing.

---

## 14. Decisões deliberadas — não refazer

Sugestões avaliadas em revisão e **descartadas com motivo**. Um modelo
que reconstrua o projeto tende a propô-las de novo; não as adote sem
argumento novo.

### 14.1 Não use conversão sem cópia para a string

Devolver uma string que aponta, sem cópia, para o buffer de 36 bytes
alocado na pilha da função produz **comportamento indefinido** assim que
o quadro for reutilizado. Como `String` é API pública, não há como
garantir que o consumidor não retenha o resultado. A implementação atual
(laço sobre buffer fixo) já é a mais rápida entre as testadas — mais
rápida, inclusive, que a variante baseada em codificação hexadecimal da
biblioteca padrão (26,70 ns contra 29,45 ns).

### 14.2 Não suba o piso da versão da linguagem sem necessidade

O código compila e passa em `go 1.22`; `math/rand/v2` existe desde essa
versão e nenhuma API posterior é usada. O campo `go` do manifesto declara
a versão **mínima** suportada — subi-lo reduz a base de consumidores sem
ganho concreto.

### 14.3 Não adote contador monotônico como padrão

Um contador de desempate resolve a ordenação por completo (medido: de
44,0% de pares fora de ordem para 0,0%) e custa pouco em fluxo serial
(+8,5%), mas o estado compartilhado destrói a escalabilidade que é o
principal atrativo da biblioteca: **32 vezes mais lento no caminho
paralelo** (7,40 ns para 233,6 ns). Se for oferecido, que seja por um
construtor dedicado e explícito, nunca como comportamento padrão.

### 14.4 Não troque o desenho de entropia sem decisão consciente

Ver a nota ao final da seção 11.

---

## 15. Semântica de ordenação — descreva com precisão

Esta é a parte da documentação que mais facilmente vira promessa falsa.

**O que é verdade:** sempre que o instante embutido de B for maior que o
de A, a string de B é maior que a de A, e o binário também, nos três
níveis. A ordenação é **cronológica na resolução do nível**.

**O que não é verdade:** não há contador monotônico. Dois UUIDs do mesmo
instante embutido são ordenados por bits aleatórios, ou seja,
aleatoriamente. Gerar um UUID custa cerca de 45 ns, bem menos que o passo
do relógio da maioria dos hosts, então empates são o **caso comum**, não
a exceção. O Nível 1 empata praticamente 100% das vezes em qualquer
plataforma, porque sua resolução é o milissegundo.

Medição real (Apple M2, 100.000 pares consecutivos): Nível 1, 100,0% de
empates; Níveis 2 e 3, cerca de 87%. O relógio desse host oferece apenas
4.935 instantes distintos em 100.000 leituras.

**Consequência para os testes:** ver 16.3. Um teste que conta
"regressões de ordenação em uma sequência fechada, com limiar tolerado"
mede a resolução do relógio do host, não a biblioteca, e falha
permanentemente em hosts de relógio com resolução de microssegundo.
**Não escreva esse teste.**

---

## 16. Suíte de testes obrigatória

### 16.1 Regras estruturais

- Todos os testes ficam em uma subpasta própria (`tests/`), **fora da
  raiz de produção**, e importam a biblioteca **pelo caminho público do
  módulo**, como um consumidor externo faria — não como pacote interno.
- Os testes de massa (1.000.000 de amostras) devem ser puláveis pelo modo
  curto do executor de testes.
- Mensagens de falha em português, e formatando os **bytes** do UUID, não
  a string (ver 9.1).

### 16.2 Cobertura mínima por assunto

| Assunto | O que travar |
|---------|--------------|
| Versão e variante | Todo UUID gerado, em qualquer nível — inclusive níveis desconhecidos — tem versão 7 e variante `10`. |
| Round-trip | Binário → string → binário devolve os mesmos 16 bytes, em milhares de amostras, incluindo os extremos (todos os bytes zero e todos em 0xFF). |
| Layout de bits | Com entropia determinística (zero e todos-uns), conferir posição a posição os 48 bits de milissegundos, os 12 de `rand_a` e os 10 do topo de `rand_b`. |
| Faixas sub-ms | Nos Níveis 2 e 3, microssegundos importados em 0..999; no Nível 3, nanossegundos em 0..999 — em centenas de milhares de amostras. Documentar que no Nível 1 esses campos chegam a 4095 e 1023. |
| Importação coerente | O instante reconstruído cai dentro do intervalo medido em torno da geração, com tolerância de 1 ms. Incluir um vetor conhecido, de valor fixo. |
| Strings inválidas | Tamanho errado, hifens errados, dígitos não hexadecimais; fronteiras de tamanho (35, 36, 37); maiúsculas e minúsculas aceitas; valor devolvido junto com o erro é o zero. |
| Robustez do analisador | **Toda mutação de um byte** sobre uma string canônica válida (36 × 256 entradas) sem pânico, incluindo separadores extras em posições inesperadas; e uma campanha de *fuzzing* dedicada. |
| Bordas do gerador | Fonte de entropia nula rejeitada na construção; gerador obtido pelo valor zero do tipo não derruba o processo; nível desconhecido se comporta como Nível 1; sorteios de entropia por nível (2 no Nível 1 e nos desconhecidos, 1 nos Níveis 2 e 3). |
| Alocações | Zero em `Generate`, `FromString` e `ImportBinary`; no máximo uma em `GenerateString`. |
| Unicidade | Nenhuma duplicata em 1.000.000 por nível, e nenhuma em mais de 1.000.000 gerados por dezenas de goroutines simultâneas. |
| Concorrência | 1.000.000 distribuídos por centenas de threads sem erro nem corrupção; suíte inteira limpa sob detector de corrida. |
| Atalhos de pacote | As funções de pacote e os apelidos de conversão testados diretamente, não apenas por via indireta. |

### 16.3 Ordenação — a forma correta do teste

- **Invariante independente do relógio (obrigatória).** Construir pares
  em que o instante embutido de B é comprovadamente maior que o de A, e
  exigir que a ordem se mantenha em binário e em string, nos três níveis,
  em centenas de milhares de amostras.
- **Diagnóstico (recomendado).** Um teste informativo que **relata** a
  taxa de empates por nível, sem falhar. Ele documenta a característica
  do host e evita que alguém a confunda com defeito.
- **Teste contra o relógio real (opcional).** Se existir, precisa inserir
  uma pausa **maior que a resolução do relógio de qualquer host
  suportado** (200 microssegundos é suficiente) entre as gerações, e
  então exigir ordem **estrita**.
- **Proibido.** Contar regressões em sequência fechada com limiar
  tolerado. Ver seção 15.

### 16.4 Nomes de teste sugeridos

Agrupados por arquivo, como referência de organização (não normativos nos
nomes, mas normativos no conteúdo):

- **geração** — `TestVersionVariant`, `TestRoundTripString`,
  `TestConversionAliases`, `TestFromStringInvalid`, `TestUniqueness`,
  `TestMonotonicity` (na forma com pausa, de 16.3),
  `TestSubMillisecondMatchesClock`, `TestTimestampMatchesClock`.
- **parsing** — `TestFromStringNeverPanics`, `TestFromStringExtraHyphen`,
  `TestFromStringLengthBoundaries`, `TestFromStringCaseInsensitive`,
  `TestFromStringHyphenPositions`, `TestFromStringErrorReturnsZero`,
  `TestStringKnownVector`, `TestStringRoundTripExtremes`.
- **layout** — `TestLayoutZeroEntropy`, `TestLayoutFullEntropy`,
  `TestVersionVariantAllLevels`.
- **importação** — `TestImportKnownVector`, `TestImportInvalidString`,
  `TestImportLevel3`, `TestImportSubMillisecondRanges`,
  `TestImportRoundTripFromClock`.
- **ordenação** — `TestOrderingFollowsEmbeddedTime`,
  `TestStringOrderMatchesBinaryOrder`, `TestTieRateReport`,
  `TestConcurrentUniqueness`.
- **robustez** — `TestNewGeneratorWithNilSourcePanics`,
  `TestZeroGeneratorUsesDefaultEntropy`, `TestEntropyDrawsPerLevel`,
  `TestUnknownLevelBehavesAsLevel1`, `TestPackageLevelShortcuts`.
- **alocações** — `TestGenerateZeroAllocations`,
  `TestGenerateStringSingleAllocation`, `TestFromStringZeroAllocations`,
  `TestImportZeroAllocations`.
- **fuzzing** — `FuzzFromString` (ausência de pânico, round-trip e
  coerência com a importação).
- **massa** — `TestMassOneMillion`, `TestMassConcurrent`.

---

## 17. Benchmarks obrigatórios

### 17.1 Benchmarks do executor de testes

`BenchmarkGenerateLevel1`, `BenchmarkGenerateLevel2`,
`BenchmarkGenerateLevel3`, `BenchmarkGenerateStringLevel1`,
`BenchmarkGenerateStringLevel3`, `BenchmarkGenerateLevel3Parallel`,
`BenchmarkFromString`, `BenchmarkImportBinary`.

Todos reportando alocações por operação.

### 17.2 Geração em massa

Medir o tempo para gerar **1.000.000** de UUIDs em cada cenário,
reportando tempo total, nanossegundos por UUID e UUIDs por milissegundo:

- Nível 1, 2 e 3 em binário;
- Nível 1, 2 e 3 em string;
- variante concorrente do Nível 3 (centenas de goroutines), com vazão
  agregada.

### 17.3 Executável autônomo de benchmark

Um programa executável em subpasta própria dos testes, que imprime a
tabela dos cenários acima. Requisitos:

- parâmetro de linha de comando para a quantidade por cenário, com
  padrão de 1.000.000;
- **passagem de aquecimento descartada** antes da primeira medição (ver
  10.4);
- consumo dos resultados em uma variável sumidouro, para impedir a
  eliminação do trabalho pelo compilador — **sem** estado compartilhado
  entre goroutines (ver 13.7);
- taxa calculada a partir de nanossegundos.

---

## 18. Organização de arquivos

**A raiz contém apenas o necessário para usar a biblioteca em produção**,
mais os arquivos de suporte ao desenvolvimento explicitamente
autorizados. Preserve essa separação: não acrescente arquivos
não-produtivos à raiz.

```
go-loghub-uuid/
├── go.mod
├── uuid.go             # tipos, níveis, Generator, geração, gerador padrão
├── conversion.go       # String / FromString e os dois apelidos
├── import.go           # Time, Import, ImportBinary
├── README.md
├── STARTHERE.md
├── LICENSE
├── .gitignore
├── CLAUDE.md           # instruções de manutenção (apoio ao desenvolvimento)
├── PROMPT.md           # este documento
├── docs/
│   ├── DEPLOY-FAST.md
│   ├── DEPLOY-FULL.md
│   ├── TEST-AND-BENCHMARK.md
│   ├── SPEC.md
│   └── git.md
└── tests/
    ├── doc.go
    ├── generation_test.go
    ├── parsing_test.go
    ├── layout_test.go
    ├── import_test.go
    ├── ordering_test.go
    ├── robustness_test.go
    ├── alloc_test.go
    ├── fuzz_test.go
    ├── benchmark_test.go
    └── benchmark-bulk/
        └── main.go
```

Três arquivos de produção, cada um com uma responsabilidade distinta —
geração, conversão, importação. Não fundir, não fragmentar mais.

---

## 19. Documentação a produzir

Toda em português do Brasil, sem emojis, com links relativos **que
existam de fato** (links quebrados para arquivos inexistentes já foram um
defeito registrado).

1. **`README.md`** — descrição curta, instalação, uso rápido, tabela dos
   níveis, **seção "Aviso de segurança"** (12.1) e ponteiros para o
   restante. Os trechos de exemplo precisam **compilar**: nada de baixar
   o módulo local do próprio exemplo, nada de redeclarar variável no
   mesmo escopo, e o manifesto de exemplo coerente com o comando de
   instalação mostrado ao lado.
2. **`STARTHERE.md`** — mapa completo: o que é, árvore de arquivos
   **fiel ao repositório real**, listagem da API pública inteira, layout
   de bits, caminhos de leitura recomendados, comandos úteis e
   desempenho de referência.
3. **`docs/DEPLOY-FAST.md`** — uso rápido: instalar e gerar uma string em
   poucos segundos; escolher o nível; criar um gerador único no boot.
4. **`docs/DEPLOY-FULL.md`** — uso completo de todas as funções: binário,
   string, conversões, importação de tempo, reconstrução do instante,
   gerador com entropia personalizada, tabela de entropia por nível e a
   advertência de segurança como **requisito** para identificadores que
   precisem ser inadivinháveis.
5. **`docs/TEST-AND-BENCHMARK.md`** — como rodar testes, detector de
   corrida, fuzzing, benchmarks e a geração em massa; tabelas de
   referência **identificando o host**; explicação de por que o Nível 1 é
   o mais lento (10.3) e de onde está o teto de desempenho (10.2).
6. **`docs/SPEC.md`** — especificação **independente de linguagem**, sem
   exemplos de código, suficiente para reimplementar a biblioteca do zero
   em qualquer linguagem: conceitos, os três níveis, layout de bits,
   algoritmos de geração/importação/conversão, contrato conceitual de
   API, requisitos não-funcionais (desempenho, concorrência, qualidade da
   aleatoriedade, robustez), casos de teste obrigatórios, benchmark
   obrigatório e organização de arquivos sugerida.
7. **`docs/git.md`** — comandos de publicação e versionamento do
   repositório.

Coerência obrigatória: o layout de bits aparece no comentário de
documentação da geração, no `STARTHERE.md` e no `docs/SPEC.md`. Se mudar
em um, muda nos três.

---

## 20. Comandos que a documentação deve ensinar

```bash
go build ./...
go vet ./...
go test ./tests/ -v
go test ./tests/ -short -v
go test ./tests/ -race
go test ./tests/ -run '^$' -bench Benchmark -benchmem
go test ./tests/ -run '^$' -fuzz FuzzFromString -fuzztime 60s
go run ./tests/benchmark-bulk
```

---

## 21. Critérios de aceitação

A entrega só está pronta quando **todos** os itens abaixo forem
verdadeiros, verificados por execução — não por inspeção:

1. `go build ./...` sem erros; `go vet ./...` sem apontamentos; `gofmt`
   sem saída.
2. Suíte de testes inteira passando, **inclusive com o detector de
   corrida ligado**.
3. Campanha de fuzzing do analisador executada por pelo menos 60
   segundos, sem falha.
4. Zero alocações confirmadas por teste em `Generate`, `FromString` e
   `ImportBinary`; no máximo uma em `GenerateString`.
5. Nenhuma duplicata em 1.000.000 de UUIDs por nível.
6. Todo UUID gerado, em todos os níveis e também em níveis
   desconhecidos, com versão 7 e variante RFC.
7. Tabelas de desempenho da documentação **medidas no código final**,
   com aquecimento, identificando o host.
8. Advertência de segurança presente nos quatro lugares de 12.1.
9. Árvore de arquivos e links da documentação conferindo com o
   repositório real.
10. Raiz contendo apenas os arquivos autorizados na seção 18.

---

## 22. API considerada e deliberadamente não implementada

Registrado para que a decisão seja consciente em uma reconstrução. Nada
disso é defeito; são adições de superfície pública, todas úteis, nenhuma
delas alterando o layout de bits ou o caminho quente. Implemente-as
somente se o dono do projeto decidir ampliar o escopo.

| Adição | Motivo pelo qual seria útil |
|--------|-----------------------------|
| Serialização de texto (`MarshalText` / `UnmarshalText`) | Uso nativo com JSON, YAML e formatos derivados; hoje um UUID dentro de uma estrutura serializa como arranjo de 16 números. |
| Serialização JSON dedicada | Controle direto do formato. |
| Integração com banco de dados (valor e leitura) | Gravar e ler sem conversão manual — caso de uso central de um projeto de log. |
| `Time.ToTime` | A aritmética de reconstrução do instante hoje é duplicada na documentação e nos testes; é o tipo de código que o usuário erra. |
| `AppendTo` sobre buffer do chamador | Medido: 29% mais rápido que `String` e **zero alocações**; relevante para quem serializa milhões de identificadores. Seria a base natural de `MarshalText`. |
| `Bytes` | Contorna a armadilha do `%x` descrita em 9.1. |
| `FromBytes` | Interoperabilidade com dados binários vindos de fora. |
| `IsZero`, `IsValid`, `Compare` | Conveniências triviais e sem custo. |
| Relógio injetável | Permite vetores dourados de verdade e teste determinístico das bordas (viradas de segundo e de milissegundo, ano 2262, relógio pré-época). **Consequência prática hoje:** a correção de 13.2 não tem teste de regressão na suíte, porque não há como fixar o instante de fora do pacote. Se um relógio injetável for adicionado, este é o primeiro teste a escrever. |
| Construtor de gerador monotônico | Ordenação estrita mesmo dentro do mesmo instante, ao custo descrito em 14.3. Se existir, precisa ser opcional e explícito. |

---

## Prompt

Construa a biblioteca UUIDv7 multinível em Go conforme a especificação
acima, produzindo o código de produção, a suíte de testes, os
benchmarks, o executável de benchmark em massa e toda a documentação
listada na seção 19. Ao final, execute a verificação da seção 21 e
relate os resultados medidos.
