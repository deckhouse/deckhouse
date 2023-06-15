/*
Copyright 2021 Flant JSC

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

var _ = Describe("Modules :: nodeManager :: hooks :: handle_capi_clusters ::", func() {
	var (
		initialState = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: dev1
  namespace: d8-cloud-instance-manager
status:
  infrastructureReady: false
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: dev2
  namespace: d8-cloud-instance-manager
status:
  infrastructureReady: true
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: dev3
  namespace: d8-cloud-instance-manager
`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta1", "Cluster", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("update statuses", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(initialState))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("clusters status infrastructure state should be true in all cases", func() {
			Expect(f).To(ExecuteSuccessfully())
			dev1 := f.KubernetesResource("Cluster", "d8-cloud-instance-manager", "dev1")
			dev2 := f.KubernetesResource("Cluster", "d8-cloud-instance-manager", "dev2")
			dev3 := f.KubernetesResource("Cluster", "d8-cloud-instance-manager", "dev3")

			Expect(dev1.Exists()).To(BeTrue())
			Expect(dev2.Exists()).To(BeTrue())
			Expect(dev3.Exists()).To(BeTrue())
			Expect(dev1.Field("status.infrastructureReady").Bool()).To(BeTrue())
			Expect(dev2.Field("status.infrastructureReady").Bool()).To(BeTrue())
			Expect(dev3.Field("status.infrastructureReady").Bool()).To(BeTrue())
		})

	})

})
