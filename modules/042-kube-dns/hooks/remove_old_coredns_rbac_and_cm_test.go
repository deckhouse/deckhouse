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

const kubeDNSResources = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:coredns
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:coredns
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: coredns
  namespace: kube-system
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns
  namespace: kube-system
`

var _ = Describe("KubeDns hooks :: removeKubeDNSRBACAndConfigMap", func() {
	f := HookExecutionConfigInit("", "")

	Context("All kube-dns RBAC and ConfigMap exist", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kubeDNSResources), f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook executes successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("All resources are marked for deletion", func() {
			expectResourcesDeleted(f)
		})
	})

	Context("All kube-dns RBAC and ConfigMap do not exist", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook executes successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("All resources are marked for deletion", func() {
			expectResourcesDeleted(f)
		})
	})
})

func expectResourcesDeleted(f *HookExecutionConfig) {
	Expect(f.KubernetesResource("ClusterRole", "", "system:coredns").Exists()).To(BeFalse())
	Expect(f.KubernetesResource("ClusterRoleBinding", "", "system:coredns").Exists()).To(BeFalse())
	Expect(f.KubernetesResource("ServiceAccount", "kube-system", "coredns").Exists()).To(BeFalse())
	Expect(f.KubernetesResource("ConfigMap", "kube-system", "coredns").Exists()).To(BeFalse())
}
