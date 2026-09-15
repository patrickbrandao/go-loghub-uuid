---
name: go-loghub-uuid
description: Use when writing, reviewing, or debugging Go code that generates, parses, stores, or migrates UUIDs with github.com/patrickbrandao/go-loghub-uuid — a dependency-free UUIDv7 (RFC 9562) library with millisecond/microsecond/nanosecond precision levels, time-range primary-key queries (MinAt/MaxAt/RangeAt), all RFC 9562 versions (v1 to v8), and database/sql integration (NullUUID, BinaryUUID). Also use when migrating Go code away from github.com/google/uuid, or whenever the user mentions UUIDv7, sortable/ordered primary keys, or time-ordered identifiers in a Go project.
license: See LICENSE in the repository root (MIT).
metadata:
  package: github.com/patrickbrandao/go-loghub-uuid
  go-package-name: loghubuuid
---

# go-loghub-uuid — guia para agentes de código

Biblioteca Go **sem dependências** (só biblioteca padrão) que gera
**UUIDv7** (RFC 9562) com três níveis de precisão temporal embutida, e
também todas as demais versões da RFC — 1, 2, 3, 4, 5, 6 e 8. Módulo:
`github.com/patrickbrandao/go-loghub-uuid`; nome do pacote no código:
`loghubuuid` (importe com o apelido `uuid`, como o próprio projeto faz).

Esta skill ensina **como usar** a biblioteca em código Go de aplicação.
Para a especificação completa (layout de bits, decisões de projeto,
casos de teste), veja `docs/INDEX.md` no repositório da biblioteca —
esta skill não repete aquele conteúdo, só a parte prática de consumo da
API.

## Instalação e import

```bash
go get github.com/patrickbrandao/go-loghub-uuid
```

```go
import uuid "github.com/patrickbrandao/go-loghub-uuid"
```

Requer Go 1.22 ou superior. O apelido `uuid` é só conveniência — o nome
real do pacote é `loghubuuid`.

## Regras rápidas antes de escrever código

1. **Para chave primária de banco de dados, use UUIDv7**, não v1 nem v4.
   `Generate(uuid.Level1|Level2|Level3)`, ou os nomes por versão/nível
   (`GenerateV7`, `GenerateV7Level1/2/3`) descritos abaixo.
2. **Escolha um nível por tabela e nunca o troque.** `Level1` (só
   milissegundo) é o padrão RFC e a opção mais compatível; `Level2`
   (+ microssegundo) e `Level3` (+ nanossegundo) dão desempate mais fino
   dentro do mesmo milissegundo, ao custo de menos bits aleatórios. As
   fronteiras de consulta (`MinAt`/`MaxAt`/`RangeAt`) só funcionam
   corretamente quando **todos** os UUIDs da coluna foram gerados no
   mesmo nível — misturar níveis faz consultas por intervalo devolverem
   linhas a menos, **sem erro nenhum**.
3. **Em serviços de alto volume, crie um `*Generator` uma vez no boot**
   (`var Gen = uuid.NewGenerator()`) e reutilize-o; não chame
   `NewGenerator()` a cada requisição. As funções de pacote
   (`uuid.Generate`, `uuid.GenerateString`, ...) já usam um gerador
   padrão interno e também são seguras para uso concorrente sem
   configuração alguma.
4. **Não use o gerador padrão para segredos** (token de sessão, link de
   recuperação de senha): ele prioriza velocidade, não imprevisibilidade
   criptográfica — para isso use `uuid.NewCryptoGenerator()`. Mesmo
   assim, todo UUIDv7 expõe o instante de criação nos primeiros bytes,
   então não é adequado como segredo de qualquer forma.
5. **`SetNodeID` e `SetClockSequence` alteram estado global do processo**
   (usado por v1, v2 e v6). Evite chamá-los fora do boot, a não ser que
   precise fixar um nó de rede real. Em testes que os chamam, restaure o
   estado anterior ao final e nunca rode esses testes em paralelo — o
   estado é compartilhado pelo processo inteiro.
