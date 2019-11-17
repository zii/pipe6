// m:n multiplex proxy model @local
package mux

import (
	"net"
	"sync"

	"github.com/xtaci/smux"
	"github.com/zii/pipe6/base"
)

type Connector func() (net.Conn, error)

type WorkerPool struct {
	connector Connector
	mux       *sync.RWMutex
	nextId    int
	workers   map[int]*smux.Session
}

func NewWorkerPool(connector func() (net.Conn, error)) *WorkerPool {
	w := &WorkerPool{
		connector: connector,
		mux:       new(sync.RWMutex),
		nextId:    1,
		workers:   make(map[int]*smux.Session),
	}
	return w
}

func (p *WorkerPool) pickWorker() *smux.Session {
	p.mux.Lock()
	defer p.mux.Unlock()

	for id, worker := range p.workers {
		if worker.IsClosed() {
			delete(p.workers, id)
			continue
		}
		ns := worker.NumStreams()
		debug("streams:", id, ns)
		if ns < 20 {
			return worker
		}
	}
	return nil
}

func (p *WorkerPool) createWorker() *smux.Session {
	p.mux.Lock()
	defer p.mux.Unlock()

	remote, err := p.connector()
	if err != nil {
		debug("create worker err:", err)
		return nil
	}
	id := p.nextId
	p.nextId++
	session, err := smux.Client(remote, nil)
	base.Raise(err)
	p.workers[id] = session
	return session
}

func (p *WorkerPool) GetWorker() *smux.Session {
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
