package tests

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // exigido pela RFC 9562 para a versão 5
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"testing"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// TestParseAcceptsFourFormats confere que Parse aceita as quatro formas
// usuais de escrever um UUID, todas resultando no mesmo valor.
func TestParseAcceptsFourFormats(t *testing.T) {
	reference, err := uuid.FromString(canonical)
	if err != nil {
		t.Fatalf("FromString rejeitou a referência: %v", err)
	}

	compact := ""
	for _, c := range canonical {
		if c != '-' {
			compact += string(c)
		}
	}

	cases := map[string]string{
		"canônico":        canonical,
		"maiúsculas":      "0192F7C5-1A2B-7C3D-8E4F-AABBCCDDEEFF",
		"entre chaves":    "{" + canonical + "}",
		"URN":             "urn:uuid:" + canonical,
		"URN maiúsculo":   "URN:UUID:" + canonical,
		"hexadecimal cru": compact,
	}
	for name, input := range cases {
		got, err := uuid.Parse(input)
		if err != nil {
			t.Errorf("Parse(%s): erro inesperado %v", name, err)
			continue
		}
		if got != reference {
			t.Errorf("Parse(%s): %s, esperado %s", name, got, reference)
		}
		if got, err := uuid.ParseBytes([]byte(input)); err != nil || got != reference {
			t.Errorf("ParseBytes(%s): %s, erro %v", name, got, err)
		}
		if err := uuid.Validate(input); err != nil {
			t.Errorf("Validate(%s): erro inesperado %v", name, err)
		}
	}
}

// TestParseRejectsMalformedInput confere as recusas de Parse e garante
// que todo erro continua reconhecível como ErrInvalidFormat.
func TestParseRejectsMalformedInput(t *testing.T) {
	invalid := []string{
		"",
		"nao-e-um-uuid",
		canonical + "0",
		canonical[:35],
		"{" + canonical,
		"urn:uiid:" + canonical,
		"0192f7c5-1a2b-7c3d-8e4f-aabbccddeeg f",
		"0192f7c51a2b7c3d8e4faabbccddeeg0",
	}
	for _, input := range invalid {
		got, err := uuid.Parse(input)
		if err == nil {
			t.Errorf("Parse(%q) deveria falhar", input)
			continue
		}
		if !errors.Is(err, uuid.ErrInvalidFormat) {
			t.Errorf("Parse(%q): erro %v não é reconhecível como ErrInvalidFormat", input, err)
		}
		if !got.IsZero() {
			t.Errorf("Parse(%q) devolveu %s em vez do UUID nulo", input, got)
		}
	}
}

// TestInvalidLengthIsDistinguishable confere que o erro de comprimento é
// identificável sem deixar de ser um erro de formato.
func TestInvalidLengthIsDistinguishable(t *testing.T) {
	_, err := uuid.Parse("abc")
	if !uuid.IsInvalidLengthError(err) {
		t.Errorf("erro de comprimento não reconhecido: %v", err)
	}
	if !errors.Is(err, uuid.ErrInvalidFormat) {
		t.Error("o erro de comprimento deveria continuar sendo um erro de formato")
	}

	_, err = uuid.Parse("0192f7c5-1a2b-7c3d-8e4f-aabbccddeezz")
	if uuid.IsInvalidLengthError(err) {
		t.Error("dígito inválido foi classificado como erro de comprimento")
	}

	// Como no pacote github.com/google/uuid, o reconhecimento atravessa
	// camadas que embrulham o erro com %w.
	_, err = uuid.Parse("abc")
	wrapped := fmt.Errorf("lendo identificador: %w", err)
	if !uuid.IsInvalidLengthError(wrapped) {
		t.Errorf("erro de comprimento embrulhado não reconhecido: %v", wrapped)
	}
	if uuid.IsInvalidLengthError(nil) || uuid.IsInvalidLengthError(errors.New("outro")) {
		t.Error("IsInvalidLengthError aceitou um erro que não é de comprimento")
	}
}

// TestFromStringErrorUnchanged trava a compatibilidade do contrato
// anterior: FromString continua devolvendo exatamente ErrInvalidFormat,
// inclusive para quem compara com o operador de igualdade em vez de
// errors.Is.
func TestFromStringErrorUnchanged(t *testing.T) {
	for _, input := range []string{"", "curto", canonical + "x", "0192f7c5+1a2b-7c3d-8e4f-aabbccddeeff"} {
		if _, err := uuid.FromString(input); err != uuid.ErrInvalidFormat { //nolint:errorlint // a comparação direta é o próprio contrato testado
			t.Errorf("FromString(%q): erro %v, esperado exatamente ErrInvalidFormat", input, err)
		}
	}
}

// TestFromStringStillRejectsExtendedFormats confere que a permissividade
// nova ficou restrita a Parse: FromString continua aceitando só a forma
// canônica, como sempre aceitou.
func TestFromStringStillRejectsExtendedFormats(t *testing.T) {
	for _, input := range []string{"{" + canonical + "}", "urn:uuid:" + canonical} {
		if _, err := uuid.FromString(input); err == nil {
			t.Errorf("FromString(%q) deveria continuar recusando", input)
		}
	}
}

// TestFromBytes confere a construção a partir de 16 bytes crus.
func TestFromBytes(t *testing.T) {
	reference := uuid.Generate(uuid.Level3)

	got, err := uuid.FromBytes(reference[:])
	if err != nil || got != reference {
		t.Errorf("FromBytes: %s, erro %v", got, err)
	}
	if _, err := uuid.FromBytes(reference[:15]); !uuid.IsInvalidLengthError(err) {
		t.Errorf("FromBytes com 15 bytes: erro %v, esperado erro de comprimento", err)
	}
}