6. **`Import`/`ImportBinary` são "cegos" quanto ao nível**: sempre leem
   `rand_a` como microssegundos e o topo de `rand_b` como nanossegundos,
   mesmo em um UUID de `Level1` onde esses bits são aleatórios. Se você
   sabe o nível, prefira `(UUID).TimestampWithLevel(nível)`, que descarta
   o campo quando ele está fora de `0..999` em vez de devolver ruído
   como se fosse tempo.
7. **Vindo de `github.com/google/uuid`?** Troque só o caminho do import
   — a maioria dos nomes de função foi replicada em `compat.go`. Veja a
   seção "Migrando de github.com/google/uuid" abaixo antes de reescrever
   qualquer coisa.

## Qual versão/nível usar

| Preciso de... | Use |
|---|---|
| Chave primária ordenável por tempo, caso comum | `uuid.GenerateV7()` ou `uuid.Generate(uuid.Level1)` |
| Chave primária com desempate mais fino sob rajada (logs/eventos de alta frequência) | `uuid.GenerateV7Level2()` ou `uuid.GenerateV7Level3()` |
| Consultar uma tabela UUIDv7 por janela de tempo, sem coluna de timestamp | `uuid.MinAt`/`uuid.MaxAt`/`uuid.RangeAt` (mesmo nível da geração) |
| Reprocessar histórico preservando a ordem cronológica da chave | `uuid.GenerateAt(nível, instanteOriginal)` |
| Identificador puramente aleatório, sem informação de tempo | `uuid.GenerateV4()` |
| Identificador determinístico a partir de um nome (ex.: derivar de uma URL) | `uuid.GenerateV5(espaço, nome)` (SHA-1; prefira a v3/MD5 só por compatibilidade) |
| Identificador de credencial/entidade DCE, legado | `uuid.GenerateV2(...)` — não use em sistemas novos |
| Compatibilidade com sistema legado que exige UUIDv1/v6 | `uuid.GenerateV1()` / `uuid.GenerateV6()` (v6 é ordenável por texto, v1 não é) |
| Esquema de bits totalmente proprietário | `uuid.GenerateV8(dados)` |
| Token de sessão, link secreto, chave de recuperação | **não use UUID como segredo.** Se ainda assim precisar de aleatoriedade forte, `uuid.NewCryptoGenerator()` |

## Tipos e construtores principais

```go
type Level uint8                 // Level1, Level2, Level3
type UUID [16]byte                // o valor binário de 128 bits
type Generator struct{ /* ... */ } // criado uma vez, seguro para concorrência

g := uuid.NewGenerator()                    // padrao rapido: ChaCha8 do runtime do Go, sem lock
g  = uuid.NewGeneratorWith(minhaFonte)       // minhaFonte func() uint64, precisa ser thread-safe
g  = uuid.NewGeneratorWithReader(algumReader) // qualquer io.Reader thread-safe (ex.: crypto/rand.Reader)
g  = uuid.NewCryptoGenerator()               // atalho para NewGeneratorWithReader(crypto/rand.Reader)
```

`NewGeneratorWith` e `NewGeneratorWithReader` entram em pânico
imediatamente se receberem fonte/leitor nulo — é erro de configuração e
deve aparecer no boot, não na primeira geração. Um `*Generator` de valor
zero, ou um ponteiro nulo usado como receptor, **não** entra em pânico:
recorre ao gerador padrão do pacote.

## Referência rápida da API completa

**Geração de UUIDv7** (binário ou string; como método de `*Generator` e
como função de pacote usando o gerador padrão interno):

```go
u := uuid.Generate(uuid.Level1 /* ou Level2, Level3 */) // uuid.UUID
s := uuid.GenerateString(uuid.Level3)                   // string canônica

u  = uuid.GenerateV7()          // == Generate(Level1); UUIDv7 padrão da RFC pelo nome da versão
u  = uuid.GenerateV7Level1()    // == GenerateV7
u  = uuid.GenerateV7Level2()    // == Generate(Level2)
u  = uuid.GenerateV7Level3()    // == Generate(Level3)
// todos também existem como método: g.Generate(...), g.GenerateV7(), g.GenerateV7Level2(), ...
```

**Construção a partir de um instante** (para reprocessar histórico e
para consulta por intervalo — ver exemplos 1 e 2 abaixo):

