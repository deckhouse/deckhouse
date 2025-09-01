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
	"strings"

	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func checkMetric(metrics []operation.MetricOperation, value float64) {
	Expect(metrics).To(HaveLen(2))
	Expect(metrics[0]).To(BeEquivalentTo(operation.MetricOperation{
		Group:  checkCNIConfigMetricGroup,
		Action: "expire",
	}))
	Expect(metrics[1].Name).To(BeEquivalentTo(checkCNIConfigMetricName))
	Expect(metrics[1].Group).To(BeEquivalentTo(checkCNIConfigMetricGroup))
	Expect(metrics[1].Action).To(BeEquivalentTo("set"))
	Expect(metrics[1].Value).To(BeEquivalentTo(ptr.To(value)))
	Expect(metrics[1].Labels).To(BeEquivalentTo(map[string]string{"cni": cniName}))
}

var _ = Describe("Modules :: cni-simple-bridge :: hooks :: check_cni_configuration", func() {

	const (
		initValuesString       = `{"cniSimpleBridge":{"internal": {}}}`
		initConfigValuesString = `{"cniSimpleBridge":{}}`
		anotherCni             = "flannel"
		foreignDesiredCM       = `
apiVersion: v1
kind: ConfigMap
metadata:
  creationTimestamp: null
  name: desiredCNIModuleConfig
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
  cni-cilium-mc.yaml: |
    apiVersion: deckhouse.io/v1alpha1
    kind: ModuleConfig
    metadata:
      creationTimestamp: null
      name: cni-cilium
    spec:
      enabled: true
      settings:
        masqueradeMode: BPF
        tunnelMode: VXLAN
        debugLogging: true
      version: 1
    status:
      message: ""
      version: ""
`
	)
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	cniSecretYAML := func(cniName, data string) string {
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
				Name:      "d8-cni-configuration",
				Namespace: "kube-system",
			},
			Data: secretData,
		}
		marshaled, _ := yaml.Marshal(s)
		return string(marshaled)
	}
	cniSecretWithAnnotationYAML := func(cniName, data string) string {
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
				Name:      "d8-cni-configuration",
				Namespace: "kube-system",
				Annotations: map[string]string{
					"network.deckhouse.io/cni-configuration-source-priority": "ModuleConfig",
				},
			},
			Data: secretData,
		}
		marshaled, _ := yaml.Marshal(s)
		return string(marshaled)
	}
	cniMCYAML := func(cniName string, enabled *bool, settings v1alpha1.SettingsValues) string {
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
				Settings: settings,
				Enabled:  enabled,
			},
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

	Context("Cluster has cni secret but key `cni` does not equal `simple-bridge`", func() {
		BeforeEach(func() {
			resources := []string{
				cniSecretYAML(anotherCni, ""),
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
				cniSecretWithAnnotationYAML(cni, ``),
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

	Context("Cluster has cni secret, key `cni` eq `simple-bridge`, but cni MC does not exist", func() {
		BeforeEach(func() {
			resources := []string{
				cniSecretYAML(cni, ``),
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

	Context("Cluster has cni secret, key `cni` eq `simple-bridge`, MC exists but explicitly disabled", func() {
		BeforeEach(func() {
			requirements.RemoveValue(cniConfigurationSettledKey)
			f.ValuesSet("global.clusterIsBootstrapped", true)
			resources := []string{
				cniSecretYAML(cni, ``),
				cniMCYAML(cniName, ptr.To(false), v1alpha1.SettingsValues{}),
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
			resources := []string{
				cniSecretYAML(cni, ``),
				cniMCYAML(cniName, ptr.To(true), v1alpha1.SettingsValues{}),
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

	Context("Cluster has cni secret, key `cni` eq `simple-bridge`, cni MC exist and enabled but secret key `simple-bridge` does not exist", func() {
		BeforeEach(func() {
			resources := []string{
				cniSecretYAML(cni, ``),
				cniMCYAML(cniName, ptr.To(true), v1alpha1.SettingsValues{}),
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

	Context("Cluster has cni secret, key `cni` eq `simple-bridge`, cni MC exist and enabled but secret key `simple-bridge` exist and it is empty", func() {
		BeforeEach(func() {
			resources := []string{
				cniSecretYAML(cni, `{}`),
				cniMCYAML(cniName, ptr.To(true), v1alpha1.SettingsValues{}),
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

	Context("Cluster has cni secret, key `cni` eq `simple-bridge`, cni MC exist and enabled", func() {
		BeforeEach(func() {
			resources := []string{
				cniSecretYAML(cni, ``),
				cniMCYAML(cniName, ptr.To(true), v1alpha1.SettingsValues{}),
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
			secretData := make(map[string][]byte)
			secretData["cni"] = []byte(cni)
			secretData[cni] = []byte(``)
			s := &v1core.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "d8-cni-configuration",
					Namespace: "kube-system",
					Annotations: map[string]string{
						"network.deckhouse.io/cni-configuration-source-priority": "CustomValue",
					},
				},
				Data: secretData,
			}
			marshaled, _ := yaml.Marshal(s)
			f.KubeStateSet(string(marshaled))
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
			resources := []string{
				cniSecretYAML(cni, ``),
				cniMCYAML(cniName, nil, v1alpha1.SettingsValues{}),
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
})
