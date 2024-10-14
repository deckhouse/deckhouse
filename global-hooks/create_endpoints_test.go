// Copyright 2021 Flant JSC
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
	"context"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: create_endpoints ", func() {
	f := HookExecutionConfigInit(`{"global": {}}`, `{}`)

	Context("Cluster with old Endpoint and EndpointSlices", func() {
		BeforeEach(func() {
			os.Setenv("ADDON_OPERATOR_LISTEN_ADDRESS", "192.168.1.1")
			os.Setenv("DECKHOUSE_NODE_NAME", "test-node")
			os.Setenv("DECKHOUSE_POD", "deckhouse-test-1")
			f.KubeStateSet(oldEndpointSliceYaml + `
---
apiVersion: v1
kind: Service
metadata:
  name: deckhouse
  namespace: d8-system
spec:
  ports:
  - name: self
    port: 8080
    protocol: TCP
    targetPort: 4222
  - name: webhook
    port: 4223
    protocol: TCP
    targetPort: 4223
  selector:
    app: deckhouse
`)
			generateEndpoints()
			generateOldEndpointSlice()
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Should overwrite Endpoint", func() {
			Expect(f).To(ExecuteSuccessfully())
			ep := f.KubernetesResource("Endpoints", "d8-system", "deckhouse")
			Expect(ep.Field("subsets.0.addresses.0.ip").String()).To(Equal("192.168.1.1"))
			Expect(ep.Field("subsets.0.addresses.0.nodeName").String()).To(Equal("test-node"))
			Expect(ep.Field("subsets.0.addresses.0.targetRef.name").String()).To(Equal("deckhouse-test-1"))
			Expect(len(ep.Field("subsets.0.ports").Array())).To(Equal(3))
		})

		It("Should create EndpointSlice", func() {
			eps := f.KubernetesResource("EndpointSlice", "d8-system", "deckhouse")

			Expect(eps.Field("endpoints.0.addresses.0").String()).To(Equal("192.168.1.1"))
			Expect(eps.Field("endpoints.0.nodeName").String()).To(Equal("test-node"))
			Expect(eps.Field("endpoints.0.targetRef.name").String()).To(Equal("deckhouse-test-1"))
			Expect(len(eps.Field("ports").Array())).To(Equal(3))
		})

		It("Should remove old endpointslices", func() {
			Expect(f.KubernetesResource("EndpointSlice", d8Namespace, "deckhouse-old").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", d8Namespace, d8Name).Field("spec.selector").Exists()).To(BeFalse())
		})
	})
})

func generateEndpoints() {
	epYaml := `
---
apiVersion: v1
kind: Endpoints
metadata:
  labels:
    app: deckhouse
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: deckhouse
  name: deckhouse
  namespace: d8-system
subsets:
- addresses:
  - ip: 10.241.0.32
    nodeName: main-master-2
    targetRef:
      kind: Pod
      name: deckhouse-6cb4c7bcfd-jf265
      namespace: d8-system
      resourceVersion: "2238272329"
      uid: fac9948d-d350-420d-8075-78b9e1fa66c8
  ports:
  - name: self
    port: 4222
    protocol: TCP
  - name: webhook
    port: 4223
    protocol: TCP
  - name: debug-server
    port: 9652
    protocol: TCP
`
	var ep corev1.Endpoints
	_ = yaml.Unmarshal([]byte(epYaml), &ep)
	_, _ = dependency.TestDC.MustGetK8sClient().CoreV1().Endpoints("d8-system").Create(context.TODO(), &ep, metav1.CreateOptions{})
}

const oldEndpointSliceYaml = `
addressType: IPv4
apiVersion: discovery.k8s.io/v1
endpoints:
- addresses:
  - 172.16.42.12
  conditions:
    ready: true
    serving: true
    terminating: false
  nodeName: test-1
  targetRef:
    kind: Pod
    name: deckhouse-794c777669-8vrg6
    namespace: d8-system
    resourceVersion: "2739276166"
    uid: 0ec6ad04-7aec-4be5-a1a6-b905e519f9be
kind: EndpointSlice
metadata:
  labels:
    app: deckhouse
    app.kubernetes.io/managed-by: Helm
    endpointslice.kubernetes.io/managed-by: endpointslice-controller.k8s.io
    heritage: deckhouse
    kubernetes.io/service-name: deckhouse
    module: deckhouse
  name: deckhouse-old
  namespace: d8-system
  ownerReferences:
  - apiVersion: v1
    blockOwnerDeletion: true
    controller: true
    kind: Service
    name: deckhouse
    uid: 5546653a-64ef-4d32-9bf2-c0cc7b0deded
ports:
- name: self
  port: 4222
  protocol: TCP
- name: webhook
  port: 4223
  protocol: TCP
`

func generateOldEndpointSlice() {
	var eps v1.EndpointSlice
	_ = yaml.Unmarshal([]byte(oldEndpointSliceYaml), &eps)
	_, _ = dependency.TestDC.MustGetK8sClient().DiscoveryV1().EndpointSlices("d8-system").Create(context.TODO(), &eps, metav1.CreateOptions{})
}
