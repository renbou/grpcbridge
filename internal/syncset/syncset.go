package syncset

import (
	"sync"
)

// SyncSet is a synchronized wrapper around map[K]struct{} usable in concurrent environments.
type SyncSet[K comparable] struct {
	mu  sync.Mutex
	set map[K]struct{}
}

// New initializes a new SyncSet.
func New[K comparable]() *SyncSet[K] {
	return &SyncSet[K]{
		set: make(map[K]struct{}),
	}
}

// Add adds a key to the set, returning true if the key was added.
func (ss *SyncSet[K]) Add(key K) bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if _, ok := ss.set[key]; ok {
		return false
	}

	ss.set[key] = struct{}{}
	return true
}

// Remove removes a key from the set, returning true if the key was removed.
func (ss *SyncSet[K]) Remove(key K) bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if _, ok := ss.set[key]; !ok {
		return false
	}

	delete(ss.set, key)
	return true
}
