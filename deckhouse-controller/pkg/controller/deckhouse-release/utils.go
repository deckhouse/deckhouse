/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
