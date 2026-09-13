package tests

import (
	"math"
	"sync"
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
	checkShape(t, "GenerateV7", uuid.GenerateV7(), 7)
	checkShape(t, "GenerateV7Level1", uuid.GenerateV7Level1(), 7)
	checkShape(t, "GenerateV7Level2", uuid.GenerateV7Level2(), 7)
	checkShape(t, "GenerateV7Level3", uuid.GenerateV7Level3(), 7)
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

// TestNamespacesMatchRFC confere os quatro espaços de nomes contra a
// tabela 3 da seção 6.6 da RFC 9562. Um dígito trocado na transcrição
// muda em silêncio todo UUID de versão 3 e 5 derivado do espaço, e só o
// espaço DNS tem vetor de geração publicado (TestNameBasedVectors).
// Valores fixos implicam também que nenhum é nulo e que não colidem.
func TestNamespacesMatchRFC(t *testing.T) {
	for _, tc := range []struct {
		name  string
		space uuid.UUID
		want  string
	}{
		{"NameSpaceDNS", uuid.NameSpaceDNS, "6ba7b810-9dad-11d1-80b4-00c04fd430c8"},
		{"NameSpaceURL", uuid.NameSpaceURL, "6ba7b811-9dad-11d1-80b4-00c04fd430c8"},
		{"NameSpaceOID", uuid.NameSpaceOID, "6ba7b812-9dad-11d1-80b4-00c04fd430c8"},
		{"NameSpaceX500", uuid.NameSpaceX500, "6ba7b814-9dad-11d1-80b4-00c04fd430c8"},
	} {
		if got := tc.space.String(); got != tc.want {
			t.Errorf("%s: %s, divergente da RFC 9562 (esperado %s)", tc.name, got, tc.want)
		}
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
	withIsolatedClockState(t)

	// Os testes de volume anteriores deixam o relógio interno adiantado:
	// cada geração dentro do mesmo tique avança 100 nanossegundos. Sortear
	// uma sequência inédita (-1) descarta esse acúmulo, que é a forma
	// prevista para ressincronizar com o relógio do sistema. Veja
	// TestTimeBasedClockDrift.
	uuid.SetClockSequence(-1)

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

// TestTimestampWithLevelDiscardsOutOfRangeFields confere que, quando os
// campos sub-milissegundo denunciam bits aleatórios (fora de 0..999),
// TimestampWithLevel devolve apenas o milissegundo, sem somar o campo que
// por acaso ainda caiba na faixa.
func TestTimestampWithLevelDiscardsOutOfRangeFields(t *testing.T) {
	// UUIDv7 montado à mão: rand_a = 0xFFF (4095, fora da faixa) e
	// topo de rand_b = 5 nanossegundos (dentro da faixa).
	base := uuid.UUID{
		0x01, 0x92, 0xf7, 0xc5, 0x1a, 0x2b, // unix_ts_ms
		0x7F, 0xFF, // versão 7 + rand_a = 0xFFF
		0x80, 0x50, // variante 10 + nano = (0x00 << 4) | (0x50 >> 4) = 5
		0, 0, 0, 0, 0, 0,
	}
	coarse, ok := base.Timestamp()
	if !ok {
		t.Fatal("Timestamp devolveu falso para um UUIDv7")
	}

	for _, level := range []uuid.Level{uuid.Level2, uuid.Level3} {
		fine, ok := base.TimestampWithLevel(level)
		if !ok {
			t.Fatalf("nível %d: TimestampWithLevel devolveu falso", level)
		}
		if !fine.Equal(coarse) {
			t.Errorf("nível %d: rand_a fora da faixa deveria devolver só o milissegundo; obtido %v, esperado %v",
				level, fine, coarse)
		}
	}

	// Caso simétrico: micro válido, nano fora da faixa (1023) no nível 3.
	other := base
	other[6], other[7] = 0x70, 0x07 // rand_a = 7 microssegundos
	other[8], other[9] = 0xBF, 0xF0 // nano = (0x3F << 4) | 0xF = 1023
	fine, _ := other.TimestampWithLevel(uuid.Level3)
	if !fine.Equal(coarse) {
		t.Errorf("nível 3 com nano fora da faixa deveria devolver só o milissegundo; obtido %v", fine)
	}
	// No nível 2 o nano é ignorado, então os 7 microssegundos valem.
	fine, _ = other.TimestampWithLevel(uuid.Level2)
	if want := coarse.Add(7 * time.Microsecond); !fine.Equal(want) {
		t.Errorf("nível 2 deveria somar 7 microssegundos; obtido %v, esperado %v", fine, want)
	}

	// Caso válido nos dois campos: 7 microssegundos e 5 nanossegundos.
	valid := base
	valid[6], valid[7] = 0x70, 0x07
	fine, _ = valid.TimestampWithLevel(uuid.Level3)
	if want := coarse.Add(7*time.Microsecond + 5*time.Nanosecond); !fine.Equal(want) {
		t.Errorf("nível 3 válido deveria somar 7 us e 5 ns; obtido %v, esperado %v", fine, want)
	}
}

// TestClockSequenceIsRecoverable confere que a sequência fixada pelo
// chamador aparece nos UUIDs gerados em seguida.
func TestClockSequenceIsRecoverable(t *testing.T) {
	withIsolatedClockState(t)
	uuid.SetClockSequence(0x1234)

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
	withIsolatedClockState(t)

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
	withIsolatedClockState(t)

	// Tipado explicitamente: o literal sem sinal, passado direto a uma
	// função variádica de interface, defasa para "int" e estoura em
	// plataformas de 32 bits (0xDEADBEEF excede math.MaxInt32). O
	// identificador local da versão 2 é sempre 32 bits sem sinal.
	const testID = uint32(0xDEADBEEF)

	u := uuid.GenerateV2(uuid.Group, testID)

	domain, ok := u.Domain()
	if !ok || domain != uuid.Group {
		t.Errorf("Domain: %v, ok=%v, esperado Group", domain, ok)
	}
	id, ok := u.ID()
	if !ok || id != testID {
		t.Errorf("ID: %#x, ok=%v, esperado %#x", id, ok, testID)
	}
	if _, ok := uuid.GenerateV4().Domain(); ok {
		t.Error("Domain deveria devolver falso para a versão 4")
	}

	// A sequência da versão 2 tem só 6 bits: o byte baixo virou domínio.
	uuid.SetClockSequence(0x1234)
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
	withIsolatedClockState(t)

	// Sequência inédita: começa sem adiantamento acumulado.
	uuid.SetClockSequence(-1)

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
	drift := time.Until(embedded)

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
	withIsolatedClockState(t)
	uuid.SetClockSequence(-1)

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

// TestClockSequenceReuseNeverRepeats confere a invariante que garante a
// unicidade dos UUIDv1/v6: para cada sequência de relógio, os instantes
// emitidos são estritamente crescentes durante toda a vida do processo,
// mesmo quando o chamador alterna entre sequências.
//
// REGRESSÃO: antes da correção, qualquer troca de sequência zerava o piso
// do relógio interno. Uma rajada com a sequência A adiantava o relógio;
// trocar para B e voltar para A zerava o piso e os próximos UUIDs de A
// recebiam instantes já emitidos, repetindo valores.
func TestClockSequenceReuseNeverRepeats(t *testing.T) {
	const burst = 200_000
	const seqA, seqB = 0x0AAA, 0x0BBB
	withIsolatedClockState(t)

	seen := make(map[uuid.UUID]struct{}, burst+2_000)
	record := func(name string, u uuid.UUID) {
		t.Helper()
		if _, dup := seen[u]; dup {
			t.Fatalf("%s: UUID repetido %s", name, u)
		}
		seen[u] = struct{}{}
	}

	// Rajada com A: o relógio interno fica adiantado em até 20 ms.
	uuid.SetClockSequence(seqA)
	var lastA uuid.UUID
	for i := 0; i < burst; i++ {
		lastA = uuid.GenerateV6()
		record("rajada A", lastA)
	}
	lastTimeA, _ := lastA.GregorianTime()

	// B é inédita: o piso é descartado e B volta a acompanhar o relógio.
	uuid.SetClockSequence(seqB)
	for i := 0; i < 1_000; i++ {
		record("rajada B", uuid.GenerateV6())
	}

	// De volta a A: o piso de A precisa ser restaurado. Nenhum UUID pode
	// repetir e o primeiro instante tem de superar o último emitido com A.
	uuid.SetClockSequence(seqA)
	first := uuid.GenerateV6()
	record("retorno a A", first)
	if firstTime, _ := first.GregorianTime(); firstTime <= lastTimeA {
		t.Fatalf("ao voltar para a sequência A o instante regrediu: %d depois de %d", firstTime, lastTimeA)
	}
	if seq, _ := first.ClockSequence(); seq != seqA {
		t.Fatalf("sequência gravada %#x, esperado %#x", seq, seqA)
	}
	for i := 0; i < 1_000; i++ {
		record("retorno a A", uuid.GenerateV6())
	}
}

// TestSetClockSequenceRandomIsFresh confere que SetClockSequence(-1)
// sempre sorteia uma sequência ainda não usada neste processo, e que só
// essa troca para uma sequência inédita descarta o adiantamento.
func TestSetClockSequenceRandomIsFresh(t *testing.T) {
	withIsolatedClockState(t)

	used := map[int]bool{}
	for _, explicit := range []int{0x0001, 0x0002, 0x0003} {
		uuid.SetClockSequence(explicit)
		uuid.GenerateV1()
		used[uuid.ClockSequence()] = true
	}
	for i := 0; i < 50; i++ {
		uuid.SetClockSequence(-1)
		seq := uuid.ClockSequence()
		if used[seq] {
			t.Fatalf("sorteio %d devolveu a sequência %#x, que já tinha sido usada", i, seq)
		}
		used[seq] = true
		uuid.GenerateV1()
	}

	// Voltar a uma sequência antiga mantém o piso dela: o instante do
	// próximo UUID supera o último emitido com ela.
	uuid.SetClockSequence(0x0001)
	before := uuid.GenerateV1()
	for i := 0; i < 10_000; i++ {
		uuid.GenerateV1()
	}
	last := uuid.GenerateV1()
	uuid.SetClockSequence(0x0002)
	uuid.GenerateV1()
	uuid.SetClockSequence(0x0001)
	after := uuid.GenerateV1()

	beforeTime, _ := before.GregorianTime()
	lastTime, _ := last.GregorianTime()
	afterTime, _ := after.GregorianTime()
	if beforeTime >= lastTime || lastTime >= afterTime {
		t.Fatalf("instantes da sequência 0x0001 não são estritamente crescentes: %d, %d, %d",
			beforeTime, lastTime, afterTime)
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
	withIsolatedClockState(t)
	uuid.SetClockSequence(0x1234)

	_, seq := uuid.GetTime()
	if int(seq&0x3FFF) != uuid.ClockSequence() {
		t.Errorf("GetTime: sequência %#x, ClockSequence %#x", seq&0x3FFF, uuid.ClockSequence())
	}
	if seq&0xC000 != 0x8000 {
		t.Errorf("GetTime: bits de variante %#x, esperado 0x8000", seq&0xC000)
	}
}

// TestDomainString confere a descrição em texto dos três domínios da
// versão 2 e de um valor fora da lista.
func TestDomainString(t *testing.T) {
	cases := map[uuid.Domain]string{
		uuid.Person:     "pessoa",
		uuid.Group:      "grupo",
		uuid.Org:        "organizacao",
		uuid.Domain(7):  "dominio 7",
		uuid.Domain(42): "dominio 42",
	}
	for domain, want := range cases {
		if got := domain.String(); got != want {
			t.Errorf("Domain(%d).String() = %q, esperado %q", byte(domain), got, want)
		}
	}
}

// TestInspectorsRejectOtherVersions confere que os leitores de campos das
// versões baseadas em relógio devolvem falso, ou nulo, para as versões
// que não carregam aquele campo, e que a versão 2 devolve os 6 bits de
// sequência que preserva.
func TestInspectorsRejectOtherVersions(t *testing.T) {
	others := map[string]uuid.UUID{
		"v3": uuid.GenerateV3(uuid.NameSpaceDNS, []byte("x")),
		"v4": uuid.GenerateV4(),
		"v5": uuid.GenerateV5(uuid.NameSpaceDNS, []byte("x")),
		"v7": uuid.Generate(uuid.Level1),
		"v8": uuid.GenerateV8Random(),
	}
	for name, u := range others {
		// A versão 7 entra no mapa porque não carrega sequência, nó,
		// domínio nem identificador local, mas é a única que carrega
		// tempo Unix: a leitura por nível só devolve falso nas outras.
		if name != "v7" {
			if _, ok := u.TimestampWithLevel(uuid.Level3); ok {
				t.Errorf("%s: TimestampWithLevel deveria devolver falso", name)
			}
		}
		if _, ok := u.ClockSequence(); ok {
			t.Errorf("%s: ClockSequence deveria devolver falso", name)
		}
		if node := u.NodeID(); node != nil {
			t.Errorf("%s: NodeID deveria devolver nulo, veio %x", name, node)
		}
		if _, ok := u.Domain(); ok {
			t.Errorf("%s: Domain deveria devolver falso", name)
		}
		if _, ok := u.ID(); ok {
			t.Errorf("%s: ID deveria devolver falso", name)
		}
	}

	// A versão 2 preserva só os 6 bits altos da sequência (o byte baixo é
	// o domínio), e ClockSequence devolve exatamente esses 6 bits.
	v2 := uuid.GenerateV2(uuid.Org, 4242)
	seq, ok := v2.ClockSequence()
	if !ok || seq != int(v2[8]&0x3F) || seq > 63 {
		t.Errorf("v2: ClockSequence = %d, ok %v, esperado os 6 bits baixos do byte 8 (%d)", seq, ok, v2[8]&0x3F)
	}
	if node := v2.NodeID(); len(node) != 6 {
		t.Errorf("v2: NodeID com %d bytes, esperado 6", len(node))
	}
}

// TestGregorianUnixTimeBeforeEpochIsCanonical trava a conversão inversa
// da seção 4.1 da especificação (caso obrigatório 19): o par devolvido
// por UnixTime é canônico, com nanossegundos em 0..999_999_999, também
// para carimbos anteriores à época Unix. É o único caso que distingue a
// divisão euclidiana da truncada, que devolveria (0, -100) para um tique
// antes da época; instantes posteriores a 1970 passam nas duas.
func TestGregorianUnixTimeBeforeEpochIsCanonical(t *testing.T) {
	const gregorianOffset = 0x01B21DD213814000

	cases := []struct {
		name string
		g    uuid.GregorianTime
		sec  int64
		nsec int64
	}{
		{"época gregoriana de 1582", 0, -12_219_292_800, 0},
		{"um tique antes da época Unix", gregorianOffset - 1, -1, 999_999_900},
		{"meio segundo antes da época Unix", gregorianOffset - 5_000_000, -1, 500_000_000},
		{"1969-07-20T20:17:40Z", gregorianOffset - 14_182_940*10_000_000, -14_182_940, 0},
		{"época Unix", gregorianOffset, 0, 0},
		{"um tique depois da época Unix", gregorianOffset + 1, 0, 100},
		{"2026-01-01T00:00:00.1234567Z", 139_865_184_001_234_567, 1_767_225_600, 123_456_700},
	}
	for _, c := range cases {
		sec, nsec := c.g.UnixTime()
		if sec != c.sec || nsec != c.nsec {
			t.Errorf("%s: UnixTime = (%d, %d), esperado (%d, %d)", c.name, sec, nsec, c.sec, c.nsec)
		}
		if nsec < 0 || nsec >= 1_000_000_000 {
			t.Errorf("%s: nanossegundos %d fora da faixa canônica", c.name, nsec)
		}
		// O instante construído a partir do par é o de Time(), e a ida da
		// seção 4.1 anula a volta.
		if got, want := time.Unix(sec, nsec).UTC(), c.g.Time(); !got.Equal(want) {
			t.Errorf("%s: time.Unix(par) = %v, Time() = %v", c.name, got, want)
		}
		if back := sec*10_000_000 + nsec/100 + gregorianOffset; back != int64(c.g) {
			t.Errorf("%s: ida e volta devolveu %d, esperado %d", c.name, back, int64(c.g))
		}
	}

	// Montados com carimbo pré-época pelos empacotamentos de referência de
	// golden_test.go, um UUIDv1 e um UUIDv6 devolvem o mesmo instante
	// canônico pela leitura da biblioteca.
	const umTiqueAntes = gregorianOffset - 1
	want := time.Date(1969, 12, 31, 23, 59, 59, 999_999_900, time.UTC)
	for name, u := range map[string]uuid.UUID{
		"versão 1": packV1Reference(umTiqueAntes, goldenSeq, goldenNode),
		"versão 6": packV6Reference(umTiqueAntes, goldenSeq, goldenNode),
	} {
		g, ok := u.GregorianTime()
		if !ok || int64(g) != umTiqueAntes {
			t.Errorf("%s: GregorianTime = %d (ok %v), esperado %d", name, g, ok, int64(umTiqueAntes))
		}
		if sec, nsec := g.UnixTime(); sec != -1 || nsec != 999_999_900 {
			t.Errorf("%s: UnixTime = (%d, %d), esperado (-1, 999999900)", name, sec, nsec)
		}
		if ts, ok := u.Timestamp(); !ok || !ts.Equal(want) {
			t.Errorf("%s: Timestamp = %v (ok %v), esperado %v", name, ts, ok, want)
		}
	}
}

// TestGregorianUnixTimeOverflowSaturation confere que valores de
// GregorianTime abaixo do limiar minGregorianTicks não causam estouro de
// int64 (o que devolveria um instante futuro no ano ~30.800), mas sim
// saturam no piso representável por math.MinInt64.
func TestGregorianUnixTimeOverflowSaturation(t *testing.T) {
	const gregorian100ns = int64(122_192_928_000_000_000)
	const minGregorianTicks = math.MinInt64 + gregorian100ns

	// Instante saturado: piso de math.MinInt64.
	// -9223372036854775808 / 10000000 = -922337203685, rem = -4775808 -> sec = -922337203686, nsec = 522419200
	const wantSec = int64(-922_337_203_686)
	const wantNsec = int64(522_419_200)

	cases := []struct {
		name string
		g    uuid.GregorianTime
		sec  int64
		nsec int64
	}{
		{"mínimo absoluto int64", uuid.GregorianTime(math.MinInt64), wantSec, wantNsec},
		{"um abaixo do limiar (satura)", uuid.GregorianTime(minGregorianTicks - 1), wantSec, wantNsec},
		{"limiar exato (não satura)", uuid.GregorianTime(minGregorianTicks), wantSec, wantNsec},
		{"um acima do limiar (não satura)", uuid.GregorianTime(minGregorianTicks + 1), wantSec, wantNsec + 100},
	}

	for _, c := range cases {
		sec, nsec := c.g.UnixTime()
		if sec != c.sec || nsec != c.nsec {
			t.Errorf("%s: UnixTime = (%d, %d), esperado (%d, %d)", c.name, sec, nsec, c.sec, c.nsec)
		}
		if nsec < 0 || nsec >= 1_000_000_000 {
			t.Errorf("%s: nanossegundos %d fora da faixa canônica", c.name, nsec)
		}
		if nsec%100 != 0 {
			t.Errorf("%s: nanossegundos %d não é múltiplo de 100", c.name, nsec)
		}
		if sec > 0 {
			t.Errorf("%s: sec %d é positivo (estouro de int64)", c.name, sec)
		}
		if got, want := c.g.Time(), time.Unix(sec, nsec).UTC(); !got.Equal(want) {
			t.Errorf("%s: Time() = %v, esperado %v", c.name, got, want)
		}
	}
}

// TestTimeBasedConcurrentUniqueness gera UUIDv1, v2 e v6 de várias
// goroutines simultâneas e confere ausência de repetição. É o par
// concorrente de TestTimeBasedUniqueness (serial) e de
// TestConcurrentUniqueness em generation_test.go, que só cobre a
// versão 7: sem este teste, remover a trava de clock.go que protege o
// relógio e o nó compartilhados (clockMu) passa despercebido, inclusive
// sob o detector de corrida, porque nenhum teste anterior chamava
// GenerateV1/V2/V6 de mais de uma goroutine ao mesmo tempo. Rode também
// com -race:
//
//	go test ./tests/ -race -run TestTimeBasedConcurrentUniqueness
func TestTimeBasedConcurrentUniqueness(t *testing.T) {
	withIsolatedClockState(t)
	uuid.SetClockSequence(-1)

	const goroutines = 32
	const perGoroutine = 5_000

	for name, generate := range map[string]func() uuid.UUID{
		"GenerateV1": uuid.GenerateV1,
		"GenerateV6": uuid.GenerateV6,
		"GenerateV2": func() uuid.UUID { return uuid.GenerateV2(uuid.Org, 1) },
	} {
		results := make([][]uuid.UUID, goroutines)
		var wg sync.WaitGroup
		for w := 0; w < goroutines; w++ {
			wg.Add(1)
			go func(w int) {
				defer wg.Done()
				batch := make([]uuid.UUID, perGoroutine)
				for i := range batch {
					batch[i] = generate()
				}
				results[w] = batch
			}(w)
		}
		wg.Wait()

		// A versão 2 sacrifica os 32 bits baixos do tempo pelo
		// identificador local fixo, então repete dentro da janela de
		// sete minutos (TestV2RepeatsWithinWindow, documentado): a
		// unicidade exigida aqui é só de forma (versão e variante), não
		// de valor.
		if name == "GenerateV2" {
			for _, batch := range results {
				for _, u := range batch {
					if u.Version() != 2 || u.Variant() != 0b10 {
						t.Fatalf("%s: UUID corrompido gerado em paralelo: %s", name, u)
					}
				}
			}
			continue
		}

		seen := make(map[uuid.UUID]struct{}, goroutines*perGoroutine)
		for _, batch := range results {
			for _, u := range batch {
				if u.Version() == 0 {
					t.Fatalf("%s: UUID zerado gerado em paralelo", name)
				}
				if _, dup := seen[u]; dup {
					t.Fatalf("%s: UUID duplicado gerado em paralelo: %s", name, u)
				}
				seen[u] = struct{}{}
			}
		}
	}
}
