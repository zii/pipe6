// m:n multiplex proxy model @local
package main

import (
	"log"
	"net"
	"sync"

	"github.com/zii/pipe6/mux"
)

type Connector func() (net.Conn, error)

type WorkerPool struct {
	connector Connector
	mux       *sync.RWMutex
	nextId    int
	workers   map[int]*mux.Session
}

func NewWorkerPool(connector func() (net.Conn, error)) *WorkerPool {
	w := &WorkerPool{
		connector: connector,
		mux:       new(sync.RWMutex),
		nextId:    1,
		workers:   make(map[int]*mux.Session),
	}
	return w
}

func (p *WorkerPool) pickWorker() *mux.Session {
	p.mux.Lock()
	defer p.mux.Unlock()

	for id, worker := range p.workers {
		if worker.Closed() {
			delete(p.workers, id)
			continue
		}
		ns := worker.NumStreams()
		log.Println("streams:", id, ns)
		if ns < 20 {
			return worker
		}
	}
	return nil
}

func (p *WorkerPool) createWorker() *mux.Session {
	p.mux.Lock()
	defer p.mux.Unlock()

	remote, err := p.connector()
	if err != nil {
		log.Println("create worker err:", err)
		return nil
	}
	id := p.nextId
	p.nextId++
	session := mux.Client(remote, nil)
	p.workers[id] = session
	return session
}

func (p *WorkerPool) GetWorker() *mux.Session {
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
