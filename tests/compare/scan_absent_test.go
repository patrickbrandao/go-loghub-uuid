package compare

import (
	"testing"

	google "github.com/google/uuid"
	loghub "github.com/patrickbrandao/go-loghub-uuid"
)

// referenciaScan é um UUID não nulo usado para popular o destino antes
// de cada leitura de valor ausente, para que a diferença entre "não
// tocar o destino" e "zerar o destino" seja observável.
const referenciaScan = "0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff"

// TestGoogleScanAbsentDoesNotTouchDestination mede a divergência
// documentada em docs/12-migracao-google-uuid.md §1 e em CLAUDE.md: Scan
// de um valor ausente (nil, string vazia ou fatia de bytes vazia) não
// devolve erro em nenhuma das duas bibliotecas, mas o pacote do Google
// não escreve no destino nesse caso, enquanto esta biblioteca sempre
// grava o UUID nulo (e, em NullUUID, Valid falso). Quem reaproveita o
// destino entre linhas — o próprio database/sql — vê o valor da linha
// anterior sobreviver lá e desaparecer aqui.
func TestGoogleScanAbsentDoesNotTouchDestination(t *testing.T) {
	for _, ausente := range []any{nil, "", []byte{}} {
		gu := google.MustParse(referenciaScan)
		if err := gu.Scan(ausente); err != nil {
			t.Fatalf("google: Scan(%#v) devolveu erro %v, esperado nulo", ausente, err)
		}
		if gu.String() != referenciaScan {
			t.Errorf("google: Scan(%#v) alterou o destino para %s; esperado o valor de referência intacto",
				ausente, gu)
		}

		lu := loghub.MustParse(referenciaScan)
		if err := lu.Scan(ausente); err != nil {
			t.Fatalf("loghub: Scan(%#v) devolveu erro %v, esperado nulo", ausente, err)
		}
		if !lu.IsZero() {
			t.Errorf("loghub: Scan(%#v) deixou %s; esperado o UUID nulo (a divergência medida deixou de existir)",
				ausente, lu)
		}
	}
}
