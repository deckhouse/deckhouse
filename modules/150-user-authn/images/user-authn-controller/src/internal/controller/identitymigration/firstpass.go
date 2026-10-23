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
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/useridentity"
)

const (
	// firstPassPageSize bounds one list response: an OfflineSessions object with its refresh
	// references and secrets is a few kilobytes, and the controller runs in 150Mi.
	firstPassPageSize = 500
	// firstPassPairsPerSecond paces the first pass: 10 000 pairs take a few minutes and
	// cost the API server a steady trickle instead of a burst.
	firstPassPairsPerSecond = 50

	firstPassMinBackoff = 5 * time.Second
	firstPassMaxBackoff = 5 * time.Minute
)

// cacheSyncer is the part of the manager's cache the first pass waits on.
type cacheSyncer interface {
	WaitForCacheSync(ctx context.Context) bool
}

// FirstPass enqueues every pair the old places hold. It is a leader-only runnable.
type FirstPass struct {
	reader   client.Reader
	cache    cacheSyncer
	events   chan<- event.GenericEvent
	progress *progress
	limiter  flowcontrol.RateLimiter
	pageSize int64
	log      logr.Logger
	now      func() time.Time

	minBackoff time.Duration
	maxBackoff time.Duration
}

// FirstPass returns the first pass of m, reading through m's API reader.
func (m *Migrator) FirstPass(cache cacheSyncer) *FirstPass {
	return &FirstPass{
		reader:   m.apiReader,
		cache:    cache,
		events:   m.events,
		progress: m.progress,
		limiter:  flowcontrol.NewTokenBucketRateLimiter(firstPassPairsPerSecond, firstPassPairsPerSecond),
		pageSize: firstPassPageSize,
		log:      m.log.WithName("first-pass"),
		now:      m.now,

		minBackoff: firstPassMinBackoff,
		maxBackoff: firstPassMaxBackoff,
	}
}

// NeedLeaderElection makes the pass run on the leader only, with the controller it feeds.
func (f *FirstPass) NeedLeaderElection() bool {
	return true
}

// Start lists the old places page by page and enqueues their pairs. A failed list is retried
// from the start with backoff until it succeeds or ctx ends; the pending gauge stays above zero
// meanwhile.
func (f *FirstPass) Start(ctx context.Context) error {
	f.progress.startPass()
	if !f.cache.WaitForCacheSync(ctx) {
		return nil
	}

	backoff := f.minBackoff
	for {
		started := time.Now()
		pairs, err := f.run(ctx)
		if err == nil {
			f.progress.finishPass()
			f.log.Info("Enqueued every pair of the old Dex storage places and every UserIdentity not marked yet", "pairs", pairs, "took", time.Since(started).String())
			return nil
		}
		if ctx.Err() != nil {
			return nil
		}
		f.log.Error(err, "First pass failed, retrying", "after", backoff.String())
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, f.maxBackoff)
	}
}

// run enqueues every OfflineSessions, then every Password locked now (a local user locked before
// logging in has no OfflineSessions), then every UserIdentity without the annotation: Dex creates
// one at every login, also for a pair the old places know nothing about. It returns how many pairs
// it enqueued; a pair may be counted twice.
func (f *FirstPass) run(ctx context.Context) (int, error) {
	pairs := 0
	err := f.forEachPage(ctx, controller.OfflineSessionsGVK, func(item *unstructured.Unstructured) error {
		pairs++
		return f.enqueue(ctx, item.GetName())
	})
	if err != nil {
		return pairs, err
	}

	now := f.now()
	err = f.forEachPage(ctx, controller.PasswordGVK, func(item *unstructured.Unstructured) error {
		if !passwordLocked(item, now) {
			return nil
		}
		userID, _, _ := unstructured.NestedString(item.Object, "userID")
		if userID == "" {
			return nil
		}
		pairs++
		return f.enqueue(ctx, useridentity.Name(userID, naming.LocalConnectorID))
	})
	if err != nil {
		return pairs, err
	}

	err = f.forEachPage(ctx, controller.UserIdentityGVK, func(item *unstructured.Unstructured) error {
		if useridentity.FromUnstructured(item).Migrated() {
			return nil
		}
		pairs++
		return f.enqueue(ctx, item.GetName())
	})
	return pairs, err
}

func (f *FirstPass) forEachPage(ctx context.Context, gvk schema.GroupVersionKind, fn func(*unstructured.Unstructured) error) error {
	token := ""
	for {
		list := controller.List(gvk)
		opts := []client.ListOption{client.InNamespace(naming.DexNamespace), client.Limit(f.pageSize)}
		if token != "" {
			opts = append(opts, client.Continue(token))
		}
		if err := f.reader.List(ctx, list, opts...); err != nil {
			return fmt.Errorf("list %s: %w", gvk.Kind, err)
		}
		for i := range list.Items {
			if err := fn(&list.Items[i]); err != nil {
				return err
			}
		}
		token = list.GetContinue()
		if token == "" {
			return nil
		}
	}
}

func (f *FirstPass) enqueue(ctx context.Context, name string) error {
	if err := f.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("wait to enqueue pair %s: %w", name, err)
	}
	f.progress.add(name)
	obj := controller.Object(controller.OfflineSessionsGVK)
	obj.SetNamespace(naming.DexNamespace)
	obj.SetName(name)
	select {
	case f.events <- event.GenericEvent{Object: obj}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func passwordLocked(item *unstructured.Unstructured, now time.Time) bool {
	value, _, _ := unstructured.NestedString(item.Object, "lockedUntil")
	until, err := controller.ParseRFC3339(value)
	return err == nil && until != nil && until.After(now)
}