```go
u := uuid.GenerateAt(uuid.Level2, instante)          // UUIDv7 daquele instante, bits livres SORTEADOS
s := uuid.GenerateAtString(uuid.Level2, instante)    // idem, em string
lo := uuid.MinAt(uuid.Level2, instante)              // menor UUIDv7 gerável naquele instante+nível
hi := uuid.MaxAt(uuid.Level2, instante)              // maior UUIDv7 gerável naquele instante+nível
lo, hi = uuid.RangeAt(uuid.Level2, inicio, fim)       // par [inicio, fim) pronto para WHERE id >= lo AND id < hi
```

**Outras versões da RFC 9562:**

```go
uuid.GenerateV1() uuid.UUID                                   // tempo gregoriano + sequência + nó
uuid.GenerateV2(domain uuid.Domain, id uint32) uuid.UUID       // DCE Security
uuid.GenerateV2Person() uuid.UUID                              // atalho: usa o UID do processo
uuid.GenerateV2Group() uuid.UUID                               // atalho: usa o GID do processo
uuid.GenerateV3(space uuid.UUID, name []byte) uuid.UUID        // MD5, determinístico
uuid.GenerateV4() uuid.UUID                                    // aleatório; também g.GenerateV4()
uuid.GenerateV5(space uuid.UUID, name []byte) uuid.UUID        // SHA-1, determinístico
uuid.GenerateV6() uuid.UUID                                    // gregoriano reordenado, ordenável
uuid.GenerateV8(data [16]byte) uuid.UUID                       // formato livre, bits do chamador
g.GenerateV8() uuid.UUID                                       // formato livre, bits sorteados
uuid.GenerateV8Random() uuid.UUID                              // idem, gerador padrão do pacote
```

Espaços de nomes para v3/v5: `uuid.NameSpaceDNS`, `uuid.NameSpaceURL`,
`uuid.NameSpaceOID`, `uuid.NameSpaceX500`.

**Conversão e parsing:**

```go
s := u.String()                       // binário -> string canônica
u, err := uuid.FromString(s)          // string canônica (só esse formato) -> binário
u, err  = uuid.Parse(s)               // aceita 4 formatos: canônico, sem hífen, {entre chaves}, urn:uuid:...
u, err  = uuid.ParseBytes(b)          // igual a Parse, a partir de []byte, sem alocar
u, err  = uuid.FromBytes(b)           // exatamente 16 bytes crus
u       = uuid.MustParse(s)           // como Parse, mas entra em pânico — só para constantes do código
err     = uuid.Validate(s)            // só valida, não devolve o UUID
if errors.Is(err, uuid.ErrInvalidFormat) { /* ... */ }
if uuid.IsInvalidLengthError(err)      { /* ... */ }
```

**Importação de tempo (cega quanto ao nível) e inspeção:**

```go
t, err := uuid.Import(s)              // uuid.Time{Seconds, Milliseconds, Microseconds, Nanoseconds}
t        = uuid.ImportBinary(u)

instante, ok := u.Timestamp()                       // v1, v6, v7; ms apenas no v7
instante, ok  = u.TimestampWithLevel(uuid.Level3)    // v7: soma a precisão sub-ms, descarta se fora de 0..999

u.Version() byte          // 1..8
u.Variant() byte          // 2 (0b10) para a variante RFC
u.IsValid() bool          // variante RFC e versão 1..8; Nil e Max também contam
u.IsZero() bool           // é uuid.Nil?
u.IsMax()  bool           // é uuid.Max (todos os bits em 1)?
u.Bytes() []byte          // cópia dos 16 bytes
u.URN()   string          // "urn:uuid:..."
a.Compare(b) int           // -1, 0, 1; para igualdade use a == b direto

g, ok := u.GregorianTime()           // v1, v6: tiques de 100ns desde 1582
seq, ok := u.ClockSequence()          // v1, v2 (só 6 bits altos), v6
no := u.NodeID()                      // v1, v2, v6; nil nas demais
dom, ok := u.Domain()                 // só v2
id, ok  := u.ID()                     // só v2

uuid.VersionString(u.Version()) // texto legível ("versao 7")
uuid.VariantString(u.Variant()) // idem para a variante
```

