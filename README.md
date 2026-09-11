# go-loghub-uuid

[![ci](https://github.com/patrickbrandao/go-loghub-uuid/actions/workflows/ci.yml/badge.svg)](https://github.com/patrickbrandao/go-loghub-uuid/actions/workflows/ci.yml)

Biblioteca Go leve e rápida para gerar **UUIDv7** (RFC 9562) com **três
níveis de precisão temporal**, conversões string⇄binário e importação
das propriedades de tempo. Gera também **todas as demais versões de
UUID** da RFC 9562 — 1, 2, 3, 4, 5, 6 e 8.

- **Rápida**: geração binária em dezenas de nanossegundos, **zero
  alocações**, mais de **11 mil UUIDs/ms** por núcleo (em VM modesta).
- **Concorrente**: um único `Generator` criado no boot atende centenas
  de goroutines sem trava global.
- **Sem dependências**: apenas a biblioteca padrão do Go.
- **Multinível**: do milissegundo padrão até nanossegundos embutidos.
- **Completa**: todas as versões da RFC 9562, análise permissiva de
  texto, serialização em JSON e integração com `database/sql`.

## Níveis

| Nível    | Precisão embutida           | Compatível UUIDv7 |
|----------|-----------------------------|:-----------------:|
| `Level1` | milissegundos (padrão)      | Sim               |
| `Level2` | + microssegundos em rand_a  | Sim               |
| `Level3` | + nanossegundos em rand_b   | Sim               |

Todos preservam versão 7 e variante RFC.

## Todas as versões de UUID

| Função                  | Versão | Base                                    |
|-------------------------|:------:|-----------------------------------------|
| `GenerateV1`            | 1      | tempo gregoriano + nó + sequência       |
| `GenerateV2`            | 2      | versão 1 com domínio e identificador    |
| `GenerateV3`            | 3      | resumo MD5 de espaço de nomes + nome    |
| `GenerateV4`            | 4      | 122 bits aleatórios                     |
| `GenerateV5`            | 5      | resumo SHA-1 de espaço de nomes + nome  |
| `GenerateV6`            | 6      | versão 1 com tempo reordenado, ordenável|
| `Generate(Level1..3)`   | 7      | tempo Unix, com os três níveis          |
| `GenerateV8`            | 8      | 122 bits livres, definidos pelo chamador|

As versões 1, 2 e 6 compartilham um relógio interno com trava própria. O
caminho do UUIDv7 continua sem trava alguma e sem alocações.

## Instalação

```bash
go get github.com/patrickbrandao/go-loghub-uuid
```

## Uso rápido no Linux

Instalar Go (é necessário **Go 1.22 ou superior**; confira com
`go version` — se a distribuição empacotar versão inferior, use o
instalador oficial):
```bash
apt-get update;
apt-get install -y golang-go;
```

Arquivo go.mod:
```go
module uuid-test

go 1.22

require github.com/patrickbrandao/go-loghub-uuid v0.3.0
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
go mod download;
go build -o test-uuid test-uuid.go;
```

Compilar (alternativa):
```bash
go get github.com/patrickbrandao/go-loghub-uuid@latest;
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

O gerador padrão (`NewGenerator`, e as funções de pacote `Generate`,
`GenerateString`, `GenerateV4` e `GenerateV8Random`) usa um PRNG
**estatístico** (PCG), não criptográfico. Os apelidos de compatibilidade
`New`, `NewString`, `NewRandom` e `NewV7` leem de `crypto/rand`.
Quem observar alguns identificadores consegue reconstruir o estado
interno e prever os seguintes; além disso, todo UUIDv7 expõe o instante
de criação por construção.

**Não use estes UUIDs como segredo** — token de sessão, link privado,
chave de recuperação ou senha de uso único. Para identificadores que
precisem ser inadivinháveis, monte o gerador com entropia criptográfica:

```go
var Gen = uuid.NewCryptoGenerator()
```

`NewGeneratorWithReader` aceita qualquer `io.Reader` seguro para uso
concorrente, e `NewGeneratorWith` continua aceitando uma função que
devolve 64 bits.

Como identificador de registro, chave primária ou correlação de log — o
uso a que a biblioteca se destina — o gerador padrão é adequado.

## Mais

- **Mapa completo do projeto**: [STARTHERE.md](STARTHERE.md)
- **Histórico de mudanças**: [CHANGELOG.md](CHANGELOG.md)
- Vindo do pacote `github.com/google/uuid`:
  [docs/MIGRATION.md](docs/MIGRATION.md)
- Uso rápido: [docs/DEPLOY-FAST.md](docs/DEPLOY-FAST.md)
- Uso completo (todas as funções): [docs/DEPLOY-FULL.md](docs/DEPLOY-FULL.md)
- Testes e benchmark: [docs/TEST-AND-BENCHMARK.md](docs/TEST-AND-BENCHMARK.md)
- Especificação de desenvolvimento (agnóstica de linguagem):
  [docs/SPEC.md](docs/SPEC.md)

## Licença

[MIT](LICENSE).
