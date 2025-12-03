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
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cni-cilium :: hooks :: set_cilium_mode", func() {

	const (
		initValuesString       = `{"cniCilium":{"internal": {}}}`
		initConfigValuesString = `{"cniCilium":{}}`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	cniSecretYAML := func(cniName, data string, creationTime *time.Time, annotations map[string]string) string {
		secretData := make(map[string][]byte)
		secretData["cni"] = []byte(cniName)
		if data != "" {
			secretData[cniName] = []byte(data)
		}
		s := &v1core.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        "d8-cni-configuration",
				Namespace:   "kube-system",
				Annotations: annotations,
			},
			Data: secretData,
		}
		if creationTime != nil {
			s.ObjectMeta.CreationTimestamp = metav1.NewTime(*creationTime)
		}
		marshaled, _ := yaml.Marshal(s)
		return string(marshaled)
	}
	cniMCYAML := func(cniName string, enabled *bool, settings map[string]any, creationTime *time.Time) string {
		mc := &v1alpha1.ModuleConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "deckhouse.io/v1alpha1",
				Kind:       "ModuleConfig",
			},

			ObjectMeta: metav1.ObjectMeta{
				Name: cniName,
			},

			Spec: v1alpha1.ModuleConfigSpec{
				Version:  1,
				Settings: v1alpha1.MakeMappedFields(settings),
				Enabled:  enabled,
			},
		}
		if creationTime != nil {
			mc.ObjectMeta.CreationTimestamp = metav1.NewTime(*creationTime)
		}
		marshaled, _ := yaml.Marshal(mc)
		return string(marshaled)
	}

	Context("fresh cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Secret is present, but cni != `cilium`", func() {
		BeforeEach(func() {
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			resources := []string{
				cniSecretYAML("flannel", "", nil, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Secret is present, cni == `cilium`, but cilium field is not set", func() {
		BeforeEach(func() {
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			resources := []string{
				cniSecretYAML(cni, "", nil, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Secret is present, cni == `cilium`, cilium mode == VXLAN", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			resources := []string{
				cniSecretYAML(cni, `{"mode": "VXLAN"}`, nil, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be set to `VXLAN`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Secret is present, cni == `cilium`, cilium mode == DirectWithNodeRoutes, masqueradeMode == Netfilter", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			resources := []string{
				cniSecretYAML(cni, `{"mode": "DirectWithNodeRoutes", "masqueradeMode": "Netfilter"}`, nil, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())

			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be set to `DirectWithNodeRoutes`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("Netfilter"))
		})
	})

	Context("Secret is absent, MC is present: tunnelMode set to `VXLAN`", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			resources := []string{
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"tunnelMode": "VXLAN",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, secret should be changed", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Secret is absent, MC is present: masqueradeMode set to `Netfilter`, tunnelMode set to `VXLAN`", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			f.ConfigValuesSet("cniCilium.masqueradeMode", "Netfilter")
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			resources := []string{
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"tunnelMode":     "VXLAN",
					"masqueradeMode": "Netfilter",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, secret should be changed", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("Netfilter"))
		})
	})

	Context("Secret is absent, MC is present: tunnelMode set to `Disabled`, but previously the mode was discovered to `VXLAN`", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniCilium.tunnelMode", "Disabled")
			f.ValuesSet("cniCilium.internal.mode", "VXLAN")
			resources := []string{
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"tunnelMode": "Disabled",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, mode must be changed to Direct", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
		})
	})

	Context("Secret is absent, MC is present: createNodeRoutes set to `true`", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniCilium.createNodeRoutes", true)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			resources := []string{
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"createNodeRoutes": true,
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `DirectWithNodeRoutes`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Secret is absent, MC is present: createNodeRoutes set to `false`", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniCilium.createNodeRoutes", false)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			resources := []string{
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"createNodeRoutes": false,
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Static(Secret is absent), MC is absent", func() {
		BeforeEach(func() {
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
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `DirectWithNodeRoutes`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Not Static, Secret is absent, MC is absent", func() {
		BeforeEach(func() {
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
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `Direct`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Static(Secret is absent), MC is present: tunnelMode == VXLAN and masqueradeMode == Netfilter", func() {
		BeforeEach(func() {
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
			resources := []string{
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"tunnelMode":     "VXLAN",
					"masqueradeMode": "Netfilter",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `VXLAN` and masqueradeMode is `Netfilter`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("Netfilter"))
		})
	})

	Context("Static(Secret is absent), MC is present: createNodeRoutes == false", func() {
		BeforeEach(func() {
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
			f.ConfigValuesSet("cniCilium.createNodeRoutes", false)
			resources := []string{
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"createNodeRoutes": false,
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `DirectWithNodeRoutes`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Static(Secret is absent), MC is present: tunnelMode == Disabled", func() {
		BeforeEach(func() {
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
			resources := []string{
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"tunnelMode": "Disabled",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should use DirectWithNodeRoutes for Static cluster, overriding tunnelMode", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Secret is present, MC is present and has priority, merge test 0", func() {
		BeforeEach(func() {
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.ConfigValuesSet("cniCilium.tunnelMode", "Disabled")
			f.ConfigValuesSet("cniCilium.createNodeRoutes", true)
			resources := []string{
				cniSecretYAML(cni, `{"mode": "VXLAN", "masqueradeMode": "BPF"}`, nil, map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "ModuleConfig",
				}),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"tunnelMode":       "Disabled",
					"createNodeRoutes": true,
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `DirectWithNodeRoutes` from MC, not secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Secret is present, MC is present and has priority, merge test 1", func() {
		BeforeEach(func() {
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.ConfigValuesSet("cniCilium.masqueradeMode", "Netfilter")
			resources := []string{
				cniSecretYAML(cni, `{"mode": "VXLAN", "masqueradeMode": "BPF"}`, nil, map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "ModuleConfig",
				}),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"masqueradeMode": "Netfilter",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should take masqueradeMode from MC and mode from secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("Netfilter"))
		})
	})

	Context("Secret is present, MC is present and has priority, merge test 2 (tunnelMode VXLAN should return early)", func() {
		BeforeEach(func() {
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			resources := []string{
				cniSecretYAML(cni, `{"mode": "DirectWithNodeRoutes"}`, nil, map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "ModuleConfig",
				}),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"tunnelMode": "VXLAN",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should set mode to VXLAN and return early (not process createNodeRoutes)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Secret is present, MC is present and has priority, merge test 3 (MC settings is empty)", func() {
		BeforeEach(func() {
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			resources := []string{
				cniSecretYAML(cni, `{"mode": "DirectWithNodeRoutes"}`, nil, map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "ModuleConfig",
				}),
				cniMCYAML(cniName, ptr.To(true), map[string]any{}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should use secret mode when MC is empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Secret is present, MC is present and has priority, merge test 4 (MC and Secret settings is empty)", func() {
		BeforeEach(func() {
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			resources := []string{
				cniSecretYAML(cni, `{}`, nil, map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "ModuleConfig",
				}),
				cniMCYAML(cniName, ptr.To(true), map[string]any{}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should use config VXLAN when MC and secret are empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("BPF"))
		})
	})

	Context("Secret is present and has priority, MC is present", func() {
		BeforeEach(func() {
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.ConfigValuesSet("cniCilium.tunnelMode", "Disabled")
			f.ConfigValuesSet("cniCilium.createNodeRoutes", true)
			resources := []string{
				cniSecretYAML(cni, `{"mode": "VXLAN", "masqueradeMode": "Netfilter"}`, nil, map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "Secret",
				}),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"tunnelMode":       "Disabled",
					"createNodeRoutes": true,
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be `VXLAN` from secret, not MC", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("Netfilter"))
		})
	})

	Context("Secret is present and has priority (annotation=Secret), MC is present", func() {
		BeforeEach(func() {
			f.ValuesSet("cniCilium.internal.mode", "VXLAN")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			f.ConfigValuesSet("cniCilium.masqueradeMode", "BPF")
			resources := []string{
				cniSecretYAML(cni, `{"mode": "Direct", "masqueradeMode": "Netfilter"}`, nil, map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "Secret",
				}),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"tunnelMode":     "VXLAN",
					"masqueradeMode": "BPF",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should use secret values even when MC differs", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("Netfilter"))
		})
	})

	Context("Secret is present and has priority (annotation=CustomValue), MC is present", func() {
		BeforeEach(func() {
			f.ValuesSet("cniCilium.internal.mode", "VXLAN")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			f.ConfigValuesSet("cniCilium.masqueradeMode", "BPF")
			resources := []string{
				cniSecretYAML(cni, `{"mode": "Direct", "masqueradeMode": "Netfilter"}`, nil, map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "CustomValue",
				}),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"tunnelMode":     "VXLAN",
					"masqueradeMode": "BPF",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should treat non-ModuleConfig value as Secret priority", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("Direct"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("Netfilter"))
		})
	})

	Context("Secret and MC are present, priority annotation is absent, cluster is not bootstrapped", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", false)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.ConfigValuesSet("cniCilium.tunnelMode", "Disabled")
			f.ConfigValuesSet("cniCilium.createNodeRoutes", true)
			resources := []string{
				cniSecretYAML(cni, `{"mode": "VXLAN", "masqueradeMode": "Netfilter"}`, nil, nil),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"tunnelMode":       "Disabled",
					"createNodeRoutes": true,
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be from MC (DirectWithNodeRoutes), masqueradeMode from secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("DirectWithNodeRoutes"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("Netfilter"))
		})
	})

	Context("Secret and MC are present, priority annotation is absent, cluster is bootstrapped", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ValuesSet("cniCilium.internal.mode", "Direct")
			f.ValuesSet("cniCilium.internal.masqueradeMode", "BPF")
			f.ConfigValuesSet("cniCilium.tunnelMode", "Disabled")
			f.ConfigValuesSet("cniCilium.createNodeRoutes", true)
			resources := []string{
				cniSecretYAML(cni, `{"mode": "VXLAN", "masqueradeMode": "Netfilter"}`, nil, nil),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"tunnelMode":       "Disabled",
					"createNodeRoutes": true,
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, cilium mode should be from secret (VXLAN), not MC", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.mode").String()).To(Equal("VXLAN"))
			Expect(f.ValuesGet("cniCilium.internal.masqueradeMode").String()).To(Equal("Netfilter"))
		})
	})

})
