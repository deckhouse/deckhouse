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

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// cachedDex serves the OfflineSessions and UserIdentity objects the way the cache does for a list
// without a copy: every list hands out the same maps. Other kinds go to the fake client.
type cachedDex struct {
	sessions   []*unstructured.Unstructured
	identities []*unstructured.Unstructured
}

func (d cachedDex) list(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
	items, ok := list.(*unstructured.UnstructuredList)
	if !ok {
		return c.List(ctx, list, opts...)
	}
	var objs []*unstructured.Unstructured
	switch items.GroupVersionKind().Kind {
	case controller.OfflineSessionsGVK.Kind + "List":
		objs = d.sessions
	case controller.UserIdentityGVK.Kind + "List":
		objs = d.identities
	default:
		return c.List(ctx, list, opts...)
	}
	items.Items = make([]unstructured.Unstructured, len(objs))
	for i, obj := range objs {
		items.Items[i] = *obj
	}
	return nil
}

func (d cachedDex) reconciler(t testing.TB) *Reconciler {
	t.Helper()
	r := newTestReconciler(t, time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	r.client = interceptor.NewClient(r.client.(client.WithWatch), interceptor.Funcs{List: d.list})
	return r
}

// standSession is an OfflineSessions shaped like those of a cluster: the pair, the grants of its
// devices with their browsers, and the connector data.
func standSession(connID, userID string, grants int) *unstructured.Unstructured {
	byID := map[string]any{}
	refresh := map[string]any{}
	for i := range grants {
		id := fmt.Sprintf("%s-%s-%02d", connID, userID, i)
		ref := map[string]any{
			"ID":         id,
			"ClientID":   fmt.Sprintf("dex-authenticator-app-%02d", i%5),
			"SessionID":  "sid-" + id,
			"IPAddress":  "10.0.0.1",
			"UserAgent":  "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
			"DeviceHash": strings.Repeat("d", 43),
			"CreatedAt":  "2026-11-16T10:00:00.123456789Z",
			"LastUsed":   "2026-11-16T11:00:00.123456789Z",
		}
		byID[id] = ref
		if i < 5 {
			refresh[ref["ClientID"].(string)] = runtime.DeepCopyJSONValue(ref)
		}
	}
	name := naming.ToFnvLikeDex(userID + connID)
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion":    controller.OfflineSessionsGVK.GroupVersion().String(),
		"kind":          controller.OfflineSessionsGVK.Kind,
		"metadata":      map[string]any{"name": name, "namespace": naming.DexNamespace, "resourceVersion": "1"},
		"userID":        userID,
		"connID":        connID,
		"email":         userID + "@example.com",
		"grants":        byID,
		"refresh":       refresh,
		"connectorData": strings.Repeat("Q", 1024),
	}}
}

// standIdentity is a UserIdentity shaped like those of a cluster: the claims with groups, the known
// browsers and a confirmed TOTP.
func standIdentity(connID, userID string) *unstructured.Unstructured {
	groups := make([]any, 0, 20)
	for i := range 20 {
		groups = append(groups, fmt.Sprintf("group-of-the-company-%02d", i))
	}
	devices := make([]any, 0, 5)
	for i := range 5 {
		devices = append(devices, map[string]any{"hash": fmt.Sprintf("%043d", i), "firstSeen": "2026-11-16T10:00:00Z"})
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion":  controller.UserIdentityGVK.GroupVersion().String(),
		"kind":        controller.UserIdentityGVK.Kind,
		"metadata":    map[string]any{"name": naming.ToFnvLikeDex(userID + connID), "namespace": naming.DexNamespace},
		"userID":      userID,
		"connectorID": connID,
		"claims": map[string]any{
			"userID": userID, "username": userID, "email": userID + "@example.com", "emailVerified": true, "groups": groups,
		},
		"knownDevices": devices,
		"mfaSecrets": map[string]any{"totp": map[string]any{
			"authenticatorID": "totp", "type": "TOTP", "secret": "dh-test-secret-0123456789abcdef", "confirmed": true,
			"createdAt": "2026-11-16T10:00:00Z",
		}},
		"createdAt": "2026-11-16T10:00:00Z",
		"lastLogin": "2026-11-16T11:00:00Z",
	}}
}

