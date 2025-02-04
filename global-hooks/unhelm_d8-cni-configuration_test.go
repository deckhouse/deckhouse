// Copyright 2025 Flant JSC
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

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: unhelm_d8-cni-configuration ::", func() {

	secretW := `
---
apiVersion: v1
data:
  cilium: eyJtb2RlIjogIkRpcmVjdFdpdGhOb2RlUm91dGVzIiwgIm1hc3F1ZXJhZGVNb2RlIjogIk5ldGZpbHRlciJ9
  cni: Y2lsaXVt
kind: Secret
metadata:
  annotations:
    meta.helm.sh/release-name: cloud-provider-openstack
    meta.helm.sh/release-namespace: d8-system
  creationTimestamp: "2023-11-23T12:49:17Z"
  labels:
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: cloud-provider-openstack
  name: d8-cni-configuration
  namespace: kube-system
  resourceVersion: "876"
  uid: e59aa054-0c06-47dc-8b38-2cf05aca6883
type: Opaque
`
	secretWO := `
---
apiVersion: v1
data:
  cilium: eyJtb2RlIjogIkRpcmVjdFdpdGhOb2RlUm91dGVzIiwgIm1hc3F1ZXJhZGVNb2RlIjogIk5ldGZpbHRlciJ9
  cni: Y2lsaXVt
kind: Secret
metadata:
  annotations:
  creationTimestamp: "2023-11-23T12:49:17Z"
  labels:
  name: d8-cni-configuration
  namespace: kube-system
  resourceVersion: "876"
  uid: e59aa054-0c06-47dc-8b38-2cf05aca6883
type: Opaque
`

	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)

	Context("Cluster has no d8-cni-configuration secret", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("ExecuteSuccessfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster has d8-cni-configuration secret with helm labels and annotations", func() {
		BeforeEach(func() {
			f.KubeStateSet(secretW)
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("ExecuteSuccessfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			d8CNISecret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(d8CNISecret.Exists()).To(BeTrue())
			Expect(d8CNISecret.Field(`metadata.annotations.meta\.helm\.sh\/release-name`).Exists()).To(BeFalse())
			Expect(d8CNISecret.Field(`metadata.annotations.meta\.helm\.sh\/release-namespace`).Exists()).To(BeFalse())
			Expect(d8CNISecret.Field(`metadata.annotations.helm\.sh\/resource-policy`).Exists()).To(BeTrue())
			Expect(d8CNISecret.Field(`metadata.labels.app\.kubernetes\.io\/managed-by`).Exists()).To(BeFalse())
			Expect(d8CNISecret.Field(`metadata.labels.heritage`).Exists()).To(BeFalse())
			Expect(d8CNISecret.Field(`metadata.labels.module`).Exists()).To(BeFalse())
		})
	})

	Context("Cluster has d8-cni-configuration secret without helm labels and annotations", func() {
		BeforeEach(func() {
			f.KubeStateSet(secretWO)
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("ExecuteSuccessfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			d8CNISecret := f.KubernetesResource("Secret", "kube-system", "d8-cni-configuration")
			Expect(d8CNISecret.Exists()).To(BeTrue())
			Expect(d8CNISecret.Field(`metadata.annotations.meta\.helm\.sh\/release-name`).Exists()).To(BeFalse())
			Expect(d8CNISecret.Field(`metadata.annotations.meta\.helm\.sh\/release-namespace`).Exists()).To(BeFalse())
			Expect(d8CNISecret.Field(`metadata.annotations.helm\.sh\/resource-policy`).Exists()).To(BeTrue())
			Expect(d8CNISecret.Field(`metadata.labels.app\.kubernetes\.io\/managed-by`).Exists()).To(BeFalse())
			Expect(d8CNISecret.Field(`metadata.labels.heritage`).Exists()).To(BeFalse())
			Expect(d8CNISecret.Field(`metadata.labels.module`).Exists()).To(BeFalse())
		})
	})
})
