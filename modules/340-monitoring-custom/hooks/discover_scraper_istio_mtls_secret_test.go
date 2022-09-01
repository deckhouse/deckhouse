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

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: monitoring-custom :: hooks :: discover_scraper_istio_mtls_secret ::", func() {

	f := HookExecutionConfigInit(
		`{"monitoringCustom":{"internal":{}},"global":{"enabledModules":[]}}`,
		`{}`,
	)

	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(mTLSSwitchPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(mTLSSwitchPath).Bool()).To(BeFalse())
		})
	})

	var istioMTLSCertSecret = `
---
apiVersion: v1
data:
  tls.crt: MTExCg==
  tls.key: MTExCg==
kind: Secret
metadata:
  name: prometheus-scraper-istio-mtls
  namespace: d8-monitoring
type: kubernetes.io/tls
	`

	Context("Istio mTLS certificate exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(istioMTLSCertSecret))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(mTLSSwitchPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(mTLSSwitchPath).Bool()).To(BeTrue())
		})
	})

})
