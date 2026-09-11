package tests

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

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
