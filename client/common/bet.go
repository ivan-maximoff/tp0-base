package common

import (
	"encoding/binary"
)

type Bet struct {
	Agency    string
	Name      string
	LastName  string
	ID        string
	BirthDate string
	Number    string
}

// Serialize converts the Bet fields into a byte slice.
// Format: [Len1 (2b)][Field1][Len2 (2b)][Field2]...
func (b *Bet) Serialize() []byte {
	buf := make([]byte, 0)
	fields := []string{b.Agency, b.Name, b.LastName, b.ID, b.BirthDate, b.Number}

	for _, field := range fields {
		fieldBytes := []byte(field)
		length := uint16(len(fieldBytes))
		
		// Add 2 bytes for length (Big Endian)
		lenBuf := make([]byte, 2)
		binary.BigEndian.PutUint16(lenBuf, length)
		
		buf = append(buf, lenBuf...)
		buf = append(buf, fieldBytes...)
	}

	return buf
}

func BetFromCSV(record []string, agencyID string) Bet {
    return Bet{
        Agency:    agencyID,
        Name:      record[0],
        LastName:  record[1],
        ID:        record[2],
        BirthDate: record[3],
        Number:    record[4],
    }
}