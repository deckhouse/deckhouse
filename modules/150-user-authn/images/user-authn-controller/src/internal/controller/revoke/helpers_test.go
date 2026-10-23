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
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
	"user-authn-controller/internal/useridentity"
)

var revokeTime = time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)

// calls records the writes of a test in order.
type calls struct {
	mu  sync.Mutex
	log []string
}

func (c *calls) add(call string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.log = append(c.log, call)
}

func (c *calls) list() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.log...)
}

// newTestClient returns a fake client with the indexes of the controller. A JSON Patch that does not
// apply fails with 422 Invalid, as the API server answers; the fake client returns the error of
// the patch library instead. Every write is recorded as "<verb> <Kind> <name>".
func newTestClient(t *testing.T, rec *calls, funcs interceptor.Funcs, objs ...client.Object) client.WithWatch {
	t.Helper()
	builder := fake.NewClientBuilder().
		WithScheme(runtime.NewScheme()).
		WithRESTMapper(newTestMapper()).
		WithObjects(objs...)
	for _, index := range controller.Indexes() {
		builder = builder.WithIndex(index.Object, index.Field, index.Extract)
	}
	base := builder.Build()

	recorded := interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if funcs.Create != nil {
				if err := funcs.Create(ctx, c, obj, opts...); err != nil {
					return err
				}
			} else if err := c.Create(ctx, obj, opts...); err != nil {
				return err
			}
			rec.add("create " + kind(obj) + " " + obj.GetName())
			return nil
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			var err error
			if funcs.Update != nil {
				err = funcs.Update(ctx, c, obj, opts...)
			} else {
				err = c.Update(ctx, obj, opts...)
			}
			if err == nil {
				rec.add("update " + kind(obj) + " " + obj.GetName())
			}
			return err
		},
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			err := c.Delete(ctx, obj, opts...)
			if err == nil {
				rec.add("delete " + kind(obj) + " " + obj.GetName())
			}
			return err
		},
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if funcs.Patch != nil {
				if err := funcs.Patch(ctx, c, obj, patch, opts...); err != nil {
					return err
				}
			}
			err := c.Patch(ctx, obj, patch, opts...)
			if err != nil && patch.Type() == types.JSONPatchType && !apierrors.IsNotFound(err) {
				if _, isStatus := err.(apierrors.APIStatus); !isStatus {
					return apierrors.NewGenericServerResponse(http.StatusUnprocessableEntity, "", schema.GroupResource{}, "", err.Error(), 0, false)
				}
			}
			if err == nil {
				rec.add("patch " + kind(obj) + " " + obj.GetName())
			}
			return err
		},
	}
	return interceptor.NewClient(base, recorded)
}

func newTestRevoker(c client.Client) *Revoker {
	return New(c, c, logr.Discard(), func() time.Time { return revokeTime })
}

func newTestMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "dex.coreos.com", Version: "v1"}})
	for _, gvk := range []schema.GroupVersionKind{
		controller.OfflineSessionsGVK,
		controller.RefreshTokenGVK,
		controller.UserIdentityGVK,
		controller.AuthSessionGVK,
		controller.RevocationGVK,
		controller.AuthCodeGVK,
		controller.AuthRequestGVK,
	} {
		m.Add(gvk, meta.RESTScopeNamespace)
	}
	return m
}

func kind(obj client.Object) string {
	return obj.GetObjectKind().GroupVersionKind().Kind
}

func dexObject(gvk schema.GroupVersionKind, name string, fields map[string]any) *unstructured.Unstructured {
	obj := controller.Object(gvk)
	obj.SetNamespace(naming.DexNamespace)
	obj.SetName(name)
	for k, v := range fields {
		obj.Object[k] = v
	}
	return obj
}

// tokenObj is a refresh token Dex issued to (userID, connID) for clientID at generation.
func tokenObj(name, connID, userID, clientID string, generation int64) *unstructured.Unstructured {
	fields := map[string]any{
		"clientID":    clientID,
		"connectorID": connID,
		"claims":      map[string]any{"userID": userID, "username": userID, "email": userID + "@example.com"},
	}
	if generation > 0 {
		fields["generation"] = generation
	}
	return dexObject(controller.RefreshTokenGVK, name, fields)
}

// sessionObj is a browser session of (userID, connID) with state for clients.
func sessionObj(sid, connID, userID string, generation int64, clients ...string) *unstructured.Unstructured {
	states := map[string]any{}
	for _, c := range clients {
		states[c] = map[string]any{"AuthenticatedAt": "2026-10-16T09:00:00Z"}
	}
	fields := map[string]any{"userID": userID, "connectorID": connID, "clientStates": states}
	if generation > 0 {
		fields["generation"] = generation
	}
	return dexObject(controller.AuthSessionGVK, sid, fields)
}

// signInObj is an AuthCode or an AuthRequest Dex issued to (userID, connID) for clientID at
// generation. Without userID it is an auth request nobody has signed in to yet.
func signInObj(gvk schema.GroupVersionKind, name, connID, userID string, generation int64) *unstructured.Unstructured {
	fields := map[string]any{
		"clientID":    "console",
		"redirectURI": "https://console.example.com/callback",
		"nonce":       "n-" + name,
	}
	if userID != "" {
		fields["connectorID"] = connID
		fields["claims"] = map[string]any{"userID": userID, "username": userID, "email": userID + "@example.com"}
	}
	if generation > 0 {
		fields["generation"] = generation
	}
	return dexObject(gvk, name, fields)
}

