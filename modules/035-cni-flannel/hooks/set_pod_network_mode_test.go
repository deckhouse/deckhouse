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

var _ = Describe("Modules :: cniFlannel :: hooks :: set_pod_network_mode ::", func() {
	f := HookExecutionConfigInit(`{"cniFlannel":{"podNetworkMode":"HostGW", "internal":{}}}`, ``)

	state := `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cni-configuration
  namespace: kube-system
data:
  cni: Zmxhbm5lbA== # flannel
  flannel: ICAgIHsKICAgICAgInBvZE5ldHdvcmtNb2RlIjogInZ4bGFuIgogICAgfQ== # {"podNetworkMode":"vxlan"}
`

	stateWithEmptyFlannelConfig := `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cni-configuration
  namespace: kube-system
data:
  cni: Zmxhbm5lbA== # flannel
  flannel: e30= # {}
`

	stateWithoutFlannelConfig := `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cni-configuration
  namespace: kube-system
data:
  cni: Zmxhbm5lbA== # flannel
`

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})
		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("d8-cni-configuration", func() {
		It("Must be executed successfully", func() {
			By("podNetworkMode must be vxlan", func() {
				f.BindingContexts.Set(f.KubeStateSet(state))
				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("vxlan"))
			})
		})

		It("Must be executed successfully", func() {
			By("podNetworkMode must be vxlan, because secret has higher priority, than config", func() {
				f.BindingContexts.Set(f.KubeStateSet(state))
				f.ConfigValuesSet("cniFlannel.podNetworkMode", "HostGW")
				f.RunHook()
				Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("vxlan"))
			})

		})
		It("Must be executed successfully", func() {
			By("podNetworkMode must be host-gw", func() {
				f.BindingContexts.Set(f.KubeStateSet(stateWithEmptyFlannelConfig))
				f.RunHook()
				Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
			})
		})

		It("Must be executed successfully", func() {
			By("podNetworkMode must be host-gw", func() {
				f.BindingContexts.Set(f.KubeStateSet(stateWithoutFlannelConfig))
				f.RunHook()
				Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
			})
		})

	})

	// BeforeHelm without snapshots.
	Context("BeforeHelm on empty cluster", func() {
		It("Should use config values", func() {
			By("podNetworkMode must be host-gw", func() {
				f.ConfigValuesSet("cniFlannel.podNetworkMode", "HostGW")
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
				Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
			})

		})
	})

	// BeforeHelm without snapshot.
	Context("BeforeHelm on cluster with Secret", func() {
		It("Should use value from Secret", func() {
			By("podNetworkMode must be vxlan", func() {
				f.ConfigValuesSet("cniFlannel.podNetworkMode", "HostGW")
				f.KubeStateSet(state)
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
				Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("vxlan"))
			})
		})
	})

})
