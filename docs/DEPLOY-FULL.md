# Uso Completo

Referência de todas as funções da biblioteca `go-loghub-uuid`, com
exemplos.

## Instalação

```bash
go get github.com/patrickbrandao/go-loghub-uuid
```

> É necessário **Go 1.22 ou superior** (confira com `go version`).

```go
import uuid "github.com/patrickbrandao/go-loghub-uuid"
```

> O nome do pacote declarado no código é `loghubuuid`; o apelido de
> import `uuid` usado aqui é só uma conveniência.

---

## Tipos

| Tipo        | Descrição                                                     |
|-------------|---------------------------------------------------------------|
| `Level`     | Nível de precisão: `Level1`, `Level2`, `Level3`.              |
| `UUID`      | `[16]byte` — o valor binário de 128 bits.                     |
| `Generator` | Objeto gerador, criado no boot, seguro para concorrência.     |
| `Time`      | Componentes de tempo importados (segundos/ms/us/ns).          |
| `Domain`    | Domínio da versão 2: `Person`, `Group`, `Org`.                |
| `GregorianTime` | Tempo das versões 1 e 6, em tiques de 100 ns desde 1582.  |
| `NullUUID`  | UUID que pode ser `NULL` no banco de dados.                   |
| `UUIDs`     | Lista de UUIDs, com `Strings()`.                              |

```go
type Time struct {
	Seconds      int64 // timestamp Unix (segundos)
	Milliseconds int   // 0..999
	Microseconds int   // 0..999 em Level2/Level3; 0..4095 em Level1 (12 bits aleatorios)
	Nanoseconds  int   // 0..999 em Level3; 0..1023 em Level1/Level2 (10 bits aleatorios)
}
```

> A importação é cega quanto ao nível (ver abaixo): em UUIDs de Nível 1 os
> campos `Microseconds` e `Nanoseconds` são bits aleatórios lidos como se
> fossem tempo, e por isso podem ultrapassar 999. Não valide esses campos
> contra 0..999 sem saber o nível de origem.

---

## Criar geradores

### Gerador padrão (rápido)

```go
g := uuid.NewGenerator()
```

A entropia vem do gerador do runtime do Go (`math/rand/v2`): uma
instância de ChaCha8 por thread, semeada pelo sistema operacional na
carga do programa. Sem contenção de lock; ideal para alto volume.

### Gerador com entropia personalizada

Para forçar, por exemplo, entropia **criptográfica** em toda geração:

```go
import (
	crand "crypto/rand"
	"encoding/binary"
)

func cryptoBits() uint64 {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		panic(err)
	}
	return binary.LittleEndian.Uint64(b[:])
}

g := uuid.NewGeneratorWith(cryptoBits)
```

A função fornecida deve devolver 64 bits aleatórios e **ser segura para
uso concorrente** (será chamada por várias goroutines). Ela é chamada uma
vez por UUID nos níveis 2 e 3 e duas vezes no nível 1. Passar `nil` faz
`NewGeneratorWith` entrar em pânico imediatamente, para que o erro de
configuração apareça no boot.

> **Quando isto deixa de ser opcional.** O gerador padrão usa o ChaCha8
> do runtime, que resiste a predição — bem mais forte que o PCG usado
> até a `v0.3.0` —, mas a própria documentação do Go recomenda
> `crypto/rand` para uso sensível a segurança, e a biblioteca não promete
> força criptográfica nessa fonte. Somado a isso, todo UUIDv7 revela o
> instante de criação por construção, qualquer que seja a entropia. Se o
> identificador precisar ser inadivinhável — token de sessão, link
> privado, chave de recuperação — o gerador com entropia criptográfica
> acima é **requisito**, não conveniência. Para chave primária,
> identificador de registro e correlação de log, o gerador padrão é
> adequado.

---

### Gerador com entropia criptográfica

Atalho para o caso mais comum de entropia forte:

```go
var Gen = uuid.NewCryptoGenerator()
```

### Gerador a partir de um `io.Reader`

Aceita qualquer fonte no formato da biblioteca padrão. O leitor **precisa
ser seguro para uso concorrente**, porque será chamado por várias
goroutines ao mesmo tempo:

```go
var Gen = uuid.NewGeneratorWithReader(crand.Reader)
```

Se uma leitura falhar durante a geração, a chamada entra em pânico: uma
fonte de entropia quebrada não pode degradar em silêncio para um gerador
previsível.

