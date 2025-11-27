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
	"encoding/json"
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

var _ = Describe("Modules :: cni-flannel :: hooks :: set_pod_network_mode", func() {

	const (
		initValuesString       = `{"cniFlannel":{"internal": {}}}`
		initConfigValuesString = `{"cniFlannel":{}}`
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
		rawSettings, _ := json.Marshal(settings)

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
				Settings: &v1alpha1.SettingsValues{Raw: rawSettings},
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
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be `host-gw`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("Secret is present, but cni != `flannel`", func() {
		BeforeEach(func() {
			resources := []string{
				cniSecretYAML("cilium", "", nil, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be `host-gw`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("Secret is present, cni == `flannel`, but flannel field is not set", func() {
		BeforeEach(func() {
			resources := []string{
				cniSecretYAML(cni, "", nil, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be `host-gw`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("Secret is present, cni = `flannel`, flannel mode = vxlan", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "vxlan"}`, nil, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be set to `vxlan`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("vxlan"))
		})
	})

	Context("Secret is present, cni = `flannel`, flannel mode = host-gw", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "host-gw"}`, nil, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be set to `host-gw`", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("Secret is absent, MC is present: podNetworkMode set to `VXLAN`", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")
			resources := []string{
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "VXLAN",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, mode should be changed to vxlan", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("vxlan"))
		})
	})

	Context("Secret is absent, MC is present: podNetworkMode set to `HostGW`", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "HostGW")
			resources := []string{
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "HostGW",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, mode should be changed to host-gw", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("Secret is present, MC is present and has priority", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "HostGW")
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "vxlan"}`, nil, map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "ModuleConfig",
				}),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "HostGW",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be `host-gw` from MC, not secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("Secret is present and has priority (annotation=Secret), MC is present", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "HostGW")
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "vxlan"}`, nil, map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "Secret",
				}),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "HostGW",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be `vxlan` from secret, not MC", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("vxlan"))
		})
	})

	Context("Secret is present and has priority (annotation=Secret), MC is present 2", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "host-gw"}`, nil, map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "Secret",
				}),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "VXLAN",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should use secret values even when MC differs", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("Secret is present and has priority (annotation=CustomValue), MC is present", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "host-gw"}`, nil, map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "CustomValue",
				}),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "VXLAN",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should treat non-ModuleConfig value as Secret priority", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("Secret and MC are present, priority annotation is absent, cluster is not bootstrapped", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", false)
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "HostGW")
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "vxlan"}`, nil, nil),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "HostGW",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be from MC (host-gw), not secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("Secret and MC are present, priority annotation is absent, cluster is bootstrapped", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "HostGW")
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "vxlan"}`, nil, nil),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "HostGW",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, flannel mode should be from secret (vxlan), not MC", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("vxlan"))
		})
	})

	Context("Secret is absent, MC is present but has empty podNetworkMode", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")
			resources := []string{
				cniMCYAML(cniName, ptr.To(true), map[string]any{}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, mode should fallback to config values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("vxlan"))
		})
	})

	Context("Secret is absent, MC is present with unsupported podNetworkMode value", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")
			resources := []string{
				cniMCYAML("cni-flannel", ptr.To(true), map[string]any{
					"podNetworkMode": "UnsupportedMode",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, mode should fallback to config values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("vxlan"))
		})
	})

	Context("Secret is present with empty podNetworkMode, MC is present and has priority", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "HostGW")
			resources := []string{
				cniSecretYAML("flannel", `{"podNetworkMode": ""}`, nil, map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "ModuleConfig",
				}),
				cniMCYAML("cni-flannel", ptr.To(true), map[string]any{
					"podNetworkMode": "VXLAN",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, mode should be from MC when secret is empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("vxlan"))
		})
	})

	Context("Secret is present with empty podNetworkMode, MC is present has priority but is empty too", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "HostGW")
			resources := []string{
				cniSecretYAML("flannel", `{"podNetworkMode": ""}`, nil, map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "ModuleConfig",
				}),
				cniMCYAML("cni-flannel", ptr.To(true), map[string]any{}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, mode should fallback to config when both secret and MC are empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

	Context("Secret is absent, MC is absent", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("hook should run successfully, mode should remain unchanged when no sources exist", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniFlannel.internal.podNetworkMode").String()).To(Equal("host-gw"))
		})
	})

})
