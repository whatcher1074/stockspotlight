package cache

import (
	"sync"
	"time"
)

type cacheEntry struct {
	data      interface{}
	timestamp time.Time
	ttl       time.Duration
}

type Cache struct {
	data map[string]cacheEntry
	mu   sync.RWMutex
}

// New creates a new cache
func New() *Cache {
	return &Cache{
		data: make(map[string]cacheEntry),
	}
}

// Set adds or updates a cache entry
func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = cacheEntry{
		data:      value,
		timestamp: time.Now(),
		ttl:       ttl,
	}
}

// Get retrieves a value if not expired
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.data[key]
	if !ok {
		return nil, false
	}
	if time.Since(entry.timestamp) > entry.ttl {
		delete(c.data, key)
		return nil, false
	}
	return entry.data, true
}

// Delete removes a key manually
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}
