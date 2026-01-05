package ollama

import (
	"context"
	"sync"
	"time"
)

// ShowCache caches /api/show results per model to avoid repeated upstream calls.
//
// This keeps latency low and reduces load on Ollama.
type ShowCache struct {
	client *Client
	ttl    time.Duration

	mu      sync.Mutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	value   ShowResponse
	expires time.Time
}

func NewShowCache(client *Client, ttl time.Duration) *ShowCache {
	return &ShowCache{
		client:  client,
		ttl:     ttl,
		entries: make(map[string]cacheEntry),
	}
}

// Get returns the cached /api/show result or fetches a fresh one.
func (c *ShowCache) Get(ctx context.Context, model string) (ShowResponse, error) {
	if model == "" {
		return ShowResponse{}, nil
	}
	if c.ttl <= 0 {
		return c.client.Show(ctx, model, false)
	}

	now := time.Now()
	c.mu.Lock()
	ent, ok := c.entries[model]
	if ok && now.Before(ent.expires) {
		v := ent.value
		c.mu.Unlock()
		return v, nil
	}
	c.mu.Unlock()

	// Fetch without holding the lock.
	v, err := c.client.Show(ctx, model, false)
	if err != nil {
		return ShowResponse{}, err
	}

	c.mu.Lock()
	c.entries[model] = cacheEntry{value: v, expires: now.Add(c.ttl)}
	c.mu.Unlock()
	return v, nil
}
