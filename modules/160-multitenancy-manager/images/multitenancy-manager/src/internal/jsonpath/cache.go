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

	"github.com/theory/jsonpath"
	"k8s.io/utils/lru"
)

var _ Factory = &CachingFactory{}

const (
	// MaxPathLen is the longest expression Path parses. The parser recurses without a depth limit,
	// so a deeply nested expression (a filter with hundreds of thousands of parentheses) overflows
	// the goroutine stack: a fatal error that recover cannot catch, so a single AdmissionReview would
	// restart the controller. The CRDs cap the path fields at 256 characters, at most 1024 bytes of
	// UTF-8, so a stored path never hits this limit.
	MaxPathLen = 1024

	// MaxCachedPaths bounds the number of parsed paths the cache keeps. The paths worth caching are
	// the ones stored in GrantableClusterResourceReferences, GrantableClusterResourceDefinitions and
	// their match guards: tens to hundreds in a cluster. Every AdmissionReview the webhooks decode
	// also goes through the cache, including rejected and dry-run objects that are never stored, and
	// the webhook server is reachable from any pod without a client certificate. An unbounded cache
	// would let anyone inflate it until the controller is OOM-killed, taking down /is-granted, which
	// runs with failurePolicy: Fail. A thousand entries keep every legitimate path with a wide
	// margin, while junk paths only evict each other and the least recently used ones.
	MaxCachedPaths = 1024

	// MaxCachedPathLen keeps expressions longer than this many bytes out of the cache (they are still
	// parsed). It equals the maxLength of the path fields in the CRDs, which the API server counts in
	// characters, so a stored ASCII path is always cached and a non-ASCII one near the limit may be
	// parsed on every use instead: a cost in CPU only. Real paths are a few dozen ASCII characters.
	// A parsed path is much larger than its source, up to about 40 bytes of heap per source byte for
	// the densest shapes (one-letter member names, bare filters such as [?@], descendant segments):
	// a cache full of such paths of MaxCachedPathLen measured about 10.5 MiB, one full of index
	// lists or slices about 3 MiB. Budget ~11 MiB as the worst case.
	MaxCachedPathLen = 256
)

// CachingFactory parses JSONPath expressions and keeps the successfully parsed ones in a bounded,
// concurrency-safe LRU cache. Parse errors are not cached, so unparsable junk cannot evict valid
// paths; parseable junk can, and an evicted path is parsed again on its next use.
type CachingFactory struct {
	cache  *lru.Cache
	parser *jsonpath.Parser
}

func NewWithCache() *CachingFactory {
	return &CachingFactory{
		cache:  lru.New(MaxCachedPaths),
		parser: jsonpath.NewParser(),
	}
}

func (c *CachingFactory) Path(expr string) (*jsonpath.Path, error) {
	if p, found := c.cache.Get(expr); found {
		return p.(*jsonpath.Path), nil
	}

	if len(expr) > MaxPathLen {
		return nil, fmt.Errorf("JSONPath expression is %d bytes long, longer than the limit of %d", len(expr), MaxPathLen)
	}

	fieldPath, err := c.parser.Parse(expr)
	if err != nil {
		return nil, err
	}

	if len(expr) <= MaxCachedPathLen {
		c.cache.Add(expr, fieldPath)
	}
	return fieldPath, nil
}
