package metadata

import (
	"sync"

	"github.com/9triver/iarnet/internal/domain/application/types"
)

type Cache struct {
	mu    sync.RWMutex
	cache map[string]types.AppMetadata
}

func NewCache() *Cache {
	return &Cache{
		cache: make(map[string]types.AppMetadata),
	}
}

func (c *Cache) GetAll() ([]types.AppMetadata, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	values := make([]types.AppMetadata, 0, len(c.cache))
	for _, metadata := range c.cache {
		values = append(values, metadata)
	}
	return values, nil
}

func (c *Cache) Get(appID string) (types.AppMetadata, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache[appID], nil
}

func (c *Cache) Set(appID string, metadata types.AppMetadata) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[appID] = metadata
}

func (c *Cache) Delete(appID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, appID)
}
