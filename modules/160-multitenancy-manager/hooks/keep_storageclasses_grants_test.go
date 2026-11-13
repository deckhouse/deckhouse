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

const storageClassesGrantState = `
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: storageclasses
  annotations:
    meta.helm.sh/release-name: multitenancy-manager
    meta.helm.sh/release-namespace: d8-system
spec:
  enforcement: Managed
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: storageclasses-pvc
  annotations:
    meta.helm.sh/release-name: storage-foundation
    meta.helm.sh/release-namespace: d8-system
spec:
  grantableClusterResourceName: storageclasses
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: loadbalancerclasses
  annotations:
    meta.helm.sh/release-name: multitenancy-manager
    meta.helm.sh/release-namespace: d8-system
spec:
  enforcement: Managed
`

var _ = Describe("Modules :: multitenancy-manager :: hooks :: keep_storageclasses_grants ::", func() {
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

	Context("storage-foundation registers the StorageClass grant", func() {
		BeforeEach(func() {
			f.ValuesSet(storageFoundationShipsStorageClassKey, true)
			f.BindingContexts.Set(f.KubeStateSet(storageClassesGrantState), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("keeps both StorageClass grant objects from being pruned, whichever release owns them", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(policy("GrantableClusterResourceDefinition", "storageclasses")).To(Equal("keep"))
			Expect(policy("GrantableClusterResourceReference", "storageclasses-pvc")).To(Equal("keep"))
		})

		It("leaves the other registrations alone", func() {
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "loadbalancerclasses").Field(resourcePolicyPath).Exists()).To(BeFalse())
		})

		It("keeps the release ownership as it is", func() {
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "storageclasses").
				Field("metadata.annotations.meta\\.helm\\.sh/release-name").String()).To(Equal("multitenancy-manager"))
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", "storageclasses-pvc").
				Field("metadata.annotations.meta\\.helm\\.sh/release-name").String()).To(Equal("storage-foundation"))
		})
	})

	Context("storage-foundation does not register the StorageClass grant", func() {
		BeforeEach(func() {
			f.ValuesSet(storageFoundationShipsStorageClassKey, false)
			f.BindingContexts.Set(f.KubeStateSet(storageClassesGrantState), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("leaves the objects to the template of this module", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "storageclasses").Field(resourcePolicyPath).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", "storageclasses-pvc").Field(resourcePolicyPath).Exists()).To(BeFalse())
		})
	})

	Context("the objects are gone", func() {
		BeforeEach(func() {
			f.ValuesSet(storageFoundationShipsStorageClassKey, true)
			f.BindingContexts.Set(f.KubeStateSet(""), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("does nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "storageclasses").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", "storageclasses-pvc").Exists()).To(BeFalse())
		})
	})
})
