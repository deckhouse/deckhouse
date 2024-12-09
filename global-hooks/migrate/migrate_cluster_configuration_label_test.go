// Copyright 2024 Flant JSC
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

var _ = Describe("Global hooks :: migrate cluster_configuration labels", func() {
	f := HookExecutionConfigInit(`{"global": {"internal": {"modules": {"resourcesRequests": {}}}}}`, `{}`)

	Context("Secret without label", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
data: {}
kind: Secret
metadata:
  labels:
    a: b
  name: d8-cluster-configuration
  namespace: kube-system
type: Opaque
`))
			f.RunHook()
		})

		It("Should have added label name", func() {
			Expect(f).To(ExecuteSuccessfully())
			sec := f.KubernetesResource("Secret", "kube-system", "d8-cluster-configuration")
			Expect(sec.Field("metadata.labels.name").String()).To(Equal("d8-cluster-configuration"))
		})
	})

	Context("Secret with label", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
data: {}
kind: Secret
metadata:
  labels:
    a: b
    c: d
    name: d8-cluster-configuration
  name: d8-cluster-configuration
  namespace: kube-system
type: Opaque
`))
			f.RunHook()
		})

		It("Should be kept untouch", func() {
			Expect(f).To(ExecuteSuccessfully())
			sec := f.KubernetesResource("Secret", "kube-system", "d8-cluster-configuration")
			Expect(sec.Field("metadata.labels").Map()).To(HaveLen(3))
		})
	})
})
