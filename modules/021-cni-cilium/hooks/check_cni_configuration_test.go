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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cni-cilium :: hooks :: check_cni_configuration", func() {
	cniSecret := func(cniName, data string) string {
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
		j, err := json.Marshal(s)
		if err != nil {
			panic(err)
		}
		c, err := yaml.JSONToYAML(j)
		if err != nil {
			panic(err)
		}
		return string(c)
	}

	cniMC := func(cniName string) string {
		return fmt.Sprintf(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: %s
spec:
  enabled: true
  version: 1
`, cniName)
	}

	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("Cluster has not d8-cni-configuration secret and has not cni mc", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("ExecuteSuccessfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster has d8-cni-configuration secret and has correct CM 1", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(cniSecret("cilium", `{"mode": "VXLAN", "masqueradeMode": "BPF"}`)))
			f.ConfigValuesSet("cniCilium.tunnelMode", "VXLAN")
			f.ConfigValuesSet("cniCilium.masqueradeMode", "BPF")
			f.RunHook()
		})

		It("ExecuteSuccessfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
})
