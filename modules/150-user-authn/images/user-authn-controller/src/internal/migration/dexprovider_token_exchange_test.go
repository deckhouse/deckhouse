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

package migration

import (
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

func newProviderTestClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	for _, gvk := range []schema.GroupVersionKind{controller.DexClientGVK, controller.DexAuthenticatorGVK} {
		scheme.AddKnownTypeWithName(gvk, &metav1.PartialObjectMetadata{})
		scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &metav1.PartialObjectMetadataList{})
	}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "deckhouse.io", Version: "v1"}, controller.DexAuthenticatorGVK.GroupVersion(), {Version: "v1"}})
	mapper.Add(controller.DexProviderGVK, meta.RESTScopeRoot)
	mapper.Add(controller.DexClientGVK, meta.RESTScopeNamespace)
	mapper.Add(controller.DexAuthenticatorGVK, meta.RESTScopeNamespace)
	mapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)
	return fake.NewClientBuilder().WithScheme(scheme).WithRESTMapper(mapper).WithObjects(objs...).Build()
}

func dexProvider(name, providerType string, tokenExchange map[string]any) *unstructured.Unstructured {
	p := controller.Object(controller.DexProviderGVK)
	p.SetName(name)
	spec := map[string]any{"type": providerType, "displayName": name}
	if tokenExchange != nil {
		spec["tokenExchange"] = tokenExchange
	}
	p.Object["spec"] = spec
	return p
}

func dexClient(name, namespace, allowAccess string) *metav1.PartialObjectMetadata {
	return peer(controller.DexClientGVK, controller.DexClientAllowAccessToKubernetesAnnotation, name, namespace, allowAccess)
}

func authenticatorPeer(name, namespace, allowAccess string) *metav1.PartialObjectMetadata {
	return peer(controller.DexAuthenticatorGVK, controller.DexAuthenticatorAllowAccessToKubernetesAnnotation, name, namespace, allowAccess)
}

func peer(gvk schema.GroupVersionKind, annotation, name, namespace, allowAccess string) *metav1.PartialObjectMetadata {
	c := &metav1.PartialObjectMetadata{}
	c.SetGroupVersionKind(gvk)
	c.SetName(name)
	c.SetNamespace(namespace)
	if allowAccess != "" {
		c.SetAnnotations(map[string]string{annotation: allowAccess})
	}
	return c
}

// deleting marks the object as being deleted; the finalizer keeps it in the fake client.
func deleting(obj *metav1.PartialObjectMetadata) *metav1.PartialObjectMetadata {
	now := metav1.Now()
	obj.SetDeletionTimestamp(&now)
	obj.SetFinalizers([]string{"test"})
	return obj
}

// tokenExchangeOf returns spec.tokenExchange.enabled of the provider, and whether it is set.
func tokenExchangeOf(t *testing.T, c client.Client, name string) (bool, bool) {
	t.Helper()
	p := controller.Object(controller.DexProviderGVK)
	if err := c.Get(t.Context(), client.ObjectKey{Name: name}, p); err != nil {
		t.Fatal(err)
	}
	enabled, set, _ := unstructured.NestedBool(p.Object, "spec", "tokenExchange", "enabled")
	return enabled, set
}

func runProviderMigration(t *testing.T, c client.Client) {
	t.Helper()
	if err := NewDexProviderTokenExchange(c, c, logr.Discard(), 0).Run(t.Context()); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func markerExists(t *testing.T, c client.Client) bool {
	t.Helper()
	err := c.Get(t.Context(), client.ObjectKey{Namespace: naming.DexNamespace, Name: DexProviderTokenExchangeMarker}, &corev1.ConfigMap{})
	return err == nil
}

func TestDexProviderTokenExchange(t *testing.T) {
	tests := []struct {
		name    string
		clients []client.Object
		// expected is the flag of the providers without one: OIDC "corp" and Gitlab "gitlab".
		expected bool
	}{
		{name: "a DexClient may reach the Kubernetes API", clients: []client.Object{dexClient("ci", "d8-user-authn", "true"), dexClient("app", "apps", "")}, expected: true},
		{name: "no DexClient may", clients: []client.Object{dexClient("app", "apps", "")}},
		{name: "the annotation says false", clients: []client.Object{dexClient("ci", "d8-user-authn", "false")}},
		{name: "the annotation is not a boolean", clients: []client.Object{dexClient("ci", "d8-user-authn", "yes")}},
		// Its oauth2-proxy never exchanges tokens, so a DexAuthenticator with access keeps no provider exchanging.
		{name: "only a DexAuthenticator may reach the Kubernetes API", clients: []client.Object{authenticatorPeer("app", "apps", "true"), dexClient("app", "apps", "")}},
		{name: "the only annotated DexClient is being deleted", clients: []client.Object{deleting(dexClient("ci", "d8-user-authn", "true"))}},
		{name: "no DexClients or DexAuthenticators at all"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := append([]client.Object{
				dexProvider("corp", "OIDC", nil),
				dexProvider("gitlab", "Gitlab", nil),
				// An administrator turned it off before the migration ran: it stays off.
				dexProvider("turned-off", "OIDC", map[string]any{"enabled": false}),
				// A type whose connector does not exchange tokens keeps no flag.
				dexProvider("ldap", "LDAP", nil),
			}, tt.clients...)
			c := newProviderTestClient(t, objs...)

			runProviderMigration(t, c)

			for _, name := range []string{"corp", "gitlab"} {
				if enabled, set := tokenExchangeOf(t, c, name); !set || enabled != tt.expected {
					t.Errorf("%s: tokenExchange.enabled = %v (set %v), want %v", name, enabled, set, tt.expected)
				}
			}
			if enabled, set := tokenExchangeOf(t, c, "turned-off"); !set || enabled {
				t.Errorf("turned-off: tokenExchange.enabled = %v (set %v), want false", enabled, set)
			}
			if _, set := tokenExchangeOf(t, c, "ldap"); set {
				t.Error("ldap got a token exchange flag")
			}
			if !markerExists(t, c) {
				t.Error("no marker after the migration")
			}
		})
	}
}

// Once the marker is there, the migration leaves the providers alone: one created later without the flag stays without
// exchange, and one an administrator turned off is not turned on again.
func TestDexProviderTokenExchangeRunsOnce(t *testing.T) {
	c := newProviderTestClient(t, dexProvider("corp", "OIDC", nil), dexClient("ci", "d8-user-authn", "true"))
	runProviderMigration(t, c)
	if enabled, _ := tokenExchangeOf(t, c, "corp"); !enabled {
		t.Fatal("corp was not turned on")
	}

	p := controller.Object(controller.DexProviderGVK)
	if err := c.Get(t.Context(), client.ObjectKey{Name: "corp"}, p); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(p.Object, false, "spec", "tokenExchange", "enabled"); err != nil {
		t.Fatal(err)
	}
	if err := c.Update(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	if err := c.Create(t.Context(), dexProvider("later", "OIDC", nil)); err != nil {
		t.Fatal(err)
	}

	runProviderMigration(t, c)

	if enabled, _ := tokenExchangeOf(t, c, "corp"); enabled {
		t.Error("the second pass turned corp on again")
	}
	if _, set := tokenExchangeOf(t, c, "later"); set {
		t.Error("the second pass set the flag of a provider created after the migration")
	}
}