## Gerar

### Binário (128 bits)

```go
u := g.Generate(uuid.Level3) // u é um uuid.UUID ([16]byte)
```

### String canônica

```go
s := g.GenerateString(uuid.Level3) // ex.: 019e99e3-7471-71c2-8e43-f955d7ea2ec6
```

### Atalhos de pacote (gerador padrão interno)

```go
u := uuid.Generate(uuid.Level2)
s := uuid.GenerateString(uuid.Level1)
```

---

## Converter

### Binário → string

```go
s := u.String()              // método
s := uuid.BinaryToString(u)  // função equivalente
```

### Binário → texto sem alocar

`String` aloca a string devolvida a cada chamada. Quando são milhões de
identificadores por segundo — uma linha de log, um corpo JSON montado à
mão —, `AppendTo` escreve os mesmos 36 bytes no buffer do chamador e não
aloca nada enquanto houver capacidade:

```go
buf := make([]byte, 0, 64)
for _, u := range lista {
	buf = u.AppendTo(buf[:0])
	escreva(buf)
}
```

Medido neste repositório (Apple M2, Go 1.27): `AppendTo` custa ~18,8 ns
e zero alocações, contra ~26 ns e uma alocação de 48 bytes de `String`.
Passar `nil` como `dst` é válido e aloca os 36 bytes.

`AppendText` é o mesmo método com a assinatura de
`encoding.TextAppender` (Go 1.24), devolvendo um erro sempre nulo.

### String → binário

```go
u, err := uuid.FromString(s) // método de fábrica
// equivalente: u, err := uuid.StringToBinary(s)
if err != nil {
	// uuid.ErrInvalidFormat se a string não for canônica
}
```

`FromString` aceita maiúsculas ou minúsculas e exige o formato canônico
`8-4-4-4-12`.

---

## Importar propriedades de tempo

A importação é **cega quanto ao nível**: lê sempre os mesmos campos e
trata os bits como tempo preciso. Para UUIDs de Nível 1, micro/nano
serão aleatórios (esperado).

### A partir de string

```go
t, err := uuid.Import(s)
if err != nil {
	// formato inválido
}
fmt.Println(t.Seconds, t.Milliseconds, t.Microseconds, t.Nanoseconds)
```

### A partir de binário

```go
t := uuid.ImportBinary(u)
```

### Reconstruir um `time.Time` (Nível 3)

```go
import "time"

t := uuid.ImportBinary(u)
instant := time.Unix(
	t.Seconds,
	int64(t.Milliseconds)*1_000_000+
		int64(t.Microseconds)*1_000+
		int64(t.Nanoseconds),
)
```

---

## Gerar a partir de um instante conhecido

`Generate` lê o relógio. `GenerateAt` recebe o instante, e é o sentido
inverso de `Import`: ali se lê o tempo de um identificador, aqui se
constrói um identificador para um tempo.

```go
import "time"

quando := time.Date(2019, 3, 14, 10, 0, 0, 0, time.UTC)

u := uuid.GenerateAt(uuid.Level3, quando)       // binário
s := uuid.GenerateAtString(uuid.Level3, quando) // string canônica
```

Também existe como método, para a entropia configurada continuar
valendo:

```go
gen := uuid.NewCryptoGenerator()
u := gen.GenerateAt(uuid.Level3, quando)
```

### É gerador, não construtor determinístico

**Duas chamadas com o mesmo instante devolvem UUIDs diferentes.** Os
campos de tempo são iguais, os bits livres são sorteados:

```go
a := uuid.GenerateAt(uuid.Level3, quando)
b := uuid.GenerateAt(uuid.Level3, quando)
// a != b, mas uuid.ImportBinary(a) == uuid.ImportBinary(b) nos campos de tempo
```

Isso é proposital. Um gerador que devolvesse sempre o mesmo valor para o
mesmo instante colidiria na primeira repetição. Se o que você quer é o
valor determinístico de um instante, use `MinAt` ou `MaxAt`, da seção
seguinte.

A unicidade vem inteiramente dos bits livres: 74 no Nível 1, 62 no Nível
2 e 52 no Nível 3. Gerando pelo relógio isso nunca é uma escolha sua,
porque o instante avança. Aqui o instante é seu, então vale saber que
gerar em volume para um **único** instante é o caso em que essa margem
importa.

### Reprocessar um histórico preservando a ordem

