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
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: common :: hooks :: check_cni_secret ::", func() {
	const (
		cniSecret = `
---
apiVersion: v1
data:
  cni: Zmxhbm5lbA==
  flannel: eyJwb2ROZXR3b3JrTW9kZSI6Imhvc3QtZ3cifQ==
kind: Secret
metadata:
  name: d8-cni-configuration
  namespace: kube-system
type: Opaque
`
	)

	f := HookExecutionConfigInit(`{"common":{"internal": {}}}`, `{}`)
	Context("Empti cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("common.internal.cniSecretData").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with valid secret", func() {
		BeforeEach(func() {
			f.KubeStateSet(cniSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			d := f.ValuesGet("common.internal.cniSecretData").String()
			dataYaml, _ := base64.StdEncoding.DecodeString(d)
			Expect(dataYaml).To(MatchYAML(`cni: Zmxhbm5lbA==
flannel: eyJwb2ROZXR3b3JrTW9kZSI6Imhvc3QtZ3cifQ==
`))
		})
	})

})
