# go-loghub-uuid

Biblioteca Go leve e rápida para gerar **UUIDv7** (RFC 9562) com **três
níveis de precisão temporal**, conversões string⇄binário e importação
das propriedades de tempo.

- **Rápida**: geração binária em dezenas de nanossegundos, **zero
  alocações**, mais de **11 mil UUIDs/ms** por núcleo (em VM modesta).
- **Concorrente**: um único `Generator` criado no boot atende centenas
  de goroutines sem trava global.
- **Sem dependências**: apenas a biblioteca padrão do Go.
- **Multinível**: do milissegundo padrão até nanossegundos embutidos.

## Níveis

| Nível    | Precisão embutida           | Compatível UUIDv7 |
|----------|-----------------------------|:-----------------:|
| `Level1` | milissegundos (padrão)      | Sim               |
| `Level2` | + microssegundos em rand_a  | Sim               |
| `Level3` | + nanossegundos em rand_b   | Sim               |

Todos preservam versão 7 e variante RFC.

## Instalação

```bash
go get github.com/patrickbrandao/go-loghub-uuid
```

## Uso rápido

```go
package main

import (
	"fmt"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

func main() {
	// Gera um UUIDv7 em string (gerador padrão interno, pronto para uso).
	fmt.Println(uuid.GenerateString(uuid.Level1))
	// ex.: 019e99e3-42f0-7882-9719-2305ff84949c
}
```

Em serviços de alto volume, crie **um** gerador no boot e reutilize:

```go
var Gen = uuid.NewGenerator()

func newID() string { return Gen.GenerateString(uuid.Level3) }
```

## Mais

- **Mapa completo do projeto**: [STARTHERE.md](STARTHERE.md)
- Uso rápido: [docs/DEPLOY-FAST.md](docs/DEPLOY-FAST.md)
- Uso completo (todas as funções): [docs/DEPLOY-FULL.md](docs/DEPLOY-FULL.md)
- Testes e benchmark: [docs/TEST-AND-BENCHMARK.md](docs/TEST-AND-BENCHMARK.md)
- Especificação de desenvolvimento (agnóstica de linguagem):
  [especificacao/ESPECIFICACAO-DESENVOLVIMENTO.md](especificacao/ESPECIFICACAO-DESENVOLVIMENTO.md)

## Licença

[MIT](LICENSE).
