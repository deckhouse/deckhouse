/*
Copyright 2022 Flant JSC

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
	v1 "k8s.io/api/core/v1"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: descheduler :: hooks :: migrate_from_cm ::", func() {
	f := HookExecutionConfigInit(`{"descheduler":{"internal":{}}}`, ``)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Descheduler", false)

	Context("Cluster with configured descheduler", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("descheduler.tolerations", []v1.Toleration{
				{
					Key:   "test",
					Value: "test",
				},
			})
			f.ConfigValuesSet("descheduler.removePodsHavingTooManyRestarts", true)
			f.KubeStateSet("")
			f.RunHook()
		})

		It("Should create the default Descheduler CR", func() {
			Expect(f).To(ExecuteSuccessfully())

			legacyCR := f.KubernetesGlobalResource("Descheduler", "legacy")
			Expect(legacyCR.ToYaml()).To(MatchYAML(`
apiVersion: deckhouse.io/v1alpha1
kind: Descheduler
metadata:
  name: legacy
spec:
  deploymentTemplate:
    tolerations:
    - key: test
      value: test
  deschedulerPolicy:
    strategies:
      removePodsHavingTooManyRestarts:
        enabled: true
`))
		})
	})

})
