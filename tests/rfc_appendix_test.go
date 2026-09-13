package tests

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// Este arquivo trava os vetores dos Apêndices A ("Test Vectors") e B
// ("Illustrative Examples") da RFC 9562, publicados no texto da própria
// especificação — não calculados por esta biblioteca. A RFC dá vetor
// para as versões 1, 3, 4, 5, 6 e 7 no Apêndice A, e dois exemplos de
// versão 8 no Apêndice B; não há vetor de versão 2, fora do escopo da
// RFC (seção 5.2). Antes deste arquivo, só V3 e V5 (caso obrigatório 5
// da seção 10 de docs/SPEC.md) estavam travados contra a RFC: os demais
// vetores existiam só como documentação (docs/SPEC.md seção 10, caso 18,
// e o comentário deste pacote em golden_test.go), sem teste algum —
// nenhum dos dois lugares nota que a RFC também publica vetor de V1 e
// V6, apesar de os dois afirmarem o contrário. Ver docs/SPEC.md seção
// 10 caso 18 e a correção registrada no CHANGELOG.

const (
	rfcA1V1 = "C232AB00-9414-11EC-B3C8-9F6BDECED846"
	rfcA3V4 = "919108f7-52d1-4320-9bac-f847db4148a8"
	rfcA5V6 = "1EC9414C-232A-6B00-B3C8-9F6BDECED846"
	rfcA6V7 = "017F22E2-79B0-7CC3-98C4-DC0C0C07398F"
	rfcB1V8 = "2489E9AD-2EE2-8E00-8EC9-32D5F69181C0"
	rfcB2V8 = "5c146b14-3c52-8afd-938a-375d0df1fbf6"
)

// rfcNode é o nó do Apêndice A.1/A.5: "todos gerados com dados
// aleatórios", com o bit multicast já ligado no valor publicado.
var rfcNode = [6]byte{0x9F, 0x6B, 0xDE, 0xCE, 0xD8, 0x46}

// TestRFCAppendixA1A5ReadV1V6 confere a leitura dos vetores de versão 1
// e 6 do Apêndice A: os dois compartilham o mesmo tempo gregoriano
// (0x1EC9414C232AB00, 2022-02-22T19:22:22Z), a mesma sequência de
// relógio (0x33C8, embutida no byte 8 como "10110011" = variante + seis
// bits altos, e no byte 9 como 0xC8) e o mesmo nó.
func TestRFCAppendixA1A5ReadV1V6(t *testing.T) {
	const rfcTicks = 0x1EC9414C232AB00
	const rfcSeq = 0x33C8
	wantInstant := time.Date(2022, 2, 22, 19, 22, 22, 0, time.UTC)

	for _, versao := range []struct {
		nome string
		s    string
	}{{"v1", rfcA1V1}, {"v6", rfcA5V6}} {
		u := uuid.MustParse(versao.s)

		g, ok := u.GregorianTime()
		if !ok || uint64(g) != rfcTicks {
			t.Errorf("%s: GregorianTime = %#x (ok=%v), esperado %#x", versao.nome, uint64(g), ok, uint64(rfcTicks))
		}
		if seq, ok := u.ClockSequence(); !ok || seq != rfcSeq {
			t.Errorf("%s: ClockSequence = %#x (ok=%v), esperado %#x", versao.nome, seq, ok, rfcSeq)
		}
		if node := u.NodeID(); !bytes.Equal(node, rfcNode[:]) {
			t.Errorf("%s: NodeID = %x, esperado %x", versao.nome, node, rfcNode)
		}
		if ts, ok := u.Timestamp(); !ok || !ts.Equal(wantInstant) {
			t.Errorf("%s: Timestamp = %v (ok=%v), esperado %v", versao.nome, ts, ok, wantInstant)
		}
		if !u.IsValid() {
			t.Errorf("%s: IsValid devolveu falso para um vetor publicado", versao.nome)
		}
	}

	// O reempacotamento independente de golden_test.go, alimentado com o
	// tempo e a sequência do Apêndice A, tem de reproduzir os 16 bytes
	// publicados — fixa o escritor com os mesmos valores que fixam o
	// leitor acima.
	if got := packV1Reference(rfcTicks, rfcSeq, rfcNode); got != uuid.MustParse(rfcA1V1) {
		t.Errorf("packV1Reference(vetor RFC A.1) = %s, esperado %s", got, rfcA1V1)
	}
	if got := packV6Reference(rfcTicks, rfcSeq, rfcNode); got != uuid.MustParse(rfcA5V6) {
		t.Errorf("packV6Reference(vetor RFC A.5) = %s, esperado %s", got, rfcA5V6)
	}
}

