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

package user

// The load stand of the User controller: a start of the controller, a pass for each of U users
// whose Password exists already. Run it with
//
//	go test -tags load -run TestLoad -v -timeout 60m ./internal/controller/user/
//
// The Users and Passwords are read the way the cache of controller-runtime serves them: from a
// client-go indexer with the indexes of the controller, copied on every read. Writes go to the fake
// client and are mirrored into the indexer.

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
)

type loadStore map[schema.GroupVersionKind]cache.Indexer

func newLoadStore() loadStore {
	s := loadStore{}
	for _, gvk := range []schema.GroupVersionKind{controller.PasswordGVK, controller.UserGVK} {
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

func TestLoad(t *testing.T) {
	sizes := []int{1000, 2000, 5000}
	if env := os.Getenv("LOAD_USERS"); env != "" {
		sizes = nil
		for _, f := range strings.Split(env, ",") {
			var n int
			if _, err := fmt.Sscanf(f, "%d", &n); err != nil {
				t.Fatalf("LOAD_USERS %q: %v", f, err)
			}
			sizes = append(sizes, n)
		}
	}
	t.Logf("%-6s | %-7s | %-7s %-8s %-10s %-9s %-8s", "U", "pass", "passes", "writes", "us/pass", "KiB/pass", "CPU s")
	for _, users := range sizes {
		store := newLoadStore()
		var writes atomic.Int64
		objs := make([]client.Object, 0, 2*users)
		for i := range users {
			name := fmt.Sprintf("user%d", i)
			email := name + "@example.com"
			objs = append(objs, userObj(name, email, adminPassword, ""),
				passwordObj(passwordName(email), map[string]any{
					"email": email, "username": name, "userID": name, "hash": encodePasswordHash(adminPassword),
				}, map[string]string{heritageLabel: heritageValue, moduleLabel: moduleValue, appLabel: appValue}, nil))
		}
		mirror := func(ctx context.Context, c client.Reader, obj client.Object) {
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			indexer, ok := store[u.GroupVersionKind()]
			if !ok {
				return
			}
			fresh := controller.Object(u.GroupVersionKind())
			if err := c.Get(ctx, client.ObjectKeyFromObject(u), fresh); err != nil {
				if apierrors.IsNotFound(err) {
					_ = indexer.Delete(u)
				}
				return
			}
			_ = indexer.Update(fresh)
		}
		c := newFakeClientBuilder(t, objs...).WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if u, ok := obj.(*unstructured.Unstructured); ok {
					if indexer, ok := store[u.GroupVersionKind()]; ok {
						k := key.Name
						if key.Namespace != "" {
							k = key.Namespace + "/" + key.Name
						}
						item, found, err := indexer.GetByKey(k)
						if err != nil {
							return err
						}
						if !found {
							return apierrors.NewNotFound(schema.GroupResource{Group: u.GroupVersionKind().Group, Resource: u.GetKind()}, key.Name)
						}
						item.(*unstructured.Unstructured).DeepCopyInto(u)
						return nil
					}
				}
				return cl.Get(ctx, key, obj, opts...)
			},
			List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if l, ok := list.(*unstructured.UnstructuredList); ok {
					gvk := l.GroupVersionKind()
					gvk.Kind = strings.TrimSuffix(gvk.Kind, "List")
					if indexer, ok := store[gvk]; ok {
						o := (&client.ListOptions{}).ApplyOptions(opts)
						var items []any
						var err error
						switch {
						case o.FieldSelector != nil && !o.FieldSelector.Empty():
							req := o.FieldSelector.Requirements()[0]
							items, err = indexer.ByIndex("field:"+req.Field, req.Value)
						case o.Namespace != "":
							items, err = indexer.ByIndex(cache.NamespaceIndex, o.Namespace)
						default:
							items = indexer.List()
						}
						if err != nil {
							return err
						}
						l.Items = make([]unstructured.Unstructured, 0, len(items))
						for _, item := range items {
							l.Items = append(l.Items, *item.(*unstructured.Unstructured).DeepCopy())
						}
						return nil
					}
				}
				return cl.List(ctx, list, opts...)
			},
			Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				writes.Add(1)
				err := cl.Create(ctx, obj, opts...)
				mirror(ctx, cl, obj)
				return err
			},
			Update: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				writes.Add(1)
				err := cl.Update(ctx, obj, opts...)
				mirror(ctx, cl, obj)
				return err
			},
			Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				writes.Add(1)
				err := cl.Patch(ctx, obj, patch, opts...)
				mirror(ctx, cl, obj)
				return err
			},
			Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				writes.Add(1)
				err := cl.Delete(ctx, obj, opts...)
				mirror(ctx, cl, obj)
				return err
			},
			SubResourcePatch: func(ctx context.Context, cl client.Client, sub string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
				writes.Add(1)
				err := cl.SubResource(sub).Patch(ctx, obj, patch, opts...)
				mirror(ctx, cl, obj)
				return err
			},
		}).Build()
		for _, obj := range objs {
			u := obj.(*unstructured.Unstructured)
			if err := store[u.GroupVersionKind()].Add(u.DeepCopy()); err != nil {
				t.Fatal(err)
			}
		}
		now := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)
		r := New(c, c, logr.Discard(), func() time.Time { return now })

		for pass := range 2 {
			before := writes.Load()
			runtime.GC()
			var m0, m1 runtime.MemStats
			runtime.ReadMemStats(&m0)
			start := time.Now()
			for i := range users {
				if _, err := r.Reconcile(context.Background(), userReq(fmt.Sprintf("user%d", i))); err != nil {
					t.Fatalf("Reconcile user%d: %v", i, err)
				}
			}
			elapsed := time.Since(start)
			runtime.ReadMemStats(&m1)
			t.Logf("%-6d | %-7s | %-7d %-8d %-10.1f %-9.1f %-8.2f", users, []string{"first", "restart"}[pass], users,
				writes.Load()-before, float64(elapsed.Microseconds())/float64(users),
				float64(m1.TotalAlloc-m0.TotalAlloc)/1024/float64(users), elapsed.Seconds())
		}
	}
}
