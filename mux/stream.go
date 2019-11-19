package mux

import (
	"net"
	"sync"
	"time"
)

type Stream struct {
	id           uint32
	session      *Session
	maxFrameSize int

	dieLock sync.Once
	die     chan struct{}

	chRead     chan struct{} // read buffers trigger
	buffers    [][]byte      // read buffers
	bufferLock sync.Mutex    // read buffers lock

	readDeadline  time.Time
	writeDeadline time.Time
}

func (s *Stream) ID() uint32 {
	return s.id
}

func (s *Stream) readOnce(b []byte) (int, error) {
	n := 0
	s.bufferLock.Lock()
	if len(s.buffers) > 0 {
		n = copy(b, s.buffers[0])
		s.buffers[0] = s.buffers[0][n:]
		if len(s.buffers[0]) == 0 {
			s.buffers = s.buffers[1:]
		}
	}
	s.bufferLock.Unlock()

	if n > 0 {
		return n, nil
	}

	var deadline <-chan time.Time
	if !s.readDeadline.IsZero() {
		timer := time.NewTimer(time.Until(s.readDeadline))
		defer timer.Stop()
		deadline = timer.C
	}

	select {
	case <-s.chRead:
		return 0, nil
	case <-s.die:
		return 0, ErrStreamDead
	case <-deadline:
		return n, ErrTimeout
	}
}

func (s *Stream) Read(b []byte) (int, error) {
	if len(b) == 0 {
		debug("Read: len(b) == 0")
		return 0, nil
	}

	for {
		n, err := s.readOnce(b)
		if n == 0 && err == nil {
			continue
		}
		debug("Read:", s.id, n, err)
		return n, err
	}
}

func (s *Stream) Write(b []byte) (int, error) {
	if s.Closed() {
		debug("Stream.Write(err1)", ErrStreamDead)
		return 0, ErrStreamDead
	}

	var deadline <-chan time.Time
	if !s.writeDeadline.IsZero() {
		timer := time.NewTimer(time.Until(s.writeDeadline))
		deadline = timer.C
	}

	var sent int
	for len(b) > 0 {
		var data []byte
		if len(b) > s.maxFrameSize {
			data = b[:s.maxFrameSize]
			b = b[s.maxFrameSize:]
		} else {
			data = b
			b = b[len(b):]
		}
		n, err := s.session.writeFrame(cmdPSH, s.id, data, deadline)
		sent += n
		if err != nil {
			debug("Stream.Write(err2)", err)
			return sent, err
		}
	}
	return sent, nil
}

func (s *Stream) Close() error {
	var did = true
	s.dieLock.Do(func() {
		debug("Stream.Close()", s.id)
		close(s.die)
		did = false
	})
	if did {
		return nil
	}
	s.session.writeFrame(cmdFIN, s.id, nil, nil)
	s.session.removeStream(s.id)
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
	s.readDeadline = t
	return nil
}

func (s *Stream) SetWriteDeadline(t time.Time) error {
	s.writeDeadline = t
	return nil
}

//////////

func newStream(id uint32, session *Session, maxFrameSize int) *Stream {
	return &Stream{
		id:           id,
		session:      session,
		maxFrameSize: maxFrameSize,
		dieLock:      sync.Once{},
		die:          make(chan struct{}),
		chRead:       make(chan struct{}),
	}
}

func (s *Stream) Closed() bool {
	select {
	case <-s.die:
		return true
	default:
		return false
	}
}

func (s *Stream) notifyClose() {
	s.dieLock.Do(func() {
		close(s.die)
		debug("Stream.notifyClose()")
	})
}

func (s *Stream) notifyRead() {
	select {
	case s.chRead <- struct{}{}:
	default:
	}
}

func (s *Stream) push(b []byte) {
	s.bufferLock.Lock()
	s.buffers = append(s.buffers, b)
	s.bufferLock.Unlock()
}
