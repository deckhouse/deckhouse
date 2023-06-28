package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Multitenancy Manager hooks :: handle Projects ready status ::", func() {
	f := HookExecutionConfigInit(`{"multitenancyManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Project", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Projects map must be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Set Project sync status if hook is executed", func() {

		Context("Cluster with two valid Projects", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateTwoProjectsWithDeployingStatus))
				f.RunHook()
			})

			It("Valid Projects status Sync", func() {
				pr1 := f.KubernetesGlobalResource("Project", "test-1")
				Expect(pr1.Exists()).To(BeTrue())

				Expect(pr1.Field("status.conditions")).To(MatchJSON(`[{"message":"Can't find valid ProjectType '' for Project","name":"Error","status":false},{"name":"Sync","status":true}]`))
				Expect(pr1.Field("status.statusSummary")).To(MatchJSON(`{"status":true}`))

				pr2 := f.KubernetesGlobalResource("Project", "test-2")
				Expect(pr2.Exists()).To(BeTrue())

				Expect(pr2.Field("status.conditions")).To(MatchJSON(`[{"name":"Deploying","status":false},{"name":"Sync","status":true}]`))
				Expect(pr2.Field("status.statusSummary")).To(MatchJSON(`{"status":true}`))
			})

		})
	})
})

const (
	stateTwoProjectsWithDeployingStatus = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Project
metadata:
  name: test-1
status:
  conditions:
    - message: Can't find valid ProjectType '' for Project
      name: Error
      status: false
  statusSummary:
    status: false
---
apiVersion: deckhouse.io/v1alpha1
kind: Project
metadata:
  name: test-2
status:
  conditions:
    - name: Deploying
      status: false
  statusSummary:
    status: false
`
)
