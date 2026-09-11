package loghubuuid_test

// Este é o único arquivo de exemplos da biblioteca e fica na raiz de
// propósito: o godoc só associa funções Example ao pacote documentado
// quando elas vivem no mesmo diretório, no próprio pacote ou no seu pacote
// externo de teste. Como ./tests/ é outro pacote, exemplos lá não
// apareceriam em pkg.go.dev. O pacote é loghubuuid_test, então o arquivo
// importa a biblioteca pelo caminho do módulo, como um consumidor faria, e
// não tem acesso a nada interno.
//
// Cada exemplo reproduz um trecho de README.md, docs/DEPLOY-FAST.md ou
// docs/DEPLOY-FULL.md, para que a documentação seja compilada e executada
// pela suíte. Os exemplos com saída verificável (comentário "Output:")
// usam apenas valores fixos; os que dependem do relógio ou de
// aleatoriedade não declaram saída e servem só para compilar o uso.

import (
	"encoding/json"
	"fmt"
	"time"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// A forma mais curta: as funções de pacote usam um gerador padrão
// interno, já pronto e seguro para concorrência.
func ExampleGenerateString() {
	s1 := uuid.GenerateString(uuid.Level1) // só milissegundos (UUIDv7 padrão)
	s2 := uuid.GenerateString(uuid.Level2) // até microssegundos (rand_a)
	s3 := uuid.GenerateString(uuid.Level3) // até nanossegundos (rand_a + rand_b)
	fmt.Println(len(s1), len(s2), len(s3))
	// Output: 36 36 36
}

// Em serviços de alto volume, crie um único Generator no boot e
// reutilize-o em todas as goroutines.
func ExampleGenerator_Generate() {
	gen := uuid.NewGenerator()

	u := gen.Generate(uuid.Level3) // u é um uuid.UUID ([16]byte)
	fmt.Println(u.Version(), u.Variant())
	// Output: 7 2
}

// FromString aceita apenas a forma canônica 8-4-4-4-12, em maiúsculas ou
// minúsculas, e devolve ErrInvalidFormat para qualquer outra coisa.
func ExampleFromString() {
	u, err := uuid.FromString("0192F7C5-1A2B-7C3D-8E4F-AABBCCDDEEFF")
	if err != nil {
		fmt.Println("erro:", err)
		return
	}
	fmt.Println(u.String())

	_, err = uuid.FromString("0192f7c51a2b7c3d8e4faabbccddeeff")
	fmt.Println(err == uuid.ErrInvalidFormat) //nolint:errorlint // FromString devolve o sentinela puro; a comparação direta faz parte do contrato
	// Output:
	// 0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff
	// true
}

// ImportBinary lê os campos de tempo de um UUIDv7. A leitura é cega
// quanto ao nível: rand_a é sempre interpretado como microssegundos e o
// topo de rand_b como nanossegundos, mesmo que o UUID seja de Nível 1 e
// esses bits sejam aleatórios.
func ExampleImportBinary() {
	// UUIDv7 de Nível 3 montado à mão: 0x0192f7c51a2b ms desde a época,
	// 0x1c3 = 451 microssegundos em rand_a e 0x393 = 915 nanossegundos
	// nos 10 bits altos de rand_b.
	u := uuid.MustParse("0192f7c5-1a2b-71c3-b930-0000aabbccdd")

	t := uuid.ImportBinary(u)
	fmt.Printf("seg=%d ms=%03d us=%03d ns=%03d\n",
		t.Seconds, t.Milliseconds, t.Microseconds, t.Nanoseconds)
	// Output: seg=1730733742 ms=635 us=451 ns=915
}

// Parse aceita as quatro formas usuais de escrever um UUID; todas
// produzem o mesmo valor.
func ExampleParse() {
	inputs := []string{
		"0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff",
		"{0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff}",
		"urn:uuid:0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff",
		"0192f7c51a2b7c3d8e4faabbccddeeff",
	}
	for _, in := range inputs {
		u, err := uuid.Parse(in)
		if err != nil {
			fmt.Println("erro:", err)
			continue
		}
		fmt.Println(u)
	}
	// Output:
	// 0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff
	// 0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff
	// 0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff
	// 0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff
}

// TimestampWithLevel reconstrói o instante com a precisão do nível
// informado. O nível não pode ser deduzido do UUID, por isso o chamador
// precisa dizê-lo.
func ExampleUUID_TimestampWithLevel() {
	// O mesmo UUID de Nível 3 usado em ExampleImportBinary.
	u := uuid.MustParse("0192f7c5-1a2b-71c3-b930-0000aabbccdd")

	ms, _ := u.Timestamp() // só o milissegundo, como qualquer UUIDv7
	l3, _ := u.TimestampWithLevel(uuid.Level3)

	fmt.Println(ms.Format(time.RFC3339Nano))
	fmt.Println(l3.Format(time.RFC3339Nano))
	// Output:
	// 2024-11-04T15:22:22.635Z
	// 2024-11-04T15:22:22.635451915Z
}

// GenerateV5 é determinística: o mesmo par de espaço de nomes e nome
// sempre devolve o mesmo UUID. O valor abaixo é o vetor de teste da RFC
// 9562 para o espaço DNS.
func ExampleGenerateV5() {
	u := uuid.GenerateV5(uuid.NameSpaceDNS, []byte("www.example.com"))
	fmt.Println(u)
	// Output: 2ed6657d-e927-568b-95e1-2665a8aea6a2
}

// NullUUID representa uma coluna que aceita NULL e viaja em JSON como a
// string canônica ou como null.
func ExampleNullUUID() {
	type Registro struct {
		ID  uuid.UUID     `json:"id"`
		Pai uuid.NullUUID `json:"pai"`
	}

	raiz := Registro{ID: uuid.MustParse("0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff")}
	dados, _ := json.Marshal(raiz)
	fmt.Println(string(dados))

	var lido Registro
	_ = json.Unmarshal([]byte(`{"id":"0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff","pai":"{0192F7C5-1A2B-7C3D-8E4F-AABBCCDDEEFF}"}`), &lido)
	fmt.Println(lido.Pai.Valid, lido.Pai.UUID == lido.ID)
	// Output:
	// {"id":"0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff","pai":null}
	// true true
}

// NewCryptoGenerator monta um gerador cuja entropia vem inteiramente de
// crypto/rand, para identificadores que precisem ser inadivinháveis. O
// gerador padrão é mais rápido, mas a recomendação para segredos continua
// sendo crypto/rand.
func ExampleNewCryptoGenerator() {
	gen := uuid.NewCryptoGenerator()

	u := gen.Generate(uuid.Level1)
	fmt.Println(u.Version(), u.Variant())
	// Output: 7 2
}

// MinAt e MaxAt devolvem o menor e o maior UUIDv7 que a biblioteca
// poderia gerar em um instante, no nível informado. As duas preservam a
// versão 7 e a variante RFC, e é isso que as torna limites corretos: a
// fronteira superior do Nível 1 termina em 7fff-bfff, não em ffff-ffff.
//
// No Nível 2 o campo rand_a carrega os microssegundos do instante (456,
// ou 0x1c8), então ele é igual nas duas fronteiras e só rand_b varia.
func ExampleMinAt() {
	instante := time.Date(2026, 9, 11, 12, 34, 56, 123_456_789, time.UTC)

	fmt.Println(uuid.MinAt(uuid.Level1, instante))
	fmt.Println(uuid.MaxAt(uuid.Level1, instante))
	fmt.Println(uuid.MinAt(uuid.Level2, instante))
	fmt.Println(uuid.MaxAt(uuid.Level2, instante))
	// Output:
	// 01a09076-bdfb-7000-8000-000000000000
	// 01a09076-bdfb-7fff-bfff-ffffffffffff
	// 01a09076-bdfb-71c8-8000-000000000000
	// 01a09076-bdfb-71c8-bfff-ffffffffffff
}

// RangeAt devolve as duas fronteiras de um intervalo semiaberto, prontas
// para consultar por faixa usando o índice da própria chave primária:
//
//	SELECT * FROM eventos WHERE id >= $1 AND id < $2 ORDER BY id
//
// A fronteira só vale para identificadores gravados no mesmo nível: os
// bits abaixo do milissegundo significam coisas diferentes em cada um.
func ExampleRangeAt() {
	inicio := time.Date(2026, 9, 11, 12, 34, 56, 123_456_789, time.UTC)
	fim := inicio.Add(time.Millisecond)

	lo, hi := uuid.RangeAt(uuid.Level2, inicio, fim)
	fmt.Println(lo)
	fmt.Println(hi)
	// Output:
	// 01a09076-bdfb-71c8-8000-000000000000
	// 01a09076-bdfc-71c8-8000-000000000000
}

// GenerateAt constrói um UUIDv7 para um instante que você informa, em
// vez do instante atual. É o sentido inverso de Import, e serve para
// reprocessar histórico preservando a ordenação cronológica da chave.
//
// Os bits livres são sorteados: com o gerador padrão, duas chamadas com
// o mesmo instante devolvem valores diferentes. Aqui a entropia é fixa
// em zero só para o exemplo ter saída verificável.
func ExampleGenerateAt() {
	quando := time.Date(2019, 3, 14, 10, 0, 0, 123_456_789, time.UTC)
	gen := uuid.NewGeneratorWith(func() uint64 { return 0 })

	u := gen.GenerateAt(uuid.Level3, quando)
	fmt.Println(u)

	// Os campos de tempo voltam exatos no Nível 3, que grava os três.
	t := uuid.ImportBinary(u)
	fmt.Println(t.Seconds, t.Milliseconds, t.Microseconds, t.Nanoseconds)
	// Output:
	// 01697ba4-ed7b-71c8-b150-000000000000
	// 1552557600 123 456 789
}
