package common

import (
	"encoding/binary"
	"io"
	"net"
)

const (
	OpcodeBet   byte = 0x01
	OpcodeAck   byte = 0x02
	OpcodeError byte = 0x03
	OpcodeBatch byte = 0x04
	OpcodeEndData    byte = 0x05
    OpcodeGetWinners byte = 0x06
)

type Frame struct {
	Opcode byte
	Body   []byte
}

// WriteFrame sends the header (5 bytes) + body over the network
func WriteFrame(conn net.Conn, opcode byte, body []byte) error {
	header := make([]byte, 5)
	header[0] = opcode
	binary.BigEndian.PutUint32(header[1:], uint32(len(body)))
	    data := append(header, body...)
    totalSent := 0

    for totalSent < len(data) {
        n, err := conn.Write(data[totalSent:])
        if err != nil {
            return err
        }
        if n == 0 {
            break
        }
        totalSent += n
    }

    return nil
}

// ReadFrame reads exactly 5 bytes for the header, then 'length' bytes for the body
func ReadFrame(conn net.Conn) (*Frame, error) {
	header := make([]byte, 5)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}

	opcode := header[0]
	length := binary.BigEndian.Uint32(header[1:])

	body := make([]byte, length)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, err
	}

	return &Frame{Opcode: opcode, Body: body}, nil
}