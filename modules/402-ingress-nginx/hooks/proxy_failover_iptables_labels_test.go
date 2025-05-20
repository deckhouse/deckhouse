/*
Copyright 2025 Flant JSC

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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var readyProxyPod = `
---
apiVersion: v1
kind: Pod
metadata:
  name: proxy-failover-main
  namespace: d8-ingress-nginx
  labels:
    app: proxy-failover
    name: main
spec:
  nodeName: frontend-1
status:
  conditions:
    - type: Ready
      status: "True"
`

var nodeFrontend1 = `
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-1
  labels:
    node-role.deckhouse.io/frontend: ""
spec:
  podCIDR: 10.111.1.0/24
  podCIDRs:
    - 10.111.1.0/24
status:
  conditions:
    - type: Ready
      status: "True"
`

var nodeFrontend2WithLabel = `
---
apiVersion: v1
kind: Node
metadata:
  name: frontend-2
  labels:
    node-role.deckhouse.io/frontend: ""
    ingress-nginx-controller.deckhouse.io/need-hostwithfailover-cleanup: "false"
spec:
  podCIDR: 10.111.2.0/24
  podCIDRs:
    - 10.111.2.0/24
status:
  conditions:
    - type: Ready
      status: "True"
`

var _ = Describe("Modules :: ingress-nginx :: hooks :: proxy_failover_iptables_labels", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	const labelKey = "ingress-nginx-controller.deckhouse.io/need-hostwithfailover-cleanup"

	Context("Node with proxy-failover pod", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeFrontend1 + readyProxyPod))
			f.RunGoHook()
		})

		It("should add label with value 'true'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Node", "frontend-1").
				Field("metadata.labels").Map()).To(HaveKey(labelKey))
			Expect(f.KubernetesGlobalResource("Node", "frontend-1").
				Field("metadata.labels").Map()[labelKey].Bool()).To(BeFalse())
		})
	})

	Context("Node with label but no pod", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeFrontend2WithLabel))
			f.RunGoHook()
		})

		It("should change label value to 'false'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Node", "frontend-2").
				Field("metadata.labels").Map()).To(HaveKey(labelKey))
			Expect(f.KubernetesGlobalResource("Node", "frontend-2").
				Field("metadata.labels").Map()[labelKey].Bool()).To(BeTrue())
		})
	})
})
