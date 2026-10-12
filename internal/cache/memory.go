package cache

import (
	"sync"
)

type item struct {
	v *Entry
}

type Memory struct {
	mu  sync.RWMutex
	m   map[string]*item
	max int
	ttl int64
}

func NewMemory(max int, ttl int64) *Memory {
	if max <= 0 {
		max = 1000
	}
	return &Memory{m: make(map[string]*item, max), max: max, ttl: ttl}
}

func (c *Memory) Get(key string) (*Entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	it, ok := c.m[key]
	if !ok {
		return nil, false
	}
	return it.v, true
}

func (c *Memory) Set(key string, e *Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= c.max {
		// simple evict: clear some? or random; keep simple: drop oldest not tracked; just overwrite by deleting one
		for k := range c.m {
			delete(c.m, k)
			break
		}
	}
	c.m[key] = &item{v: e}
}
