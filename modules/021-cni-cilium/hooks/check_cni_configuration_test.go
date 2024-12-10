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

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"

	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

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

var _ = Describe("Modules :: cni-cilium :: hooks :: check_cni_configuration", func() {

	const (
		initValuesString       = `{"cniCilium":{"internal": {}}}`
		initConfigValuesString = `{"cniCilium":{}}`
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

		It("ExecuteSuccessfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("true"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 0.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", "desiredCNIModuleConfig")
			Expect(cm.Exists()).To(BeFalse())
		})
	})

	Context("Cluster has cni secret but key `cni` does not equal `cilium`", func() {})

	Context("Cluster has cni secret, key `cni` eq `cilium`, but cni MC does not exist or it not explicitly enabled", func() {
		BeforeEach(func() {
			resources := []string{
				cniSecretYAML(cni, `{"mode": "VXLAN", "masqueradeMode": "BPF"}`),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()

		})

		It("ExecuteSuccessfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("false"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 1.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", "desiredCNIModuleConfig")
			Expect(cm.Exists()).To(BeTrue())
		})
	})

	Context("Cluster has cni secret, key `cni` eq `cilium`, cni MC exist and enabled but secret key `cilium` does not exist", func() {})

	Context("??Cluster has cni secret, key `cni` eq `cilium`, cni MC exist and enabled, secret key `cilium` exist but it is empty ??", func() {})

	Context("Cluster has cni secret, key `cni` eq `cilium`, cni MC exist and enabled, secret key `cilium` exist and not empty but some parameters misconfigured", func() {})

	Context("Cluster has cni secret, key `cni` eq `cilium`, cni MC exist and enabled, secret key `cilium` exist and not empty but some parameters has unexpected value", func() {})

	Context("Cluster has cni secret, key `cni` eq `cilium`, cni MC exist and enabled, secret key `cilium` exist and not empty and all parameters equal", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			f.ConfigValuesSet("cniCilium.masqueradeMode", "BPF")
			resources := []string{
				cniSecretYAML(cni, `{"mode": "VXLAN", "masqueradeMode": "BPF"}`),
				cniMCYAML(cniName, pointer.Bool(true), v1alpha1.SettingsValues{
					"tunnelMode":     "VXLAN",
					"masqueradeMode": "BPF",
				}),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("ExecuteSuccessfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(cniConfigurationSettledKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEquivalentTo("true"))
			checkMetric(f.MetricsCollector.CollectedMetrics(), 0.0)
			cm := f.KubernetesResource("ConfigMap", "d8-system", "desiredCNIModuleConfig")
			Expect(cm.Exists()).To(BeFalse())
		})
	})
})
