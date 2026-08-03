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

package run

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/registry-syncer/internal/distribution"
	"github.com/deckhouse/registry-syncer/internal/fill"
	"github.com/deckhouse/registry-syncer/internal/metrics"
	"github.com/deckhouse/registry-syncer/internal/report"
)

// Leadership tells the loop whether this replica currently holds the lease.
type Leadership interface {
	IsLeader() bool
}

// Loop keeps one replica in step with the desired storage state.
type Loop struct {
	Log    *slog.Logger
	Client client.Client

	// Node this replica runs on, and the key of its status entry.
	Node string

	// Applier keeps the serving process configured.
	Applier *distribution.Applier

	// Publisher writes this replica's status entry.
	Publisher *report.Publisher

	// Leadership decides which work this replica does.
	Leadership Leadership

	// LocalAddress is where this replica's own registry answers, used for the
	// catalogue read and as the destination of a fill.
	LocalAddress string

	// ReportedAddress is where OTHER replicas reach this one, which is not the same
	// as LocalAddress: this replica talks to itself over the loopback interface,
	// while its neighbours come in over the node address.
	ReportedAddress string

	// LocalCA and LocalAuth are how to reach it.
	LocalCA                  string
	LocalUser, LocalPassword string

	// Metrics is what this replica reports about its own work. Optional: nothing about
	// filling a cache should depend on whether anyone is watching.
	Metrics *metrics.Metrics

	// desired is the last desired state that was read, so that a component on another
	// schedule — the garbage collection — can act on the same instruction without racing
	// this loop for it.
	desired atomic.Pointer[registryv1alpha1.RegistryStorageSpec]

	// Interval is how often the desired state is re-read. A poll rather than a
	// watch: the loop has to re-apply the configuration and re-report anyway, and a
	// missed event would otherwise leave the replica silently stale.
	Interval time.Duration
}

// Desired is the last desired state this loop read, or nil before the first pass.
//
// Exposed rather than re-read because the two readers must not disagree: a collection that
// rendered a configuration from a newer spec than the loop is applying would be undone by the
// loop's next pass, and the replica would flip between them.
func (l *Loop) Desired() *registryv1alpha1.RegistryStorageSpec { return l.desired.Load() }

// LocalOptions is how to talk to this replica's own registry.
func (l *Loop) LocalOptions() []remote.Option {
	registry, err := l.localRegistry()
	if err != nil {
		return nil
	}
	return registry.Options
}