**Valores especiais e listas:**

```go
uuid.Nil   // 00000000-0000-0000-0000-000000000000
uuid.Max   // ffffffff-ffff-ffff-ffff-ffffffffffff

lista := uuid.UUIDs{a, b, c}
lista.Strings() // []string

import "slices"
slices.SortFunc(lista, uuid.UUID.Compare) // forma recomendada de ordenar; UUIDs não implementa sort.Interface
```

**Serialização sem alocar, JSON, texto e binário:**

```go
buf = u.AppendTo(buf[:0])                 // 36 bytes canônicos, sem alocar se buf tiver capacidade
buf, _ = u.AppendText(buf[:0])             // igual, com a assinatura encoding.TextAppender (Go 1.24)
buf, _ = u.AppendBinary(buf[:0])           // 16 bytes crus, sem alocar
data, _ := u.MarshalText()                 // implementa encoding.TextMarshaler -> JSON vira string canônica
data, _  = u.MarshalBinary()               // implementa encoding.BinaryMarshaler
err := (&u).UnmarshalText(data)
err  = (&u).UnmarshalBinary(data)
```

**Banco de dados (`database/sql`):**

```go
u.Value() (driver.Value, error)   // grava sempre como string canônica
(&u).Scan(src any) error          // aceita nil, string (qualquer formato de Parse) e []byte de 16 bytes

type uuid.NullUUID struct {
    UUID  uuid.UUID
    Valid bool // false = coluna NULL
}
// NullUUID e NullBinaryUUID implementam Scan, Value, MarshalJSON/UnmarshalJSON,
// MarshalText/UnmarshalText e MarshalBinary/UnmarshalBinary.

type uuid.BinaryUUID uuid.UUID // grava/lê 16 bytes crus em vez da string; para BINARY(16)/BLOB
type uuid.NullBinaryUUID struct {
    UUID  uuid.UUID
    Valid bool
}
```

**Estado global de v1/v2/v6** (relógio, sequência e nó — cuidado, ver
regra 5 acima):

```go
uuid.NodeID() []byte              // cópia do nó atual (sorteado uma vez, bit multicast ligado)
uuid.SetNodeID(id []byte) bool    // false se id tiver menos de 6 bytes; nada é alterado nesse caso
uuid.ClockSequence() int          // sequência de 14 bits em uso
uuid.SetClockSequence(seq int)    // -1 sorteia uma sequência inédita (ressincroniza com o relógio real)
uuid.GetTime() (uuid.GregorianTime, uint16)
```

**Erros exportados:** `uuid.ErrInvalidFormat`, `uuid.ErrInvalidLength`,
`uuid.ErrInvalidBrackets`, `uuid.ErrInvalidScanType`,
`uuid.ErrEntropySource`.

## Exemplos

### 1. Chave primária UUIDv7 (Level1/2/3) ao inserir um registro

```go
package main

import (
	"database/sql"
	"fmt"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// Gen é criado uma unica vez, no boot, e reutilizado por toda goroutine.
var Gen = uuid.NewGenerator()

func criarPedido(db *sql.DB, cliente string, total int64) (string, error) {
	// Level1 basta para a maioria das tabelas. Use Level2/Level3 se a
	// tabela recebe muitas linhas dentro do mesmo milissegundo e voce
	// quer desempate mais fino sem trocar de coluna de ordenacao.
	id := Gen.GenerateString(uuid.Level1)

	_, err := db.Exec(
		"INSERT INTO pedidos (id, cliente, total_centavos) VALUES ($1, $2, $3)",
		id, cliente, total,
	)
	if err != nil {
		return "", fmt.Errorf("inserir pedido: %w", err)
	}
	return id, nil
}
```

### 2. Consulta por intervalo de tempo sem coluna de timestamp

A chave já está em ordem cronológica; a janela de tempo vira varredura
de faixa no próprio índice primário. **Use o mesmo nível com que a
coluna foi gravada.**

