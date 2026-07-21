// Copyright 2026 Flant JSC
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
1. If Deckhouse has its own Deployment in d8-system, global.deckhouseSelfHosted is true.
2. Otherwise (no own Deployment, managed from outside via kubeconfig) it is false.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const deckhouseDeploymentState = `
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
        image: registry.example.com/deckhouse:tag
`

var _ = Describe("Global hooks :: deckhouse_self_hosted ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Cluster has no deckhouse Deployment", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("`global.deckhouseSelfHosted` must be false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.deckhouseSelfHosted").Bool()).To(BeFalse())
		})
	})

	Context("Cluster has deckhouse Deployment", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(deckhouseDeploymentState))
			f.RunHook()
		})

		It("`global.deckhouseSelfHosted` must be true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.deckhouseSelfHosted").Bool()).To(BeTrue())
		})
	})
})
