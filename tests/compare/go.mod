// Modulo ANINHADO, deliberadamente separado da raiz.
//
// A ausencia de dependencias e caracteristica do projeto: go.mod e
// go.sum da raiz nao podem ganhar nenhum require. "go test ./..." na
// raiz nao desce em modulos aninhados, entao a dependencia de
// github.com/google/uuid usada aqui fica contida neste diretorio.
//
// Ver docs/TEST-AND-BENCHMARK.md e o CLAUDE.md.
module github.com/patrickbrandao/go-loghub-uuid/tests/compare

go 1.22

replace github.com/patrickbrandao/go-loghub-uuid => ../..

require (
	github.com/google/uuid v1.6.0
	github.com/patrickbrandao/go-loghub-uuid v0.0.0-00010101000000-000000000000
)
