package loghubuuid

// GenerateV8 produz um UUID de versão 8: formato livre, reservado pela RFC
// 9562 para esquemas próprios de fornecedor.
//
// Os 16 bytes informados são copiados como estão, e só os 6 bits de versão
// e variante são sobrescritos. Restam 122 bits sob controle do chamador.
// Nada aqui é interpretado como tempo, e a ordenação depende inteiramente
// do conteúdo fornecido.
func GenerateV8(data [16]byte) UUID {
	u := UUID(data)
	u[6] = (u[6] & 0x0F) | 0x80 // nibble alto = versão 8
	u[8] = (u[8] & 0x3F) | 0x80 // bits 7..6 = variante "10"
	return u
}

// GenerateV8 produz um UUID de versão 8 com os 122 bits livres preenchidos
// pela fonte de entropia deste Generator.
//
// No gerador padrão a fonte é o ChaCha8 do runtime do Go; para segredos,
// prefira um gerador com entropia criptográfica. Veja o aviso em
// GenerateV4.
//
// O resultado é indistinguível de um UUIDv4 quanto ao conteúdo; muda
// apenas o número de versão. Use quando precisar marcar identificadores
// como pertencentes a um esquema próprio.
func (g *Generator) GenerateV8() UUID {
	return GenerateV8(g.GenerateV4())
}

// GenerateV8Random produz um UUID de versão 8 aleatório usando o gerador
// padrão do pacote.
func GenerateV8Random() UUID { return defaultGenerator.GenerateV8() }
