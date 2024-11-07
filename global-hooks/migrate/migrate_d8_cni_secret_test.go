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

	"github.com/flant/addon-operator/sdk"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: migrate_d8_cni_secret ::", func() {
	createFakeCNISecret := func(name, data string) {
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

		_, err := dependency.TestDC.MustGetK8sClient().CoreV1().Secrets("kube-system").Create(context.TODO(), s, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
	}

	createCNIModuleConfig := func(name string, enabled *bool, settings config.SettingsValues) {
		mc := config.ModuleConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1alpha1",
				Kind:       "ModuleConfig",
			},

			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},

			Spec: config.ModuleConfigSpec{
				Version:  1,
				Settings: settings,
				Enabled:  enabled,
			},
		}

		mcu, err := sdk.ToUnstructured(&mc)
		if err != nil {
			panic(err)
		}

		_, err = dependency.TestDC.MustGetK8sClient().Dynamic().Resource(config.ModuleConfigGVR).Create(context.TODO(), mcu, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
	}

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

	Context("Cluster has ModuleConfig cni-flannel with disabled and ModuleConfig cni-cilium enabled and settings", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createCNIModuleConfig("cni-flannel", pointer.Bool(false), nil)
			createCNIModuleConfig("cni-cilium", pointer.Bool(true), config.SettingsValues{
				"tunnelMode":     "VXLAN",
				"masqueradeMode": "Netfilter",
			})
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Module configs should not changed", func() {
			mc := f.KubernetesResource("ModuleConfig", "", "cni-cilium")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
			Expect(mc.Field("spec.settings.tunnelMode").String()).To(Equal("VXLAN"))
			Expect(mc.Field("spec.settings.masqueradeMode").String()).To(Equal("Netfilter"))
			Expect(mc.Field("spec.settings.createNodeRoutes").Bool()).To(BeFalse())

			mc = f.KubernetesResource("ModuleConfig", "", "cni-flannel")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.enabled").Bool()).To(BeFalse())
		})
	})

	Context("Cluster has ModuleConfig cni-flannel cni-cilium enabled and settings and cni configuration secret with simple bridge", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			createFakeCNISecret("simple-bridge", "")
			createCNIModuleConfig("cni-cilium", pointer.Bool(true), config.SettingsValues{
				"tunnelMode":     "VXLAN",
				"masqueradeMode": "Netfilter",
			})
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("ModuleConfig cni-cilium should not changed, ModuleConfig cni-simple-bridge should not created", func() {
			mc := f.KubernetesResource("ModuleConfig", "", "cni-cilium")
			Expect(mc.Exists()).To(BeTrue())
			Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
			Expect(mc.Field("spec.settings.tunnelMode").String()).To(Equal("VXLAN"))
			Expect(mc.Field("spec.settings.masqueradeMode").String()).To(Equal("Netfilter"))
			Expect(mc.Field("spec.settings.createNodeRoutes").Bool()).To(BeFalse())

			mc = f.KubernetesResource("ModuleConfig", "", "cni-simple-bridge")
			Expect(mc.Exists()).To(BeFalse())
		})
	})

	Context("CNI Simple Bridge", func() {
		Context("Cluster has ModuleConfig enabled", func() {
			Context("CNI secret does not exist", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(``))
					createCNIModuleConfig("cni-simple-bridge", pointer.Bool(true), nil)
					f.RunHook()
				})

				It("Should run successfully", func() {
					Expect(f).To(ExecuteSuccessfully())
				})

				It("MC cni-simple-bridge should be not changed, d8-cni-configuration secret should be exist", func() {
					mc := f.KubernetesResource("ModuleConfig", "", "cni-simple-bridge")
					Expect(mc.Exists()).To(BeTrue())
					Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
					secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
					Expect(secret.Exists()).To(BeFalse())
				})
			})

			Context("CNI secret exists", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(``))
					createCNIModuleConfig("cni-simple-bridge", pointer.Bool(true), nil)
					createFakeCNISecret("simple-bridge", "")
					f.RunHook()
				})

				It("Should run successfully", func() {
					Expect(f).To(ExecuteSuccessfully())
				})

				It("MC cni-simple-bridge should be not changed, d8-cni-configuration secret should be removed", func() {
					mc := f.KubernetesResource("ModuleConfig", "", "cni-simple-bridge")
					Expect(mc.Exists()).To(BeTrue())
					Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
					secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
					Expect(secret.Exists()).To(BeFalse())
				})
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
	})

	Context("CNI Flannel", func() {
		Context("Cluster has ModuleConfig with enabled only and has configuration secret", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(``))
				createCNIModuleConfig("cni-flannel", pointer.Bool(true), nil)
				createFakeCNISecret("flannel", `{"podNetworkMode": "VXLAN"}`)
				f.RunHook()
			})

			It("Should run successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("MC cni-flannel should be updated, d8-cni-configuration secret should be removed", func() {
				mc := f.KubernetesResource("ModuleConfig", "", "cni-flannel")
				Expect(mc.Exists()).To(BeTrue())
				Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
				Expect(mc.Field("spec.settings.podNetworkMode").String()).To(Equal("VXLAN"))
				secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
				Expect(secret.Exists()).To(BeFalse())
			})
		})

		Context("Cluster has ModuleConfig with enabled and set settings and has configuration secret", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(``))
				createCNIModuleConfig("cni-flannel", pointer.Bool(true), config.SettingsValues{
					"podNetworkMode": "HostGW",
				})
				createFakeCNISecret("flannel", `{"podNetworkMode": "VXLAN"}`)
				f.RunHook()
			})

			It("Should run successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("MC cni-flannel should not be changed, d8-cni-configuration secret should be removed", func() {
				mc := f.KubernetesResource("ModuleConfig", "", "cni-flannel")
				Expect(mc.Exists()).To(BeTrue())
				Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
				Expect(mc.Field("spec.settings.podNetworkMode").String()).To(Equal("HostGW"))
				secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
				Expect(secret.Exists()).To(BeFalse())
			})
		})

		Context("Cluster has ModuleConfig with enabled and set settings and does not have configuration secret", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(``))
				createCNIModuleConfig("cni-flannel", pointer.Bool(true), config.SettingsValues{
					"podNetworkMode": "VXLAN",
				})
				f.RunHook()
			})

			It("Should run successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("MC cni-flannel should not be changed, d8-cni-configuration secret should not be created", func() {
				mc := f.KubernetesResource("ModuleConfig", "", "cni-flannel")
				Expect(mc.Exists()).To(BeTrue())
				Expect(mc.Field("spec.enabled").Bool()).To(BeTrue())
				Expect(mc.Field("spec.settings.podNetworkMode").String()).To(Equal("VXLAN"))
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
	})

	Context("CNI Cilium", func() {
		Context("Cluster has ModuleConfig cni-cilium with enabled only and has cni configuration ", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(``))
				createCNIModuleConfig("cni-cilium", pointer.Bool(true), nil)
				createFakeCNISecret("cilium", `{"mode": "VXLAN", "masqueradeMode": "BPF"}`)
				f.RunHook()
			})

			It("Should run successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("MC cni-cilium should be updated, d8-cni-configuration secret should be removed", func() {
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

		Context("Cluster has ModuleConfig cni-cilium with enabled and settings and has cni configuration ", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(``))
				createCNIModuleConfig("cni-cilium", pointer.Bool(true), config.SettingsValues{
					"tunnelMode":     "VXLAN",
					"masqueradeMode": "Netfilter",
				})
				createFakeCNISecret("cilium", `{"mode": "VXLAN", "masqueradeMode": "BPF"}`)
				f.RunHook()
			})

			It("Should run successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("MC cni-cilium should not be updated, d8-cni-configuration secret should be removed", func() {
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
		})

		Context("Cluster has d8-cni-configuration secret for cni-cilium (not set, not set)", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(``))
				createFakeCNISecret("cilium", "")
				f.RunHook()
			})

			It("Should not run successfully", func() {
				Expect(f).NotTo(ExecuteSuccessfully())
			})
		})
	})
})