// TestRFCAppendixA1A5GenerateV1V6 fecha o cancelamento simétrico para os
// valores do Apêndice A: com a sequência e o nó fixados nos valores
// publicados, GenerateV1 e GenerateV6 gravam esses mesmos campos nos
// bytes 8 a 15 (o tempo não é fixável, por não haver relógio injetável —
// ver docs/SPEC.md seção 11.2).
func TestRFCAppendixA1A5GenerateV1V6(t *testing.T) {
	withIsolatedClockState(t)
	uuid.SetClockSequence(0x33C8)
	if !uuid.SetNodeID(rfcNode[:]) {
		t.Fatal("SetNodeID recusou o nó de 6 bytes do vetor")
	}

	ref := uuid.MustParse(rfcA1V1)
	for nome, gen := range map[string]func() uuid.UUID{"GenerateV1": uuid.GenerateV1, "GenerateV6": uuid.GenerateV6} {
		u := gen()
		if !bytes.Equal(u[8:], ref[8:]) {
			t.Errorf("%s: bytes 8..15 = %x, esperado %x (sequência e nó do vetor RFC A.1)", nome, u[8:], ref[8:])
		}
	}
}

// TestRFCAppendixA6V7 confere o vetor de versão 7 do Apêndice A.6:
// unix_ts_ms = 1645557742000, rand_a = 0xCC3, rand_b = 0b01 seguido de
// 0x8C4DC0C0C07398F. GenerateAt com uma fonte de entropia fixa nesses
// bits, no instante do vetor, tem de reproduzir os 16 bytes publicados —
// prova a leitura (Timestamp, ImportBinary) e a escrita (GenerateAt)
// pela mesma via pública que os demais casos desta suíte usam.
func TestRFCAppendixA6V7(t *testing.T) {
	want := uuid.MustParse(rfcA6V7)
	instante := time.UnixMilli(1645557742000)

	if got, ok := want.Timestamp(); !ok || !got.Equal(instante.UTC()) {
		t.Errorf("Timestamp do vetor RFC A.6 = %v (ok=%v), esperado %v", got, ok, instante.UTC())
	}
	tm := uuid.ImportBinary(want)
	if tm.Seconds != 1645557742 || tm.Milliseconds != 0 || tm.Microseconds != 0x0CC3 {
		t.Errorf("ImportBinary do vetor RFC A.6 = %+v, esperado micro=0xcc3 sobre o milissegundo do vetor", tm)
	}

	// rand_a = 0x0CC3 (12 bits) seguido de rand_b = 0b01_1000_1100_0100_1101_1100_0000_1100_0000_1100_0000_0111_0011_1001_1111 nos
	// 62 bits livres, exatamente como o Apêndice A.6 descreve.
	entropia, err := hex.DecodeString("0000000000000CC3" + "18C4DC0C0C07398F")
	if err != nil {
		t.Fatalf("entropia do vetor mal formada: %v", err)
	}
	g := uuid.NewGeneratorWithReader(bytes.NewReader(entropia))
	if got := g.GenerateAt(uuid.Level1, instante); got != want {
		t.Errorf("GenerateAt(Level1) com a entropia do vetor = %s, esperado %s", got, want)
	}
}

// TestRFCAppendixA3V4 confere o vetor de versão 4 do Apêndice A.3: os 128
// bits aleatórios publicados, com versão e variante sobrepostas.
func TestRFCAppendixA3V4(t *testing.T) {
	entropia, err := hex.DecodeString("919108F752D133205BACF847DB4148A8")
	if err != nil {
		t.Fatalf("entropia do vetor mal formada: %v", err)
	}
	want := uuid.MustParse(rfcA3V4)

	if got := uuid.NewGeneratorWithReader(bytes.NewReader(entropia)).GenerateV4(); got != want {
		t.Errorf("GenerateV4 com a entropia do vetor = %s, esperado %s", got, want)
	}
	if got, err := uuid.NewRandomFromReader(bytes.NewReader(entropia)); err != nil || got != want {
		t.Errorf("NewRandomFromReader com a entropia do vetor = %s, erro %v, esperado %s", got, err, want)
	}
}

// TestRFCAppendixB1B2V8 confere os dois exemplos de versão 8 do Apêndice
// B: o time-based (B.1), montado à mão com GenerateV8, e o name-based
// (B.2), resumo SHA-256 do espaço NameSpaceDNS com "www.example.com" —
// a mesma derivação de V3/V5, com outro algoritmo de resumo e a versão
// 8, como a RFC seção 5.5 recomenda para hashes modernos.
func TestRFCAppendixB1B2V8(t *testing.T) {
	raw, err := hex.DecodeString("2489E9AD2EE20E000EC932D5F69181C0")
	if err != nil {
		t.Fatalf("dados do vetor B.1 mal formados: %v", err)
	}
	var data [16]byte
	copy(data[:], raw)
	if got := uuid.GenerateV8(data); got != uuid.MustParse(rfcB1V8) {
		t.Errorf("GenerateV8(vetor B.1) = %s, esperado %s", got, rfcB1V8)
	}

	name := []byte("www.example.com")
	if got := uuid.GenerateHash(sha256.New(), uuid.NameSpaceDNS, name, 8); got.String() != rfcB2V8 {
		t.Errorf("GenerateHash(SHA-256, NameSpaceDNS, %q, 8) = %s, esperado %s", name, got, rfcB2V8)
	}
	if got := uuid.NewHash(sha256.New(), uuid.NameSpaceDNS, name, 8); got.String() != rfcB2V8 {
		t.Errorf("NewHash(SHA-256, NameSpaceDNS, %q, 8) = %s, esperado %s", name, got, rfcB2V8)
	}
}
