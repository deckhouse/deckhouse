//go:build load

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

package useraccount

// The load stand of the UserAccount controller. Run it with
//
//	go test -tags load -run TestLoad -v -timeout 60m ./internal/controller/useraccount/
//
// LOAD_SIZES=10000x5 picks the sizes, LOAD_ROUNDS=6 the number of 5-minute rounds.
//
// U users, each with D devices: browsers signed in to Dex, each with a DexAuthenticator application
// bound to its session, and kubeconfig files. Every device refreshes its tokens once in 5 minutes,
// and Dex writes for it what it writes in a cluster: the AuthSession of a browser (its use by the
// refresh) and the OfflineSessions of the pair (the last use of the grant). Each write is an event
// of the controller: its map function and a pass of Reconcile. A write of a UserAccount is an event
// too, as the controller watches its own objects.
//
// The Dex objects are read the way the cache of controller-runtime serves them: from a client-go
// indexer with the indexes of the controller, copied on every read. The UserAccounts live in the fake
// client, which counts the writes the API server would get.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/api/v1alpha1"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/useridentity"
)

const loadRound = 5 * time.Minute

// dexStore keeps the Dex objects of the stand in client-go indexers, one per kind.
type dexStore map[schema.GroupVersionKind]cache.Indexer

func newDexStore() dexStore {
	s := dexStore{}
	for _, gvk := range []schema.GroupVersionKind{
		controller.PasswordGVK, controller.OfflineSessionsGVK, controller.UserIdentityGVK,
		controller.AuthSessionGVK, controller.RefreshTokenGVK, controller.UserGVK, controller.DexProviderGVK,
	} {
		indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
		for _, index := range controller.Indexes() {
			if index.Object.GetObjectKind().GroupVersionKind() != gvk {
				continue
			}
			extract := index.Extract
			indexers["field:"+index.Field] = func(obj any) ([]string, error) { return extract(obj.(client.Object)), nil }
		}
		s[gvk] = cache.NewIndexer(cache.MetaNamespaceKeyFunc, indexers)
	}
	return s
}

func (s dexStore) put(obj *unstructured.Unstructured) {
	if err := s[obj.GroupVersionKind()].Update(obj); err != nil {
		panic(err)
	}
}

func (s dexStore) get(gvk schema.GroupVersionKind, key client.ObjectKey, out *unstructured.Unstructured) (bool, error) {
	store, ok := s[gvk]
	if !ok {
		return false, nil
	}
	name := key.Name
	if key.Namespace != "" {
		name = key.Namespace + "/" + key.Name
	}
	item, found, err := store.GetByKey(name)
	if err != nil || !found {
		return found, err
	}
	item.(*unstructured.Unstructured).DeepCopyInto(out)
	return true, nil
}

func (s dexStore) list(list *unstructured.UnstructuredList, opts ...client.ListOption) (bool, error) {
	gvk := list.GroupVersionKind()
	gvk.Kind = strings.TrimSuffix(gvk.Kind, "List")
	store, ok := s[gvk]
	if !ok {
		return false, nil
	}
	o := (&client.ListOptions{}).ApplyOptions(opts)
	var items []any
	var err error
	if o.FieldSelector != nil && !o.FieldSelector.Empty() {
		reqs := o.FieldSelector.Requirements()
		items, err = store.ByIndex("field:"+reqs[0].Field, reqs[0].Value)
	} else if o.Namespace != "" {
		items, err = store.ByIndex(cache.NamespaceIndex, o.Namespace)
	} else {
		items = store.List()
	}
	if err != nil {
		return true, err
	}
	list.Items = make([]unstructured.Unstructured, 0, len(items))
	for _, item := range items {
		u := item.(*unstructured.Unstructured)
		if o.LabelSelector != nil && !o.LabelSelector.Matches(labels.Set(u.GetLabels())) {
			continue
		}
		list.Items = append(list.Items, *u.DeepCopy())
	}
	return true, nil
}

// loadUser is a user of the stand and what Dex writes for it.
type loadUser struct {
	userID   string
	pair     string
	browsers []string
	grants   []string // the grants of the browsers come first, one per browser
	created  time.Time
}

func loadObject(gvk schema.GroupVersionKind, namespace, name string, fields map[string]any) *unstructured.Unstructured {
	obj := controller.Object(gvk)
	obj.SetNamespace(namespace)
	obj.SetName(name)
	for k, v := range fields {
		obj.Object[k] = v
	}
	return obj
}

