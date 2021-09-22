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

/*

User-stories:
1. There is Deployment 'deckhouse' in ns 'd8-system'. Hook must parse registry url from `.spec.template.spec.containers[0].image` and store it to `global.modulesImages.registry`.
2. There is Secret 'deckhouse-registry' in ns 'd8-system'. Hook must parse `.data.".dockerconfigjson"` and store it to `global.modulesImages.registryDockercfg`.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: deckhouse_registry ::", func() {
	const (
		initValuesString       = `{"global": {"modulesImages": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateDeployAndSecret = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      containers:
      - name: deckhouse
        image: registry.example.com/developers/deckhouse/dev:dashboard-spare-domain-fix
---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-registry
  namespace: d8-system
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: eHl6Cg==
`
		stateDeployOnly = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      containers:
      - name: deckhouse
        image: registry.example.com/developers/deckhouse/dev:dashboard-spare-domain-fix
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})

		Context("Deployment and Secret are created", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateDeployAndSecret))
				f.RunHook()
			})

			It("`global.modulesImages.registry` must be 'registry.example.com/developers/deckhouse' and `global.modulesImages.registryDockercfg` must be 'eHl6Cg=='", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.modulesImages.registry").String()).To(Equal("registry.example.com/developers/deckhouse"))
				Expect(f.ValuesGet("global.modulesImages.registryDockercfg").String()).To(Equal("eHl6Cg=="))
			})
		})
	})

	Context("Deployment and Secret are in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeployAndSecret))
			f.RunHook()
		})

		It("`global.modulesImages.registry` must be 'registry.example.com/developers/deckhouse' and `global.modulesImages.registryDockercfg` must be 'eHl6Cg=='", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.modulesImages.registry").String()).To(Equal("registry.example.com/developers/deckhouse"))
			Expect(f.ValuesGet("global.modulesImages.registryDockercfg").String()).To(Equal("eHl6Cg=="))
		})

		Context("Secret was deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateDeployOnly))
				f.RunHook()
			})

			It("Hook must fail", func() {
				Expect(f).To(Not(ExecuteSuccessfully()))
			})
		})
	})
})
