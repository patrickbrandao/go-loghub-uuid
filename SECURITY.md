# Política de segurança

## Versões suportadas

Só a última versão etiquetada recebe correções. Ao relatar um problema,
confirme antes que ele se reproduz na versão mais recente listada em
`https://github.com/patrickbrandao/go-loghub-uuid/tags`.

| Versão          | Suportada |
| --------------- | --------- |
| última tag      | sim       |
| tags anteriores | não       |

## Como relatar uma vulnerabilidade

Não abra uma issue pública para um problema de segurança. Use o relato
privado do GitHub, disponível na aba "Security" do repositório, opção
"Report a vulnerability":

`https://github.com/patrickbrandao/go-loghub-uuid/security/advisories/new`

Inclua a versão afetada, os passos para reproduzir e, se possível, uma
avaliação do impacto. A resposta inicial chega em até sete dias; a
correção, quando confirmada, é publicada como versão nova com a entrada
correspondente no `CHANGELOG.md`, e o relato é creditado no aviso, salvo
pedido em contrário.

## O que a biblioteca garante e o que não garante

Esta biblioteca gera identificadores para registros, chaves primárias e
correlação de logs. Os pontos abaixo são propriedades do projeto, já
documentadas no `README.md` e em `docs/DEPLOY-FULL.md`, e não são
considerados vulnerabilidades:

- **O gerador padrão não serve para segredos.** `NewGenerator` e as
  funções de pacote `Generate`, `GenerateString`, `GenerateV7`,
  `GenerateV7Level1` a `GenerateV7Level3`, `GenerateV4` e
  `GenerateV8Random` leem do gerador do runtime do Go (ChaCha8 por
  thread). É resistente a predição, mas a documentação do
  Go recomenda `crypto/rand` para uso sensível a segurança e a
  biblioteca não promete força criptográfica nessa fonte. Para
  identificadores que precisem ser inadivinháveis (token de sessão, link
  privado, chave de recuperação), use `NewCryptoGenerator`,
  `NewGeneratorWithReader` com `crypto/rand`, ou os apelidos de
  compatibilidade `New`, `NewString`, `NewRandom` e `NewV7`, que já leem
  de `crypto/rand`.
- **Todo UUIDv7 expõe o instante de criação**, com precisão de
  milissegundo no Nível 1 e até nanossegundo no Nível 3. Isso é a
  função do formato, não um vazamento.
- **As versões 1, 2 e 6 expõem o nó e a sequência de relógio.** O nó
  padrão é sorteado uma vez por processo com o bit multicast ligado; se o
  chamador fornecer um endereço MAC real por `SetNodeID`, ele passa a
  constar em todos os identificadores gerados. A versão 2 expõe ainda o
  identificador local de usuário ou grupo.
- **As versões 3 e 5 usam MD5 e SHA-1** porque a RFC 9562 as define
  assim. Elas derivam um identificador estável de um nome; não protegem
  o nome nem resistem a colisões escolhidas.

Relatos bem-vindos: pânico ou leitura fora dos limites a partir de
entrada externa nos analisadores (`FromString`, `Parse`, `Scan`,
`UnmarshalJSON` e afins), repetição de identificador em condições que a
documentação promete únicas, corrida de dados no uso concorrente
documentado, e qualquer desvio do layout de bits da RFC 9562.
