package mux

import (
	"log"
	"net"
	"sync"
	"time"
)

type Session struct {
	Id      uint16
	manager *SessionManager
	sub     net.Conn
	closed  bool
}

// 1 master <=> n sub
type SessionManager struct {
	master   net.Conn
	nextId   uint16
	mux      *sync.RWMutex
	iomux    *sync.RWMutex
	sessions map[uint16]*Session // nil: closed
}

func NewSessionManager(master net.Conn) *SessionManager {
	sm := &SessionManager{
		master:   master,
		nextId:   1,
		mux:      new(sync.RWMutex),
		iomux:    new(sync.RWMutex),
		sessions: make(map[uint16]*Session),
	}
	return sm
}

func (sm *SessionManager) Available() bool {
	sm.mux.RLock()
	defer sm.mux.RUnlock()

	return true
	//return len(sm.sessions) <= 8
}

// 生产新session, @local
func (sm *SessionManager) CreateSession(sub net.Conn) *Session {
	sm.mux.Lock()
	defer sm.mux.Unlock()

	id := sm.nextId
	sm.nextId++
	session := &Session{
		Id:      id,
		manager: sm,
		sub:     sub,
	}
	sm.sessions[id] = session
	//debug("sessions:", len(sm.sessions))
	return session
}

// 添加session, @remote
func (sm *SessionManager) AddSession(id uint16, sub net.Conn) *Session {
	sm.mux.Lock()
	defer sm.mux.Unlock()

	if sm.sessions == nil {
		return nil
	}

	session := &Session{
		Id:      id,
		sub:     sub,
		manager: sm,
		closed:  false,
	}

	sm.sessions[id] = session
	//debug("sessions:", len(sm.sessions))
	return session
}

func (sm *SessionManager) RemoveSession(id uint16) {
	sm.mux.Lock()
	defer sm.mux.Unlock()

	if sm.sessions == nil {
		return
	}

	delete(sm.sessions, id)
}

func (sm *SessionManager) Write(data []byte) error {
	st := time.Now()
	sm.iomux.Lock()
	defer sm.iomux.Unlock()

	n, err := sm.master.Write(data)
	took := time.Since(st)
	if took > 1*time.Second {
		debug("slow write took:", took)
	}
	if n != len(data) && err == nil {
		log.Fatal("write error: n != len(data)")
	}
	return err
}

func (sm *SessionManager) Close() {
	sm.mux.Lock()
	defer sm.mux.Unlock()

	if sm.sessions == nil {
		return
	}

	sm.master.Close()
	for _, s := range sm.sessions {
		s.Destory()
	}
	sm.sessions = nil
	debug("master closed:", sm.master.RemoteAddr())
}

func (sm *SessionManager) Closed() bool {
	return sm.sessions == nil
}

func (sm *SessionManager) GetSession(id uint16) *Session {
	sm.mux.RLock()
	defer sm.mux.RUnlock()
	return sm.sessions[id]
}

func (sm *SessionManager) RunOnLocal() {
	master := sm.master
	defer sm.Close()
	for {
		frame := ReadFrame(master)
		if frame == nil {
			break
		}
		session := sm.GetSession(frame.SessionId)
		if session == nil {
			continue
		}
		if frame.Stage == StageTransfer {
			_, ew := session.sub.Write(frame.Payload)
			if ew != nil {
				session.Close()
			}
		} else if frame.Stage == StageClose {
			session.Close()
			debug("recv close:", frame.SessionId)
		}
	}
}

// connect to dst addr
func dialDst(addr string) net.Conn {
	dstAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		log.Println("resolve dst err:", err)
		return nil
	}
	dst, err := net.DialTCP("tcp", nil, dstAddr)
	if err != nil {
		log.Println("dial dst err:", err)
		return nil
	}
	return dst
}

func (sm *SessionManager) RunOnRemote() {
	master := sm.master
	defer func() {
		sm.Close()
	}()
	for {
		frame := ReadFrame(master)
		if frame == nil {
			break
		}
		//log.Println("recv frame:", frame.SessionId, frame.Stage, len(frame.Payload))
		if frame.Stage == StageEstablish {
			if _, ok := sm.sessions[frame.SessionId]; ok {
				log.Fatal("impossible: sessionId exists!", frame.SessionId)
			}
			addr := string(frame.Payload)
			debug("recv establish:", frame.SessionId, addr)
			dst := dialDst(addr)
			if dst != nil {
				session := sm.AddSession(frame.SessionId, dst)
				if session != nil {
					go session.HandleRemote()
				}
			} else {
				debug("dial remote fail:", addr)
				reply := EncodeFrame(frame.SessionId, StageClose, nil)
				err := sm.Write(reply)
				if err != nil {
					break
				}
			}
		} else if frame.Stage == StageTransfer {
			session := sm.GetSession(frame.SessionId)
			if session == nil {
				continue
			}
			_, err := session.sub.Write(frame.Payload)
			if err != nil {
				session.Close()
				reply := EncodeFrame(session.Id, StageClose, nil)
				err := sm.Write(reply)
				if err != nil {
					break
				}
			}
		} else if frame.Stage == StageClose {
			session := sm.GetSession(frame.SessionId)
			if session == nil {
				continue
			}
			session.Close()
			debug("recv close:", frame.SessionId)
		}
	}
}

func (s *Session) HandleLocal(remoteAddr string) {
	debug("ready establish:", s.Id, remoteAddr)
	sub := s.sub
	defer func() {
		if !s.closed {
			s.Close()
			frame := EncodeFrame(s.Id, StageClose, nil)
			err := s.manager.Write(frame)
			if err != nil {
				s.manager.Close()
			}
		}
	}()
	// send establish frame
	data := EncodeFrame(s.Id, StageEstablish, []byte(remoteAddr))
	//master.SetWriteDeadline(time.Now().Add(1 * time.Second))
	err := s.manager.Write(data)
	if err != nil {
		s.manager.Close()
		return
	}
	// upstream
	var buf = make([]byte, 8192)
	for {
		n, er := sub.Read(buf)
		var ew error
		if n > 0 {
			frame := EncodeFrame(s.Id, StageTransfer, buf[:n])
			ew = s.manager.Write(frame)
		}
		if er != nil {
			break
		} else if ew != nil {
			s.manager.Close()
			break
		}
	}
}

func (s *Session) HandleRemote() {
	dst := s.sub
	defer func() {
		if !s.closed {
			s.Close()
			frame := EncodeFrame(s.Id, StageClose, nil)
			ew := s.manager.Write(frame)
			if ew != nil {
				s.manager.Close()
			}
		}
	}()
	var buf = make([]byte, 8192)
	for {
		n, er := dst.Read(buf)
		var ew error
		if n > 0 {
			frame := EncodeFrame(s.Id, StageTransfer, buf[:n])
			ew = s.manager.Write(frame)
		}
		if ew != nil {
			s.manager.Close()
			break
		} else if er != nil {
			break
		}
	}
}

func (s *Session) Destory() {
	if s.closed {
		return
	}
	s.closed = true
	s.sub.Close()
}

func (s *Session) Close() {
	if s.closed {
		return
	}
	s.Destory()
	s.manager.RemoveSession(s.Id)
	debug("session closed:", s.Id, s.sub.RemoteAddr())
}
