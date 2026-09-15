// Pacote compare guarda as verificações que exigem o pacote
// github.com/google/uuid, contra o qual esta biblioteca se posiciona
// como destino de migração.
//
// Ele é um MÓDULO ANINHADO, com go.mod próprio, e isso é deliberado: a
// ausência de dependências é característica do projeto e não pode ser
// perdida por causa de um teste comparativo. O comando "go test ./..."
// na raiz não desce em módulos aninhados, então nem o go.mod nem o
// go.sum da raiz ganham qualquer require. Uma auditoria que encontre
// este diretório não deve confundi-lo com dependência do projeto.
//
// O escopo aqui é DIFERENÇA DE COMPORTAMENTO, não velocidade. A
// documentação do projeto afirma, em docs/06-relogio-e-concorrencia.md
// seção 4.2 e no CLAUDE.md, que o outro pacote pode repetir um UUIDv1 ao
// voltar a uma sequência de relógio já usada, enquanto esta biblioteca
// não. É uma afirmação factual sobre software alheio, usada como
// diferencial, e merece prova executável em vez de prosa.
//
// Não há aqui tabela de benchmark comparativo: ela envelheceria a cada
// versão do pacote de terceiros e a decisão de não mantê-la está
// registrada em CHANGELOG.md.
//
// Para rodar:
//
//	cd tests/compare && go test -v ./...
package compare
