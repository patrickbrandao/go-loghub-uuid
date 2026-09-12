package tests

import (
	"bytes"
	"encoding/json"
	"testing"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// ---------- BinaryUUID ----------

// TestBinaryUUIDMarshalTextIsCanonical confere que MarshalText devolve a
// string canônica de 36 bytes, exatamente como UUID.
func TestBinaryUUIDMarshalTextIsCanonical(t *testing.T) {
	u := uuid.MustParse(amostraCanonica)
	b := uuid.BinaryUUID(u)

	text, err := b.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText devolveu erro: %v", err)
	}
	if string(text) != amostraCanonica {
		t.Errorf("MarshalText devolveu %q, esperado %q", text, amostraCanonica)
	}
}

// TestBinaryUUIDUnmarshalTextRoundTrip prova a ida e volta pelo texto.
func TestBinaryUUIDUnmarshalTextRoundTrip(t *testing.T) {
	original := uuid.BinaryUUID(uuid.MustParse(amostraCanonica))

	text, err := original.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText devolveu erro: %v", err)
	}

	var lido uuid.BinaryUUID
	if err := lido.UnmarshalText(text); err != nil {
		t.Fatalf("UnmarshalText devolveu erro: %v", err)
	}
	if lido != original {
		t.Errorf("ida e volta devolveu %s, esperado %s", lido, original)
	}
}

// TestBinaryUUIDMarshalBinaryIs16Bytes confere que MarshalBinary devolve
// os 16 bytes crus.
func TestBinaryUUIDMarshalBinaryIs16Bytes(t *testing.T) {
	u := uuid.MustParse(amostraCanonica)
	b := uuid.BinaryUUID(u)

	bin, err := b.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary devolveu erro: %v", err)
	}
	if len(bin) != 16 {
		t.Fatalf("MarshalBinary devolveu %d bytes, esperado 16", len(bin))
	}
	if !bytes.Equal(bin, u.Bytes()) {
		t.Errorf("MarshalBinary devolveu %x, esperado %x", bin, u.Bytes())
	}
}

// TestBinaryUUIDUnmarshalBinaryRoundTrip prova a ida e volta pelo binário.
func TestBinaryUUIDUnmarshalBinaryRoundTrip(t *testing.T) {
	original := uuid.BinaryUUID(uuid.MustParse(amostraCanonica))

	bin, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary devolveu erro: %v", err)
	}

	var lido uuid.BinaryUUID
	if err := lido.UnmarshalBinary(bin); err != nil {
		t.Fatalf("UnmarshalBinary devolveu erro: %v", err)
	}
	if lido != original {
		t.Errorf("ida e volta devolveu %s, esperado %s", lido, original)
	}
}

// TestBinaryUUIDJSONIsCanonicalString é o teste mais importante: sem
// MarshalText, encoding/json gravaria um vetor de 16 números. Com o
// método presente, deve gravar a string canônica entre aspas.
func TestBinaryUUIDJSONIsCanonicalString(t *testing.T) {
	u := uuid.MustParse(amostraCanonica)
	b := uuid.BinaryUUID(u)

	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("json.Marshal devolveu erro: %v", err)
	}

	esperado := `"` + amostraCanonica + `"`
	if string(data) != esperado {
		t.Errorf("json.Marshal devolveu %s, esperado %s", data, esperado)
	}

	// Ida e volta pelo JSON.
	var lido uuid.BinaryUUID
	if err := json.Unmarshal(data, &lido); err != nil {
		t.Fatalf("json.Unmarshal devolveu erro: %v", err)
	}
	if lido != b {
		t.Errorf("ida e volta JSON devolveu %s, esperado %s", lido, b)
	}
}

// ---------- NullBinaryUUID — JSON ----------

// TestNullBinaryUUIDJSONWithValue confere que um valor presente sai como
// string canônica entre aspas, sem o envelope {"UUID":"...","Valid":true}.
func TestNullBinaryUUIDJSONWithValue(t *testing.T) {
	n := uuid.NullBinaryUUID{UUID: uuid.MustParse(amostraCanonica), Valid: true}

	data, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("json.Marshal devolveu erro: %v", err)
	}

	esperado := `"` + amostraCanonica + `"`
	if string(data) != esperado {
		t.Errorf("json.Marshal devolveu %s, esperado %s", data, esperado)
	}
}

