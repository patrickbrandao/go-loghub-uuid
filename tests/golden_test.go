package tests

import (
	"encoding/hex"
	"testing"
	"time"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// Este arquivo guarda os vetores dourados da extensão multinível,
// publicados em docs/09-vetores-dourados-e-apendice-rfc.md seção 10,
// caso obrigatório 12.
//
// Eles são CONTRATO, não teste comum: uma implementação em outra
// linguagem confere o próprio empacotamento contra esta tabela. Mudar
// qualquer valor aqui é mudança de formato de dados, não ajuste de
// teste. Se um destes falhar, o defeito está no código, não no vetor.
//
// Os valores foram calculados por uma implementação independente,
// escrita a partir das regras das seções 3.1, 3.2 e 3.5 da
// especificação, e só então conferidos contra esta biblioteca. Um vetor
// gerado pela própria implementação e conferido contra ela mesma não
// provaria nada.
//
// MinAt e MaxAt são o caminho para produzi-los sem relógio: elas dão,
// por construção, os 16 bytes de um instante fixo com todos os bits
// livres em zero e em um.

// goldenVector é uma linha da tabela publicada.
type goldenVector struct {
	grupo     string // identificador do grupo na especificação
	instante  string // o instante em texto, como publicado
	sec       int64  // segundos Unix do instante
	nsec      int    // nanossegundos dentro do segundo
	level     uuid.Level
	bitsLivre string // "zero" ou "um"
	canonica  string // a string canônica esperada
}

var goldenVectors = []goldenVector{
	{"A", "2026-01-01T00:00:00.123456789Z", 1767225600, 123456789, uuid.Level1, "zero", "019b76da-a87b-7000-8000-000000000000"},
	{"A", "2026-01-01T00:00:00.123456789Z", 1767225600, 123456789, uuid.Level1, "um", "019b76da-a87b-7fff-bfff-ffffffffffff"},
	{"A", "2026-01-01T00:00:00.123456789Z", 1767225600, 123456789, uuid.Level2, "zero", "019b76da-a87b-71c8-8000-000000000000"},
	{"A", "2026-01-01T00:00:00.123456789Z", 1767225600, 123456789, uuid.Level2, "um", "019b76da-a87b-71c8-bfff-ffffffffffff"},
	{"A", "2026-01-01T00:00:00.123456789Z", 1767225600, 123456789, uuid.Level3, "zero", "019b76da-a87b-71c8-b150-000000000000"},
	{"A", "2026-01-01T00:00:00.123456789Z", 1767225600, 123456789, uuid.Level3, "um", "019b76da-a87b-71c8-b15f-ffffffffffff"},
	{"B", "2026-01-01T00:00:00.000000000Z", 1767225600, 0, uuid.Level1, "zero", "019b76da-a800-7000-8000-000000000000"},
	{"B", "2026-01-01T00:00:00.000000000Z", 1767225600, 0, uuid.Level1, "um", "019b76da-a800-7fff-bfff-ffffffffffff"},
	{"B", "2026-01-01T00:00:00.000000000Z", 1767225600, 0, uuid.Level2, "zero", "019b76da-a800-7000-8000-000000000000"},
	{"B", "2026-01-01T00:00:00.000000000Z", 1767225600, 0, uuid.Level2, "um", "019b76da-a800-7000-bfff-ffffffffffff"},
	{"B", "2026-01-01T00:00:00.000000000Z", 1767225600, 0, uuid.Level3, "zero", "019b76da-a800-7000-8000-000000000000"},
	{"B", "2026-01-01T00:00:00.000000000Z", 1767225600, 0, uuid.Level3, "um", "019b76da-a800-7000-800f-ffffffffffff"},
	{"C", "1970-01-01T00:00:00.000000000Z", 0, 0, uuid.Level1, "zero", "00000000-0000-7000-8000-000000000000"},
	{"C", "1970-01-01T00:00:00.000000000Z", 0, 0, uuid.Level1, "um", "00000000-0000-7fff-bfff-ffffffffffff"},
	{"C", "1970-01-01T00:00:00.000000000Z", 0, 0, uuid.Level2, "zero", "00000000-0000-7000-8000-000000000000"},
	{"C", "1970-01-01T00:00:00.000000000Z", 0, 0, uuid.Level2, "um", "00000000-0000-7000-bfff-ffffffffffff"},
	{"C", "1970-01-01T00:00:00.000000000Z", 0, 0, uuid.Level3, "zero", "00000000-0000-7000-8000-000000000000"},
	{"C", "1970-01-01T00:00:00.000000000Z", 0, 0, uuid.Level3, "um", "00000000-0000-7000-800f-ffffffffffff"},
}

// TestGoldenVectors confere os 16 bytes e a string canônica de cada
// vetor publicado, nos três níveis e nas duas pontas de entropia.
// Qualquer bit que troque de lugar quebra este teste.
func TestGoldenVectors(t *testing.T) {
	for _, v := range goldenVectors {
		instante := time.Unix(v.sec, int64(v.nsec)).UTC()

		var got uuid.UUID
		switch v.bitsLivre {
		case "zero":
			got = uuid.MinAt(v.level, instante)
		case "um":
			got = uuid.MaxAt(v.level, instante)
		default:
			t.Fatalf("vetor %s: bits livres %q desconhecido", v.grupo, v.bitsLivre)
		}

		if got.String() != v.canonica {
			t.Errorf("grupo %s, %s, nível %d, bits livres em %s:\n  obtido:   %s\n  esperado: %s",
				v.grupo, v.instante, v.level, v.bitsLivre, got, v.canonica)
			continue
		}

		// Confere também os 16 bytes, não só o texto: a formatação
		// canônica é outro caminho de código e poderia mascarar um erro
		// de empacotamento se os dois fossem escritos juntos.
		querBytes, err := hex.DecodeString(
			v.canonica[0:8] + v.canonica[9:13] + v.canonica[14:18] + v.canonica[19:23] + v.canonica[24:])
		if err != nil {
			t.Fatalf("vetor %s: a string publicada não é hexadecimal válido: %v", v.grupo, err)
		}
		if string(got.Bytes()) != string(querBytes) {
			t.Errorf("grupo %s, nível %d, bits livres em %s: bytes %x, esperado %x",
				v.grupo, v.level, v.bitsLivre, got.Bytes(), querBytes)
		}
	}
}

// TestGoldenVectorsCoverAllLevelsAndFills confere que a tabela publicada
// não perdeu linhas: três grupos, três níveis e duas pontas de entropia.
// Uma tabela incompleta passaria despercebida, porque cada linha se
// verifica sozinha.
func TestGoldenVectorsCoverAllLevelsAndFills(t *testing.T) {
	const esperado = 3 * 3 * 2
	if len(goldenVectors) != esperado {
		t.Fatalf("a tabela tem %d vetores, esperado %d", len(goldenVectors), esperado)
	}

	visto := make(map[string]bool, esperado)
	for _, v := range goldenVectors {
		chave := v.grupo + string(rune('0'+v.level)) + v.bitsLivre
		if visto[chave] {
			t.Errorf("vetor repetido: grupo %s, nível %d, bits livres em %s", v.grupo, v.level, v.bitsLivre)
		}
		visto[chave] = true
	}
}

// TestGoldenVectorPreEpochFloorsToEpoch confere o piso pré-época contra
// a mesma tabela: um instante anterior a 1970 produz exatamente os
// vetores do grupo C, o da própria época.
func TestGoldenVectorPreEpochFloorsToEpoch(t *testing.T) {
	preEpoca := time.Date(1969, 12, 31, 23, 59, 59, 999_999_999, time.UTC)

	for _, v := range goldenVectors {
		if v.grupo != "C" {
			continue
		}
		var got uuid.UUID
		if v.bitsLivre == "zero" {
			got = uuid.MinAt(v.level, preEpoca)
		} else {
			got = uuid.MaxAt(v.level, preEpoca)
		}
		if got.String() != v.canonica {
			t.Errorf("pré-época, nível %d, bits livres em %s: %s, esperado o vetor da época %s",
				v.level, v.bitsLivre, got, v.canonica)
		}
	}
}

// --- vetores dourados das versões de tempo gregoriano (caso 18) ---

// A segunda tabela deste arquivo cobre as versões 1, 2 e 6, publicadas
// em docs/09-vetores-dourados-e-apendice-rfc.md seção 10, caso
// obrigatório 18. A RFC 9562 não tem vetores para elas, e a ordenação
// crescente não substitui um vetor: um deslocamento errado por uma casa,
// aplicado igualmente em version1.go e em inspect.go, passa em toda a
// suíte e produz um identificador que nenhuma outra implementação lê. É
// esse cancelamento simétrico que a tabela fecha: ela fixa o leitor, e o
// reempacotamento de referência abaixo fixa o escritor.
//
// Os valores foram calculados por um programa escrito a partir das
// fórmulas da seção 4.1, sem chamar esta biblioteca, e conferidos na
// versão 1 contra a biblioteca padrão do Python; a leitura das versões
// 1 e 2 é conferida também contra github.com/google/uuid, em
// tests/compare. Preencher uma linha com a saída desta biblioteca
// anularia o propósito do vetor.

// gregorianVector é uma linha da tabela do caso 18.
type gregorianVector struct {
	grupo    string
	instante string
	now      uint64 // o campo de 60 bits: tiques de 100 ns desde 1582
	version  byte
	canonica string
}

// Entradas fixas da tabela: sequência de relógio, nó e, na versão 2,
// domínio e identificador local.
const (
	goldenSeq    = 0x33C8
	goldenDomain = uuid.Group
	goldenID     = uint32(1000)
)

var goldenNode = [6]byte{0x02, 0x11, 0x22, 0x33, 0x44, 0x55}

var gregorianVectors = []gregorianVector{
	{"A", "2026-01-01T00:00:00.123456789Z", 139_865_184_001_234_567, 1, "d0d69687-e6a4-11f0-b3c8-021122334455"},
	{"A", "2026-01-01T00:00:00.123456789Z", 139_865_184_001_234_567, 6, "1f0e6a4d-0d69-6687-b3c8-021122334455"},
	{"A", "2026-01-01T00:00:00.123456789Z", 139_865_184_001_234_567, 2, "000003e8-e6a4-21f0-b301-021122334455"},
	{"C", "1970-01-01T00:00:00.000000000Z", 122_192_928_000_000_000, 1, "13814000-1dd2-11b2-b3c8-021122334455"},
	{"C", "1970-01-01T00:00:00.000000000Z", 122_192_928_000_000_000, 6, "1b21dd21-3814-6000-b3c8-021122334455"},
	{"C", "1970-01-01T00:00:00.000000000Z", 122_192_928_000_000_000, 2, "000003e8-1dd2-21b2-b301-021122334455"},
}

// instanteDoGrupo é o instante de cada grupo truncado à resolução do
// campo, 100 ns, que é o que Timestamp deve devolver.
var instanteDoGrupo = map[string]time.Time{
	"A": time.Date(2026, 1, 1, 0, 0, 0, 123_456_700, time.UTC),
	"C": time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
}

// packV1Reference, packV6Reference e packV2Reference empacotam pelas
// fórmulas da seção 4.1 da especificação. São escritas AQUI, sem chamar
// a biblioteca, de propósito: são o lado independente que fixa o
// escritor. Se version1.go mudar um deslocamento, estas não acompanham,
// e é assim que o defeito aparece.
func packV1Reference(now uint64, seq uint16, node [6]byte) uuid.UUID {
	var u uuid.UUID
	timeLow := uint32(now)
	timeMid := uint16(now >> 32)
	timeHi := uint16(now>>48) & 0x0FFF
	u[0], u[1], u[2], u[3] = byte(timeLow>>24), byte(timeLow>>16), byte(timeLow>>8), byte(timeLow)
	u[4], u[5] = byte(timeMid>>8), byte(timeMid)
	u[6] = 0x10 | byte(timeHi>>8)
	u[7] = byte(timeHi)
	u[8] = 0x80 | byte(seq>>8)&0x3F
	u[9] = byte(seq)
	copy(u[10:], node[:])
	return u
}

func packV6Reference(now uint64, seq uint16, node [6]byte) uuid.UUID {
	var u uuid.UUID
	timeHigh := uint32(now >> 28)
	timeMid := uint16(now >> 12)
	timeLow := uint16(now) & 0x0FFF
	u[0], u[1], u[2], u[3] = byte(timeHigh>>24), byte(timeHigh>>16), byte(timeHigh>>8), byte(timeHigh)
	u[4], u[5] = byte(timeMid>>8), byte(timeMid)
	u[6] = 0x60 | byte(timeLow>>8)
	u[7] = byte(timeLow)
	u[8] = 0x80 | byte(seq>>8)&0x3F
	u[9] = byte(seq)
	copy(u[10:], node[:])
	return u
}

func packV2Reference(now uint64, seq uint16, node [6]byte, domain uuid.Domain, id uint32) uuid.UUID {
	u := packV1Reference(now, seq, node)
	u[6] = 0x20 | u[6]&0x0F
	u[9] = byte(domain)
	u[0], u[1], u[2], u[3] = byte(id>>24), byte(id>>16), byte(id>>8), byte(id)
	return u
}

// packReference escolhe o empacotamento de referência pela versão, com
// as entradas fixas da tabela.
func packReference(t *testing.T, version byte, now uint64) uuid.UUID {
	t.Helper()
	switch version {
	case 1:
		return packV1Reference(now, goldenSeq, goldenNode)
	case 6:
		return packV6Reference(now, goldenSeq, goldenNode)
	case 2:
		return packV2Reference(now, goldenSeq, goldenNode, goldenDomain, goldenID)
	}
	t.Fatalf("versão %d sem empacotamento de referência", version)
	return uuid.Nil
}

// TestGregorianGoldenVectorsMatchReferencePacker confere que a tabela
// publicada e o empacotamento de referência deste arquivo concordam.
// Liga as duas metades independentes: uma edição errada em qualquer uma
// aparece aqui, antes de qualquer conclusão sobre a biblioteca.
func TestGregorianGoldenVectorsMatchReferencePacker(t *testing.T) {
	for _, v := range gregorianVectors {
		if got := packReference(t, v.version, v.now).String(); got != v.canonica {
			t.Errorf("grupo %s, versão %d: empacotamento de referência %s, tabela %s",
				v.grupo, v.version, got, v.canonica)
		}
	}
}

// TestGregorianGoldenVectorsAreReadCorrectly fixa o LEITOR: os campos
// lidos a partir dos bytes publicados têm de ser os valores de entrada.
func TestGregorianGoldenVectorsAreReadCorrectly(t *testing.T) {
	for _, v := range gregorianVectors {
		u, err := uuid.FromString(v.canonica)
		if err != nil {
			t.Fatalf("grupo %s, versão %d: a string publicada não é canônica: %v", v.grupo, v.version, err)
		}
		if u.Version() != v.version || u.Variant() != 0b10 {
			t.Errorf("grupo %s: versão %d e variante %b, esperado versão %d e variante 10",
				v.grupo, u.Version(), u.Variant(), v.version)
		}
		if node := u.NodeID(); string(node) != string(goldenNode[:]) {
			t.Errorf("grupo %s, versão %d: nó %x, esperado %x", v.grupo, v.version, node, goldenNode)
		}

		seq, ok := u.ClockSequence()
		wantSeq := goldenSeq
		if v.version == 2 {
			wantSeq = goldenSeq >> 8 // só os 6 bits altos sobrevivem ao domínio
		}
		if !ok || seq != wantSeq {
			t.Errorf("grupo %s, versão %d: sequência %#x (ok %v), esperado %#x",
				v.grupo, v.version, seq, ok, wantSeq)
		}

		g, gOK := u.GregorianTime()
		ts, tsOK := u.Timestamp()
		switch v.version {
		case 1, 6:
			if !gOK || uint64(g) != v.now {
				t.Errorf("grupo %s, versão %d: instante gregoriano %d (ok %v), esperado %d",
					v.grupo, v.version, g, gOK, v.now)
			}
			if want := instanteDoGrupo[v.grupo]; !tsOK || !ts.Equal(want) {
				t.Errorf("grupo %s, versão %d: Timestamp %v (ok %v), esperado %v",
					v.grupo, v.version, ts, tsOK, want)
			}
		case 2:
			if gOK || tsOK {
				t.Errorf("grupo %s, versão 2: GregorianTime e Timestamp deveriam devolver falso", v.grupo)
			}
			if domain, ok := u.Domain(); !ok || domain != goldenDomain {
				t.Errorf("grupo %s, versão 2: domínio %v (ok %v), esperado %v", v.grupo, domain, ok, goldenDomain)
			}
			if id, ok := u.ID(); !ok || id != goldenID {
				t.Errorf("grupo %s, versão 2: identificador %d (ok %v), esperado %d", v.grupo, id, ok, goldenID)
			}
		}
	}
}

// TestGregorianGoldenVectorsRelateV1V2V6 confere as relações entre as
// linhas de um mesmo grupo, que é o que torna a tabela mais que três
// vetores soltos: v1 e v6 carregam os mesmos 60 bits em ordem diferente,
// e v2 é o v1 com os bytes 0 a 3 e 9 substituídos.
func TestGregorianGoldenVectorsRelateV1V2V6(t *testing.T) {
	porGrupo := map[string]map[byte]uuid.UUID{}
	for _, v := range gregorianVectors {
		if porGrupo[v.grupo] == nil {
			porGrupo[v.grupo] = map[byte]uuid.UUID{}
		}
		porGrupo[v.grupo][v.version] = uuid.MustParse(v.canonica)
	}
	for grupo, linhas := range porGrupo {
		v1, v6, v2 := linhas[1], linhas[6], linhas[2]
		g1, _ := v1.GregorianTime()
		g6, _ := v6.GregorianTime()
		if g1 != g6 {
			t.Errorf("grupo %s: v1 e v6 leem instantes diferentes, %d e %d", grupo, g1, g6)
		}

		// v2 = v1 com o identificador nos bytes 0..3, versão 2 e domínio
		// no byte 9; tudo o mais, inclusive os 28 bits altos do tempo, a
		// sequência e o nó, fica como está.
		want := v1
		want[0], want[1], want[2], want[3] = 0x00, 0x00, 0x03, 0xE8
		want[6] = 0x20 | want[6]&0x0F
		want[9] = byte(goldenDomain)
		if v2 != want {
			t.Errorf("grupo %s: v2 %s não é o v1 com os campos substituídos, esperado %s", grupo, v2, want)
		}

		// Sequência e nó (bytes 8 a 15) são idênticos entre v1 e v6.
		for i := 8; i < 16; i++ {
			if v1[i] != v6[i] {
				t.Errorf("grupo %s: byte %d difere entre v1 (%#x) e v6 (%#x)", grupo, i, v1[i], v6[i])
			}
		}
	}
}

// TestGregorianGenerationMatchesReferencePacker fixa o ESCRITOR. O
// relógio não é injetável (docs/10-armadilhas-e-decisoes-de-projeto.md
// seção 11.2), então a geração é conferida em duas partes: com a
// sequência e o nó fixados, o UUID gerado carrega esses campos; e o
// instante lido de volta, reempacotado pela função de referência deste
// arquivo, reproduz os 16 bytes. Um deslocamento errado em version1.go e
// em inspect.go ao mesmo tempo passa na ida e volta da biblioteca, mas
// não passa aqui.
func TestGregorianGenerationMatchesReferencePacker(t *testing.T) {
	withIsolatedClockState(t)
	uuid.SetClockSequence(goldenSeq)
	if !uuid.SetNodeID(goldenNode[:]) {
		t.Fatal("SetNodeID recusou o nó de 6 bytes")
	}

	for _, tc := range []struct {
		name     string
		generate func() uuid.UUID
		pack     func(now uint64) uuid.UUID
	}{
		{"GenerateV1", uuid.GenerateV1, func(now uint64) uuid.UUID { return packV1Reference(now, goldenSeq, goldenNode) }},
		{"GenerateV6", uuid.GenerateV6, func(now uint64) uuid.UUID { return packV6Reference(now, goldenSeq, goldenNode) }},
	} {
		u := tc.generate()
		g, ok := u.GregorianTime()
		if !ok {
			t.Fatalf("%s: GregorianTime devolveu falso", tc.name)
		}
		if want := tc.pack(uint64(g)); u != want {
			t.Errorf("%s: gerado %s, reempacotamento de referência %s", tc.name, u, want)
		}
	}

	// Na versão 2 os 32 bits baixos do tempo foram substituídos, então o
	// instante não se recompõe: reempacota-se a partir dos campos de tempo
	// que sobraram (bytes 4 a 7), com time_low em zero.
	u := uuid.GenerateV2(goldenDomain, goldenID)
	partial := uint64(u[6]&0x0F)<<56 | uint64(u[7])<<48 | uint64(u[4])<<40 | uint64(u[5])<<32
	if want := packV2Reference(partial, goldenSeq, goldenNode, goldenDomain, goldenID); u != want {
		t.Errorf("GenerateV2: gerado %s, reempacotamento de referência %s", u, want)
	}
}

// TestGregorianGoldenVectorsCoverAllVersions confere que a tabela não
// perdeu linhas: dois grupos e três versões, sem repetição.
func TestGregorianGoldenVectorsCoverAllVersions(t *testing.T) {
	const esperado = 2 * 3
	if len(gregorianVectors) != esperado {
		t.Fatalf("a tabela tem %d vetores, esperado %d", len(gregorianVectors), esperado)
	}
	visto := make(map[string]bool, esperado)
	for _, v := range gregorianVectors {
		chave := v.grupo + string(rune('0'+v.version))
		if visto[chave] {
			t.Errorf("vetor repetido: grupo %s, versão %d", v.grupo, v.version)
		}
		visto[chave] = true
	}
}
