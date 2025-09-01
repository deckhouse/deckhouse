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
	"encoding/base64"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cni-flannel :: hooks :: set_pod_network_mode", func() {
	f := HookExecutionConfigInit(`{"cniFlannel":{"internal":{}}}`, "")

	Context("fresh cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniFlannel.internal.podNetworkMode", "host-gw")
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be `host-gw`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("kube-system/d8-cni-configuration is present, but cni != `flannel`", func() {
		cniSecret := generateCniConfigurationSecret("cilium", "")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniFlannel.internal.podNetworkMode", "host-gw")
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be `host-gw`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("kube-system/d8-cni-configuration is present, cni == `flannel`, but flannel field is not set", func() {
		cniSecret := generateCniConfigurationSecret("flannel", "")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniFlannel.internal.podNetworkMode", "host-gw")
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be `host-gw`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("kube-system/d8-cni-configuration is present, cni = `flannel`, flannel mode = vxlan", func() {
		cniSecret := generateCniConfigurationSecret("flannel", "vxlan")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ValuesSet("cniFlannel.internal.podNetworkMode", "host-gw")
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be set to `vxlan`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("vxlan"))
		})
	})

	Context("kube-system/d8-cni-configuration is present, cni = `flannel`, flannel mode = host-gw", func() {
		cniSecret := generateCniConfigurationSecret("flannel", "host-gw")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ValuesSet("cniFlannel.internal.podNetworkMode", "vxlan")
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be set to `host-gw`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("kube-system/d8-cni-configuration is absent, podNetworkMode set to `VXLAN`", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")
			f.ValuesSet("cniFlannel.internal.podNetworkMode", "host-gw")
			f.RunHook()
		})
		It("hook should run successfully, mode should be changed to vxlan", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("vxlan"))
		})
	})

	Context("kube-system/d8-cni-configuration is absent, podNetworkMode set to `HostGW`", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "HostGW")
			f.ValuesSet("cniFlannel.internal.podNetworkMode", "vxlan")
			f.RunHook()
		})
		It("hook should run successfully, mode should be changed to host-gw", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("kube-system/d8-cni-configuration with annotation network.deckhouse.io/cni-configuration-source-priority=ModuleConfig, flannel mode = vxlan", func() {
		cniSecret := generateCniConfigurationSecretWithAnnotations("flannel", "vxlan", map[string]string{
			"network.deckhouse.io/cni-configuration-source-priority": "ModuleConfig",
		})
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniFlannel.internal.podNetworkMode", "host-gw")
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "HostGW")
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be `host-gw` from MC, not secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("kube-system/d8-cni-configuration with annotation network.deckhouse.io/cni-configuration-source-priority=Secret, flannel mode = vxlan", func() {
		cniSecret := generateCniConfigurationSecretWithAnnotations("flannel", "vxlan", map[string]string{
			"network.deckhouse.io/cni-configuration-source-priority": "Secret",
		})
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniFlannel.internal.podNetworkMode", "host-gw")
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "HostGW")
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be `vxlan` from secret, not MC", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("vxlan"))
		})
	})

	Context("kube-system/d8-cni-configuration without annotation, cluster is not bootstrapped", func() {
		cniSecret := generateCniConfigurationSecret("flannel", "vxlan")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("global.clusterIsBootstrapped", false)
			f.ValuesSet("cniFlannel.internal.podNetworkMode", "host-gw")
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "HostGW")
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be from MC (host-gw), not secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("kube-system/d8-cni-configuration without annotation, cluster is bootstrapped", func() {
		cniSecret := generateCniConfigurationSecret("flannel", "vxlan")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ValuesSet("cniFlannel.internal.podNetworkMode", "host-gw")
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "HostGW")
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be from secret (vxlan), not MC", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("vxlan"))
		})
	})

	Context("Priority annotation with value Secret", func() {
		cniSecret := generateCniConfigurationSecretWithAnnotations("flannel", "host-gw", map[string]string{
			"network.deckhouse.io/cni-configuration-source-priority": "Secret",
		})
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniFlannel.internal.podNetworkMode", "vxlan")
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")
			f.RunHook()
		})
		It("should use secret values even when MC differs", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("Priority annotation with non-standard value", func() {
		cniSecret := generateCniConfigurationSecretWithAnnotations("flannel", "host-gw", map[string]string{
			"network.deckhouse.io/cni-configuration-source-priority": "CustomValue",
		})
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("cniFlannel.internal.podNetworkMode", "vxlan")
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")
			f.RunHook()
		})
		It("should treat non-ModuleConfig value as Secret priority", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})
})

func generateCniConfigurationSecret(cni string, podNetworkMode string) string {
	return generateCniConfigurationSecretWithAnnotations(cni, podNetworkMode, nil)
}

func generateCniConfigurationSecretWithAnnotations(cni string, podNetworkMode string, annotations map[string]string) string {
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

	jsonByte, _ := generateJSONFlannelConf(podNetworkMode)
	secretTemplate = fmt.Sprintf("%s\ndata:\n  cni: %s", secretTemplate, base64.StdEncoding.EncodeToString([]byte(cni)))
	if podNetworkMode != "" {
		secretTemplate = fmt.Sprintf("%s\n  flannel: %s", secretTemplate, base64.StdEncoding.EncodeToString(jsonByte))
	}
	return secretTemplate
}

func generateJSONFlannelConf(podNetworkMode string) ([]byte, error) {
	var confMAP FlannelConfigStruct
	if podNetworkMode != "" {
		confMAP.PodNetworkMode = podNetworkMode
	}

	return json.Marshal(confMAP)
}
