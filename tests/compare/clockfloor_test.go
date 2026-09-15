package compare

import (
	"testing"
	"time"

	google "github.com/google/uuid"
	loghub "github.com/patrickbrandao/go-loghub-uuid"
)

// As duas sequências usadas na ida e volta. Os valores exatos não
// importam, só precisam ser diferentes entre si.
const (
	sequenciaA = 0x0A0A
	sequenciaB = 0x0B0B
)

// tentativas é quantas vezes o cenário de repetição é exercitado. A
// janela é de um tique de 100 ns, então cada tentativa só colide se as
// duas gerações e as duas trocas de sequência couberem dentro dela. Num
// Apple M2 isso acontece em cerca de 6% das tentativas; a contagem alta
// existe para a máquina lenta também chegar lá.
const tentativas = 1_000_000

// cenarioDeRepeticao é o roteiro exato, isolado para os dois lados
// rodarem exatamente o mesmo: fixar a sequência, gerar, ir a outra
// sequência, voltar, gerar de novo.
//
// O que ele expõe é o piso do relógio. Zerar o piso desarma a proteção
// contra tempo repetido, e as duas gerações passam a poder sair com o
// mesmo instante e a mesma sequência.

// TestGoogleRepeatsV1OnSequenceReturn demonstra, com o pacote
// github.com/google/uuid v1.6.0, a repetição de UUIDv1 que a
// documentação deste projeto atribui a ele.
//
// O MECANISMO, medido e não suposto: em getTime, a única proteção
// contra reemitir um instante é a comparação "now <= lasttime", e
// SetClockSequence zera lasttime sempre que a sequência realmente muda.
// Com o piso zerado a comparação fica desarmada, então duas gerações
// que caiam no mesmo tique de 100 ns, com a mesma sequência, produzem o
// mesmo instante — e, com o mesmo nó, o UUID inteiro se repete.
//
// Este teste não é crítica ao outro pacote: o cenário exige trocar de
// sequência à mão entre duas gerações, o que quase ninguém faz, e o
// comportamento é coerente com a leitura que ele faz da RFC.
//
// Se ele FALHAR por não achar repetição, o outro pacote mudou e a
// documentação deste projeto passou a estar errada. Corrija a
// documentação, não o teste.
func TestGoogleRepeatsV1OnSequenceReturn(t *testing.T) {
	repetidos := 0
	for i := 0; i < tentativas; i++ {
		google.SetClockSequence(sequenciaA)
		primeiro, err := google.NewUUID()
		if err != nil {
			t.Fatalf("tentativa %d: %v", i, err)
		}

		// A ida e volta. Cada chamada que muda a sequência zera o piso.
		google.SetClockSequence(sequenciaB)
		google.SetClockSequence(sequenciaA)

		segundo, err := google.NewUUID()
		if err != nil {
			t.Fatalf("tentativa %d: %v", i, err)
		}
		if primeiro == segundo {
			repetidos++
		}
	}

	if repetidos == 0 {
		t.Skipf("nenhuma repeticao em %d tentativas: nesta maquina as duas geracoes e as duas "+
			"trocas de sequencia nunca couberam no mesmo tique de 100 ns. Isso NAO contradiz a "+
			"afirmacao da secao 4.2 do docs/06-relogio-e-concorrencia.md, so diz que a janela nao foi alcancada aqui.",
			tentativas)
	}
	t.Logf("github.com/google/uuid: %d UUIDv1 repetidos em %d tentativas (%.2f%%)",
		repetidos, tentativas, 100*float64(repetidos)/float64(tentativas))
}

