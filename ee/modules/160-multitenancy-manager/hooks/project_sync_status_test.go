/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Multitenancy Manager hooks :: handle Projects ready status ::", func() {
	f := HookExecutionConfigInit(`{"multitenancyManager":{"internal":{"projects": []}}}`, `{}`)
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
				f.ValuesSetFromYaml("multitenancyManager.internal.projects", stateTwoProjectsValues)
				f.BindingContexts.Set(f.KubeStateSet(stateTwoProjectsWithDeployingStatusAndOneWithError))
				f.RunHook()
			})

			It("Valid Projects status Sync", func() {
				conds := []testProjectStatus{
					{
						name:   "test-1",
						exists: true,
						status: `{"sync":true,"state":"Sync"}`,
					},
					{
						name:   "test-2",
						exists: true,
						status: `{"sync":false,"state":"Deploying","message":"Deckhouse is creating the project, see deckhouse logs for more details."}`,
					},
					{
						name:   "test-3",
						exists: true,
						status: `{"sync":false,"state":"Error","message":"Can't find valid ProjectType '' for Project."}`,
					},
				}
				for _, tc := range conds {
					checkProjectStatus(f, tc)
				}
			})

		})
	})
})

const (
	stateTwoProjectsWithDeployingStatusAndOneWithError = `
---
apiVersion: deckhouse.io/v1alpha1
kind: Project
metadata:
  name: test-1
status:
  state: Deploying
  message: "Deckhouse is creating the project, see deckhouse logs for more details."
  sync: false
---
apiVersion: deckhouse.io/v1alpha1
kind: Project
metadata:
  name: test-2
status:
  state: Deploying
  message: "Deckhouse is creating the project, see deckhouse logs for more details."
  sync: false
---
apiVersion: deckhouse.io/v1alpha1
kind: Project
metadata:
  name: test-3
status:
  state: Error
  message: "Can't find valid ProjectType '' for Project."
  sync: false
`
)

var (
	stateTwoProjectsValues = []byte(`
- projectName: test-1
- projectName: test-3
`)
)
