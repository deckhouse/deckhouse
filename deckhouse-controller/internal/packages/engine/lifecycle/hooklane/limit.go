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

// A hook's execution rate (executionMinInterval, executionBurst) and the coalescing of its runs.

package hooklane

import (
	"sync"
	"time"

	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
)

// limiter holds a hook's runs to one per interval, burst of them at once: a token bucket with
// shell-operator's semantics (CreateRateLimiter: full at start), on the lanes' clock.
type limiter struct {
	interval time.Duration
	burst    float64

	mu     sync.Mutex
	tokens float64
	last   time.Time
}

func newLimiter(interval time.Duration, burst int) *limiter {
	return &limiter{interval: interval, burst: float64(max(burst, 1)), tokens: float64(max(burst, 1))}
}

// take takes a run at now; false and when the next one is allowed otherwise.
func (b *limiter) take(now time.Time) (time.Time, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.last.IsZero() {
		b.tokens = min(b.burst, b.tokens+float64(now.Sub(b.last))/float64(b.interval))
	}

	b.last = now

	if b.tokens >= 1 {
		b.tokens--
		return time.Time{}, true
	}

	return now.Add(time.Duration((1 - b.tokens) * float64(b.interval))), false
}

// mergeable reports whether it can join last, the last waiting item: the same hook, kind and
// failure policy, not a synchronization (it unlocks a monitor and may be waited for).
func mergeable(last, it *item) bool {
	return it.kind != KindSync && last.kind == it.kind && last.hook == it.hook && last.allowFailure == it.allowFailure
}

// appendCompact appends contexts to dst, replacing dst's last context when the next has the same
// non-empty group: shell-operator's group compaction (a context is dropped when the next one has
// its group), done as items are built and merged rather than at the run. A grouped binding reads
// its snapshots, which are taken at the run, so of consecutive contexts of one group only the last
// counts; compacting at delivery keeps a waiting item of a grouped hook at one context however many
// events join it. addon-operator puts every Go and batch hook in group "main", so this is the
// common case, not an edge.
func appendCompact(dst []bctx.BindingContext, contexts ...bctx.BindingContext) []bctx.BindingContext {
	for _, c := range contexts {
		if n := len(dst); n > 0 && c.Metadata.Group != "" && dst[n-1].Metadata.Group == c.Metadata.Group {
			dst[n-1] = c
			continue
		}

		dst = append(dst, c)
	}

	return dst
}
