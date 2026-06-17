/*
Copyright 2022 Flant JSC

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

var _ = Describe("Modules :: node-manager :: hooks :: set_node_controller_leader_pod ::", func() {
	f := HookExecutionConfigInit(`{"global":{},"nodeManager":{"internal":{}}}`, `{}`)

	f.RegisterCRD("coordination.k8s.io", "v1", "Lease", true)

	Context("Leader lease exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSetAndWaitForBindingContexts(
					nodeControllerLease+
						nodeControllerLeaderPod+
						nodeControllerFollowerPod,
					3,
				),
			)

			f.RunHook()
		})

		It("Should set leader label only on leader pod", func() {
			Expect(f).To(ExecuteSuccessfully())

			leader := f.KubernetesResource(
				"Pod",
				nodeControllerNamespace,
				"leader-pod",
			)

			Expect(leader.Exists()).To(BeTrue())
			Expect(leader.Field("metadata.labels.leader").Exists()).To(BeTrue())
			Expect(leader.Field("metadata.labels.leader").String()).To(Equal("true"))

			follower := f.KubernetesResource(
				"Pod",
				nodeControllerNamespace,
				"follower-pod",
			)

			Expect(follower.Exists()).To(BeTrue())
			Expect(follower.Field("metadata.labels.leader").Exists()).To(BeFalse())
		})
	})
})

const (
	nodeControllerLease = `
---
apiVersion: coordination.k8s.io/v1
kind: Lease
metadata:
  name: node-controller.deckhouse.io
  namespace: d8-cloud-instance-manager
spec:
  holderIdentity: leader-pod_test-uuid
`

	nodeControllerLeaderPod = `
---
apiVersion: v1
kind: Pod
metadata:
  name: leader-pod
  namespace: d8-cloud-instance-manager
  labels:
    app: node-controller
spec:
  containers:
    - name: node-controller
      image: node-controller:test
`

	nodeControllerFollowerPod = `
---
apiVersion: v1
kind: Pod
metadata:
  name: follower-pod
  namespace: d8-cloud-instance-manager
  labels:
    app: node-controller
    leader: "true"
spec:
  containers:
    - name: node-controller
      image: node-controller:test
`
)
