package workflow

import "sync"

type keyedLocks struct {
	mu      sync.Mutex
	entries map[string]*lockEntry
}
type lockEntry struct {
	mu   sync.Mutex
	refs int
}

func newKeyedLocks() *keyedLocks { return &keyedLocks{entries: map[string]*lockEntry{}} }

func (k *keyedLocks) lock(key string) func() {
	k.mu.Lock()
	e := k.entries[key]
	if e == nil {
		e = &lockEntry{}
		k.entries[key] = e
	}
	e.refs++
	k.mu.Unlock()
	e.mu.Lock()
	return func() {
		e.mu.Unlock()
		k.mu.Lock()
		e.refs--
		if e.refs == 0 {
			delete(k.entries, key)
		}
		k.mu.Unlock()
	}
}
