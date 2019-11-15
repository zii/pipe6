// handshake protocal for pipe6
package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
)

var std = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)

func debug(a ...interface{}) {
	std.Output(2, fmt.Sprintln(a...))
}

//+---------+---------+------------+
//| NETWORK |  NADDR  |  ADDRESS   |
//+---------+---------+------------+
//|    1    |    2    | 1 to 65535 |
//+---------+---------+------------+
// NETWORK:
//     TCP: 1
//     UDP: 2
type Hello struct {
	Network byte
	Addr    string
}

func (h *Hello) NetworkString() string {
	switch h.Network {
	case 1:
		return "tcp"
	case 2:
		return "udp"
	default:
		return ""
	}
}

func (h *Hello) Encode() []byte {
	w := bytes.NewBuffer(nil)
	w.Write([]byte{h.Network})
	binary.Write(w, binary.BigEndian, uint16(len(h.Addr)))
	w.Write([]byte(h.Addr))
	return w.Bytes()
}

func (h *Hello) Decode(r io.Reader) bool {
	var b3 [3]byte
	_, err := io.ReadFull(r, b3[:])
	if err != nil {
		return false
	}
	network := b3[0]
	if network != 1 {
		debug("unsupport network:", network, b3)
		return false
	}
	naddr := binary.BigEndian.Uint16(b3[1:3])
	if naddr == 0 {
		debug("naddr invalid: 0")
		return false
	}
	var buf = make([]byte, naddr)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return false
	}
	h.Network = network
	h.Addr = string(buf)
	return true
}
