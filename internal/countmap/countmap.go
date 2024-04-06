package countmap

import (
	"sync"

	"golang.org/x/exp/constraints"
)

// CountMap is a simple synchronized map storing an integer count for each key.
type CountMap[K comparable, V constraints.Integer] struct {
	mu     sync.Mutex
	counts map[K]V
}

// New initializes a new CountMap.
func New[K comparable, V constraints.Integer]() *CountMap[K, V] {
	return &CountMap[K, V]{
		counts: make(map[K]V),
	}
}

// Inc increments the count for the given key and returns the new count.
func (cm *CountMap[K, V]) Inc(key K) V {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	val := cm.counts[key]
	val++
	cm.counts[key] = val
	return val
}

// Dec decrements the count for the given key and returns the new count.
func (cm *CountMap[K, V]) Dec(key K) V {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	val := cm.counts[key]
	val--
	cm.counts[key] = val
	return val
}
