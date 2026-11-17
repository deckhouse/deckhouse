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

package dexauthenticator

import (
	"reflect"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/adoption"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// referenceGrant returns the ReferenceGrant of a namespace in d8-user-authn, if there is one.
func (e *testEnv) referenceGrant(namespace string) (*unstructured.Unstructured, bool) {
	e.t.Helper()

	obj := controller.Object(ReferenceGrantGVK)
	err := e.client.Get(e.t.Context(), client.ObjectKey{Namespace: naming.DexNamespace, Name: referenceGrantName(namespace)}, obj)
	if apierrors.IsNotFound(err) {
		return nil, false
	}
	if err != nil {
		e.t.Fatal(err)
	}
	return obj, true
}

// grantedServices are the Services of d8-user-authn the grant lets the HTTPRoute objects of its
// namespace refer to; it fails the test when the grant lets anything else refer to anything.
func grantedServices(t *testing.T, grant *unstructured.Unstructured, namespace string) []string {
	t.Helper()

	from, _, _ := unstructured.NestedSlice(grant.Object, "spec", "from")
	wantFrom := []any{map[string]any{"group": gatewayGroup, "kind": "HTTPRoute", "namespace": namespace}}
	if !reflect.DeepEqual(from, wantFrom) {
		t.Errorf("from = %v, want %v", from, wantFrom)
	}
	to, _, _ := unstructured.NestedSlice(grant.Object, "spec", "to")
	var names []string
	for _, item := range to {
		target, _ := item.(map[string]any)
		if target["group"] != "" || target["kind"] != "Service" {
			t.Errorf("to = %v, want Services only", to)
		}
		name, _ := target["name"].(string)
		names = append(names, name)
	}
	return names
}

// TestReferenceGrants: every namespace with a served DexAuthenticator gets a ReferenceGrant in
// d8-user-authn from its HTTPRoute objects to the Service of the instance that serves it, and only
// such a namespace. A grant that was changed is put back, a grant of a namespace without a served
// object goes, and a grant the controller did not make is left alone.
func TestReferenceGrants(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testSettings, true,
		authenticatorObject("shop", "app", 0, "app.example.com"),
		authenticatorObject("shop", "other", 1, "other.example.com"),
		authenticatorObject("d8-monitoring", "grafana", 2, "grafana.example.com"),
		authenticatorObject("conflict", "app", 3, "app.example.com"))
	env.tables.referenceGrants = true

	stale := controller.Object(ReferenceGrantGVK)
	stale.SetNamespace(naming.DexNamespace)
	stale.SetName(referenceGrantName("gone"))
	stale.SetLabels(referenceGrantLabels("gone"))
	stale.Object["spec"] = referenceGrantSpec("gone", []string{naming.UserAuthenticatorInstance})
	foreign := controller.Object(ReferenceGrantGVK)
	foreign.SetNamespace(naming.DexNamespace)
	foreign.SetName("somebody-else")
	foreign.SetLabels(map[string]string{labelApp: referenceGrantApp, LabelNamespace: "gone"})
	foreign.Object["spec"] = referenceGrantSpec("gone", []string{naming.UserAuthenticatorInstance})
	for _, obj := range []client.Object{stale, foreign} {
		if err := env.client.Create(t.Context(), obj); err != nil {
			t.Fatal(err)
		}
	}

	env.buildTables()
	for namespace, want := range map[string]string{"shop": naming.UserAuthenticatorInstance, "d8-monitoring": naming.SystemAuthenticatorInstance} {
		grant, found := env.referenceGrant(namespace)
		if !found {
			t.Fatalf("no ReferenceGrant for %s", namespace)
		}
		if got := grantedServices(t, grant, namespace); !reflect.DeepEqual(got, []string{want}) {
			t.Errorf("%s is granted %v, want %v", namespace, got, []string{want})
		}
		if grant.GetLabels()[adoption.ManagedByLabel] != adoption.ManagedByValue || grant.GetLabels()[LabelNamespace] != namespace {
			t.Errorf("labels of the grant of %s = %v", namespace, grant.GetLabels())
		}
	}
	if _, found := env.referenceGrant("conflict"); found {
		t.Error("a namespace without a served DexAuthenticator got a ReferenceGrant")
	}
	if _, found := env.referenceGrant("gone"); found {
		t.Error("the grant of a namespace without DexAuthenticator objects stayed")
	}
	if !env.exists(controller.Object(ReferenceGrantGVK), naming.DexNamespace, "somebody-else") {
		t.Error("a ReferenceGrant without the managed-by label was removed")
	}

	// A grant that lets more in is put back.
	grant, _ := env.referenceGrant("shop")
	grant.Object["spec"] = referenceGrantSpec("shop", []string{naming.UserAuthenticatorInstance, "kube-apiserver"})
	if err := env.client.Update(t.Context(), grant); err != nil {
		t.Fatal(err)
	}
	env.buildTables()
	grant, _ = env.referenceGrant("shop")
	if got := grantedServices(t, grant, "shop"); !reflect.DeepEqual(got, []string{naming.UserAuthenticatorInstance}) {
		t.Errorf("the changed grant of shop = %v", got)
	}

	// One object of the namespace left: the grant stays; none left: the grant goes.
	for _, name := range []string{"other", "app"} {
		obj := controller.Object(controller.DexAuthenticatorGVK)
		env.exists(obj, "shop", name)
		if err := env.client.Delete(t.Context(), obj); err != nil {
			t.Fatal(err)
		}
		env.buildTables()
		_, found := env.referenceGrant("shop")
		if name == "other" && !found {
			t.Error("the grant of shop went with one of its two objects")
		}
		if name == "app" && found {
			t.Error("the grant of shop stayed without DexAuthenticator objects")
		}
	}
}

// TestReferenceGrants_Off: a cluster without ReferenceGrant objects gets none written.
func TestReferenceGrants_Off(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t, testSettings, false, authenticatorObject("shop", "app", 0, "app.example.com"))
	env.buildTables()
	if _, found := env.referenceGrant("shop"); found {
		t.Error("a ReferenceGrant was written without the ReferenceGrant kind")
	}
}

// TestReferenceGrantName: the name of the grant of the longest namespace fits.
func TestReferenceGrantName(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("n", 63)
	if name := referenceGrantName(long); len(name) > 63 || referenceGrantName(long+"x") == name {
		t.Errorf("name of the grant of a long namespace = %q", name)
	}
}
