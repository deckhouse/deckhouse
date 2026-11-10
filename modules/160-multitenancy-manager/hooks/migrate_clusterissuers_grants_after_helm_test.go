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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: multitenancy-manager :: hooks :: migrate_clusterissuers_grants_after_helm ::", func() {
	const initValues = `
global:
  discovery: {}
multitenancyManager:
  internal:
    admissionWebhookCert: {}
`

	f := HookExecutionConfigInit(initValues, `{}`)
	f.RegisterCRD("multitenancy.deckhouse.io", "v1alpha1", "GrantableClusterResourceDefinition", false)
	f.RegisterCRD("multitenancy.deckhouse.io", "v1alpha1", "GrantableClusterResourceReference", false)

	// The state BeforeHelm left: every ClusterIssuer grant object and the unrelated one carry keep.
	kept := strings.ReplaceAll(clusterIssuersGrantState,
		"    meta.helm.sh/release-namespace: d8-system\n",
		"    meta.helm.sh/release-namespace: d8-system\n    helm.sh/resource-policy: keep\n")

	Context("cert-manager is enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["cert-manager", "multitenancy-manager"]`))
			f.BindingContexts.Set(f.KubeStateSet(kept), f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("removes the keep policy once the release no longer lists the objects", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "clusterissuers").Field(resourcePolicyPath).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", "clusterissuers-certificate").Field(resourcePolicyPath).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", "clusterissuers-ingress").Field(resourcePolicyPath).Exists()).To(BeFalse())
		})

		It("leaves the keep policy of other objects alone", func() {
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "storageclasses").Field(resourcePolicyPath).String()).To(Equal("keep"))
		})
	})

	Context("cert-manager is disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["multitenancy-manager"]`))
			f.BindingContexts.Set(f.KubeStateSet(kept), f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("still removes the keep policy", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "clusterissuers").Field(resourcePolicyPath).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", "clusterissuers-certificate").Field(resourcePolicyPath).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", "clusterissuers-ingress").Field(resourcePolicyPath).Exists()).To(BeFalse())
		})
	})

	Context("the objects are gone", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["cert-manager", "multitenancy-manager"]`))
			f.BindingContexts.Set(f.KubeStateSet(""), f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("does nothing and does not recreate them", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "clusterissuers").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", "clusterissuers-certificate").Exists()).To(BeFalse())
		})
	})
})
