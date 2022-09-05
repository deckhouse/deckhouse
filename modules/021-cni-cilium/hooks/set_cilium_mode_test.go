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

var _ = Describe("Modules :: cni-cilium :: hooks :: set_cilium_mode", func() {
	f := HookExecutionConfigInit(`{"cniCilium":{"internal":{}}}`, "")
	Context("fresh cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
		})
	})

	Context("kube-system/d8-cni-configuration is present, but cni != `cilium`", func() {
		cniSecret := generateCniConfigurationSecret("flannel", "")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
		})
	})

	Context("kube-system/d8-cni-configuration is present, cni == `cilium`, but cilium field is not set", func() {
		cniSecret := generateCniConfigurationSecret("cilium", "")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
		})
	})

	Context("kube-system/d8-cni-configuration is present, cni = `cilium`, cilium mode = VXLAN", func() {
		cniSecret := generateCniConfigurationSecret("cilium", "VXLAN")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be set to `VXLAN`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
		})
	})

	Context("kube-system/d8-cni-configuration is absent, tunnelMode set to `VXLAN`", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.RunHook()
		})
		It("hook should run successfully, secret should be changed", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
		})
	})

	Context("kube-system/d8-cni-configuration is absent, createNodeRoutes set to `true`", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("cniCilium.createNodeRoutes", true)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `DirectWithNodeRoutes`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
		})
	})

	Context("kube-system/d8-cni-configuration is absent, createNodeRoutes set to `false`", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("cniCilium.createNodeRoutes", false)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
		})
	})

	f = HookExecutionConfigInit(`{}`, `{}`)

	Context("kube-system/d8-cni-configuration is absent, createNodeRoutes is absent, cloud-provider = openstack", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSetFromYaml("global.clusterConfiguration", []byte(`
apiVersion: deckhouse.io/v1
cloud:
  prefix: dev
  provider: OpenStack
clusterDomain: cluster.local
clusterType: Cloud
defaultCRI: Containerd
kind: ClusterConfiguration
kubernetesVersion: "1.20"
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
`))
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `DirectWithNodeRoutes`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
		})
	})

	Context("kube-system/d8-cni-configuration is absent, createNodeRoutes is absent, cloud-provider = vsphere", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSetFromYaml("global.clusterConfiguration", []byte(`
apiVersion: deckhouse.io/v1
cloud:
  prefix: dev
  provider: vSphere
clusterDomain: cluster.local
clusterType: Cloud
defaultCRI: Containerd
kind: ClusterConfiguration
kubernetesVersion: "1.20"
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
`))
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `DirectWithNodeRoutes`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
		})
	})

	Context("kube-system/d8-cni-configuration is absent, createNodeRoutes is absent, cloud-provider nor vsphere nor openstack", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSetFromYaml("global.clusterConfiguration", []byte(`
apiVersion: deckhouse.io/v1
cloud:
  prefix: dev
  provider: Yandex
clusterDomain: cluster.local
clusterType: Cloud
defaultCRI: Containerd
kind: ClusterConfiguration
kubernetesVersion: "1.20"
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
`))
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
		})
	})

})
