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
	"bytes"
	"context"
	"text/template"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

type testCaseParams struct {
	Name               string // The name of resource (nodegroup, namespace, lease, pod)
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

const nodeAndLeaseTemplate = `
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
		// add nodegroup internal values
		ngValues := TemplateToYAML(internalNodeGroupValuesTemplate, testCase)
		f.ValuesSetFromYaml(`nodeManager.internal`, []byte(ngValues))

		// add node and lease state
		nodeAndLeaseState := TemplateToYAML(nodeAndLeaseTemplate, testCase)
		f.BindingContexts.Set(f.KubeStateSet(nodeAndLeaseState))

		// add pod on node
		_, _ = f.KubeClient().CoreV1().Pods(testCase.Name).Create(context.TODO(), &v1core.Pod{ObjectMeta: metav1.ObjectMeta{Name: testCase.Name}}, metav1.CreateOptions{})

		By("Check hook executed successfully")
		f.RunHook()
		Expect(f).To(ExecuteSuccessfully())

		By("Check node")
		node := f.KubernetesGlobalResource("Node", testCase.Name)
		Expect(node.Exists()).To(BeEquivalentTo(want.nodeExists))

		By("Check pods")
		pod, _ := f.KubeClient().CoreV1().Pods(testCase.Name).Get(context.TODO(), testCase.Name, metav1.GetOptions{})
		podExists := pod != nil
		Expect(podExists).To(BeEquivalentTo(want.podExists))
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