```go
func pedidosDasUltimas24h(db *sql.DB) (*sql.Rows, error) {
	fim := time.Now()
	inicio := fim.Add(-24 * time.Hour)

	// RangeAt devolve o par de um intervalo semiaberto [inicio, fim).
	lo, hi := uuid.RangeAt(uuid.Level1, inicio, fim)

	return db.Query(
		"SELECT id, cliente, total_centavos FROM pedidos WHERE id >= $1 AND id < $2 ORDER BY id",
		lo.String(), hi.String(),
	)
}
```

### 3. Analisar e validar um UUID vindo de entrada do usuário (path/query param)

`Parse` aceita os quatro formatos usuais; use-o para entrada externa em
vez de `FromString`, que só aceita a forma canônica com hífens.

```go
func handlerBuscarPedido(w http.ResponseWriter, r *http.Request) {
	bruto := r.PathValue("id")

	id, err := uuid.Parse(bruto)
	if err != nil {
		// errors.Is(err, uuid.ErrInvalidFormat) tambem funciona aqui
		http.Error(w, "id de pedido invalido", http.StatusBadRequest)
		return
	}

	pedido, err := buscarPedidoPorID(r.Context(), id)
	// ...
}
```

### 4. Coluna que aceita `NULL` com `NullUUID`

```go
type Pedido struct {
	ID           uuid.UUID
	IDCancelador uuid.NullUUID // referencia opcional a outro usuario
	Cliente      string
}

func carregarPedido(db *sql.DB, id uuid.UUID) (Pedido, error) {
	var p Pedido
	err := db.QueryRow(
		"SELECT id, id_cancelador, cliente FROM pedidos WHERE id = $1", id.String(),
	).Scan(&p.ID, &p.IDCancelador, &p.Cliente)
	return p, err
}

func marcarCancelado(db *sql.DB, pedido, quemCancelou uuid.UUID) error {
	_, err := db.Exec(
		"UPDATE pedidos SET id_cancelador = $1 WHERE id = $2",
		uuid.NullUUID{UUID: quemCancelou, Valid: true}, pedido.String(),
	)
	return err
}
```

Para coluna binária de 16 bytes (`BINARY(16)`/`BLOB`, útil em MySQL,
MariaDB e SQLite onde não existe tipo `uuid` nativo), converta no ponto
da consulta em vez de mudar o tipo do campo:

```go
_, err := db.Exec(
	"INSERT INTO pedidos (id, id_cancelador) VALUES (?, ?)",
	uuid.BinaryUUID(pedido.ID), uuid.NullBinaryUUID{UUID: quemCancelou, Valid: true},
)
```

### 5. UUIDv4 aleatório (sem informação de tempo)

```go
func gerarChaveDeIdempotencia() string {
	return uuid.GenerateV4().String()
}
```

### 6. UUIDv5 determinístico a partir de um nome

Útil para derivar um identificador estável de algo que já existe (uma
URL, um caminho de arquivo, um identificador externo) sem consultar
banco nenhum: a mesma entrada sempre produz o mesmo UUID.

```go
func idDeRecursoExterno(url string) uuid.UUID {
	return uuid.GenerateV5(uuid.NameSpaceURL, []byte(url))
}
```

### 7. Gerador próprio com entropia criptográfica

Use quando o identificador não pode ser previsível — por exemplo, um
link de confirmação de e-mail com validade curta (mesmo assim, prefira
não tratar o UUID como segredo de longa duração; ele expõe o instante de
criação).

```go
var GenSeguro = uuid.NewCryptoGenerator() // crypto/rand em toda geração

func gerarLinkDeConfirmacao() string {
	return GenSeguro.Generate(uuid.Level1).String()
}
```

### 8. Migrando código de `github.com/google/uuid`

Troque o import; a maioria das chamadas continua compilando sem outra
mudança, porque `compat.go` reproduz os nomes:

```go
// antes: import "github.com/google/uuid"
import uuid "github.com/patrickbrandao/go-loghub-uuid"

id := uuid.New()               // continua gerando v4, agora com o mesmo nome
s := uuid.NewString()          // v4 em texto
id, err := uuid.NewRandom()    // v4, erro só possível em teoria (crypto/rand)
id, err = uuid.NewUUID()       // v1
id, err = uuid.NewV7()         // v7, nivel 1, com entropia criptografica (mais lento que uuid.GenerateV7())
```