// TestMustParseAndMust confere as variantes que entram em pânico.
func TestMustParseAndMust(t *testing.T) {
	if uuid.MustParse(canonical).String() != canonical {
		t.Error("MustParse não devolveu o valor esperado")
	}
	if uuid.Must(uuid.Parse(canonical)).String() != canonical {
		t.Error("Must não devolveu o valor esperado")
	}

	defer func() {
		if recover() == nil {
			t.Error("MustParse deveria entrar em pânico com entrada inválida")
		}
	}()
	uuid.MustParse("nao-e-um-uuid")
}

// TestNilAndMax confere os dois valores especiais da RFC 9562.
func TestNilAndMax(t *testing.T) {
	if uuid.Nil.String() != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("Nil: %s", uuid.Nil)
	}
	if uuid.Max.String() != "ffffffff-ffff-ffff-ffff-ffffffffffff" {
		t.Errorf("Max: %s", uuid.Max)
	}
	if !uuid.Nil.IsZero() || uuid.Nil.IsMax() {
		t.Error("predicados errados para Nil")
	}
	if !uuid.Max.IsMax() || uuid.Max.IsZero() {
		t.Error("predicados errados para Max")
	}
	if uuid.Generate(uuid.Level1).IsZero() {
		t.Error("um UUID gerado nunca deveria ser nulo")
	}
}

// TestCompareOrdersLikeStrings confere que Compare concorda com a ordem
// lexicográfica das strings canônicas.
func TestCompareOrdersLikeStrings(t *testing.T) {
	if uuid.Nil.Compare(uuid.Max) != -1 {
		t.Error("Nil deveria vir antes de Max")
	}
	if uuid.Max.Compare(uuid.Nil) != 1 {
		t.Error("Max deveria vir depois de Nil")
	}
	if uuid.Max.Compare(uuid.Max) != 0 {
		t.Error("Compare de um valor com ele mesmo deveria ser zero")
	}

	previous := uuid.GenerateV6()
	for i := 0; i < 1_000; i++ {
		current := uuid.GenerateV6()
		byBytes := previous.Compare(current)
		byText := 0
		switch {
		case previous.String() < current.String():
			byText = -1
		case previous.String() > current.String():
			byText = 1
		}
		if byBytes != byText {
			t.Fatalf("divergência na comparação %d: bytes %d, texto %d", i, byBytes, byText)
		}
		previous = current
	}
}

// TestURN confere a forma URN e o caminho de volta por Parse.
func TestURN(t *testing.T) {
	u := uuid.MustParse(canonical)
	urn := u.URN()
	if urn != "urn:uuid:"+canonical {
		t.Errorf("URN: %s", urn)
	}
	if back, err := uuid.Parse(urn); err != nil || back != u {
		t.Errorf("volta pela URN: %s, erro %v", back, err)
	}
}

// TestUUIDsStrings confere a conveniência de lista.
func TestUUIDsStrings(t *testing.T) {
	list := uuid.UUIDs{uuid.Nil, uuid.Max}
	got := list.Strings()
	if len(got) != 2 || got[0] != uuid.Nil.String() || got[1] != uuid.Max.String() {
		t.Errorf("Strings: %v", got)
	}
	if len(uuid.UUIDs(nil).Strings()) != 0 {
		t.Error("lista vazia deveria produzir fatia vazia")
	}
}

// TestVersionAndVariantStrings confere as descrições em texto.
func TestVersionAndVariantStrings(t *testing.T) {
	if got := uuid.VersionString(uuid.Generate(uuid.Level1).Version()); got != "versao 7" {
		t.Errorf("VersionString: %s", got)
	}
	if got := uuid.VariantString(uuid.Generate(uuid.Level1).Variant()); got != "RFC 9562" {
		t.Errorf("VariantString: %s", got)
	}
	if got := uuid.VersionString(15); got == "versao 15" {
		t.Error("VersionString deveria marcar versões fora da faixa como desconhecidas")
	}
}

// TestTextMarshaling confere as interfaces de texto da biblioteca padrão.
func TestTextMarshaling(t *testing.T) {
	u := uuid.MustParse(canonical)

	text, err := u.MarshalText()
	if err != nil || string(text) != canonical {
		t.Fatalf("MarshalText: %q, erro %v", text, err)
	}

	var back uuid.UUID
	if err := back.UnmarshalText(text); err != nil || back != u {
		t.Errorf("UnmarshalText: %s, erro %v", back, err)
	}
	// A leitura de texto aceita os mesmos formatos de Parse.
	if err := back.UnmarshalText([]byte("{" + canonical + "}")); err != nil || back != u {
		t.Errorf("UnmarshalText entre chaves: %s, erro %v", back, err)
	}

	original := back
	if err := back.UnmarshalText([]byte("invalido")); err == nil {
		t.Error("UnmarshalText deveria recusar entrada inválida")
	} else if back != original {
		t.Error("UnmarshalText alterou o receptor mesmo falhando")
	}
}

