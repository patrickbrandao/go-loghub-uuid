package tests

import (
	"encoding/hex"
	"testing"
	"time"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// Este arquivo guarda os vetores dourados da extensão multinível,
// publicados em docs/SPEC.md seção 10, caso obrigatório 12.
//
// Eles são CONTRATO, não teste comum: uma implementação em outra
// linguagem confere o próprio empacotamento contra esta tabela. Mudar
// qualquer valor aqui é mudança de formato de dados, não ajuste de
// teste. Se um destes falhar, o defeito está no código, não no vetor.
//
// Os valores foram calculados por uma implementação independente,
// escrita a partir das regras das seções 3.1, 3.2 e 3.5 da
// especificação, e só então conferidos contra esta biblioteca. Um vetor
// gerado pela própria implementação e conferido contra ela mesma não
// provaria nada.
//
// MinAt e MaxAt são o caminho para produzi-los sem relógio: elas dão,
// por construção, os 16 bytes de um instante fixo com todos os bits
// livres em zero e em um.

// goldenVector é uma linha da tabela publicada.
type goldenVector struct {
	grupo     string // identificador do grupo na especificação
	instante  string // o instante em texto, como publicado
	sec       int64  // segundos Unix do instante
	nsec      int    // nanossegundos dentro do segundo
	level     uuid.Level
	bitsLivre string // "zero" ou "um"
	canonica  string // a string canônica esperada
}

var goldenVectors = []goldenVector{
	{"A", "2026-01-01T00:00:00.123456789Z", 1767225600, 123456789, uuid.Level1, "zero", "019b76da-a87b-7000-8000-000000000000"},
	{"A", "2026-01-01T00:00:00.123456789Z", 1767225600, 123456789, uuid.Level1, "um", "019b76da-a87b-7fff-bfff-ffffffffffff"},
	{"A", "2026-01-01T00:00:00.123456789Z", 1767225600, 123456789, uuid.Level2, "zero", "019b76da-a87b-71c8-8000-000000000000"},
	{"A", "2026-01-01T00:00:00.123456789Z", 1767225600, 123456789, uuid.Level2, "um", "019b76da-a87b-71c8-bfff-ffffffffffff"},
	{"A", "2026-01-01T00:00:00.123456789Z", 1767225600, 123456789, uuid.Level3, "zero", "019b76da-a87b-71c8-b150-000000000000"},
	{"A", "2026-01-01T00:00:00.123456789Z", 1767225600, 123456789, uuid.Level3, "um", "019b76da-a87b-71c8-b15f-ffffffffffff"},
	{"B", "2026-01-01T00:00:00.000000000Z", 1767225600, 0, uuid.Level1, "zero", "019b76da-a800-7000-8000-000000000000"},
	{"B", "2026-01-01T00:00:00.000000000Z", 1767225600, 0, uuid.Level1, "um", "019b76da-a800-7fff-bfff-ffffffffffff"},
	{"B", "2026-01-01T00:00:00.000000000Z", 1767225600, 0, uuid.Level2, "zero", "019b76da-a800-7000-8000-000000000000"},
	{"B", "2026-01-01T00:00:00.000000000Z", 1767225600, 0, uuid.Level2, "um", "019b76da-a800-7000-bfff-ffffffffffff"},
	{"B", "2026-01-01T00:00:00.000000000Z", 1767225600, 0, uuid.Level3, "zero", "019b76da-a800-7000-8000-000000000000"},
	{"B", "2026-01-01T00:00:00.000000000Z", 1767225600, 0, uuid.Level3, "um", "019b76da-a800-7000-800f-ffffffffffff"},
	{"C", "1970-01-01T00:00:00.000000000Z", 0, 0, uuid.Level1, "zero", "00000000-0000-7000-8000-000000000000"},
	{"C", "1970-01-01T00:00:00.000000000Z", 0, 0, uuid.Level1, "um", "00000000-0000-7fff-bfff-ffffffffffff"},
	{"C", "1970-01-01T00:00:00.000000000Z", 0, 0, uuid.Level2, "zero", "00000000-0000-7000-8000-000000000000"},
	{"C", "1970-01-01T00:00:00.000000000Z", 0, 0, uuid.Level2, "um", "00000000-0000-7000-bfff-ffffffffffff"},
	{"C", "1970-01-01T00:00:00.000000000Z", 0, 0, uuid.Level3, "zero", "00000000-0000-7000-8000-000000000000"},
	{"C", "1970-01-01T00:00:00.000000000Z", 0, 0, uuid.Level3, "um", "00000000-0000-7000-800f-ffffffffffff"},
}

// TestGoldenVectors confere os 16 bytes e a string canônica de cada
// vetor publicado, nos três níveis e nas duas pontas de entropia.
// Qualquer bit que troque de lugar quebra este teste.
func TestGoldenVectors(t *testing.T) {
	for _, v := range goldenVectors {
		instante := time.Unix(v.sec, int64(v.nsec)).UTC()

		var got uuid.UUID
		switch v.bitsLivre {
		case "zero":
			got = uuid.MinAt(v.level, instante)
		case "um":
			got = uuid.MaxAt(v.level, instante)
		default:
			t.Fatalf("vetor %s: bits livres %q desconhecido", v.grupo, v.bitsLivre)
		}

		if got.String() != v.canonica {
			t.Errorf("grupo %s, %s, nível %d, bits livres em %s:\n  obtido:   %s\n  esperado: %s",
				v.grupo, v.instante, v.level, v.bitsLivre, got, v.canonica)
			continue
		}

		// Confere também os 16 bytes, não só o texto: a formatação
		// canônica é outro caminho de código e poderia mascarar um erro
		// de empacotamento se os dois fossem escritos juntos.
		querBytes, err := hex.DecodeString(
			v.canonica[0:8] + v.canonica[9:13] + v.canonica[14:18] + v.canonica[19:23] + v.canonica[24:])
		if err != nil {
			t.Fatalf("vetor %s: a string publicada não é hexadecimal válido: %v", v.grupo, err)
		}
		if string(got.Bytes()) != string(querBytes) {
			t.Errorf("grupo %s, nível %d, bits livres em %s: bytes %x, esperado %x",
				v.grupo, v.level, v.bitsLivre, got.Bytes(), querBytes)
		}
	}
}

// TestGoldenVectorsCoverAllLevelsAndFills confere que a tabela publicada
// não perdeu linhas: três grupos, três níveis e duas pontas de entropia.
// Uma tabela incompleta passaria despercebida, porque cada linha se
// verifica sozinha.
func TestGoldenVectorsCoverAllLevelsAndFills(t *testing.T) {
	const esperado = 3 * 3 * 2
	if len(goldenVectors) != esperado {
		t.Fatalf("a tabela tem %d vetores, esperado %d", len(goldenVectors), esperado)
	}

	visto := make(map[string]bool, esperado)
	for _, v := range goldenVectors {
		chave := v.grupo + string(rune('0'+v.level)) + v.bitsLivre
		if visto[chave] {
			t.Errorf("vetor repetido: grupo %s, nível %d, bits livres em %s", v.grupo, v.level, v.bitsLivre)
		}
		visto[chave] = true
	}
}

// TestGoldenVectorPreEpochFloorsToEpoch confere o piso pré-época contra
// a mesma tabela: um instante anterior a 1970 produz exatamente os
// vetores do grupo C, o da própria época.
func TestGoldenVectorPreEpochFloorsToEpoch(t *testing.T) {
	preEpoca := time.Date(1969, 12, 31, 23, 59, 59, 999_999_999, time.UTC)

	for _, v := range goldenVectors {
		if v.grupo != "C" {
			continue
		}
		var got uuid.UUID
		if v.bitsLivre == "zero" {
			got = uuid.MinAt(v.level, preEpoca)
		} else {
			got = uuid.MaxAt(v.level, preEpoca)
		}
		if got.String() != v.canonica {
			t.Errorf("pré-época, nível %d, bits livres em %s: %s, esperado o vetor da época %s",
				v.level, v.bitsLivre, got, v.canonica)
		}
	}
}
