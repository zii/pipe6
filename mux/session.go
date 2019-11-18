package mux

import (
	"net"
	"sync"
	"time"
)

type Session struct {
	c      net.Conn
	closed bool

	nextId     int
	streams    map[int]*Stream
	streamLock sync.RWMutex

	config *Config

	chAccept chan *Stream
}

func (s *Session) Read(b []byte) (int, error) {
	return 0, nil
}

func (s *Session) Write(b []byte) (int, error) {
	return s.c.Write(b)
}

func (s *Session) Close() error {
	return nil
}

func (s *Session) LocalAddr() net.Addr {
	return s.c.LocalAddr()
}

func (s *Session) RemoteAddr() net.Addr {
	return s.c.RemoteAddr()
}

func (s *Session) SetDeadline(t time.Time) error {
	return s.c.SetDeadline(t)
}

func (s *Session) SetReadDeadline(t time.Time) error {
	return s.c.SetReadDeadline(t)
}

func (s *Session) SetWriteDeadline(t time.Time) error {
	return s.c.SetWriteDeadline(t)
}

///////////////

func NewSession(conn net.Conn, config *Config) *Session {
	s := &Session{
		c:          conn,
		closed:     false,
		nextId:     1,
		streams:    make(map[int]*Stream),
		streamLock: sync.RWMutex{},
		config:     config,
	}
	return s
}

func (s *Session) Open() net.Conn {
	return s.OpenSteam()
}

func (s *Session) OpenSteam() *Stream {
	s.streamLock.Lock()
	defer s.streamLock.Unlock()

	if s.closed {
		return nil
	}

	stream := &Stream{
		id:      s.nextId,
		session: s,
	}
	s.nextId++
	s.streams[stream.id] = stream
	return stream
}

func (s *Session) Accept() *Stream {

}