// TestAppendTo confere a escrita no buffer do chamador: o conteúdo
// anexado, a preservação do que já estava em dst, o buffer nulo e a
// reutilização com dst[:0].
func TestAppendTo(t *testing.T) {
	u := uuid.MustParse(canonical)

	if got := string(u.AppendTo(nil)); got != canonical {
		t.Errorf("AppendTo(nil) = %q, esperado %q", got, canonical)
	}

	prefixo := []byte("id=")
	if got := string(u.AppendTo(prefixo)); got != "id="+canonical {
		t.Errorf("AppendTo sobre prefixo = %q, esperado %q", got, "id="+canonical)
	}
	if string(prefixo) != "id=" {
		t.Errorf("AppendTo alterou o comprimento de dst: %q", prefixo)
	}

	// Reutilizar o mesmo buffer é o uso previsto pela documentação.
	buf := make([]byte, 0, 64)
	for _, esperado := range []uuid.UUID{u, uuid.Nil, uuid.Max} {
		buf = esperado.AppendTo(buf[:0])
		if string(buf) != esperado.String() {
			t.Errorf("AppendTo reutilizando o buffer = %q, esperado %q", buf, esperado)
		}
	}

	// AppendText é a mesma escrita com a assinatura de encoding.TextAppender.
	texto, err := u.AppendText([]byte("x"))
	if err != nil || string(texto) != "x"+canonical {
		t.Errorf("AppendText = %q, erro %v", texto, err)
	}
}

// TestAppendBinary confere a escrita dos 16 bytes crus no fim do buffer
// do chamador, que é a assinatura de encoding.BinaryAppender. O conteúdo
// tem de ser idêntico ao de MarshalBinary: se divergirem, um dos dois
// caminhos de serialização binária está errado.
func TestAppendBinary(t *testing.T) {
	u := uuid.MustParse(canonical)

	cru, err := u.AppendBinary(nil)
	if err != nil {
		t.Fatalf("AppendBinary(nil) devolveu erro %v, esperado nulo", err)
	}
	if !bytes.Equal(cru, u[:]) {
		t.Errorf("AppendBinary(nil) = %x, esperado %x", cru, u[:])
	}

	marshalado, err := u.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary devolveu erro %v", err)
	}
	if !bytes.Equal(cru, marshalado) {
		t.Errorf("AppendBinary = %x, mas MarshalBinary = %x; os dois têm de coincidir", cru, marshalado)
	}

	prefixo := []byte{0xAA}
	comPrefixo, err := u.AppendBinary(prefixo)
	if err != nil {
		t.Fatalf("AppendBinary sobre prefixo devolveu erro %v", err)
	}
	if !bytes.Equal(comPrefixo, append([]byte{0xAA}, u[:]...)) {
		t.Errorf("AppendBinary sobre prefixo = %x", comPrefixo)
	}
	if len(prefixo) != 1 {
		t.Errorf("AppendBinary alterou o comprimento de dst: %x", prefixo)
	}

	// A ida e volta fecha com UnmarshalBinary, e os valores especiais
	// entram porque são os que não carregam versão nem variante.
	for _, esperado := range []uuid.UUID{u, uuid.Nil, uuid.Max} {
		buf, err := esperado.AppendBinary(make([]byte, 0, 16))
		if err != nil {
			t.Fatalf("%v: AppendBinary devolveu erro %v", esperado, err)
		}
		var volta uuid.UUID
		if err := volta.UnmarshalBinary(buf); err != nil {
			t.Fatalf("%v: UnmarshalBinary devolveu erro %v", esperado, err)
		}
		if volta != esperado {
			t.Errorf("ida e volta binária = %v, esperado %v", volta, esperado)
		}
	}
}

// TestBytes confere que Bytes devolve uma cópia independente dos 16
// bytes, ao contrário de u[:].
func TestBytes(t *testing.T) {
	u := uuid.MustParse(canonical)

	raw := u.Bytes()
	if len(raw) != 16 {
		t.Fatalf("Bytes devolveu %d bytes, esperado 16", len(raw))
	}
	if uuid.UUID(raw[0:16]) != u {
		t.Errorf("Bytes = %x, esperado %x", raw, u[:])
	}

	raw[0] = 0xFF
	if u[0] == 0xFF {
		t.Error("Bytes devolveu uma fatia sobre o próprio valor de origem")
	}
}

// TestIsValid confere a semântica adotada: variante RFC e versão de 1 a
// 8, com Nil e Max aceitos como os valores especiais que a RFC 9562
// define.
func TestIsValid(t *testing.T) {
	validos := map[string]uuid.UUID{
		"nulo":     uuid.Nil,
		"maximo":   uuid.Max,
		"versao 1": uuid.GenerateV1(),
		"versao 2": uuid.GenerateV2(uuid.Org, 1),
		"versao 3": uuid.GenerateV3(uuid.NameSpaceDNS, []byte("x")),
		"versao 4": uuid.GenerateV4(),
		"versao 5": uuid.GenerateV5(uuid.NameSpaceDNS, []byte("x")),
		"versao 6": uuid.GenerateV6(),
		"versao 7": uuid.Generate(uuid.Level3),
		"versao 8": uuid.GenerateV8Random(),
		"canonico": uuid.MustParse(canonical),
	}
	for nome, u := range validos {
		if !u.IsValid() {
			t.Errorf("%s: IsValid devolveu falso para %s", nome, u)
		}
	}

	base := uuid.MustParse(canonical)

	semVersao := base
	semVersao[6] &= 0x0F // versão 0
	if semVersao.IsValid() {
		t.Error("IsValid deveria recusar versão 0")
	}

	versaoNove := base
	versaoNove[6] = (versaoNove[6] & 0x0F) | 0x90
	if versaoNove.IsValid() {
		t.Error("IsValid deveria recusar versão 9")
	}

	for _, variante := range []byte{0x00, 0x40, 0xC0, 0xE0} {
		naoRFC := base
		naoRFC[8] = (naoRFC[8] & 0x3F) | variante
		if naoRFC.IsValid() {
			t.Errorf("IsValid deveria recusar a variante %#02x (codigo %d)", variante, naoRFC.Variant())
		}
	}
}

