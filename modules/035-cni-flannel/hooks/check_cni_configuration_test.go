/*
Copyright 2024 Flant JSC

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
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func checkMetric(metrics []operation.MetricOperation, value float64) {
	Expect(metrics).To(HaveLen(2))
	Expect(metrics[0]).To(BeEquivalentTo(operation.MetricOperation{
		Group:  checkCNIConfigMetricGroup,
		Action: operation.ActionExpireMetrics,
	}))
	Expect(metrics[1].Name).To(BeEquivalentTo(checkCNIConfigMetricName))
	Expect(metrics[1].Group).To(BeEquivalentTo(checkCNIConfigMetricGroup))
	Expect(metrics[1].Action).To(BeEquivalentTo(operation.ActionGaugeSet))
	Expect(metrics[1].Value).To(BeEquivalentTo(ptr.To(value)))
	Expect(metrics[1].Labels).To(BeEquivalentTo(map[string]string{"cni": cniName}))
}

var _ = Describe("Modules :: cni-flannel :: hooks :: check_cni_configuration", func() {

	const (
		initValuesString       = `{"cniFlannel":{"internal": {}}}`
		initConfigValuesString = `{"cniFlannel":{}}`
		anotherCni             = "simple-bridge"
		foreignDesiredCM       = `
apiVersion: v1
kind: ConfigMap
metadata:
  creationTimestamp: null
  name: desired-cni-moduleconfig
  namespace: d8-system
data:
  cni-flannel-mc.yaml: |
    apiVersion: deckhouse.io/v1alpha1
    kind: ModuleConfig
    metadata:
      creationTimestamp: null
      name: cni-flannel
    spec:
      enabled: true
      settings:
        podNetworkMode: VXLAN
      version: 1
    status:
      message: ""
      version: ""
`
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
				Settings: v1alpha1.SettingsValues{Raw: rawSettings},
				Enabled:  enabled,
			},
		}
		if creationTime != nil {
			mc.ObjectMeta.CreationTimestamp = metav1.NewTime(*creationTime)
		}
		marshaled, _ := yaml.Marshal(mc)
		return string(marshaled)
	}

	Context("Cluster has not cni secret (and has not cni mc too)", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully, set req=true and metric=0 and should not create desired mc", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("true"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 0.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", desiredCNIModuleConfigName)
			Expect(cm.Exists()).To(BeFalse())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeFalse())
		})
	})

	Context("Cluster has cni secret but key `cni` does not equal `flannel`", func() {
		BeforeEach(func() {
			resources := []string{
				cniSecretYAML(anotherCni, "", nil, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully, set req=true and metric=0 and should not create desired mc", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("true"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 0.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", desiredCNIModuleConfigName)
			Expect(cm.Exists()).To(BeFalse())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations`).Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).String()).To(Equal("ModuleConfig"))
		})
	})

	Context("Cluster has cni secret with priority annotation", func() {
		BeforeEach(func() {
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "vxlan"}`, nil, map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "ModuleConfig",
				}),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully, set req=true and metric=0 and should not create desired mc", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("true"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 0.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", desiredCNIModuleConfigName)
			Expect(cm.Exists()).To(BeFalse())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations`).Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).String()).To(Equal("ModuleConfig"))
		})
	})

	Context("Cluster has cni secret, key `cni` eq `flannel`, but cni MC does not exist", func() {
		BeforeEach(func() {
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "vxlan"}`, nil, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully, set req=true and metric=0 and create MC directly", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("true"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 0.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", desiredCNIModuleConfigName)
			Expect(cm.Exists()).To(BeFalse())
			mc := f.KubernetesResource("ModuleConfig", "", cniName)
			Expect(mc.Exists()).To(BeTrue())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations`).Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).String()).To(Equal("ModuleConfig"))
		})
	})

	Context("Cluster has cni secret, key `cni` eq `flannel`, MC exists but explicitly disabled", func() {
		BeforeEach(func() {
			requirements.RemoveValue(cniConfigurationSettledKey)
			f.ValuesSet("global.clusterIsBootstrapped", true)
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "vxlan"}`, nil, nil),
				cniMCYAML(cniName, ptr.To(false), map[string]any{}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully and do nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			_, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeFalse())
			Expect(len(f.MetricsCollector.CollectedMetrics())).To(Equal(1)) // only expire
			cm := f.KubernetesResource("ConfigMap", "d8-system", desiredCNIModuleConfigName)
			Expect(cm.Exists()).To(BeFalse())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).Exists()).To(BeFalse())
		})
	})

	Context("Cluster is not bootstrapped and has cni secret with mismatched configuration", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", false)
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "host-gw"}`, nil, nil),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "VXLAN",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully, set req=true and metric=0 and should not create desired mc", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("true"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 0.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", desiredCNIModuleConfigName)
			Expect(cm.Exists()).To(BeFalse())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations`).Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).String()).To(Equal("ModuleConfig"))
		})
	})

	Context("Cluster has cni secret, key `cni` eq `flannel`, cni MC exist and enabled but secret key `flannel` does not exist", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")
			resources := []string{
				cniSecretYAML(cni, ``, nil, nil),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "VXLAN",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully, set req=true and metric=0 and should not create desired mc", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("true"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 0.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", desiredCNIModuleConfigName)
			Expect(cm.Exists()).To(BeFalse())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations`).Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).String()).To(Equal("ModuleConfig"))
		})
	})

	Context("Cluster has cni secret, key `cni` eq `flannel`, cni MC exist and enabled, secret key `flannel` exist but it is empty", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")
			resources := []string{
				cniSecretYAML(cni, `{}`, nil, nil),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "VXLAN",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully, set req=true and metric=0 and should not create desired mc", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("true"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 0.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", desiredCNIModuleConfigName)
			Expect(cm.Exists()).To(BeFalse())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations`).Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).String()).To(Equal("ModuleConfig"))
		})
	})

	Context("Cluster is bootstrapped, has cni secret, key `cni` eq `flannel`, cni MC exist and enabled, secret key `flannel` exist and not empty but some parameters misconfigured 1", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "host-gw"}`, nil, nil),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "VXLAN",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully, set req=false and metric=1 and create desired mc", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("false"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 1.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", desiredCNIModuleConfigName)
			Expect(cm.Exists()).To(BeTrue())
			Expect(cm.Field(`data.cni-flannel-mc\.yaml`).Exists()).To(BeTrue())
			Expect(cm.Field(`data.cni-flannel-mc\.yaml`).String()).To(MatchYAML(`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  creationTimestamp: null
  name: cni-flannel
spec:
  enabled: true
  settings:
    podNetworkMode: HostGW
  version: 1
status:
  message: ""
  version: ""
`))
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).Exists()).To(BeFalse())
		})
	})

	// space for easy compare

	Context("Cluster is bootstrapped, has cni secret, key `cni` eq `flannel`, cni MC exist and enabled, secret key `flannel` exist and not empty but some parameters misconfigured 2. And foreign desiredCNIModuleConfig exist", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "HostGW")
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "vxlan"}`, nil, nil),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "HostGW",
				}, nil),
				foreignDesiredCM,
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()

		})

		It("Should execute successfully, set req=false and metric=1 and create desired mc", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("false"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 1.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", desiredCNIModuleConfigName)
			Expect(cm.Field(`data.cni-flannel-mc\.yaml`).Exists()).To(BeTrue())
			Expect(cm.Field(`data.cni-flannel-mc\.yaml`).String()).To(MatchYAML(`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  creationTimestamp: null
  name: cni-flannel
spec:
  enabled: true
  settings:
    podNetworkMode: VXLAN
  version: 1
status:
  message: ""
  version: ""
`))
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).Exists()).To(BeFalse())
		})
	})

	Context("Cluster is bootstrapped, has cni secret, key `cni` eq `flannel`, cni MC exist and enabled, secret key `flannel` exist and not empty but some parameters has unexpected value", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "UnknownMode"}`, nil, nil),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "VXLAN",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully with warnings, set req=true and metric=0 and should not create desired mc", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("true"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 0.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", desiredCNIModuleConfigName)
			Expect(cm.Exists()).To(BeFalse())
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("unknown flannel podNetworkMode"))
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations`).Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).String()).To(Equal("ModuleConfig"))
		})
	})

	Context("Cluster has cni secret, key `cni` eq `flannel`, cni MC exist and enabled, secret key `flannel` exist and not empty and all parameters equal", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "vxlan"}`, nil, nil),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "VXLAN",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully, set req=true and metric=0 and should not create desired mc", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("true"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 0.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", desiredCNIModuleConfigName)
			Expect(cm.Exists()).To(BeFalse())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations`).Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).String()).To(Equal("ModuleConfig"))
		})
	})

	Context("Cluster has cni secret with annotation having non-standard priority value", func() {
		BeforeEach(func() {
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "vxlan"}`, nil, map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "CustomValue",
				}),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully, set req=true and metric=0 and should not create desired mc", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("true"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 0.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", desiredCNIModuleConfigName)
			Expect(cm.Exists()).To(BeFalse())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations`).Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).String()).To(Equal("CustomValue"))
		})
	})

	Context("Cluster has cni secret, MC enabled=nil (implicitly enabled)", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")
			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "host-gw"}`, nil, nil),
				cniMCYAML(cniName, nil, map[string]any{
					"podNetworkMode": "VXLAN",
				}, nil),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should detect mismatch, set req=false and metric=1 and create desired mc", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("false"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 1.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", desiredCNIModuleConfigName)
			Expect(cm.Exists()).To(BeTrue())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).Exists()).To(BeFalse())
		})
	})

	Context("Cluster is bootstrapped, has cni secret(with mismatched configuration) created after MC, so MC takes priority", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")

			mcTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)     // earlier timestamp
			secretTime := time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC) // 1 day later

			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "host-gw"}`, &secretTime, nil),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "VXLAN",
				}, &mcTime),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully, set req=true and metric=0 and should not create desired mc (secret created after MC)", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("true"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 0.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", "desired-cni-moduleconfig")
			Expect(cm.Exists()).To(BeFalse())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations`).Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).String()).To(Equal("ModuleConfig"))
		})
	})

	Context("Cluster is bootstrapped, has cni secret(with mismatched configuration) created just within 10 minutes after MC, so Secret takes priority", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")

			mcTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
			secretTime := time.Date(2023, 1, 1, 12, 5, 0, 0, time.UTC) // 5 minutes later

			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "host-gw"}`, &secretTime, nil),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "VXLAN",
				}, &mcTime),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully, set req=false and metric=1 and create desired mc (secret created within 10min after MC)", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("false"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 1.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", "desired-cni-moduleconfig")
			Expect(cm.Exists()).To(BeTrue())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).Exists()).To(BeFalse())
		})
	})

	Context("Cluster is bootstrapped, has cni secret(with mismatched configuration) created exactly 10 minutes after MC, so Secret takes priority", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")

			mcTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
			secretTime := time.Date(2023, 1, 1, 12, 10, 0, 0, time.UTC) // exactly 10 minutes later

			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "host-gw"}`, &secretTime, nil),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "VXLAN",
				}, &mcTime),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully, set req=false and metric=1 and create desired mc (secret created exactly 10min after MC)", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("false"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 1.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", "desired-cni-moduleconfig")
			Expect(cm.Exists()).To(BeTrue())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).Exists()).To(BeFalse())
		})
	})

	Context("Cluster is bootstrapped, has cni secret(with mismatched configuration) created just after 10 minutes threshold, so MC takes priority", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")

			mcTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
			secretTime := time.Date(2023, 1, 1, 12, 10, 1, 0, time.UTC) // 10 minutes and 1 second later

			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "host-gw"}`, &secretTime, nil),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "VXLAN",
				}, &mcTime),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully, set req=true and metric=0 and should not create desired mc (secret created after 10min threshold)", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("true"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 0.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", "desired-cni-moduleconfig")
			Expect(cm.Exists()).To(BeFalse())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations`).Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).String()).To(Equal("ModuleConfig"))
		})
	})

	Context("Cluster is bootstrapped, has cni secret(with mismatched configuration) created much earlier than MC, so Secret takes priority", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ConfigValuesSet("cniFlannel.podNetworkMode", "VXLAN")

			secretTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC) // 1 day earlier
			mcTime := time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC)     // 1 day later

			resources := []string{
				cniSecretYAML(cni, `{"podNetworkMode": "host-gw"}`, &secretTime, nil),
				cniMCYAML(cniName, ptr.To(true), map[string]any{
					"podNetworkMode": "VXLAN",
				}, &mcTime),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should execute successfully, set req=false and metric=1 and create desired mc (secret created much earlier than MC)", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("false"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 1.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", "desired-cni-moduleconfig")
			Expect(cm.Exists()).To(BeTrue())
			Expect(cm.Field(`data.cni-flannel-mc\.yaml`).Exists()).To(BeTrue())
			Expect(cm.Field(`data.cni-flannel-mc\.yaml`).String()).To(MatchYAML(`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  creationTimestamp: null
  name: cni-flannel
spec:
  enabled: true
  settings:
    podNetworkMode: HostGW
  version: 1
status:
  message: ""
  version: ""
`))
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`metadata.annotations.network\.deckhouse\.io/cni-configuration-source-priority`).Exists()).To(BeFalse())
		})
	})
})
