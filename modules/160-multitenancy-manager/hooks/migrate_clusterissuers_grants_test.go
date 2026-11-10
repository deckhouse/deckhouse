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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// clusterIssuersGrantState is the ClusterIssuer grant registration as the previous release of this
// module left it, plus an unrelated definition that must stay untouched.
const clusterIssuersGrantState = `
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: clusterissuers
  annotations:
    meta.helm.sh/release-name: multitenancy-manager
    meta.helm.sh/release-namespace: d8-system
spec:
  enforcement: Managed
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: clusterissuers-certificate
  annotations:
    meta.helm.sh/release-name: multitenancy-manager
    meta.helm.sh/release-namespace: d8-system
spec:
  grantableClusterResourceName: clusterissuers
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: clusterissuers-ingress
  annotations:
    meta.helm.sh/release-name: cert-manager
    meta.helm.sh/release-namespace: d8-system
spec:
  grantableClusterResourceName: clusterissuers
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: storageclasses
  annotations:
    meta.helm.sh/release-name: multitenancy-manager
    meta.helm.sh/release-namespace: d8-system
spec:
  enforcement: Managed
`

const resourcePolicyPath = "metadata.annotations.helm\\.sh/resource-policy"

var _ = Describe("Modules :: multitenancy-manager :: hooks :: migrate_clusterissuers_grants ::", func() {
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

	policy := func(kind, name string) string {
		return f.KubernetesGlobalResource(kind, name).Field(resourcePolicyPath).String()
	}

	Context("cert-manager is enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["cert-manager", "multitenancy-manager"]`))
			f.BindingContexts.Set(f.KubeStateSet(clusterIssuersGrantState), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("keeps every ClusterIssuer grant object from being pruned, whichever release owns it", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(policy("GrantableClusterResourceDefinition", "clusterissuers")).To(Equal("keep"))
			Expect(policy("GrantableClusterResourceReference", "clusterissuers-certificate")).To(Equal("keep"))
			Expect(policy("GrantableClusterResourceReference", "clusterissuers-ingress")).To(Equal("keep"))
		})

		It("leaves the other registrations alone", func() {
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "storageclasses").Field(resourcePolicyPath).Exists()).To(BeFalse())
		})

		It("keeps the release ownership as it is", func() {
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "clusterissuers").
				Field("metadata.annotations.meta\\.helm\\.sh/release-name").String()).To(Equal("multitenancy-manager"))
		})
	})

	Context("cert-manager is disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["multitenancy-manager"]`))
			f.BindingContexts.Set(f.KubeStateSet(clusterIssuersGrantState), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("lets the release prune the objects", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "clusterissuers").Field(resourcePolicyPath).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", "clusterissuers-certificate").Field(resourcePolicyPath).Exists()).To(BeFalse())
		})
	})

	Context("the objects are gone", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["cert-manager", "multitenancy-manager"]`))
			f.BindingContexts.Set(f.KubeStateSet(""), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("does nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "clusterissuers").Exists()).To(BeFalse())
		})
	})
})