// standDex has the numbers of OfflineSessions and UserIdentity of a large cluster, spread over ten
// providers.
func standDex(sessions, identities, grants int) cachedDex {
	var d cachedDex
	for i := range sessions {
		d.sessions = append(d.sessions, standSession(fmt.Sprintf("provider-%d", i%10), fmt.Sprintf("user-%04d", i), grants))
	}
	for i := range identities {
		d.identities = append(d.identities, standIdentity(fmt.Sprintf("provider-%d", i%10), fmt.Sprintf("user-%04d", i)))
	}
	return d
}

func requestNames(r *Reconciler, provider string) []string {
	var names []string
	for _, req := range r.mapDexProvider(context.Background(), providerUnstructured(provider, "OIDC")) {
		names = append(names, req.Name)
	}
	slices.Sort(names)
	return names
}

// A DexProvider maps to the accounts of every pair of its connector, whether Dex wrote the pair in an
// OfflineSessions, a UserIdentity or both, and an object with a field the reconciler cannot read
// does not hide the others.
func TestMapDexProvider_MapsThePairsOfTheConnector(t *testing.T) {
	t.Parallel()

	broken := standSession("corp", "carol", 1)
	broken.Object["lockedUntil"] = "not a time"
	d := cachedDex{
		sessions: []*unstructured.Unstructured{
			standSession("corp", "ann", 2), standSession("other", "bob", 2), broken,
		},
		identities: []*unstructured.Unstructured{
			standIdentity("corp", "ann"), standIdentity("corp", "dave"), standIdentity("other", "bob"),
		},
	}
	got := requestNames(d.reconciler(t), "corp")
	want := []string{
		naming.ExternalName("corp", "ann"), naming.ExternalName("corp", "carol"), naming.ExternalName("corp", "dave"),
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("requests = %v, want %v", got, want)
	}
}

// mapDexProvider runs on every event of every DexProvider, twice for an update, over every pair of
// the cluster. It reads the cached objects in place: they come out unchanged, and the work does not
// grow with their content, only with their number.
func TestMapDexProvider_ReadsTheCachedObjectsInPlace(t *testing.T) {
	light := standDex(50, 50, 1)
	heavy := standDex(50, 50, 40)
	before := make([]map[string]any, 0, len(heavy.sessions)+len(heavy.identities))
	for _, obj := range slices.Concat(heavy.sessions, heavy.identities) {
		before = append(before, runtime.DeepCopyJSON(obj.Object))
	}

	rLight, rHeavy := light.reconciler(t), heavy.reconciler(t)
	allocsLight := testing.AllocsPerRun(20, func() { rLight.mapDexProvider(context.Background(), providerUnstructured("provider-1", "OIDC")) })
	allocsHeavy := testing.AllocsPerRun(20, func() { rHeavy.mapDexProvider(context.Background(), providerUnstructured("provider-1", "OIDC")) })
	if allocsHeavy > allocsLight+10 {
		t.Errorf("allocations grow with the content of the objects: %.0f with 1 grant a pair, %.0f with 40", allocsLight, allocsHeavy)
	}
	for i, obj := range slices.Concat(heavy.sessions, heavy.identities) {
		if !reflect.DeepEqual(obj.Object, before[i]) {
			t.Fatalf("cached object %s changed", obj.GetName())
		}
	}
}

// BenchmarkMapDexProvider is the map of one DexProvider event over the Dex objects of a large cluster:
// 1719 OfflineSessions and 1442 UserIdentity.
func BenchmarkMapDexProvider(b *testing.B) {
	r := standDex(1719, 1442, 25).reconciler(b)
	provider := providerUnstructured("provider-1", "OIDC")
	b.ReportAllocs()
	for b.Loop() {
		r.mapDexProvider(context.Background(), provider)
	}
}
