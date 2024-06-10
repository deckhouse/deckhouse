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
	"encoding/base64"

	_ "github.com/flant/addon-operator/sdk"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery/clusterConfiguration ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	var (
		stateAClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
cloud:
  provider: OpenStack
  prefix: kube
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.29"
clusterDomain: "test.local"
`
		stateA = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration))

		stateBClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: AWS
  prefix: lube
podSubnetCIDR: 10.122.0.0/16
podSubnetNodeCIDRPrefix: "26"
serviceSubnetCIDR: 10.213.0.0/16
kubernetesVersion: "1.29"
clusterDomain: "test.local"
`
		stateB = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateBClusterConfiguration))

		stateCClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: AWS
  prefix: lube
podSubnetCIDR: 10.122.0.0/16
podSubnetNodeCIDRPrefix: "26"
serviceSubnetCIDR: 10.213.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "test.local"
`
		stateC = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateCClusterConfiguration))
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

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
			Expect(f.ValuesGet("global.clusterConfiguration.kubernetesVersion").String()).To(Equal("1.29"))

			Expect(f.ValuesGet("global.discovery.podSubnet").String()).To(Equal("10.111.0.0/16"))
			Expect(f.ValuesGet("global.discovery.serviceSubnet").String()).To(Equal("10.222.0.0/16"))
			Expect(f.ValuesGet("global.discovery.clusterDomain").String()).To(Equal("test.local"))

			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(1))
			value := metrics[0].Value
			Expect(*value).To(Equal(float64(256)))
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
				Expect(f.ValuesGet("global.clusterConfiguration.kubernetesVersion").String()).To(Equal("1.29"))

				Expect(f.ValuesGet("global.discovery.podSubnet").String()).To(Equal("10.122.0.0/16"))
				Expect(f.ValuesGet("global.discovery.serviceSubnet").String()).To(Equal("10.213.0.0/16"))
				Expect(f.ValuesGet("global.discovery.clusterDomain").String()).To(Equal("test.local"))

				metrics := f.MetricsCollector.CollectedMetrics()
				Expect(metrics).To(HaveLen(1))
				value := metrics[0].Value
				Expect(*value).To(Equal(float64(1024)))
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
			Expect(f.ValuesGet("global.clusterConfiguration.kubernetesVersion").String()).To(Equal(config.DefaultKubernetesVersion))

			Expect(f.ValuesGet("global.discovery.podSubnet").String()).To(Equal("10.122.0.0/16"))
			Expect(f.ValuesGet("global.discovery.serviceSubnet").String()).To(Equal("10.213.0.0/16"))
			Expect(f.ValuesGet("global.discovery.clusterDomain").String()).To(Equal("test.local"))

			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(1))
			value := metrics[0].Value
			Expect(*value).To(Equal(float64(1024)))
		})

	})

})
