package tests

import (
	"database/sql/driver"
	"testing"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// amostraCanonica é um UUIDv7 fixo usado nos testes de gravação binária.
const amostraCanonica = "0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff"

// TestBinaryUUIDValueIsSixteenBytes confere o que o tipo existe para
// fazer: entregar ao banco os 16 bytes crus, e não os 36 caracteres da
// forma canônica.
func TestBinaryUUIDValueIsSixteenBytes(t *testing.T) {
	u := uuid.MustParse(amostraCanonica)

	v, err := uuid.BinaryUUID(u).Value()
	if err != nil {
		t.Fatalf("Value devolveu erro: %v", err)
	}

	bytes, ok := v.([]byte)
	if !ok {
		t.Fatalf("Value devolveu %T, esperado []byte", v)
	}
	if len(bytes) != 16 {
		t.Fatalf("Value devolveu %d bytes, esperado 16", len(bytes))
	}
	if string(bytes) != string(u.Bytes()) {
		t.Errorf("Value devolveu %x, esperado %x", bytes, u.Bytes())
	}

	// O tipo original continua gravando texto: mudar isso quebraria em
	// silêncio quem já tem texto na coluna.
	texto, err := u.Value()
	if err != nil {
		t.Fatalf("Value de UUID devolveu erro: %v", err)
	}
	if s, ok := texto.(string); !ok || s != amostraCanonica {
		t.Errorf("Value de UUID devolveu %#v, esperado a string canônica", texto)
	}
}

// TestBinaryUUIDValueCopiesTheBytes confere que a fatia devolvida é
// independente do valor de origem. O driver pode retê-la depois do
// retorno, e devolver uma fatia apoiada no próprio UUID deixaria o
// chamador capaz de alterar o que já foi entregue.
func TestBinaryUUIDValueCopiesTheBytes(t *testing.T) {
	u := uuid.MustParse(amostraCanonica)
	v, err := uuid.BinaryUUID(u).Value()
	if err != nil {
		t.Fatalf("Value devolveu erro: %v", err)
	}
	saida := v.([]byte)
	saida[0] ^= 0xFF

	if saida[0] == u[0] {
		t.Error("alterar a fatia devolvida alterou o UUID de origem")
	}
}

// TestBinaryUUIDRoundTrip prova que o que se grava se lê de volta: os
// 16 bytes entregues por Value voltam pelo Scan como o mesmo UUID.
func TestBinaryUUIDRoundTrip(t *testing.T) {
	casos := map[string]uuid.UUID{
		"UUIDv7":      uuid.MustParse(amostraCanonica),
		"nulo":        uuid.Nil,
		"máximo":      uuid.Max,
		"gerado":      uuid.Generate(uuid.Level3),
		"fronteira":   uuid.MinAt(uuid.Level3, baseInstant),
		"fronteira 2": uuid.MaxAt(uuid.Level1, baseInstant),
	}
	for nome, original := range casos {
		v, err := uuid.BinaryUUID(original).Value()
		if err != nil {
			t.Fatalf("%s: Value devolveu erro: %v", nome, err)
		}

		var lido uuid.BinaryUUID
		if err := lido.Scan(v); err != nil {
			t.Fatalf("%s: Scan devolveu erro: %v", nome, err)
		}
		if uuid.UUID(lido) != original {
			t.Errorf("%s: ida e volta devolveu %s, esperado %s", nome, lido, original)
		}

		// O Scan do tipo binário aceita texto do mesmo jeito: o tipo
		// existe para a escrita, não para restringir a leitura.
		var deTexto uuid.BinaryUUID
		if err := deTexto.Scan(original.String()); err != nil {
			t.Fatalf("%s: Scan de texto devolveu erro: %v", nome, err)
		}
		if uuid.UUID(deTexto) != original {
			t.Errorf("%s: Scan de texto devolveu %s, esperado %s", nome, deTexto, original)
		}
	}
}

// TestNullBinaryUUIDDistinguishesAbsentFromNil é a armadilha principal
// do tipo: ausência de valor e UUID nulo são coisas diferentes e viram a
// mesma linha se alguém confundir. Valor ausente tem de produzir NULL,
// não dezesseis bytes zerados.
func TestNullBinaryUUIDDistinguishesAbsentFromNil(t *testing.T) {
	ausente := uuid.NullBinaryUUID{Valid: false}
	v, err := ausente.Value()
	if err != nil {
		t.Fatalf("valor ausente: Value devolveu erro: %v", err)
	}
	if v != nil {
		t.Errorf("valor ausente: Value devolveu %#v, esperado NULL", v)
	}

	nuloPresente := uuid.NullBinaryUUID{UUID: uuid.Nil, Valid: true}
	v, err = nuloPresente.Value()
	if err != nil {
		t.Fatalf("UUID nulo presente: Value devolveu erro: %v", err)
	}
	bytes, ok := v.([]byte)
	if !ok {
		t.Fatalf("UUID nulo presente: Value devolveu %T, esperado []byte", v)
	}
	if len(bytes) != 16 {
		t.Fatalf("UUID nulo presente: Value devolveu %d bytes, esperado 16", len(bytes))
	}
	for i, b := range bytes {
		if b != 0 {
			t.Errorf("UUID nulo presente: byte %d = %#x, esperado 0", i, b)
		}
	}
}

// TestNullBinaryUUIDScan confere que a leitura trata ausência do mesmo
// jeito que NullUUID: NULL, texto vazio e fatia vazia produzem valor
// ausente sem erro.
func TestNullBinaryUUIDScan(t *testing.T) {
	ausentes := []any{nil, "", []byte{}}
	for _, src := range ausentes {
		var n uuid.NullBinaryUUID
		if err := n.Scan(src); err != nil {
			t.Fatalf("Scan(%#v) devolveu erro: %v", src, err)
		}
		if n.Valid {
			t.Errorf("Scan(%#v): Valid verdadeiro, esperado falso", src)
		}
		if n.UUID != uuid.Nil {
			t.Errorf("Scan(%#v): UUID %s, esperado o nulo", src, n.UUID)
		}
	}

	original := uuid.MustParse(amostraCanonica)
	for nome, src := range map[string]any{
		"16 bytes crus": original.Bytes(),
		"texto":         original.String(),
	} {
		var n uuid.NullBinaryUUID
		if err := n.Scan(src); err != nil {
			t.Fatalf("%s: Scan devolveu erro: %v", nome, err)
		}
		if !n.Valid {
			t.Errorf("%s: Valid falso, esperado verdadeiro", nome)
		}
		if n.UUID != original {
			t.Errorf("%s: leu %s, esperado %s", nome, n.UUID, original)
		}
	}
}

// TestBinaryTypesSatisfyDatabaseInterfaces confere em tempo de
// compilação e de execução que os dois tipos servem onde o database/sql
// espera, que é o propósito inteiro deles.
func TestBinaryTypesSatisfyDatabaseInterfaces(t *testing.T) {
	var _ driver.Valuer = uuid.BinaryUUID{}
	var _ driver.Valuer = uuid.NullBinaryUUID{}

	valores := []driver.Valuer{
		uuid.BinaryUUID(uuid.MustParse(amostraCanonica)),
		uuid.NullBinaryUUID{UUID: uuid.Max, Valid: true},
	}
	for i, v := range valores {
		got, err := v.Value()
		if err != nil {
			t.Fatalf("valor %d: %v", i, err)
		}
		if _, err := driver.DefaultParameterConverter.ConvertValue(got); err != nil {
			t.Errorf("valor %d: o driver padrão recusou %#v: %v", i, got, err)
		}
	}
}

// TestBinaryUUIDString confere que o tipo continua legível em log e em
// mensagem de erro, em vez de aparecer como vetor de bytes.
func TestBinaryUUIDString(t *testing.T) {
	u := uuid.MustParse(amostraCanonica)
	if got := uuid.BinaryUUID(u).String(); got != amostraCanonica {
		t.Errorf("String devolveu %s, esperado %s", got, amostraCanonica)
	}
}
