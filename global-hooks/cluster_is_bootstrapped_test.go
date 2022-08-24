// Copyright 2021 Flant JSC
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

/*

User-stories:
1. If there is other ready nodes in addition to master-nodes, we can assume that the cluster has been bootstrapped.

*/

package hooks

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{"global": {}}`
	initConfigValuesString = `{}`
)

const (
	stateMasterOnly = `
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-master
spec:
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
status:
  conditions:
  - status: "True"
    type: Ready
`
	stateMasterAndNotReadyNode = `
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-master
spec:
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-worker-1
spec:
taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/worker
status:
  conditions:
  - status: "False"
    type: Ready
`

	stateMasterAndReadyNode = `
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-master
spec:
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-worker-1
spec:
taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/worker
status:
  conditions:
  - status: "True"
    type: Ready
`
)

var stateMasterAndCM = fmt.Sprintf(`
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-master
spec:
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: kube-system
`, clusterBootstrappedConfigMap)

var _ = Describe("Global hooks :: cluster_is_bootstrapped ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Cluster has no nodes except master", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterOnly))
			f.RunHook()
		})

		It("`global.clusterIsBootstrapped` must not exist", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.clusterIsBootstrapped").Exists()).To(BeFalse())
		})

		Context("Worker node with status NotReady added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateMasterAndNotReadyNode))
				f.RunHook()
			})

			It("`global.clusterIsBootstrapped` must not exist", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.clusterIsBootstrapped").Exists()).To(BeFalse())
			})

			Context("State of additional node changed to Ready", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateMasterAndReadyNode))
					f.RunHook()
				})

				It("`global.clusterIsBootstrapped` must be 'true'", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("global.clusterIsBootstrapped").Bool()).To(BeTrue())
				})

				It("cluster bootstrap configmap must be created", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.KubernetesResource("ConfigMap", "kube-system", clusterBootstrappedConfigMap).Exists()).To(BeTrue())
				})
			})
		})

		Context("Someone creates cluster bootstrap configmap", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateMasterAndCM))
				f.RunHook()
			})

			It("`global.clusterIsBootstrapped` must be 'true'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.clusterIsBootstrapped").Bool()).To(BeTrue())
			})
		})
	})

	Context("Cluster has master and additional nodes in NotReady state", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterAndNotReadyNode))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
		})

		It("`global.clusterIsBootstrapped` must not exist", func() {
			Expect(f.ValuesGet("global.clusterIsBootstrapped").Exists()).To(BeFalse())
		})
	})

	Context("Cluster has master and additional nodes in Ready state", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterAndReadyNode))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
		})

		It("`global.clusterIsBootstrapped` must be 'true'", func() {
			Expect(f.ValuesGet("global.clusterIsBootstrapped").Bool()).To(BeTrue())
			Expect(f.KubernetesResource("ConfigMap", "kube-system", clusterBootstrappedConfigMap).Exists()).To(BeTrue())
		})
	})

	Context("Cluster has cluster bootstrap configmap", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterAndCM))
			f.RunHook()
		})

		It("`global.clusterIsBootstrapped` must be 'true'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.clusterIsBootstrapped").Bool()).To(BeTrue())
		})

		It("cluster bootstrap configmap must be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "kube-system", clusterBootstrappedConfigMap).Exists()).To(BeTrue())
		})

		Context("Cluster bootstrap configmap was deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateMasterOnly))
				f.RunHook()
			})

			It("configmap must be recreate", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesResource("ConfigMap", "kube-system", clusterBootstrappedConfigMap).Exists()).To(BeTrue())
			})

			It("`global.clusterIsBootstrapped` must be stay as 'true'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.clusterIsBootstrapped").Bool()).To(BeTrue())
			})
		})
	})
})
