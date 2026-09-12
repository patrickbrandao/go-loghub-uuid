package tests

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// FuzzFromString procura entradas que façam FromString entrar em pânico
// ou aceitar uma string que não sobreviva ao round-trip. Rode com:
//
//	go test ./tests/ -run '^$' -fuzz FuzzFromString -fuzztime 30s
func FuzzFromString(f *testing.F) {
	f.Add(canonical)
	f.Add(strings.ToUpper(canonical))
	f.Add("")
	f.Add("00000000-0000-0000-0000-000000000000")
	f.Add("ffffffff-ffff-ffff-ffff-ffffffffffff")
	f.Add("0192f7c5-1a2b-7c3d-8e4f--abbccddeeff") // hífen extra: ver BUG-01
	f.Add("0192f7c5-1a2b-7c3d-8e4f-aabbccddee-f")
	f.Add(uuid.GenerateString(uuid.Level3))

	f.Fuzz(func(t *testing.T, s string) {
		// FromString jamais pode entrar em pânico, qualquer que seja a entrada.
		u, err := uuid.FromString(s)
		if err != nil {
			if u != (uuid.UUID{}) {
				t.Fatalf("FromString(%q) devolveu erro e um UUID não zerado: %v", s, u)
			}
			return
		}

		// Se aceitou, a string reemitida precisa ser a forma canônica
		// minúscula da entrada e voltar aos mesmos 16 bytes.
		canon := u.String()
		if canon != strings.ToLower(s) {
			t.Fatalf("FromString(%q) aceitou, mas String() devolveu %q", s, canon)
		}
		again, err := uuid.FromString(canon)
		if err != nil {
			t.Fatalf("round-trip falhou para %q: %v", canon, err)
		}
		if again != u {
			t.Fatalf("round-trip divergiu para %q: %v vs %v", s, u, again)
		}

		// Importar nunca pode falhar para uma string já aceita.
		if _, err := uuid.Import(s); err != nil {
			t.Fatalf("Import(%q) falhou depois de FromString aceitar: %v", s, err)
		}
	})
}

// FuzzParse procura entradas que façam o analisador permissivo entrar em
// pânico ou aceitar algo que não sobreviva ao round-trip.
//
// Vale mais que FuzzFromString porque Parse tem quatro caminhos e usa
// aritmética de índice — recortes como v[9:] e v[1:37] só são seguros
// por causa do teste de comprimento que os precede. Rode com:
//
//	go test ./tests/ -run '^$' -fuzz FuzzParse -fuzztime 30s
func FuzzParse(f *testing.F) {
	f.Add(canonical)
	f.Add(strings.ToUpper(canonical))
	f.Add("{" + canonical + "}")
	f.Add("urn:uuid:" + canonical)
	f.Add("URN:UUID:" + canonical)
	f.Add(strings.ReplaceAll(canonical, "-", ""))
	f.Add("")
	f.Add("{")
	f.Add("}")
	f.Add("urn:uuid:")
	f.Add("{" + canonical)
	f.Add(canonical + "}")

	f.Fuzz(func(t *testing.T, s string) {
		// Parse jamais pode entrar em pânico, qualquer que seja a entrada.
		u, err := uuid.Parse(s)

		// ParseBytes precisa concordar com Parse em todos os casos.
		ub, errb := uuid.ParseBytes([]byte(s))
		if u != ub || (err == nil) != (errb == nil) {
			t.Fatalf("Parse e ParseBytes divergiram para %q: %v/%v e %v/%v", s, u, err, ub, errb)
		}

		// Validate precisa concordar com Parse quanto a aceitar ou recusar.
		if (uuid.Validate(s) == nil) != (err == nil) {
			t.Fatalf("Validate divergiu de Parse para %q", s)
		}

		if err != nil {
			if !u.IsZero() {
				t.Fatalf("Parse(%q) devolveu erro e um UUID não zerado: %v", s, u)
			}
			if !errors.Is(err, uuid.ErrInvalidFormat) {
				t.Fatalf("Parse(%q): erro %v não é reconhecível como ErrInvalidFormat", s, err)
			}
			return
		}

		// Se aceitou, as três reemissões precisam voltar ao mesmo valor.
		for name, text := range map[string]string{
			"String": u.String(),
			"URN":    u.URN(),
		} {
			again, err := uuid.Parse(text)
			if err != nil {
				t.Fatalf("round-trip por %s falhou para %q: %v", name, s, err)
			}
			if again != u {
				t.Fatalf("round-trip por %s divergiu para %q: %v vs %v", name, s, u, again)
			}
		}

		// A serialização em texto precisa ser lida de volta sem perda.
		text, err := u.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText falhou para %q: %v", s, err)
		}
		var back uuid.UUID
		if err := back.UnmarshalText(text); err != nil || back != u {
			t.Fatalf("volta pelo texto divergiu para %q: %v vs %v, erro %v", s, u, back, err)
		}
	})
}