// TestBinaryMarshaling confere as interfaces binárias da biblioteca
// padrão, inclusive a garantia de que o resultado não aponta para o valor
// de origem.
func TestBinaryMarshaling(t *testing.T) {
	u := uuid.MustParse(canonical)

	raw, err := u.MarshalBinary()
	if err != nil || len(raw) != 16 {
		t.Fatalf("MarshalBinary: %d bytes, erro %v", len(raw), err)
	}
	raw[0] = 0xFF
	if u[0] == 0xFF {
		t.Error("MarshalBinary devolveu uma fatia sobre o próprio valor de origem")
	}

	raw, _ = u.MarshalBinary()
	var back uuid.UUID
	if err := back.UnmarshalBinary(raw); err != nil || back != u {
		t.Errorf("UnmarshalBinary: %s, erro %v", back, err)
	}
	if err := back.UnmarshalBinary(raw[:10]); !uuid.IsInvalidLengthError(err) {
		t.Errorf("UnmarshalBinary com 10 bytes: erro %v, esperado erro de comprimento", err)
	}
}

// TestJSONRoundTrip confere que o UUID agora viaja como string em JSON.
func TestJSONRoundTrip(t *testing.T) {
	type record struct {
		ID uuid.UUID `json:"id"`
	}
	original := record{ID: uuid.MustParse(canonical)}

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(encoded) != `{"id":"`+canonical+`"}` {
		t.Errorf("JSON gerado: %s", encoded)
	}

	var back record
	if err := json.Unmarshal(encoded, &back); err != nil || back != original {
		t.Errorf("json.Unmarshal: %+v, erro %v", back, err)
	}
}

// TestSQLScanAndValue confere a integração com database/sql.
func TestSQLScanAndValue(t *testing.T) {
	reference := uuid.MustParse(canonical)

	value, err := reference.Value()
	if err != nil || value != canonical {
		t.Errorf("Value: %v, erro %v", value, err)
	}

	var u uuid.UUID
	if err := u.Scan(canonical); err != nil || u != reference {
		t.Errorf("Scan de string: %s, erro %v", u, err)
	}
	if err := u.Scan([]byte(canonical)); err != nil || u != reference {
		t.Errorf("Scan de texto em bytes: %s, erro %v", u, err)
	}
	if err := u.Scan(reference[:]); err != nil || u != reference {
		t.Errorf("Scan de 16 bytes: %s, erro %v", u, err)
	}
	if err := u.Scan(nil); err != nil || !u.IsZero() {
		t.Errorf("Scan de NULL: %s, erro %v", u, err)
	}
	if err := u.Scan(42); !errors.Is(err, uuid.ErrInvalidScanType) {
		t.Errorf("Scan de inteiro: erro %v, esperado ErrInvalidScanType", err)
	}

	// Texto vazio é ausência de valor, como no pacote github.com/google/uuid:
	// grava o UUID nulo e não devolve erro.
	u = reference
	if err := u.Scan(""); err != nil || !u.IsZero() {
		t.Errorf("Scan de string vazia: %s, erro %v, esperado UUID nulo sem erro", u, err)
	}
	u = reference
	if err := u.Scan([]byte{}); err != nil || !u.IsZero() {
		t.Errorf("Scan de bytes vazios: %s, erro %v, esperado UUID nulo sem erro", u, err)
	}

	// Em caso de erro o receptor não é alterado.
	u = reference
	if err := u.Scan("invalido"); err == nil || u != reference {
		t.Errorf("Scan de texto inválido: %s, erro %v, esperado erro sem alterar o receptor", u, err)
	}

	// Fatia de bytes que não está vazia nem tem 16 bytes cai no caminho de
	// texto, e um texto inválido ali precisa falhar sem alterar o receptor.
	// É o ramo que distingue "bytes crus" de "texto em bytes".
	u = reference
	if err := u.Scan([]byte("nao-e-um-uuid")); !errors.Is(err, uuid.ErrInvalidFormat) || u != reference {
		t.Errorf("Scan de bytes de texto inválido: %s, erro %v, esperado erro de formato sem alterar o receptor", u, err)
	}
}

