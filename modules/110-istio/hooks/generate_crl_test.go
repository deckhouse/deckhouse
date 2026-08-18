/*
Copyright 2026 Flant JSC

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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: generate_crl ::", func() {
	f := HookExecutionConfigInit(`{"istio":{"internal":{}}}`, "")

	Context("settings.crl is unset", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("Should leave istio.internal.crl unset", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.crl").Exists()).To(BeFalse())
		})
	})

	Context("settings.crl is empty", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.crl", "   ")
			f.RunHook()
		})

		It("Should leave istio.internal.crl unset", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.crl").Exists()).To(BeFalse())
		})
	})

	Context("settings.crl PEM is set", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.crl", "-----BEGIN X509 CRL-----\nLOCAL\n-----END X509 CRL-----\n")
			f.RunHook()
		})

		It("Should copy PEM to istio.internal.crl", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(strings.TrimSpace(f.ValuesGet("istio.internal.crl").String())).To(Equal(
				"-----BEGIN X509 CRL-----\nLOCAL\n-----END X509 CRL-----",
			))
		})
	})
})
