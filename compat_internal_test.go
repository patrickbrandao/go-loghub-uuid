package loghubuuid

// Testes internos dos apelidos de compatibilidade. Vivem na raiz porque
// precisam ler e trocar compatGenerator, estado privado do pacote — o
// mesmo motivo que justifica clock_internal_test.go. Nenhum deles altera
// o caminho quente de Generate; ambos restauram o estado ao final.

import (
	crand "crypto/rand"
	"io"
	"sync/atomic"
	"testing"
)

// TestNewCryptoGeneratorSourcesFromCryptoRand confere que NewCryptoGenerator
// de fato lê de crypto/rand.Reader, e não de outra fonte qualquer que
// produza bits com a mesma forma. Troca a variável exportada
// crypto/rand.Reader por um leitor que conta as chamadas, constrói um
// gerador novo (não o compatGenerator do pacote, já montado antes de
// qualquer teste rodar) e confere que o leitor substituto foi consultado.
//
// REGRESSÃO: um teste de mutação que fizesse NewCryptoGenerator devolver
// NewGenerator() (a fonte rápida do runtime, sem garantia
// criptográfica) passava por toda a suíte existente, porque nenhum teste
// olhava para a fonte de bits, só para a forma do UUID resultante.
func TestNewCryptoGeneratorSourcesFromCryptoRand(t *testing.T) {
	original := crand.Reader
	var calls atomic.Int64
	crand.Reader = &countingReader{r: original, calls: &calls}
	t.Cleanup(func() { crand.Reader = original })

	g := NewCryptoGenerator()
	_ = g.Generate(Level1)
	_ = g.GenerateV4()

	if calls.Load() == 0 {
		t.Error("NewCryptoGenerator não leu de crypto/rand.Reader durante a geração")
	}
}

// TestCompatAliasesUseCompatGenerator confere que New, NewString,
// NewRandom e NewV7 delegam todos ao mesmo compatGenerator do pacote, e
// não a defaultGenerator ou a um gerador construído à parte. Troca
// compatGenerator por um gerador determinístico, confere que as quatro
// funções produzem exatamente os bytes que essa fonte determina, e
// restaura o gerador original ao final.
//
// REGRESSÃO: um teste de mutação que fizesse qualquer uma destas quatro
// funções chamar defaultGenerator.GenerateV4()/Generate(Level1)
// diretamente, em vez de compatGenerator, passava pela suíte inteira:
// as duas fontes produzem UUIDs da mesma forma (versão e variante
// corretas), e nenhum teste anterior distinguia a origem dos bits.
func TestCompatAliasesUseCompatGenerator(t *testing.T) {
	original := compatGenerator
	compatGenerator = NewGeneratorWith(constantSourceForTest(0))
	t.Cleanup(func() { compatGenerator = original })

	wantV4 := UUID{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40, 0x00,
		0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	if got := New(); got != wantV4 {
		t.Errorf("New() = %x, esperado %x (fonte de compatGenerator)", got[:], wantV4[:])
	}
	if got := NewString(); got != wantV4.String() {
		t.Errorf("NewString() = %s, esperado %s", got, wantV4.String())
	}
	if got, err := NewRandom(); err != nil || got != wantV4 {
		t.Errorf("NewRandom() = %x, erro %v, esperado %x sem erro", got[:], err, wantV4[:])
	}

	wantV7 := UUID{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x70, 0x00,
		0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	if got, err := NewV7(); err != nil || got[6] != wantV7[6] || got[7] != wantV7[7] || got[8] != wantV7[8] {
		t.Errorf("NewV7() = %x, erro %v; bits livres deveriam vir zerados da fonte de compatGenerator", got[:], err)
	}
}

// countingReader embrulha outro io.Reader e conta quantas vezes Read foi
// chamado, sem alterar o conteúdo devolvido.
type countingReader struct {
	r     io.Reader
	calls *atomic.Int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	c.calls.Add(1)
	return c.r.Read(p)
}

// constantSourceForTest devolve uma fonte de entropia determinística.
// Duplicada aqui (em vez de reusar a de tests/layout_test.go) porque
// este arquivo vive no pacote da raiz, e a suíte externa não pode ser
// importada por ele.
func constantSourceForTest(v uint64) func() uint64 {
	return func() uint64 { return v }
}
