package tests

import (
	"testing"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// withIsolatedClockState impede que um teste deixe o estado global das
// versões 1, 2 e 6 — o identificador de nó e a sequência de relógio, em
// clock.go — alterado para os testes seguintes. Guarda o nó em uso e, ao
// fim do teste, devolve-o e entra em uma sequência inédita.
//
// Restaurar é seguro porque a biblioteca mantém um piso de relógio por
// sequência: voltar a um nó antigo nunca repete um UUID. E
// SetClockSequence(-1) descarta o adiantamento que a rajada do teste
// acumulou no relógio interno, entregando ao teste seguinte um relógio
// alinhado com o do sistema.
//
// Nenhum teste das versões 1, 2 ou 6 pode usar t.Parallel(): esse estado
// é global ao pacote, então testes concorrentes sobrescreveriam o nó e a
// sequência uns dos outros e produziriam falhas intermitentes que não
// apontam para o teste culpado.
func withIsolatedClockState(t *testing.T) {
	t.Helper()
	node := uuid.NodeID()
	t.Cleanup(func() {
		if !uuid.SetNodeID(node) {
			t.Errorf("nao foi possivel restaurar o no %x ao fim do teste", node)
		}
		uuid.SetClockSequence(-1)
	})
}

// TestDefaultNodeIsMulticast confere que o nó sorteado por padrão tem o
// bit multicast ligado, como pede a RFC 9562 seção 6.10 para nós que não
// são endereços MAC reais. O teste só é confiável porque todo teste que
// chama SetNodeID passa por withIsolatedClockState e devolve o nó.
func TestDefaultNodeIsMulticast(t *testing.T) {
	node := uuid.NodeID()
	if node[0]&0x01 != 0x01 {
		t.Errorf("no padrao %x sem o bit multicast; algum teste deixou um no proprio para tras", node)
	}
}
