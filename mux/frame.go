package mux

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
)

// frame struct
// 2 byte: frame length
// 2 byte: session id
// 1 byte: stage, 1 establish 2 transfer 3 close
//
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
	if length > 65535-3 {
		log.Fatal("encode: payload exceed limit:", length)
	}
	frameSize := 3 + length
	buffer := bytes.NewBuffer(make([]byte, 0, frameSize+2))
	binary.Write(buffer, binary.BigEndian, uint16(frameSize))
	binary.Write(buffer, binary.BigEndian, sessionId)
	buffer.WriteByte(stage)
	if length > 0 {
		buffer.Write(payload)
	}
	return buffer.Bytes()
}

func ReadFrame(r io.Reader) *Frame {
	var size uint16
	err := binary.Read(r, binary.BigEndian, &size)
	if err != nil {
		log.Println("frame e1")
		return nil
	}
	if size < 3 {
		log.Println("frame e2:", size)
		return nil
	}
	var sessionId uint16
	err = binary.Read(r, binary.BigEndian, &sessionId)
	if err != nil {
		log.Println("frame e3")
		return nil
	}
	var stage byte
	err = binary.Read(r, binary.BigEndian, &stage)
	if err != nil {
		return nil
	}
	n := size - 3
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
