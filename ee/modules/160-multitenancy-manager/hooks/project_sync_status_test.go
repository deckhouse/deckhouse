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
						name:       "test-1",
						exists:     true,
						conditions: `[{"name":"Deploying","status":false},{"name":"Sync","status":true}]`,
						status:     `{"status":true}`,
					},
					{
						name:       "test-2",
						exists:     true,
						conditions: `[{"name":"Deploying","status":false},{"name":"Sync","status":true}]`,
						status:     `{"status":true}`,
					},
					{
						name:       "test-3",
						exists:     true,
						conditions: `[{"message":"Can't find valid ProjectType '' for Project","name":"Error","status":false}]`,
						status:     `{"status":false}`,
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
  conditions:
    - name: Deploying
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
---
apiVersion: deckhouse.io/v1alpha1
kind: Project
metadata:
  name: test-3
status:
  conditions:
    - message: Can't find valid ProjectType '' for Project
      name: Error
      status: false
  statusSummary:
    status: false
`
)

var (
	stateTwoProjectsValues = []byte(`
- projectName: test-1
- projectName: test-2
`)
)