// PublishCollection records the outcome of a garbage collection in this replica's status.
//
// Written whatever the outcome, because "it has not collected since Tuesday" is the useful
// fact and it is invisible if only successes are recorded.
func (l *Loop) PublishCollection(ctx context.Context, finished time.Time, failure error) error {
	if l.Publisher == nil {
		return nil
	}

	state := report.State{
		Node:            l.Node,
		Role:            Role(l.Leadership != nil && l.Leadership.IsLeader()),
		Address:         l.ReportedAddress,
		CollectedAt:     &finished,
		CollectionError: errorText(failure),
	}
	return l.Publisher.PublishCollection(ctx, state)
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// observeLeadership records whether this replica holds the lease.
//
// Per replica rather than as one cluster-wide gauge, because the interesting failures are
// disagreements — two replicas both believing they lead, or none of them doing so — and a
// single gauge could express neither.
func (l *Loop) observeLeadership(isLeader bool) {
	if l.Metrics == nil {
		return
	}

	if isLeader {
		l.Metrics.Leader.Set(1)
		return
	}
	l.Metrics.Leader.Set(0)
}

// observePass records what a pass did, from the same state that is reported to the API.
//
// Taken from the report rather than counted separately, so the metric and the status cannot
// disagree about the same pass — which is the failure mode of every parallel accounting.
func (l *Loop) observePass(action string, started time.Time, state *report.State) {
	if l.Metrics == nil {
		return
	}

	result := metrics.ResultSuccess
	if state.Error != "" {
		result = metrics.ResultFailure
	}
	l.Metrics.Passes.WithLabelValues(action, result).Inc()
	l.Metrics.FillDuration.Observe(time.Since(started).Seconds())

	if state.VerifiedDigests > 0 {
		l.Metrics.References.WithLabelValues(metrics.OutcomeWritten).Add(float64(state.VerifiedDigests))
	}
}

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
			// Never fatal. A replica that exits on an error stops serving images, which
			// is worse than a replica that keeps serving stale content and says so.
			l.Log.Error("the pass failed", "error", err.Error())
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (l *Loop) once(ctx context.Context) error {
	spec, err := l.desiredState(ctx)
	if err != nil {
		return err
	}

	// Remembered for the garbage collection, which runs on its own schedule and must act on
	// the same instruction this loop is applying rather than read its own.
	l.desired.Store(spec)
	if spec == nil {
		// No storage object: the cache is off, or the controller has not created it
		// yet. Nothing to configure and nothing to report.
		return nil
	}

	// The configuration is applied before anything else, and regardless of role.
	// This is the path a credential change takes while the Deckhouse operator is
	// down, and it must not wait behind a fill.
	if changed, err := l.Applier.Apply(spec); err != nil {
		return fmt.Errorf("applying the registry configuration: %w", err)
	} else if changed {
		l.Log.Info("the registry configuration changed and the process was restarted")
	}

	isLeader := l.Leadership.IsLeader()
	// The address is reported by the replica itself, so a follower can replicate from
	// the leader without resolving a node name and without reading Node objects.
	state := report.State{Node: l.Node, Role: Role(isLeader), Address: l.ReportedAddress}
	expected := ExpectedDigests(spec)

	leader, err := l.currentLeader(ctx)
	if err != nil {
		return err
	}

	l.observeLeadership(isLeader)

	switch action := Decide(spec, isLeader, leader); action {
	case ActionFill:
		started := time.Now()
		l.applyFill(ctx, spec, expected, &state)
		l.observePass(string(action), started, &state)
	case ActionReplicate:
		started := time.Now()
		l.applyReplicate(ctx, leader, expected, &state)
		l.observePass(string(action), started, &state)
	case ActionCountCatalogue:
		l.applyCatalogue(ctx, expected, &state)
	case ActionNone:
		// Keep whatever was reported last: this pass learned nothing new, and
		// overwriting the count with a zero would look like the storage emptied.
		if err := l.carryOverCount(ctx, &state, expected); err != nil {
			return err
		}
	}

	return l.Publisher.Publish(ctx, state)
}

func (l *Loop) applyFill(
	ctx context.Context, spec *registryv1alpha1.RegistryStorageSpec, expected int32, state *report.State,
) {
	copier, err := l.copier(spec)
	if err != nil {
		state.Error = err.Error()
		return
	}

	l.Log.Info("filling the storage from the upstream", "upstream", spec.Upstream.Host, "expected", expected)

	result, err := copier.Run(ctx)
	if err != nil {
		state.Error = err.Error()
		state.VerifiedDigests = result.Written
		return
	}

	state.VerifiedDigests = result.Written
	state.Full = result.Complete(expected)
	if len(result.Failed) > 0 {
		// A partial fill is reported as a failure with the count it did reach: the
		// cache is still useful, and the nodes fall back to the upstream for the rest.
		state.Error = fmt.Sprintf("%d of %d references could not be copied, the first being %s",
			len(result.Failed), expected, result.Failed[0])
	}

	l.Log.Info("the fill finished",
		"written", result.Written, "skipped", result.Skipped, "failed", len(result.Failed), "full", state.Full)
}

// applyReplicate copies the leader's content into this replica.
//
// The read credentials are the same on every replica, so a follower reaches the
// leader with what it already has: no separate replication identity to distribute,
// and nothing that grants more than a pull.
func (l *Loop) applyReplicate(
	ctx context.Context, leader *Leader, expected int32, state *report.State,
) {
	local, err := l.localRegistry()
	if err != nil {
		state.Error = err.Error()
		return
	}

	source := local
	source.Address = leader.Address

	l.Log.Info("replicating from the leader", "leader", leader.Node, "address", leader.Address)

	copier := &fill.Copier{
		Source:      source,
		Destination: local,
		OnProgress: func(done, total int32) {
			l.Log.Debug("replication progress", "done", done, "total", total)
		},
	}

	result, err := copier.Run(ctx)
	state.Source = leader.Node
	state.VerifiedDigests = result.Written

	if err != nil {
		state.Error = err.Error()
		return
	}

	state.Full = result.Complete(expected)
	if len(result.Failed) > 0 {
		state.Error = fmt.Sprintf("%d of %d references could not be replicated, the first being %s",
			len(result.Failed), expected, result.Failed[0])
	}

	l.Log.Info("the replication finished",
		"written", result.Written, "skipped", result.Skipped, "failed", len(result.Failed), "full", state.Full)
}

// currentLeader reads who to replicate from out of the reports.
//
// Taken from the replica entries rather than from the aggregate `leader` field: the
// entries are what each replica wrote about itself, so a stale or hand-edited
// summary cannot send a follower to copy from a replica that is not actually
// complete.
func (l *Loop) currentLeader(ctx context.Context) (*Leader, error) {
	storage := &registryv1alpha1.RegistryStorage{}
	if err := l.Client.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading the storage state: %w", err)
	}

	for i := range storage.Status.Replicas {
		replica := &storage.Status.Replicas[i]
		if replica.Role != registryv1alpha1.ReplicaRoleLeader || replica.Node == l.Node {
			continue
		}
		return &Leader{
			Node:    replica.Node,
			Address: replica.Address,
			// A replica that both claims completeness and reports a failure is not
			// trustworthy enough to copy a whole set from.
			Full: replica.Full && replica.Error == "",
		}, nil
	}
	return nil, nil
}

