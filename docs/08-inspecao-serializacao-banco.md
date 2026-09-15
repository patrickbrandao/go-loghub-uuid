# Inspeção, Contrato de API, Serialização e Banco de Dados

> Parte da especificação de desenvolvimento agnóstica de linguagem.
> Reproduz as seções 7 ("Inspeção e Contrato de API") e 8
> ("Serialização, Banco de Dados e Valores Especiais") de `docs/SPEC.md`
> (documento original, já removido — ver [INDEX.md](INDEX.md)). As duas
> seções descrevem, juntas, tudo que se faz com um `UUID` depois de
> gerado: ler seus campos de volta, e entregá-lo a um formato de
> transporte ou a uma coluna de banco.

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
  uma operação entrega um instante, a outra entrega os bits. Ver
  [10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)
  seção 11.3.
- **`ImportBinary(UUID) Time`** e **`Import(texto) (Time, erro)`**:
  extração completa dos campos de tempo de um UUIDv7, devolvendo uma
  estrutura com quatro campos. A forma em texto analisa a string pelo
  analisador **estrito** da seção 6.2 (ver
  [07-parsing-e-conversao.md](07-parsing-e-conversao.md)), devolve o
  sentinela puro com a estrutura zerada em caso de recusa, e delega à
  forma binária.
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
  instante da seção 3.6 (ver
  [04-uuidv7-formato-e-niveis.md](04-uuidv7-formato-e-niveis.md)), e a
  estrutura devolvida **NÃO** passa pelo descarte da leitura por nível
  acima.
- **`GregorianTime() (GregorianTime, bool)`** (método de `UUID`): o
  campo de tempo de 60 bits das versões 1 e 6, recomposto pela leitura
  inversa da seção 4.1 (ver
  [05-outras-versoes-uuid.md](05-outras-versoes-uuid.md)). Devolve falso
  para as demais versões, **inclusive a 2**, cujo carimbo perdeu os 32
  bits baixos e não é utilizável. O tipo devolvido conta tiques de cem
  nanossegundos desde 1582 e oferece as duas conversões:
  `UnixTime() (sec, nsec)`, o par canônico da conversão inversa da seção
  4.1, e `Time()`, o instante da linguagem em UTC.
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
- **`AppendBinary(dst) dst`**: o equivalente binário, escrevendo os 16
  bytes em ordem de rede no fim do buffer do chamador, também sem alocar
  quando houver capacidade. O conteúdo é idêntico ao da serialização
  binária, portanto acrescentá-lo **não** muda formato de dados gravado.

  **Regra normativa — os dois anexadores andam juntos.** Uma
  implementação que ofereça um **DEVE** oferecer o outro. A assimetria
  não tem justificativa técnica: se o motivo de existir o anexador de
  texto é serializar em volume sem alocar, o mesmo motivo vale para os
  bytes, e quem grava em coluna binária é justamente quem grava em
  volume. Em linguagens que definam interfaces de anexação em texto e em
  binário, como o Go a partir da 1.24, satisfazer só a primeira deixa o
  tipo pela metade em todo consumidor genérico que prefira anexar a
  alocar.

  O lado binário não precisa de uma segunda forma sem erro, ao contrário
  do de texto: a formatação canônica é um cálculo, os 16 bytes não são.
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
da seção 3.5 (ver
[04-uuidv7-formato-e-niveis.md](04-uuidv7-formato-e-niveis.md)), que são
o sentido inverso das acima: recebem um instante e um nível e devolvem o
identificador que os delimita. Diferentemente de todas as operações
desta seção, elas não são métodos de um UUID.

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
em [04-uuidv7-formato-e-niveis.md](04-uuidv7-formato-e-niveis.md) seções
3.5 e 3.6.

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
   - O tipo de escrita binária **DEVE implementar as mesmas interfaces de
     serialização** que o tipo padrão (`MarshalText`, `UnmarshalText`,
     `MarshalBinary`, `UnmarshalBinary`), delegando para a implementação
     do tipo base. Sem esses métodos, `encoding/json` serializaria o
     valor como vetor de 16 números inteiros em vez da string canônica,
     violando a regra 1 desta seção.
   - Fornecer um tipo `NullUUID` contendo o UUID e um booleano `Valid`
     para campos de tabela que permitem valor `NULL`, e o equivalente
     para a escrita binária. O equivalente binário anulável **DEVE
     implementar as mesmas interfaces de serialização** que o anulável
     padrão (`MarshalJSON`, `UnmarshalJSON`, `MarshalText`,
     `UnmarshalText`, `MarshalBinary`, `UnmarshalBinary`), para que a
     tabela normativa de ausência abaixo se aplique igualmente a ambos.
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
     normativa. Entrada inválida devolve erro e, nas desserializações,
     **preserva o receptor inteiro**, identificador e booleano, conforme
     a seção 6.4 (ver
     [07-parsing-e-conversao.md](07-parsing-e-conversao.md)); só a
     leitura de valor de banco derruba o booleano, pela exceção
     registrada ali.
3. **Valores Especiais**:
   - `Nil`: todos os 16 bytes em zero (`00000000-0000-0000-0000-000000000000`).
   - `Max`: todos os 16 bytes em `0xFF` (`ffffffff-ffff-ffff-ffff-ffffffffffff`).
   - `Compare(a, b)`: comparação byte a byte em ordem lexicográfica
     retornando `-1`, `0` ou `1`.

---

Ver [03-guia-completo.md](03-guia-completo.md) seções "Inspecionar",
"JSON, texto e binário" e "Banco de dados" para exemplos de uso em Go, e
[10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)
para o porquê de `BinaryUUID` não ganhar anexadores nem métodos de
inspeção próprios.
