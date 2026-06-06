
# Biblioteca UUIDv7 em GoLang

Você é um engenheiro de software especializado em linguagem GO (GoLang).

Crie uma biblioteca para geração de UUIDv7 com os seguintes recursos:

## Níveis

Nível 1 utiliza apenas 48 bits para timestamp+milissegundos;
Nível 2 invade rand_a para salvar até microssegundos;
Nível 3 invade o rand_a para ocupar microssegundos e nanossegundos.

## Recursos

A biblioteca deve:
- Gerar: gerar UUIDv7 do nível desejado, função para produzir string e outra para binário 128 bits;
- Importar: Recebe uma UUIDv7 em string e importar as propriedades de tempo separadas por timestamp (segundos), milisegundos (0-999), microsegundos (0-999) e nanosegundos (0-999), considerar dados aleatórios como sendo o tempo preciso;
- Converter de string para binário de 128 bits, converter de binário de 128 bits para string;

## Requisitos

Requisitos:
- Leve e rápida, capaz de gerar milhares de identificadores por milisegundo quando executada em processadores de altíssima velocidade;
- Objeto simples, criado no boot do software e capaz de ser invocado por centenas de threads.
- Foco em código legível e bem explicado nos comentários em portugues.

## Objetivo

Gere:
- Especificação de desenvolvimento clara para que modelos de IA possam gerar o codigo do zero sem incluir exemplo de código na especificação, ela deve ser capaz de gerar a biblioteca em qualquer linguagem;
- Especificação de uso rápido ensinando como incluir a biblioteca (https://github.com/patrickbrandao/go-loghub-uuid) em projetos e gerar UUIDv7 em string rapidamente;
- Especificação de uso completa ensinando como incluir no projeto e fazer uso de todas as funções;
- Especificação de como testar a geração em massa de até 1 milhão de UUIDv7 de cada nível com benchmark de tempo consumido para gerar essa quantidade em cada nível;
- Deixe na pasta principal somente arquivos úteis para usar a biblioteca em produção, todos os demais arquivos devem estar em pastas proprias (documentação, testes, especificações).
- Crie o README.md para o github com a descrição rápida e instrução para uso no modo rápido;
- Crie o arquivo STARTHERE.md para o github com o mapa completo da biblioteca;
- Escreva a biblioteca e disponibilize o pacote zip para download.

# Prompt

Crie a Biblioteca UUIDv7 em GoLang.

