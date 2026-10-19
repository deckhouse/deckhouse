// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package status

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/enginetest"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1beta1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// liveSource is a source whose Report the test changes while the service runs.
type liveSource struct {
	mu     sync.Mutex
	report model.Report
}

func (s *liveSource) set(r model.Report) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.report = r
}

func (s *liveSource) Report(string) (model.Report, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.report, true
}

func (s *liveSource) Observed(string) observed.Observation { return ready }

// statusClient is a fake client with a Module whose status patches are counted; failFirst rejects
// the first one.
type statusClient struct {
	client.Client

	mu      sync.Mutex
	patches int
	patched chan struct{}
}

func newStatusClient(t *testing.T, failFirst bool) *statusClient {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	sc := &statusClient{patched: make(chan struct{}, 16)}

	rejected := !failFirst
	sc.Client = fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&v1beta1.Module{}).
		WithObjects(&v1beta1.Module{ObjectMeta: metav1.ObjectMeta{Name: moduleName}}).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourcePatch: func(ctx context.Context, cl client.Client, sub string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
				sc.mu.Lock()
				first := !rejected
				rejected = true
				sc.mu.Unlock()

				if first {
					return errors.New("conflict")
				}

				if err := cl.SubResource(sub).Patch(ctx, obj, patch, opts...); err != nil {
					return err
				}

				sc.mu.Lock()
				sc.patches++
				sc.mu.Unlock()

				sc.patched <- struct{}{}

				return nil
			},
		}).
		Build()

	return sc
}

func (c *statusClient) module(t *testing.T) *v1beta1.Module {
	t.Helper()

	m := new(v1beta1.Module)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: moduleName}, m))

	return m
}

func (c *statusClient) waitPatch(t *testing.T) {
	t.Helper()

	select {
	case <-c.patched:
	case <-time.After(10 * time.Second):
		t.Fatal("no status patch")
	}
}

func startService(t *testing.T, cl client.Client, src source) workqueue.TypedRateLimitingInterface[string] {
	t.Helper()

	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())
	t.Cleanup(queue.ShutDown)

	NewService(cl, src, log.NewNop()).Start(context.Background(), queue)

	return queue
}

// TestServiceReflectsEveryChange: each notification projects the Report as it is when handled, so
// the last change always reaches the CR.
func TestServiceReflectsEveryChange(t *testing.T) {
	cl := newStatusClient(t, false)
	src := &liveSource{report: model.Report{CurrentVersion: v1}}
	queue := startService(t, cl, src)

	queue.Add(moduleName)
	cl.waitPatch(t)
	assert.Equal(t, v1, cl.module(t).Status.CurrentVersion.Version)

	src.set(running(v2))
	queue.Add(moduleName)
	cl.waitPatch(t)

	m := cl.module(t)
	assert.Equal(t, v2, m.Status.CurrentVersion.Version)
	assert.JSONEq(t, `{"replicas":1}`, string(m.Status.LastAppliedConfiguration.Raw))
	assert.True(t, meta.IsStatusConditionTrue(m.Status.Conditions, ConditionInstalled))
}

// TestServiceRetriesAFailedPatch: a rejected patch is requeued, not dropped.
func TestServiceRetriesAFailedPatch(t *testing.T) {
	cl := newStatusClient(t, true)
	queue := startService(t, cl, &liveSource{report: running(v1)})

	queue.Add(moduleName)
	cl.waitPatch(t)

	assert.Equal(t, v1, cl.module(t).Status.CurrentVersion.Version)
}

// TestServiceSkipsAnUnchangedStatus: the resync re-projects every name; an equal status is not
// patched again.
func TestServiceSkipsAnUnchangedStatus(t *testing.T) {
	cl := newStatusClient(t, false)
	svc := NewService(cl, &liveSource{report: running(v1)}, log.NewNop())

	require.NoError(t, svc.handleEvent(context.Background(), moduleName))
	require.NoError(t, svc.handleEvent(context.Background(), moduleName))

	cl.mu.Lock()
	defer cl.mu.Unlock()
	assert.Equal(t, 1, cl.patches)
}