É o caso de uso principal. Ao importar registros antigos, gerar a chave
com o instante original mantém o índice primário em ordem cronológica,
como se os registros tivessem sido gravados na época:

```go
type Antigo struct {
	CriadoEm time.Time
	Corpo    string
}

gen := uuid.NewGenerator()

for _, registro := range historico {
	id := gen.GenerateAtString(uuid.Level2, registro.CriadoEm)
	_, err := db.Exec(
		"INSERT INTO eventos (id, corpo) VALUES ($1, $2)",
		id, registro.Corpo,
	)
	if err != nil {
		return err
	}
}
```

Depois disso a consulta por intervalo da seção seguinte funciona sobre
os registros importados, porque a chave carrega o instante de origem.
Se em vez disso você gerasse com `Generate`, todos os registros antigos
receberiam o carimbo do momento da importação e a ordenação da chave
passaria a refletir a ordem de importação, não a do histórico.

Use o **mesmo nível** do resto da tabela. Níveis misturados quebram a
consulta por intervalo, pelo motivo detalhado na seção seguinte.

### Bordas

- Instante anterior a `1970-01-01T00:00:00Z`: degrada para a própria
  época, exatamente como `Generate` faz com um relógio atrasado.
- Instante posterior a `10889-08-02T05:31:50.655999999Z`: satura no
  último instante representável, porque o campo de milissegundos tem 48
  bits.
- Nível desconhecido: tratado como Nível 1, como em `Generate`.

Nenhuma dessas situações devolve erro. Nenhum gerador desta biblioteca
devolve erro, e `GenerateAt` não abre exceção.

**Só existe para o UUIDv7.** As versões 1, 2 e 6 usam a época
gregoriana e garantem unicidade por um piso de relógio interno; aceitar
um instante arbitrário fura essa garantia e permitiria reemitir um
UUIDv1 já produzido. Para essas versões, gere pelo relógio.

---

## Consultar por intervalo de tempo

Este é o motivo prático de adotar UUIDv7 como chave primária: o próprio
índice da chave já está em ordem cronológica, então uma janela de tempo
vira uma varredura de faixa, **sem coluna nem índice de carimbo
temporal**.

O que falta para montar a consulta são os dois identificadores que
delimitam a janela. `MinAt` e `MaxAt` devolvem, respectivamente, o menor
e o maior UUIDv7 que a biblioteca poderia gerar em um instante, e
`RangeAt` devolve o par de um intervalo semiaberto `[from, to)`.

```go
import "time"

fim := time.Now()
inicio := fim.Add(-24 * time.Hour)

lo, hi := uuid.RangeAt(uuid.Level2, inicio, fim)

rows, err := db.Query(
	`SELECT id, mensagem FROM eventos
	  WHERE id >= $1 AND id < $2
	  ORDER BY id`,
	lo.String(), hi.String(),
)
```

O plano dessa consulta é uma varredura de faixa no índice primário. Não
há `WHERE criado_em BETWEEN ...`, não há índice secundário para manter e
a ordenação por `id` já sai cronológica.

Para um intervalo **fechado** nas duas pontas, use as fronteiras
diretamente:

```go
lo := uuid.MinAt(uuid.Level2, inicio)
hi := uuid.MaxAt(uuid.Level2, fim)

rows, err := db.Query(
	"SELECT id, mensagem FROM eventos WHERE id BETWEEN $1 AND $2 ORDER BY id",
	lo.String(), hi.String(),
)
```

### Nunca misture níveis na mesma coluna

**Este é o erro mais provável, e ele não dá mensagem nenhuma: devolve
linhas a menos.**

Os bits abaixo do milissegundo significam coisas diferentes em cada
nível. No Nível 1 os 74 bits abaixo do carimbo são livres; no Nível 2 os
12 bits de `rand_a` carregam os microssegundos exatos; no Nível 3
`rand_a` carrega os microssegundos e os 10 bits altos de `rand_b`
carregam os nanossegundos.

Uma fronteira calculada para um nível só delimita identificadores
gravados **naquele mesmo nível**. Uma fronteira superior de Nível 3, por
exemplo, fica abaixo de boa parte dos identificadores de Nível 1 do
mesmo instante, porque nela `rand_a` vale os microssegundos reais
(0 a 999) enquanto no Nível 1 ele é aleatório (0 a 4095).

Escolha o nível quando criar a tabela e não o mude. Se já houver dados
misturados, a consulta por faixa precisa usar a fronteira do nível mais
permissivo — Nível 1 — nas duas pontas, o que devolve linhas a mais e
exige filtro adicional.

