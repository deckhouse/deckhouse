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

package dexclient

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/adoption"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

const (
	testName      = "grafana"
	testNamespace = "monitoring"
	testUID       = types.UID("dexclient-uid")
)

var testRequest = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: testName}}

func TestReconcile_CreatesSecretAndClients(t *testing.T) {
	t.Parallel()

	dc := dexClient(map[string]any{
		"redirectURIs": []any{"https://grafana.example.com/callback"},
		"allowedGroups": []any{
			"admins",
		},
		"secretMetadata": map[string]any{
			"labels":      map[string]any{"team": "observability"},
			"annotations": map[string]any{"reflector/enabled": "true"},
		},
	}, map[string]string{AllowAccessToKubernetesAnnotation: "true"})
	r, c := newTestReconciler(t, dc)

	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	secret := getSecret(t, c)
	value := string(secret.Data[SecretKey])
	if len(value) != clientSecretLength {
		t.Fatalf("client secret %q, want %d characters", value, clientSecretLength)
	}
	for key, want := range map[string]string{
		LabelApp: AppValue, LabelName: NameValue, LabelHeritage: HeritageValue, LabelModule: ModuleValue, "team": "observability",
	} {
		if got := secret.Labels[key]; got != want {
			t.Errorf("secret label %s = %q, want %q", key, got, want)
		}
	}
	if secret.Annotations["reflector/enabled"] != "true" || secret.Annotations[AllowAccessToKubernetesAnnotation] != "true" {
		t.Errorf("secret annotations = %v", secret.Annotations)
	}
	if len(secret.OwnerReferences) != 1 || secret.OwnerReferences[0].UID != testUID {
		t.Errorf("secret owner references = %v, want the DexClient", secret.OwnerReferences)
	}

	for _, id := range []string{"dex-client-grafana@monitoring", "dex-client-grafana:monitoring"} {
		obj := getOAuth2Client(t, c, id)
		if got, _, _ := unstructured.NestedString(obj.Object, "secret"); got != value {
			t.Errorf("%s secret = %q, want the Secret's %q", id, got, value)
		}
		if got, _, _ := unstructured.NestedString(obj.Object, "name"); got != id {
			t.Errorf("%s name = %q", id, got)
		}
		if got, _, _ := unstructured.NestedStringSlice(obj.Object, "redirectURIs"); len(got) != 1 || got[0] != "https://grafana.example.com/callback" {
			t.Errorf("%s redirectURIs = %v", id, got)
		}
		if got, _, _ := unstructured.NestedStringSlice(obj.Object, "allowedGroups"); len(got) != 1 || got[0] != "admins" {
			t.Errorf("%s allowedGroups = %v", id, got)
		}
		if _, found := obj.Object["trustedPeers"]; found {
			t.Errorf("%s has trustedPeers, want none for an empty list", id)
		}
		if obj.GetLabels()[LabelApp] != AppValue || obj.GetLabels()[LabelModule] != ModuleValue {
			t.Errorf("%s labels = %v", id, obj.GetLabels())
		}
	}

	// A second pass changes nothing.
	before := secret.ResourceVersion
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("second Reconcile: %v", err)
	}
	if after := getSecret(t, c).ResourceVersion; after != before {
		t.Errorf("second reconcile rewrote the Secret: resourceVersion %s -> %s", before, after)
	}
}

