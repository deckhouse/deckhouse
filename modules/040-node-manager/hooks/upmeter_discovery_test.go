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

var _ = Describe("Modules :: node-manager :: hooks :: upmeter_discovery ::", func() {
	var (
		valuesKey  = "nodeManager.internal.upmeterDiscovery.ephemeralNodeGroupNames"
		initValues = `{"nodeManager":{"internal":{} }}` // no inited values intentionally

		f = HookExecutionConfigInit(initValues, `{}`)
	)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

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
			"Nodegroup with minPerZone > 0 is counted",
			[]string{
				cloudEphemeralNodeGroupYAML("system", 2, "a", "b"),
			},
			[]string{"system"},
		),
		Entry(
			"Nodegroup with minPerZone == 0 is ignored",
			[]string{
				cloudEphemeralNodeGroupYAML("spot", 0, "a", "b"),
			},
			[]string{},
		),
		Entry(
			"Nodegroup with minPerZone-maxUnavailable > 0 is counted",
			[]string{
				cloudEphemeralNodeGroupWithMaxUnavailableYAML("system", 2, 1, "a", "b"),
			},
			[]string{"system"},
		),
		Entry(
			"Nodegroup with minPerZone-maxUnavailable == 0 is ignored",
			[]string{
				cloudEphemeralNodeGroupWithMaxUnavailableYAML("spot", 2, 2, "a", "b"),
			},
			[]string{},
		),
		Entry(
			"Nodegroup other than CloudEphemeral are ignored",
			[]string{
				cloudStaticNodeGroupYAML("gpu"),
			},
			[]string{},
		),
		Entry(
			"Nodegroups with various minPerZone values included ",
			[]string{
				cloudEphemeralNodeGroupYAML("frontend", 2, "a"),
				cloudEphemeralNodeGroupYAML("spot", 0, "a"),
				cloudEphemeralNodeGroupYAML("worker", 5, "b"),
			},
			[]string{"frontend", "worker"},
		),
		Entry(
			"All names are sorted",
			[]string{
				cloudEphemeralNodeGroupYAML("ng-b", 1, "c", "b", "a"),
				cloudEphemeralNodeGroupYAML("ng-a", 1, "b", "a", "c"),
				cloudEphemeralNodeGroupYAML("ng-c", 1, "a", "c", "b"),
			},
			[]string{"ng-a", "ng-b", "ng-c"},
		),
	)
})

func cloudEphemeralNodeGroupYAML(name string, minPerZone int64, zones ...string) string {
	tpl := `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: %q
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: %d
    zones: %s
`
	zstr := "[" + strings.Join(zones, ",") + "]"
	return fmt.Sprintf(tpl, name, minPerZone, zstr)
}

func cloudEphemeralNodeGroupWithMaxUnavailableYAML(name string, minPerZone, maxUnavailablePerZone int64, zones ...string) string {
	tpl := `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: %q
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: %d
    maxUnavailablePerZone: %d
    zones: %s
`
	zstr := "[" + strings.Join(zones, ",") + "]"
	return fmt.Sprintf(tpl, name, minPerZone, maxUnavailablePerZone, zstr)
}

func cloudStaticNodeGroupYAML(name string) string {
	tpl := `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: %q
spec:
  nodeType: CloudStatic
`
	return fmt.Sprintf(tpl, name)
}
