package stats

import (
	"testing"
)

func TestPool(t *testing.T) {
	assert := func(ok bool, format string, params ...interface{}) {
		if !ok {
			t.Fatalf(format, params...)
		}
	}
	pool := NewPool()
	assert(pool.Length() == 0, "Unexpected pool length: %d != 0", pool.Length())
	pool.Push(new(Incoming))
	pool.Push(new(Incoming))
	pool.Push(new(Incoming))
	assert(pool.Length() == 3, "Unexpected pool length: %d != 3", pool.Length())

	items := pool.Clear()
	assert(len(items) == 3, "Unexpected items length: %d != 3", len(items))
	assert(pool.Length() == 0, "Unexpected pool length: %d != 0", pool.Length())
}