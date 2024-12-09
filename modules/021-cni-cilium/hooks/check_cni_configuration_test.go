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

	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cni-cilium :: hooks :: check_cni_configuration", func() {
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
				APIVersion: "v1alpha1",
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

	//f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)
	f := HookExecutionConfigInit(`{"cniCilium": {"internal": {}}}`, `{"cniCilium":{}}`)
	//f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("Cluster has not d8-cni-configuration secret and has not cni mc", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		FIt("ExecuteSuccessfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster has d8-cni-configuration secret and has correct MC", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			f.ConfigValuesSet("cniCilium.masqueradeMode", "BPF")
			resources := []string{
				cniSecretYAML("cilium", `{"mode": "VXLAN", "masqueradeMode": "BPF"}`),
				cniMCYAML("cilium", pointer.Bool(true), v1alpha1.SettingsValues{
					"tunnelMode":     "VXLAN",
					"masqueradeMode": "BPF",
				}),
			}
			f.BindingContexts.Set(f.KubeStateSet(strings.Join(resources, "\n---\n")))
			f.RunHook()
		})

		It("ExecuteSuccessfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
})
