package mux

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"math/rand"
)

// frame struct
// 2 byte: session id
// 1 byte: stage, 1 establish 2 transfer 3 close
// 2 byte: random_id, for debug
// 2 byte: frame length
// n byte: payload

const (
	StageEstablish = 1
	StageTransfer  = 2
	StageClose     = 3
)

type Frame struct {
	SessionId uint16
	Stage     byte
	Payload   []byte
}

func EncodeFrame(sessionId uint16, stage byte, payload []byte) []byte {
	length := len(payload)
	if length > 65535 {
		log.Fatal("encode: payload exceed limit:", length)
	}
	buffer := bytes.NewBuffer(nil)
	binary.Write(buffer, binary.BigEndian, sessionId)
	buffer.WriteByte(stage)
	randomId := uint16(rand.Intn(65536))
	binary.Write(buffer, binary.BigEndian, randomId)
	binary.Write(buffer, binary.BigEndian, uint16(length))
	if length > 0 {
		n, err := buffer.Write(payload)
		if err != nil {
			log.Fatal("impossible2:", err)
		}
		if n != length {
			log.Fatal("impossible3")
		}
	}
	b := buffer.Bytes()
	//debug("encode:", sessionId, randomId, "size=", length)
	return b
}

func ReadFrame(r io.Reader) *Frame {
	var err error
	var sessionId uint16
	err = binary.Read(r, binary.BigEndian, &sessionId)
	if err != nil {
		return nil
	}
	//log.Println("frame e2 session_id:", sessionId)
	var stage byte
	err = binary.Read(r, binary.BigEndian, &stage)
	if err != nil {
		return nil
	}
	if stage < 1 || stage > 3 {
	}
	var randomId uint16
	err = binary.Read(r, binary.BigEndian, &randomId)
	if err != nil {
		return nil
	}
	var n uint16
	err = binary.Read(r, binary.BigEndian, &n)
	if err != nil {
		return nil
	}
	var payload []byte
	if n > 0 {
		payload = make([]byte, n)
		_, err = io.ReadFull(r, payload)
		if err != nil {
			return nil
		}
	}
	out := &Frame{
		SessionId: sessionId,
		Stage:     stage,
		Payload:   payload,
	}
	return out
}
