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
	"errors"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func testCRD(name string, storedVersions []string) *unstructured.Unstructured {
	stored := make([]interface{}, 0, len(storedVersions))
	for _, version := range storedVersions {
		stored = append(stored, version)
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata": map[string]interface{}{
			"name": name,
		},
		"status": map[string]interface{}{
			"storedVersions": stored,
		},
	}}
}

func TestPickStoredVersion(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(),
		testCRD("machinedeployments.machine.sapcloud.io", []string{"v1alpha1"}),
		testCRD("machinedeployments.cluster.x-k8s.io", []string{"v1beta1", "v1beta2"}),
		testCRD("staticmachinetemplates.infrastructure.cluster.x-k8s.io", []string{"v1alpha1"}),
	)

	version, ok, err := pickStoredVersion(t.Context(), dyn, "machine.sapcloud.io", "machinedeployments", mcmStoredVersions)
	if err != nil {
		t.Fatalf("pick MCM version: %v", err)
	}
	if !ok || version != "v1alpha1" {
		t.Fatalf("MCM version=%q ok=%v, want v1alpha1/true", version, ok)
	}

	version, ok, err = pickStoredVersion(t.Context(), dyn, "cluster.x-k8s.io", "machinedeployments", storedVersionPreference)
	if err != nil {
		t.Fatalf("pick CAPI version: %v", err)
	}
	if !ok || version != "v1beta1" {
		t.Fatalf("CAPI version=%q ok=%v, want v1beta1/true", version, ok)
	}

	version, ok, err = pickStoredVersion(t.Context(), dyn, "cluster.x-k8s.io", "missing", storedVersionPreference)
	if err != nil {
		t.Fatalf("missing CRD returned error: %v", err)
	}
	if ok || version != "" {
		t.Fatalf("missing CRD version=%q ok=%v, want empty/false", version, ok)
	}

	version, ok, err = pickStoredVersion(t.Context(), dyn, "infrastructure.cluster.x-k8s.io", "staticmachinetemplates", []string{"v1alpha1"})
	if err != nil {
		t.Fatalf("pick StaticMachineTemplate version: %v", err)
	}
	if !ok || version != "v1alpha1" {
		t.Fatalf("StaticMachineTemplate version=%q ok=%v, want v1alpha1/true", version, ok)
	}
}

func TestCapiResourcesIncludeStaticMachineTemplates(t *testing.T) {
	for _, res := range capiResources {
		if res.Group == "infrastructure.cluster.x-k8s.io" && res.Resource == "staticmachinetemplates" {
			if len(res.versionPreference) != 1 || res.versionPreference[0] != "v1alpha1" {
				t.Fatalf("StaticMachineTemplate preference=%v, want [v1alpha1]", res.versionPreference)
			}
			return
		}
	}
	t.Fatal("StaticMachineTemplate must be kept from Helm prune during migration")
}

func TestIsConversionUnavailable(t *testing.T) {
	if !isConversionUnavailable(apierrors.NewServiceUnavailable("conversion webhook unavailable")) {
		t.Fatal("service unavailable must be treated as conversion unavailable")
	}
	if !isConversionUnavailable(errors.New("object is (re)initializing, conversion webhook is not ready")) {
		t.Fatal("conversion webhook message must be treated as conversion unavailable")
	}
	if isConversionUnavailable(errors.New("forbidden")) {
		t.Fatal("unrelated errors must not be treated as conversion unavailable")
	}
}

