# Uso Rápido

Como incluir a biblioteca em um projeto Go e gerar um UUIDv7 em string
em poucos segundos.

## 1. Instalar

No diretório do seu projeto (que já tem um `go.mod`):

```bash
go get github.com/patrickbrandao/go-loghub-uuid
```

## 2. Gerar um UUIDv7 em string

```go
package main

import (
	"fmt"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

func main() {
	// Forma mais curta: funções de pacote usam um gerador padrão interno,
	// já pronto e seguro para concorrência.
	s1 := uuid.GenerateString(uuid.Level1) // só milissegundos   (UUIDv7 padrão)
	s2 := uuid.GenerateString(uuid.Level2) // ate microssegundos (UUIDv7 + rand_a)
	s3 := uuid.GenerateString(uuid.Level3) // ate nanosegundos   (UUIDv7 ++ rand_a)
	fmt.Println(s1)                        // ex.: 019e99e3-42f0-7882-9719-2305ff84949c
	fmt.Println(s2)                        // ex.: 019e99e3-42f0-7882-9719-2305ff84949c
	fmt.Println(s3)                        // ex.: 019e99e3-42f0-7882-9719-2305ff84949c
}
```

Pronto. É isso para o caso mais comum.

## 3. Escolher o nível de precisão

```go
uuid.GenerateString(uuid.Level1) // milissegundos
uuid.GenerateString(uuid.Level2) // + microssegundos embutidos
uuid.GenerateString(uuid.Level3) // + microssegundos e nanossegundos embutidos
```

Todos os níveis produzem UUIDv7 válidos (versão 7, variante RFC).

## 4. Se você gera muito (recomendado em serviços)

Crie **um** gerador no boot e reutilize-o em todas as goroutines:

```go
var Gen = uuid.NewGenerator() // crie uma vez, no início do programa

func handler() string {
	return Gen.GenerateString(uuid.Level3)
}
```

O mesmo `*Generator` pode ser chamado por centenas de goroutines ao mesmo
tempo, sem trava global.

---

Para todas as funções (binário, conversões, importação de tempo), veja
[DEPLOY-FULL.md](DEPLOY-FULL.md).

