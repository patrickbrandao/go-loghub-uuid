package tests

import (
	"testing"
	"time"

	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

// checkShape confere versão e variante de um UUID recém-gerado.
func checkShape(t *testing.T, name string, u uuid.UUID, version byte) {
	t.Helper()
	if got := u.Version(); got != version {
		t.Errorf("%s: versão %d, esperado %d", name, got, version)
	}
	if got := u.Variant(); got != 0b10 {
		t.Errorf("%s: variante %d, esperado 2", name, got)
	}
}

// TestAllVersionsHaveCorrectBits confere que cada gerador grava o número
// de versão certo e a variante da RFC, sem exceção.
func TestAllVersionsHaveCorrectBits(t *testing.T) {
	checkShape(t, "GenerateV1", uuid.GenerateV1(), 1)
	checkShape(t, "GenerateV2", uuid.GenerateV2(uuid.Org, 42), 2)
	checkShape(t, "GenerateV2Person", uuid.GenerateV2Person(), 2)
	checkShape(t, "GenerateV2Group", uuid.GenerateV2Group(), 2)
	checkShape(t, "GenerateV3", uuid.GenerateV3(uuid.NameSpaceDNS, []byte("exemplo")), 3)
	checkShape(t, "GenerateV4", uuid.GenerateV4(), 4)
	checkShape(t, "GenerateV5", uuid.GenerateV5(uuid.NameSpaceDNS, []byte("exemplo")), 5)
	checkShape(t, "GenerateV6", uuid.GenerateV6(), 6)
	checkShape(t, "GenerateV8Random", uuid.GenerateV8Random(), 8)

	// A versão 8 é de conteúdo livre: mesmo com todos os bits em um, só a
	// versão e a variante podem ter sido alteradas.
	var full [16]byte
	for i := range full {
		full[i] = 0xFF
	}
	checkShape(t, "GenerateV8", uuid.GenerateV8(full), 8)

	// Os três níveis desta biblioteca continuam produzindo versão 7.
	for _, level := range []uuid.Level{uuid.Level1, uuid.Level2, uuid.Level3} {
		checkShape(t, "Generate", uuid.Generate(level), 7)
	}
}

// TestV8PreservesCallerBits confere que a versão 8 respeita os 122 bits
// entregues pelo chamador, mexendo apenas em versão e variante.
func TestV8PreservesCallerBits(t *testing.T) {
	var data [16]byte
	for i := range data {
		data[i] = byte(i)
	}
	u := uuid.GenerateV8(data)

	for i := 0; i < 16; i++ {
		if i == 6 || i == 8 {
			continue
		}
		if u[i] != data[i] {
			t.Errorf("byte %d alterado: %#02x, esperado %#02x", i, u[i], data[i])
		}
	}
	if u[6]&0x0F != data[6]&0x0F {
		t.Error("byte 6: os 4 bits baixos deveriam ter sido preservados")
	}
	if u[8]&0x3F != data[8]&0x3F {
		t.Error("byte 8: os 6 bits baixos deveriam ter sido preservados")
	}
}

// TestNameBasedVectors confere as versões 3 e 5 contra os vetores de
// teste publicados na RFC 9562. São valores fixos: qualquer divergência
// significa que a derivação quebrou.
func TestNameBasedVectors(t *testing.T) {
	const name = "www.example.com"

	if got := uuid.GenerateV3(uuid.NameSpaceDNS, []byte(name)).String(); got != "5df41881-3aed-3515-88a7-2f4a814cf09e" {
		t.Errorf("GenerateV3: %s, divergente do vetor da RFC 9562", got)
	}
	if got := uuid.GenerateV5(uuid.NameSpaceDNS, []byte(name)).String(); got != "2ed6657d-e927-568b-95e1-2665a8aea6a2" {
		t.Errorf("GenerateV5: %s, divergente do vetor da RFC 9562", got)
	}
}

// TestNameBasedIsDeterministic confere que as versões 3 e 5 não usam
// entropia: o mesmo par de espaço e nome sempre devolve o mesmo UUID.
func TestNameBasedIsDeterministic(t *testing.T) {
	a := uuid.GenerateV5(uuid.NameSpaceURL, []byte("https://exemplo.com.br"))
	b := uuid.GenerateV5(uuid.NameSpaceURL, []byte("https://exemplo.com.br"))
	if a != b {
		t.Error("GenerateV5 devolveu valores diferentes para a mesma entrada")
	}

	c := uuid.GenerateV5(uuid.NameSpaceOID, []byte("https://exemplo.com.br"))
	if a == c {
		t.Error("espaços de nomes distintos deveriam produzir UUIDs distintos")
	}
}

// TestNamespacesAreDistinct confere que os quatro espaços de nomes bem
// conhecidos foram transcritos corretamente e não colidem.
func TestNamespacesAreDistinct(t *testing.T) {
	spaces := map[string]uuid.UUID{
		"DNS":  uuid.NameSpaceDNS,
		"URL":  uuid.NameSpaceURL,
		"OID":  uuid.NameSpaceOID,
		"X500": uuid.NameSpaceX500,
	}
	seen := make(map[uuid.UUID]string, len(spaces))
	for name, u := range spaces {
		if u.IsZero() {
			t.Errorf("espaço de nomes %s ficou nulo", name)
		}
		if other, dup := seen[u]; dup {
			t.Errorf("espaços de nomes %s e %s são iguais", name, other)
		}
		seen[u] = name
	}
}

// TestTimeBasedUniqueness confere que o relógio interno das versões 1 e 6
// avança a cada chamada, mesmo em rajada dentro do mesmo tique.
func TestTimeBasedUniqueness(t *testing.T) {
	const total = 50_000

	for _, tc := range []struct {
		name     string
		generate func() uuid.UUID
	}{
		{"GenerateV1", uuid.GenerateV1},
		{"GenerateV6", uuid.GenerateV6},
	} {
		seen := make(map[uuid.UUID]struct{}, total)
		for i := 0; i < total; i++ {
			u := tc.generate()
			if _, dup := seen[u]; dup {
				t.Fatalf("%s: repetição na geração %d", tc.name, i)
			}
			seen[u] = struct{}{}
		}
	}
}

// TestV6IsLexicographicallyOrdered confere a promessa central da versão
// 6: como o tempo vem do bit mais significativo para o menos, a ordem das
// strings é a ordem de criação. O relógio interno é estritamente
// crescente, portanto aqui não há empate tolerado.
func TestV6IsLexicographicallyOrdered(t *testing.T) {
	previous := uuid.GenerateV6().String()
	for i := 0; i < 10_000; i++ {
		current := uuid.GenerateV6().String()
		if current <= previous {
			t.Fatalf("regressão de ordem na geração %d: %s depois de %s", i, current, previous)
		}
		previous = current
	}
}

// TestTimestampRoundTrip confere que o instante extraído das versões 1, 6
// e 7 fica próximo do relógio do sistema no momento da geração.
func TestTimestampRoundTrip(t *testing.T) {
	// Os testes de volume anteriores deixam o relógio interno adiantado:
	// cada geração dentro do mesmo tique avança 100 nanossegundos. Trocar
	// a sequência de relógio zera esse acúmulo, exatamente como aconteceria
	// se o relógio do sistema tivesse voltado. Veja TestTimeBasedClockDrift.
	uuid.SetClockSequence(0x0001)
	uuid.SetClockSequence(0x0002)
	defer uuid.SetClockSequence(-1)

	for _, tc := range []struct {
		name     string
		generate func() uuid.UUID
	}{
		{"versão 1", uuid.GenerateV1},
		{"versão 6", uuid.GenerateV6},
		{"versão 7", func() uuid.UUID { return uuid.Generate(uuid.Level1) }},
	} {
		before := time.Now().Add(-2 * time.Millisecond)
		u := tc.generate()
		after := time.Now().Add(2 * time.Millisecond)

		got, ok := u.Timestamp()
		if !ok {
			t.Errorf("%s: Timestamp devolveu falso", tc.name)
			continue
		}
		if got.Before(before) || got.After(after) {
			t.Errorf("%s: instante %v fora da janela [%v, %v]", tc.name, got, before, after)
		}
	}
}

// TestTimestampRejectsVersionsWithoutTime confere que as versões sem
// carimbo de tempo devolvem falso em vez de um instante inventado.
func TestTimestampRejectsVersionsWithoutTime(t *testing.T) {
	cases := map[string]uuid.UUID{
		"versão 2": uuid.GenerateV2(uuid.Person, 1),
		"versão 3": uuid.GenerateV3(uuid.NameSpaceDNS, []byte("x")),
		"versão 4": uuid.GenerateV4(),
		"versão 5": uuid.GenerateV5(uuid.NameSpaceDNS, []byte("x")),
		"versão 8": uuid.GenerateV8Random(),
	}
	for name, u := range cases {
		if _, ok := u.Timestamp(); ok {
			t.Errorf("%s: Timestamp deveria devolver falso", name)
		}
	}
}

// TestTimestampWithLevelRecoversSubMillisecond confere que a precisão
// gravada pelos níveis 2 e 3 é recuperada como time.Time.
func TestTimestampWithLevelRecoversSubMillisecond(t *testing.T) {
	for _, level := range []uuid.Level{uuid.Level2, uuid.Level3} {
		u := uuid.Generate(level)

		coarse, ok := u.Timestamp()
		if !ok {
			t.Fatalf("nível %d: Timestamp devolveu falso", level)
		}
		fine, ok := u.TimestampWithLevel(level)
		if !ok {
			t.Fatalf("nível %d: TimestampWithLevel devolveu falso", level)
		}

		delta := fine.Sub(coarse)
		if delta < 0 || delta >= time.Millisecond {
			t.Errorf("nível %d: refinamento de %v fora da faixa de um milissegundo", level, delta)
		}

		imported := uuid.ImportBinary(u)
		if got := fine.Nanosecond() / 1_000 % 1_000; got != imported.Microseconds {
			t.Errorf("nível %d: microssegundos %d, esperado %d", level, got, imported.Microseconds)
		}
	}
}

// TestClockSequenceIsRecoverable confere que a sequência fixada pelo
// chamador aparece nos UUIDs gerados em seguida.
func TestClockSequenceIsRecoverable(t *testing.T) {
	uuid.SetClockSequence(0x1234)
	defer uuid.SetClockSequence(-1)

	if got := uuid.ClockSequence(); got != 0x1234 {
		t.Fatalf("ClockSequence: %#x, esperado %#x", got, 0x1234)
	}
	for _, u := range []uuid.UUID{uuid.GenerateV1(), uuid.GenerateV6()} {
		got, ok := u.ClockSequence()
		if !ok {
			t.Fatalf("versão %d: ClockSequence devolveu falso", u.Version())
		}
		if got != 0x1234 {
			t.Errorf("versão %d: sequência %#x, esperado %#x", u.Version(), got, 0x1234)
		}
	}
}

// TestNodeIDIsRecoverable confere que o identificador de nó fixado pelo
// chamador aparece nos UUIDs gerados em seguida, e que NodeID devolve uma
// cópia, não o estado interno.
func TestNodeIDIsRecoverable(t *testing.T) {
	wanted := []byte{0x02, 0x11, 0x22, 0x33, 0x44, 0x55}
	if !uuid.SetNodeID(wanted) {
		t.Fatal("SetNodeID recusou 6 bytes válidos")
	}
	if uuid.SetNodeID([]byte{1, 2, 3}) {
		t.Error("SetNodeID deveria recusar menos de 6 bytes")
	}

	got := uuid.NodeID()
	got[0] = 0xFF // alterar a cópia não pode afetar o estado interno
	if again := uuid.NodeID(); again[0] != 0x02 {
		t.Error("NodeID devolveu o estado interno em vez de uma cópia")
	}

	for _, u := range []uuid.UUID{uuid.GenerateV1(), uuid.GenerateV6()} {
		node := u.NodeID()
		if len(node) != 6 {
			t.Fatalf("versão %d: NodeID devolveu %d bytes", u.Version(), len(node))
		}
		for i := range wanted {
			if node[i] != wanted[i] {
				t.Errorf("versão %d: nó %#v, esperado %#v", u.Version(), node, wanted)
				break
			}
		}
	}
}

// TestV2CarriesDomainAndID confere que a versão 2 grava e devolve o
// domínio e o identificador local.
func TestV2CarriesDomainAndID(t *testing.T) {
	u := uuid.GenerateV2(uuid.Group, 0xDEADBEEF)

	domain, ok := u.Domain()
	if !ok || domain != uuid.Group {
		t.Errorf("Domain: %v, ok=%v, esperado Group", domain, ok)
	}
	id, ok := u.ID()
	if !ok || id != 0xDEADBEEF {
		t.Errorf("ID: %#x, ok=%v, esperado %#x", id, ok, 0xDEADBEEF)
	}
	if _, ok := uuid.GenerateV4().Domain(); ok {
		t.Error("Domain deveria devolver falso para a versão 4")
	}

	// A sequência da versão 2 tem só 6 bits: o byte baixo virou domínio.
	uuid.SetClockSequence(0x1234)
	defer uuid.SetClockSequence(-1)
	seq, ok := uuid.GenerateV2(uuid.Org, 1).ClockSequence()
	if !ok || seq != 0x1234>>8 {
		t.Errorf("ClockSequence na versão 2: %#x, ok=%v, esperado %#x", seq, ok, 0x1234>>8)
	}
}

// TestV4Uniqueness confere que a versão 4 não repete valores em volume.
func TestV4Uniqueness(t *testing.T) {
	const total = 200_000
	seen := make(map[uuid.UUID]struct{}, total)
	for i := 0; i < total; i++ {
		u := uuid.GenerateV4()
		if _, dup := seen[u]; dup {
			t.Fatalf("repetição na geração %d", i)
		}
		seen[u] = struct{}{}
	}
}

// TestTimeBasedClockDrift documenta o custo da ordem estrita nas versões
// 1 e 6: quando a geração é mais rápida que o tique de 100 nanossegundos,
// o relógio interno avança sozinho e o instante embutido fica à frente do
// relógio do sistema. A RFC 9562 permite essa estratégia. Ela difere da
// do pacote google/uuid, que mantém o instante do relógio de parede e
// incrementa a sequência de relógio de 14 bits quando o tempo repete ou
// regride: lá não há deriva, mas também não há ordem estrita no UUIDv6.
//
// O teste confere que o adiantamento existe, que ele é limitado ao número
// de tiques consumidos e que a ordem nunca regride.
func TestTimeBasedClockDrift(t *testing.T) {
	const burst = 200_000

	uuid.SetClockSequence(0x0003)
	uuid.SetClockSequence(0x0004)
	defer uuid.SetClockSequence(-1)

	previous := uuid.GenerateV6()
	for i := 1; i < burst; i++ {
		current := uuid.GenerateV6()
		if current.Compare(previous) <= 0 {
			t.Fatalf("regressão de ordem na geração %d", i)
		}
		previous = current
	}

	embedded, ok := previous.Timestamp()
	if !ok {
		t.Fatal("Timestamp devolveu falso")
	}
	drift := embedded.Sub(time.Now())

	// Cada geração consome no máximo um tique de 100 nanossegundos, então
	// o adiantamento não pode passar do total de tiques da rajada.
	limit := time.Duration(burst) * 100 * time.Nanosecond
	if drift > limit {
		t.Errorf("adiantamento de %v acima do teto de %v", drift, limit)
	}
	t.Logf("adiantamento de %v apos %d geracoes", drift, burst)
}

// TestSetNodeIDSameNodeKeepsUniqueness confere que reaplicar o nó já em
// uso, logo após uma rajada, não repete UUIDs de versão 1.
func TestSetNodeIDSameNodeKeepsUniqueness(t *testing.T) {
	const burst = 200_000
	uuid.SetClockSequence(-1)
	defer uuid.SetClockSequence(-1)

	seen := make(map[uuid.UUID]struct{}, burst+1_000)
	for i := 0; i < burst; i++ {
		seen[uuid.GenerateV1()] = struct{}{}
	}
	if !uuid.SetNodeID(uuid.NodeID()) {
		t.Fatal("SetNodeID recusou o nó em uso")
	}
	for i := 0; i < 1_000; i++ {
		u := uuid.GenerateV1()
		if _, dup := seen[u]; dup {
			t.Fatalf("repetição na geração %d após SetNodeID com o mesmo nó", i)
		}
		seen[u] = struct{}{}
	}
}

// TestV2RepeatsWithinWindow documenta que a versão 2 identifica um
// principal, não um evento: com os mesmos argumentos, chamadas feitas
// dentro da mesma janela de cerca de sete minutos devolvem o mesmo UUID.
// Por isso a versão 2 fica fora de TestTimeBasedUniqueness.
func TestV2RepeatsWithinWindow(t *testing.T) {
	a := uuid.GenerateV2(uuid.Org, 4242)
	b := uuid.GenerateV2(uuid.Org, 4242)
	if a != b {
		// Só pode acontecer se a chamada cruzou a fronteira de 2^32 tiques
		// ou se outro teste trocou a sequência ou o nó em paralelo.
		t.Skipf("janela de sete minutos cruzada entre as chamadas: %s e %s", a, b)
	}
	if c := uuid.GenerateV2(uuid.Org, 4243); c == a {
		t.Error("identificadores distintos deveriam produzir UUIDs distintos")
	}
}

// TestGetTimeSequenceFormat fixa o formato do segundo retorno de GetTime:
// 14 bits de sequência iguais aos de ClockSequence, com a variante RFC
// nos dois bits altos.
func TestGetTimeSequenceFormat(t *testing.T) {
	uuid.SetClockSequence(0x1234)
	defer uuid.SetClockSequence(-1)

	_, seq := uuid.GetTime()
	if int(seq&0x3FFF) != uuid.ClockSequence() {
		t.Errorf("GetTime: sequência %#x, ClockSequence %#x", seq&0x3FFF, uuid.ClockSequence())
	}
	if seq&0xC000 != 0x8000 {
		t.Errorf("GetTime: bits de variante %#x, esperado 0x8000", seq&0xC000)
	}
}
