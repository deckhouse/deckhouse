/*
Copyright 2024 Flant JSC

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
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: discovery_preflight_check ::", func() {
	initValues := `
istio:
  internal:
    istioToK8sCompatibilityMap:
      "1.27": ["1.32", "1.33", "1.34", "1.35", "1.36"]
      "1.29": ["1.32", "1.33", "1.34", "1.35", "1.36"]
`
	f := HookExecutionConfigInit(initValues, "")

	// Present in every context below except the managed-cluster one: this key is published from the
	// d8-cluster-configuration Secret, so its presence is what tells the hook that Deckhouse owns the
	// Kubernetes version at all.
	setClusterConfiguration := func() {
		f.ValuesSetFromYaml("global.clusterConfiguration", []byte(`
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
clusterDomain: cluster.local
`))
	}

	Context("kubernetesVersionIsDefault is true", func() {
		BeforeEach(func() {
			setClusterConfiguration()
			f.ValuesSet("global.discovery.targetKubernetesVersion", "1.36")
			f.ValuesSet("global.discovery.kubernetesVersionIsDefault", true)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should publish compatibility map and automatic flag", func() {
			Expect(f).To(ExecuteSuccessfully())

			isAutomatic, exists := requirements.GetValue(isK8sVersionAutomaticKey)
			Expect(exists).To(BeTrue())
			Expect(isAutomatic).To(BeEquivalentTo(true))

			compatibilityMap, exists := requirements.GetValue(istioToK8sCompatibilityMapKey)
			Expect(exists).To(BeTrue())
			Expect(compatibilityMap).To(BeEquivalentTo(map[string][]string{
				"1.27": {"1.32", "1.33", "1.34", "1.35", "1.36"},
				"1.29": {"1.32", "1.33", "1.34", "1.35", "1.36"},
			}))
		})
	})

	Context("kubernetesVersionIsDefault is false", func() {
		BeforeEach(func() {
			setClusterConfiguration()
			f.ValuesSet("global.discovery.targetKubernetesVersion", "1.34")
			f.ValuesSet("global.discovery.kubernetesVersionIsDefault", false)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should not mark kube version as automatic", func() {
			Expect(f).To(ExecuteSuccessfully())

			isAutomatic, exists := requirements.GetValue(isK8sVersionAutomaticKey)
			Expect(exists).To(BeTrue())
			Expect(isAutomatic).To(BeEquivalentTo(false))
		})
	})

	// The gate this hook feeds is skipped whenever the published value is false
	// (requirements/check.go returns true early), so "could not determine" must not be reported as
	// false. The flag has a schema default of false, which is indistinguishable from a real pin —
	// targetKubernetesVersion has no default, so its emptiness is what marks "not resolved yet".
	Context("global discovery has not published a target version", func() {
		BeforeEach(func() {
			setClusterConfiguration()
			f.ValuesSet("global.discovery.targetKubernetesVersion", "")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should fail instead of silently disabling the compatibility gate", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("global.discovery.targetKubernetesVersion is empty"))
		})
	})

	// Managed cluster: no ClusterConfiguration, so control-plane-manager is disabled and the provider
	// owns the Kubernetes version. The gate fed by this key compares the coming Deckhouse release's
	// default version against installed Istio versions, which would block Deckhouse updates over a
	// version this cluster never runs. Reproduces the pre-move behaviour: with the
	// d8-cluster-configuration Secret absent the hook fell through to the actual cluster version,
	// which never equalled the literal "Automatic".
	Context("managed cluster without ClusterConfiguration", func() {
		f := HookExecutionConfigInit(initValues, "")

		BeforeEach(func() {
			// Deliberately set: in a managed cluster nothing is pinned anywhere, so the global hook
			// publishes the Deckhouse default with isDefault=true. The gate must still be skipped.
			f.ValuesSet("global.discovery.targetKubernetesVersion", "1.36")
			f.ValuesSet("global.discovery.kubernetesVersionIsDefault", true)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should report the version as not automatic and still publish the compatibility map", func() {
			Expect(f).To(ExecuteSuccessfully())

			isAutomatic, exists := requirements.GetValue(isK8sVersionAutomaticKey)
			Expect(exists).To(BeTrue())
			Expect(isAutomatic).To(BeEquivalentTo(false))

			compatibilityMap, exists := requirements.GetValue(istioToK8sCompatibilityMapKey)
			Expect(exists).To(BeTrue())

			// Compared against the values rather than a literal: what this context is about is that the
			// map still reaches the requirement even though the hook returns early right after saving
			// it. A hard-coded copy would only restate initValues, and go stale on the next Istio
			// revision bump — which is exactly how it broke.
			var fromValues map[string][]string
			Expect(json.Unmarshal([]byte(f.ValuesGet("istio.internal.istioToK8sCompatibilityMap").String()), &fromValues)).To(Succeed())
			Expect(fromValues).NotTo(BeEmpty())
			Expect(compatibilityMap).To(BeEquivalentTo(fromValues))
		})
	})
})