// replay projects reports one after the other onto one Module, as the CR carries the status from
// one reconcile to the next, and returns the Module after each.
func replay(reports []model.Report) []*v1beta1.Module {
	module := &v1beta1.Module{ObjectMeta: metav1.ObjectMeta{Name: moduleName}}

	out := make([]*v1beta1.Module, 0, len(reports))
	for _, r := range reports {
		svc := &Service{source: fakeSource{report: r, known: true, obs: ready}}
		svc.computeAndApplyConditions(moduleName, module)
		out = append(out, module.DeepCopy())
	}

	return out
}

// TestUpdateSequenceOfTheEngine replays the Reports the engine produced for an update v1 → v2:
// Installed never leaves True, and UpdateInstalled turns True only once v2's own run reported
// (G2), never on v1's conditions after the activation.
func TestUpdateSequenceOfTheEngine(t *testing.T) {
	k := enginetest.New(t, enginetest.WithGlobal())
	k.MustBoot()

	k.ApplyModule(enginetest.Module(moduleName, v1), true)
	k.WaitReport(moduleName, "v1 runs", func(r model.Report) bool {
		return enginetest.Running(r, v1) && r.Current.AppliedChecksum != ""
	})

	installed := len(k.History(moduleName))

	k.ApplyModule(enginetest.Module(moduleName, v2), true)
	k.WaitReport(moduleName, "v2 runs", func(r model.Report) bool { return enginetest.Running(r, v2) })
	k.Wait("the last report collected", func() bool {
		h := k.History(moduleName)
		return len(h) > 0 && enginetest.Running(h[len(h)-1], v2)
	})

	history := k.History(moduleName)
	modules := replay(history)

	for i, m := range modules[installed:] {
		r := history[installed+i]
		installedCond := meta.FindStatusCondition(m.Status.Conditions, ConditionInstalled)
		require.NotNil(t, installedCond, "report %d: %s", installed+i, enginetest.Dump(r))
		assert.Equal(t, metav1.ConditionTrue, installedCond.Status, "Installed flapped at %s", enginetest.Dump(r))

		update := meta.FindStatusCondition(m.Status.Conditions, ConditionUpdateInstalled)
		if r.CurrentVersion == v2 && r.Current.Version != v2 && update != nil {
			assert.NotEqual(t, metav1.ConditionTrue, update.Status,
				"UpdateInstalled True on v1's conditions after activating v2: %s", enginetest.Dump(r))
		}
	}

	stale := 0
	for _, r := range history[installed:] {
		if r.CurrentVersion == v2 && r.Current.Version != v2 {
			stale++
		}
	}

	t.Logf("reports after the activation with v1's Current section: %d of %d", stale, len(history)-installed)

	last := modules[len(modules)-1]
	assert.Equal(t, v2, last.Status.CurrentVersion.Version)
	assert.True(t, meta.IsStatusConditionTrue(last.Status.Conditions, ConditionUpdateInstalled))
}

// TestDisableSequenceOfTheEngine replays a disable: the summary is Suspended at once, Installed
// stays True until the disable succeeded and is False/Disabled afterwards (G9).
func TestDisableSequenceOfTheEngine(t *testing.T) {
	k := enginetest.New(t, enginetest.WithGlobal())
	k.MustBoot()

	k.ApplyModule(enginetest.Module(moduleName, v1), true)
	k.WaitReport(moduleName, "v1 runs", func(r model.Report) bool { return enginetest.Running(r, v1) })

	installed := len(k.History(moduleName))

	k.ApplyModule(enginetest.Module(moduleName, v1), false)
	k.Wait("the disable reported", func() bool {
		h := k.History(moduleName)
		return len(h) > 0 && enginetest.False(h[len(h)-1].Current.Installed, model.ReasonDisabled)
	})

	history := k.History(moduleName)
	modules := replay(history)

	for i, m := range modules[installed:] {
		r := history[installed+i]
		if r.Current.Decision == nil || r.Current.Decision.Enabled {
			continue
		}

		assert.Equal(t, stateSuspended, m.Status.Summary.State, "at %s", enginetest.Dump(r))

		want := metav1.ConditionTrue
		if enginetest.False(r.Current.Installed, "") {
			want = metav1.ConditionFalse
		}

		assert.Equal(t, want, meta.FindStatusCondition(m.Status.Conditions, ConditionInstalled).Status,
			"at %s", enginetest.Dump(r))
	}

	last := modules[len(modules)-1]
	assert.Equal(t, ptrFalse(), last.Status.Enabled)
}

func ptrFalse() *bool {
	f := false
	return &f
}