func stamp(t time.Time) string { return t.Format(time.RFC3339Nano) }

// offlineSessions is the OfflineSessions of u with every grant last used at used.
func (u *loadUser) offlineSessions(used time.Time) *unstructured.Unstructured {
	grants := map[string]any{}
	refresh := map[string]any{}
	for i, id := range u.grants {
		ref := map[string]any{"ID": id, "CreatedAt": stamp(u.created), "LastUsed": stamp(used),
			"IPAddress": "198.51.100.7", "UserAgent": "Mozilla/5.0 (X11; Linux x86_64) Firefox/140.0"}
		client := fmt.Sprintf("kubeconfig-generator-%d", i)
		if i < len(u.browsers) {
			client = "console"
			ref["SessionID"] = u.browsers[i]
		}
		ref["ClientID"] = client
		grants[id] = ref
		refresh[client] = map[string]any{"ID": id, "ClientID": client}
	}
	return loadObject(controller.OfflineSessionsGVK, naming.DexNamespace, u.pair, map[string]any{
		"userID": u.userID, "connID": naming.LocalConnectorID, "grants": grants, "refresh": refresh,
	})
}

// authSession is browser i of u last used at used.
func (u *loadUser) authSession(i int, used time.Time) *unstructured.Unstructured {
	return loadObject(controller.AuthSessionGVK, naming.DexNamespace, u.browsers[i], map[string]any{
		"userID": u.userID, "connectorID": naming.LocalConnectorID,
		"clientStates": map[string]any{"console": map[string]any{"lastActivity": stamp(used)}},
		"ipAddress":    "203.0.113.7", "userAgent": "Mozilla/5.0 (X11; Linux x86_64) Firefox/140.0",
		"createdAt": stamp(u.created), "lastActivity": stamp(used),
		"idleExpiry": stamp(used.Add(24 * time.Hour)), "absoluteExpiry": stamp(u.created.Add(168 * time.Hour)),
		"secret": "secret-session-cookie", "nonce": "secret-nonce",
	})
}

type loadStand struct {
	dex dexStore
	// accounts are the UserAccounts as the API server and the cache keep them.
	accounts cache.Indexer
	version  int64
	c        client.Client
	r        *Reconciler
	now      time.Time
	users    []*loadUser
	// requests are the writes of UserAccounts sent to the API server; writes are those that changed
	// the object. The API server stores an unchanged object as a no-op: no new resourceVersion, no
	// watch event.
	requests atomic.Int64
	writes   atomic.Int64
}

// store writes a UserAccount the way the API server answers a write: the object sent, at a new
// resourceVersion, as it comes back through JSON (whole seconds for the times). data is the body the
// client sends, built as controller-runtime builds it.
func (s *loadStand) store(obj client.Object, data func() ([]byte, error)) error {
	account, ok := obj.(*v1alpha1.UserAccount)
	if !ok {
		return fmt.Errorf("the stand writes only UserAccounts, not %T", obj)
	}
	if _, err := data(); err != nil {
		return err
	}
	s.requests.Add(1)
	raw, err := json.Marshal(account)
	if err != nil {
		return err
	}
	stored := &v1alpha1.UserAccount{}
	if err := json.Unmarshal(raw, stored); err != nil {
		return err
	}
	if item, found, _ := s.accounts.GetByKey(stored.Name); found {
		old := item.(*v1alpha1.UserAccount)
		stored.ResourceVersion = old.ResourceVersion
		if equality.Semantic.DeepEqual(old, stored) {
			account.ResourceVersion = old.ResourceVersion
			return nil
		}
	}
	s.writes.Add(1)
	s.version++
	stored.ResourceVersion = strconv.FormatInt(s.version, 10)
	account.ResourceVersion = stored.ResourceVersion
	return s.accounts.Update(stored)
}

