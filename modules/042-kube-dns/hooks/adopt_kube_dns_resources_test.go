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

const kubeDNSService = `
apiVersion: v1
kind: Service
metadata:
  name: kube-dns
  namespace: kube-system
spec:
  type: ClusterIP
  clusterIP: 10.96.0.10
  ports:
    - port: 53
      protocol: UDP
`

const coreDNSDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: coredns
  namespace: kube-system
spec:
  replicas: 1
`

var _ = Describe("KubeDns hooks :: adopt_kube_dns_resources", func() {
	f := HookExecutionConfigInit("", "")

	Context("kube-dns Service is ClusterIP", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kubeDNSService+"---"+coreDNSDeployment), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook executes successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Service kube-dns has helm metadata applied", func() {
			svc := f.KubernetesResource("Service", "kube-system", "kube-dns")
			Expect(svc.Field("metadata.labels.app\\.kubernetes\\.io/managed-by").String()).To(Equal("Helm"))
			Expect(svc.Field("metadata.labels.app\\.kubernetes\\.io/instance").String()).To(Equal("kube-dns"))
			Expect(svc.Field("metadata.annotations.meta\\.helm\\.sh/release-name").String()).To(Equal("kube-dns"))
			Expect(svc.Field("metadata.annotations.meta\\.helm\\.sh/release-namespace").String()).To(Equal("d8-system"))
		})

		It("Deployment coredns is deleted", func() {
			Expect(f.KubernetesResource("Deployment", "kube-system", "coredns").Exists()).To(BeFalse())
		})
	})
})
