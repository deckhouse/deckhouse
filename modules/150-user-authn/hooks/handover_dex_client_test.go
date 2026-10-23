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

package hooks

import (
	"context"
	"errors"
	"testing"

	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
	"github.com/deckhouse/deckhouse/modules/150-user-authn/hooks/internal/handover"
)

var (
	grafanaOAuth2Client       = encoding.ToFnvLikeDex("dex-client-grafana@monitoring")
	grafanaLegacyOAuth2Client = encoding.ToFnvLikeDex("dex-client-grafana:monitoring")
)

func handoverObject(gvr schema.GroupVersionResource, kind, namespace, name string, labels, annotations map[string]string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(schema.GroupVersion{Group: gvr.Group, Version: gvr.Version}.String())
	obj.SetKind(kind)
	obj.SetNamespace(namespace)
	obj.SetName(name)
	obj.SetLabels(labels)
	obj.SetAnnotations(annotations)
	return obj
}

func newHandoverFakeClient(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		dexClientGVR:    "DexClientList",
		secretGVR:       "SecretList",
		oauth2ClientGVR: "OAuth2ClientList",
	}, objs...)
}

var (
	chartAnnotations = map[string]string{"meta.helm.sh/release-name": "user-authn", "meta.helm.sh/release-namespace": "d8-system"}
)

func policyOf(t *testing.T, c *dynamicfake.FakeDynamicClient, gvr schema.GroupVersionResource, namespace, name string) string {
	t.Helper()
	obj, err := c.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get %s/%s: %v", namespace, name, err)
	}
	return obj.GetAnnotations()[handover.ResourcePolicyAnnotation]
}

// addon-operator keeps the hooks of a module by name, the file they are registered from: a second
// hook from the same file replaces the first.
func TestHandoverHooksAreRegistered(t *testing.T) {
	names := map[string]int{}
	var beforeHelm bool
	for _, h := range sdk.Registry().GetModuleHooks("user-authn") {
		names[h.GetName()]++
		switch h.GetName() {
		case "150-user-authn/hooks/handover_protect.go":
			beforeHelm = h.GetConfig().OnBeforeHelm != nil
		}
	}
	for name, n := range names {
		if n > 1 {
			t.Errorf("%d hooks are registered from %s; only the last one runs", n, name)
		}
	}
	if !beforeHelm {
		t.Errorf("OnBeforeHelm hook not registered; hooks %v", names)
	}
}

func TestHandoverProtectsChartObjects(t *testing.T) {
	c := newHandoverFakeClient(
		handoverObject(dexClientGVR, "DexClient", "monitoring", "grafana", nil, nil),
		// Labels overridden through secretMetadata: still found by name.
		handoverObject(secretGVR, "Secret", "monitoring", "dex-client-grafana", map[string]string{"app": "custom"}, chartAnnotations),
		handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, grafanaOAuth2Client, map[string]string{"app": "dex-client"}, chartAnnotations),
		handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, grafanaLegacyOAuth2Client, map[string]string{"app": "dex-client"}, chartAnnotations),
		handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, kubernetesClientName, map[string]string{"app": "dex"}, chartAnnotations),
		// Not the chart's: a Secret with a matching name but written by someone else.
		handoverObject(secretGVR, "Secret", "other", "dex-client-grafana", nil, nil),
	)

	res, err := handover.Protect(context.Background(), c, handoverKinds)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if res.Stamped != 4 || res.Owned != 4 {
		t.Errorf("result = %+v, want 4 owned and stamped", res)
	}
	for _, ref := range []handover.Ref{{Namespace: "monitoring", Name: "dex-client-grafana"}} {
		if got := policyOf(t, c, secretGVR, ref.Namespace, ref.Name); got != "keep" {
			t.Errorf("Secret %s policy = %q", ref, got)
		}
	}
	for _, name := range []string{grafanaOAuth2Client, grafanaLegacyOAuth2Client, kubernetesClientName} {
		if got := policyOf(t, c, oauth2ClientGVR, dexNamespace, name); got != "keep" {
			t.Errorf("OAuth2Client %s policy = %q", name, got)
		}
	}
	if got := policyOf(t, c, secretGVR, "other", "dex-client-grafana"); got != "" {
		t.Errorf("foreign Secret policy = %q, want untouched", got)
	}

	res, err = handover.Protect(context.Background(), c, handoverKinds)
	if err != nil || res.Stamped != 0 || res.Owned != 4 {
		t.Errorf("second Protect = %+v, %v; want nothing stamped, 4 still owned", res, err)
	}
}

func TestHandoverProtectWithoutCRDs(t *testing.T) {
	c := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		dexClientGVR: "DexClientList",
	})
	if _, err := handover.Protect(context.Background(), c, handoverKinds); err != nil {
		t.Fatalf("Protect: %v", err)
	}
}

// A failed patch fails the run: the release would otherwise prune the object.
func TestHandoverProtectFailsWhenPatchFails(t *testing.T) {
	c := newHandoverFakeClient(
		handoverObject(dexClientGVR, "DexClient", "monitoring", "grafana", nil, nil),
		handoverObject(secretGVR, "Secret", "monitoring", "dex-client-grafana", nil, chartAnnotations),
	)
	c.PrependReactor("patch", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})
	res, err := handover.Protect(context.Background(), c, handoverKinds)
	if err == nil {
		t.Fatal("Protect succeeded, want the patch error")
	}
	if res.Stamped != 0 || res.Owned != 1 {
		t.Errorf("result = %+v, want 1 owned, none stamped", res)
	}
}

// Objects the controller adopted carry no release annotation: nothing is owned, and the hook stops
// asking once it has seen that.
func TestHandoverProtectReportsNothingOwnedAfterAdoption(t *testing.T) {
	c := newHandoverFakeClient(
		handoverObject(dexClientGVR, "DexClient", "monitoring", "grafana", nil, nil),
		handoverObject(secretGVR, "Secret", "monitoring", "dex-client-grafana", map[string]string{"app": "dex-client"}, nil),
		handoverObject(oauth2ClientGVR, "OAuth2Client", dexNamespace, grafanaOAuth2Client, map[string]string{"app": "dex-client"}, nil),
	)
	res, err := handover.Protect(context.Background(), c, handoverKinds)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if res != (handover.Result{}) {
		t.Errorf("result = %+v, want nothing owned", res)
	}
}