// TestNullUUID confere o tipo que aceita coluna nula.
func TestNullUUID(t *testing.T) {
	reference := uuid.MustParse(canonical)

	var n uuid.NullUUID
	if err := n.Scan(nil); err != nil || n.Valid {
		t.Errorf("Scan de NULL: %+v, erro %v", n, err)
	}
	if value, err := n.Value(); err != nil || value != nil {
		t.Errorf("Value sem valor: %v, erro %v", value, err)
	}
	if encoded, err := json.Marshal(n); err != nil || string(encoded) != "null" {
		t.Errorf("JSON sem valor: %s, erro %v", encoded, err)
	}

	if err := n.Scan(canonical); err != nil || !n.Valid || n.UUID != reference {
		t.Errorf("Scan com valor: %+v, erro %v", n, err)
	}

	// Escapes JSON precisam ser interpretados como o encoding/json faria
	// para o tipo UUID: \u0030 vale 0, \u002d vale o hífen.
	// REGRESSÃO: antes da correção, o conteúdo entre aspas era passado cru
	// a ParseBytes e qualquer escape era recusado como formato inválido.
	escaped := `"\u0030192f7c5\u002d1a2b-7c3d-8e4f-aabbccddeeff"`
	var fromEscaped uuid.NullUUID
	if err := json.Unmarshal([]byte(escaped), &fromEscaped); err != nil || !fromEscaped.Valid || fromEscaped.UUID != reference {
		t.Errorf("JSON com escapes: %+v, erro %v, esperado %s válido", fromEscaped, err, reference)
	}
	// O mesmo JSON precisa produzir o mesmo valor no tipo UUID.
	var plain uuid.UUID
	if err := json.Unmarshal([]byte(escaped), &plain); err != nil || plain != fromEscaped.UUID {
		t.Errorf("UUID e NullUUID divergiram ao ler escapes: %s vs %s, erro %v", plain, fromEscaped.UUID, err)
	}
	// Escape inválido e conteúdo inválido após o escape são erros de
	// formato, e o receptor não é alterado. A chamada é direta ao método
	// porque json.Unmarshal rejeita o documento com escape inválido antes
	// de chegar a ele; o que se testa aqui é a conversão feita pelo método.
	kept := fromEscaped
	for _, bad := range []string{`"\x0192f7c5-1a2b-7c3d-8e4f-aabbccddeeff"`, `"\u0030192f7c5-1a2b-7c3d-8e4f-aabbccddeezz"`} {
		err := fromEscaped.UnmarshalJSON([]byte(bad))
		if !errors.Is(err, uuid.ErrInvalidFormat) {
			t.Errorf("JSON %s: erro %v, esperado ErrInvalidFormat", bad, err)
		}
		if fromEscaped != kept {
			t.Errorf("JSON %s alterou o receptor mesmo falhando: %+v", bad, fromEscaped)
		}
	}

	// Texto vazio produz valor ausente, sem erro, como no pacote de origem.
	for name, empty := range map[string]any{"string vazia": "", "bytes vazios": []byte{}} {
		n.Scan(canonical) //nolint:errcheck // reposiciona um valor válido antes de cada caso
		if err := n.Scan(empty); err != nil || n.Valid || !n.UUID.IsZero() {
			t.Errorf("Scan de %s: %+v, erro %v, esperado valor ausente sem erro", name, n, err)
		}
	}
	if err := n.Scan("invalido"); err == nil || n.Valid {
		t.Errorf("Scan de texto inválido: %+v, erro %v, esperado erro com Valid falso", n, err)
	}

	if err := n.Scan(canonical); err != nil || !n.Valid || n.UUID != reference {
		t.Errorf("Scan com valor após ausência: %+v, erro %v", n, err)
	}
	encoded, err := json.Marshal(n)
	if err != nil || string(encoded) != `"`+canonical+`"` {
		t.Fatalf("JSON com valor: %s, erro %v", encoded, err)
	}

	var back uuid.NullUUID
	if err := json.Unmarshal(encoded, &back); err != nil || back != n {
		t.Errorf("volta pelo JSON: %+v, erro %v", back, err)
	}
	if err := json.Unmarshal([]byte("null"), &back); err != nil || back.Valid {
		t.Errorf("volta de null: %+v, erro %v", back, err)
	}
}

// TestNullUUIDRejectsUnsupportedType confere que um valor de tipo
// inesperado não é confundido com ausência de valor: o auxiliar que
// reconhece NULL, texto vazio e fatia vazia precisa recusar qualquer
// outro tipo, e o erro tem de chegar ao chamador com Valid falso.
func TestNullUUIDRejectsUnsupportedType(t *testing.T) {
	n := uuid.NullUUID{UUID: uuid.MustParse(canonical), Valid: true}
	if err := n.Scan(42); !errors.Is(err, uuid.ErrInvalidScanType) {
		t.Errorf("Scan de inteiro: erro %v, esperado ErrInvalidScanType", err)
	}
	if n.Valid {
		t.Error("Scan com erro deveria deixar Valid falso")
	}

	// Um float também não é ausência de valor, nem um booleano: o auxiliar
	// só trata NULL, string e fatia de bytes.
	for _, src := range []any{3.14, true, struct{}{}} {
		var outro uuid.NullUUID
		if err := outro.Scan(src); !errors.Is(err, uuid.ErrInvalidScanType) {
			t.Errorf("Scan de %T: erro %v, esperado ErrInvalidScanType", src, err)
		}
		if outro.Valid {
			t.Errorf("Scan de %T deveria deixar Valid falso", src)
		}
	}
}

// TestGeneratorWithReader confere a fonte de entropia em formato io.Reader.
func TestGeneratorWithReader(t *testing.T) {
	// Dezesseis bytes zerados: o nível 1 consome as duas palavras, e todos
	// os bits aleatórios saem em zero. Sobram apenas tempo, versão e
	// variante.
	g := uuid.NewGeneratorWithReader(bytes.NewReader(make([]byte, 16)))
	u := g.Generate(uuid.Level1)
	if u.Version() != 7 || u.Variant() != 0b10 {
		t.Fatalf("UUID malformado: %s", u)
	}
	if u[6]&0x0F != 0 || u[7] != 0 {
		t.Errorf("rand_a deveria estar zerado: %s", u)
	}

	// Fonte curta: a leitura falha no meio da geração.
	defer func() {
		if recover() == nil {
			t.Error("uma fonte de entropia esgotada deveria entrar em pânico")
		}
	}()
	uuid.NewGeneratorWithReader(bytes.NewReader(make([]byte, 4))).Generate(uuid.Level1)
}

// TestGeneratorWithNilReaderPanics confere que o leitor nulo é rejeitado
// na configuração, e não na primeira geração.
func TestGeneratorWithNilReaderPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewGeneratorWithReader(nil) deveria entrar em pânico")
		}
	}()
	uuid.NewGeneratorWithReader(nil)
}

