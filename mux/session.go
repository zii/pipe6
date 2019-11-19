package mux

import (
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type WriteRequest struct {
	cmd    byte
	sid    uint32
	data   []byte
	result chan WriteResult
}

type WriteResult struct {
	n   int
	err error
}

type Session struct {
	c       net.Conn
	dieLock sync.Once
	die     chan struct{}

	nextId     uint32
	streams    map[uint32]*Stream
	streamLock sync.Mutex

	config *Config

	chAccept chan *Stream
	chWrite  chan WriteRequest

	active int32
}

func (s *Session) Close() {
	var did = true
	s.dieLock.Do(func() {
		fmt.Println("Session.Close()")
		close(s.die)
		did = false
	})
	if did {
		return
	}
	s.streamLock.Lock()
	for _, stream := range s.streams {
		stream.notifyClose()
	}
	s.c.Close()
	s.streamLock.Unlock()
}

func (s *Session) removeStream(sid uint32) {
	s.streamLock.Lock()
	delete(s.streams, sid)
	s.streamLock.Unlock()
}

func (s *Session) LocalAddr() net.Addr {
	return s.c.LocalAddr()
}

func (s *Session) RemoteAddr() net.Addr {
	return s.c.RemoteAddr()
}

///////////////

func newSession(conn net.Conn, config *Config, client bool) *Session {
	if config == nil {
		config = DefaultConfig()
	}
	s := &Session{
		c:          conn,
		nextId:     1,
		streams:    make(map[uint32]*Stream),
		streamLock: sync.Mutex{},
		config:     config,
		dieLock:    sync.Once{},
		die:        make(chan struct{}),
		chWrite:    make(chan WriteRequest, config.WriteQueueSize),
	}
	if !client {
		s.chAccept = make(chan *Stream, config.Backlog)
	}
	go s.recvLoop()
	go s.sendLoop()
	go s.keepAliveLoop()
	return s
}

func Server(conn net.Conn, config *Config) *Session {
	return newSession(conn, config, false)
}

func Client(conn net.Conn, config *Config) *Session {
	return newSession(conn, config, true)
}

func (s *Session) NumStreams() int {
	s.streamLock.Lock()
	n := len(s.streams)
	s.streamLock.Unlock()
	return n
}

func (s *Session) Closed() bool {
	select {
	case <-s.die:
		return true
	default:
		return false
	}
}

func (s *Session) Open() (net.Conn, error) {
	return s.OpenStream()
}

func (s *Session) OpenStream() (*Stream, error) {
	if s.Closed() {
		return nil, ErrSessionDead
	}

	sid := s.nextId
	s.nextId++
	stream := newStream(sid, s, s.config.MaxFrameSize)

	debug("open stream:", sid)

	_, err := s.writeFrame(cmdSYN, sid, nil, nil)
	if err != nil {
		return nil, err
	}

	s.streamLock.Lock()
	s.streams[stream.id] = stream
	s.streamLock.Unlock()

	return stream, nil
}

func (s *Session) Accept() *Stream {
	select {
	case stream := <-s.chAccept:
		return stream
	case <-s.die:
		debug("Accept die!")
		return nil
	}
}

func (s *Session) recvLoop() {
	defer s.Close()
	for {
		var bh [sizeHeader]byte
		_, err := io.ReadFull(s.c, bh[:])
		if err != nil {
			debug("recv#1")
			break
		}
		hdr, err := decHdr(bh)
		if err != nil {
			debug("decHdr:", err)
			break
		}
		atomic.StoreInt32(&s.active, 1)
		var data []byte
		if hdr.n > 0 {
			data = make([]byte, hdr.n)
			_, err := io.ReadFull(s.c, data)
			if err != nil {
				debug("recv#2")
				break
			}
			atomic.StoreInt32(&s.active, 1)
		}
		sid := hdr.sid
		cmd := hdr.cmd
		if cmd == cmdSYN {
			debug("接收 cmdSYN:", sid)
			s.streamLock.Lock()
			if _, ok := s.streams[sid]; !ok {
				stream := newStream(sid, s, s.config.MaxFrameSize)
				s.streams[sid] = stream
				select {
				case s.chAccept <- stream:
				case <-s.die:
					debug("接收die")
					return
				}
			}
			s.streamLock.Unlock()
		} else if cmd == cmdFIN {
			debug("接收 cmdFIN:", sid)
			s.streamLock.Lock()
			stream := s.streams[sid]
			if stream != nil {
				stream.notifyRead()
				stream.notifyClose()
			}
			s.streamLock.Unlock()
		} else if cmd == cmdPSH {
			debug("接收 cmdPSH:", sid)
			s.streamLock.Lock()
			stream := s.streams[sid]
			if stream != nil {
				stream.push(data)
				stream.notifyRead()
			}
			s.streamLock.Unlock()
		} else if cmd == cmdNOP {
		} else {
			debug("recv invalid cmd:", cmd)
			break
		}
	}
}

func (s *Session) sendLoop() {
	defer s.Close()
	for {
		select {
		case request := <-s.chWrite:
			//debug("Session.send:", request.sid, request.cmd, len(request.data))
			hdr := encHdr(newHdr(request.cmd, uint16(len(request.data)), request.sid))
			var buffers = [][]byte{hdr}
			if len(request.data) > 0 {
				buffers = append(buffers, request.data)
			}
			n, err := writeBuffers(s.c, buffers)
			n -= len(hdr)
			if n < 0 {
				n = 0
			}
			result := WriteResult{
				n:   n,
				err: err,
			}
			request.result <- result
			close(request.result)
			if err != nil {
				return
			}
		case <-s.die:
			return
		}
	}
}

func (s *Session) writeFrame(cmd byte, sid uint32, data []byte, deadline <-chan time.Time) (int, error) {
	request := WriteRequest{
		cmd:    cmd,
		sid:    sid,
		data:   data,
		result: make(chan WriteResult, 1),
	}

	debug("Session.writeFrame():", sid, cmd, len(data))
	select {
	case s.chWrite <- request:
	case <-s.die:
		return 0, ErrSessionDead
	case <-deadline:
		return 0, ErrTimeout
	}

	select {
	case result := <-request.result:
		return result.n, result.err
	case <-s.die:
		return 0, ErrSessionDead
	case <-deadline:
		return 0, ErrTimeout
	}
}

func (s *Session) keepAliveLoop() {
	tickerPing := time.NewTicker(s.config.PingInterval)
	tickerTimeout := time.NewTicker(s.config.KeepAliveInterval)
	defer tickerPing.Stop()
	defer tickerTimeout.Stop()

	for {
		select {
		case <-tickerPing.C:
			_, err := s.writeFrame(cmdNOP, 0, nil, nil)
			if err != nil {
				return
			}
		case <-tickerTimeout.C:
			if !atomic.CompareAndSwapInt32(&s.active, 1, 0) {
				s.Close()
				return
			}
		case <-s.die:
			return
		}
	}
}
