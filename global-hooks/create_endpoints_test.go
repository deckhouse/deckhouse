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
			f.KubeStateSet("")
			generateEndpoints()
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Should overwrite Endpoint", func() {
			Expect(f).To(ExecuteSuccessfully())
			ep := f.KubernetesResource("Endpoints", "d8-system", "deckhouse")
			Expect(ep.Field("subsets.0.addresses.0.ip").String()).To(Equal("192.168.1.1"))
			Expect(ep.Field("subsets.0.addresses.0.nodeName").String()).To(Equal("test-node"))
			Expect(ep.Field("subsets.0.addresses.0.targetRef.name").String()).To(Equal("deckhouse-test-1"))
			Expect(len(ep.Field("subsets.0.ports").Array())).To(Equal(2))
		})

		It("Should create EndpointSlice", func() {
			eps := f.KubernetesResource("EndpointSlice", "d8-system", "deckhouse")

			Expect(eps.Field("endpoints.0.addresses.0").String()).To(Equal("192.168.1.1"))
			Expect(eps.Field("endpoints.0.nodeName").String()).To(Equal("test-node"))
			Expect(eps.Field("endpoints.0.targetRef.name").String()).To(Equal("deckhouse-test-1"))
			Expect(len(eps.Field("ports").Array())).To(Equal(2))
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
    port: 9650
    protocol: TCP
  - name: webhook
    port: 9651
    protocol: TCP
`
	var ep corev1.Endpoints
	_ = yaml.Unmarshal([]byte(epYaml), &ep)
	_, _ = dependency.TestDC.MustGetK8sClient().CoreV1().Endpoints("d8-system").Create(context.TODO(), &ep, metav1.CreateOptions{})
}