// TestCryptoGenerator confere que o gerador criptográfico produz UUIDs
// válidos em todos os níveis, nos nomes do UUIDv7 por versão e por nível,
// e também na versão 4.
func TestCryptoGenerator(t *testing.T) {
	g := uuid.NewCryptoGenerator()
	for _, level := range []uuid.Level{uuid.Level1, uuid.Level2, uuid.Level3} {
		u := g.Generate(level)
		if u.Version() != 7 || u.Variant() != 0b10 {
			t.Errorf("nível %d: UUID malformado %s", level, u)
		}
	}
	if u := g.GenerateV7(); u.Version() != 7 || u.Variant() != 0b10 {
		t.Errorf("versão 7 pelo nome malformada: %s", u)
	}
	if u := g.GenerateV7Level1(); u.Version() != 7 || u.Variant() != 0b10 {
		t.Errorf("Nível 1 pelo nome malformado: %s", u)
	}
	if u := g.GenerateV7Level2(); u.Version() != 7 || u.Variant() != 0b10 {
		t.Errorf("Nível 2 pelo nome malformado: %s", u)
	}
	if u := g.GenerateV7Level3(); u.Version() != 7 || u.Variant() != 0b10 {
		t.Errorf("Nível 3 pelo nome malformado: %s", u)
	}
	if u := g.GenerateV4(); u.Version() != 4 || u.Variant() != 0b10 {
		t.Errorf("versão 4 malformada: %s", u)
	}
}

// TestCompatibilityAliases confere os apelidos com os nomes do pacote
// github.com/google/uuid: versão correta e erro sempre nulo.
func TestCompatibilityAliases(t *testing.T) {
	if u := uuid.New(); u.Version() != 4 {
		t.Errorf("New: versão %d", u.Version())
	}
	if err := uuid.Validate(uuid.NewString()); err != nil {
		t.Errorf("NewString: %v", err)
	}

	cases := map[string]struct {
		version byte
		call    func() (uuid.UUID, error)
	}{
		"NewRandom":      {4, uuid.NewRandom},
		"NewUUID":        {1, uuid.NewUUID},
		"NewV6":          {6, uuid.NewV6},
		"NewV7":          {7, uuid.NewV7},
		"NewDCEPerson":   {2, uuid.NewDCEPerson},
		"NewDCEGroup":    {2, uuid.NewDCEGroup},
		"NewDCESecurity": {2, func() (uuid.UUID, error) { return uuid.NewDCESecurity(uuid.Org, 7) }},
	}
	for name, tc := range cases {
		u, err := tc.call()
		if err != nil {
			t.Errorf("%s: erro %v, esperado nulo", name, err)
		}
		if u.Version() != tc.version {
			t.Errorf("%s: versão %d, esperado %d", name, u.Version(), tc.version)
		}
	}

	if uuid.NewMD5(uuid.NameSpaceDNS, []byte("x")) != uuid.GenerateV3(uuid.NameSpaceDNS, []byte("x")) {
		t.Error("NewMD5 divergiu de GenerateV3")
	}
	if uuid.NewSHA1(uuid.NameSpaceDNS, []byte("x")) != uuid.GenerateV5(uuid.NameSpaceDNS, []byte("x")) {
		t.Error("NewSHA1 divergiu de GenerateV5")
	}
}

// TestCompatibilityReaders confere as variantes que recebem io.Reader,
// inclusive a conversão de fonte esgotada em erro devolvido.
func TestCompatibilityReaders(t *testing.T) {
	u, err := uuid.NewV7FromReader(bytes.NewReader(make([]byte, 16)))
	if err != nil || u.Version() != 7 {
		t.Errorf("NewV7FromReader: %s, erro %v", u, err)
	}
	if u, err := uuid.NewRandomFromReader(bytes.NewReader(make([]byte, 16))); err != nil || u.Version() != 4 {
		t.Errorf("NewRandomFromReader: %s, erro %v", u, err)
	}

	if _, err := uuid.NewV7FromReader(bytes.NewReader(make([]byte, 2))); !errors.Is(err, uuid.ErrEntropySource) {
		t.Errorf("fonte esgotada: erro %v, esperado ErrEntropySource", err)
	}
	if _, err := uuid.NewV7FromReader(nil); !errors.Is(err, uuid.ErrEntropySource) {
		t.Errorf("leitor nulo: erro %v, esperado ErrEntropySource", err)
	}
}

// TestNullUUIDTextAndBinary confere as serializações em texto e binário
// de NullUUID, nos dois estados: com valor e ausente. Entrada vazia
// produz valor ausente sem erro; entrada inválida devolve erro e deixa
// Valid falso.
func TestNullUUIDTextAndBinary(t *testing.T) {
	reference := uuid.MustParse(canonical)
	present := uuid.NullUUID{UUID: reference, Valid: true}
	var absent uuid.NullUUID

	// Texto.
	if text, err := present.MarshalText(); err != nil || string(text) != canonical {
		t.Errorf("MarshalText com valor: %q, erro %v", text, err)
	}
	if text, err := absent.MarshalText(); err != nil || len(text) != 0 {
		t.Errorf("MarshalText sem valor: %q, erro %v, esperado vazio", text, err)
	}
	var fromText uuid.NullUUID
	if err := fromText.UnmarshalText([]byte("{" + canonical + "}")); err != nil || fromText != present {
		t.Errorf("UnmarshalText entre chaves: %+v, erro %v", fromText, err)
	}
	if err := fromText.UnmarshalText(nil); err != nil || fromText.Valid || !fromText.UUID.IsZero() {
		t.Errorf("UnmarshalText vazio: %+v, erro %v, esperado valor ausente", fromText, err)
	}
	fromText = present
	if err := fromText.UnmarshalText([]byte("nao-e-um-uuid")); !errors.Is(err, uuid.ErrInvalidFormat) || fromText.Valid {
		t.Errorf("UnmarshalText inválido: %+v, erro %v, esperado ErrInvalidFormat com Valid falso", fromText, err)
	}

	// Binário.
	if raw, err := present.MarshalBinary(); err != nil || !bytes.Equal(raw, reference[:]) {
		t.Errorf("MarshalBinary com valor: %x, erro %v", raw, err)
	}
	if raw, err := absent.MarshalBinary(); err != nil || len(raw) != 0 {
		t.Errorf("MarshalBinary sem valor: %x, erro %v, esperado vazio", raw, err)
	}
	var fromBinary uuid.NullUUID
	if err := fromBinary.UnmarshalBinary(reference[:]); err != nil || fromBinary != present {
		t.Errorf("UnmarshalBinary: %+v, erro %v", fromBinary, err)
	}
	if err := fromBinary.UnmarshalBinary([]byte{}); err != nil || fromBinary.Valid || !fromBinary.UUID.IsZero() {
		t.Errorf("UnmarshalBinary vazio: %+v, erro %v, esperado valor ausente", fromBinary, err)
	}
	fromBinary = present
	if err := fromBinary.UnmarshalBinary(reference[:15]); !uuid.IsInvalidLengthError(err) || fromBinary.Valid {
		t.Errorf("UnmarshalBinary com 15 bytes: %+v, erro %v, esperado ErrInvalidLength com Valid falso", fromBinary, err)
	}

	// Value com valor presente grava a string canônica.
	if value, err := present.Value(); err != nil || value != canonical {
		t.Errorf("Value com valor: %v, erro %v", value, err)
	}
}