func TestReconcile_AdoptsChartObjects(t *testing.T) {
	t.Parallel()

	helmAnnotations := map[string]string{
		"meta.helm.sh/release-name":      "user-authn",
		"meta.helm.sh/release-namespace": "d8-system",
		"helm.sh/resource-policy":        "keep",
		"werf.io/fail-mode":              "IgnoreAndContinueDeployProcess",
		"werf.io/track-termination-mode": "NonBlocking",
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dex-client-grafana",
			Namespace: testNamespace,
			Labels: map[string]string{
				LabelApp: AppValue, LabelName: NameValue, LabelHeritage: HeritageValue, LabelModule: ModuleValue,
				"app.kubernetes.io/managed-by": "Helm", "team": "observability",
			},
			Annotations: helmAnnotations,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{SecretKey: []byte("rendered-by-chart")},
	}
	clients := []client.Object{secret}
	for _, id := range []string{"dex-client-grafana@monitoring", "dex-client-grafana:monitoring"} {
		obj := oauth2Client(id, "rendered-by-chart")
		obj.SetLabels(map[string]string{LabelApp: AppValue, LabelHeritage: HeritageValue, LabelModule: ModuleValue, "app.kubernetes.io/managed-by": "Helm"})
		obj.SetAnnotations(helmAnnotations)
		obj.Object["redirectURIs"] = []any{"https://grafana.example.com/callback"}
		clients = append(clients, obj)
	}

	dc := dexClient(map[string]any{
		"redirectURIs":   []any{"https://grafana.example.com/callback"},
		"secretMetadata": map[string]any{"labels": map[string]any{"team": "observability"}},
	}, nil)
	r, c := newTestReconciler(t, append(clients, dc)...)

	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	got := getSecret(t, c)
	if v := string(got.Data[SecretKey]); v != "rendered-by-chart" {
		t.Errorf("client secret = %q, want the value the chart rendered", v)
	}
	if got.Labels[adoption.ManagedByLabel] != adoption.ManagedByValue {
		t.Errorf("secret is not marked as written by the controller: %v", got.Labels)
	}
	for key := range helmAnnotations {
		if _, found := got.Annotations[key]; found {
			t.Errorf("secret keeps %s", key)
		}
	}
	if got.Labels["team"] != "observability" {
		t.Errorf("secret lost a secretMetadata label: %v", got.Labels)
	}
	if len(got.OwnerReferences) != 1 || got.OwnerReferences[0].UID != testUID {
		t.Errorf("owner references = %v", got.OwnerReferences)
	}

	for _, id := range []string{"dex-client-grafana@monitoring", "dex-client-grafana:monitoring"} {
		obj := getOAuth2Client(t, c, id)
		if v, _, _ := unstructured.NestedString(obj.Object, "secret"); v != "rendered-by-chart" {
			t.Errorf("%s secret = %q", id, v)
		}
		if obj.GetLabels()[adoption.ManagedByLabel] != adoption.ManagedByValue {
			t.Errorf("%s is not marked as written by the controller: %v", id, obj.GetLabels())
		}
		if a := obj.GetAnnotations(); len(a) != 1 || !adoption.HasManagedMetadata(obj) {
			t.Errorf("%s annotations = %v, want only the managed-metadata marker", id, a)
		}
	}
}

// The chart let secretMetadata override the module labels, so such a Secret is outside the
// label-scoped cache. Its value must still win over a generated one, and reach the clients.
func TestReconcile_AdoptsChartSecretOutsideTheCache(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dex-client-grafana", Namespace: testNamespace,
			Labels:      map[string]string{LabelApp: "custom", LabelName: NameValue},
			Annotations: map[string]string{"meta.helm.sh/release-name": "user-authn"},
		},
		Data: map[string][]byte{SecretKey: []byte("rendered-by-chart")},
	}
	dc := dexClient(map[string]any{"secretMetadata": map[string]any{"labels": map[string]any{"app": "custom"}}}, nil)
	r, c := newTestReconciler(t, dc, secret)
	r.client = interceptor.NewClient(c.(client.WithWatch), interceptor.Funcs{
		Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if s, ok := obj.(*corev1.Secret); ok {
				if err := cl.Get(ctx, key, s, opts...); err != nil {
					return err
				}
				if !isCredentialsSecret(s) {
					return apierrors.NewNotFound(corev1.Resource("secrets"), key.Name)
				}
				return nil
			}
			return cl.Get(ctx, key, obj, opts...)
		},
	})
	r.writer.Client = r.client

	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	got := getSecret(t, c)
	if v := string(got.Data[SecretKey]); v != "rendered-by-chart" {
		t.Errorf("client secret = %q, want the chart's", v)
	}
	if got.Labels[LabelApp] != AppValue {
		t.Errorf("app = %q, want the module label back", got.Labels[LabelApp])
	}
	for _, id := range []string{"dex-client-grafana@monitoring", "dex-client-grafana:monitoring"} {
		obj := getOAuth2Client(t, c, id)
		if v, _, _ := unstructured.NestedString(obj.Object, "secret"); v != "rendered-by-chart" {
			t.Errorf("%s secret = %q, want the chart's", id, v)
		}
	}
}