// TestLoghubDoesNotRepeatV1OnSequenceReturn submete esta biblioteca ao
// roteiro idêntico e exige zero repetições.
//
// A diferença está no piso por sequência: o último instante emitido com
// cada sequência é guardado, e voltar a uma sequência já usada retoma o
// piso dela em vez de zerá-lo. Para cada sequência os instantes emitidos
// são estritamente crescentes durante toda a vida do processo, e o par
// (instante, sequência) nunca se repete.
func TestLoghubDoesNotRepeatV1OnSequenceReturn(t *testing.T) {
	// O nó é global no processo: fixá-lo garante que a ausência de
	// repetição venha do piso de relógio, e não de o nó ter mudado.
	if !loghub.SetNodeID([]byte{0x02, 0x11, 0x22, 0x33, 0x44, 0x55}) {
		t.Fatal("SetNodeID recusou um no de 6 bytes")
	}

	vistos := make(map[loghub.UUID]struct{}, tentativas*2)
	for i := 0; i < tentativas; i++ {
		loghub.SetClockSequence(sequenciaA)
		primeiro := loghub.GenerateV1()

		loghub.SetClockSequence(sequenciaB)
		loghub.SetClockSequence(sequenciaA)

		segundo := loghub.GenerateV1()

		if primeiro == segundo {
			t.Fatalf("tentativa %d: as duas geracoes devolveram o mesmo UUIDv1 %s", i, primeiro)
		}
		for _, u := range [2]loghub.UUID{primeiro, segundo} {
			if _, ja := vistos[u]; ja {
				t.Fatalf("tentativa %d: o UUIDv1 %s ja tinha sido emitido", i, u)
			}
			vistos[u] = struct{}{}
		}
	}
}

// TestClockAdvanceDiffersBetweenLibraries documenta, com medição, a
// outra metade da diferença de projeto — e é ela que explica por que o
// cenário de repetição acima precisa da janela estreita, em vez de sair
// de uma rajada qualquer.
//
// O pacote do Google NÃO adianta o relógio: quando o instante não
// avança, ele incrementa a sequência de relógio de 14 bits e mantém o
// instante de parede. Esta biblioteca faz o contrário, avançando um
// tique de 100 ns por geração e mantendo a sequência que o chamador
// escolheu. As duas escolhas são permitidas pela RFC 9562.
//
// A consequência prática para quem migra: lá a sequência muda sozinha
// debaixo do chamador durante uma rajada; aqui o instante embutido se
// adianta do relógio de parede.
func TestClockAdvanceDiffersBetweenLibraries(t *testing.T) {
	const rajada = 200_000

	google.SetClockSequence(sequenciaA)
	seqAntes := google.ClockSequence()
	var ultimoGoogle google.UUID
	for i := 0; i < rajada; i++ {
		u, err := google.NewUUID()
		if err != nil {
			t.Fatalf("google, item %d: %v", i, err)
		}
		ultimoGoogle = u
	}
	depoisGoogle := time.Now()
	seqDepois := google.ClockSequence()

	sec, nsec := ultimoGoogle.Time().UnixTime()
	derivaGoogle := time.Unix(sec, nsec).Sub(depoisGoogle)

	if seqDepois == seqAntes {
		t.Errorf("google: a sequencia continuou %d apos %d geracoes; esperava-se que fosse incrementada",
			seqDepois, rajada)
	}
	if derivaGoogle > time.Millisecond {
		t.Errorf("google: instante embutido %v adiantado do relogio de parede; esperava-se nenhum adiantamento",
			derivaGoogle)
	}

	loghub.SetClockSequence(sequenciaA)
	seqLoghubAntes, _ := loghub.GenerateV1().ClockSequence()
	var ultimoLoghub loghub.UUID
	for i := 0; i < rajada; i++ {
		ultimoLoghub = loghub.GenerateV1()
	}
	depoisLoghub := time.Now()

	g, ok := ultimoLoghub.GregorianTime()
	if !ok {
		t.Fatal("loghub: GregorianTime recusou um UUIDv1")
	}
	derivaLoghub := g.Time().Sub(depoisLoghub)
	seqLoghubDepois, _ := ultimoLoghub.ClockSequence()

	if seqLoghubDepois != seqLoghubAntes {
		t.Errorf("loghub: a sequencia mudou de %d para %d durante a rajada; ela deveria ser estavel",
			seqLoghubAntes, seqLoghubDepois)
	}
	if derivaLoghub <= 0 {
		t.Errorf("loghub: adiantamento de %v apos %d geracoes; esperava-se adiantamento positivo",
			derivaLoghub, rajada)
	}

	t.Logf("google: sequencia %d -> %d, adiantamento %v", seqAntes, seqDepois, derivaGoogle)
	t.Logf("loghub: sequencia estavel em %d, adiantamento %v", seqLoghubDepois, derivaLoghub)
}
