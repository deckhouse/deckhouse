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

package identitymigration

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/useridentity"
)

// pagedList serves List page by page, as the API server does with limit and continue; the fake
// client ignores both. It records every limit it saw.
type pagedList struct {
	mu     sync.Mutex
	limits []int64
	fails  atomic.Int32
}

func (p *pagedList) funcs() *interceptor.Funcs {
	return &interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if p.fails.Load() > 0 {
				p.fails.Add(-1)
				return apierrors.NewTimeoutError("list timed out", 1)
			}
			listOpts := &client.ListOptions{}
			listOpts.ApplyOptions(opts)
			if err := c.List(ctx, list, opts...); err != nil {
				return err
			}
			u, ok := list.(*unstructured.UnstructuredList)
			if !ok || listOpts.Limit == 0 {
				return nil
			}
			p.mu.Lock()
			p.limits = append(p.limits, listOpts.Limit)
			p.mu.Unlock()

			start := 0
			if listOpts.Continue != "" {
				var err error
				if start, err = strconv.Atoi(listOpts.Continue); err != nil {
					return fmt.Errorf("bad continue token %q", listOpts.Continue)
				}
			}
			total := len(u.Items)
			end := min(start+int(listOpts.Limit), total)
			u.Items = u.Items[start:end]
			u.SetContinue("")
			if end < total {
				u.SetContinue(strconv.Itoa(end))
			}
			return nil
		},
	}
}

func newTestFirstPass(h *harness, pageSize int64) *FirstPass {
	f := h.m.FirstPass(syncedCache{})
	f.limiter = flowcontrol.NewFakeAlwaysRateLimiter()
	f.pageSize = pageSize
	f.minBackoff = time.Millisecond
	f.maxBackoff = time.Millisecond
	return f
}

type syncedCache struct{}

func (syncedCache) WaitForCacheSync(context.Context) bool { return true }

type unsyncedCache struct{}

func (unsyncedCache) WaitForCacheSync(context.Context) bool { return false }

func drain(h *harness) []string {
	names := []string{}
	for {
		select {
		case e := <-h.m.events:
			names = append(names, e.Object.GetName())
		default:
			slices.Sort(names)
			return names
		}
	}
}

func TestFirstPass_EnqueuesEveryPairPageByPage(t *testing.T) {
	t.Parallel()

	paging := &pagedList{}
	objs := []client.Object{
		sessionObj("a", naming.LocalConnectorID, sessionOpts{}),
		sessionObj("b", "corp-ldap", sessionOpts{}),
		sessionObj("c", "corp-oidc", sessionOpts{}),
		sessionObj("d", "corp-oidc", sessionOpts{}),
		sessionObj("e", "corp-oidc", sessionOpts{}),
		// Locked before ever logging in: only a Password.
		passwordObj("locked", "locked@example.com", passwordOpts{lockedUntil: &testLock}),
		// Not locked and never logged in: nothing to copy.
		passwordObj("idle", "idle@example.com", passwordOpts{}),
		// Lock expired.
		passwordObj("expired", "expired@example.com", passwordOpts{lockedUntil: ptr(testNow.Add(-time.Minute))}),
	}
	h := newHarness(t, paging.funcs(), objs...)
	f := newTestFirstPass(h, 2)

	if err := f.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	expected := []string{
		useridentity.Name("a", naming.LocalConnectorID),
		useridentity.Name("b", "corp-ldap"),
		useridentity.Name("c", "corp-oidc"),
		useridentity.Name("d", "corp-oidc"),
		useridentity.Name("e", "corp-oidc"),
		useridentity.Name("locked", naming.LocalConnectorID),
	}
	slices.Sort(expected)
	if got := drain(h); !slices.Equal(got, expected) {
		t.Errorf("enqueued = %v, want %v", got, expected)
	}
	for _, limit := range paging.limits {
		if limit != 2 {
			t.Errorf("list limit = %d, want 2", limit)
		}
	}
	if len(paging.limits) < 4 {
		t.Errorf("lists = %d, want the sessions and the passwords in pages of 2", len(paging.limits))
	}
	if got := h.m.progress.value(); got != len(expected) {
		t.Errorf("pending after the pass = %d, want %d until the pairs are copied", got, len(expected))
	}

	for _, name := range expected {
		if _, err := h.m.Reconcile(t.Context(), reconcile.Request{NamespacedName: client.ObjectKey{Name: name}}); err != nil {
			t.Fatalf("Reconcile %s: %v", name, err)
		}
	}
	if got := h.m.progress.value(); got != 0 {
		t.Errorf("pending after copying = %d, want 0", got)
	}
	if got := metricValue(t, h.m.progress.gauge); got != 0 {
		t.Errorf("gauge = %v, want 0", got)
	}
}

func TestFirstPass_RetriesAFailedList(t *testing.T) {
	t.Parallel()

	paging := &pagedList{}
	paging.fails.Store(2)
	h := newHarness(t, paging.funcs(), sessionObj("a", "corp-ldap", sessionOpts{}))
	f := newTestFirstPass(h, 10)

	if err := f.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got := drain(h); len(got) != 1 {
		t.Errorf("enqueued = %v, want the one pair after the list recovered", got)
	}
}

func TestFirstPass_PendingWhileRunning(t *testing.T) {
	t.Parallel()

	h := newHarness(t, nil)
	f := newTestFirstPass(h, 10)
	// The manager stops before the cache syncs: the pass never gets to list anything.
	f.cache = unsyncedCache{}

	if err := f.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got := h.m.progress.value(); got != 1 {
		t.Errorf("pending = %d, want 1 for a pass that did not finish", got)
	}
}

func TestFirstPass_NeedsLeaderElection(t *testing.T) {
	t.Parallel()

	if !newTestFirstPass(newHarness(t, nil), 10).NeedLeaderElection() {
		t.Error("the first pass must run on the leader only")
	}
}
