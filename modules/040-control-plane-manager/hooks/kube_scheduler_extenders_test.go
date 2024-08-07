// Copyright 2022 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = FDescribe("Control-plane-manager :: kube-scheduler-extenders-test", func() {
	const initValues = `{"global": {"discovery": {"clusterDomain": "cluster.local"}}, "controlPlaneManager": {"internal": {"kubeSchedulerExtenders": [{}]}}}`
	f := HookExecutionConfigInit(initValues, `{}`)

	var nodeGroupResource = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "kubeschedulerwebhookconfigurations"}
	f.RegisterCRD(nodeGroupResource.Group, nodeGroupResource.Version, "KubeSchedulerWebhookConfiguration", false)

	const (
		extender1 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: KubeSchedulerWebhookConfiguration
metadata:
  name: test1
webhooks:
- weight: 5
  failurePolicy: Ignore
  clientConfig:
    service:
      name: scheduler
      namespace: test1
      port: 8080
      path: /scheduler
    caBundle: ABCD=
  timeoutSeconds: 5
`
		extender2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: KubeSchedulerWebhookConfiguration
metadata:
  name: test2
webhooks:
- weight: 10
  failurePolicy: Fail
  clientConfig:
    service:
      name: scheduler
      namespace: test2
      port: 8080
      path: /scheduler
    caBundle: ABCD=
  timeoutSeconds: 5
- weight: 20
  failurePolicy: Ignore
  clientConfig:
    service:
      name: scheduler2
      namespace: test2
      port: 8080
      path: /scheduler
    caBundle: ABCD=
  timeoutSeconds: 10
`
	)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It(configPath+" must be empty", func() {
			Expect(len(f.ValuesGet(configPath).Array())).To(BeZero())
		})

	})

	Context("Add one extender", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(extender1))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It(configPath+" must contain one element", func() {
			Expect(len(f.ValuesGet(configPath).Array())).To(Equal(1))
			Expect(f.ValuesGet(configPath).String()).To(MatchJSON(`
[
          {
            "urlPrefix": "https://scheduler.test1.cluster.local:8080/scheduler",
            "weight": 5,
            "timeout": 5,
            "ignorable": true,
            "caData": "ABCD="
          }
        ]

`))
		})

	})

	Context("Add three extenders", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(extender1 + extender2))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It(configPath+" must contain three elements", func() {
			Expect(len(f.ValuesGet(configPath).Array())).To(Equal(3))
			Expect(f.ValuesGet(configPath).String()).To(MatchJSON(`
[
          {
            "urlPrefix": "https://scheduler.test1.cluster.local:8080/scheduler",
            "weight": 5,
            "timeout": 5,
            "ignorable": true,
            "caData": "ABCD="
          },
          {
            "urlPrefix": "https://scheduler.test2.cluster.local:8080/scheduler",
            "weight": 10,
            "timeout": 5,
            "ignorable": false,
            "caData": "ABCD="
          },
          {
            "urlPrefix": "https://scheduler2.test2.cluster.local:8080/scheduler",
            "weight": 20,
            "timeout": 10,
            "ignorable": true,
            "caData": "ABCD="
          }
        ]

`))
		})

	})

})