func TestReconcile_RecoversSecretFromOAuth2Client(t *testing.T) {
	t.Parallel()

	legacy := oauth2Client("dex-client-grafana:monitoring", "from-legacy")
	legacy.SetLabels(moduleLabels(AppValue))
	r, c := newTestReconciler(t, dexClient(map[string]any{}, nil), legacy)

	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if v := string(getSecret(t, c).Data[SecretKey]); v != "from-legacy" {
		t.Errorf("client secret = %q, want the one Dex already has", v)
	}
	main := getOAuth2Client(t, c, "dex-client-grafana@monitoring")
	if v, _, _ := unstructured.NestedString(main.Object, "secret"); v != "from-legacy" {
		t.Errorf("new client secret = %q, want the recovered one", v)
	}
}

func TestReconcile_SecretIsTheSourceOfTruth(t *testing.T) {
	t.Parallel()

	dc := dexClient(map[string]any{}, nil)
	r, c := newTestReconciler(t, dc)
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	secret := getSecret(t, c)
	secret.Data[SecretKey] = []byte("rotated-by-user")
	if err := c.Update(t.Context(), secret); err != nil {
		t.Fatalf("update secret: %v", err)
	}
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	obj := getOAuth2Client(t, c, "dex-client-grafana@monitoring")
	if v, _, _ := unstructured.NestedString(obj.Object, "secret"); v != "rotated-by-user" {
		t.Errorf("client secret = %q, want the one from the Secret", v)
	}
}

func TestReconcile_RemovesDroppedMetadataKeepsForeign(t *testing.T) {
	t.Parallel()

	dc := dexClient(map[string]any{
		"secretMetadata": map[string]any{
			"labels":      map[string]any{"team": "observability", "tier": "gold"},
			"annotations": map[string]any{"note": "a"},
		},
	}, nil)
	r, c := newTestReconciler(t, dc)
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	secret := getSecret(t, c)
	secret.Labels["argocd.argoproj.io/instance"] = "apps"
	secret.Annotations["kubectl.kubernetes.io/last-applied-configuration"] = "{}"
	if err := c.Update(t.Context(), secret); err != nil {
		t.Fatalf("update secret: %v", err)
	}

	live := controller.Object(controller.DexClientGVK)
	if err := c.Get(t.Context(), testRequest.NamespacedName, live); err != nil {
		t.Fatalf("get DexClient: %v", err)
	}
	if err := unstructured.SetNestedStringMap(live.Object, map[string]string{"team": "platform"}, "spec", "secretMetadata", "labels"); err != nil {
		t.Fatal(err)
	}
	unstructured.RemoveNestedField(live.Object, "spec", "secretMetadata", "annotations")
	if err := c.Update(t.Context(), live); err != nil {
		t.Fatalf("update DexClient: %v", err)
	}

	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	got := getSecret(t, c)
	if got.Labels["team"] != "platform" {
		t.Errorf("team = %q, want platform", got.Labels["team"])
	}
	if _, found := got.Labels["tier"]; found {
		t.Error("label removed from secretMetadata is still on the Secret")
	}
	if _, found := got.Annotations["note"]; found {
		t.Error("annotation removed from secretMetadata is still on the Secret")
	}
	if got.Labels["argocd.argoproj.io/instance"] != "apps" || got.Annotations["kubectl.kubernetes.io/last-applied-configuration"] != "{}" {
		t.Errorf("foreign metadata removed: labels %v, annotations %v", got.Labels, got.Annotations)
	}
}

