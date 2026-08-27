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

## Uso rápido no Linux

Instalar Go:
```bash
apt-get update;
apt-get install -y golang-go;
```

Arquivo go.mod:
```go
module uuid-test

go 1.22

require github.com/patrickbrandao/go-loghub-uuid v0.2.0
```

> O `require` acima é preenchido automaticamente pelo `go get` mostrado
> na seção de compilação; declará-lo à mão é opcional.

Arquivo test-uuid.go:
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

Compilar:
```bash
#go mod download;
GOSUMDB=off go mod download;
#go env -w GOPROXY=direct && go mod download;
#go build .;
go build -o test-uuid test-uuid.go;
```

Compilar (alternativa):
```bash
go env -w GOSUMDB=off;
go get github.com/patrickbrandao/go-loghub-uuid;
go mod tidy;
go build -o test-uuid test-uuid.go;
```

Compilar (multi plataforma):
```
# Windows
GOOS=windows GOARCH=amd64 go build -o test-uuid.exe test-uuid.go

# macOS
GOOS=darwin GOARCH=amd64 go build -o test-uuid test-uuid.go

# Linux (outros processadores)
GOOS=linux GOARCH=arm64 go build -o test-uuid test-uuid.go
```

Rodar:
```bash
./test-uuid;
    # 019e9ace-a992-79d2-9460-e33944a68428
```

Em serviços de alto volume, crie **um** gerador no boot e reutilize:

```go
var Gen = uuid.NewGenerator()

func newID() string { return Gen.GenerateString(uuid.Level3) }
```

## Aviso de segurança

O gerador padrão (`NewGenerator`, e as funções de pacote `Generate` /
`GenerateString`) usa um PRNG **estatístico** (PCG), não criptográfico.
Quem observar alguns identificadores consegue reconstruir o estado
interno e prever os seguintes; além disso, todo UUIDv7 expõe o instante
de criação por construção.

**Não use estes UUIDs como segredo** — token de sessão, link privado,
chave de recuperação ou senha de uso único. Para identificadores que
precisem ser inadivinháveis, monte o gerador com entropia criptográfica:

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

var Gen = uuid.NewGeneratorWith(cryptoBits)
```

Como identificador de registro, chave primária ou correlação de log — o
uso a que a biblioteca se destina — o gerador padrão é adequado.

## Mais

- **Mapa completo do projeto**: [STARTHERE.md](STARTHERE.md)
- Uso rápido: [docs/DEPLOY-FAST.md](docs/DEPLOY-FAST.md)
- Uso completo (todas as funções): [docs/DEPLOY-FULL.md](docs/DEPLOY-FULL.md)
- Testes e benchmark: [docs/TEST-AND-BENCHMARK.md](docs/TEST-AND-BENCHMARK.md)
- Especificação de desenvolvimento (agnóstica de linguagem):
  [docs/SPEC.md](docs/SPEC.md)

## Licença

[MIT](LICENSE).