// TestNullBinaryUUIDJSONAbsent confere que ausência produz o literal
// null, conforme a tabela normativa da SPEC §8.
func TestNullBinaryUUIDJSONAbsent(t *testing.T) {
	n := uuid.NullBinaryUUID{Valid: false}

	data, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("json.Marshal devolveu erro: %v", err)
	}

	if string(data) != "null" {
		t.Errorf("json.Marshal de ausente devolveu %s, esperado null", data)
	}
}

// TestNullBinaryUUIDUnmarshalJSONNull confere que ler null produz
// Valid=false.
func TestNullBinaryUUIDUnmarshalJSONNull(t *testing.T) {
	var n uuid.NullBinaryUUID
	if err := json.Unmarshal([]byte("null"), &n); err != nil {
		t.Fatalf("json.Unmarshal devolveu erro: %v", err)
	}
	if n.Valid {
		t.Error("após ler null, Valid deveria ser falso")
	}
	if n.UUID != uuid.Nil {
		t.Errorf("após ler null, UUID deveria ser Nil, obteve %s", n.UUID)
	}
}

// TestNullBinaryUUIDUnmarshalJSONValue confere que ler uma string
// canônica produz Valid=true com o UUID correto.
func TestNullBinaryUUIDUnmarshalJSONValue(t *testing.T) {
	entrada := `"` + amostraCanonica + `"`
	var n uuid.NullBinaryUUID
	if err := json.Unmarshal([]byte(entrada), &n); err != nil {
		t.Fatalf("json.Unmarshal devolveu erro: %v", err)
	}
	if !n.Valid {
		t.Error("após ler string, Valid deveria ser verdadeiro")
	}
	if n.UUID != uuid.MustParse(amostraCanonica) {
		t.Errorf("UUID lido %s, esperado %s", n.UUID, amostraCanonica)
	}
}

// ---------- NullBinaryUUID — Texto ----------

// TestNullBinaryUUIDMarshalTextWithValue confere a string canônica.
func TestNullBinaryUUIDMarshalTextWithValue(t *testing.T) {
	n := uuid.NullBinaryUUID{UUID: uuid.MustParse(amostraCanonica), Valid: true}

	text, err := n.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText devolveu erro: %v", err)
	}
	if string(text) != amostraCanonica {
		t.Errorf("MarshalText devolveu %q, esperado %q", text, amostraCanonica)
	}
}

// TestNullBinaryUUIDMarshalTextAbsent confere sequência vazia.
func TestNullBinaryUUIDMarshalTextAbsent(t *testing.T) {
	n := uuid.NullBinaryUUID{Valid: false}

	text, err := n.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText devolveu erro: %v", err)
	}
	if len(text) != 0 {
		t.Errorf("MarshalText de ausente devolveu %q, esperado vazio", text)
	}
}

// TestNullBinaryUUIDUnmarshalTextEmpty confere que entrada vazia produz
// ausência.
func TestNullBinaryUUIDUnmarshalTextEmpty(t *testing.T) {
	var n uuid.NullBinaryUUID
	if err := n.UnmarshalText([]byte{}); err != nil {
		t.Fatalf("UnmarshalText devolveu erro: %v", err)
	}
	if n.Valid {
		t.Error("após ler vazio, Valid deveria ser falso")
	}
}

// TestNullBinaryUUIDUnmarshalTextValue confere ida e volta pelo texto.
func TestNullBinaryUUIDUnmarshalTextValue(t *testing.T) {
	var n uuid.NullBinaryUUID
	if err := n.UnmarshalText([]byte(amostraCanonica)); err != nil {
		t.Fatalf("UnmarshalText devolveu erro: %v", err)
	}
	if !n.Valid {
		t.Error("após ler texto, Valid deveria ser verdadeiro")
	}
	if n.UUID != uuid.MustParse(amostraCanonica) {
		t.Errorf("UUID lido %s, esperado %s", n.UUID, amostraCanonica)
	}
}

// ---------- NullBinaryUUID — Binário ----------

