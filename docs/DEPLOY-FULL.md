# Uso Completo

Referência de todas as funções da biblioteca `go-loghub-uuid`, com
exemplos.

## Instalação

```bash
go get github.com/patrickbrandao/go-loghub-uuid
```

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

```go
type Time struct {
	Seconds      int64 // timestamp Unix (segundos)
	Milliseconds int   // 0..999
	Microseconds int   // 0..999
	Nanoseconds  int   // 0..999
}
```

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
	crand.Read(b[:])
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
u.Version() // 7 para UUIDs gerados aqui
u.Variant() // 2 (binário 10) — variante RFC
```

---

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
