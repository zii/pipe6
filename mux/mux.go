package mux

import (
	"fmt"
	"io"
	"time"
)

var ErrInvalidVersion = fmt.Errorf("invalid protocal version")
var ErrInvalidCmd = fmt.Errorf("invalid cmd")
var ErrStreamDead = fmt.Errorf("stream dead")
var ErrTimeout = fmt.Errorf("timeout")
var ErrSessionDead = fmt.Errorf("session dead")

type Config struct {
	Backlog           uint // server side: accept buffer size
	MaxFrameSize      int
	WriteQueueSize    uint
	PingInterval      time.Duration
	KeepAliveInterval time.Duration
}

func DefaultConfig() *Config {
	var out = &Config{
		Backlog:           1024,
		MaxFrameSize:      32768,
		WriteQueueSize:    1024,
		PingInterval:      10 * time.Second,
		KeepAliveInterval: 30 * time.Second,
	}
	return out
}

func writeBuffers(w io.Writer, bs [][]byte) (int, error) {
	written := 0
	for _, b := range bs {
		n, err := w.Write(b)
		written += n
		if err != nil {
			return written, err
		}
	}
	return written, nil
}
