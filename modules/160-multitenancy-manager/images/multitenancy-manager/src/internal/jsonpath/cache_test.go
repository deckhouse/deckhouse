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

package jsonpath

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func key(i int) string { return fmt.Sprintf("$.spec.f%d", i) }

func TestCacheIsBounded(t *testing.T) {
	c := NewWithCache()
	for i := range MaxCachedPaths + 100 {
		if _, err := c.Path(key(i)); err != nil {
			t.Fatalf("parse %q: %v", key(i), err)
		}
	}
	if n := c.cache.Len(); n > MaxCachedPaths {
		t.Fatalf("cache holds %d entries, want at most %d", n, MaxCachedPaths)
	}
}

func TestCacheEvictsLeastRecentlyUsed(t *testing.T) {
	c := NewWithCache()
	hot, err := c.Path(key(0))
	if err != nil {
		t.Fatal(err)
	}
	cold, err := c.Path(key(1))
	if err != nil {
		t.Fatal(err)
	}
	// Touch the hot entry after every insert so it stays the most recently used.
	for i := 2; i < MaxCachedPaths+2; i++ {
		if _, err := c.Path(key(i)); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Path(key(0)); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := c.Path(key(0)); got != hot {
		t.Error("recently used path was evicted")
	}
	if got, _ := c.Path(key(1)); got == cold {
		t.Error("least recently used path survived eviction")
	}
}

func TestCacheHitReturnsSamePath(t *testing.T) {
	c := NewWithCache()
	a, err := c.Path("$.metadata.annotations['cert-manager.io/cluster-issuer']")
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.Path("$.metadata.annotations['cert-manager.io/cluster-issuer']")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Error("cache hit returned a different *Path")
	}
}

func TestCacheSkipsParseErrorsAndLongPaths(t *testing.T) {
	c := NewWithCache()
	if _, err := c.Path("$.spec[?"); err == nil {
		t.Fatal("expected a parse error")
	}
	long := "$." + strings.Repeat("a", MaxCachedPathLen)
	if _, err := c.Path(long); err != nil {
		t.Fatal(err)
	}
	if n := c.cache.Len(); n != 0 {
		t.Errorf("cache holds %d entries, want 0", n)
	}
}

// TestPathRejectsLongExpressions: the parser recurses without a depth limit, and 800000 nested
// parentheses overflow the stack, which recover cannot catch. Path must refuse such input before
// parsing it, while nesting that fits in MaxPathLen still parses.
func TestPathRejectsLongExpressions(t *testing.T) {
	nested := func(depth int) string {
		return "$[?" + strings.Repeat("(", depth) + "@" + strings.Repeat(")", depth) + "]"
	}
	c := NewWithCache()
	fits := nested((MaxPathLen - len("$[?@]")) / 2)
	if len(fits) > MaxPathLen {
		t.Fatalf("test expression is %d bytes, want at most %d", len(fits), MaxPathLen)
	}
	if _, err := c.Path(fits); err != nil {
		t.Fatalf("expression of %d bytes: %v", len(fits), err)
	}
	if _, err := c.Path("$." + strings.Repeat("a", MaxPathLen-1)); err == nil {
		t.Errorf("expression of %d bytes parsed, want an error", MaxPathLen+1)
	}
	if _, err := c.Path(nested(800000)); err == nil {
		t.Error("deeply nested expression parsed, want an error")
	}
}

func TestCacheConcurrentAccess(t *testing.T) {
	c := NewWithCache()
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 2000 {
				p, err := c.Path(key((g*7 + i) % (MaxCachedPaths + 500)))
				if err != nil || p == nil {
					t.Errorf("Path: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if n := c.cache.Len(); n > MaxCachedPaths {
		t.Fatalf("cache holds %d entries, want at most %d", n, MaxCachedPaths)
	}
}
