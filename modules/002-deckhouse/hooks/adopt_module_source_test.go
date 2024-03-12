/*
Copyright 2024 Flant JSC

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

var _ = Describe("Modules :: deckhouse :: hooks :: adopt_module_source ::", func() {
	f := HookExecutionConfigInit(`{"deckhouse": { "internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleSource", false)

	Context("Cluster has ModuleSource without helm labels", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: deckhouse
spec:
  registry:
    ca: ""
    dockerCfg: YQo=
    repo: registry.deckhouse.io/fe/deckhouse/modules
    scheme: HTTPS
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ModuleSource", "deckhouse").ToYaml()).To(MatchYAML(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  annotations:
    meta.helm.sh/release-name: deckhouse
    meta.helm.sh/release-namespace: d8-system
  labels:
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: deckhouse
  name: deckhouse
spec:
  registry:
    ca: ""
    dockerCfg: YQo=
    repo: registry.deckhouse.io/fe/deckhouse/modules
    scheme: HTTPS
`))
		})
	})

})
