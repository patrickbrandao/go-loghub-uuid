package tests

import (
	"bytes"
	"encoding/json"
	"errors"
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
// válidos em todos os níveis e também na versão 4.
func TestCryptoGenerator(t *testing.T) {
	g := uuid.NewCryptoGenerator()
	for _, level := range []uuid.Level{uuid.Level1, uuid.Level2, uuid.Level3} {
		u := g.Generate(level)
		if u.Version() != 7 || u.Variant() != 0b10 {
			t.Errorf("nível %d: UUID malformado %s", level, u)
		}
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
