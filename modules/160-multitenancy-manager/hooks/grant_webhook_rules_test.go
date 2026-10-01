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

var _ = Describe("Modules :: multitenancy-manager :: hooks :: grant_webhook_rules ::", func() {
	const initValues = `
global:
  discovery: {}
multitenancyManager:
  internal:
    admissionWebhookCert: {}
`

	const managedDefinitions = `
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: clusterroles
spec:
  enforcement: Managed
  defaultAvailability: All
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: storageclasses
spec:
  enforcement: Managed
  defaultAvailability: All
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: external
spec:
  enforcement: External
  defaultAvailability: All
`

	const rolebindingReference = `
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: clusterroles-rolebinding
spec:
  grantableClusterResourceName: clusterroles
  rule:
    apiGroups: ["rbac.authorization.k8s.io"]
    apiVersions: ["v1"]
    resources: ["rolebindings"]
`

	const otherReferences = `
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: storageclasses-pvc
spec:
  grantableClusterResourceName: storageclasses
  rule:
    apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["persistentvolumeclaims"]
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: clusterroles-rolebinding-duplicate
spec:
  grantableClusterResourceName: clusterroles
  rule:
    apiGroups: ["rbac.authorization.k8s.io"]
    apiVersions: ["v1"]
    resources: ["rolebindings"]
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: dangling
spec:
  grantableClusterResourceName: missing
  rule:
    apiGroups: ["apps"]
    apiVersions: ["v1"]
    resources: ["deployments"]
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: external
spec:
  grantableClusterResourceName: external
  rule:
    apiGroups: ["batch"]
    apiVersions: ["v1"]
    resources: ["jobs"]
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: wildcard-group
spec:
  grantableClusterResourceName: clusterroles
  rule:
    apiGroups: ["*"]
    apiVersions: ["v1"]
    resources: ["configmaps"]
`

	f := HookExecutionConfigInit(initValues, `{}`)
	f.RegisterCRD("multitenancy.deckhouse.io", "v1alpha1", "GrantableClusterResourceDefinition", false)
	f.RegisterCRD("multitenancy.deckhouse.io", "v1alpha1", "GrantableClusterResourceReference", false)

	Context("no grantable references", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(managedDefinitions))
			f.RunHook()
		})

		It("writes an empty list", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(grantWebhookRulesValuesPath).String()).To(MatchJSON(`[]`))
		})
	})

	Context("references of every kind", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(managedDefinitions + rolebindingReference + otherReferences))
			f.RunHook()
		})

		It("keeps managed targets only, once each, sorted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(grantWebhookRulesValuesPath).String()).To(MatchJSON(`[
				{"apiGroup": "", "resource": "persistentvolumeclaims"},
				{"apiGroup": "rbac.authorization.k8s.io", "resource": "rolebindings"}
			]`))
		})

		Context("a reference is deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(managedDefinitions + rolebindingReference))
				f.RunHook()
			})

			It("drops its rule", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(grantWebhookRulesValuesPath).String()).To(MatchJSON(`[
					{"apiGroup": "rbac.authorization.k8s.io", "resource": "rolebindings"}
				]`))
			})
		})
	})

	Context("the target definition switches to External", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: clusterroles
spec:
  enforcement: External
  defaultAvailability: All
` + rolebindingReference))
			f.RunHook()
		})

		It("stops intercepting the resource", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(grantWebhookRulesValuesPath).String()).To(MatchJSON(`[]`))
		})
	})
})
