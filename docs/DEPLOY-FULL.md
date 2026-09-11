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

Internamente mantém um pool de PRNGs PCG (um por thread em uso), cada um
semeado de `crypto/rand`. Sem contenção de lock; ideal para alto volume.

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

> **Quando isto deixa de ser opcional.** O gerador padrão usa PCG, um
> PRNG estatístico e **não** criptográfico: a partir de poucas amostras
> observadas é possível reconstruir o estado interno e prever os UUIDs
> seguintes. Somado a isso, todo UUIDv7 revela o instante de criação por
> construção. Se o identificador precisar ser inadivinhável — token de
> sessão, link privado, chave de recuperação — o gerador com entropia
> criptográfica acima é **requisito**, não conveniência. Para chave
> primária, identificador de registro e correlação de log, o gerador
> padrão é adequado.

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
a.Compare(b)        // -1, 0 ou 1; para igualdade basta a == b
u.URN()             // urn:uuid:0192f7c5-...

lista := uuid.UUIDs{a, b, c}
lista.Strings()     // []string com as formas canônicas
```

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
fiel do relógio sob carga sustentada.

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