func TestReconcile_DroppedListIsRemovedFromClient(t *testing.T) {
	t.Parallel()

	dc := dexClient(map[string]any{"trustedPeers": []any{"other"}}, nil)
	r, c := newTestReconciler(t, dc)
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	live := controller.Object(controller.DexClientGVK)
	if err := c.Get(t.Context(), testRequest.NamespacedName, live); err != nil {
		t.Fatal(err)
	}
	unstructured.RemoveNestedField(live.Object, "spec", "trustedPeers")
	if err := c.Update(t.Context(), live); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	obj := getOAuth2Client(t, c, "dex-client-grafana@monitoring")
	if _, found := obj.Object["trustedPeers"]; found {
		t.Error("trustedPeers removed from the DexClient is still on the OAuth2Client")
	}
}

func TestReconcile_LogoutFields(t *testing.T) {
	t.Parallel()

	dc := dexClient(map[string]any{
		"postLogoutRedirectURIs": []any{"https://grafana.example.com/logged-out"},
		"backchannelLogoutURI":   "https://grafana.example.com/backchannel",
		"refreshTokenLifetime":   "Session",
	}, nil)
	r, c := newTestReconciler(t, dc)
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	for _, id := range []string{"dex-client-grafana@monitoring", "dex-client-grafana:monitoring"} {
		obj := getOAuth2Client(t, c, id)
		if got, _, _ := unstructured.NestedStringSlice(obj.Object, "postLogoutRedirectURIs"); len(got) != 1 || got[0] != "https://grafana.example.com/logged-out" {
			t.Errorf("%s postLogoutRedirectURIs = %v", id, got)
		}
		if got, _, _ := unstructured.NestedString(obj.Object, "backchannelLogoutURI"); got != "https://grafana.example.com/backchannel" {
			t.Errorf("%s backchannelLogoutURI = %q", id, got)
		}
		if got, _, _ := unstructured.NestedString(obj.Object, "refreshTokenLifetime"); got != "session" {
			t.Errorf("%s refreshTokenLifetime = %q, want the lower case Dex takes", id, got)
		}
	}

	live := controller.Object(controller.DexClientGVK)
	if err := c.Get(t.Context(), testRequest.NamespacedName, live); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"postLogoutRedirectURIs", "backchannelLogoutURI", "refreshTokenLifetime"} {
		unstructured.RemoveNestedField(live.Object, "spec", field)
	}
	if err := c.Update(t.Context(), live); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	obj := getOAuth2Client(t, c, "dex-client-grafana@monitoring")
	for _, field := range []string{"postLogoutRedirectURIs", "backchannelLogoutURI", "refreshTokenLifetime"} {
		if _, found := obj.Object[field]; found {
			t.Errorf("%s removed from the DexClient is still on the OAuth2Client", field)
		}
	}
}

func TestReconcile_ForeignSecretIsNotTakenOver(t *testing.T) {
	t.Parallel()

	foreign := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "dex-client-grafana", Namespace: testNamespace},
		Data:       map[string][]byte{"password": []byte("user-data")},
	}
	r, c := newTestReconciler(t, dexClient(map[string]any{}, nil), foreign)

	res, err := r.Reconcile(t.Context(), testRequest)
	if err != nil || res.RequeueAfter != adoption.ForeignRecheckInterval {
		t.Fatalf("Reconcile = %v, %v, want a recheck after %v", res, err, adoption.ForeignRecheckInterval)
	}
	got := getSecret(t, c)
	if string(got.Data["password"]) != "user-data" || len(got.Data) != 1 {
		t.Errorf("foreign secret modified: %v", got.Data)
	}
	obj := controller.Object(controller.OAuth2ClientGVK)
	err = c.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: OAuth2ClientName("dex-client-grafana@monitoring")}, obj)
	if !apierrors.IsNotFound(err) {
		t.Errorf("OAuth2Client created for a DexClient whose Secret is foreign: %v", err)
	}

	// Its removal brings no event: the recheck creates the objects.
	if err := c.Delete(t.Context(), foreign); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile after the foreign Secret is gone: %v", err)
	}
	if got := getSecret(t, c); len(got.Data[SecretKey]) == 0 {
		t.Error("Secret not created once the name is free")
	}
	getOAuth2Client(t, c, "dex-client-grafana@monitoring")
}

