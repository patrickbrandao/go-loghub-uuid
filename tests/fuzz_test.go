package tests

import (
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
