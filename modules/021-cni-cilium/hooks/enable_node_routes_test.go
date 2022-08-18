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

var _ = Describe("Modules :: cni-cilium :: hooks :: enable-node-routes", func() {
	f := HookExecutionConfigInit(
		`{
"cniCilium": {"internal": {"hubble": {"certs": {"ca":{}}}}},
"global": {
  "clusterConfiguration": {
    "apiVersion": "deckhouse.io/v1",
    "cloud": {
      "provider": "OpenStack"
    },
    "kind": "ClusterConfiguration",
    "kubernetesVersion": "1.21",
    "clusterType": "Cloud",
    "podSubnetCIDR": "10.111.0.0/16",
    "podSubnetNodeCIDRPrefix": "24",
    "serviceSubnetCIDR": "10.222.0.0/16"
  }
}
}`,
		`{"cniCilium":{}}`,
	)
	Context("fresh Openstack cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should be true value", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.createNodeRoutes").Bool()).To(BeTrue())
		})

		Context("with tunnelMode set to VXLAN in config", func() {
			BeforeEach(func() {
				f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
				f.RunHook()
			})

			It("should be false", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("cniCilium.createNodeRoutes").Bool()).To(BeFalse())
			})
		})

		Context("with directly set createNodeRoutes value in config", func() {
			BeforeEach(func() {
				f.ConfigValuesSet("cniCilium.createNodeRoutes", false)
				f.RunHook()
			})

			It("should be false", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("cniCilium.createNodeRoutes").Bool()).To(BeFalse())
			})
		})
	})

	Context("fresh AWS cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.cloud.provider", "AWS")
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should be false value", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.createNodeRoutes").Bool()).To(BeFalse())
		})
		Context("with tunnelMode set to VXLAN in config", func() {
			BeforeEach(func() {
				f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
				f.RunHook()
			})

			It("should be false", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("cniCilium.createNodeRoutes").Bool()).To(BeFalse())
			})
		})
		Context("with directly set createNodeRoutes value in config", func() {
			BeforeEach(func() {
				f.ConfigValuesSet("cniCilium.createNodeRoutes", true)
				f.RunHook()
			})

			It("should be true", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("cniCilium.createNodeRoutes").Bool()).To(BeTrue())
			})
		})
	})
})
