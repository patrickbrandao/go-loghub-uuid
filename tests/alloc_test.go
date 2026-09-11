package tests

import (
	"testing"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// skipIfRaceDetector pula as travas de alocação que dependem do gerador
// padrão quando a suíte roda com -race.
//
// Motivo: o sync.Pool da biblioteca padrão, compilado com o detector de
// corrida, descarta de propósito um em cada quatro itens devolvidos por
// Put (é assim que ele exercita o caso "item novo" sob o detector).
// Cada descarte obriga o próximo Get a construir outro PRNG, e
// testing.AllocsPerRun contabiliza essas realocações, que não existem
// fora do -race. Em Go 1.22 isso fazia a trava falhar de forma
// intermitente; a medição válida é a feita sem o detector, que o fluxo
// de integração contínua executa em passo próprio.
func skipIfRaceDetector(t *testing.T) {
	t.Helper()
	if raceDetectorEnabled {
		t.Skip("trava de alocações pulada sob -race: o sync.Pool descarta itens ao acaso com o detector ativo")
	}
}

// TestGenerateZeroAllocations trava a propriedade de "zero alocações" da
// geração binária, exigida pela especificação (seção 8). Uma regressão
// aqui indica que algum caminho quente passou a escapar para o heap.
func TestGenerateZeroAllocations(t *testing.T) {
	skipIfRaceDetector(t)
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
	skipIfRaceDetector(t)
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
	skipIfRaceDetector(t)
	g := uuid.NewGenerator()
	allocs := testing.AllocsPerRun(1_000, func() {
		sinkU = g.GenerateV4()
	})
	if allocs != 0 {
		t.Errorf("GenerateV4: %.0f alocações por chamada, esperado 0", allocs)
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
		"Parse canônico":     func() { sinkU, _ = uuid.Parse(canonical) },
		"Parse entre chaves": func() { sinkU, _ = uuid.Parse("{" + canonical + "}") },
		"Parse URN":          func() { sinkU, _ = uuid.Parse("urn:uuid:" + canonical) },
		"ParseBytes":         func() { sinkU, _ = uuid.ParseBytes(raw) },
	}
	for name, call := range cases {
		if allocs := testing.AllocsPerRun(1_000, call); allocs != 0 {
			t.Errorf("%s: %.0f alocações por chamada, esperado 0", name, allocs)
		}
	}
}
