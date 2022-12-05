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
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: ingress-nginx :: hooks :: upmeter_discovery ::", func() {
	var (
		valuesKey  = "ingressNginx.internal.upmeterDiscovery.controllerNames"
		initValues = `{"ingressNginx":{"internal":{ }}}` // no value intentionally

		f = HookExecutionConfigInit(initValues, `{}`)
	)
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", true)

	DescribeTable("Objects discovery for Upmeter dynamic probes",
		func(objectsYAMLs, want []string) {
			yamlState := strings.Join(objectsYAMLs, "\n---\n")
			f.BindingContexts.Set(f.KubeStateSet(yamlState))

			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())

			var got []string
			value := f.ValuesGet(valuesKey).Raw
			if err := yaml.Unmarshal([]byte(value), &got); err != nil {
				panic(err)
			}

			Expect(got).To(Equal(want))
		},
		Entry("No object, no cloud", []string{}, []string{}),
		Entry(
			"Single controller",
			[]string{
				ingressNginxControllerYAML("main"),
			},
			[]string{"main"},
		),
		Entry(
			"Two controllers",
			[]string{
				ingressNginxControllerYAML("main"),
				ingressNginxControllerYAML("main-w-pp"),
			},
			[]string{"main", "main-w-pp"},
		),
		Entry(
			"Controllers are sorted",
			[]string{
				ingressNginxControllerYAML("bbb"),
				ingressNginxControllerYAML("aaa"),
				ingressNginxControllerYAML("ccc"),
			},
			[]string{"aaa", "bbb", "ccc"},
		),
	)
})

func ingressNginxControllerYAML(name string) string {
	tpl := `
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: %q
spec:
  ingressClass: nginx
  inlet: LoadBalancer
`
	return fmt.Sprintf(tpl, name)
}
