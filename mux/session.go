package mux

import (
	"log"
	"net"
	"sync"
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
	sessions map[uint16]*Session // nil: closed
}

func NewSessionManager(master net.Conn) *SessionManager {
	sm := &SessionManager{
		master:   master,
		nextId:   1,
		mux:      new(sync.RWMutex),
		sessions: make(map[uint16]*Session),
	}
	return sm
}

func (sm *SessionManager) Available() bool {
	return true
	//return len(sm.sessions) <= 10
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
	debug("sessions:", len(sm.sessions))
	return session
}

// 添加session, @remote
func (sm *SessionManager) AddSession(id uint16, sub net.Conn) *Session {
	session := &Session{
		Id:      id,
		sub:     sub,
		manager: sm,
		closed:  false,
	}
	sm.mux.Lock()
	defer sm.mux.Unlock()

	sm.sessions[id] = session
	debug("sessions:", len(sm.sessions))
	return session
}

func (sm *SessionManager) RemoveSession(id uint16) {
	if sm.sessions == nil {
		return
	}
	sm.mux.Lock()
	defer sm.mux.Unlock()

	delete(sm.sessions, id)
}

func (sm *SessionManager) Close() {
	if sm.sessions == nil {
		return
	}
	sm.mux.Lock()
	defer sm.mux.Unlock()

	sm.master.Close()
	for _, s := range sm.sessions {
		s.Close()
	}
	sm.sessions = nil
}

func (sm *SessionManager) Closed() bool {
	return sm.sessions == nil
}

func (sm *SessionManager) GetSession(id uint16) *Session {
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
		debug("master closed:", master.RemoteAddr())
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
				go session.HandleRemote()
			} else {
				debug("dial remote fail:", addr)
				reply := EncodeFrame(frame.SessionId, StageClose, nil)
				_, err := master.Write(reply)
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
				_, err = master.Write(reply)
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
		} else {
			log.Fatal("invalid stage:", frame.Stage)
		}
	}
}

func (s *Session) HandleLocal(remoteAddr string) {
	debug("ready establish:", s.Id, remoteAddr)
	sub := s.sub
	master := s.manager.master
	defer func() {
		if !s.closed {
			s.Close()
			frame := EncodeFrame(s.Id, StageClose, nil)
			_, err := master.Write(frame)
			if err != nil {
				s.manager.Close()
			}
		}
	}()
	// send establish frame
	data := EncodeFrame(s.Id, StageEstablish, []byte(remoteAddr))
	_, err := master.Write(data)
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
			_, ew = master.Write(frame)
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
	master := s.manager.master
	dst := s.sub
	defer func() {
		if !s.closed {
			s.Close()
			frame := EncodeFrame(s.Id, StageClose, nil)
			_, ew := master.Write(frame)
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
			_, ew = master.Write(frame)
		}
		if ew != nil {
			s.manager.Close()
			break
		} else if er != nil {
			break
		}
	}
}

func (s *Session) Close() {
	if s.closed {
		return
	}
	s.closed = true
	s.sub.Close()
	s.manager.RemoveSession(s.Id)
	debug("session closed:", s.Id, s.sub.RemoteAddr())
}
