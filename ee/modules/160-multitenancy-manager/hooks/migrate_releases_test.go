/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Multitenancy Manager hooks :: migrate releases ::", func() {
	f := HookExecutionConfigInit(`{"multitenancyManager":{"internal":{"projects":[]}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Project", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ProjectType", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with Projects", func() {
		BeforeEach(func() {
			f.KubeStateSet(validProject + namespace + randomNamespace)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Project namespace annotations are updated", func() {
			Expect(f.KubernetesGlobalResource("Namespace", "test-project").Field(`metadata.annotations.meta\.helm\.sh/release-name`).String()).To(Equal("test-project"))
			Expect(f.KubernetesGlobalResource("Namespace", "test-project").Field(`metadata.annotations.meta\.helm\.sh/release-namespace`).String()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("Namespace", "test-project").Field(`metadata.annotations.helm\.sh/resource-policy`).String()).To(Equal("keep"))
		})

		It("Random namespace annotations don't exist", func() {
			Expect(f.KubernetesGlobalResource("Namespace", "random").Field(`metadata.annotations.meta\.helm\.sh/release-name`).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("Namespace", "random").Field(`metadata.annotations.meta\.helm\.sh/release-namespace`).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("Namespace", "random").Field(`metadata.annotations.helm\.sh/resource-policy`).Exists()).To(BeFalse())
		})
	})
})

const validProject = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Project
metadata:
  name: test-project
spec:
  description: Test case from Deckhouse documentation
  projectTypeName: test-project-type
  template:
    requests:
      cpu: 5
      memory: 5Gi
      storage: 1Gi
    limits:
      cpu: 5
      memory: 5Gi
`

const namespace = `
---
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    extended-monitoring.deckhouse.io/enabled: ""
    meta.helm.sh/release-name: d8-multitenancy-manager
    meta.helm.sh/release-namespace: "d8-system"
  labels:
    app.kubernetes.io/managed-by: Helm
  name: test-project
`

const randomNamespace = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: random
  annotations:
    extended-monitoring.deckhouse.io/enabled: ""
`
