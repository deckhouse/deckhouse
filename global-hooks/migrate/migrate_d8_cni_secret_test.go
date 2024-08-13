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
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func createFakeCNISecret(name, data string) {
	secretData := make(map[string][]byte)
	secretData["cni"] = []byte(name)
	if data != "" {
		secretData[name] = []byte(data)
	}

	s := &v1core.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},

		ObjectMeta: metav1.ObjectMeta{
			Name:      "d8-cni-configuration",
			Namespace: "kube-system",
		},

		Data: secretData,
	}

	_, _ = dependency.TestDC.MustGetK8sClient().CoreV1().Secrets("kube-system").Create(context.TODO(), s, metav1.CreateOptions{})
}

var _ = Describe("Global hooks :: migrate_d8_cni_secret ::", func() {
	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("Cluster has no d8-cni-configuration secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster has d8-cni-configuration secret for cni-simple-bridge", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createFakeCNISecret("simple-bridge", "")
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("MC cni-simple-bridge should be created, d8-cni-configuration secret should be removed", func() {
			mc := f.KubernetesResource("ModuleConfig", "", "cni-simple-bridge")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeFalse())
		})
	})

	Context("Cluster has d8-cni-configuration secret for cni-flannel (VXLAN mode)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createFakeCNISecret("flannel", `{"podNetworkMode": "VXLAN"}`)
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("MC cni-flannel should be created, d8-cni-configuration secret should be removed", func() {
			mc := f.KubernetesResource("ModuleConfig", "", "cni-flannel")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
			Expect(mc.Field("spec.settings.podNetworkMode").String()).To(Equal("VXLAN"))
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeFalse())
		})
	})

	Context("Cluster has d8-cni-configuration secret for cni-flannel (HostGW mode)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createFakeCNISecret("flannel", `{"podNetworkMode": "HostGW"}`)
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("MC cni-flannel should be created, d8-cni-configuration secret should be removed", func() {
			mc := f.KubernetesResource("ModuleConfig", "", "cni-flannel")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
			Expect(mc.Field("spec.settings.podNetworkMode").String()).To(Equal("HostGW"))
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeFalse())
		})
	})

	Context("Cluster has d8-cni-configuration secret for cni-flannel (mode doesn't set)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createFakeCNISecret("flannel", "")
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("MC cni-flannel should be created, d8-cni-configuration secret should be removed", func() {
			mc := f.KubernetesResource("ModuleConfig", "", "cni-flannel")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
			Expect(mc.Field("spec.settings.podNetworkMode").String()).To(Equal("HostGW"))
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeFalse())
		})

	})

	Context("Cluster has d8-cni-configuration secret for cni-cilium (VXLAN, BPF)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createFakeCNISecret("cilium", `{"mode": "VXLAN", "masqueradeMode": "BPF"}`)
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("MC cni-cilium should be created, d8-cni-configuration secret should be removed", func() {
			mc := f.KubernetesResource("ModuleConfig", "", "cni-cilium")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
			Expect(mc.Field("spec.settings.tunnelMode").String()).To(Equal("VXLAN"))
			Expect(mc.Field("spec.settings.masqueradeMode").String()).To(Equal("BPF"))
			Expect(mc.Field("spec.settings.createNodeRoutes").Bool()).To(BeFalse())

			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeFalse())
		})
	})

	Context("Cluster has d8-cni-configuration secret for cni-cilium (VXLAN, Netfilter)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createFakeCNISecret("cilium", `{"mode": "VXLAN", "masqueradeMode": "Netfilter"}`)
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("MC cni-cilium should be created, d8-cni-configuration secret should be removed", func() {
			mc := f.KubernetesResource("ModuleConfig", "", "cni-cilium")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
			Expect(mc.Field("spec.settings.tunnelMode").String()).To(Equal("VXLAN"))
			Expect(mc.Field("spec.settings.masqueradeMode").String()).To(Equal("Netfilter"))
			Expect(mc.Field("spec.settings.createNodeRoutes").Bool()).To(BeFalse())

			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeFalse())
		})
	})

	Context("Cluster has d8-cni-configuration secret for cni-cilium (Direct, BPF)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createFakeCNISecret("cilium", `{"mode": "Direct", "masqueradeMode": "BPF"}`)
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("MC cni-cilium should be created, d8-cni-configuration secret should be removed", func() {
			mc := f.KubernetesResource("ModuleConfig", "", "cni-cilium")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
			Expect(mc.Field("spec.settings.tunnelMode").String()).To(Equal("Disabled"))
			Expect(mc.Field("spec.settings.masqueradeMode").String()).To(Equal("BPF"))
			Expect(mc.Field("spec.settings.createNodeRoutes").Bool()).To(BeFalse())

			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeFalse())
		})
	})

	Context("Cluster has d8-cni-configuration secret for cni-cilium (Direct, Netfilter)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createFakeCNISecret("cilium", `{"mode": "Direct", "masqueradeMode": "Netfilter"}`)
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("MC cni-cilium should be created, d8-cni-configuration secret should be removed", func() {
			mc := f.KubernetesResource("ModuleConfig", "", "cni-cilium")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
			Expect(mc.Field("spec.settings.tunnelMode").String()).To(Equal("Disabled"))
			Expect(mc.Field("spec.settings.masqueradeMode").String()).To(Equal("Netfilter"))
			Expect(mc.Field("spec.settings.createNodeRoutes").Bool()).To(BeFalse())

			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeFalse())
		})
	})

	Context("Cluster has d8-cni-configuration secret for cni-cilium (DirectWithNodeRoutes, BPF)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createFakeCNISecret("cilium", `{"mode": "DirectWithNodeRoutes", "masqueradeMode": "BPF"}`)
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("MC cni-cilium should be created, d8-cni-configuration secret should be removed", func() {
			mc := f.KubernetesResource("ModuleConfig", "", "cni-cilium")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
			Expect(mc.Field("spec.settings.tunnelMode").String()).To(Equal("Disabled"))
			Expect(mc.Field("spec.settings.masqueradeMode").String()).To(Equal("BPF"))
			Expect(mc.Field("spec.settings.createNodeRoutes").Bool()).To(BeTrue())

			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeFalse())
		})
	})

	Context("Cluster has d8-cni-configuration secret for cni-cilium (DirectWithNodeRoutes, Netfilter)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createFakeCNISecret("cilium", `{"mode": "DirectWithNodeRoutes", "masqueradeMode": "Netfilter"}`)
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("MC cni-cilium should be created, d8-cni-configuration secret should be removed", func() {
			mc := f.KubernetesResource("ModuleConfig", "", "cni-cilium")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
			Expect(mc.Field("spec.settings.tunnelMode").String()).To(Equal("Disabled"))
			Expect(mc.Field("spec.settings.masqueradeMode").String()).To(Equal("Netfilter"))
			Expect(mc.Field("spec.settings.createNodeRoutes").Bool()).To(BeTrue())

			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeFalse())
		})
	})

	Context("Cluster has d8-cni-configuration secret for cni-cilium (DirectWithNodeRoutes, not set)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createFakeCNISecret("cilium", `{"mode": "DirectWithNodeRoutes"}`)
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("MC cni-cilium should be created, d8-cni-configuration secret should be removed", func() {
			mc := f.KubernetesResource("ModuleConfig", "", "cni-cilium")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
			Expect(mc.Field("spec.settings.tunnelMode").String()).To(Equal("Disabled"))
			Expect(mc.Field("spec.settings.masqueradeMode").String()).To(BeEmpty())
			Expect(mc.Field("spec.settings.createNodeRoutes").Bool()).To(BeTrue())

			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeFalse())
		})
	})

	Context("Cluster has d8-cni-configuration secret for cni-cilium (not set, not set)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createFakeCNISecret("cilium", "")
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
		})
	})

})
