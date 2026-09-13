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
- **Consultável por intervalo**: `MinAt`, `MaxAt` e `RangeAt` dão os
  UUIDs que delimitam uma janela de tempo, para responder a ela com o
  índice da própria chave primária, sem coluna nem índice de carimbo.
- **Completa**: todas as versões da RFC 9562, análise permissiva de
  texto, serialização em JSON e integração com `database/sql`, inclusive
  em coluna binária de 16 bytes.

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
| `GenerateV7`            | 7      | tempo Unix em milissegundos (Nível 1)   |
| `GenerateV7Level1..3`   | 7      | os três níveis pelo nome, sem argumento |
| `Generate(Level1..3)`   | 7      | tempo Unix, com os três níveis          |
| `GenerateV8`            | 8      | 122 bits livres, definidos pelo chamador|

As versões 1, 2 e 6 compartilham um relógio interno com trava própria. O
caminho do UUIDv7 continua sem trava alguma e sem alocações.

`GenerateV7` é o UUIDv7 padrão da RFC 9562 pedido pelo nome da versão,
como as demais: exatamente `Generate(Level1)`, com precisão de
milissegundo, também como método do `Generator`. Os três níveis também
têm nome próprio, sem o argumento de nível: `GenerateV7Level1` (o mesmo
que `GenerateV7`), `GenerateV7Level2` e `GenerateV7Level3` são
exatamente `Generate` com o nível correspondente, e deixam o nível
legível no ponto da chamada.

## Consulta por intervalo de tempo

É o motivo prático de usar UUIDv7 como chave primária. O índice da chave
já está em ordem cronológica, então uma janela de tempo vira varredura de
faixa:

```go
lo, hi := uuid.RangeAt(uuid.Level2, inicio, fim)

rows, err := db.Query(
	"SELECT id, corpo FROM eventos WHERE id >= $1 AND id < $2 ORDER BY id",
	lo.String(), hi.String(),
)
```

`RangeAt` devolve o intervalo semiaberto `[inicio, fim)`. Para as
fronteiras separadas, `MinAt` e `MaxAt`. As três respeitam o nível: no
Nível 2 o campo `rand_a` carrega os microssegundos reais do instante, e
zerá-lo daria uma fronteira errada.

Para gravar a chave de um instante conhecido, ao reprocessar um
histórico, `GenerateAt(nível, instante)` gera com o tempo que você
informa, preservando a ordenação da chave.

> A fronteira só vale para UUIDs gravados no **mesmo nível**. Misturar
> níveis na mesma coluna faz a consulta devolver linhas a menos, sem erro
> nenhum. Detalhes em [docs/DEPLOY-FULL.md](docs/DEPLOY-FULL.md).

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

require github.com/patrickbrandao/go-loghub-uuid v0.6.0
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
`GenerateString`, `GenerateV7`, `GenerateV7Level1` a `GenerateV7Level3`,
`GenerateV4` e `GenerateV8Random`) tira entropia do gerador do runtime do
Go, uma instância de **ChaCha8** por thread semeada pelo sistema
operacional. É uma cifra de fluxo, resistente a predição, e não o PRNG
estatístico usado até a `v0.3.0`. Ainda assim, a própria documentação do
Go recomenda `crypto/rand` para uso sensível a
segurança — os apelidos de compatibilidade `New`, `NewString`,
`NewRandom` e `NewV7` já leem de lá. E, independentemente da fonte, todo
UUIDv7 expõe o instante de criação por construção.

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
- Como contribuir: [CONTRIBUTING.md](CONTRIBUTING.md)
- Como relatar uma vulnerabilidade: [SECURITY.md](SECURITY.md)

## Licença

[MIT](LICENSE).
