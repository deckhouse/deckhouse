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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/useridentity"
)

var (
	testNow       = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	testLock      = time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	testCreatedAt = time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
)

const (
	totpSecretA = "otpauth://totp/Dex:jane@example.com?algorithm=SHA1&digits=6&issuer=Dex&period=30&secret=AAAAAAAAAAAAAAAA"
	totpSecretB = "otpauth://totp/Dex:jane@example.com?algorithm=SHA1&digits=6&issuer=Dex&period=30&secret=BBBBBBBBBBBBBBBB"
)

// harness is a Migrator over a fake API server. api sees whole objects; the migrator's cached
// client strips Dex secrets the way the informer cache does, so a TOTP secret can only come
// through the API reader.
type harness struct {
	m   *Migrator
	api client.WithWatch
}

func newHarness(t *testing.T, funcs *interceptor.Funcs, objs ...client.Object) *harness {
	t.Helper()
	builder := fake.NewClientBuilder().
		WithScheme(runtime.NewScheme()).
		WithRESTMapper(newTestMapper()).
		WithIndex(controller.Object(controller.PasswordGVK), controller.PasswordPairField, controller.PasswordPair).
		WithObjects(objs...)
	if funcs != nil {
		builder = builder.WithInterceptorFuncs(*funcs)
	}
	api := builder.Build()
	cached := interceptor.NewClient(api, interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if err := c.Get(ctx, key, obj, opts...); err != nil {
				return err
			}
			stripInPlace(obj)
			return nil
		},
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if err := c.List(ctx, list, opts...); err != nil {
				return err
			}
			if u, ok := list.(*unstructured.UnstructuredList); ok {
				for i := range u.Items {
					stripInPlace(&u.Items[i])
				}
			}
			return nil
		},
	})
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_pending"})
	m := newMigrator(cached, api, logr.Discard(), func() time.Time { return testNow }, gauge, newTotalCounter())
	return &harness{m: m, api: api}
}

func stripInPlace(obj client.Object) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return
	}
	stripped, err := controller.StripDexSecrets(u)
	if err != nil {
		return
	}
	if s, ok := stripped.(*unstructured.Unstructured); ok {
		u.Object = s.Object
	}
}

func newTestMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "dex.coreos.com", Version: "v1"}})
	for _, gvk := range []schema.GroupVersionKind{
		controller.PasswordGVK,
		controller.OfflineSessionsGVK,
		controller.RefreshTokenGVK,
		controller.UserIdentityGVK,
	} {
		m.Add(gvk, meta.RESTScopeNamespace)
	}
	m.Add(controller.UserGVK, meta.RESTScopeRoot)
	return m
}

func (h *harness) reconcile(t *testing.T, name string) {
	t.Helper()
	if _, err := h.m.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
		t.Fatalf("Reconcile %s: %v", name, err)
	}
}

func (h *harness) identity(t *testing.T, name string) *useridentity.Identity {
	t.Helper()
	obj := controller.Object(controller.UserIdentityGVK)
	if err := h.api.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, obj); err != nil {
		t.Fatalf("get useridentity %s: %v", name, err)
	}
	return useridentity.FromUnstructured(obj)
}

func (h *harness) identityExists(t *testing.T, name string) bool {
	t.Helper()
	obj := controller.Object(controller.UserIdentityGVK)
	err := h.api.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, obj)
	return err == nil
}

func (h *harness) record(t *testing.T, name string) useridentity.Record {
	t.Helper()
	rec, ok, err := h.identity(t, name).Record()
	if err != nil || !ok {
		t.Fatalf("record of %s = (%v, %v)", name, ok, err)
	}
	return rec
}

type sessionOpts struct {
	uid           string
	email         string
	totp          string
	totpConfirmed bool
	lockedUntil   *time.Time
	adminLock     bool
	refresh       map[string]string // client -> refresh token ID
	refreshAt     time.Time
}

func sessionObj(userID, connID string, o sessionOpts) *unstructured.Unstructured {
	return sessionNamed(useridentity.Name(userID, connID), userID, connID, o)
}

