package stats

import (
	"sync"
)

type Pool struct {
	sync.RWMutex
	values []*Incoming
}

func NewPool() *Pool {
	return &Pool{
		values: make([]*Incoming, 0),
	}
}

func (p *Pool) Clear() (result []*Incoming) {
	length := p.Length()

	p.Lock()
	defer p.Unlock()

	result, p.values = p.values[:length], p.values[length:]
	return
}

func (p *Pool) Push(item *Incoming) {
	p.Lock()
	defer p.Unlock()
	p.values = append(p.values, item)
}

func (p *Pool) Length() int {
	p.RLock()
	defer p.RUnlock()
	return len(p.values)
}
