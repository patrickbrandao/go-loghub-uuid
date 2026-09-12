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
	s1 := uuid.GenerateString(uuid.Level1) // só milissegundos (UUIDv7 padrão)
	s2 := uuid.GenerateString(uuid.Level2) // até microssegundos (UUIDv7 + rand_a)
	s3 := uuid.GenerateString(uuid.Level3) // até nanossegundos (UUIDv7 + rand_a + rand_b)
	fmt.Println(s1)                        // ex.: 019e99e3-42f0-7882-9719-2305ff84949c
	fmt.Println(s2)                        // ex.: 019e99e3-42f0-71a2-9719-2305ff84949c
	fmt.Println(s3)                        // ex.: 019e99e3-42f0-71a2-9719-81a2ff84949c
}
```

Pronto. É isso para o caso mais comum.

## 3. Escolher o nível de precisão

```go
uuid.GenerateString(uuid.Level1) // milissegundos
uuid.GenerateString(uuid.Level2) // + microssegundos embutidos
uuid.GenerateString(uuid.Level3) // + microssegundos e nanossegundos embutidos
```

Todos os níveis produzem UUIDv7 válidos (versão 7, variante RFC). Os
três também existem pelo nome, sem o argumento de nível, devolvendo o
binário:

```go
uuid.GenerateV7()       // o UUIDv7 padrão da RFC: o mesmo que Generate(uuid.Level1)
uuid.GenerateV7Level1() // o mesmo que GenerateV7
uuid.GenerateV7Level2() // o mesmo que Generate(uuid.Level2)
uuid.GenerateV7Level3() // o mesmo que Generate(uuid.Level3)
```

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

## 5. Consultar por intervalo de tempo

A chave já está em ordem cronológica, então a janela de tempo vira
varredura de faixa no índice primário, sem coluna de carimbo:

```go
lo, hi := uuid.RangeAt(uuid.Level3, inicio, fim) // intervalo [inicio, fim)

rows, err := db.Query(
	"SELECT id, corpo FROM eventos WHERE id >= $1 AND id < $2 ORDER BY id",
	lo.String(), hi.String(),
)
```

Use **o mesmo nível** com que os identificadores foram gravados. Níveis
misturados na mesma coluna fazem a consulta devolver linhas a menos, sem
erro nenhum.

## 6. Gerar para um instante que você já tem

Ao importar registros antigos, gerar a chave com o instante original
mantém o índice em ordem cronológica, como se tivessem sido gravados na
época:

```go
id := uuid.GenerateAtString(uuid.Level2, registro.CriadoEm)
```

Com `Generate`, todos os registros importados receberiam o carimbo do
momento da importação.

---

Para todas as funções (binário, conversões, importação de tempo, coluna
binária de 16 bytes), veja [DEPLOY-FULL.md](DEPLOY-FULL.md).
