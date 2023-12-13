/*
Copyright 2023 Flant JSC

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

var _ = Describe("Modules :: node-manager :: hooks :: cluster_api_deployment_required ::", func() {
	const (
		nodeGroupCloudEphemeral = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
status: {}
`
		nodeGroupStatic = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
status: {}
`
		nodeGroupStaticWithStaticInstances = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  staticInstances: {}
status: {}
`
		capsEnableCm = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: capi-controller-manager
  namespace: d8-cloud-instance-manager
data:
  enable: "true"
`
		capsDisableCm = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: capi-controller-manager
  namespace: d8-cloud-instance-manager
data:
  enable: "false"
`
	)

	f := HookExecutionConfigInit(`{"global":{"discovery":{"kubernetesVersion": "1.16.15", "kubernetesVersions":["1.16.15"], "clusterUUID":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}},"nodeManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail; flag must not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.capsControllerManagerEnabled").Exists()).To(BeFalse())
			Expect(f.ValuesGet("nodeManager.internal.capiControllerManagerEnabled").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with CloudEphemeral NodeGroup only", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(nodeGroupCloudEphemeral, 1))
			f.RunHook()
		})

		It("Hook must not fail; flag must not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.capsControllerManagerEnabled").Exists()).To(BeFalse())
			Expect(f.ValuesGet("nodeManager.internal.capiControllerManagerEnabled").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with Static NodeGroup only", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(nodeGroupStatic, 1))
			f.RunHook()
		})

		It("Hook must not fail; flag must not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.capsControllerManagerEnabled").Exists()).To(BeFalse())
			Expect(f.ValuesGet("nodeManager.internal.capiControllerManagerEnabled").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with Static NodeGroup with staticInstances field", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(nodeGroupStaticWithStaticInstances, 1))
			f.RunHook()
		})

		It("Hook must not fail; flag must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.capsControllerManagerEnabled").String()).To(Equal("true"))
			Expect(f.ValuesGet("nodeManager.internal.capiControllerManagerEnabled").String()).To(Equal("true"))
		})
	})

	Context("Cluster with capi provider but without static node group", func() {
		BeforeEach(func() {
			f.ValuesSet("nodeManager.internal.cloudProvider.capiClusterName", "cluster-name")
			f.RunHook()
		})

		It("Hook must not fail; capi flag should set and caps flag should not set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.capsControllerManagerEnabled").Bool()).To(BeFalse())
			Expect(f.ValuesGet("nodeManager.internal.capiControllerManagerEnabled").Bool()).To(BeTrue())
		})
	})

	Context("Cluster with capi provider and with static node group with staticInstances field", func() {
		BeforeEach(func() {
			f.ValuesSet("nodeManager.internal.cloudProvider.capiClusterName", "cluster-name")
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupStaticWithStaticInstances))
			f.RunHook()
		})

		It("Hook must not fail; capi flags should set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.capsControllerManagerEnabled").Bool()).To(BeTrue())
			Expect(f.ValuesGet("nodeManager.internal.capiControllerManagerEnabled").Bool()).To(BeTrue())
		})
	})

	Context("Cluster with capi provider and with static node group without staticInstances field", func() {
		BeforeEach(func() {
			f.ValuesSet("nodeManager.internal.cloudProvider.capiClusterName", "cluster-name")
			f.BindingContexts.Set(f.KubeStateSet(nodeGroupStatic))
			f.RunHook()
		})

		It("Hook must not fail; capi flag should set and caps flag should not set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.capsControllerManagerEnabled").Bool()).To(BeFalse())
			Expect(f.ValuesGet("nodeManager.internal.capiControllerManagerEnabled").Bool()).To(BeTrue())
		})
	})

	Context("Cluster with capi provider and with caps enabled cm", func() {
		BeforeEach(func() {
			f.ValuesSet("nodeManager.internal.cloudProvider.capiClusterName", "cluster-name")
			f.BindingContexts.Set(f.KubeStateSet(capsEnableCm))
			f.RunHook()
		})

		It("Hook must not fail; capi flag should set and caps flag should not set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.capsControllerManagerEnabled").Bool()).To(BeTrue())
			Expect(f.ValuesGet("nodeManager.internal.capiControllerManagerEnabled").Bool()).To(BeTrue())
		})
	})

	Context("Cluster with capi provider and with caps disable cm", func() {
		BeforeEach(func() {
			f.ValuesSet("nodeManager.internal.cloudProvider.capiClusterName", "cluster-name")
			f.BindingContexts.Set(f.KubeStateSet(capsDisableCm))
			f.RunHook()
		})

		It("Hook must not fail; capi flag should set and caps flag should not set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.capsControllerManagerEnabled").Bool()).To(BeFalse())
			Expect(f.ValuesGet("nodeManager.internal.capiControllerManagerEnabled").Bool()).To(BeTrue())
		})
	})
})