func newLoadStand(t testing.TB, users, devices int) *loadStand {
	t.Helper()
	s := &loadStand{dex: newDexStore(), now: time.Date(2026, 11, 9, 9, 0, 0, 123456789, time.UTC)}
	s.accounts = cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	body := func(obj client.Object) func() ([]byte, error) {
		return func() ([]byte, error) { return json.Marshal(obj) }
	}
	patchBody := func(obj client.Object, patch client.Patch) func() ([]byte, error) {
		return func() ([]byte, error) { return patch.Data(obj) }
	}
	s.c = fake.NewClientBuilder().
		WithScheme(newTestScheme(t.(*testing.T))).
		WithRESTMapper(newTestMapper()).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				switch o := obj.(type) {
				case *unstructured.Unstructured:
					gvk := o.GroupVersionKind()
					found, err := s.dex.get(gvk, key, o)
					if err == nil && !found {
						return apierrors.NewNotFound(schema.GroupResource{Group: gvk.Group, Resource: gvk.Kind}, key.Name)
					}
					return err
				case *v1alpha1.UserAccount:
					item, found, err := s.accounts.GetByKey(key.Name)
					if err != nil {
						return err
					}
					if !found {
						return apierrors.NewNotFound(v1alpha1.SchemeGroupVersion.WithResource("useraccounts").GroupResource(), key.Name)
					}
					item.(*v1alpha1.UserAccount).DeepCopyInto(o)
					return nil
				}
				return fmt.Errorf("the stand does not serve %T", obj)
			},
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if u, ok := list.(*unstructured.UnstructuredList); ok {
					if served, err := s.dex.list(u, opts...); served {
						return err
					}
				}
				return fmt.Errorf("the stand does not list %T", list)
			},
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				return s.store(obj, body(obj))
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				return s.store(obj, body(obj))
			},
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				return s.store(obj, patchBody(obj, patch))
			},
			SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
				return s.store(obj, body(obj))
			},
			SubResourcePatch: func(ctx context.Context, c client.Client, sub string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
				return s.store(obj, patchBody(obj, patch))
			},
		}).
		Build()
	s.r = New(s.c, logr.Discard(), func() time.Time { return s.now })

	browsers := (devices + 1) / 2
	for i := range users {
		u := &loadUser{userID: fmt.Sprintf("user%d", i), created: s.now.Add(-time.Duration(i%3600) * time.Second)}
		u.pair = naming.OfflineTokenName(u.userID, naming.LocalConnectorID)
		email := u.userID + "@example.com"
		s.dex.put(loadObject(controller.UserGVK, "", u.userID, map[string]any{
			"spec": map[string]any{"email": email, "userID": u.userID, "password": "secret-user-password-hash"},
		}))
		s.dex.put(loadObject(controller.PasswordGVK, naming.DexNamespace, naming.ToFnvLikeDex(email), map[string]any{
			"email": email, "username": u.userID, "userID": u.userID, "hash": "secret-password-hash",
		}))
		id := useridentity.New(u.userID, naming.LocalConnectorID)
		id.SetClaims(useridentity.Claims{UserID: u.userID, Email: email})
		s.dex.put(id.Object())
		for b := range browsers {
			u.browsers = append(u.browsers, fmt.Sprintf("s%dx%d", i, b))
		}
		for g := range browsers + devices - browsers {
			u.grants = append(u.grants, fmt.Sprintf("g%dx%d", i, g))
		}
		for b := range u.browsers {
			s.dex.put(u.authSession(b, s.now))
		}
		for _, g := range u.grants {
			s.dex.put(loadObject(controller.RefreshTokenGVK, naming.DexNamespace, g, map[string]any{
				"claims": map[string]any{"userID": u.userID}, "connectorID": naming.LocalConnectorID, "token": "secret",
			}))
		}
		s.dex.put(u.offlineSessions(s.now))
		s.users = append(s.users, u)
	}
	return s
}

// handle runs an event the way the controller does: the map function, then a pass of Reconcile for
// each request, and one more pass for each write of a UserAccount (its own watch event).
func (s *loadStand) handle(t testing.TB, reqs []reconcile.Request) {
	for len(reqs) > 0 {
		req := reqs[0]
		reqs = reqs[1:]
		before := s.writes.Load()
		if _, err := s.r.Reconcile(context.Background(), req); err != nil {
			t.Fatalf("Reconcile %s: %v", req.Name, err)
		}
		if s.writes.Load() != before {
			reqs = append(reqs, req)
		}
	}
}

type loadResult struct {
	events, passes, requests, writes int64
	elapsed                          time.Duration
	allocs, bytes                    uint64
}

func measure(fn func() (events, passes int64)) loadResult {
	runtime.GC()
	var m0, m1 runtime.MemStats
	runtime.ReadMemStats(&m0)
	start := time.Now()
	events, passes := fn()
	elapsed := time.Since(start)
	runtime.ReadMemStats(&m1)
	return loadResult{events: events, passes: passes, elapsed: elapsed, allocs: m1.Mallocs - m0.Mallocs, bytes: m1.TotalAlloc - m0.TotalAlloc}
}