// FuzzNullUUIDJSON procura entradas que façam NullUUID.UnmarshalJSON
// entrar em pânico ou divergir do tipo UUID lido pelo encoding/json.
// NullUUID tem um leitor de JSON próprio, com caminho direto sem escapes
// e delegação ao encoding/json quando há barra invertida; os dois
// caminhos precisam concordar com o que o encoding/json faria para um
// UUID comum. Rode com:
//
//	go test ./tests/ -run '^$' -fuzz FuzzNullUUIDJSON -fuzztime 30s
func FuzzNullUUIDJSON(f *testing.F) {
	f.Add([]byte(`"` + canonical + `"`))
	f.Add([]byte(`"{` + canonical + `}"`))
	f.Add([]byte(`"urn:uuid:` + canonical + `"`))
	f.Add([]byte(`"` + strings.ReplaceAll(canonical, "-", "") + `"`))
	f.Add([]byte(`"` + strings.ReplaceAll(canonical, "0", `\u0030`) + `"`))
	f.Add([]byte(`"` + strings.ReplaceAll(canonical, "-", `\u002d`) + `"`))
	f.Add([]byte(`"\x00"`))
	f.Add([]byte(`null`))
	f.Add([]byte(`""`))
	f.Add([]byte(`"`))
	f.Add([]byte(`123`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		// A chamada direta jamais pode entrar em pânico, e todo erro dela
		// precisa ser reconhecível como erro de formato.
		var direct uuid.NullUUID
		if err := direct.UnmarshalJSON(data); err != nil && !errors.Is(err, uuid.ErrInvalidFormat) {
			t.Fatalf("UnmarshalJSON(%q): erro %v não é reconhecível como ErrInvalidFormat", data, err)
		}

		// Pelo encoding/json, NullUUID e UUID precisam aceitar e recusar as
		// mesmas entradas e produzir o mesmo valor. O literal null é a
		// exceção: vira valor ausente em NullUUID e não toca um UUID.
		var viaNull uuid.NullUUID
		var viaUUID uuid.UUID
		errNull := json.Unmarshal(data, &viaNull)
		errUUID := json.Unmarshal(data, &viaUUID)
		if (errNull == nil) != (errUUID == nil) {
			t.Fatalf("NullUUID e UUID divergiram ao aceitar %q: %v vs %v", data, errNull, errUUID)
		}
		if errNull != nil {
			return
		}
		if viaNull.Valid {
			if viaNull.UUID != viaUUID {
				t.Fatalf("NullUUID e UUID divergiram no valor de %q: %s vs %s", data, viaNull.UUID, viaUUID)
			}
		} else if !viaUUID.IsZero() {
			t.Fatalf("NullUUID ausente mas UUID leu %s de %q", viaUUID, data)
		}
	})
}