func TestReconcile_CleansUpAfterDeletedDexClient(t *testing.T) {
	t.Parallel()

	var objs []client.Object
	for _, id := range []string{"dex-client-grafana@monitoring", "dex-client-grafana:monitoring"} {
		obj := oauth2Client(id, "s")
		obj.SetLabels(moduleLabels(AppValue))
		objs = append(objs, obj)
	}
	fromChart := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dex-client-grafana", Namespace: testNamespace,
			Labels:      map[string]string{LabelApp: AppValue, LabelName: NameValue},
			Annotations: map[string]string{"meta.helm.sh/release-name": "user-authn"},
		},
	}
	labelledByUser := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dex-client-copy", Namespace: testNamespace,
			Labels: map[string]string{LabelApp: AppValue, LabelName: NameValue},
		},
	}
	r, c := newTestReconciler(t, append(objs, fromChart, labelledByUser)...)

	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	for _, id := range []string{"dex-client-grafana@monitoring", "dex-client-grafana:monitoring"} {
		obj := controller.Object(controller.OAuth2ClientGVK)
		err := c.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: OAuth2ClientName(id)}, obj)
		if !apierrors.IsNotFound(err) {
			t.Errorf("%s left after its DexClient is gone: %v", id, err)
		}
	}
	if err := c.Get(t.Context(), client.ObjectKeyFromObject(fromChart), &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Errorf("chart secret left after its DexClient is gone: %v", err)
	}

	copyReq := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "copy"}}
	if _, err := r.Reconcile(t.Context(), copyReq); err != nil {
		t.Fatalf("Reconcile copy: %v", err)
	}
	if err := c.Get(t.Context(), client.ObjectKeyFromObject(labelledByUser), &corev1.Secret{}); err != nil {
		t.Errorf("a Secret neither owned nor rendered by the module was removed: %v", err)
	}
}

// The cache holds only the objects labelled by the controller; one the chart rendered is found
// through the API.
func TestReconcile_KeepsChartClientSecretOutsideTheCache(t *testing.T) {
	t.Parallel()

	var objs []client.Object
	for _, id := range []string{"dex-client-grafana@monitoring", "dex-client-grafana:monitoring"} {
		obj := oauth2Client(id, "rendered-by-chart")
		obj.SetLabels(moduleLabels(AppValue))
		obj.SetAnnotations(map[string]string{"meta.helm.sh/release-name": "user-authn"})
		objs = append(objs, obj)
	}
	r, c := newTestReconciler(t, append(objs, dexClient(map[string]any{}, nil))...)
	withLabelScopedCache(r, c)

	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if v := string(getSecret(t, c).Data[SecretKey]); v != "rendered-by-chart" {
		t.Errorf("client secret = %q, want the one Dex already has", v)
	}
	for _, id := range []string{"dex-client-grafana@monitoring", "dex-client-grafana:monitoring"} {
		if v, _, _ := unstructured.NestedString(getOAuth2Client(t, c, id).Object, "secret"); v != "rendered-by-chart" {
			t.Errorf("%s secret = %q, want it kept", id, v)
		}
	}
}

func TestReconcile_CleansUpChartObjectsOutsideTheCache(t *testing.T) {
	t.Parallel()

	var objs []client.Object
	for _, id := range []string{"dex-client-grafana@monitoring", "dex-client-grafana:monitoring"} {
		obj := oauth2Client(id, "s")
		obj.SetLabels(moduleLabels(AppValue))
		obj.SetAnnotations(map[string]string{"meta.helm.sh/release-name": "user-authn"})
		objs = append(objs, obj)
	}
	fromChart := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dex-client-grafana", Namespace: testNamespace,
			Labels:      map[string]string{LabelApp: AppValue, LabelName: NameValue},
			Annotations: map[string]string{"meta.helm.sh/release-name": "user-authn"},
		},
	}
	r, c := newTestReconciler(t, append(objs, fromChart)...)
	withLabelScopedCache(r, c)

	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	for _, id := range []string{"dex-client-grafana@monitoring", "dex-client-grafana:monitoring"} {
		err := c.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: OAuth2ClientName(id)}, controller.Object(controller.OAuth2ClientGVK))
		if !apierrors.IsNotFound(err) {
			t.Errorf("%s left after its DexClient is gone: %v", id, err)
		}
	}
	if err := c.Get(t.Context(), client.ObjectKeyFromObject(fromChart), &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Errorf("chart secret left after its DexClient is gone: %v", err)
	}
}

