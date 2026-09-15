# Como contribuir

Obrigado pelo interesse. Este guia resume o que uma contribuição precisa
ter para ser aceita. As regras completas de manutenção estão em
`CLAUDE.md`; o mapa do projeto, em `STARTHERE.md`.

## Antes de começar

- Abra uma issue descrevendo o problema ou a proposta antes de um pull
  request grande.
- **Consulte antes a seção 11 do
  `docs/10-armadilhas-e-decisoes-de-projeto.md`**, o registro de decisões
  firmadas. Ela diz o que já foi avaliado e decidido — fonte de entropia
  do gerador padrão, ausência de contador monotônico, relógio não
  injetável, as duplicações deliberadas de código, a permanência em
  `v0.x` — com o motivo de cada uma e o que justificaria revê-la.
  Proposta que apenas reconhece um desses padrões, sem trazer argumento
  novo, será encerrada apontando para lá. Medição própria, caso de uso
  concreto ou mudança na RFC são argumentos novos; preferência de estilo
  e "outro pacote faz diferente" não são.
- O histórico de cada decisão, com data, está na seção "Decisões" do
  `CHANGELOG.md` da versão correspondente.
- Problemas de segurança seguem o caminho privado descrito em
  `SECURITY.md`, nunca uma issue pública.

## Convenções

- **Idioma.** Identificadores (tipos, funções, variáveis, constantes,
  nomes de arquivo) em inglês. Comentários, documentação, mensagens de
  erro e rótulos de console em português do Brasil. Sem emojis em lugar
  nenhum.
- **Raiz mínima.** Só código de produção fica na raiz, ao lado de
  `go.mod`, dos documentos principais e de `.github/`. Testes ficam em
  `./tests/` e importam a biblioteca pelo caminho do módulo, como um
  consumidor externo. As exceções na raiz são `clock_internal_test.go`
  (funções internas do relógio) e `example_test.go` (exemplos que o
  `go doc` precisa encontrar junto do pacote).
- **Sem dependências.** Apenas a biblioteca padrão. `go.mod` fica em
  `go 1.22`, a versão mínima suportada; a integração contínua compila e
  testa nessa versão.
- **Caminho quente intocável.** `uuid.go`, `conversion.go` e
  `import.go` não podem ganhar custo nem alocações. Se precisar mexer
  neles, meça antes e depois com
  `go test ./tests/ -run '^$' -bench BenchmarkGenerate -benchmem` e
  inclua os números no pull request.
- **Toda mudança de comportamento vem com teste** em `./tests/`, e com
  uma entrada em `CHANGELOG.md`, na seção "Não publicado", citando o
  arquivo alterado. Correção de defeito vem com um teste que falharia
  antes da correção.
- **Commits** em português, sem acentos no assunto, com prefixo
  `fix:`, `feat:`, `docs:`, `ci:`, `chore:` ou `test:`.

## Verificação local

Antes de abrir o pull request, tudo abaixo precisa passar:

```bash
gofmt -l .                                                      # saída vazia
go vet ./...
golangci-lint run ./...                                         # mesma configuração do CI (.golangci.yml)
go test ./... -race -short                                      # suíte sob o detector de corrida
go test ./tests/ -short -run 'Allocations|SingleAllocation' -v  # travas de alocação, sem -race
```

As travas de alocação são medidas sem `-race` de propósito; o motivo
está em `CLAUDE.md` e em `docs/11-testes-e-benchmark.md`.

A integração contínua roda esses mesmos passos no Go 1.22 e na versão
estável, em Linux, e a suíte curta em Windows e macOS. Um pull request
só é revisado com o fluxo `test` verde.

## O que evitar

- Testes de ordenação que contam regressões em laço apertado: eles medem
  o relógio do host, não a biblioteca. A invariante independente do
  relógio está em `TestOrderingFollowsEmbeddedTime`.
- Afrouxar uma trava de alocação ou uma invariante para fazer um teste
  passar. Se um teste falha em um sistema específico, a correção é no
  teste ou na documentação.
- Arquivos novos na raiz que não sejam código de produção.
