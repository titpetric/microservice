package stats

import (
	"testing"
)

func TestQueue(t *testing.T) {
	assert := func(ok bool, format string, params ...interface{}) {
		if !ok {
			t.Fatalf(format, params...)
		}
	}

	queue := NewQueue()
	assert(queue.Length() == 0, "Unexpected queue length: %d != 0", queue.Length())

	assert(nil == queue.Push(new(Incoming)), "Expected no error on queue.Push")
	assert(nil == queue.Push(new(Incoming)), "Expected no error on queue.Push")
	assert(nil == queue.Push(new(Incoming)), "Expected no error on queue.Push")

	assert(queue.Length() == 3, "Unexpected queue length: %d != 3", queue.Length())

	items := queue.Clear()
	assert(len(items) == 3, "Unexpected items length: %d != 3", len(items))
	assert(queue.Length() == 0, "Unexpected queue length: %d != 0", queue.Length())

	queues := NewQueues(16)
	assert(len(queues) == 16, "Unexpected queue count: %d != 16", len(queues))
	for k, v := range queues {
		assert(v != nil, "Unexpected queue value: expected not nil, index %d", k)
	}
}
