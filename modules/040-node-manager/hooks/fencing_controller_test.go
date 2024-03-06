/*
Copyright 2023 Flant JSC

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
	"bytes"
	"text/template"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

type testCaseParams struct {
	Name               string
	FencingEnabled     bool
	MaintanenceEnabled bool
	RenewTime          time.Time
}

type testCaseResult struct {
	nodeExists bool
	podExists  bool
}

const internalNodeGroupValuesTemplate = `
nodeGroups:
- name: {{ .Name }}
  fencing:
    mode: Watchdog
`

const kubeStateTemplate = `
---
apiVersion: v1
kind: Node
metadata:
  annotations:
    test: test
    {{ if .MaintanenceEnabled }}update.node.deckhouse.io/disruption-approved: ""{{ end }}
  labels:
    {{ if .FencingEnabled }}node-manager.deckhouse.io/fencing-enabled: "true"{{ end }}
  name: {{ .Name }}
spec: {}
---
apiVersion: coordination.k8s.io/v1
kind: Lease
metadata:
  name: {{ .Name }}
  namespace: kube-node-lease
spec:
  holderIdentity: {{ .Name }}
  renewTime: {{ .RenewTime.Format "2006-01-02T15:04:05.000000Z07:00" }}
---
apiVersion: v1
kind: Pod
metadata:
  name: {{ .Name }}
  namespace: {{ .Name }}
spec:
  nodeName: {{ .Name }}
`

func TemplateToYAML(tmpl string, params interface{}) string {
	var output bytes.Buffer
	t := template.Must(template.New("").Parse(tmpl))
	_ = t.Execute(&output, params)
	return output.String()
}

var _ = Describe("Modules :: nodeManager :: hooks :: fencing_controller ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	DescribeTable("Testing fencing", func(testCase testCaseParams, want testCaseResult) {
		ngValues := TemplateToYAML(internalNodeGroupValuesTemplate, testCase)
		f.ValuesSetFromYaml(`nodeManager.internal`, []byte(ngValues))
		st := TemplateToYAML(kubeStateTemplate, testCase)
		f.BindingContexts.Set(f.KubeStateSet(st))
		f.RunHook()
		Expect(f).To(ExecuteSuccessfully())

		node := f.KubernetesGlobalResource("Node", testCase.Name)
		Expect(node.Exists()).To(BeEquivalentTo(want.nodeExists))

		pod := f.KubernetesResource("Pod", testCase.Name, testCase.Name)
		Expect(pod.Exists()).To(BeEquivalentTo(want.podExists))
	},
		Entry("Node with enabled fencing and lease updated in time", testCaseParams{
			Name:               "everything-ok",
			FencingEnabled:     true,
			MaintanenceEnabled: false,
			RenewTime:          time.Now(),
		}, testCaseResult{
			nodeExists: true,
			podExists:  true,
		}),
		Entry("Node with enabled fencing but lease time is rotten", testCaseParams{
			Name:               "rotten-lease-time",
			FencingEnabled:     true,
			MaintanenceEnabled: false,
			RenewTime:          time.Now().Add(-time.Hour),
		}, testCaseResult{
			nodeExists: false,
			podExists:  false,
		}),
		Entry("Node with disabled fencing", testCaseParams{
			Name:               "disabled-fencing",
			FencingEnabled:     false,
			MaintanenceEnabled: false,
			RenewTime:          time.Now().Add(-time.Hour),
		}, testCaseResult{
			nodeExists: true,
			podExists:  true,
		}),
		Entry("Node with enabled fencing but in maintenance mode", testCaseParams{
			Name:               "maintenance-mode",
			FencingEnabled:     true,
			MaintanenceEnabled: true,
			RenewTime:          time.Now().Add(-time.Hour),
		}, testCaseResult{
			nodeExists: true,
			podExists:  true,
		}),
	)
})