Pontos que exigem atenção manual — não são apenas renomeações — estão
detalhados em `docs/12-migracao-google-uuid.md` da biblioteca: em
especial, `Version()`/`Variant()` agora devolvem `byte` em vez de tipo
próprio; `Time()` virou `Timestamp() (time.Time, bool)`; e o
comportamento de `Scan` com valor ausente sobre um destino que já tinha
valor diverge do pacote original (aqui o `UUID` é zerado; lá, mantido).
Não presuma paridade total sem checar esse guia quando o código migrado
depender de algum desses detalhes.

## Armadilhas comuns (revise antes de aprovar um PR)

- **Nível de UUIDv7 misturado na mesma coluna.** Gerar parte das linhas
  com `Level1` e parte com `Level2`/`Level3` não quebra a inserção, mas
  silenciosamente quebra `MinAt`/`MaxAt`/`RangeAt`: a fronteira de um
  nível não delimita corretamente identificadores de outro nível.
  Fixe o nível quando desenhar a tabela.
- **`Generate`, `GenerateV4`, `GenerateV8`... nunca devolvem erro.**
  Nenhum gerador desta biblioteca falha (falha na fonte de entropia vira
  pânico, não erro devolvido). Só funções de *parsing* (`Parse`,
  `FromString`, `Import`...) e os apelidos de compatibilidade que leem
  `crypto/rand` diretamente (`NewRandom`, `NewV7`) devolvem erro.
- **`GenerateAt` é gerador, não é determinístico.** Duas chamadas com o
  mesmo instante devolvem UUIDs *diferentes* (só os campos de tempo
  coincidem). Se você precisa do valor determinístico de um instante —
  por exemplo, para montar os limites de uma consulta — use `MinAt`
  ou `MaxAt`, não `GenerateAt`.
- **`GenerateAt`/`MinAt`/`MaxAt`/`RangeAt` só existem para UUIDv7.** Não
  há equivalente para v1/v2/v6: essas versões garantem unicidade por um
  piso de relógio interno que um instante arbitrário do chamador
  quebraria.
- **`FromString` só aceita a forma canônica com hífens.** Se a entrada
  pode vir sem hífens, entre chaves, ou com prefixo `urn:uuid:`, use
  `Parse`, não `FromString`.
- **`NullUUID`/`Scan` de valor ausente zera o receptor** (`Nil`,
  `Valid=false`), diferente de `github.com/google/uuid`, que deixa o
  destino intocado. Em um `Scan` que reaproveita a mesma variável entre
  linhas, isso é a diferença entre "sobrou o valor da linha anterior" e
  "ausência correta" — a implementação desta biblioteca é a que evita o
  bug, mas se o código migrado dependia do comportamento antigo, revise.
- **Não escreva teste de ordenação em laço apertado contando
  "regressões".** Gerar UUIDs custa dezenas de nanossegundos, menos que
  o passo do relógio da maioria dos hosts, então empates no mesmo
  milissegundo (desempatados por bits aleatórios, não por contador) são
  o caso normal em `Level1`, não uma falha. Teste fazendo o instante
  avançar de verdade entre gerações (ex.: com `time.Sleep`).
- **Coluna binária de 16 bytes é opt-in por tipo, não por configuração.**
  `Value()` de `UUID` sempre grava texto; para gravar binário use o tipo
  `BinaryUUID` explicitamente na consulta. Não existe uma flag global
  para trocar o formato — isso é proposital, para não misturar dois
  formatos na mesma coluna em silêncio.

## Onde aprofundar

- `docs/INDEX.md` no repositório da biblioteca — mapa completo da
  especificação (layout de bits do UUIDv7, as demais versões, relógio e
  concorrência, parsing, serialização, casos de teste obrigatórios,
  decisões de projeto firmadas).
- `docs/03-guia-completo.md` — o mesmo conteúdo prático desta skill, em
  formato de guia de referência mais longo, com mais contexto sobre cada
  decisão.
- `docs/12-migracao-google-uuid.md` — guia de migração completo a partir
  de `github.com/google/uuid`.
- `example_test.go` na raiz do repositório — exemplos executáveis
  (`go test ./ -run Example -v`) que servem de referência de estilo.
