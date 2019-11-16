// m:n multiplex proxy model @local
package mux

import (
	"net"
	"sync"
)

type Connector func() (net.Conn, error)

type WorkerPool struct {
	connector Connector
	mux       *sync.RWMutex
	nextId    int
	workers   map[int]*SessionManager
}

func NewWorkerPool(connector func() (net.Conn, error)) *WorkerPool {
	w := &WorkerPool{
		connector: connector,
		mux:       new(sync.RWMutex),
		nextId:    1,
		workers:   make(map[int]*SessionManager),
	}
	return w
}

func (p *WorkerPool) pickWorker() *SessionManager {
	p.mux.RLock()
	defer p.mux.RUnlock()

	for id, worker := range p.workers {
		if worker.Closed() {
			delete(p.workers, id)
			continue
		}
		if worker.Available() {
			return worker
		}
	}
	return nil
}

func (p *WorkerPool) createWorker() *SessionManager {
	p.mux.Lock()
	defer p.mux.Unlock()

	remote, err := p.connector()
	if err != nil {
		debug("create worker err:", err)
		return nil
	}
	id := p.nextId
	p.nextId++
	sm := NewSessionManager(remote)
	go sm.RunOnLocal()
	p.workers[id] = sm
	return sm
}

func (p *WorkerPool) GetWorker() *SessionManager {
	worker := p.pickWorker()
	if worker != nil {
		return worker
	}
	worker = p.createWorker()
	return worker
}

func (p *WorkerPool) Size() int {
	return len(p.workers)
}