// FuzzInstantArithmetic procura instantes que quebrem as invariantes da
// construção a partir de um instante explícito: as fronteiras da seção
// 3.5 e a geração da seção 3.6 da especificação.
//
// É o alvo que falta ao lado dos três de texto, e vale por um motivo
// diferente deles. Ali a entrada é uma string e o risco é leitura fora
// dos limites; aqui a entrada é um instante e o risco é aritmético:
// saturação nas duas pontas, o estouro da multiplicação por mil e a
// divisão que decide o piso na época. A suíte cobre esses pontos por
// tabela, com valores escolhidos à mão; o fuzzing varre a faixa inteira
// de segundos, incluindo as duas metades do inteiro com sinal, que é
// onde o estouro troca o sinal.
//
// Rode com:
//
//	go test ./tests/ -run '^$' -fuzz FuzzInstantArithmetic -fuzztime 30s
func FuzzInstantArithmetic(f *testing.F) {
	f.Add(int64(0), int64(0), int64(0), int64(0))
	f.Add(int64(1767225600), int64(123456789), int64(1767225600), int64(123456790))
	f.Add(int64(-1), int64(999999999), int64(0), int64(0))            // travessia da época
	f.Add(int64(1)<<48/1000, int64(0), int64(1)<<48/1000+1, int64(0)) // borda dos 48 bits
	f.Add(int64(1)<<62, int64(0), int64(1)<<62+1, int64(0))           // estouro da multiplicação
	f.Add(-(int64(1) << 62), int64(0), -(int64(1)<<62)+1, int64(0))   // estouro ao contrário
	f.Add(int64(math.MaxInt64/1000), int64(999999999), int64(math.MaxInt64/1000), int64(0))

	f.Fuzz(func(t *testing.T, sec1, nsec1, sec2, nsec2 int64) {
		// time.Unix normaliza a fração, então qualquer par de entrada
		// produz um instante válido; é justamente o que se quer varrer.
		t1 := time.Unix(sec1, nsec1)
		t2 := time.Unix(sec2, nsec2)

		for _, level := range []uuid.Level{uuid.Level1, uuid.Level2, uuid.Level3, uuid.Level(9)} {
			lo := uuid.MinAt(level, t1)
			hi := uuid.MaxAt(level, t1)

			// Versão e variante sobrevivem ao preenchimento, nas duas
			// pontas e em qualquer instante. É o que torna a fronteira um
			// limite correto.
			for nome, u := range map[string]uuid.UUID{"MinAt": lo, "MaxAt": hi} {
				if u.Version() != 7 {
					t.Fatalf("nível %d, %s(%v): versão %d, esperado 7", level, nome, t1, u.Version())
				}
				if u.Variant() != 0b10 {
					t.Fatalf("nível %d, %s(%v): variante %#b, esperado 10", level, nome, t1, u.Variant())
				}
			}

			// A fronteira inferior nunca passa da superior do mesmo instante.
			if lo.Compare(hi) > 0 {
				t.Fatalf("nível %d, instante %v: MinAt %v acima de MaxAt %v", level, t1, lo, hi)
			}

			// O valor gerado para o instante cai dentro das fronteiras dele.
			gerado := uuid.GenerateAt(level, t1)
			if gerado.Compare(lo) < 0 || gerado.Compare(hi) > 0 {
				t.Fatalf("nível %d, instante %v: GenerateAt %v fora de [%v, %v]", level, t1, gerado, lo, hi)
			}

			// Monotonicidade: instante que não regride não produz fronteira
			// que regride. É a propriedade que a saturação por truncamento
			// quebraria em silêncio, e a razão de a saturação levar micro e
			// nano a 999 no teto.
			if !t2.Before(t1) {
				if uuid.MinAt(level, t2).Compare(lo) < 0 {
					t.Fatalf("nível %d: instante %v não é anterior a %v, mas MinAt regrediu", level, t2, t1)
				}
				if uuid.MaxAt(level, t2).Compare(hi) < 0 {
					t.Fatalf("nível %d: instante %v não é anterior a %v, mas MaxAt regrediu", level, t2, t1)
				}
			}
		}

		// O intervalo semiaberto nunca tem o limite superior abaixo do
		// inferior quando os argumentos estão em ordem.
		if !t2.Before(t1) {
			lo, hi := uuid.RangeAt(uuid.Level3, t1, t2)
			if lo.Compare(hi) > 0 {
				t.Fatalf("RangeAt(%v, %v) devolveu intervalo invertido: %v acima de %v", t1, t2, lo, hi)
			}
		}
	})
}