func (l *Loop) applyCatalogue(ctx context.Context, expected int32, state *report.State) {
	local, err := l.localRegistry()
	if err != nil {
		state.Error = err.Error()
		return
	}

	held, err := fill.CountCatalogue(ctx, local)
	if err != nil {
		state.Error = err.Error()
		return
	}

	state.VerifiedDigests = held
	state.Full = expected > 0 && held >= expected
}

// carryOverCount keeps the previously reported count when this pass did no
// accounting, so an idle pass does not look like the storage emptied.
func (l *Loop) carryOverCount(ctx context.Context, state *report.State, expected int32) error {
	storage := &registryv1alpha1.RegistryStorage{}
	if err := l.Client.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage); err != nil {
		return client.IgnoreNotFound(err)
	}

	for i := range storage.Status.Replicas {
		if storage.Status.Replicas[i].Node != l.Node {
			continue
		}
		state.VerifiedDigests = storage.Status.Replicas[i].VerifiedDigests
		state.Full = expected > 0 && state.VerifiedDigests >= expected
		return nil
	}
	return nil
}

func (l *Loop) desiredState(ctx context.Context) (*registryv1alpha1.RegistryStorageSpec, error) {
	storage := &registryv1alpha1.RegistryStorage{}
	if err := l.Client.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading the desired storage state: %w", err)
	}
	return &storage.Spec, nil
}

func (l *Loop) copier(spec *registryv1alpha1.RegistryStorageSpec) (*fill.Copier, error) {
	username, password := "", ""
	if auth := spec.Upstream.Auth; !auth.IsEmpty() {
		username, password = auth.Username, auth.Password
	}

	upstreamOptions, err := fill.RegistryOptions(spec.Upstream.CA, username, password)
	if err != nil {
		return nil, fmt.Errorf("upstream: %w", err)
	}

	local, err := l.localRegistry()
	if err != nil {
		return nil, err
	}

	return &fill.Copier{
		Source: fill.Registry{
			Address:    spec.Upstream.Host,
			Repository: trimSlashes(spec.Upstream.Path),
			Options:    upstreamOptions,
		},
		Destination: local,
		OnProgress: func(done, total int32) {
			l.Log.Debug("fill progress", "done", done, "total", total)
		},
	}, nil
}

func (l *Loop) localRegistry() (fill.Registry, error) {
	options, err := fill.RegistryOptions(l.LocalCA, l.LocalUser, l.LocalPassword)
	if err != nil {
		return fill.Registry{}, fmt.Errorf("local storage: %w", err)
	}

	return fill.Registry{
		Address:    l.LocalAddress,
		Repository: trimSlashes(constant.Path),
		Options:    options,
	}, nil
}

func trimSlashes(path string) string {
	for len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	for len(path) > 0 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	return path
}
