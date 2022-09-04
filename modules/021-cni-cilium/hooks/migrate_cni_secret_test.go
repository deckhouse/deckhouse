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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func generateCniConfigurationSecret(cni string, mode string) string {
	var (
		secretTemplate = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cni-configuration
  namespace: kube-system
type: Opaque`
	)

	secretTemplate = fmt.Sprintf("%s\ndata:\n  cni: %s", secretTemplate, base64.StdEncoding.EncodeToString([]byte(cni)))
	if mode != "" {
		secretTemplate = fmt.Sprintf("%s\n  cilium: %s", secretTemplate, base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("{\"mode\": \"%s\"}", mode))))
	}
	return secretTemplate
}

var _ = Describe("Modules :: cni-cilium :: hooks :: migrate_cni_secret", func() {
	f := HookExecutionConfigInit(
		`{}`,
		`{"cniCilium":{}}`,
	)
	Context("fresh cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, skip migration", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration").Exists()).To(BeFalse())
		})
	})

	Context("kube-system/d8-cni-configuration is present, but cni != `cilium`, skip migration", func() {
		cniSecret := generateCniConfigurationSecret("flannel", "")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, secret should be unchanged", func() {
			Expect(f).To(ExecuteSuccessfully())
			s := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(s.ToYaml()).To(MatchYAML(cniSecret))
		})
	})

	Context("kube-system/d8-cni-configuration is present, cni = `cilium`, but cilium field is present, skip migration", func() {
		cniSecret := generateCniConfigurationSecret("cilium", "Direct")
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, secret should be unchanged", func() {
			Expect(f).To(ExecuteSuccessfully())
			s := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(s.ToYaml()).To(MatchYAML(cniSecret))
		})
	})

	Context("kube-system/d8-cni-configuration is present, cni = `cilium`, cilium field is absent, tunnelMode = VXLAN", func() {
		BeforeEach(func() {
			f.KubeStateSet(generateCniConfigurationSecret("cilium", ""))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			f.RunHook()
		})
		It("hook should run successfully, secret should be changed", func() {
			Expect(f).To(ExecuteSuccessfully())
			s := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(s.ToYaml()).To(MatchYAML(generateCniConfigurationSecret("cilium", "VXLAN")))
		})
	})

	Context("kube-system/d8-cni-configuration is present, cni = `cilium`, cilium field is absent, createNodeRoutes = true", func() {
		BeforeEach(func() {
			f.KubeStateSet(generateCniConfigurationSecret("cilium", ""))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("cniCilium.createNodeRoutes", true)
			f.RunHook()
		})
		It("hook should run successfully, secret should be changed", func() {
			Expect(f).To(ExecuteSuccessfully())
			s := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(s.ToYaml()).To(MatchYAML(generateCniConfigurationSecret("cilium", "DirectWithNodeRoutes")))
		})
	})

	Context("kube-system/d8-cni-configuration is present, cni = `cilium`, cilium field is absent, createNodeRoutes = false", func() {
		BeforeEach(func() {
			f.KubeStateSet(generateCniConfigurationSecret("cilium", ""))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("cniCilium.createNodeRoutes", false)
			f.RunHook()
		})
		It("hook should run successfully, secret should be changed", func() {
			Expect(f).To(ExecuteSuccessfully())
			s := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(s.ToYaml()).To(MatchYAML(generateCniConfigurationSecret("cilium", "Direct")))
		})
	})

	f = HookExecutionConfigInit(`{}`, `{}`)

	Context("kube-system/d8-cni-configuration is present, cni = `cilium`, cilium field is absent, createNodeRoutes is absent, cloud-provider = openstack", func() {
		BeforeEach(func() {
			f.KubeStateSet(generateCniConfigurationSecret("cilium", ""))
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
			f.RunHook()
		})
		It("hook should run successfully, secret should be changed", func() {
			Expect(f).To(ExecuteSuccessfully())
			s := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(s.ToYaml()).To(MatchYAML(generateCniConfigurationSecret("cilium", "DirectWithNodeRoutes")))
		})
	})

	Context("kube-system/d8-cni-configuration is present, cni = `cilium`, cilium field is absent, createNodeRoutes is absent, cloud-provider = vsphere", func() {
		BeforeEach(func() {
			f.KubeStateSet(generateCniConfigurationSecret("cilium", ""))
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
			f.RunHook()
		})
		It("hook should run successfully, secret should be changed", func() {
			Expect(f).To(ExecuteSuccessfully())
			s := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(s.ToYaml()).To(MatchYAML(generateCniConfigurationSecret("cilium", "DirectWithNodeRoutes")))
		})
	})

	Context("kube-system/d8-cni-configuration is present, cni = `cilium`, cilium field is absent, createNodeRoutes is absent, cloud-provider nor vsphere nor openstack", func() {
		BeforeEach(func() {
			f.KubeStateSet(generateCniConfigurationSecret("cilium", ""))
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
			f.RunHook()
		})
		It("hook should run successfully, secret should be changed", func() {
			Expect(f).To(ExecuteSuccessfully())
			s := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(s.ToYaml()).To(MatchYAML(generateCniConfigurationSecret("cilium", "Direct")))
		})
	})

})
