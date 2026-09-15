# Índice da documentação

Mapa de leitura dos documentos em `docs/`, na ordem recomendada. Esta
pasta reúne a especificação de desenvolvimento agnóstica de linguagem
(originalmente `SPEC.md`, um único arquivo de mais de 1500 linhas) e os
guias de uso em Go (originalmente `DEPLOY-FAST.md`, `DEPLOY-FULL.md`,
`MIGRATION.md`, `TEST-AND-BENCHMARK.md` e `RELEASE.md`), reorganizados
em arquivos menores e focados em um único tema cada. Nenhum conteúdo
técnico, decisão ou justificativa foi perdido nessa reorganização — os
números de seção originais (`§3.5`, `§11.2` etc.) foram preservados
dentro do texto para que referências antigas continuem localizáveis por
busca.

| Arquivo | Conteúdo |
|---|---|
| [01-visao-geral.md](01-visao-geral.md) | Objetivo e escopo da biblioteca; conceitos universais de UUID (campos de versão/variante, representação canônica em texto). Ponto de partida. |
| [02-guia-rapido.md](02-guia-rapido.md) | Uso rápido em Go: instalar, gerar um UUIDv7 em string, escolher nível, criar um gerador compartilhado, consultar por intervalo e gerar para um instante conhecido — em poucos minutos. |
| [03-guia-completo.md](03-guia-completo.md) | Referência completa de uso em Go: todos os tipos, construtores de `Generator`, geração, conversão, importação de tempo, consulta por intervalo, inspeção, as outras versões de UUID, parsing, JSON/texto/binário e banco de dados, com exemplos. |
| [04-uuidv7-formato-e-niveis.md](04-uuidv7-formato-e-niveis.md) | Especificação normativa do núcleo da biblioteca: layout de bits do UUIDv7 multinível, aritmética temporal segura, sorteio de entropia por nível, política de ordenação sem contador monotônico, fronteiras de tempo (`MinAt`/`MaxAt`/`RangeAt`) e geração a partir de um instante explícito (`GenerateAt`). |
| [05-outras-versoes-uuid.md](05-outras-versoes-uuid.md) | Especificação normativa das demais versões da RFC 9562: v1 e v6 (tempo gregoriano e sua conversão inversa), v2 (DCE Security), v3 e v5 (baseadas em espaço de nomes), v4 e v8 (aleatória e de formato livre). |
| [06-relogio-e-concorrencia.md](06-relogio-e-concorrencia.md) | Como a biblioteca mantém estado compartilhado em segurança sob concorrência: o relógio monotônico de v1/v2/v6 (adiantamento, piso por sequência, identificador de nó) e a arquitetura de entropia do gerador padrão (fonte por thread, política de falha rápida, fontes criptográficas dedicadas). |
| [07-parsing-e-conversao.md](07-parsing-e-conversao.md) | Algoritmos de conversão e parsing seguro: formatação binário→texto, o analisador estrito (imune a pânico), o analisador permissivo de quatro formatos e a taxonomia normativa de erros. |
| [08-inspecao-serializacao-banco.md](08-inspecao-serializacao-banco.md) | Contrato de API de inspeção (`Timestamp`, `TimestampWithLevel`, `Import`/`ImportBinary`, `GregorianTime`, `ClockSequence` etc.) e especificação de serialização/banco de dados (JSON, `NullUUID`, `BinaryUUID`, `NullBinaryUUID`, valores especiais `Nil`/`Max`). |
| [09-vetores-dourados-e-apendice-rfc.md](09-vetores-dourados-e-apendice-rfc.md) | Os 21 casos de teste obrigatórios para validar uma implementação, incluindo os vetores dourados da extensão multinível (exclusivos deste projeto), os vetores das versões baseadas em tempo gregoriano e os vetores do Apêndice A da RFC 9562 para v3/v5. |
| [10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md) | Catálogo das 12 armadilhas históricas evitadas (guia anti-regressão) e o registro de decisões de projeto firmadas (§11.1 a §11.4) — leia antes de propor qualquer mudança de projeto ou de auditar a biblioteca. |
| [11-testes-e-benchmark.md](11-testes-e-benchmark.md) | Como rodar a suíte de testes, o detector de corrida, o fuzzing, o linter, a cobertura, a comparação com `github.com/google/uuid`, os benchmarks e a geração em massa; resultados de referência medidos. |
| [12-migracao-google-uuid.md](12-migracao-google-uuid.md) | Guia de migração a partir de `github.com/google/uuid`: troca de import, diferenças que exigem edição, conversão entre os dois tipos, atenção com dados já gravados e o que esta biblioteca tem a mais. |
| [13-processo-de-release.md](13-processo-de-release.md) | Procedimento de release: pré-requisitos, etiquetar e publicar, verificar a publicação, e por que tags são imutáveis. |

## Como os documentos se relacionam

- **Para usar a biblioteca em Go**, siga 02 → 03. Os dois cobrem a
  mesma API; 02 é o caminho mais curto e 03 é a referência completa.
- **Para reimplementar a biblioteca em outra linguagem**, ou entender o
  "porquê" de cada regra, leia 01, 04, 05, 06, 07 e 08 nessa ordem — é a
  especificação agnóstica de linguagem completa, equivalente ao antigo
  `SPEC.md` seções 1 a 8. Os arquivos 09 e 10 fecham a especificação com
  os casos de teste obrigatórios, os vetores de conformidade e o
  registro de decisões (antigo `SPEC.md` seções 9, 10 e 11).
- **Para propor uma mudança de projeto, ou auditar a biblioteca**, leia
  [10-armadilhas-e-decisoes-de-projeto.md](10-armadilhas-e-decisoes-de-projeto.md)
  primeiro: a maioria das perguntas já tem resposta registrada lá.
- **Para contribuir, testar ou publicar uma versão**, veja 11 e 13.
- **Para migrar de `github.com/google/uuid`**, veja 12, mais
  `tests/compare/` no repositório para a diferença de comportamento
  provada em código.

Fora desta pasta: o mapa geral do projeto está em
[../STARTHERE.md](../STARTHERE.md), o histórico de mudanças em
[../CHANGELOG.md](../CHANGELOG.md), e as instruções de manutenção para
ferramental de IA em [../CLAUDE.md](../CLAUDE.md).
