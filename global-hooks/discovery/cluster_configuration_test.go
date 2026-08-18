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

package hooks

import (
	_ "github.com/flant/addon-operator/sdk"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery/clusterConfiguration ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	var (
		stateA = clusterConfigurationSecret(ccStateAClusterConfiguration)
		stateB = clusterConfigurationSecret(ccStateBClusterConfiguration)
		stateC = clusterConfigurationSecret(ccStateCClusterConfiguration)
	)

	// Set default value for test purposes. Normally this var set to specific kube version on the build stage.
	hooks.DefaultKubernetesVersion = "1.36"

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("Cluster has a d8-cluster-configuration Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA, 1))
			f.RunHook()
		})

		It("Should correctly fill the Values store from it", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.clusterConfiguration.clusterType").String()).To(Equal("Static"))
			Expect(f.ValuesGet("global.clusterConfiguration.cloud.provider").String()).To(Equal("OpenStack"))
			Expect(f.ValuesGet("global.clusterConfiguration.cloud.prefix").String()).To(Equal("kube"))
			Expect(f.ValuesGet("global.clusterConfiguration.podSubnetCIDR").String()).To(Equal("10.111.0.0/16"))
			Expect(f.ValuesGet("global.clusterConfiguration.podSubnetNodeCIDRPrefix").String()).To(Equal("24"))
			Expect(f.ValuesGet("global.clusterConfiguration.serviceSubnetCIDR").String()).To(Equal("10.222.0.0/16"))
			Expect(f.ValuesGet("global.clusterConfiguration.kubernetesVersion").String()).To(Equal("1.33"))

			Expect(f.ValuesGet("global.discovery.podSubnet").String()).To(Equal("10.111.0.0/16"))
			Expect(f.ValuesGet("global.discovery.serviceSubnet").String()).To(Equal("10.222.0.0/16"))
			Expect(f.ValuesGet("global.discovery.clusterDomain").String()).To(Equal("test.local"))

			var maxNodes *float64
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				if m.Name == "d8_max_nodes_amount_by_pod_cidr" {
					maxNodes = m.Value
					break
				}
			}
			Expect(maxNodes).NotTo(BeNil())
			Expect(*maxNodes).To(Equal(float64(256)))
		})

		Context("d8-cluster-configuration Secret has changed", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateB, 1))
				f.RunHook()
			})

			It("Should correctly fill the Values store from it", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.clusterConfiguration.clusterType").String()).To(Equal("Cloud"))
				Expect(f.ValuesGet("global.clusterConfiguration.cloud.provider").String()).To(Equal("AWS"))
				Expect(f.ValuesGet("global.clusterConfiguration.cloud.prefix").String()).To(Equal("lube"))
				Expect(f.ValuesGet("global.clusterConfiguration.podSubnetCIDR").String()).To(Equal("10.122.0.0/16"))
				Expect(f.ValuesGet("global.clusterConfiguration.podSubnetNodeCIDRPrefix").String()).To(Equal("26"))
				Expect(f.ValuesGet("global.clusterConfiguration.serviceSubnetCIDR").String()).To(Equal("10.213.0.0/16"))
				Expect(f.ValuesGet("global.clusterConfiguration.kubernetesVersion").String()).To(Equal("1.33"))

				Expect(f.ValuesGet("global.discovery.podSubnet").String()).To(Equal("10.122.0.0/16"))
				Expect(f.ValuesGet("global.discovery.serviceSubnet").String()).To(Equal("10.213.0.0/16"))
				Expect(f.ValuesGet("global.discovery.clusterDomain").String()).To(Equal("test.local"))

				var maxNodes *float64
				for _, m := range f.MetricsCollector.CollectedMetrics() {
					if m.Name == "d8_max_nodes_amount_by_pod_cidr" {
						maxNodes = m.Value
						break
					}
				}
				Expect(maxNodes).NotTo(BeNil())
				Expect(*maxNodes).To(Equal(float64(1024)))
			})
		})

		Context("d8-cluster-configuration Secret got deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts("", 0))
				f.RunHook()
			})

			It("Should not fail, but should not create any Values", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.ValuesGet("global.clusterConfiguration").Exists()).To(Not(BeTrue()))
			})
		})
	})

	Context("Cluster doesn't have a d8-cluster-configuration Secret", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts("", 0))
			f.RunHook()
		})

		It("Should not fail, but should not create any Values", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("global.clusterConfiguration").Exists()).To(Not(BeTrue()))
		})
	})

	Context("Cluster has a d8-cluster-configuration Secret with kubernetesVersion = `Automatic`", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateC, 1))
			f.RunHook()
		})

		It("Should correctly fill the Values store from it", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("global.clusterConfiguration.clusterType").String()).To(Equal("Cloud"))
			Expect(f.ValuesGet("global.clusterConfiguration.cloud.provider").String()).To(Equal("AWS"))
			Expect(f.ValuesGet("global.clusterConfiguration.cloud.prefix").String()).To(Equal("lube"))
			Expect(f.ValuesGet("global.clusterConfiguration.podSubnetCIDR").String()).To(Equal("10.122.0.0/16"))
			Expect(f.ValuesGet("global.clusterConfiguration.podSubnetNodeCIDRPrefix").String()).To(Equal("26"))
			Expect(f.ValuesGet("global.clusterConfiguration.serviceSubnetCIDR").String()).To(Equal("10.213.0.0/16"))
			Expect(f.ValuesGet("global.clusterConfiguration.kubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))

			Expect(f.ValuesGet("global.discovery.podSubnet").String()).To(Equal("10.122.0.0/16"))
			Expect(f.ValuesGet("global.discovery.serviceSubnet").String()).To(Equal("10.213.0.0/16"))
			Expect(f.ValuesGet("global.discovery.clusterDomain").String()).To(Equal("test.local"))

			var maxNodes *float64
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				if m.Name == "d8_max_nodes_amount_by_pod_cidr" {
					maxNodes = m.Value
					break
				}
			}
			Expect(maxNodes).NotTo(BeNil())
			Expect(*maxNodes).To(Equal(float64(1024)))
		})
	})

})
