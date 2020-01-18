package stats

import (
	"sync"
)

// Queue provides a queuing structure for Incoming{}
type Queue struct {
	sync.RWMutex
	values []*Incoming
}

// NewQueue creates a new *Queue instance
func NewQueue() *Queue {
	return &Queue{
		values: make([]*Incoming, 0),
	}
}

// NewQueues creates a slice of *Queue instances
func NewQueues(size int) []*Queue {
	result := make([]*Queue, size)
	for i := 0; i < size; i++ {
		result[i] = NewQueue()
	}
	return result
}

// Push adds a new item to the queue
func (p *Queue) Push(item *Incoming) error {
	p.Lock()
	defer p.Unlock()
	p.values = append(p.values, item)
	return nil
}

// Clear returns current queue items and clears it
func (p *Queue) Clear() (result []*Incoming) {
	length := p.Length()

	p.Lock()
	defer p.Unlock()

	result, p.values = p.values[:length], p.values[length:]
	return
}

// Length returns the current queue size
func (p *Queue) Length() int {
	p.RLock()
	defer p.RUnlock()
	return len(p.values)
}
