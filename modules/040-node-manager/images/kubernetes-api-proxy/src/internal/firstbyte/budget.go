/*
Copyright 2026 Flant JSC

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

package firstbyte

import (
	"sync"
	"time"
)

const chainFloors = 3

type Budget struct {
	mu    sync.Mutex
	floor time.Duration
	wait  map[string]time.Duration
}

func New(floor time.Duration) *Budget {
	return &Budget{floor: floor, wait: map[string]time.Duration{}}
}

func (b *Budget) Wait(addr string) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.waitLocked(addr)
}

func (b *Budget) Observe(addr string, elapsed time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.wait[addr] = min(max(2*elapsed, b.floor), 2*b.floor)
}

func (b *Budget) Timeout(addr string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.wait[addr] = min(b.waitLocked(addr)*3/2, 2*b.floor)
}

func (b *Budget) Chain() time.Duration {
	return chainFloors * b.floor
}

func (b *Budget) waitLocked(addr string) time.Duration {
	if wait, ok := b.wait[addr]; ok {
		return wait
	}

	return b.floor
}
