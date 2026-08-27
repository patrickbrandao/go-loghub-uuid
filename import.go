package loghubuuid

// Time reúne as propriedades temporais extraídas de um UUIDv7.
//
// Os campos Microseconds e Nanoseconds só têm significado real se o
// UUID foi gerado com Level2/Level3 (micro) e Level3 (nano). Para UUIDs
// de Level1 esses campos contêm bits aleatórios — ainda assim, conforme
// a especificação, a importação os trata como tempo preciso e devolve o
// valor lido nos respectivos campos sem julgar a origem.
type Time struct {
	// Seconds é o timestamp Unix em segundos desde 1970-01-01 UTC.
	Seconds int64

	// Milliseconds é a fração do segundo: sempre 0..999.
	Milliseconds int

	// Microseconds é lido de rand_a. Vale 0..999 (fração do
	// milissegundo) quando o UUID é de Level2 ou Level3; em UUIDs de
	// Level1 são 12 bits aleatórios, portanto 0..4095.
	Microseconds int

	// Nanoseconds é lido de rand_b[61:52]. Vale 0..999 (fração do
	// microssegundo) quando o UUID é de Level3; em UUIDs de Level1 ou
	// Level2 são 10 bits aleatórios, portanto 0..1023.
	Nanoseconds int
}

// ImportBinary extrai as propriedades de tempo de um UUID binário.
//
// A leitura é "cega" quanto ao nível: ela sempre interpreta rand_a como
// microssegundos e os 10 bits altos de rand_b como nanossegundos,
// considerando dados aleatórios como se fossem o tempo preciso.
func ImportBinary(u UUID) Time {
	// 48 bits de milissegundos (bytes 0..5).
	ms := int64(u[0])<<40 |
		int64(u[1])<<32 |
		int64(u[2])<<24 |
		int64(u[3])<<16 |
		int64(u[4])<<8 |
		int64(u[5])

	// rand_a (12 bits): 4 bits baixos do byte 6 + byte 7 → microssegundos.
	micro := int(uint16(u[6]&0x0F)<<8 | uint16(u[7]))

	// 10 bits altos de rand_b: 6 bits baixos do byte 8 + 4 bits altos do byte 9 → nanossegundos.
	nano := int(uint16(u[8]&0x3F)<<4 | uint16(u[9])>>4)

	return Time{
		Seconds:      ms / 1000,
		Milliseconds: int(ms % 1000),
		Microseconds: micro,
		Nanoseconds:  nano,
	}
}

// Import recebe um UUIDv7 em string e devolve suas propriedades de
// tempo (segundos, milissegundos, microssegundos e nanossegundos).
// Retorna ErrInvalidFormat se a string não for um UUID canônico válido.
func Import(s string) (Time, error) {
	u, err := FromString(s)
	if err != nil {
		return Time{}, err
	}
	return ImportBinary(u), nil
}
