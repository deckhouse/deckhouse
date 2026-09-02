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

	// Published from the d8-cluster-configuration Secret: its presence is what tells the hook that
	// Deckhouse owns the Kubernetes version. Set everywhere below except the managed-cluster context.
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

	// A published false skips the gate (requirements/check.go returns true early), so "could not
	// determine" must not become false. kubernetesVersionIsDefault defaults to false in the schema and
	// so looks like a real pin; targetKubernetesVersion has no default and marks "not resolved yet".
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

	// Managed cluster: no ClusterConfiguration, the provider owns the Kubernetes version. The gate
	// would block Deckhouse updates over a version this cluster never runs, so it must stay skipped —
	// as it was before the move, when the absent Secret left a real version that is never "Automatic".
	Context("managed cluster without ClusterConfiguration", func() {
		f := HookExecutionConfigInit(initValues, "")

		BeforeEach(func() {
			// isDefault=true on purpose: nothing is pinned in a managed cluster either, and the gate
			// must be skipped regardless.
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

			// Compared against the values, not a literal: the point is that the map still reaches the
			// requirement despite the early return, and a hard-coded copy would go stale on the next
			// Istio revision bump — which is how this broke before.
			var fromValues map[string][]string
			Expect(json.Unmarshal([]byte(f.ValuesGet("istio.internal.istioToK8sCompatibilityMap").String()), &fromValues)).To(Succeed())
			Expect(fromValues).NotTo(BeEmpty())
			Expect(compatibilityMap).To(BeEquivalentTo(fromValues))
		})
	})
})
