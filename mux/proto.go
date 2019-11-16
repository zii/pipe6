package mux

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
)

// hello package
// 1 byte: network
// 2 byte: address length
// n byte: address

type Hello struct {
	Network byte
	Addr    string
}

func EncodeHello(network byte, addr string) []byte {
	w := bytes.NewBuffer(nil)
	w.WriteByte(network)
	binary.Write(w, binary.BigEndian, uint16(len(addr)))
	w.Write([]byte(addr))
	return w.Bytes()
}

func DecodeHello(r io.Reader) *Hello {
	var b3 [3]byte
	_, err := io.ReadFull(r, b3[:])
	if err != nil {
		return nil
	}
	network := b3[0]
	if network != 1 {
		log.Fatal("DecodeHello: network != 1")
	}
	n := binary.BigEndian.Uint16(b3[1:3])
	var addrb = make([]byte, n)
	_, err = io.ReadFull(r, addrb)
	if err != nil {
		return nil
	}
	out := &Hello{
		Network: b3[0],
		Addr:    string(addrb),
	}
	return out
}
