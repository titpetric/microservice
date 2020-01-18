package stats

import (
	"sync"
)

// Pool provides a queuing structure for Incoming{}
type Pool struct {
	sync.RWMutex
	values []*Incoming
}

// NewPool creates a new *Pool instance
func NewPool() *Pool {
	return &Pool{
		values: make([]*Incoming, 0),
	}
}

// NewPools creates a slice of *Pool instances
func NewPools(size int) []*Pool {
	result := make([]*Pool, size)
	for i := 0; i < size; i++ {
		result[i] = NewPool()
	}
	return result
}

// Clear returns current pool items and clears it
func (p *Pool) Clear() (result []*Incoming) {
	length := p.Length()

	p.Lock()
	defer p.Unlock()

	result, p.values = p.values[:length], p.values[length:]
	return
}

// Push adds a new item to the pool
func (p *Pool) Push(item *Incoming) {
	p.Lock()
	defer p.Unlock()
	p.values = append(p.values, item)
}

// Length returns the current pool size
func (p *Pool) Length() int {
	p.RLock()
	defer p.RUnlock()
	return len(p.values)
}