### Precisão da fronteira

A fronteira é tão precisa quanto o nível:

| Nível    | A faixa delimita |
|----------|------------------|
| `Level1` | o milissegundo inteiro |
| `Level2` | o microssegundo |
| `Level3` | o nanossegundo |

No Nível 1, `RangeAt(Level1, inicio, fim)` exclui o milissegundo inteiro
de `fim`. Se a janela precisar terminar dentro daquele milissegundo, o
nível não tem resolução para isso.

### Bordas da faixa representável

O campo de milissegundos tem 48 bits, e as fronteiras saturam nas duas
pontas em vez de dar a volta:

- Instante anterior a `1970-01-01T00:00:00Z`: devolve a fronteira da
  própria época, como faz `Generate`.
- Instante posterior a `10889-08-02T05:31:50.655999999Z`: devolve a
  fronteira do último instante representável.

Saturar mantém as fronteiras monotônicas para qualquer entrada. Em
compensação, dois instantes distintos fora da faixa devolvem o mesmo
valor, então um intervalo inteiramente fora dela é vazio.

`RangeAt` não reordena os argumentos: passar `to` anterior a `from`
devolve um intervalo vazio, e é isso que a comparação vai refletir.

---

## Inspecionar

```go
u := uuid.Generate(uuid.Level3)

u.Version()  // 7
u.Variant()  // 2 (binário 10, variante RFC)

uuid.VersionString(u.Version())  // "versao 7"
uuid.VariantString(u.Variant())  // "RFC 9562"
```

Instante de criação, de forma independente da versão:

```go
instante, ok := u.Timestamp()  // versões 1, 6 e 7
```

Para recuperar também a precisão sub-milissegundo dos níveis 2 e 3, o
nível precisa ser informado, porque ele não pode ser deduzido do UUID:

```go
instante, ok := u.TimestampWithLevel(uuid.Level3)
```

Campos das versões baseadas em relógio, cada um com um segundo retorno
que é falso quando a versão não carrega aquele campo:

```go
sequencia, ok := u.ClockSequence()
no := u.NodeID()                  // nulo para as demais versões
dominio, ok := u.Domain()         // apenas versão 2
local, ok := u.ID()               // apenas versão 2
```

Valores especiais e comparação:

```go
uuid.Nil            // 00000000-0000-0000-0000-000000000000
uuid.Max            // ffffffff-ffff-ffff-ffff-ffffffffffff
u.IsZero()          // é o valor nulo?
u.IsMax()           // tem todos os bits em um?
u.IsValid()         // variante RFC e versão de 1 a 8?
u.Bytes()           // cópia dos 16 bytes
a.Compare(b)        // -1, 0 ou 1; para igualdade basta a == b
u.URN()             // urn:uuid:0192f7c5-...

lista := uuid.UUIDs{a, b, c}
lista.Strings()     // []string com as formas canônicas
```

`IsValid` aceita `Nil` e `Max`, que a RFC 9562 define como valores
especiais válidos apesar de não carregarem versão nem variante; use
`IsZero` e `IsMax` para distingui-los. Não é uma verificação de UUIDv7:
um UUIDv4 vindo de outro sistema também é válido. Para exigir a versão
7, compare `u.Version()` com `7`.

### Ordenar uma lista

`UUIDs` não implementa `sort.Interface`, e não vai implementar. A
biblioteca padrão já resolve isso melhor, com o `Compare` que esta
biblioteca oferece usado direto como função de comparação:

```go
import "slices"

lista := uuid.UUIDs{c, a, b}
slices.SortFunc(lista, uuid.UUID.Compare)
```

`uuid.UUID.Compare` aqui é uma expressão de método: vale
`func(uuid.UUID, uuid.UUID) int`, que é exatamente a assinatura que
`slices.SortFunc` espera. Uma linha, sem alocação, sem comparador
escrito à mão.

Para UUIDv7 e UUIDv6 essa ordem é cronológica, porque coincide com a
ordem dos bytes. Para ordenar pelo texto o resultado é o mesmo: a ordem
lexicográfica das strings canônicas acompanha a dos bytes.

`Bytes` devolve uma **cópia**; `u[:]` é mais barato e não copia, mas
aponta para o próprio valor. Use `Bytes` quando o destino guardar a
referência. Serve também para contornar uma armadilha de formatação:
como `UUID` satisfaz `fmt.Stringer`, `%x` sobre um `UUID` formata a
string canônica, não os bytes — `fmt.Printf("%x", u.Bytes())` imprime os
32 dígitos esperados.

