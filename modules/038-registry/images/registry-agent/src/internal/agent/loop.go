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

// Package agent drives the node agent: it keeps the runtime pointed at itself, keeps
// its own routing rules current, and says what it is doing.
package agent

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-agent/internal/containerd"
	"github.com/deckhouse/registry-agent/internal/layout"
	"github.com/deckhouse/registry-agent/internal/metrics"
	"github.com/deckhouse/registry-agent/internal/pki"
	"github.com/deckhouse/registry-agent/internal/status"
	"github.com/deckhouse/registry-agent/internal/store"
)

// Loop keeps the node in step with its layout.
type Loop struct {
	Log *slog.Logger

	// Source produces the layout, from the API server or from the copy on disk.
	Source *layout.Source

	// Writer owns the runtime's drop-in directory.
	Writer *containerd.Writer

	// PKI is the agent's own certificate material on the node.
	PKI *pki.OnDisk

	// Publisher reports what the agent is doing. Optional: on a node that cannot reach
	// the API there is nobody to report to, and that must not stop anything.
	Publisher *status.Publisher

	// ProxyEndpoint is where the agent listens, as the runtime is told to reach it.
	ProxyEndpoint string

	// Serving reports whether the listener is up.
	Serving func() bool

	// Usable narrows backend names to those not known to be failing.
	Usable func([]string) []string

	// Connect obtains a client for the API server, and is retried until it succeeds.
	//
	// Optional, and separate from the client itself, because a node can start without
	// any credentials at all: the kubelet's kubeconfig does not exist until the node has
	// completed its TLS bootstrap, and the agent has to be pulling images before that.
	// Failing to start over it would be the wrong trade — the agent's whole reason for
	// existing is to work when the cluster does not.
	Connect func() (client.Client, error)

	// Metrics is what the agent reports about itself. Optional: nothing on the pull path
	// may depend on whether anyone is watching.
	Metrics *metrics.Metrics

	// StorePath is where the in-cluster cache keeps its blobs on this node, measured
	// while no cache is configured so that data left behind is not left behind quietly.
	// Empty disables the measurement.
	StorePath string

	// StoreReview is how often that measurement is redone. Rarely, because it walks a
	// directory that can hold tens of gigabytes, and the answer changes only when an
	// operator acts on it.
	StoreReview time.Duration

	// Interval is how often the layout is re-read.
	//
	// A poll rather than a watch: the agent has to survive an API server it cannot
	// reach at all, so it can never be correct only as long as a watch is alive. A
	// missed event on a working API is a delay; a missed reconnection would be a node
	// stuck on a layout nobody can see.
	Interval time.Duration

	// current is the layout the proxy routes by. Swapped as a whole so a request never
	// sees half of one.
	current atomic.Pointer[registryv1alpha1.RegistryNodeSpec]

	// staleBytes is the last measurement of the cache data left on this node, and when
	// it was taken.
	staleBytes atomic.Int64
	measuredAt atomic.Int64
}

// Current is the layout the proxy should route by.
func (l *Loop) Current() *registryv1alpha1.RegistryNodeSpec { return l.current.Load() }

