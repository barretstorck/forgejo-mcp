// Package doccache memoizes per-document extraction results, keyed by
// "<blobSHA>|<extractorVersion>" so cache entries invalidate automatically
// when a file changes in git or the extractor is upgraded.
package doccache

import (
	"sync"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/extract"
)

type Cache struct {
	mu       sync.Mutex
	capacity int
	entries  map[string][]extract.PageText
	order    []string // FIFO insertion order for eviction
}

func New(capacity int) *Cache {
	if capacity < 1 {
		capacity = 1
	}
	return &Cache{capacity: capacity, entries: make(map[string][]extract.PageText)}
}

func (c *Cache) Get(key string) ([]extract.PageText, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pages, ok := c.entries[key]
	return pages, ok
}

func (c *Cache) Put(key string, pages []extract.PageText) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; !exists {
		if len(c.order) >= c.capacity {
			oldest := c.order[0]
			c.order = c.order[1:]
			delete(c.entries, oldest)
		}
		c.order = append(c.order, key)
	}
	c.entries[key] = pages
}
