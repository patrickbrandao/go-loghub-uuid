# Migração a partir de github.com/google/uuid

Guia para trocar o pacote `github.com/google/uuid` por esta biblioteca
sem reescrever o código que já usa UUIDs.

---

## 1. Troca do import

O apelido do import resolve a maior parte do trabalho, porque os nomes
das funções mais usadas foram reproduzidos:

```go
// antes
import "github.com/google/uuid"

// depois
import uuid "github.com/patrickbrandao/go-loghub-uuid"
```

Com isso continuam compilando, com a mesma assinatura de antes:

| Chamada                              | Versão gerada        |
|--------------------------------------|----------------------|
| `uuid.New()`                         | 4                    |
| `uuid.NewString()`                   | 4, em texto          |
| `uuid.NewRandom()`                   | 4                    |
| `uuid.NewRandomFromReader(r)`        | 4                    |
| `uuid.NewUUID()`                     | 1                    |
| `uuid.NewV6()`                       | 6                    |
| `uuid.NewV7()`                       | 7, nível 1           |
| `uuid.NewV7FromReader(r)`            | 7, nível 1           |
| `uuid.NewMD5(space, data)`           | 3                    |
| `uuid.NewSHA1(space, data)`          | 5                    |
| `uuid.NewHash(h, space, data, v)`    | a que for pedida     |
| `uuid.NewDCESecurity(domain, id)`    | 2                    |
| `uuid.NewDCEPerson()`                | 2, domínio Person    |
| `uuid.NewDCEGroup()`                 | 2, domínio Group     |

E também `uuid.Parse`, `uuid.ParseBytes`, `uuid.MustParse`, `uuid.Must`,
`uuid.Validate`, `uuid.FromBytes`, `uuid.Nil`, `uuid.Max`, `uuid.UUIDs`,
`uuid.NameSpaceDNS` e as demais constantes de espaço de nomes,
`uuid.NodeID`, `uuid.SetNodeID`, `uuid.ClockSequence`,
`uuid.SetClockSequence`, `uuid.IsInvalidLengthError`, além dos métodos
`String`, `URN`, `Version`, `Variant`, `MarshalText`, `UnmarshalText`,
`MarshalBinary`, `UnmarshalBinary`, `Scan`, `Value` e do tipo
`NullUUID`.

Sobre o erro devolvido por esses apelidos: `NewUUID`, `NewV6`,
`NewDCESecurity`, `NewDCEPerson` e `NewDCEGroup` **nunca falham**; o
erro existe apenas para manter a forma da assinatura. `NewRandom` e
`NewV7` devolvem `ErrEntropySource` se a leitura de `crypto/rand` falhar
(impossível a partir do Go 1.24, em que o próprio runtime encerra o
processo). `NewRandomFromReader` e `NewV7FromReader` devolvem
`ErrEntropySource` quando o leitor é nulo ou se esgota antes de entregar
os bytes pedidos.

`Scan` segue a mesma convenção do pacote de origem para valores ausentes:
`NULL`, string vazia e fatia de bytes vazia gravam o UUID nulo sem erro
(em `NullUUID`, produzem `Valid` falso). `IsInvalidLengthError` reconhece
o erro de comprimento mesmo depois de embrulhado com `%w`, como lá.

> **Entropia dos apelidos de compatibilidade:** Os apelidos `New()`,
> `NewString()`, `NewRandom()` e `NewV7()` usam internamente um gerador
> dedicado com `crypto/rand` (`compatGenerator`), preservando a garantia de
> segurança criptográfica do pacote `google/uuid`. Se o seu objetivo for
> máxima velocidade estatística em vez de segurança criptográfica, use
> diretamente as funções nativas `Generate(Level1)` ou `GenerateV4()`.

---

## 2. Diferenças que exigem edição

**`Version()` e `Variant()` devolvem `byte`, não tipos próprios.**
Comparações com número continuam funcionando. O que muda é a impressão:
onde antes bastava imprimir o valor, use `VersionString(u.Version())` e
`VariantString(u.Variant())`.

**`Time()` virou `Timestamp()` e devolve `time.Time`.** O tipo `Time`
desta biblioteca é outra coisa: uma struct com segundos, milissegundos,
microssegundos e nanossegundos, devolvida por `Import` e `ImportBinary`.
O equivalente ao tipo de tempo do pacote antigo chama-se
`GregorianTime`, obtido por `(UUID).GregorianTime()`.

```go
// antes
sec, nsec := u.Time().UnixTime()

// depois
sec, nsec := 0, 0
if g, ok := u.GregorianTime(); ok {
	sec, nsec = g.UnixTime()
}

// ou, direto:
instante, ok := u.Timestamp()
```

