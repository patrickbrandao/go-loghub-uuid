package tests

import (
	"testing"
	"time"

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

// TestGenerateV4ZeroAllocations estende a trava de zero alocações ao
// gerador de versão 4, que compartilha a mesma fonte de entropia.
func TestGenerateV4ZeroAllocations(t *testing.T) {
	g := uuid.NewGenerator()
	allocs := testing.AllocsPerRun(1_000, func() {
		sinkU = g.GenerateV4()
	})
	if allocs != 0 {
		t.Errorf("GenerateV4: %.0f alocações por chamada, esperado 0", allocs)
	}
}

// TestAppendToZeroAllocations trava a razão de existir de AppendTo: com
// capacidade sobrando no buffer do chamador, a escrita não aloca nada,
// enquanto String aloca a string devolvida em toda chamada.
func TestAppendToZeroAllocations(t *testing.T) {
	u := uuid.Generate(uuid.Level3)
	buf := make([]byte, 0, 64)
	allocs := testing.AllocsPerRun(1_000, func() {
		sinkB = u.AppendTo(buf[:0])
	})
	if allocs != 0 {
		t.Errorf("AppendTo: %.0f alocações por chamada, esperado 0", allocs)
	}
}

// sinkB evita que o compilador elimine as chamadas de AppendTo.
var sinkB []byte

// TestAppendBinaryZeroAllocations confere a mesma propriedade de
// AppendTo no lado binário: com capacidade sobrando no buffer do
// chamador, a escrita dos 16 bytes não aloca nada, enquanto
// MarshalBinary devolve uma fatia apoiada no valor recebido.
func TestAppendBinaryZeroAllocations(t *testing.T) {
	u := uuid.Generate(uuid.Level3)
	buf := make([]byte, 0, 32)
	allocs := testing.AllocsPerRun(1_000, func() {
		sinkB, _ = u.AppendBinary(buf[:0])
	})
	if allocs != 0 {
		t.Errorf("AppendBinary: %.0f alocações por chamada, esperado 0", allocs)
	}
}

// TestTimeBasedZeroAllocations confere que as versões 1, 2 e 6 também
// escrevem direto no valor de retorno, sem escapar para o heap.
func TestTimeBasedZeroAllocations(t *testing.T) {
	cases := map[string]func() uuid.UUID{
		"GenerateV1": uuid.GenerateV1,
		"GenerateV6": uuid.GenerateV6,
		"GenerateV2": func() uuid.UUID { return uuid.GenerateV2(uuid.Org, 1) },
	}
	for name, generate := range cases {
		allocs := testing.AllocsPerRun(1_000, func() {
			sinkU = generate()
		})
		if allocs != 0 {
			t.Errorf("%s: %.0f alocações por chamada, esperado 0", name, allocs)
		}
	}
}

// TestParseZeroAllocations confere que o analisador permissivo não paga
// alocação em nenhum dos quatro formatos, nem a partir de bytes.
func TestParseZeroAllocations(t *testing.T) {
	raw := []byte(canonical)
	cases := map[string]func(){
		"Parse canônico":        func() { sinkU, _ = uuid.Parse(canonical) },
		"Parse entre chaves":    func() { sinkU, _ = uuid.Parse("{" + canonical + "}") },
		"Parse URN":             func() { sinkU, _ = uuid.Parse("urn:uuid:" + canonical) },
		"Parse hexadecimal cru": func() { sinkU, _ = uuid.Parse("0192f7c51a2b7c3d8e4faabbccddeeff") },
		"ParseBytes":            func() { sinkU, _ = uuid.ParseBytes(raw) },
	}
	for name, call := range cases {
		if allocs := testing.AllocsPerRun(1_000, call); allocs != 0 {
			t.Errorf("%s: %.0f alocações por chamada, esperado 0", name, allocs)
		}
	}
}

// TestBoundsZeroAllocations trava a ausência de alocações nas fronteiras
// de tempo. Elas montam um UUID por valor, devolvido na pilha, e são
// chamadas uma vez por consulta — mas uma alocação aqui denunciaria que
// o instante ou o nível passaram a escapar para o heap.
func TestBoundsZeroAllocations(t *testing.T) {
	instante := time.Now()
	fim := instante.Add(time.Second)
	for _, level := range []uuid.Level{uuid.Level1, uuid.Level2, uuid.Level3} {
		cases := map[string]func(){
			"MinAt":   func() { sinkU = uuid.MinAt(level, instante) },
			"MaxAt":   func() { sinkU = uuid.MaxAt(level, instante) },
			"RangeAt": func() { sinkU, sinkU = uuid.RangeAt(level, instante, fim) },
		}
		for name, call := range cases {
			if allocs := testing.AllocsPerRun(1_000, call); allocs != 0 {
				t.Errorf("%s(nível %d): %.0f alocações por chamada, esperado 0", name, level, allocs)
			}
		}
	}
}

// TestGenerateAtZeroAllocations trava a mesma propriedade de Generate na
// geração por instante explícito: zero alocações na forma binária e no
// máximo uma, a string final, na forma em texto.
func TestGenerateAtZeroAllocations(t *testing.T) {
	g := uuid.NewGenerator()
	instante := time.Now()
	for _, level := range []uuid.Level{uuid.Level1, uuid.Level2, uuid.Level3} {
		allocs := testing.AllocsPerRun(1_000, func() {
			sinkU = g.GenerateAt(level, instante)
		})
		if allocs != 0 {
			t.Errorf("GenerateAt(nível %d): %.0f alocações por chamada, esperado 0", level, allocs)
		}

		allocs = testing.AllocsPerRun(1_000, func() {
			sinkS = g.GenerateAtString(level, instante)
		})
		if allocs > 1 {
			t.Errorf("GenerateAtString(nível %d): %.0f alocações por chamada, esperado no máximo 1", level, allocs)
		}
	}
}