// An adopted client that lost its labels is still ours: the reconcile puts them back.
func TestReconcile_RestoresLabelsOfAdoptedClient(t *testing.T) {
	t.Parallel()

	r, c := newTestReconciler(t, dexClient(map[string]any{}, nil))
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	obj := getOAuth2Client(t, c, "dex-client-grafana@monitoring")
	obj.SetLabels(nil)
	if err := c.Update(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	if !isDexClientOAuth2Client(obj) {
		t.Error("the watch drops events of the client that lost its labels")
	}

	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if got := getOAuth2Client(t, c, "dex-client-grafana@monitoring").GetLabels(); got[LabelApp] != AppValue {
		t.Errorf("labels = %v, want the module ones back", got)
	}
}

// A DexClient deleted before the controller started has only the chart's clients left. The client
// cache delivers them on its first list, and the watch maps each back to its DexClient.
func TestMapOAuth2Client_ChartClient(t *testing.T) {
	t.Parallel()

	chart := oauth2Client("dex-client-grafana@monitoring", "s")
	chart.SetLabels(moduleLabels(AppValue))
	other := oauth2Client("kubernetes", "s")
	other.SetName("other")
	other.SetLabels(map[string]string{LabelApp: "dex"})

	if got := mapOAuth2Client(t.Context(), chart); len(got) != 1 || got[0] != testRequest {
		t.Errorf("chart client maps to %v, want %v", got, testRequest)
	}
	if got := mapOAuth2Client(t.Context(), other); len(got) != 0 {
		t.Errorf("a client of another component maps to %v, want none", got)
	}
}

func TestJoinRetryable(t *testing.T) {
	t.Parallel()

	foreign := reconcile.TerminalError(errors.New("foreign"))
	conflict := errors.New("conflict")
	isTerminal := func(err error) bool { return errors.Is(err, reconcile.TerminalError(nil)) }

	if err := joinRetryable([]error{nil, nil}); err != nil {
		t.Errorf("no errors = %v", err)
	}
	if err := joinRetryable([]error{foreign, nil}); !isTerminal(err) {
		t.Errorf("only a terminal error = %v, want terminal", err)
	}
	err := joinRetryable([]error{foreign, conflict})
	if err == nil || isTerminal(err) || !errors.Is(err, conflict) {
		t.Errorf("terminal and retryable = %v, want a retryable error", err)
	}
}

func withLabelScopedCache(r *Reconciler, c client.Client) {
	r.client = interceptor.NewClient(c.(client.WithWatch), interceptor.Funcs{
		Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if err := cl.Get(ctx, key, obj, opts...); err != nil {
				return err
			}
			_, isSecret := obj.(*corev1.Secret)
			isClient := obj.GetObjectKind().GroupVersionKind().Kind == controller.OAuth2ClientGVK.Kind
			if (isSecret || isClient) && obj.GetLabels()[adoption.ManagedByLabel] != adoption.ManagedByValue {
				return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
			}
			return nil
		},
	})
	r.writer.Client = r.client
}

