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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: deckhouse :: hooks :: cleanup endpoint slices ::", func() {
	f := HookExecutionConfigInit(`{"deckhouse":{}}`, `{}`)

	Context("Have a few orphan EndpointSlices", func() {
		BeforeEach(func() {
			state := `
---
addressType: IPv4
apiVersion: discovery.k8s.io/v1
endpoints:
- addresses:
  - 10.241.0.32
  conditions:
    ready: true
    serving: true
    terminating: false
  nodeName: test-master-1
  targetRef:
    kind: Pod
    name: deckhouse-6cb4c7bcfd-jf265
    namespace: d8-system
kind: EndpointSlice
metadata:
  labels:
    app: deckhouse
    endpointslice.kubernetes.io/managed-by: endpointslice-controller.k8s.io
    heritage: deckhouse
    kubernetes.io/service-name: deckhouse
    module: deckhouse
  name: deckhouse-6hs6p
  namespace: d8-system
  ownerReferences:
  - apiVersion: v1
    controller: true
    kind: Service
    name: deckhouse
ports:
- name: self
  port: 4222
  protocol: TCP
- name: webhook
  port: 4223
  protocol: TCP
---
addressType: IPv4
apiVersion: discovery.k8s.io/v1
endpoints:
- addresses:
  - 10.240.0.30
  conditions:
    ready: true
    serving: true
    terminating: false
  nodeName: test-master-2
  targetRef:
    kind: Pod
    name: deckhouse-6cb4c7bcfd-jf266
    namespace: d8-system
kind: EndpointSlice
metadata:
  labels:
    app: deckhouse
    endpointslice.kubernetes.io/managed-by: endpointslice-controller.k8s.io
    heritage: deckhouse
    kubernetes.io/service-name: deckhouse
    module: deckhouse
  name: deckhouse-6hs6d
  namespace: d8-system
  ownerReferences:
  - apiVersion: v1
    controller: true
    kind: Service
    name: deckhouse
ports:
- name: self
  port: 4222
  protocol: TCP
- name: webhook
  port: 4223
  protocol: TCP
---
addressType: IPv4
apiVersion: discovery.k8s.io/v1
endpoints:
- addresses:
  - 192.168.199.39
  conditions:
    ready: true
    serving: true
    terminating: false
  hostname: test-master-0
  nodeName: test-master-0
  targetRef:
    kind: Pod
    name: deckhouse-c6974f69-68tlp
    namespace: d8-system
kind: EndpointSlice
metadata:
  labels:
    app: deckhouse
    heritage: deckhouse
    kubernetes.io/service-name: deckhouse
    module: deckhouse
  name: deckhouse
  namespace: d8-system
ports:
- name: self
  port: 4222
  protocol: TCP
- name: webhook
  port: 4223
  protocol: TCP
`
			f.KubeStateSet(state)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})
		It("Should delete orphan EndpointSlices", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("EndpointSlice", "d8-system", "deckhouse-6hs6p").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("EndpointSlice", "d8-system", "deckhouse-6hs6d").Exists()).To(BeFalse())

			// new EndpointSlice should be kept
			Expect(f.KubernetesResource("EndpointSlice", "d8-system", "deckhouse").Exists()).To(BeTrue())
		})
	})

})