func codeObj(name, connID, userID string, generation int64) *unstructured.Unstructured {
	return signInObj(controller.AuthCodeGVK, name, connID, userID, generation)
}

func requestObj(name, connID, userID string, generation int64) *unstructured.Unstructured {
	return signInObj(controller.AuthRequestGVK, name, connID, userID, generation)
}

// offlineObj is the OfflineSessions of (userID, connID); refs maps a client to {token id, session id}.
func offlineObj(connID, userID string, refs map[string][2]string) *unstructured.Unstructured {
	refresh := map[string]any{}
	for clientID, r := range refs {
		refresh[clientID] = map[string]any{"ID": r[0], "ClientID": clientID, "SessionID": r[1]}
	}
	return dexObject(controller.OfflineSessionsGVK, naming.OfflineTokenName(userID, connID), map[string]any{
		"userID":        userID,
		"connID":        connID,
		"refresh":       refresh,
		"connectorData": "c2VjcmV0",
	})
}

// devicesObjects is a pair signed in to the console from two devices (Dex keeps the grants of both
// in grants, refresh names the last one) and to kubeconfig, with a browser session of the laptop.
func devicesObjects() []client.Object {
	offline := offlineObj(naming.LocalConnectorID, "jane", map[string][2]string{
		"console":              {"rt-phone", ""},
		"kubeconfig-generator": {"rt-kubeconfig", ""},
	})
	grants := map[string]any{}
	for id, r := range map[string][2]string{
		"rt-laptop":     {"console", "sid-laptop"},
		"rt-phone":      {"console", ""},
		"rt-kubeconfig": {"kubeconfig-generator", ""},
	} {
		grants[id] = map[string]any{"ID": id, "ClientID": r[0], "SessionID": r[1]}
	}
	offline.Object["grants"] = grants
	return []client.Object{
		identityObj(naming.LocalConnectorID, "jane", 2),
		offline,
		tokenObj("rt-laptop", naming.LocalConnectorID, "jane", "console", 2),
		tokenObj("rt-phone", naming.LocalConnectorID, "jane", "console", 2),
		tokenObj("rt-kubeconfig", naming.LocalConnectorID, "jane", "kubeconfig-generator", 2),
		sessionObj("sid-laptop", naming.LocalConnectorID, "jane", 2, "console"),
	}
}

// grantsOfObj returns the grants of the OfflineSessions of the pair.
func grantsOfObj(t *testing.T, c client.Client, connID, userID string) map[string]any {
	t.Helper()
	obj := controller.Object(controller.OfflineSessionsGVK)
	if err := c.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: naming.OfflineTokenName(userID, connID)}, obj); err != nil {
		t.Fatalf("get offlinesessions: %v", err)
	}
	grants, _, _ := unstructured.NestedMap(obj.Object, "grants")
	return grants
}

func identityObj(connID, userID string, generation uint64) *unstructured.Unstructured {
	id := useridentity.New(userID, connID)
	id.SetClaims(useridentity.Claims{UserID: userID, Email: userID + "@example.com"})
	if generation > 0 {
		id.SetGeneration(generation)
	}
	return id.Object()
}

func exists(t *testing.T, c client.Client, gvk schema.GroupVersionKind, name string) bool {
	t.Helper()
	err := c.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, controller.Object(gvk))
	if err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("get %s %s: %v", gvk.Kind, name, err)
	}
	return err == nil
}

func getRevocation(t *testing.T, c client.Client, name string) revocation.Revocation {
	t.Helper()
	obj := controller.Object(controller.RevocationGVK)
	if err := c.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, obj); err != nil {
		t.Fatalf("get revocation %s: %v", name, err)
	}
	rev, err := revocation.FromObject(obj)
	if err != nil {
		t.Fatalf("decode revocation %s: %v", name, err)
	}
	return rev
}

func listRevocations(t *testing.T, c client.Client) []revocation.Revocation {
	t.Helper()
	list := controller.List(controller.RevocationGVK)
	if err := c.List(t.Context(), list, client.InNamespace(naming.DexNamespace)); err != nil {
		t.Fatalf("list revocations: %v", err)
	}
	out := make([]revocation.Revocation, 0, len(list.Items))
	for i := range list.Items {
		rev, err := revocation.FromObject(&list.Items[i])
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, rev)
	}
	return out
}

func generationOf(t *testing.T, c client.Client, connID, userID string) uint64 {
	t.Helper()
	obj := controller.Object(controller.UserIdentityGVK)
	if err := c.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: naming.OfflineTokenName(userID, connID)}, obj); err != nil {
		t.Fatalf("get useridentity: %v", err)
	}
	g, err := useridentity.FromUnstructured(obj).Generation()
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func refsOf(t *testing.T, c client.Client, connID, userID string) map[string]any {
	t.Helper()
	obj := controller.Object(controller.OfflineSessionsGVK)
	if err := c.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: naming.OfflineTokenName(userID, connID)}, obj); err != nil {
		t.Fatalf("get offlinesessions: %v", err)
	}
	refresh, _, _ := unstructured.NestedMap(obj.Object, "refresh")
	return refresh
}

func metricValue(t *testing.T, m prometheus.Metric) float64 {
	t.Helper()
	var out dto.Metric
	if err := m.Write(&out); err != nil {
		t.Fatalf("read metric: %v", err)
	}
	switch {
	case out.Counter != nil:
		return out.GetCounter().GetValue()
	case out.Gauge != nil:
		return out.GetGauge().GetValue()
	default:
		t.Fatalf("metric is neither a counter nor a gauge: %v", &out)
		return 0
	}
}
