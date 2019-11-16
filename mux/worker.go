// m:n multiplex proxy model @local
package mux

import (
	"net"
	"sync"

	"github.com/hashicorp/yamux"
	"github.com/zii/pipe6/base"
)

type Connector func() (net.Conn, error)

type WorkerPool struct {
	connector Connector
	mux       *sync.RWMutex
	nextId    int
	workers   map[int]*yamux.Session
}

func NewWorkerPool(connector func() (net.Conn, error)) *WorkerPool {
	w := &WorkerPool{
		connector: connector,
		mux:       new(sync.RWMutex),
		nextId:    1,
		workers:   make(map[int]*yamux.Session),
	}
	return w
}

func (p *WorkerPool) pickWorker() *yamux.Session {
	p.mux.Lock()
	defer p.mux.Unlock()

	for id, worker := range p.workers {
		if worker.IsClosed() {
			delete(p.workers, id)
			continue
		}
		debug("streams:", worker.NumStreams())
		return worker
	}
	return nil
}

func (p *WorkerPool) createWorker() *yamux.Session {
	p.mux.Lock()
	defer p.mux.Unlock()

	remote, err := p.connector()
	if err != nil {
		debug("create worker err:", err)
		return nil
	}
	id := p.nextId
	p.nextId++
	session, err := yamux.Client(remote, nil)
	base.Raise(err)
	p.workers[id] = session
	return session
}

func (p *WorkerPool) GetWorker() *yamux.Session {
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
