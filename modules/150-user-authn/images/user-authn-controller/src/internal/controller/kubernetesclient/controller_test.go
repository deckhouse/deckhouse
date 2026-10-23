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

package kubernetesclient

import (
	"errors"
	"slices"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/adoption"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

const testSecret = "kubernetes-secret"

func TestReconcile_CreatesClientWithAllPeers(t *testing.T) {
	t.Parallel()

	r, c := newTestReconciler(t,
		configMap(`["kubeconfig-generator","kubeconfig-generator-0","kubeconfig-cluster-at-example"]`),
		secret(testSecret),
		dexAuthenticator("zeta", "apps", "true"),
		dexAuthenticator("alpha", "apps", "true"),
		dexAuthenticator("denied", "apps", "false"),
		dexAuthenticator("typo", "apps", "yes"),
		dexClient("grafana", "monitoring", "true"),
		dexClient("argocd", "argocd", "true"),
		dexClient("plain", "monitoring", ""),
	)

	reconcileOK(t, r)

	got := getClient(t, c)
	want := []string{
		"alpha-apps-dex-authenticator", "zeta-apps-dex-authenticator",
		"dex-client-argocd@argocd", "dex-client-grafana@monitoring",
		"kubeconfig-generator", "kubeconfig-generator-0", "kubeconfig-cluster-at-example",
	}
	if peers := trustedPeers(got); !slices.Equal(peers, want) {
		t.Errorf("trustedPeers = %v, want %v", peers, want)
	}
	for field, want := range map[string]string{"id": ClientID, "name": ClientID, "secret": testSecret} {
		if v, _, _ := unstructured.NestedString(got.Object, field); v != want {
			t.Errorf("%s = %q, want %q", field, v, want)
		}
	}
	if uris, found, _ := unstructured.NestedSlice(got.Object, "redirectURIs"); !found || len(uris) != 0 {
		t.Errorf("redirectURIs = %v (found %v), want an empty list", uris, found)
	}
	if !adoption.ManagedSelector().Matches(labels.Set(got.GetLabels())) || !adoption.HasManagedMetadata(got) {
		t.Errorf("client is not marked as the controller's: labels %v, annotations %v", got.GetLabels(), got.GetAnnotations())
	}

	before := got.GetResourceVersion()
	reconcileOK(t, r)
	if after := getClient(t, c).GetResourceVersion(); after != before {
		t.Errorf("second reconcile rewrote the client: resourceVersion %s -> %s", before, after)
	}
}

// A DexClient granted access is a peer on its own: the chart rendered the client only for
// DexAuthenticators and the module configuration.
func TestReconcile_DexClientAloneCreatesClient(t *testing.T) {
	t.Parallel()

	r, c := newTestReconciler(t, configMap(`[]`), secret(testSecret), dexClient("grafana", "monitoring", "true"))
	reconcileOK(t, r)

	if peers := trustedPeers(getClient(t, c)); !slices.Equal(peers, []string{"dex-client-grafana@monitoring"}) {
		t.Errorf("trustedPeers = %v", peers)
	}
}

func TestReconcile_AdoptsChartClient(t *testing.T) {
	t.Parallel()

	chart := oauth2Client([]any{"old-peer"})
	chart.SetLabels(map[string]string{"app": "dex", "heritage": "deckhouse", "module": "user-authn", "app.kubernetes.io/managed-by": "Helm"})
	chart.SetAnnotations(map[string]string{
		"meta.helm.sh/release-name":      "user-authn",
		"meta.helm.sh/release-namespace": "d8-system",
		"helm.sh/resource-policy":        "keep",
	})
	chart.Object["public"] = true
	r, c := newTestReconciler(t, configMap(`["kubeconfig-generator"]`), secret(testSecret), chart)

	reconcileOK(t, r)

	got := getClient(t, c)
	if peers := trustedPeers(got); !slices.Equal(peers, []string{"kubeconfig-generator"}) {
		t.Errorf("trustedPeers = %v", peers)
	}
	if got.GetLabels()[adoption.ManagedByLabel] != adoption.ManagedByValue || got.GetLabels()["app"] != appValue {
		t.Errorf("labels = %v", got.GetLabels())
	}
	if a := got.GetAnnotations(); len(a) != 1 || !adoption.HasManagedMetadata(got) {
		t.Errorf("annotations = %v, want only the managed-metadata marker", a)
	}
	if _, found := got.Object["public"]; found {
		t.Error("a field the controller does not set is left on the client")
	}
}

func TestReconcile_DeletesClientWithoutPeers(t *testing.T) {
	t.Parallel()

	r, c := newTestReconciler(t, configMap(`[]`), secret(testSecret), dexAuthenticator("alpha", "apps", "true"))
	reconcileOK(t, r)
	getClient(t, c)

	if err := c.Delete(t.Context(), dexAuthenticator("alpha", "apps", "true")); err != nil {
		t.Fatal(err)
	}
	reconcileOK(t, r)

	err := c.Get(t.Context(), clientKey(), controller.Object(controller.OAuth2ClientGVK))
	if !apierrors.IsNotFound(err) {
		t.Fatalf("client after the last peer is gone: %v, want NotFound", err)
	}
}

func TestReconcile_ForeignClientIsLeftAlone(t *testing.T) {
	t.Parallel()

	foreign := oauth2Client([]any{"somebody"})
	r, c := newTestReconciler(t, configMap(`["kubeconfig-generator"]`), secret(testSecret), foreign)

	res, err := r.Reconcile(t.Context(), reconcile.Request{})
	if err != nil || res.RequeueAfter != adoption.ForeignRecheckInterval {
		t.Fatalf("Reconcile = %v, %v, want a recheck after %v", res, err, adoption.ForeignRecheckInterval)
	}
	if peers := trustedPeers(getClient(t, c)); !slices.Equal(peers, []string{"somebody"}) {
		t.Errorf("foreign client changed: trustedPeers = %v", peers)
	}

	// Nor is it deleted when the peers are gone.
	cm := configMap(`[]`)
	if err := c.Update(t.Context(), cm); err != nil {
		t.Fatal(err)
	}
	reconcileOK(t, r)
	getClient(t, c)
}

// Without the ConfigMap the chart is either an older one, which renders the client itself, or
// gone with the module: neither is a reason to touch the client.
func TestReconcile_NoConfigMapIsNoOp(t *testing.T) {
	t.Parallel()

	chart := oauth2Client([]any{"old-peer"})
	chart.SetAnnotations(map[string]string{"meta.helm.sh/release-name": "user-authn"})
	r, c := newTestReconciler(t, secret(testSecret), chart)

	reconcileOK(t, r)

	if peers := trustedPeers(getClient(t, c)); !slices.Equal(peers, []string{"old-peer"}) {
		t.Errorf("trustedPeers = %v, want the chart's untouched", peers)
	}
}

func TestReconcile_InvalidStaticPeersIsTerminal(t *testing.T) {
	t.Parallel()

	r, _ := newTestReconciler(t, configMap(`kubeconfig-generator`), secret(testSecret))
	_, err := r.Reconcile(t.Context(), reconcile.Request{})
	if !errors.Is(err, reconcile.TerminalError(nil)) {
		t.Fatalf("Reconcile = %v, want a terminal error", err)
	}
}

func TestOAuth2ClientNameMatchesDex(t *testing.T) {
	t.Parallel()

	if got := naming.ToFnvLikeDex(ClientID); got != OAuth2ClientName {
		t.Fatalf("OAuth2ClientName = %q, Dex names the client %q", OAuth2ClientName, got)
	}
}

func newTestReconciler(t *testing.T, objs ...client.Object) (*Reconciler, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	// DexAuthenticators are read by metadata only, which the fake client serves for a registered type.
	scheme.AddKnownTypeWithName(controller.DexAuthenticatorGVK, &metav1.PartialObjectMetadata{})
	scheme.AddKnownTypeWithName(controller.DexAuthenticatorGVK.GroupVersion().WithKind(controller.DexAuthenticatorGVK.Kind+"List"), &metav1.PartialObjectMetadataList{})
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "deckhouse.io", Version: "v1"}, {Group: "dex.coreos.com", Version: "v1"}})
	mapper.Add(controller.DexClientGVK, meta.RESTScopeNamespace)
	mapper.Add(controller.DexAuthenticatorGVK, meta.RESTScopeNamespace)
	mapper.Add(controller.OAuth2ClientGVK, meta.RESTScopeNamespace)

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(mapper).
		WithObjects(objs...).
		Build()
	return New(c, c, logr.Discard()), c
}

