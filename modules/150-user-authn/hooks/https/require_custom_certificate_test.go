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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: https :: require custom certificate ::", func() {
	f := HookExecutionConfigInit(`{"global": {"modules": {"https": {"mode": "CertManager"}}}, "userAuthn": {"internal": {}}}`, "")

	DescribeTable("The certificate the copy_custom_certificate hook leaves in the values",
		func(globalMode, moduleMode, certificate string, expectedError bool) {
			f.ValuesSet("global.modules.https.mode", globalMode)
			if moduleMode != "" {
				f.ValuesSet("userAuthn.https.mode", moduleMode)
			}
			f.ValuesSetFromYaml("userAuthn.internal.customCertificateData", []byte(certificate))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()

			if expectedError {
				Expect(f.GoHookError).To(MatchError(ContainSubstring("internal.customCertificateData is required")))
				return
			}
			Expect(f).To(ExecuteSuccessfully())
		},
		Entry("CertManager without a certificate", "CertManager", "", `{}`, false),
		Entry("the global CustomCertificate with the certificate", "CustomCertificate", "", `{"tls.crt": "crt", "tls.key": "key"}`, false),
		Entry("the global CustomCertificate without a certificate", "CustomCertificate", "", `{}`, true),
		Entry("the global CustomCertificate without the key", "CustomCertificate", "", `{"tls.crt": "crt", "ca.crt": "ca"}`, true),
		Entry("the module CustomCertificate without a certificate", "CertManager", "CustomCertificate", `{}`, true),
		Entry("the module Disabled over the global CustomCertificate", "CustomCertificate", "Disabled", `{}`, false),
	)
})
