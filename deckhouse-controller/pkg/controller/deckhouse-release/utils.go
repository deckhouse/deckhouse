package deckhouse_release

import "sync"

type container[T any] struct {
	mu  sync.Mutex
	val T
}

func (c *container[T]) Get() T {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.val
}

func (c *container[T]) Set(val T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.val = val
}