// TestMustPropagatesError confere que Must entra em pânico com o próprio
// erro recebido, e não com uma mensagem nova, para que quem recupera o
// pânico consiga reconhecê-lo.
func TestMustPropagatesError(t *testing.T) {
	defer func() {
		recovered := recover()
		err, ok := recovered.(error)
		if !ok || !errors.Is(err, uuid.ErrInvalidFormat) {
			t.Errorf("Must entrou em pânico com %v, esperado o erro ErrInvalidFormat", recovered)
		}
	}()
	uuid.Must(uuid.Parse("nao-e-um-uuid"))
	t.Error("Must deveria ter entrado em pânico")
}

// TestVariantStringCoversAllCodes confere a descrição de cada um dos
// quatro códigos de variante e de um código impossível.
func TestVariantStringCoversAllCodes(t *testing.T) {
	cases := map[byte]string{
		0: "reservada para compatibilidade NCS",
		1: "reservada para compatibilidade NCS",
		2: "RFC 9562",
		3: "reservada para a Microsoft ou para uso futuro",
	}
	for code, want := range cases {
		if got := uuid.VariantString(code); got != want {
			t.Errorf("VariantString(%d) = %q, esperado %q", code, got, want)
		}
	}
	// Variant só devolve 0..3, mas a função aceita qualquer byte.
	if got := uuid.VariantString(9); got != "variante desconhecida 9" {
		t.Errorf("VariantString(9) = %q", got)
	}
	if got := uuid.VersionString(0); got != "versao desconhecida 0" {
		t.Errorf("VersionString(0) = %q", got)
	}
}

// TestNewHashMatchesGenerateHash confere que o apelido NewHash produz o
// mesmo valor que GenerateHash, inclusive quando o resumo é menor que 16
// bytes: os bytes que faltam ficam em zero, e versão e variante são
// aplicadas mesmo assim.
func TestNewHashMatchesGenerateHash(t *testing.T) {
	name := []byte("www.example.com")

	// SHA-1 (20 bytes): o apelido tem que bater com a versão 5 da RFC.
	if got := uuid.NewHash(sha1.New(), uuid.NameSpaceDNS, name, 5); got != uuid.GenerateV5(uuid.NameSpaceDNS, name) { //nolint:gosec // exigido pela RFC 9562 para a versão 5
		t.Errorf("NewHash com SHA-1 divergiu de GenerateV5: %s", got)
	}

	// Resumo curto (8 bytes): só os 8 primeiros bytes vêm do resumo, os
	// demais ficam zerados; versão e variante continuam corretas.
	short := uuid.GenerateHash(fnv.New64a(), uuid.NameSpaceDNS, name, 8)
	if short.Version() != 8 || short.Variant() != 2 {
		t.Errorf("resumo curto: versão %d variante %d", short.Version(), short.Variant())
	}
	for i := 9; i < 16; i++ {
		if short[i] != 0 {
			t.Errorf("resumo curto: byte %d = %#02x, esperado zero", i, short[i])
		}
	}
	if got := uuid.NewHash(fnv.New64a(), uuid.NameSpaceDNS, name, 8); got != short {
		t.Errorf("NewHash com resumo curto divergiu de GenerateHash: %s vs %s", got, short)
	}
}