func TestParseClientID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id, name, namespace string
		ok                  bool
	}{
		{id: "dex-client-grafana@monitoring", name: "grafana", namespace: "monitoring", ok: true},
		{id: "dex-client-grafana:monitoring", name: "grafana", namespace: "monitoring", ok: true},
		{id: "dex-client-a.b-c@ns-1", name: "a.b-c", namespace: "ns-1", ok: true},
		{id: "kubernetes"},
		{id: "dex-client-@ns"},
		{id: "dex-client-name@"},
		{id: "dex-client-name"},
	}
	for _, tt := range tests {
		key, ok := ParseClientID(tt.id)
		name, namespace := key.Name, key.Namespace
		if ok != tt.ok || name != tt.name || namespace != tt.namespace {
			t.Errorf("ParseClientID(%q) = %q, %q, %v; want %q, %q, %v", tt.id, name, namespace, ok, tt.name, tt.namespace, tt.ok)
		}
	}
}

func TestOAuth2ClientNameMatchesChart(t *testing.T) {
	t.Parallel()

	// The names the chart rendered for the test DexClient (encoding.ToFnvLikeDex of the IDs), so
	// that the controller adopts them instead of creating duplicates.
	if got := OAuth2ClientName("dex-client-inspect-probe@default"); got != "mrsxqlldnruwk3tufvuw443qmvrxillqojxwezkamrswmylvnr2mx4u44scceizf" {
		t.Errorf("name = %s", got)
	}
	if got := OAuth2ClientName("dex-client-inspect-probe:default"); got != "mrsxqlldnruwk3tufvuw443qmvrxillqojxwezj2mrswmylvnr2mx4u44scceizf" {
		t.Errorf("legacy name = %s", got)
	}
}

func TestMapOAuth2Client(t *testing.T) {
	t.Parallel()

	obj := oauth2Client("dex-client-grafana:monitoring", "s")
	obj.SetLabels(moduleLabels(AppValue))
	got := mapOAuth2Client(t.Context(), obj)
	if len(got) != 1 || got[0] != testRequest {
		t.Errorf("mapOAuth2Client = %v, want %v", got, testRequest)
	}

	kubernetes := oauth2Client("kubernetes", "s")
	kubernetes.SetLabels(moduleLabels("dex"))
	if got := mapOAuth2Client(t.Context(), kubernetes); got != nil {
		t.Errorf("mapOAuth2Client(kubernetes) = %v, want none", got)
	}
}

func newTestReconciler(t *testing.T, objs ...client.Object) (*Reconciler, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "deckhouse.io", Version: "v1"}, {Group: "dex.coreos.com", Version: "v1"}})
	mapper.Add(controller.DexClientGVK, meta.RESTScopeNamespace)
	mapper.Add(controller.OAuth2ClientGVK, meta.RESTScopeNamespace)
	mapper.Add(corev1.SchemeGroupVersion.WithKind("Secret"), meta.RESTScopeNamespace)

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(mapper).
		WithObjects(objs...).
		Build()
	return New(c, c, nil, logr.Discard()), c
}

func dexClient(spec map[string]any, annotations map[string]string) *unstructured.Unstructured {
	u := controller.Object(controller.DexClientGVK)
	u.SetName(testName)
	u.SetNamespace(testNamespace)
	u.SetUID(testUID)
	u.SetAnnotations(annotations)
	u.Object["spec"] = spec
	return u
}

func oauth2Client(id, secret string) *unstructured.Unstructured {
	u := controller.Object(controller.OAuth2ClientGVK)
	u.SetName(OAuth2ClientName(id))
	u.SetNamespace(naming.DexNamespace)
	u.Object["id"] = id
	u.Object["name"] = id
	u.Object["secret"] = secret
	return u
}

func getSecret(t *testing.T, c client.Client) *corev1.Secret {
	t.Helper()
	secret := &corev1.Secret{}
	if err := c.Get(t.Context(), types.NamespacedName{Namespace: testNamespace, Name: "dex-client-grafana"}, secret); err != nil {
		t.Fatalf("get secret: %v", err)
	}
	return secret
}

func getOAuth2Client(t *testing.T, c client.Client, id string) *unstructured.Unstructured {
	t.Helper()
	obj := controller.Object(controller.OAuth2ClientGVK)
	if err := c.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: OAuth2ClientName(id)}, obj); err != nil {
		t.Fatalf("get OAuth2Client %s: %v", id, err)
	}
	return obj
}
