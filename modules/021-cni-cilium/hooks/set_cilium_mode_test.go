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
	"encoding/base64"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cni-cilium :: hooks :: set_cilium_mode", func() {

	const (
		initValuesString       = `{"cniCilium":{"internal": {}}}`
		initConfigValuesString = `{"cniCilium":{}}`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("fresh cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("kube-system/d8-cni-configuration is present, but cni != `cilium`", func() {
		cniSecret := generateCniConfigurationSecret("flannel", "", "")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("kube-system/d8-cni-configuration is present, cni == `cilium`, but cilium field is not set", func() {
		cniSecret := generateCniConfigurationSecret("cilium", "", "")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("kube-system/d8-cni-configuration is present, cni = `cilium`, cilium mode = VXLAN", func() {
		cniSecret := generateCniConfigurationSecret("cilium", "VXLAN", "")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be set to `VXLAN`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("kube-system/d8-cni-configuration is present, cni = `cilium`, cilium mode = DirectWithNodeRoutes, masqueradeMode = Netfilter", func() {
		cniSecret := generateCniConfigurationSecret("cilium", "DirectWithNodeRoutes", "Netfilter")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be set to `DirectWithNodeRoutes`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("Netfilter"))
		})
	})

	Context("kube-system/d8-cni-configuration is absent, tunnelMode set to `VXLAN`", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, secret should be changed", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("kube-system/d8-cni-configuration is absent, masqueradeMode set to `Netfilter`", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			f.ConfigValuesSet("cniCilium.masqueradeMode", "Netfilter")
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, secret should be changed", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("Netfilter"))
		})
	})

	Context("kube-system/d8-cni-configuration is absent, tunnelMode set to `Disabled`, but previously the mode was discovered to `VXLAN`", func() {
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

	Context("kube-system/d8-cni-configuration is absent, createNodeRoutes set to `true`", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("cniCilium.createNodeRoutes", true)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `DirectWithNodeRoutes`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("kube-system/d8-cni-configuration is absent, createNodeRoutes set to `false`", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("cniCilium.createNodeRoutes", false)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("kube-system/d8-cni-configuration is absent, config parameters is absent, but cloud provider = Static", func() {
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
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `DirectWithNodeRoutes`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("kube-system/d8-cni-configuration is absent, config parameters is absent, but cloud provider != Static", func() {
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
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("kube-system/d8-cni-configuration is absent, cloud provider = Static, tunnelMode = VXLAN and masqueradeMode = Netfilter are configured", func() {
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
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.ConfigValuesSet("cniCilium.masqueradeMode", "Netfilter")
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `VXLAN` and masqueradeMode is `Netfilter`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("Netfilter"))
		})
	})

	Context("kube-system/d8-cni-configuration with annotation network.deckhouse.io/cni-configuration-source-priority=ModuleConfig, cilium mode = VXLAN", func() {
		cniSecret := generateCniConfigurationSecretWithAnnotations("cilium", "VXLAN", "BPF", map[string]string{
			"network.deckhouse.io/cni-configuration-source-priority": "ModuleConfig",
		})
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.ConfigValuesSet("cniCilium.tunnelMode", "Disabled")
			f.ConfigValuesSet("cniCilium.createNodeRoutes", true)
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `DirectWithNodeRoutes` from MC, not secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("kube-system/d8-cni-configuration with annotation network.deckhouse.io/cni-configuration-source-priority=Secret, cilium mode = VXLAN", func() {
		cniSecret := generateCniConfigurationSecretWithAnnotations("cilium", "VXLAN", "Netfilter", map[string]string{
			"network.deckhouse.io/cni-configuration-source-priority": "Secret",
		})
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.ConfigValuesSet("cniCilium.tunnelMode", "Disabled")
			f.ConfigValuesSet("cniCilium.createNodeRoutes", true)
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `VXLAN` from secret, not MC", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("Netfilter"))
		})
	})

	Context("kube-system/d8-cni-configuration without annotation, cluster is not bootstrapped", func() {
		cniSecret := generateCniConfigurationSecret("cilium", "VXLAN", "Netfilter")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("global.clusterIsBootstrapped", false)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.ConfigValuesSet("cniCilium.tunnelMode", "Disabled")
			f.ConfigValuesSet("cniCilium.createNodeRoutes", true)
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be from MC (DirectWithNodeRoutes), not secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("kube-system/d8-cni-configuration without annotation, cluster is bootstrapped", func() {
		cniSecret := generateCniConfigurationSecret("cilium", "VXLAN", "Netfilter")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.ConfigValuesSet("cniCilium.tunnelMode", "Disabled")
			f.ConfigValuesSet("cniCilium.createNodeRoutes", true)
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be from secret (VXLAN), not MC", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("Netfilter"))
		})
	})

	Context("Static cluster with tunnelMode VXLAN configured (priority test)", func() {
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
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("tunnelMode VXLAN should take priority over Static cluster default", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Static cluster with createNodeRoutes false (should not override default)", func() {
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
			f.ConfigValuesSet("cniCilium.createNodeRoutes", false)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("Static cluster type takes priority over createNodeRoutes false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Priority annotation with value Secret", func() {
		cniSecret := generateCniConfigurationSecretWithAnnotations("cilium", "Direct", "Netfilter", map[string]string{
			"network.deckhouse.io/cni-configuration-source-priority": "Secret",
		})
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniCilium.internal.mode", "VXLAN")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			f.ConfigValuesSet("cniCilium.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("should use secret values even when MC differs", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("Netfilter"))
		})
	})

	Context("Priority annotation with non-standard value", func() {
		cniSecret := generateCniConfigurationSecretWithAnnotations("cilium", "Direct", "Netfilter", map[string]string{
			"network.deckhouse.io/cni-configuration-source-priority": "CustomValue",
		})
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniCilium.internal.mode", "VXLAN")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			f.ConfigValuesSet("cniCilium.masqueradeMode", "BPF")
			f.RunHook()
		})
		It("should treat non-ModuleConfig value as Secret priority", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("Netfilter"))
		})
	})
})

func generateCniConfigurationSecret(cni string, mode string, masqueradeMode string) string {
	return generateCniConfigurationSecretWithAnnotations(cni, mode, masqueradeMode, nil)
}

func generateCniConfigurationSecretWithAnnotations(cni string, mode string, masqueradeMode string, annotations map[string]string) string {
	var (
		secretTemplate = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cni-configuration
  namespace: kube-system`
	)

	if len(annotations) > 0 {
		secretTemplate += "\n  annotations:"
		for key, value := range annotations {
			secretTemplate += fmt.Sprintf("\n    %s: %s", key, value)
		}
	}

	secretTemplate += "\ntype: Opaque"

	jsonByte, _ := generateJSONCiliumConf(mode, masqueradeMode)
	secretTemplate = fmt.Sprintf("%s\ndata:\n  cni: %s", secretTemplate, base64.StdEncoding.EncodeToString([]byte(cni)))
	if mode != "" {
		secretTemplate = fmt.Sprintf("%s\n  cilium: %s", secretTemplate, base64.StdEncoding.EncodeToString(jsonByte))
	}
	return secretTemplate
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
