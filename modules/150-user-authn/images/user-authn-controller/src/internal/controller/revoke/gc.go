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

package revoke

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
)

// gcInterval is how often GC looks for Revocation objects past retention.
const gcInterval = 5 * time.Minute

// GC deletes Revocation objects older than the retention, whoever wrote them. Nothing a revocation
// revokes is valid longer than that (revocation.Retention), and the subscribers forget a revocation
// at the same time. It is a leader-only runnable.
type GC struct {
	client    client.Client
	retention time.Duration
	interval  time.Duration
	log       logr.Logger
	now       func() time.Time
}

var _ manager.LeaderElectionRunnable = (*GC)(nil)

// NewGC returns a GC. c reads from the cache and deletes. now defaults to time.Now when nil.
func NewGC(c client.Client, retention time.Duration, log logr.Logger, now func() time.Time) *GC {
	if now == nil {
		now = time.Now
	}
	return &GC{client: c, retention: retention, interval: gcInterval, log: log, now: now}
}

// Register adds the GC of revocations past retention to the manager and registers the metrics.
func Register(mgr manager.Manager, retention time.Duration) error {
	if err := RegisterMetrics(); err != nil {
		return err
	}
	gc := NewGC(mgr.GetClient(), retention, mgr.GetLogger().WithName("revocation-gc"), nil)
	if err := mgr.Add(gc); err != nil {
		return fmt.Errorf("add revocation gc: %w", err)
	}
	return nil
}

// NeedLeaderElection makes GC run on the leader only.
func (g *GC) NeedLeaderElection() bool {
	return true
}

// Start runs a pass at once and then every interval, until ctx ends. A failed pass is logged and
// the next one tries again. When it ends - the replica stops leading - the stored revocations
// gauge loses its series: the next leader counts them.
func (g *GC) Start(ctx context.Context) error {
	defer storedGauge.Reset()
	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()
	for {
		if err := g.Collect(ctx); err != nil && ctx.Err() == nil {
			g.log.Error(err, "Failed to delete expired revocations")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// Collect deletes the Revocation objects past retention and publishes how many are left.
func (g *GC) Collect(ctx context.Context) error {
	list := controller.List(controller.RevocationGVK)
	if err := g.client.List(ctx, list, client.InNamespace(naming.DexNamespace)); err != nil {
		return fmt.Errorf("list revocations: %w", err)
	}
	now := g.now()
	kept := map[string]int{revocation.TypePair: 0, revocation.TypeSession: 0, revocation.TypeGrant: 0}
	deleted := 0
	for i := range list.Items {
		item := &list.Items[i]
		rev, err := revocation.FromObject(item)
		if err != nil {
			// Retention counts from the creation timestamp, which does not depend on the fields
			// that failed.
			g.log.Error(err, "Failed to decode a revocation", "revocation", item.GetName())
			rev = revocation.Revocation{
				Name:    item.GetName(),
				Type:    item.GetLabels()[revocation.TypeLabel],
				Created: item.GetCreationTimestamp().UTC(),
			}
		}
		if rev.Time().IsZero() || !rev.Expires(g.retention).Before(now) {
			if _, known := kept[rev.Type]; known {
				kept[rev.Type]++
			}
			continue
		}
		if err := g.client.Delete(ctx, item); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("delete revocation %s: %w", item.GetName(), err)
		}
		deleted++
	}
	for t, n := range kept {
		storedGauge.WithLabelValues(t).Set(float64(n))
	}
	if deleted > 0 {
		g.log.Info("Deleted expired revocations", "deleted", deleted)
	}
	return nil
}