// FuzzGregorianUnixTime procura instantes gregorianos cuja conversão para
// a época Unix devolva um par não canônico.
//
// O campo de tempo das versões 1 e 6 começa em 1582, portanto metade da
// faixa representável fica antes da época Unix, e é lá que a divisão
// truncada da linguagem devolveria resto negativo. A conversão usa
// divisão euclidiana exatamente por isso, e esta é a varredura que prova
// a propriedade em toda a faixa em vez de nos poucos pontos da tabela.
//
// Rode com:
//
//	go test ./tests/ -run '^$' -fuzz FuzzGregorianUnixTime -fuzztime 30s
func FuzzGregorianUnixTime(f *testing.F) {
	f.Add(int64(0))                      // 1582-10-15, início da época gregoriana
	f.Add(int64(122192928000000000))     // a própria época Unix
	f.Add(int64(122192928000000000 - 1)) // um tique antes da época Unix
	f.Add(int64(122192928000000000 + 1)) // um tique depois
	f.Add(int64(139865184001234567))     // instante da tabela de vetores
	f.Add(int64(1)<<60 - 1)              // teto dos 60 bits do campo
	f.Add(int64(math.MaxInt64))
	f.Add(int64(math.MinInt64))
	// Limiar exato de saturação e vizinhança.
	f.Add(int64(math.MinInt64) + int64(122192928000000000))     // minGregorianTicks
	f.Add(int64(math.MinInt64) + int64(122192928000000000) - 1) // um abaixo: satura
	f.Add(int64(math.MinInt64) + int64(122192928000000000) + 1) // um acima: não satura

	f.Fuzz(func(t *testing.T, ticks int64) {
		g := uuid.GregorianTime(ticks)
		sec, nsec := g.UnixTime()

		// O par devolvido é canônico em toda a faixa: a fração nunca é
		// negativa nem alcança um segundo inteiro.
		if nsec < 0 || nsec >= 1_000_000_000 {
			t.Fatalf("GregorianTime(%d).UnixTime() devolveu nsec %d, fora de 0..999999999", ticks, nsec)
		}

		// A resolução é a do campo, 100 ns: os dois dígitos finais da
		// fração são sempre zero.
		if nsec%100 != 0 {
			t.Fatalf("GregorianTime(%d).UnixTime() devolveu nsec %d, que não é múltiplo de 100", ticks, nsec)
		}

		// Detecção de estouro: se o ticks de entrada é negativo, sec
		// não pode ser positivo e astronômico. O limiar de 1e15 (~ano
		// 31.700.000+) está muito acima de qualquer instante legítimo.
		// Se sec ultrapassar isso com ticks negativo, a subtração de
		// gregorian100ns estourou int64 e girou para o futuro.
		if ticks < 0 && sec > 1e15 {
			t.Fatalf("GregorianTime(%d).UnixTime() devolveu sec %d: provável estouro de int64", ticks, sec)
		}

		// O instante construído com o par tem de coincidir com o que o
		// tipo devolve, o que só vale se o par for canônico.
		if got, esperado := g.Time(), time.Unix(sec, nsec).UTC(); !got.Equal(esperado) {
			t.Fatalf("GregorianTime(%d): Time() = %v, mas o par (%d, %d) dá %v", ticks, got, sec, nsec, esperado)
		}
	})
}
