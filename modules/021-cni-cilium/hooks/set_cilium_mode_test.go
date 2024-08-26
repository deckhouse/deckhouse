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
	"encoding/json"

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
			f.ConfigValuesSet("cniCilium.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
		})
	})

	Context("tunnelMode set to `VXLAN`", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ConfigValuesSet("cniCilium.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, secret should be changed", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
		})
	})

	Context("tunnelMode set to `Disabled`, but previously the mode was discovered to `VXLAN`", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("cniCilium.tunnelMode", "Disabled")
			f.ValuesSet("cniCilium.internal.mode", "VXLAN")
			f.RunHook()
		})
		It("hook should run successfully, mode must be changed to Direct", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
		})
	})

	Context("createNodeRoutes set to `true`", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("cniCilium.createNodeRoutes", true)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ConfigValuesSet("cniCilium.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `DirectWithNodeRoutes`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
		})
	})

	Context("createNodeRoutes set to `false`", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("cniCilium.createNodeRoutes", false)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ConfigValuesSet("cniCilium.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
		})
	})

	Context("config parameters is absent, but cloud provider = Static", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSetFromYaml("global.clusterConfiguration", []byte(`
apiVersion: deckhouse.io/v1
clusterType: Static
kind: ClusterConfiguration
kubernetesVersion: "Automatic"
podSubnetCIDR: 10.231.0.0/16
serviceSubnetCIDR: 10.232.0.0/16
`))
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ConfigValuesSet("cniCilium.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `DirectWithNodeRoutes`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
		})
	})

	Context("config parameters is absent, but cloud provider != Static", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSetFromYaml("global.clusterConfiguration", []byte(`
apiVersion: deckhouse.io/v1
clusterType: Cloud
cloud:
  prefix: test
  provider: Yandex
clusterDomain: cluster.local
kind: ClusterConfiguration
kubernetesVersion: "Automatic"
podSubnetCIDR: 10.231.0.0/16
serviceSubnetCIDR: 10.232.0.0/16
`))
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ConfigValuesSet("cniCilium.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
		})
	})

})

type CiliumConfigStruct struct {
	Mode           string `json:"mode,omitempty"`
	MasqueradeMode string `json:"masqueradeMode,omitempty"`
}

func generateJSONCiliumConf(mode string, masqueradeMode string) ([]byte, error) {
	var confMAP CiliumConfigStruct
	if mode != "" {
		confMAP.Mode = mode
	}
	if masqueradeMode != "" {
		confMAP.MasqueradeMode = masqueradeMode
	}

	return json.Marshal(confMAP)
}