// resync is a start of the controller: a pass for every account, which creates the accounts the first
// time.
func (s *loadStand) resync(t testing.TB) loadResult {
	before, beforeRequests := s.writes.Load(), s.requests.Load()
	res := measure(func() (int64, int64) {
		var passes int64
		for _, u := range s.users {
			name := naming.LocalName(u.userID + "@example.com")
			w := s.writes.Load()
			s.handle(t, []reconcile.Request{{NamespacedName: types.NamespacedName{Name: name}}})
			passes += 1 + s.writes.Load() - w
		}
		return int64(len(s.users)), passes
	})
	res.writes, res.requests = s.writes.Load()-before, s.requests.Load()-beforeRequests
	return res
}

// round is 5 minutes of work: every device refreshes once, at a time of its own within the round.
func (s *loadStand) round(t testing.TB) loadResult {
	start := s.now
	before, beforeRequests := s.writes.Load(), s.requests.Load()
	res := measure(func() (int64, int64) {
		var events, passes int64
		for i, u := range s.users {
			s.now = start.Add(time.Duration(i%300) * time.Second)
			event := func(reqs []reconcile.Request) {
				w := s.writes.Load()
				s.handle(t, reqs)
				events++
				passes += int64(len(reqs)) + s.writes.Load() - w
			}
			for b := range u.browsers {
				// The refresh of the application of the browser: Dex records the use of the session,
				// then the last use of the grant.
				session := u.authSession(b, s.now)
				s.dex.put(session)
				event(s.r.mapAuthSession(context.Background(), session))
				offline := u.offlineSessions(s.now)
				s.dex.put(offline)
				event(s.r.mapOfflineSessions(context.Background(), offline))
			}
			for range len(u.grants) - len(u.browsers) {
				offline := u.offlineSessions(s.now)
				s.dex.put(offline)
				event(s.r.mapOfflineSessions(context.Background(), offline))
			}
		}
		return events, passes
	})
	s.now = start.Add(loadRound)
	res.writes, res.requests = s.writes.Load()-before, s.requests.Load()-beforeRequests
	return res
}

func TestLoad(t *testing.T) {
	type size struct{ users, devices int }
	sizes := []size{{1000, 2}, {5000, 2}, {10000, 2}, {1000, 5}, {5000, 5}, {10000, 5}, {1000, 20}}
	if env := os.Getenv("LOAD_SIZES"); env != "" {
		sizes = nil
		for _, f := range strings.Split(env, ",") {
			var sz size
			if _, err := fmt.Sscanf(f, "%dx%d", &sz.users, &sz.devices); err != nil {
				t.Fatalf("LOAD_SIZES %q: %v", f, err)
			}
			sizes = append(sizes, sz)
		}
	}
	t.Logf("%-6s %-3s | %-6s | %-7s %-7s %-8s %-8s %-8s %-8s | %-8s %-9s %-8s", "U", "D", "phase",
		"events", "passes", "req", "writes", "req/m", "writes/m", "us/pass", "KiB/pass", "CPU s")
	for _, sz := range sizes {
		s := newLoadStand(t, sz.users, sz.devices)
		row := func(phase string, r loadResult, minutes float64) {
			t.Logf("%-6d %-3d | %-6s | %-7d %-7d %-8d %-8d %-8.0f %-8.0f | %-8.1f %-9.1f %-8.2f", sz.users, sz.devices, phase,
				r.events, r.passes, r.requests, r.writes, float64(r.requests)/minutes, float64(r.writes)/minutes,
				float64(r.elapsed.Microseconds())/float64(r.passes), float64(r.bytes)/1024/float64(r.passes),
				r.elapsed.Seconds())
		}
		row("resync", s.resync(t), 1)
		rounds := 2
		if env := os.Getenv("LOAD_ROUNDS"); env != "" {
			if _, err := fmt.Sscanf(env, "%d", &rounds); err != nil {
				t.Fatalf("LOAD_ROUNDS %q: %v", env, err)
			}
		}
		var all loadResult
		for i := range rounds {
			r := s.round(t)
			row(fmt.Sprintf("round%d", i+1), r, loadRound.Minutes())
			all.events, all.passes, all.requests, all.writes = all.events+r.events, all.passes+r.passes, all.requests+r.requests, all.writes+r.writes
			all.elapsed, all.bytes = all.elapsed+r.elapsed, all.bytes+r.bytes
		}
		row("all", all, loadRound.Minutes()*float64(rounds))
		// A start of the controller over the accounts it wrote: a pass for each, nothing to write.
		row("restart", s.resync(t), 1)
	}
}