**`(UUID).ClockSequence()`, `Domain()` e `ID()` devolvem um segundo
retorno booleano**, falso quando a versão do UUID não carrega aquele
campo. O pacote antigo devolvia lixo em silêncio nesses casos. Para a
versão 2 (DCE Security), `ClockSequence()` devolve apenas os 6 bits altos
(0 a 63), pois o byte inferior da sequência carrega o domínio.

**Deriva de relógio em versões 1 e 6.** Em alta frequência no mesmo
tique de relógio, o pacote `google/uuid` incrementa a sequência de
relógio e mantém o carimbo inalterado, enquanto esta biblioteca adianta o
relógio em um tique de 100 ns por geração. Ambas as abordagens são
válidas segundo a RFC 9562, mas diferem no carimbo gravado sob rajada.

**`SetClockSequence` mantém um piso por sequência.** No pacote
`google/uuid`, qualquer troca de sequência descarta o último instante
emitido, o que permite repetir um UUIDv1 ao voltar para uma sequência já
usada enquanto o relógio real ainda está atrás do adiantamento acumulado.
Aqui cada sequência lembra o último instante que emitiu: voltar a ela
continua a partir dali, e só a entrada em uma sequência inédita descarta o
adiantamento. `SetClockSequence(-1)` sorteia sempre uma sequência inédita,
e por isso é a forma de ressincronizar com o relógio do sistema.

**Sem `SetRand`.** Trocar a entropia do gerador padrão em tempo de
execução exigiria uma leitura atômica no caminho quente da geração. Em
vez disso, monte um gerador próprio:

```go
// antes
uuid.SetRand(crand.Reader)

// depois
var Gen = uuid.NewCryptoGenerator()
```

**Sem `EnableRandPool` e `DisableRandPool`.** O gerador padrão já lê do
gerador do runtime do Go, que tem uma instância por thread e nenhuma
trava global. Não há o que ligar.

**Sem `SetNodeInterface` e `NodeInterface`.** Ler interfaces de rede
arrastaria o pacote `net` para dentro de quem só gera UUIDv7. O nó
padrão é sorteado e marcado como aleatório, conforme a RFC 9562 seção
6.10 recomenda. Para usar um endereço MAC real:

```go
interfaces, _ := net.Interfaces()
for _, iface := range interfaces {
	if uuid.SetNodeID(iface.HardwareAddr) {
		break
	}
}
```

---

## 3. Conversão entre os dois tipos

Os dois tipos `UUID` são `[16]byte`, então a conversão é direta e sem
custo. Isso permite migrar módulo a módulo, mantendo os dois pacotes
convivendo:

```go
import (
	google "github.com/google/uuid"
	uuid "github.com/patrickbrandao/go-loghub-uuid"
)

meu := uuid.Generate(uuid.Level3)
deles := google.UUID(meu)
devolta := uuid.UUID(deles)
```

---

## 4. Atenção com dados já gravados

Esta biblioteca passou a implementar `MarshalText` e `MarshalBinary`.
Antes disso, `encoding/json` serializava um UUID como **lista de 16
números**, porque o tipo é um vetor de bytes. Agora ele é serializado
como a **string canônica entre aspas** — que é justamente o
comportamento do pacote `github.com/google/uuid`.

Para quem vem do pacote do Google, portanto, nada muda. Para quem já
usava esta biblioteca e gravou JSON ou `gob` com o formato antigo, o
dado precisa ser convertido na migração.

---

## 5. O que esta biblioteca tem a mais

- **Três níveis de precisão no UUIDv7**: `Level2` grava microssegundos e
  `Level3` grava também nanossegundos, dentro dos campos aleatórios, sem
  quebrar versão nem variante. Veja [SPEC.md](SPEC.md).
- **`TimestampWithLevel`**: recupera a precisão sub-milissegundo de
  volta como `time.Time`.
- **`Import` e `ImportBinary`**: extraem segundos, milissegundos,
  microssegundos e nanossegundos em uma struct.
- **Geração sem trava e sem alocação** no caminho do UUIDv7.
- **`AppendTo`**: escreve a forma canônica no buffer do chamador, sem
  alocar quando há capacidade. Não existe no pacote do Google, onde o
  único caminho para texto é `String`, que aloca a cada chamada.
  `AppendText` expõe a mesma escrita com a assinatura de
  `encoding.TextAppender` (Go 1.24).
- **`Bytes()`**: cópia dos 16 bytes. Não existe no pacote do Google — lá
  o caminho é `u[:]`, que devolve uma fatia sobre o próprio valor, ou
  `MarshalBinary`, cujo erro é sempre nulo.
- **`IsValid()`**: confere variante e versão em uma chamada. Não existe
  no pacote do Google, onde a verificação equivalente é comparar
  `Version()` e `Variant()` à mão.
- **`Compare`, `IsZero`, `IsMax`** e as descrições em texto
  `VersionString` e `VariantString`.