## Gerar as outras versões de UUID

A biblioteca cobre todas as versões da RFC 9562. Nenhuma delas passa pelo
caminho do UUIDv7, que continua sem trava e sem alocações.

### Versão 4 — aleatória

```go
u := uuid.GenerateV4()          // gerador padrão do pacote
u = Gen.GenerateV4()            // ou a partir do seu Generator
```

### Versões 3 e 5 — derivadas de um nome

Determinísticas: o mesmo par de espaço de nomes e nome sempre devolve o
mesmo UUID, em qualquer máquina.

```go
u := uuid.GenerateV5(uuid.NameSpaceURL, []byte("https://exemplo.com.br"))
// sempre e66db8da-8762-5b81-afae-4f9f2f8a33dd
```

Espaços disponíveis: `NameSpaceDNS`, `NameSpaceURL`, `NameSpaceOID` e
`NameSpaceX500`. A versão 3 usa MD5 e existe por compatibilidade; para
esquemas novos prefira a versão 5, que usa SHA-1.

### Versões 1 e 6 — baseadas em relógio

Carregam tempo com resolução de 100 nanossegundos, uma sequência de
relógio de 14 bits e um identificador de nó de 48 bits:

```go
antiga  := uuid.GenerateV1()  // ordem cronológica NÃO acompanha o texto
ordenada := uuid.GenerateV6() // ordem cronológica acompanha o texto
```

O identificador de nó é sorteado uma vez e marcado como aleatório,
conforme a RFC 9562 seção 6.10 recomenda. Para usar um endereço MAC real:

```go
interfaces, _ := net.Interfaces()
for _, iface := range interfaces {
	if uuid.SetNodeID(iface.HardwareAddr) {
		break
	}
}
```

O relógio interno é estritamente crescente: em rajadas mais rápidas que
o tique de 100 nanossegundos ele avança sozinho, e o instante embutido
fica ligeiramente à frente do relógio do sistema. Isso garante a ordem,
mas significa que o carimbo de tempo de um UUIDv1 ou UUIDv6 não é leitura
fiel do relógio sob carga sustentada. O mesmo mecanismo cobre um relógio
do sistema atrasado por ajuste manual ou NTP: os instantes continuam
crescendo a partir do último emitido, adiantados em relação ao relógio
real, até ele os alcançar ou até uma ressincronização explícita.

Para descartar esse adiantamento e voltar a acompanhar o relógio do
sistema, sorteie uma sequência de relógio inédita:

```go
uuid.SetClockSequence(-1)
```

Cada sequência lembra o último instante que emitiu, então voltar a uma
sequência já usada (`uuid.SetClockSequence(valor)`) continua a partir do
ponto em que ela parou e nunca repete um UUID.

### Versão 2 — DCE Security

```go
u := uuid.GenerateV2(uuid.Org, 4242)
u = uuid.GenerateV2Person()  // usa o usuário do processo
u = uuid.GenerateV2Group()   // usa o grupo do processo
```

A versão 2 sacrifica os 32 bits baixos do tempo para guardar o
identificador local, então não carrega carimbo de tempo utilizável. Ela
identifica um sujeito/entidade (principal), e **não um evento individual**:
chamadas repetidas com o mesmo domínio e mesmo identificador dentro da
mesma janela de ~7 minutos devolvem exatamente o **mesmo UUID**. É
legado do DCE 1.1; não use em sistemas novos.

### Versão 8 — conteúdo livre

```go
var dados [16]byte
// preencha dados como quiser
u := uuid.GenerateV8(dados)   // só versão e variante são sobrescritas
u = uuid.GenerateV8Random()   // 122 bits aleatórios
```

## Ler UUIDs escritos em outros formatos

`FromString` aceita apenas a forma canônica. `Parse` aceita quatro
formas, todas com maiúsculas ou minúsculas:

```go
u, err := uuid.Parse("0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff")
u, err = uuid.Parse("{0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff}")
u, err = uuid.Parse("urn:uuid:0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff")
u, err = uuid.Parse("0192f7c51a2b7c3d8e4faabbccddeeff")
```

Complementos:

```go
u, err := uuid.ParseBytes(linha)   // o mesmo, a partir de []byte, sem alocar
u, err = uuid.FromBytes(brutos)    // 16 bytes crus vindos de coluna binária
u = uuid.MustParse(constante)      // entra em pânico; só para valores do código
err := uuid.Validate(entrada)      // apenas valida
```

Todos os erros continuam reconhecíveis pelo sentinela antigo:

```go
if errors.Is(err, uuid.ErrInvalidFormat) { ... }
if uuid.IsInvalidLengthError(err) { ... }  // caso específico de comprimento
```

## JSON, texto e binário

O tipo implementa as quatro interfaces de serialização da biblioteca
padrão, então um UUID viaja como string em JSON sem nenhum código extra:

```go
type Evento struct {
	ID uuid.UUID `json:"id"`
}

dados, _ := json.Marshal(Evento{ID: uuid.Generate(uuid.Level3)})
// {"id":"0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff"}
```

> **Atenção na migração.** Antes destes métodos existirem, o
> `encoding/json` gravava o UUID como lista de 16 números, por ser um
> vetor de bytes. Dados já gravados no formato antigo precisam ser
> convertidos. O mesmo vale para `encoding/gob`.

## Banco de dados

```go
type Registro struct {
	ID   uuid.UUID
	Pai  uuid.NullUUID  // coluna que aceita NULL
	Nome string
}

err := db.QueryRow("SELECT id, pai, nome FROM registros WHERE id = $1", chave).
	Scan(&r.ID, &r.Pai, &r.Nome)

_, err = db.Exec("INSERT INTO registros (id, nome) VALUES ($1, $2)", r.ID, r.Nome)
```

`Scan` aceita `NULL`, texto em qualquer formato reconhecido por `Parse` e
16 bytes crus. Texto vazio (string ou bytes) equivale a `NULL`: grava o
UUID nulo sem erro e, em `NullUUID`, deixa `Valid` falso. `Value` grava a
string canônica; para coluna binária, passe `r.ID[:]` explicitamente.

## Quando usar cada nível

| Nível    | Precisão embutida        | Aleatoriedade restante | Uso típico                                   |
|----------|--------------------------|------------------------|----------------------------------------------|
| `Level1` | milissegundos            | 74 bits                | UUIDv7 padrão; máxima compatibilidade        |
| `Level2` | + microssegundos         | 62 bits                | ordenação mais fina dentro do mesmo ms       |
| `Level3` | + micro e nanossegundos  | 52 bits                | logs/eventos de altíssima frequência         |

Quanto maior o nível, mais bits de tempo e menos bits aleatórios. Em
todos, a colisão é praticamente desprezível para volumes normais, mas se
sua aplicação depende criticamente de unicidade entre máquinas, combine
com um identificador de origem fora do UUID.

> **Resolução do relógio do host.** O Nível 3 só grava nanossegundos
> reais se `time.Now()` os fornecer. Em hosts cujo relógio tem resolução
> de microssegundo, como o macOS, o campo de nanossegundos sai sempre
> zero: são dez bits de aleatoriedade trocados por nada, sem ganho de
> ordenação em relação ao Nível 2. `TestTieRateReport` informa quantos
> instantes distintos o relógio do host oferece; use-o para escolher o
> nível.

> **Relógio do sistema atrasado.** O UUIDv7 lê o relógio de parede a cada
> geração. Se ele for atrasado (ajuste manual ou salto de NTP), os UUIDs
> seguintes ficam lexicograficamente antes dos anteriores até o relógio
> alcançar o instante antigo; a RFC 9562 permite esse comportamento e a
> biblioteca não tenta compensá-lo. As versões 1 e 6 têm comportamento
> distinto: como o relógio interno nunca regride, elas continuam
> emitindo instantes crescentes a partir do último valor, adiantadas em
> relação ao relógio real, até ele as alcançar. Veja a seção sobre as
> versões 1 e 6.

---

## Exemplo completo

```go
package main

import (
	"fmt"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

var Gen = uuid.NewGenerator()

func main() {
	// gerar
	u := Gen.Generate(uuid.Level3)
	s := u.String()
	fmt.Println("uuid:", s)

	// converter ida e volta
	v, _ := uuid.FromString(s)
	fmt.Println("igual:", v == u)

	// importar tempo
	t := uuid.ImportBinary(u)
	fmt.Printf("seg=%d ms=%03d us=%03d ns=%03d\n",
		t.Seconds, t.Milliseconds, t.Microseconds, t.Nanoseconds)
}
```
