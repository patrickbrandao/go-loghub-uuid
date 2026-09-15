# Visão Geral e Conceitos Gerais

> Parte da especificação de desenvolvimento **agnóstica de linguagem** da
> biblioteca `go-loghub-uuid`. Descreve, sem depender de bibliotecas
> externas ou de recursos exclusivos do Go, o objetivo e o escopo da
> biblioteca e os conceitos universais de layout de bits que valem para
> qualquer versão de UUID. Um desenvolvedor ou modelo de IA deve
> conseguir produzir uma implementação completa, interoperável, de alto
> desempenho e estritamente correta seguindo os documentos numerados
> desta pasta — sem cometer nenhum dos erros históricos já encontrados e
> corrigidos neste projeto. Ver [INDEX.md](INDEX.md) para o mapa completo.

---

## 1. Objetivo e Escopo da Biblioteca

Construir uma biblioteca leve, de altíssimo desempenho (dezenas de
nanossegundos por identificador, zero alocações de heap no caminho
quente) e segura para concorrência pesada, cobrindo todo o padrão
**RFC 9562** com extensão de precisão temporal sub-milissegundo:

1. **UUIDv7 Multinível**:
   - **Nível 1**: UUIDv7 padrão RFC 9562 com carimbo de milissegundos
     Unix e 74 bits de entropia. Existe também pelo nome por versão
     (`GenerateV7`), como as versões do item 2, e pelo nome por nível
     (`GenerateV7Level1`), que é o mesmo (seção 3.1 de
     [04-uuidv7-formato-e-niveis.md](04-uuidv7-formato-e-niveis.md)).
   - **Nível 2**: UUIDv7 com carimbo de milissegundos e
     **microssegundos** embutidos em `rand_a` (62 bits de entropia).
     Existe também pelo nome por nível (`GenerateV7Level2`).
   - **Nível 3**: UUIDv7 com carimbo de milissegundos,
     **microssegundos** em `rand_a` e **nanossegundos** no topo de
     `rand_b` (52 bits de entropia). Existe também pelo nome por nível
     (`GenerateV7Level3`).
2. **Todas as demais versões da RFC 9562** (ver
   [05-outras-versoes-uuid.md](05-outras-versoes-uuid.md)):
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
3. **Conversão e Análise (Parsing) Segura** (ver
   [07-parsing-e-conversao.md](07-parsing-e-conversao.md)):
   - Analisador estrito: formato canônico `8-4-4-4-12`.
   - Analisador permissivo: 4 formatos aceitos (canônico com hífens, sem
     hífens com 32 dígitos, entre chaves `{...}`, e prefixo URN
     `urn:uuid:...`).
   - Algoritmo de parsing imune a pânicos e leituras fora dos limites
     (out-of-bounds).
4. **Inspeção e Extração** (ver
   [08-inspecao-serializacao-banco.md](08-inspecao-serializacao-banco.md)):
   - Extração de versão, variante, instante temporal (Unix/Gregorian),
     sequência de relógio, nó de rede, domínio e ID local.
   - Duas leituras de tempo do UUIDv7 com políticas opostas, ambas
     normativas: a leitura por nível, que devolve um instante e descarta
     o que não pode ser tempo, e a extração completa, que devolve os
     campos crus sem julgar a origem dos bits (seção 7).
5. **Construção a Partir de um Instante Explícito** (ver
   [04-uuidv7-formato-e-niveis.md](04-uuidv7-formato-e-niveis.md) seção
   3.5 e 3.6):
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
     versões 1, 2 e 6 ficam de fora por decisão registrada em
     [10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)
     seção 11.3.
6. **Serialização e Integração** (ver
   [08-inspecao-serializacao-banco.md](08-inspecao-serializacao-banco.md)):
   - Serialização de texto e JSON como string canônica entre aspas.
   - Suporte a colunas de banco de dados textuais e binárias de 16 bytes,
     com e sem `NULL`: o tipo simples, o anulável (`NullUUID`), o de
     escrita binária (`BinaryUUID`) e o anulável binário
     (`NullBinaryUUID`). A seção 8 especifica os quatro.
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

Próximo documento: comece pelo [02-guia-rapido.md](02-guia-rapido.md)
para gerar um UUIDv7 em poucos segundos, ou vá direto ao
[04-uuidv7-formato-e-niveis.md](04-uuidv7-formato-e-niveis.md) para o
formato completo do UUIDv7 multinível.