// helmBootstrapSecretsState is what helm leaves in d8-cloud-instance-manager: the
// three Secrets this migration takes over, the ones it does not, and one an operator
// made by hand. Everything helm owns carries app.kubernetes.io/managed-by, which helm
// stamps on every object of a release, on top of the helm_lib_module_labels pair.
const helmBootstrapSecretsState = `
---
apiVersion: v1
kind: Secret
metadata:
  name: manual-bootstrap-for-worker
  namespace: d8-cloud-instance-manager
  labels:
    heritage: deckhouse
    module: node-manager
    app.kubernetes.io/managed-by: Helm
type: Opaque
---
apiVersion: v1
kind: Secret
metadata:
  name: capi-worker-1a2b3c4d
  namespace: d8-cloud-instance-manager
  labels:
    heritage: deckhouse
    module: node-manager
    app.kubernetes.io/managed-by: Helm
  annotations:
    # _capi_bootstrap_secret.tpl:18-21 hard-codes it, so helm never produces this
    # Secret without the annotation and the hook's patch of it is a no-op.
    helm.sh/resource-policy: keep
type: Opaque
---
apiVersion: v1
kind: Secret
metadata:
  name: mcm-worker-deadbeef
  namespace: d8-cloud-instance-manager
  labels:
    heritage: deckhouse
    module: node-manager
    app.kubernetes.io/managed-by: Helm
type: Opaque
---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-registry
  namespace: d8-cloud-instance-manager
  labels:
    heritage: deckhouse
    module: node-manager
    app.kubernetes.io/managed-by: Helm
type: Opaque
---
apiVersion: v1
kind: Secret
metadata:
  name: bashible-bashbooster
  namespace: d8-cloud-instance-manager
  labels:
    heritage: deckhouse
    module: node-manager
    app.kubernetes.io/managed-by: Helm
type: Opaque
---
apiVersion: v1
kind: Secret
metadata:
  name: bashible-api-server-tls
  namespace: d8-cloud-instance-manager
  labels:
    heritage: deckhouse
    module: node-manager
    app.kubernetes.io/managed-by: Helm
type: Opaque
---
apiVersion: v1
kind: Secret
metadata:
  name: registry-packages-proxy-token
  namespace: d8-cloud-instance-manager
  labels:
    heritage: deckhouse
    module: registry-packages-proxy
    app: registry-packages-proxy
    app.kubernetes.io/managed-by: Helm
type: Opaque
---
apiVersion: v1
kind: Secret
metadata:
  name: manual-bootstrap-for-handmade
  namespace: d8-cloud-instance-manager
  labels:
    heritage: deckhouse
    module: node-manager
type: Opaque
`

var _ = Describe("node-manager :: hooks :: set_keep_policy_on_capi_resources ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)

	keepPolicy := func(name string) string {
		secret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", name)
		return secret.Field(`metadata.annotations.helm\.sh/resource-policy`).String()
	}

	Context("with the bootstrap secrets helm used to render", func() {
		BeforeEach(func() {
			f.KubeStateSet(helmBootstrapSecretsState)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		// Every Secret this migration takes over. The hook runs before helm, so missing
		// one means the release that stops rendering it prunes it and a node loses its
		// bootstrap data.
		It("stamps keep policy on every bootstrap secret helm used to render", func() {
			Expect(f).To(ExecuteSuccessfully())

			for _, name := range []string{
				"manual-bootstrap-for-worker", // manual bootstrap secret of a static group
				"capi-worker-1a2b3c4d",        // CAPI bootstrap secret, zone-hashed name
				"mcm-worker-deadbeef",         // MCM machine-class secret, zone-hashed name
			} {
				Expect(keepPolicy(name)).To(Equal("keep"), "secret %s must survive the handover", name)
			}
		})

		// d8-cloud-instance-manager is shared. Everything else in it keeps its own
		// lifecycle, the registry-packages-proxy token most of all: that is another
		// module's release, and the annotation outlives this hook.
		It("leaves the secrets this migration does not take over alone", func() {
			Expect(f).To(ExecuteSuccessfully())

			for _, name := range []string{
				"deckhouse-registry",
				"bashible-bashbooster",
				"bashible-api-server-tls",
				"registry-packages-proxy-token",
			} {
				Expect(keepPolicy(name)).To(BeEmpty(), "secret %s is not this migration's to keep", name)
			}
		})

		// The name matches a shape this hook takes over; only the managed-by label
		// tells the two apart, and helm cannot prune what it never owned.
		It("leaves a secret helm does not manage alone", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(keepPolicy("manual-bootstrap-for-handmade")).To(BeEmpty())
		})
	})
})
