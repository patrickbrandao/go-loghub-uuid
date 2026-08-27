package tests

import (
	"testing"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// TestGenerateZeroAllocations trava a propriedade de "zero alocações" da
// geração binária, exigida pela especificação (seção 8). Uma regressão
// aqui indica que algum caminho quente passou a escapar para o heap.
func TestGenerateZeroAllocations(t *testing.T) {
	g := uuid.NewGenerator()
	for _, level := range []uuid.Level{uuid.Level1, uuid.Level2, uuid.Level3} {
		allocs := testing.AllocsPerRun(1_000, func() {
			sinkU = g.Generate(level)
		})
		if allocs != 0 {
			t.Errorf("Generate(nível %d): %.0f alocações por chamada, esperado 0", level, allocs)
		}
	}
}

// TestGenerateStringSingleAllocation trava o limite de uma única
// alocação (a string final) na geração em texto.
func TestGenerateStringSingleAllocation(t *testing.T) {
	g := uuid.NewGenerator()
	for _, level := range []uuid.Level{uuid.Level1, uuid.Level2, uuid.Level3} {
		allocs := testing.AllocsPerRun(1_000, func() {
			sinkS = g.GenerateString(level)
		})
		if allocs > 1 {
			t.Errorf("GenerateString(nível %d): %.0f alocações por chamada, esperado no máximo 1", level, allocs)
		}
	}
}

// TestFromStringZeroAllocations confere que a análise da string não
// aloca: ela escreve em um UUID por valor, devolvido na pilha.
func TestFromStringZeroAllocations(t *testing.T) {
	allocs := testing.AllocsPerRun(1_000, func() {
		sinkU, _ = uuid.FromString(canonical)
	})
	if allocs != 0 {
		t.Errorf("FromString: %.0f alocações por chamada, esperado 0", allocs)
	}
}

// TestImportZeroAllocations confere que a extração de tempo do binário
// não aloca.
func TestImportZeroAllocations(t *testing.T) {
	u := uuid.Generate(uuid.Level3)
	allocs := testing.AllocsPerRun(1_000, func() {
		sinkT = uuid.ImportBinary(u)
	})
	if allocs != 0 {
		t.Errorf("ImportBinary: %.0f alocações por chamada, esperado 0", allocs)
	}
}

// sinkT evita que o compilador elimine as chamadas de importação.
var sinkT uuid.Time
