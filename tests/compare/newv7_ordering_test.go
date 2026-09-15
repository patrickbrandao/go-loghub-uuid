package compare

import (
	"bytes"
	"testing"

	google "github.com/google/uuid"
	loghub "github.com/patrickbrandao/go-loghub-uuid"
)

// TestNewV7OrderingDiffersFromGoogle mede a divergência documentada em
// docs/12-migracao-google-uuid.md §2: o NewV7 do pacote do Google mantém
// um contador interno que garante ordem estrita entre chamadas
// consecutivas, mesmo dentro do mesmo milissegundo; esta biblioteca
// decidiu não ter contador monotônico
// (docs/10-armadilhas-e-decisoes-de-projeto.md seção 11.1), então o
// desempate dentro do milissegundo é aleatório e a ordem pode regredir.
//
// Não afirma um número exato de regressões — depende da velocidade do
// host e do relógio, como TestTieRateReport já registra para o restante
// da API de versão 7 — só que o pacote do Google não regride nenhuma
// vez numa rajada de 200 mil chamadas, e que esta biblioteca regride em
// uma fração observável (não perto de zero) das mesmas 200 mil.
func TestNewV7OrderingDiffersFromGoogle(t *testing.T) {
	const n = 200_000

	regressoes := func(gerar func() [16]byte) int {
		total := 0
		anterior := gerar()
		for i := 0; i < n; i++ {
			atual := gerar()
			if bytes.Compare(atual[:], anterior[:]) <= 0 {
				total++
			}
			anterior = atual
		}
		return total
	}

	googleRegressoes := regressoes(func() [16]byte {
		u, err := google.NewV7()
		if err != nil {
			t.Fatalf("google.NewV7: %v", err)
		}
		return u
	})
	loghubRegressoes := regressoes(func() [16]byte {
		u, err := loghub.NewV7()
		if err != nil {
			t.Fatalf("loghub.NewV7: %v", err)
		}
		return u
	})

	t.Logf("regressões de ordem em %d pares consecutivos: google=%d loghub=%d", n, googleRegressoes, loghubRegressoes)

	if googleRegressoes != 0 {
		t.Errorf("google.NewV7 regrediu %d vezes; esperado ordem estrita (contador interno de 12 bits)", googleRegressoes)
	}
	// Um piso conservador: com o desempate aleatório e milhares de
	// chamadas por milissegundo em qualquer host atual, a fração de
	// empates é bem acima de 1%. Nenhum número exato é afirmado, só que
	// a divergência é observável e não desaparece por acaso.
	if loghubRegressoes < n/100 {
		t.Errorf("loghub.NewV7 regrediu só %d vezes em %d; esperado uma fração observável, não perto de zero", loghubRegressoes, n)
	}
}