func sessionNamed(name, userID, connID string, o sessionOpts) *unstructured.Unstructured {
	uid := o.uid
	if uid == "" {
		uid = "session-uid-" + name
	}
	obj := map[string]any{
		"apiVersion": controller.OfflineSessionsGVK.GroupVersion().String(),
		"kind":       controller.OfflineSessionsGVK.Kind,
		"metadata": map[string]any{
			"name":              name,
			"namespace":         naming.DexNamespace,
			"uid":               uid,
			"creationTimestamp": testCreatedAt.Format(time.RFC3339),
		},
		"userID":        userID,
		"connID":        connID,
		"connectorData": "c2VjcmV0",
	}
	if o.email != "" {
		obj["email"] = o.email
	}
	if o.totp != "" {
		obj["totp"] = o.totp
		obj["totpConfirmed"] = o.totpConfirmed
	}
	if o.lockedUntil != nil {
		obj["lockedUntil"] = o.lockedUntil.UTC().Format(time.RFC3339)
	}
	if len(o.refresh) > 0 {
		refresh := map[string]any{}
		for clientID, id := range o.refresh {
			refresh[clientID] = map[string]any{
				"ID":        id,
				"ClientID":  clientID,
				"CreatedAt": o.refreshAt.UTC().Format(time.RFC3339Nano),
				"LastUsed":  o.refreshAt.UTC().Format(time.RFC3339Nano),
			}
		}
		obj["refresh"] = refresh
	}
	u := &unstructured.Unstructured{Object: obj}
	if o.adminLock {
		u.SetAnnotations(map[string]string{lockedByAdministratorAnnot: "true"})
	}
	return u
}

type passwordOpts struct {
	groups      []string
	lockedUntil *time.Time
	adminLock   bool
	created     time.Time
}

func passwordObj(username, email string, o passwordOpts) *unstructured.Unstructured {
	created := o.created
	if created.IsZero() {
		created = testCreatedAt
	}
	obj := map[string]any{
		"apiVersion": controller.PasswordGVK.GroupVersion().String(),
		"kind":       controller.PasswordGVK.Kind,
		"metadata": map[string]any{
			"name":              naming.ToFnvLikeDex(email),
			"namespace":         naming.DexNamespace,
			"uid":               "password-uid-" + username,
			"creationTimestamp": created.Format(time.RFC3339),
		},
		"email":    email,
		"username": username,
		"userID":   username,
		"hash":     "JDJ5JDEwJGhhc2g=",
	}
	if len(o.groups) > 0 {
		groups := make([]any, 0, len(o.groups))
		for _, g := range o.groups {
			groups = append(groups, g)
		}
		obj["groups"] = groups
	}
	if o.lockedUntil != nil {
		obj["lockedUntil"] = o.lockedUntil.UTC().Format(time.RFC3339)
	} else {
		obj["lockedUntil"] = nil
	}
	u := &unstructured.Unstructured{Object: obj}
	if o.adminLock {
		u.SetAnnotations(map[string]string{lockedByAdministratorAnnot: "true"})
	}
	return u
}

func userObj(name, email string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": controller.UserGVK.GroupVersion().String(),
		"kind":       controller.UserGVK.Kind,
		"metadata":   map[string]any{"name": name},
		"spec":       map[string]any{"email": email},
	}}
}

func (h *harness) sessionExists(t *testing.T, name string) bool {
	t.Helper()
	err := h.api.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, controller.Object(controller.OfflineSessionsGVK))
	if err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("get offlinesessions %s: %v", name, err)
	}
	return err == nil
}

func tokenObj(id, connID string, lastUsed time.Time, claims map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": controller.RefreshTokenGVK.GroupVersion().String(),
		"kind":       controller.RefreshTokenGVK.Kind,
		"metadata": map[string]any{
			"name":      id,
			"namespace": naming.DexNamespace,
		},
		"CreatedAt":   lastUsed.UTC().Format(time.RFC3339Nano),
		"LastUsed":    lastUsed.UTC().Format(time.RFC3339Nano),
		"clientID":    "kubeconfig-generator",
		"connectorID": connID,
		"token":       "secret-token",
		"claims":      claims,
	}}
}

func identityObj(t *testing.T, userID, connID string, build func(*useridentity.Identity)) *unstructured.Unstructured {
	t.Helper()
	id := useridentity.New(userID, connID)
	build(id)
	return id.Object()
}

// metricValue reads a counter or a gauge.
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

func ptr[T any](v T) *T {
	return &v
}
