# Guia de release

Procedimento para publicar uma versão da biblioteca: da verificação do
commit até a confirmação de que o proxy de módulos entrega a tag nova.
Nada aqui configura a máquina; pressupõe um clone com o remoto `origin`
apontando para `github.com/patrickbrandao/go-loghub-uuid` e a CLI `gh`
autenticada.

Substitua `vX.Y.Z` pela versão a publicar. As versões seguem o
versionamento semântico: correção sem mudança de comportamento incrementa
o último número; mudança de comportamento ou API nova incrementa o do
meio enquanto a versão principal for zero.

---

## 1. Pré-requisitos

Todos precisam estar satisfeitos no commit que será etiquetado.

- O fluxo `ci / test` está verde nesse commit, na aba Actions do GitHub.
  Confira pela CLI:

  ```bash
  gh run list --branch main --limit 3
  ```

  Ou reproduza localmente o essencial:

  ```bash
  gofmt -l . && go vet ./... && go test ./... -race -short
  ```

- `CHANGELOG.md` tem a seção `## [vX.Y.Z] — AAAA-MM-DD` com a data do
  dia, uma seção `## [Não publicado]` vazia acima dela, e os links de
  comparação no rodapé atualizados (`[Não publicado]` comparando
  `vX.Y.Z...HEAD` e `[vX.Y.Z]` comparando a versão anterior com a nova).
- `README.md` aponta para `vX.Y.Z` no exemplo de `go.mod`.
- `go.mod` continua em `go 1.22`, a versão mínima suportada.
- A árvore de trabalho está limpa (`git status`), tudo em `main` e
  enviado ao remoto.

Commit dessas edições, no estilo dos existentes:

```bash
git commit -am "chore: publica a versao X.Y.Z"
git push origin main
```

Espere o CI ficar verde de novo antes de seguir.

## 2. Etiquetar e publicar

Tag anotada, envio da tag e release no GitHub com as notas geradas a
partir dos commits (ou, se preferir, `--notes-file` com a seção do
`CHANGELOG.md`):

```bash
git tag -a vX.Y.Z -m "Release version X.Y.Z"
git push origin vX.Y.Z
gh release create vX.Y.Z --title "vX.Y.Z" --generate-notes
```

O push da tag dispara o fluxo `test` mais uma vez, agora sobre a tag.

## 3. Verificar a publicação

Em um diretório temporário fora do repositório, confirme que o proxy
público resolve a versão e que o módulo compila como dependência:

```bash
cd "$(mktemp -d)"
go mod init verificacao
go get github.com/patrickbrandao/go-loghub-uuid@vX.Y.Z
go list -m -versions github.com/patrickbrandao/go-loghub-uuid
```

A lista de versões deve incluir `vX.Y.Z`. O proxy pode levar alguns
minutos para indexar uma tag nova; se a busca falhar logo após o push,
repita depois.

Opcionalmente, confira a página do pacote em
`https://pkg.go.dev/github.com/patrickbrandao/go-loghub-uuid@vX.Y.Z`,
que é gerada sob demanda na primeira visita.

## 4. Tags são imutáveis

Nunca mova, apague ou recrie uma tag já enviada (`git tag -f`,
`git push -f`). O proxy de módulos e o `sum.golang.org` guardam o
conteúdo e o resumo criptográfico de cada versão no momento em que ela é
buscada pela primeira vez; uma tag reapontada faz o `go get` dos usuários
falhar com erro de verificação de soma, e não há como corrigir isso do
lado do servidor.

Se algo sair errado depois do push da tag, a correção é uma versão nova
(`vX.Y.Z+1`), com a entrada correspondente no `CHANGELOG.md`.