// Run drives the loop until the context is cancelled.
func (l *Loop) Run(ctx context.Context) error {
	interval := l.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := l.once(ctx); err != nil {
			// Never fatal. An agent that exits takes every pull on the node with it,
			// which is worse than one that keeps serving what it already knows and says
			// what went wrong.
			l.Log.Error("the pass failed", "error", err.Error())
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// connect tries to obtain API credentials, once per pass, until it has them.
//
// Retried rather than required up front: the kubelet's kubeconfig appears only after the
// node has completed its TLS bootstrap, and until then the agent runs on the layout the
// node was installed with. Both the layout source and the status publisher are handed
// the client at the same moment, so the agent never reads from an API it cannot report
// to.
func (l *Loop) connect() {
	if l.Connect == nil || l.Source.Client != nil {
		return
	}

	built, err := l.Connect()
	if err != nil {
		l.Log.Debug("no API credentials yet, running on what the node has on disk",
			"error", err.Error())
		return
	}

	l.Log.Info("API credentials are available")
	l.Source.Client = built
	if l.Publisher != nil {
		l.Publisher.Client = built
	}
}

func (l *Loop) once(ctx context.Context) error {
	l.connect()

	snapshot, err := l.Source.Get(ctx)
	if err != nil {
		l.report(ctx, status.State{
			ProxyListening: l.serving(),
			Error:          err.Error(),
		})
		return err
	}

	if snapshot == nil {
		// Not a managed node, or the layout was removed. What the agent owns in the
		// runtime configuration goes away with it; anything an administrator wrote stays.
		l.current.Store(nil)

		result, err := l.Writer.Apply(containerd.Desired{Files: map[string][]byte{}})
		if err != nil {
			return err
		}
		if result.Changed() {
			l.Log.Info("the node is no longer managed, the runtime configuration was withdrawn",
				"removed", result.RemovedHosts)
		}
		return nil
	}

	spec, origin := snapshot.Spec, snapshot.Origin

	// The proxy is given the new layout BEFORE the runtime is pointed at it. The other
	// order would leave a window where the runtime sends requests the proxy cannot yet
	// route.
	l.current.Store(spec)

	authority, err := l.PKI.CA()
	if err != nil {
		l.report(ctx, status.State{
			ObservedGeneration: snapshot.Generation,
			ProxyListening:     l.serving(),
			FromCache:          origin != layout.OriginAPI,
			Error:              err.Error(),
		})
		return err
	}

	desired, err := containerd.Render(spec, containerd.Options{
		Root:          l.Writer.Root,
		ProxyEndpoint: l.ProxyEndpoint,
		ProxyCA:       authority,
	})
	if err != nil {
		l.report(ctx, status.State{
			ObservedGeneration: snapshot.Generation,
			ProxyListening:     l.serving(),
			Error:              err.Error(),
		})
		return err
	}

	result, err := l.Writer.Apply(desired)
	if err != nil {
		l.report(ctx, status.State{
			ObservedGeneration: snapshot.Generation,
			ProxyListening:     l.serving(),
			Error:              err.Error(),
		})
		return err
	}
	if result.Changed() {
		l.Log.Info("the runtime configuration was updated",
			"written", result.Written, "removed", result.RemovedHosts, "origin", origin)
	}

	l.observeOrigin(origin)

	l.report(ctx, status.State{
		ObservedGeneration:    snapshot.Generation,
		Reconciled:            true,
		ProxyListening:        l.serving(),
		ActiveBackends:        l.usable(spec),
		FromCache:             origin != layout.OriginAPI,
		StaleStorageDataBytes: l.staleStorage(spec),
	})
	return nil
}

// observeOrigin reports where the layout in use came from.
//
// Worth a metric of its own because "running on a copy from disk" looks like success from
// every other angle: the node pulls images, the condition is True, and the configuration may
// be behind the cluster's by any amount.
func (l *Loop) observeOrigin(origin layout.Origin) {
	if l.Metrics == nil {
		return
	}

	l.Metrics.LayoutOrigin.Reset()
	l.Metrics.LayoutOrigin.WithLabelValues(string(origin)).Set(1)
}

// staleStorage is how much of the cache's data is still on this node while no cache is
// configured.
//
// Measured here because the agent is the only component the module runs on every node, and
// because the data is deliberately kept: turning the cache back on refills from what is
// already there rather than from scratch. Keeping it is the right default; keeping it
// without saying so is not.
//
// Rate-limited, since this walks a directory that can hold tens of gigabytes. A stale
// number costs nothing — nothing acts on it automatically, and an operator reclaiming the
// space is the only thing that changes it.
func (l *Loop) staleStorage(spec *registryv1alpha1.RegistryNodeSpec) int64 {
	if l.StorePath == "" {
		return 0
	}
	if spec.Cache {
		// The cache is configured, so this is not data left behind; it is the store in
		// use. Reported as zero rather than measured, because "how big is the cache" is
		// a different question with a different owner.
		l.staleBytes.Store(0)
		l.measuredAt.Store(0)
		return 0
	}

	review := l.StoreReview
	if review <= 0 {
		review = 30 * time.Minute
	}

	now := time.Now()
	if last := l.measuredAt.Load(); last != 0 && now.Sub(time.Unix(last, 0)) < review {
		return l.staleBytes.Load()
	}

	size, err := store.Usage(l.StorePath)
	if err != nil {
		// Not reported as a reconciliation failure: the node pulls images perfectly well
		// either way, and a disk that cannot be walked is not a reason to say the
		// registry is broken.
		l.Log.Warn("cannot measure the storage data left on this node",
			"path", l.StorePath, "error", err.Error())
	}

	l.staleBytes.Store(size)
	l.measuredAt.Store(now.Unix())
	return size
}

// usable lists the backends the agent would currently use.
func (l *Loop) usable(spec *registryv1alpha1.RegistryNodeSpec) []string {
	names := make([]string, 0, len(spec.Backends))
	for i := range spec.Backends {
		names = append(names, string(spec.Backends[i].Name))
	}

	if l.Usable == nil {
		return names
	}
	return l.Usable(names)
}

func (l *Loop) serving() bool {
	if l.Serving == nil {
		return false
	}
	return l.Serving()
}

// report publishes the state, and swallows a failure to do so.
//
// Nothing in the pull path reads this status, so failing to write it must not turn into
// a node that cannot pull — which is exactly what would happen if this error travelled
// up and aborted the pass.
func (l *Loop) report(ctx context.Context, state status.State) {
	if l.Publisher == nil {
		return
	}
	if err := l.Publisher.Publish(ctx, state); err != nil {
		l.Log.Warn("cannot publish the agent status", "error", err.Error())
	}
}
