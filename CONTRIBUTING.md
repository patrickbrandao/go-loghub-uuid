# Como contribuir

Obrigado pelo interesse. Este guia resume o que uma contribuição precisa
ter para ser aceita. As regras completas de manutenção estão em
`CLAUDE.md`; o mapa do projeto, em `STARTHERE.md`.

## Antes de começar

- Abra uma issue descrevendo o problema ou a proposta antes de um pull
  request grande. Mudanças de projeto já avaliadas e rejeitadas estão
  registradas na seção "Decisões" do `CHANGELOG.md`, e as propostas em
  aberto, no fim do mesmo arquivo; consulte-os para não repetir uma
  discussão encerrada sem argumento novo.
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
está em `CLAUDE.md` e em `docs/TEST-AND-BENCHMARK.md`.

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
