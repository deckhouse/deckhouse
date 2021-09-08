// Copyright 2021 Flant CJSC
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

var _ = Describe("Global hooks :: empty_module_config_values_cleanup ::", func() {
	const (
		deckhouseCMWithEmptyConf = `
apiVersion: v1
data:
  dashboard: |
    {}
  deckhouse: |
    releaseChannel: Alpha
  testEnabled: "true"
  test: |
    test: true
kind: ConfigMap
metadata:
  creationTimestamp: "2021-03-18T13:39:57Z"
  labels:
    heritage: deckhouse
  name: deckhouse
  namespace: d8-system
`
		deckhouseCleanedCM = `
apiVersion: v1
data:
  deckhouse: |
    releaseChannel: Alpha
  testEnabled: "true"
  test: |
    test: true
kind: ConfigMap
metadata:
  creationTimestamp: "2021-03-18T13:39:57Z"
  labels:
    heritage: deckhouse
  name: deckhouse
  namespace: d8-system
`

		deckhouseChangedCM = `
apiVersion: v1
data:
  deckhouse: |
    releaseChannel: Alpha
  testEnabled: "true"
kind: ConfigMap
metadata:
  creationTimestamp: "2021-03-18T13:39:57Z"
  labels:
    heritage: deckhouse
  name: deckhouse
  namespace: d8-system
`
	)

	f := HookExecutionConfigInit("{}", "{}")

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with deckhouse ConfigMap which contains empty config", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(deckhouseCMWithEmptyConf))
			f.RunHook()
		})

		It("Hook should filter empty config values sections", func() {
			Expect(f).To(ExecuteSuccessfully())

			cm := f.KubernetesResource("ConfigMap", "d8-system", "deckhouse")

			Expect(cm.ToYaml()).To(MatchYAML(deckhouseCleanedCM))
		})
	})

	Context("Cluster with cleaned deckhouse ConfigMap", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(deckhouseCleanedCM))
			f.RunHook()
		})

		It("Does not change ConfigMap", func() {
			Expect(f).To(ExecuteSuccessfully())

			cm := f.KubernetesResource("ConfigMap", "d8-system", "deckhouse")

			Expect(cm.ToYaml()).To(MatchYAML(deckhouseCleanedCM))
		})

		Context("Changing ConfigMap ", func() {
			Context("Changes don't contain empty module configuration", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(deckhouseChangedCM))
					f.RunHook()
				})

				It("Keeps changes as is", func() {
					Expect(f).To(ExecuteSuccessfully())

					cm := f.KubernetesResource("ConfigMap", "d8-system", "deckhouse")

					Expect(cm.ToYaml()).To(MatchYAML(deckhouseChangedCM))
				})
			})

			Context("Changes contain empty module configuration", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(deckhouseCMWithEmptyConf))
					f.RunHook()
				})

				It("Filters empty config values sections", func() {
					Expect(f).To(ExecuteSuccessfully())

					cm := f.KubernetesResource("ConfigMap", "d8-system", "deckhouse")

					Expect(cm.ToYaml()).To(MatchYAML(deckhouseCleanedCM))
				})
			})
		})
	})
})
