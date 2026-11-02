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

package ledger

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A leader that hands the lease over stops answering, takes its address out of the route and settles
// the approvals whose writes a quorum read shows landed, before the next leader may take over.
func TestHandOverDrainsTheLandedApprovals(t *testing.T) {
	c := newClient(t, rq("a", 12, rl("pods", "3")))
	endpoints := &Endpoints{Client: c, Self: podA}
	require.NoError(t, endpoints.Publish(context.Background(), always))
	s := state(rl("pods", "10"), rl("pods", "2"), rl("pods", "2"))
	require.True(t, s.Decide(write("a", 10, rl("pods", "2"), rl("pods", "3")), time.Now().Add(-time.Second), always).Allowed)
	require.Equal(t, 1, s.PendingCount())

	r := &runner{
		opts: Options{Self: podA}, timing: Timing{WebhookTimeout: 10 * time.Millisecond}, log: logr.Discard(), state: s,
		metrics: NewMetrics(prometheus.NewRegistry()), reader: c, endpoints: endpoints, webhook: &http.Server{ReadHeaderTimeout: time.Second},
	}
	r.leader.Store(true)
	r.synced.Store(true)

	start := time.Now()
	r.handOver()

	assert.Less(t, time.Since(start), MinFenceWindow, "a drained ledger does not wait out the fence")
	assert.False(t, r.synced.Load(), "the ledger stops answering first")
	assert.Empty(t, slice(t, c).Endpoints, "the route no longer leads to it")
	assert.Zero(t, s.PendingCount(), "the approval whose write landed is settled")
}

// A replica that is not the leader has nothing to hand over.
func TestHandOverOfAFollower(t *testing.T) {
	r := &runner{}
	r.synced.Store(true)
	r.handOver()
	assert.True(t, r.synced.Load())
}

// client-go cancels the context of a leadership before it calls stepDown, and may start lead late: a
// lead that comes after the end of its leadership leaves the flags as stepDown left them.
func TestLateLeadLeavesTheFlags(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := &runner{log: logr.Discard(), metrics: NewMetrics(prometheus.NewRegistry())}
	r.lead(ctx)
	assert.False(t, r.leader.Load())
	assert.False(t, r.synced.Load())
}
