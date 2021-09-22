/*
Copyright 2021 Flant JSC

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

var _ = Describe("Modules :: prometheus :: hooks :: remove_unlabeled_pdb ::", func() {
	const (
		initValuesString       = `{"prometheus": {"internal":{"prometheusMain":{}, "prometheusLongterm":{} }}}`
		initConfigValuesString = `{}`
	)
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("1 pdb with helm owner and 1 pdb is absent", func() {
		BeforeEach(func() {
			f.KubeStateSet(pdbs)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Must delete trickster pdb", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("PodDisruptionBudget", "d8-monitoring", "trickster").Exists()).To(BeFalse())
		})

		It("Must keep grafana pdb", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("PodDisruptionBudget", "d8-monitoring", "grafana").Exists()).To(BeTrue())
		})

	})
})

const (
	pdbs = `
apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  labels:
    app: grafana
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: prometheus
  name: grafana
  namespace: d8-monitoring
spec:
  minAvailable: 0
  selector:
    matchLabels:
      app: grafana
---
apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  labels:
    app: trickster
    heritage: deckhouse
    module: prometheus
  name: trickster
  namespace: d8-monitoring
spec:
  minAvailable: 0
  selector:
    matchLabels:
      app: grafana
`
)
