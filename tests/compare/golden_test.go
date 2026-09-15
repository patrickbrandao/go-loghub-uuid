package compare

import (
	"testing"
	"time"

	google "github.com/google/uuid"
	loghub "github.com/patrickbrandao/go-loghub-uuid"
)

// Este arquivo confere os vetores dourados das versões 1 e 2 publicados
// em docs/09-vetores-dourados-e-apendice-rfc.md seção 10, caso 18, contra
// o LEITOR do pacote github.com/google/uuid: os mesmos bytes, dois
// leitores independentes, os mesmos campos. É a metade externa da
// verificação; a metade por fórmula, e o escritor, estão em
// tests/golden_test.go.
//
// A versão 6 fica de fora de propósito, e o motivo é medido abaixo, não
// suposto.

// goldenNode é o nó fixo da tabela, com o bit multicast ligado.
var goldenNode = []byte{0x02, 0x11, 0x22, 0x33, 0x44, 0x55}

// TestGoogleReadsGregorianGoldenVectors exige que o leitor do outro
// pacote extraia dos bytes publicados exatamente os valores de entrada.
func TestGoogleReadsGregorianGoldenVectors(t *testing.T) {
	v1 := []struct {
		canonica string
		now      int64
	}{
		{"d0d69687-e6a4-11f0-b3c8-021122334455", 139_865_184_001_234_567},
		{"13814000-1dd2-11b2-b3c8-021122334455", 122_192_928_000_000_000},
	}
	for _, v := range v1 {
		g := google.MustParse(v.canonica)
		if g.Version() != 1 {
			t.Errorf("%s: google leu versao %d, esperado 1", v.canonica, g.Version())
		}
		if got := int64(g.Time()); got != v.now {
			t.Errorf("%s: google leu instante %d, publicado %d", v.canonica, got, v.now)
		}
		if got := g.ClockSequence(); got != 0x33C8 {
			t.Errorf("%s: google leu sequencia %#x, publicada 0x33c8", v.canonica, got)
		}
		if got := g.NodeID(); string(got) != string(goldenNode) {
			t.Errorf("%s: google leu no %x, publicado %x", v.canonica, got, goldenNode)
		}

		// Os dois pacotes convertem o campo para o mesmo instante.
		sec, nsec := g.Time().UnixTime()
		lg, ok := loghub.MustParse(v.canonica).GregorianTime()
		if !ok {
			t.Fatalf("%s: loghub recusou o UUIDv1", v.canonica)
		}
		if deGoogle := time.Unix(sec, nsec).UTC(); !deGoogle.Equal(lg.Time()) {
			t.Errorf("%s: instantes divergem: google %v, loghub %v", v.canonica, deGoogle, lg.Time())
		}
	}

	for _, s := range []string{
		"000003e8-e6a4-21f0-b301-021122334455",
		"000003e8-1dd2-21b2-b301-021122334455",
	} {
		g := google.MustParse(s)
		if g.Version() != 2 {
			t.Errorf("%s: google leu versao %d, esperado 2", s, g.Version())
		}
		if got := g.Domain(); got != google.Group {
			t.Errorf("%s: google leu dominio %v, publicado Group", s, got)
		}
		if got := g.ID(); got != 1000 {
			t.Errorf("%s: google leu identificador %d, publicado 1000", s, got)
		}
		if got := g.NodeID(); string(got) != string(goldenNode) {
			t.Errorf("%s: google leu no %x, publicado %x", s, got, goldenNode)
		}
		// O pacote do Google devolve os 14 bits com o dominio dentro; os 6
		// bits altos tem de bater com a sequencia publicada, e sao os que
		// esta biblioteca devolve para a versao 2.
		if got := g.ClockSequence() >> 8; got != 0x33 {
			t.Errorf("%s: google leu sequencia %#x, esperado 0x33 nos 6 bits altos", s, got)
		}
		if got, ok := loghub.MustParse(s).ClockSequence(); !ok || got != 0x33 {
			t.Errorf("%s: loghub leu sequencia %#x (ok %v), esperado 0x33", s, got, ok)
		}
	}
}

// TestGoogleV6LayoutDiffersFromRFC9562 mede por que o vetor da versao 6
// nao e conferido contra o pacote do Google. Na v1.6.0, NewV6 grava o
// carimbo de 64 bits inteiro nos bytes 0 a 7 (PutUint64) e sobrepoe a
// versao no byte 6, o que nao e a ordem time_high(32), time_mid(16),
// ver(4), time_low(12) da RFC 9562 secao 5.6. Lido pela ordem da RFC, o
// UUIDv6 dele cai seculos atras do relogio; lido pelo proprio leitor
// dele, cai perto. O UUIDv6 desta biblioteca, lido pela ordem da RFC,
// cai perto.
//
// Se este teste FALHAR por o instante lido pela RFC passar a bater, o
// outro pacote corrigiu o layout e a documentacao deste projeto passou a
// estar errada: corrija docs/09-vetores-dourados-e-apendice-rfc.md caso
// 18 e passe a conferir a versao 6 tambem.
func TestGoogleV6LayoutDiffersFromRFC9562(t *testing.T) {
	agora := time.Now()
	g, err := google.NewV6()
	if err != nil {
		t.Fatalf("google.NewV6: %v", err)
	}

	// Leitura pela ordem de campos da RFC 9562 (esta biblioteca).
	lh, err := loghub.FromBytes(g[:])
	if err != nil {
		t.Fatalf("FromBytes: %v", err)
	}
	pelaRFC, ok := lh.GregorianTime()
	if !ok {
		t.Fatal("GregorianTime recusou um UUIDv6")
	}
	desvioRFC := agora.Sub(pelaRFC.Time())

	// Leitura pelo proprio leitor do pacote do Google.
	sec, nsec := g.Time().UnixTime()
	peloGoogle := time.Unix(sec, nsec).UTC()
	desvioGoogle := agora.Sub(peloGoogle)

	const umAno = 365 * 24 * time.Hour
	if desvioRFC < umAno && desvioRFC > -umAno {
		t.Errorf("o UUIDv6 do google lido pela RFC 9562 caiu a %v do relogio; esperava-se seculos de distancia. "+
			"O pacote mudou o layout? Atualize docs/09-vetores-dourados-e-apendice-rfc.md caso 18", desvioRFC)
	}
	if desvioGoogle > time.Second || desvioGoogle < -time.Second {
		t.Errorf("o leitor do google leu o proprio UUIDv6 a %v do relogio; esperava-se menos de um segundo", desvioGoogle)
	}
	t.Logf("google v6 %s: pela RFC 9562 %v (desvio %v); pelo proprio leitor %v (desvio %v)",
		g, pelaRFC.Time(), desvioRFC, peloGoogle, desvioGoogle)

	// Esta biblioteca, lida pela ordem da RFC, acompanha o relogio.
	ours := loghub.GenerateV6()
	og, ok := ours.GregorianTime()
	if !ok {
		t.Fatal("GregorianTime recusou o proprio UUIDv6")
	}
	if d := agora.Sub(og.Time()); d > time.Second || d < -time.Second {
		t.Errorf("loghub v6 lido pela RFC 9562 a %v do relogio; esperava-se menos de um segundo", d)
	}
}