// TestNullBinaryUUIDMarshalBinaryWithValue confere os 16 bytes.
func TestNullBinaryUUIDMarshalBinaryWithValue(t *testing.T) {
	u := uuid.MustParse(amostraCanonica)
	n := uuid.NullBinaryUUID{UUID: u, Valid: true}

	bin, err := n.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary devolveu erro: %v", err)
	}
	if len(bin) != 16 {
		t.Fatalf("MarshalBinary devolveu %d bytes, esperado 16", len(bin))
	}
	if !bytes.Equal(bin, u.Bytes()) {
		t.Errorf("MarshalBinary devolveu %x, esperado %x", bin, u.Bytes())
	}
}

// TestNullBinaryUUIDMarshalBinaryAbsent confere sequência vazia.
func TestNullBinaryUUIDMarshalBinaryAbsent(t *testing.T) {
	n := uuid.NullBinaryUUID{Valid: false}

	bin, err := n.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary devolveu erro: %v", err)
	}
	if len(bin) != 0 {
		t.Errorf("MarshalBinary de ausente devolveu %d bytes, esperado 0", len(bin))
	}
}

// TestNullBinaryUUIDUnmarshalBinaryEmpty confere que entrada vazia
// produz ausência.
func TestNullBinaryUUIDUnmarshalBinaryEmpty(t *testing.T) {
	var n uuid.NullBinaryUUID
	if err := n.UnmarshalBinary([]byte{}); err != nil {
		t.Fatalf("UnmarshalBinary devolveu erro: %v", err)
	}
	if n.Valid {
		t.Error("após ler vazio, Valid deveria ser falso")
	}
}

// TestNullBinaryUUIDUnmarshalBinaryValue confere ida e volta pelo
// binário.
func TestNullBinaryUUIDUnmarshalBinaryValue(t *testing.T) {
	u := uuid.MustParse(amostraCanonica)
	original := uuid.NullBinaryUUID{UUID: u, Valid: true}

	bin, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary devolveu erro: %v", err)
	}

	var lido uuid.NullBinaryUUID
	if err := lido.UnmarshalBinary(bin); err != nil {
		t.Fatalf("UnmarshalBinary devolveu erro: %v", err)
	}
	if !lido.Valid {
		t.Error("após ler binário, Valid deveria ser verdadeiro")
	}
	if lido.UUID != original.UUID {
		t.Errorf("UUID lido %s, esperado %s", lido.UUID, original.UUID)
	}
}

// ---------- JSON em struct ----------

// TestNullBinaryUUIDInStructJSON confere o caso de uso real: o tipo
// dentro de uma struct serializada como JSON. Sem os métodos, o
// encoding/json produziria {"campo":{"UUID":"...","Valid":true}}.
func TestNullBinaryUUIDInStructJSON(t *testing.T) {
	type registro struct {
		ID uuid.NullBinaryUUID `json:"id"`
	}

	t.Run("presente", func(t *testing.T) {
		r := registro{ID: uuid.NullBinaryUUID{UUID: uuid.MustParse(amostraCanonica), Valid: true}}
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("json.Marshal devolveu erro: %v", err)
		}
		esperado := `{"id":"` + amostraCanonica + `"}`
		if string(data) != esperado {
			t.Errorf("json.Marshal devolveu %s, esperado %s", data, esperado)
		}
	})

	t.Run("ausente", func(t *testing.T) {
		r := registro{ID: uuid.NullBinaryUUID{Valid: false}}
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("json.Marshal devolveu erro: %v", err)
		}
		esperado := `{"id":null}`
		if string(data) != esperado {
			t.Errorf("json.Marshal devolveu %s, esperado %s", data, esperado)
		}
	})

	t.Run("roundtrip", func(t *testing.T) {
		original := registro{ID: uuid.NullBinaryUUID{UUID: uuid.MustParse(amostraCanonica), Valid: true}}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("json.Marshal devolveu erro: %v", err)
		}
		var lido registro
		if err := json.Unmarshal(data, &lido); err != nil {
			t.Fatalf("json.Unmarshal devolveu erro: %v", err)
		}
		if lido.ID.UUID != original.ID.UUID || lido.ID.Valid != original.ID.Valid {
			t.Errorf("ida e volta devolveu %+v, esperado %+v", lido.ID, original.ID)
		}
	})
}
