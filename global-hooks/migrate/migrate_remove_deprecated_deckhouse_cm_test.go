// Copyright 2023 Flant JSC
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

const (
	deckhouseConfigmapdWithArgoLabel = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: deckhouse
  namespace: d8-system
  labels:
    argocd.argoproj.io/instance: aaa
`
	deckhouseConfigmapd = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: deckhouse
  namespace: d8-system
`
)

var _ = Describe("Global hooks :: migrate_remove_deprecated_deckhouse_cm ", func() {
	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)

	Context("Cluster with old deckhouse Configmap", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(deckhouseConfigmapd))
			f.RunHook()
		})

		It("ExecuteSuccessfully", func() {
			Expect(f.KubernetesResource("ConfigMap", "d8-system", "deckhouse").Exists()).Should(BeFalse())
			Expect(*f.MetricsCollector.CollectedMetrics()[0].Value).Should(Equal(0.0))
		})
	})

	Context("Cluster with old deckhouse Configmap", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(deckhouseConfigmapdWithArgoLabel))
			f.RunHook()
		})

		It("ExecuteSuccessfully", func() {
			Expect(f.KubernetesResource("ConfigMap", "d8-system", "deckhouse").Exists()).Should(BeTrue())
			Expect(*f.MetricsCollector.CollectedMetrics()[0].Value).Should(Equal(1.0))
		})
	})

})