// TestErrorTaxonomy percorre a tabela da seção 6.4 da especificação
// (caso obrigatório 17): cada entrada produz o erro descrito, reconhecido
// pelo tipo e não só pela presença; o UUID devolvido é o nulo; o
// analisador estrito e Import devolvem exatamente o sentinela; e a
// assimetria registrada do prefixo URN, que devolve o sentinela enquanto
// as chaves malformadas têm erro próprio, fica travada.
func TestErrorTaxonomy(t *testing.T) {
	// classe reduz um erro à linha da tabela a que ele pertence.
	classe := func(err error) string {
		switch {
		case err == nil:
			return "nenhum"
		case uuid.IsInvalidLengthError(err):
			return "comprimento"
		case errors.Is(err, uuid.ErrInvalidBrackets):
			return "chaves"
		case err == uuid.ErrInvalidFormat: //nolint:errorlint // o sentinela puro é o próprio contrato testado
			return "sentinela puro"
		case errors.Is(err, uuid.ErrInvalidFormat):
			return "formato embrulhado"
		}
		return "outro"
	}

	const compact = "0192f7c51a2b7c3d8e4faabbccddeeff"
	cases := []struct {
		name  string
		input string
		want  string // classe esperada do analisador permissivo
	}{
		{"comprimento errado", "abc", "comprimento"},
		{"comprimento 37", canonical + "0", "comprimento"},
		{"vazio", "", "comprimento"},
		{"chaves trocadas", "(" + canonical + ")", "chaves"},
		{"chave de fechamento ausente", "{" + canonical + "0", "chaves"},
		{"prefixo URN inválido", "urn:uiid:" + canonical, "sentinela puro"},
		{"prefixo URN de outro esquema", "urn:isbn:" + canonical, "sentinela puro"},
		{"dígito inválido na forma canônica", canonical[:35] + "g", "sentinela puro"},
		{"hífen fora de lugar", "0192f7c5-1a2b-7c3d-8e4faabbccddeeff-", "sentinela puro"},
		{"dígito inválido na forma crua", compact[:31] + "g", "sentinela puro"},
		{"dígito inválido entre chaves", "{" + canonical[:35] + "g}", "sentinela puro"},
		{"dígito inválido na URN", "urn:uuid:" + canonical[:35] + "g", "sentinela puro"},
	}
	reference := uuid.MustParse(canonical)
	for _, c := range cases {
		got, err := uuid.Parse(c.input)
		if classe(err) != c.want {
			t.Errorf("%s: Parse(%q) devolveu %v (%s), esperado %s", c.name, c.input, err, classe(err), c.want)
		}
		if !got.IsZero() {
			t.Errorf("%s: Parse devolveu %s junto do erro, esperado o UUID nulo", c.name, got)
		}
		if _, err := uuid.ParseBytes([]byte(c.input)); classe(err) != c.want {
			t.Errorf("%s: ParseBytes devolveu %v (%s), esperado %s", c.name, err, classe(err), c.want)
		}
		if err := uuid.Validate(c.input); classe(err) != c.want {
			t.Errorf("%s: Validate devolveu %v (%s), esperado %s", c.name, err, classe(err), c.want)
		}

		// A desserialização de texto classifica como Parse e não altera o
		// receptor.
		u := reference
		if err := u.UnmarshalText([]byte(c.input)); classe(err) != c.want || u != reference {
			t.Errorf("%s: UnmarshalText devolveu %v (%s) e deixou %s; esperado %s sem alterar o receptor",
				c.name, err, classe(err), u, c.want)
		}

		// O analisador estrito e Import devolvem sempre o sentinela puro.
		if _, err := uuid.FromString(c.input); classe(err) != "sentinela puro" {
			t.Errorf("%s: FromString devolveu %v (%s), esperado o sentinela puro", c.name, err, classe(err))
		}
		if tm, err := uuid.Import(c.input); classe(err) != "sentinela puro" || tm != (uuid.Time{}) {
			t.Errorf("%s: Import devolveu %+v com %v (%s), esperado estrutura zerada e o sentinela puro",
				c.name, tm, err, classe(err))
		}
	}

	// Construção e desserialização binária: comprimento inválido.
	if _, err := uuid.FromBytes(reference[:15]); classe(err) != "comprimento" {
		t.Errorf("FromBytes com 15 bytes: %v (%s), esperado comprimento inválido", err, classe(err))
	}
	u := reference
	if err := u.UnmarshalBinary(reference[:15]); classe(err) != "comprimento" || u != reference {
		t.Errorf("UnmarshalBinary com 15 bytes: %v (%s) e deixou %s; esperado comprimento inválido sem alterar o receptor",
			err, classe(err), u)
	}

	// JSON do tipo anulável: valor que não é string, ou sintaxe inválida,
	// devolve o sentinela puro; string recusada devolve o erro de Parse.
	for _, c := range []struct {
		name, input, want string
	}{
		{"número no lugar da string", `42`, "sentinela puro"},
		{"objeto no lugar da string", `{}`, "sentinela puro"},
		{"string sem fechar", `"abc`, "sentinela puro"},
		{"string curta", `"abc"`, "comprimento"},
		{"string com chaves trocadas", `"(` + canonical + `)"`, "chaves"},
		{"string com prefixo URN inválido", `"urn:uiid:` + canonical + `"`, "sentinela puro"},
		{"string com escape e dígito inválido", `"0192f7c5-1a2b-7c3d-8e4f-aabbccddeefg"`, "sentinela puro"},
	} {
		n := uuid.NullUUID{UUID: reference, Valid: true}
		if err := n.UnmarshalJSON([]byte(c.input)); classe(err) != c.want || n.UUID != reference || !n.Valid {
			t.Errorf("NullUUID.UnmarshalJSON(%s): %v (%s) e deixou %+v; esperado %s sem alterar o receptor",
				c.name, err, classe(err), n, c.want)
		}
	}

	// Tipo não suportado e fonte de entropia são famílias à parte: não
	// são erros de formato.
	if err := u.Scan(3.14); !errors.Is(err, uuid.ErrInvalidScanType) || classe(err) != "outro" {
		t.Errorf("Scan de float: %v (%s), esperado ErrInvalidScanType fora da família de formato", err, classe(err))
	}
	if classe(uuid.ErrEntropySource) != "outro" {
		t.Error("ErrEntropySource não deveria ser reconhecido como erro de formato")
	}
	if _, err := uuid.NewV7FromReader(nil); !errors.Is(err, uuid.ErrEntropySource) {
		t.Errorf("NewV7FromReader(nil): %v, esperado ErrEntropySource", err)
	}
}
