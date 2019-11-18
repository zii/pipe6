package mux

import (
	"net"
	"time"
)

type Stream struct {
	id      int
	session *Session
}

func (s *Stream) Read(b []byte) (int, error) {
	return 0, nil
}

func (s *Stream) Write(b []byte) (int, error) {
	return 0, nil
}

func (s *Stream) Close() error {
	return nil
}

func (s *Stream) LocalAddr() net.Addr {
	return s.session.LocalAddr()
}

func (s *Stream) RemoteAddr() net.Addr {
	return s.session.RemoteAddr()
}

func (s *Stream) SetDeadline(t time.Time) error {
	er := s.SetReadDeadline(t)
	if er != nil {
		return er
	}
	ew := s.SetWriteDeadline(t)
	if ew != nil {
		return ew
	}
	return nil
}

func (s *Stream) SetReadDeadline(t time.Time) error {
	return nil
}

func (s *Stream) SetWriteDeadline(t time.Time) error {
	return nil
}