func reconcileOK(t *testing.T, r *Reconciler) {
	t.Helper()
	if _, err := r.Reconcile(t.Context(), reconcile.Request{}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func configMap(peers string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: ConfigMapName, Namespace: naming.DexNamespace},
		Data:       map[string]string{PeersKey: peers},
	}
}

func secret(value string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: SecretName, Namespace: naming.DexNamespace},
		Data:       map[string][]byte{SecretKey: []byte(value)},
	}
}

func dexAuthenticator(name, namespace, allow string) *metav1.PartialObjectMetadata {
	u := &metav1.PartialObjectMetadata{}
	u.SetGroupVersionKind(controller.DexAuthenticatorGVK)
	u.SetName(name)
	u.SetNamespace(namespace)
	u.SetAnnotations(map[string]string{controller.DexAuthenticatorAllowAccessToKubernetesAnnotation: allow})
	return u
}

func dexClient(name, namespace, allow string) *unstructured.Unstructured {
	u := controller.Object(controller.DexClientGVK)
	u.SetName(name)
	u.SetNamespace(namespace)
	if allow != "" {
		u.SetAnnotations(map[string]string{controller.DexClientAllowAccessToKubernetesAnnotation: allow})
	}
	return u
}

func oauth2Client(peers []any) *unstructured.Unstructured {
	u := controller.Object(controller.OAuth2ClientGVK)
	u.SetName(OAuth2ClientName)
	u.SetNamespace(naming.DexNamespace)
	u.Object["id"] = ClientID
	u.Object["name"] = ClientID
	u.Object["secret"] = "chart-secret"
	u.Object["trustedPeers"] = peers
	return u
}

func clientKey() types.NamespacedName {
	return types.NamespacedName{Namespace: naming.DexNamespace, Name: OAuth2ClientName}
}

func getClient(t *testing.T, c client.Client) *unstructured.Unstructured {
	t.Helper()
	obj := controller.Object(controller.OAuth2ClientGVK)
	if err := c.Get(t.Context(), clientKey(), obj); err != nil {
		t.Fatalf("get OAuth2Client: %v", err)
	}
	return obj
}

func trustedPeers(obj *unstructured.Unstructured) []string {
	peers, _, _ := unstructured.NestedStringSlice(obj.Object, "trustedPeers")
	return peers
}
