/*
Copyright 2024 Flant JSC

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

var _ = Describe("deckhouse :: hooks :: safe_deckhouse_ready_nodes ::", func() {
	f := HookExecutionConfigInit("", "")

	Context("One master - one ready kube-apiserver", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(`
apiVersion: v1
kind: Pod
metadata:
  labels:
    tier: control-plane
    component: kube-apiserver
  name: kube-apiserver-master-0
  namespace: kube-system
spec:
  containers:
  - name: kube-apiserver
  nodeName: master-0
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2023-03-24T15:02:56Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: master-0
  labels:
    node-role.kubernetes.io/control-plane: ""
status:
  conditions:
  - lastHeartbeatTime: "2024-05-30T10:49:14Z"
    lastTransitionTime: "2024-05-29T12:56:08Z"
    reason: KubeletReady
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: worker-0
status:
  conditions:
  - lastHeartbeatTime: "2024-05-30T10:49:14Z"
    lastTransitionTime: "2024-05-29T12:56:08Z"
    reason: KubeletReady
    status: "True"
    type: Ready
`)

			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Shouldn put label to the master node", func() {
			Expect(f).To(ExecuteSuccessfully())
			master := f.KubernetesResource("Node", "", "master-0")
			Expect(master.Field("metadata.labels.node\\.deckhouse\\.io/deckhouse-ready").String()).To(Equal("true"))
			worker := f.KubernetesResource("Node", "", "worker-0")
			Expect(worker.Field("metadata.labels.node\\.deckhouse\\.io/deckhouse-ready").Exists()).To(BeFalse())
		})
	})

	Context("One master - one not-ready kube-apiserver", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(`
apiVersion: v1
kind: Pod
metadata:
  labels:
    tier: control-plane
    component: kube-apiserver
  name: kube-apiserver-master-0
  namespace: kube-system
spec:
  containers:
  - name: kube-apiserver
  nodeName: master-0
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2023-03-24T15:02:56Z"
    status: "False"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: master-0
  labels:
    node-role.kubernetes.io/control-plane: ""
status:
  conditions:
  - lastHeartbeatTime: "2024-05-30T10:49:14Z"
    lastTransitionTime: "2024-05-29T12:56:08Z"
    reason: KubeletReady
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: worker-0
status:
  conditions:
  - lastHeartbeatTime: "2024-05-30T10:49:14Z"
    lastTransitionTime: "2024-05-29T12:56:08Z"
    reason: KubeletReady
    status: "True"
    type: Ready
`)

			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Should attach false label to the master node", func() {
			Expect(f).To(ExecuteSuccessfully())
			master := f.KubernetesResource("Node", "", "master-0")
			Expect(master.Field("metadata.labels.node\\.deckhouse\\.io/deckhouse-ready").String()).To(Equal("false"))
			worker := f.KubernetesResource("Node", "", "worker-0")
			Expect(worker.Field("metadata.labels.node\\.deckhouse\\.io/deckhouse-ready").Exists()).To(BeFalse())
		})
	})

	Context("One not-ready master - one ready kube-apiserver", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(`
apiVersion: v1
kind: Pod
metadata:
  labels:
    tier: control-plane
    component: kube-apiserver
  name: kube-apiserver-master-0
  namespace: kube-system
spec:
  containers:
  - name: kube-apiserver
  nodeName: master-0
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2023-03-24T15:02:56Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: master-0
  labels:
    node-role.kubernetes.io/control-plane: ""
status:
  conditions:
  - lastHeartbeatTime: "2024-05-30T10:49:14Z"
    lastTransitionTime: "2024-05-29T12:56:08Z"
    reason: KubeletReady
    status: "False"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: worker-0
status:
  conditions:
  - lastHeartbeatTime: "2024-05-30T10:49:14Z"
    lastTransitionTime: "2024-05-29T12:56:08Z"
    reason: KubeletReady
    status: "True"
    type: Ready
`)

			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Should attach false label to the master node", func() {
			Expect(f).To(ExecuteSuccessfully())
			master := f.KubernetesResource("Node", "", "master-0")
			Expect(master.Field("metadata.labels.node\\.deckhouse\\.io/deckhouse-ready").String()).To(Equal("false"))
			worker := f.KubernetesResource("Node", "", "worker-0")
			Expect(worker.Field("metadata.labels.node\\.deckhouse\\.io/deckhouse-ready").Exists()).To(BeFalse())
		})
	})

	Context("Two ready masters - one ready kube-apiserver", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(`
apiVersion: v1
kind: Pod
metadata:
  labels:
    tier: control-plane
    component: kube-apiserver
  name: kube-apiserver-master-1
  namespace: kube-system
spec:
  containers:
  - name: kube-apiserver
  nodeName: master-1
status:
  conditions:
  - lastProbeTime: null
    lastTransitionTime: "2023-03-24T15:02:56Z"
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: master-0
  labels:
    node-role.kubernetes.io/control-plane: ""
status:
  conditions:
  - lastHeartbeatTime: "2024-05-30T10:49:14Z"
    lastTransitionTime: "2024-05-29T12:56:08Z"
    reason: KubeletReady
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: master-1
  labels:
    node-role.kubernetes.io/control-plane: ""
status:
  conditions:
  - lastHeartbeatTime: "2024-05-30T10:49:14Z"
    lastTransitionTime: "2024-05-29T12:56:08Z"
    reason: KubeletReady
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: worker-0
status:
  conditions:
  - lastHeartbeatTime: "2024-05-30T10:49:14Z"
    lastTransitionTime: "2024-05-29T12:56:08Z"
    reason: KubeletReady
    status: "True"
    type: Ready
`)

			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Should attach false label to the master-0 node and true to the master-1", func() {
			Expect(f).To(ExecuteSuccessfully())
			master0 := f.KubernetesResource("Node", "", "master-0")
			Expect(master0.Field("metadata.labels.node\\.deckhouse\\.io/deckhouse-ready").String()).To(Equal("false"))
			master1 := f.KubernetesResource("Node", "", "master-1")
			Expect(master1.Field("metadata.labels.node\\.deckhouse\\.io/deckhouse-ready").String()).To(Equal("true"))
			worker := f.KubernetesResource("Node", "", "worker-0")
			Expect(worker.Field("metadata.labels.node\\.deckhouse\\.io/deckhouse-ready").Exists()).To(BeFalse())
		})
	})

	Context("Two ready masters - no kube-apiserver", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(`
apiVersion: v1
kind: Node
metadata:
  name: master-0
  labels:
    node-role.kubernetes.io/control-plane: ""
status:
  conditions:
  - lastHeartbeatTime: "2024-05-30T10:49:14Z"
    lastTransitionTime: "2024-05-29T12:56:08Z"
    reason: KubeletReady
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: master-1
  labels:
    node-role.kubernetes.io/control-plane: ""
status:
  conditions:
  - lastHeartbeatTime: "2024-05-30T10:49:14Z"
    lastTransitionTime: "2024-05-29T12:56:08Z"
    reason: KubeletReady
    status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: worker-0
status:
  conditions:
  - lastHeartbeatTime: "2024-05-30T10:49:14Z"
    lastTransitionTime: "2024-05-29T12:56:08Z"
    reason: KubeletReady
    status: "True"
    type: Ready
`)

			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Should attach false label to the master-0 node and true to the master-1", func() {
			Expect(f).To(ExecuteSuccessfully())
			master0 := f.KubernetesResource("Node", "", "master-0")
			Expect(master0.Field("metadata.labels.node\\.deckhouse\\.io/deckhouse-ready").String()).To(Equal("false"))
			master1 := f.KubernetesResource("Node", "", "master-1")
			Expect(master1.Field("metadata.labels.node\\.deckhouse\\.io/deckhouse-ready").String()).To(Equal("false"))
			worker := f.KubernetesResource("Node", "", "worker-0")
			Expect(worker.Field("metadata.labels.node\\.deckhouse\\.io/deckhouse-ready").Exists()).To(BeFalse())
		})
	})
})
