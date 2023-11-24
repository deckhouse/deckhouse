// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
			f.BindingContexts.Set(f.KubeStateSet(validProject + namespace))
			f.RunHook()
		})

		It("Execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Namespace annotations are updated", func() {
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
