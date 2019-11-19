package mux

import (
	"encoding/binary"
)

// frame header:
// 1b: version
// 1b: cmd
// 	 1 SYN: create new session
//	 2 FIN: close session
//   3 PSH: push data to stream
//   4 NOP: ping
// 2b: data length
// 4b: stream_id

const VER = 1

const (
	cmdSYN = 1
	cmdFIN = 2
	cmdPSH = 3
	cmdNOP = 4
)

const (
	sizeVer    = 1
	sizeCmd    = 1
	sizeLength = 2
	sizeSid    = 4
	sizeHeader = sizeVer + sizeCmd + sizeLength + sizeSid
)

type Header struct {
	ver byte
	cmd byte
	n   uint16
	sid uint32 // stream id
}

func newHdr(cmd byte, n uint16, sid uint32) *Header {
	return &Header{
		ver: VER,
		cmd: cmd,
		n:   n,
		sid: sid,
	}
}

func decHdr(b [sizeHeader]byte) (*Header, error) {
	ver := b[0]
	if ver != VER {
		return nil, ErrInvalidVersion
	}
	cmd := b[1]
	if cmd < 1 || cmd > 4 {
		return nil, ErrInvalidCmd
	}
	n := binary.LittleEndian.Uint16(b[2:4])
	sid := binary.LittleEndian.Uint32(b[4:8])
	h := &Header{
		ver: ver,
		cmd: cmd,
		n:   n,
		sid: sid,
	}
	return h, nil
}

func encHdr(h *Header) []byte {
	var b [sizeHeader]byte
	b[0] = h.ver
	b[1] = h.cmd
	binary.LittleEndian.PutUint16(b[2:4], h.n)
	binary.LittleEndian.PutUint32(b[4:8], h.sid)
	return b[:]
}
