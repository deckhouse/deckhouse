// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package cache memoizes the child services built for dynamic path segments.
package cache

import "sync"

// Cache memoizes the child services built for dynamic path segments — module,
// package, plugin, extra and security image names. Building a child is cheap
// (it only clones a client), but callers hold on to the returned pointers, so
// handing back the same instance for the same name keeps the tree stable.
//
// registry.Client implementations are safe for concurrent use, and so is this
// cache, which keeps the whole service tree safe to share across goroutines.
type Cache[T any] struct {
	mu    sync.Mutex
	items map[string]T
}

// New creates an empty cache.
func New[T any]() *Cache[T] {
	return &Cache[T]{items: make(map[string]T)}
}

// Get returns the cached entry for name, building it with build on first use.
func (c *Cache[T]) Get(name string, build func() T) T {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, ok := c.items[name]; ok {
		return item
	}

	item := build()
	c.items[name] = item

	return item
}
