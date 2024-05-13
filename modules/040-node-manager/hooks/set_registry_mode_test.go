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

var _ = Describe("Modules :: node-manager :: hooks :: set registry mode based on secret data ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager": {"internal": {}}}`, `{}`)

	const baseSecret = `
apiVersion: v1
kind: Secret
type: kubernetes.io/dockerconfigjson
metadata:
  name: deckhouse-registry
  namespace: d8-system
  labels:
    app: registry
    name: deckhouse-registry
    module: deckhouse
data:
`

	const addressLocalhost = `address: bG9jYWxob3N0OjUwMDA=`       // localhost:5000
	const addressExampleAddr = `address: ZXhhbXBsZS1hZGRyOjUwMDA=` // example-addr:5000
	const noAddress = `anotherData: VGhpcyBpcyBqdXN0IGEgdGVzdA==`  // This is just a test
	const emptyAddress = `address: ""`

	tests := map[string]struct {
		data      string
		expectVal string
	}{
		"localhost:5000":             {data: addressLocalhost, expectVal: "Indirect"},
		"example-addr:5000":          {data: addressExampleAddr, expectVal: "Direct"},
		"no address key":             {data: noAddress, expectVal: "Direct"},
		"empty address":              {data: emptyAddress, expectVal: "Direct"},
		"completely absent data key": {data: "", expectVal: "Direct"},
	}

	for name, tc := range tests {
		Context("When the secret "+name, func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(baseSecret + tc.data))
				f.RunHook()
			})

			It("`nodeManager.internal.registryMode` must be '"+tc.expectVal+"'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("nodeManager.internal.registryMode").String()).To(Equal(tc.expectVal))
			})
		})
	}
})
