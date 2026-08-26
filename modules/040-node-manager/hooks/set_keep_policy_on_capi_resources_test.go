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

// helmBootstrapSecretsState is the shape helm leaves behind: the module labels of
// helm_lib_module_labels, plus the app.kubernetes.io/managed-by label helm stamps
// on every object of a release — the label this hook selects on.
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
  name: bootstrapped-by-hand
  namespace: d8-cloud-instance-manager
  labels:
    heritage: deckhouse
    module: node-manager
type: Opaque
`

var _ = Describe("node-manager :: hooks :: set_keep_policy_on_capi_resources ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("with the bootstrap secrets helm still renders", func() {
		BeforeEach(func() {
			f.KubeStateSet(helmBootstrapSecretsState)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("stamps keep policy on the helm-managed bootstrap secrets", func() {
			Expect(f).To(ExecuteSuccessfully())

			secret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "manual-bootstrap-for-worker")
			Expect(secret.Field(`metadata.annotations.helm\.sh/resource-policy`).String()).To(Equal("keep"))
		})

		// The selector is the whole safety net: without it the hook would stamp keep
		// on a secret helm never owned and nothing would ever collect it.
		It("leaves a secret helm does not manage alone", func() {
			Expect(f).To(ExecuteSuccessfully())

			secret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bootstrapped-by-hand")
			Expect(secret.Field(`metadata.annotations.helm\.sh/resource-policy`).Exists()).To(BeFalse())
		})
	})
})
