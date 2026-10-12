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

package loadbalancer

import (
	"maps"
	"net"
	"sync"
)

type liveConns struct {
	mu sync.Mutex
	by map[string]map[*relay]net.Conn
}

func (c *liveConns) add(addr string, src net.Conn, r *relay) func() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.by == nil {
		c.by = map[string]map[*relay]net.Conn{}
	}

	if c.by[addr] == nil {
		c.by[addr] = map[*relay]net.Conn{}
	}

	c.by[addr][r] = src

	return func() { c.remove(addr, r) }
}

func (c *liveConns) remove(addr string, r *relay) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.by[addr], r)

	if len(c.by[addr]) == 0 {
		delete(c.by, addr)
	}
}

func (c *liveConns) closeAll(addr string) int {
	c.mu.Lock()
	conns := maps.Clone(c.by[addr])
	c.mu.Unlock()

	for r, src := range conns {
		closeBoth(src, r)
	}

	return len(conns)
}
